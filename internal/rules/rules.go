package rules

import (
	"log"
	"regexp"
	"sync"

	"cyd-companion/internal/config"
	"cyd-companion/internal/focus"
)

// Engine matches active windows against focus rules and calls onSwitch
// when the active profile should change.
type Engine struct {
	mu                 sync.Mutex
	rules              []compiledRule
	ruleActive         bool   // true while a rule-triggered profile is active
	preRuleProfile     string // profile to restore when rule exits ("" is valid — means "All")
	preRuleProfileSet  bool   // false until we successfully snapshot preRuleProfile
	lastSwitchedTo     string // last profile we switched to via a rule
	onSwitch           func(profileID string) error
	getProfile         func() string // reads the current active profile ID
}

type compiledRule struct {
	re        *regexp.Regexp
	profileID string
}

func New(rules []config.FocusRule, onSwitch func(profileID string) error, getProfile func() string) *Engine {
	e := &Engine{onSwitch: onSwitch, getProfile: getProfile}
	e.Update(rules)
	return e
}

// NotifyProfileChange must be called whenever the active profile changes on the
// device (either from touch UI or web UI).  If rules are active, the incoming
// profile is treated as the user's intended "base" and becomes the new restore
// target — unless the change was initiated by the rules engine itself.
func (e *Engine) NotifyProfileChange(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.ruleActive {
		return // rule not active — restore target will be snapshotted fresh on next rule entry
	}
	if id == e.lastSwitchedTo {
		return // change was driven by this engine; not a manual override
	}
	e.preRuleProfile = id
	e.preRuleProfileSet = true
	log.Printf("[Rules] external profile change while rule active → restore target updated to %q", id)
}

// Update replaces the active rule set (called when config changes).
func (e *Engine) Update(rules []config.FocusRule) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile("(?i)" + r.Match)
		if err != nil {
			log.Printf("[Rules] invalid regex %q: %v", r.Match, err)
			continue
		}
		compiled = append(compiled, compiledRule{re: re, profileID: r.ProfileID})
	}
	e.mu.Lock()
	e.rules = compiled
	e.mu.Unlock()
}

// Evaluate checks the active window against rules and fires onSwitch if needed.
// When focus enters a rule-matched app: switches to that profile, remembers the
// previous profile. When focus leaves all rule-matched apps: restores the
// remembered profile.
func (e *Engine) Evaluate(w *focus.ActiveWindow) {
	if w == nil {
		return
	}
	e.mu.Lock()
	rules := e.rules
	e.mu.Unlock()

	combined := w.Process + "|" + w.Title

	for _, r := range rules {
		if r.re.MatchString(combined) {
			e.mu.Lock()
			if !e.ruleActive {
				// Entering rule mode — snapshot the current profile so we can restore it later.
				// getProfile() returns "" for "All" (no profile filter), which is valid.
				// preRuleProfileSet distinguishes "snapshotted All" from "never snapshotted".
				if e.getProfile != nil {
					e.preRuleProfile = e.getProfile()
					e.preRuleProfileSet = true
				}
				e.ruleActive = true
			}
			needSwitch := r.profileID != e.lastSwitchedTo
			prev := e.lastSwitchedTo
			if needSwitch {
				e.lastSwitchedTo = r.profileID
			}
			e.mu.Unlock()

			if needSwitch {
				log.Printf("[Rules] → %s (process: %s)", r.profileID, w.Process)
				if err := e.onSwitch(r.profileID); err != nil {
					log.Printf("[Rules] switch to %s failed: %v — will retry next tick", r.profileID, err)
					e.mu.Lock()
					e.lastSwitchedTo = prev // revert so next tick retries
					if prev == "" {
						e.ruleActive = false // never actually entered rule mode
					}
					e.mu.Unlock()
				}
			}
			return
		}
	}

	// No rule matched — restore the pre-rule profile if we were in rule mode
	e.mu.Lock()
	if e.ruleActive {
		restore := e.preRuleProfile
		set := e.preRuleProfileSet
		e.mu.Unlock()
		if set {
			log.Printf("[Rules] left rule app, restoring → %q", restore)
			if err := e.onSwitch(restore); err != nil {
				log.Printf("[Rules] restore to %q failed: %v — will retry next tick", restore, err)
				// Keep ruleActive=true so next Evaluate() retries the restore
				return
			}
		} else {
			log.Printf("[Rules] left rule app but profile was never synced — cannot restore")
		}
		// Restore succeeded (or nothing to restore) — clear state
		e.mu.Lock()
		e.ruleActive = false
		e.lastSwitchedTo = ""
		e.preRuleProfile = ""
		e.preRuleProfileSet = false
		e.mu.Unlock()
	} else {
		e.mu.Unlock()
	}
}
