// Package tray manages the system tray icon and menu.
package tray

import (
	"time"

	"github.com/energye/systray"
)

// Callbacks holds actions triggered by tray menu items.
type Callbacks struct {
	OnOpen          func()         // Show companion window
	OnOpenDashboard func()         // Open CYD Web UI in browser
	OnQuit          func()         // Quit the application
	StatusFunc      func() string  // Returns current status string for tray display
	GetAutoStart    func() bool    // Returns current autostart state
	SetAutoStart    func(bool)     // Toggles autostart
}

// Run initialises the system tray. Blocks until the tray is destroyed.
// Call in a goroutine (safe on Windows/Linux; macOS requires main goroutine).
func Run(cb Callbacks) {
	systray.Run(
		func() { onReady(cb) },
		func() {},
	)
}

func onReady(cb Callbacks) {
	systray.SetIcon(iconPNG)
	systray.SetTitle("CYD Companion")
	systray.SetTooltip("CYD Dashboard Companion")

	// Non-clickable status item at the top
	mStatus := systray.AddMenuItem("Checking…", "Connection status")
	mStatus.Disable()
	systray.AddSeparator()

	mOpen      := systray.AddMenuItem("Open", "Show companion window")
	mDashboard := systray.AddMenuItem("Open dashboard", "Open CYD Web UI in browser")
	systray.AddSeparator()

	var mAutoStart *systray.MenuItem
	if cb.GetAutoStart != nil {
		checked := cb.GetAutoStart()
		mAutoStart = systray.AddMenuItemCheckbox("Start at login", "Run companion on system startup", checked)
	}

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop companion app")

	// Goroutine: poll StatusFunc and update the tray item + tooltip.
	if cb.StatusFunc != nil {
		go func() {
			for {
				s := cb.StatusFunc()
				mStatus.SetTitle(s)
				systray.SetTooltip("CYD Companion — " + s)
				time.Sleep(2 * time.Second)
			}
		}()
	}

	mOpen.Click(func() {
		if cb.OnOpen != nil {
			cb.OnOpen()
		}
	})
	mDashboard.Click(func() {
		if cb.OnOpenDashboard != nil {
			cb.OnOpenDashboard()
		}
	})
	if mAutoStart != nil {
		mAutoStart.Click(func() {
			if mAutoStart.Checked() {
				mAutoStart.Uncheck()
			} else {
				mAutoStart.Check()
			}
			if cb.SetAutoStart != nil {
				cb.SetAutoStart(mAutoStart.Checked())
			}
		})
	}
	mQuit.Click(func() {
		systray.Quit()
		if cb.OnQuit != nil {
			cb.OnQuit()
		}
	})
}
