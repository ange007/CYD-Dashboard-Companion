package hid

import "fmt"

// ExecHidMsg mirrors the WS message from the ESP32 firmware.
type ExecHidMsg struct {
	Type string   `json:"type"`
	Keys []string `json:"keys,omitempty"`
	Text string   `json:"text,omitempty"`
}

// Inject executes an HID action received from the ESP32.
// Platform-specific implementations are in injector_windows.go / injector_other.go.
func Inject(msg ExecHidMsg) error {
	switch msg.Type {
	case "text":
		if msg.Text == "" {
			return nil
		}
		return injectText(msg.Text)
	case "keys":
		if len(msg.Keys) == 0 {
			return nil
		}
		// Last element = key, preceding elements = modifiers.
		key := msg.Keys[len(msg.Keys)-1]
		mods := msg.Keys[:len(msg.Keys)-1]
		return injectKeys(key, mods)
	default:
		return fmt.Errorf("hid: unknown type %q", msg.Type)
	}
}
