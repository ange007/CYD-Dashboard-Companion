//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const plistLabel = "com.ange007.cydcompanion"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist"), nil
}

func Enable() error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>       <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>--minimized</string>
  </array>
  <key>RunAtLoad</key>   <true/>
  <key>KeepAlive</key>   <false/>
</dict>
</plist>
`, plistLabel, exe)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	return exec.Command("launchctl", "load", path).Run()
}

func Disable() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	_ = exec.Command("launchctl", "unload", path).Run()
	return os.Remove(path)
}

func IsEnabled() (bool, error) {
	path, err := plistPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
