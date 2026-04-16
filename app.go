package main

import (
	"context"
	"runtime"
	"sync"

	"cyd-companion/internal/autostart"
	"cyd-companion/internal/collector"
	"cyd-companion/internal/config"
	"cyd-companion/internal/rules"
	"cyd-companion/internal/ws"
)

// App is the Wails application struct. Methods on App are exposed to the frontend.
type App struct {
	ctx      context.Context
	cfg      *config.Config
	cfgPath  string
	wsClient *ws.Client
	col      *collector.Collector
	rules    *rules.Engine

	// Profiles cache — keeps the last successfully fetched list so the UI
	// can show profiles even when the device is temporarily offline.
	profilesMu     sync.RWMutex
	cachedProfiles []map[string]string
	cachedActiveID string
}

func NewApp(cfg *config.Config, cfgPath string, wsClient *ws.Client, col *collector.Collector, eng *rules.Engine) *App {
	return &App{cfg: cfg, cfgPath: cfgPath, wsClient: wsClient, col: col, rules: eng}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ── Status ────────────────────────────────────────────────────────────────────

type StatusResult struct {
	Connected     bool   `json:"connected"`
	DeviceURL     string `json:"device_url"`
	OS            string `json:"os"`
	Hostname      string `json:"hostname"`
	ActiveTitle   string `json:"active_title"`
	ActiveProcess string `json:"active_process"`
	ActiveProfile string `json:"active_profile"`
}

func (a *App) GetStatus() StatusResult {
	st := a.wsClient.GetState()

	// Fall back to cached active ID when ws state has no profile yet
	activeProfile := st.ActiveProfile
	if activeProfile == "" {
		a.profilesMu.RLock()
		activeProfile = a.cachedActiveID
		a.profilesMu.RUnlock()
	}

	return StatusResult{
		Connected:     st.Connected,
		DeviceURL:     st.DeviceURL,
		OS:            runtime.GOOS,
		ActiveTitle:   st.ActiveTitle,
		ActiveProcess: st.ActiveProcess,
		ActiveProfile: activeProfile,
	}
}

// ── Metrics ───────────────────────────────────────────────────────────────────

func (a *App) GetMetrics() []collector.Metric {
	return a.col.Catalog()
}

// ── Device diagnostics ────────────────────────────────────────────────────────

// GetDeviceDiag returns the latest device telemetry snapshot, or nil if not
// yet received (device offline or not yet polled).
func (a *App) GetDeviceDiag() *ws.DeviceDiag {
	return a.wsClient.GetState().Diag
}

// ── Config ────────────────────────────────────────────────────────────────────

func (a *App) GetConfig() *config.Config {
	return a.cfg
}

func (a *App) SaveConfig(newCfg config.Config) error {
	reconnect := a.cfg.DeviceURL != newCfg.DeviceURL ||
		a.cfg.MdnsHostHint != newCfg.MdnsHostHint ||
		a.cfg.HidRelayEnabled != newCfg.HidRelayEnabled
	*a.cfg = newCfg
	a.rules.Update(a.cfg.FocusRules)
	if reconnect {
		a.wsClient.Reconnect()
	}
	return a.cfg.Save(a.cfgPath)
}

// ── Profiles ──────────────────────────────────────────────────────────────────

type ProfilesResult struct {
	List     []map[string]string `json:"list"`
	ActiveID string              `json:"active_id"`
}

// GetProfiles fetches profiles from the device and updates the local cache.
// If the device is unreachable, returns the last known cached list.
// Returns an empty result only if the cache is also empty (never connected).
func (a *App) GetProfiles() ProfilesResult {
	list, activeID, err := a.wsClient.GetProfiles()
	if err == nil {
		// Fresh data — update cache
		a.profilesMu.Lock()
		a.cachedProfiles = list
		if activeID != "" {
			a.cachedActiveID = activeID
			// Also sync to ws state so rules engine has a restore target
			a.wsClient.SetActiveProfile(activeID)
		}
		a.profilesMu.Unlock()
		return ProfilesResult{List: list, ActiveID: activeID}
	}

	// Device offline — return cached data so UI doesn't go blank
	a.profilesMu.RLock()
	cached := a.cachedProfiles
	cachedID := a.cachedActiveID
	a.profilesMu.RUnlock()

	return ProfilesResult{List: cached, ActiveID: cachedID}
}

// ── Autostart ─────────────────────────────────────────────────────────────────

func (a *App) GetAutoStart() bool {
	ok, _ := autostart.IsEnabled()
	return ok
}

func (a *App) SetAutoStart(on bool) error {
	if on {
		return autostart.Enable()
	}
	return autostart.Disable()
}

func (a *App) SwitchProfile(id string) error {
	err := a.wsClient.SwitchProfile(id)
	if err == nil {
		// Keep cache in sync
		a.profilesMu.Lock()
		a.cachedActiveID = id
		a.profilesMu.Unlock()
	}
	return err
}
