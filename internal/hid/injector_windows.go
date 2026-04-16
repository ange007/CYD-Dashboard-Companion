//go:build windows

package hid

import (
	"strings"
	"syscall"
	"unsafe"
)

// Compile-time size assertion: keyboardInput must be exactly 40 bytes (sizeof INPUT on Win64).
type _ [unsafe.Sizeof(keyboardInput{}) - 40]struct{}

var (
	user32    = syscall.NewLazyDLL("user32.dll")
	sendInput = user32.NewProc("SendInput")
)

const (
	inputKeyboard    = 1
	keyeventfKeyup   = 0x0002
	keyeventfUnicode = 0x0004
)

// INPUT layout for keyboard input on Win64 (sizeof == 40).
// type(4) + pad(4) + KEYBDINPUT[vk(2)+scan(2)+flags(4)+time(4)+pad(4)+extraInfo(8)] + pad(8)
// The trailing 8 bytes pad KEYBDINPUT (24 B) up to MOUSEINPUT union size (32 B).
type keyboardInput struct {
	inputType uint32
	_         [4]byte // alignment pad: union requires 8-byte alignment
	vk        uint16
	scan      uint16
	flags     uint32
	time      uint32
	_         [4]byte // alignment pad: extraInfo (uintptr) must be 8-byte aligned
	extraInfo uintptr
	_         [8]byte // pad KEYBDINPUT (24 B) to MOUSEINPUT union size (32 B)
}

func sendKey(vk uint16, flags uint32) {
	input := keyboardInput{
		inputType: inputKeyboard,
		vk:        vk,
		flags:     flags,
	}
	sendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
}

func sendKeyDownUp(vk uint16) {
	sendKey(vk, 0)
	sendKey(vk, keyeventfKeyup)
}

// vkMap translates ESP32 key names to Windows virtual key codes.
var vkMap = map[string]uint16{
	// Modifiers
	"KEY_LEFT_CTRL":   0xA2, // VK_LCONTROL
	"KEY_RIGHT_CTRL":  0xA3, // VK_RCONTROL
	"KEY_LEFT_SHIFT":  0xA0, // VK_LSHIFT
	"KEY_RIGHT_SHIFT": 0xA1, // VK_RSHIFT
	"KEY_LEFT_ALT":    0xA4, // VK_LMENU
	"KEY_RIGHT_ALT":   0xA5, // VK_RMENU
	"KEY_LEFT_GUI":    0x5B, // VK_LWIN
	"KEY_RIGHT_GUI":   0x5C, // VK_RWIN

	// Navigation / editing
	"KEY_ENTER":       0x0D, // VK_RETURN
	"KEY_ESC":         0x1B, // VK_ESCAPE
	"KEY_TAB":         0x09, // VK_TAB
	"KEY_BACKSPACE":   0x08, // VK_BACK
	"KEY_DELETE":      0x2E, // VK_DELETE
	"KEY_SPACE":       0x20, // VK_SPACE
	"KEY_INSERT":      0x2D, // VK_INSERT
	"KEY_HOME":        0x24, // VK_HOME
	"KEY_END":         0x23, // VK_END
	"KEY_PAGE_UP":     0x21, // VK_PRIOR
	"KEY_PAGE_DOWN":   0x22, // VK_NEXT
	"KEY_LEFT_ARROW":  0x25, // VK_LEFT
	"KEY_RIGHT_ARROW": 0x27, // VK_RIGHT
	"KEY_UP_ARROW":    0x26, // VK_UP
	"KEY_DOWN_ARROW":  0x28, // VK_DOWN
	"KEY_CAPS_LOCK":   0x14, // VK_CAPITAL
	"KEY_PRINT_SCREEN": 0x2C, // VK_SNAPSHOT
	"KEY_SCROLL_LOCK": 0x91, // VK_SCROLL
	"KEY_PAUSE":       0x13, // VK_PAUSE
	"KEY_NUM_LOCK":    0x90, // VK_NUMLOCK

	// Function keys
	"KEY_F1": 0x70, "KEY_F2": 0x71, "KEY_F3": 0x72, "KEY_F4": 0x73,
	"KEY_F5": 0x74, "KEY_F6": 0x75, "KEY_F7": 0x76, "KEY_F8": 0x77,
	"KEY_F9": 0x78, "KEY_F10": 0x79, "KEY_F11": 0x7A, "KEY_F12": 0x7B,
	"KEY_F13": 0x7C, "KEY_F14": 0x7D, "KEY_F15": 0x7E, "KEY_F16": 0x7F,
	"KEY_F17": 0x80, "KEY_F18": 0x81, "KEY_F19": 0x82, "KEY_F20": 0x83,
	"KEY_F21": 0x84, "KEY_F22": 0x85, "KEY_F23": 0x86, "KEY_F24": 0x87,

	// Media keys
	"KEY_MEDIA_PLAY_PAUSE":  0xB3, // VK_MEDIA_PLAY_PAUSE
	"KEY_MEDIA_NEXT_TRACK":  0xB0, // VK_MEDIA_NEXT_TRACK
	"KEY_MEDIA_PREV_TRACK":  0xB1, // VK_MEDIA_PREV_TRACK
	"KEY_MEDIA_STOP":        0xB2, // VK_MEDIA_STOP
	"KEY_MEDIA_VOLUME_UP":   0xAF, // VK_VOLUME_UP
	"KEY_MEDIA_VOLUME_DOWN": 0xAE, // VK_VOLUME_DOWN
	"KEY_MEDIA_MUTE":        0xAD, // VK_VOLUME_MUTE
}

// Supported reports whether HID injection works on this platform.
func Supported() bool { return true }

func resolveVK(key string) (uint16, bool) {
	if vk, ok := vkMap[key]; ok {
		return vk, true
	}
	// Single ASCII char → use VkKeyScanA
	if len(key) == 1 {
		ch := strings.ToUpper(key)[0]
		if ch >= 'A' && ch <= 'Z' {
			return uint16(ch), true // 'A'=0x41...'Z'=0x5A
		}
		if ch >= '0' && ch <= '9' {
			return uint16(ch), true // '0'=0x30...'9'=0x39
		}
	}
	return 0, false
}

func injectKeys(key string, mods []string) error {
	// Press modifiers
	for _, mod := range mods {
		if vk, ok := resolveVK(mod); ok {
			sendKey(vk, 0)
		}
	}
	// Press + release key
	if vk, ok := resolveVK(key); ok {
		sendKeyDownUp(vk)
	}
	// Release modifiers in reverse
	for i := len(mods) - 1; i >= 0; i-- {
		if vk, ok := resolveVK(mods[i]); ok {
			sendKey(vk, keyeventfKeyup)
		}
	}
	return nil
}

func injectText(text string) error {
	// Send each rune as a Unicode input event pair (down + up).
	for _, r := range text {
		ku := keyboardInput{
			inputType: inputKeyboard,
			scan:      uint16(r),
			flags:     keyeventfUnicode,
		}
		kd := ku
		kd.flags = keyeventfUnicode | keyeventfKeyup

		// Send down then up as two separate SendInput calls for maximum compatibility.
		sendInput.Call(1, uintptr(unsafe.Pointer(&ku)), unsafe.Sizeof(ku))
		sendInput.Call(1, uintptr(unsafe.Pointer(&kd)), unsafe.Sizeof(kd))
	}
	return nil
}
