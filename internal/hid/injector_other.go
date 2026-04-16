//go:build !windows

package hid

import "fmt"

// Supported reports whether HID injection works on this platform.
func Supported() bool { return false }

func injectKeys(key string, mods []string) error {
	return fmt.Errorf("hid: keyboard injection not supported on this platform")
}

func injectText(text string) error {
	return fmt.Errorf("hid: text injection not supported on this platform")
}
