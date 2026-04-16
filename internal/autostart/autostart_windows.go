//go:build windows

package autostart

import (
	"golang.org/x/sys/windows/registry"
)

const regKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
const regValue = "CYDCompanion"

func Enable() error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(regValue, `"`+exe+`" --minimized`)
}

func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.DeleteValue(regValue)
}

func IsEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	_, _, err = k.GetStringValue(regValue)
	if err == registry.ErrNotExist {
		return false, nil
	}
	return err == nil, err
}
