// Package autostart manages run-at-login registration for the companion app.
// Each OS provides Enable, Disable, and IsEnabled backed by the native mechanism:
//   - Windows: HKCU\Software\Microsoft\Windows\CurrentVersion\Run registry value
//   - macOS: ~/Library/LaunchAgents/com.ange007.cydcompanion.plist
//   - Linux: ~/.config/autostart/cyd-companion.desktop
//
// The autostart entry always appends "--minimized" so the app starts in the
// tray without showing a window.
package autostart

import (
	"os"
	"path/filepath"
)

// exePath returns the absolute path to the running executable, resolving
// any symlinks so the autostart entry points to the real binary.
func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}
