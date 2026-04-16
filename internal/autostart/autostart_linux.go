//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

func desktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", "cyd-companion.desktop"), nil
}

func Enable() error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	path, err := desktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=CYD Companion
Exec=%s --minimized
X-GNOME-Autostart-enabled=true
Hidden=false
NoDisplay=false
Comment=CYD Dashboard Companion — background proxy and HID relay
`, exe)
	return os.WriteFile(path, []byte(content), 0644)
}

func Disable() error {
	path, err := desktopPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func IsEnabled() (bool, error) {
	path, err := desktopPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
