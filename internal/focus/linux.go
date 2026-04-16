//go:build linux

package focus

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func getActiveWindow() *ActiveWindow {
	// Try xdotool (X11)
	if w := xdotoolWindow(); w != nil {
		return w
	}
	// Try hyprctl (Hyprland / wlroots Wayland)
	if w := hyprctlWindow(); w != nil {
		return w
	}
	return nil
}

func xdotoolWindow() *ActiveWindow {
	idOut, err := exec.Command("xdotool", "getactivewindow").Output()
	if err != nil {
		return nil
	}
	wid := strings.TrimSpace(string(idOut))

	titleOut, _ := exec.Command("xdotool", "getwindowname", wid).Output()
	title := strings.TrimSpace(string(titleOut))

	pidOut, _ := exec.Command("xdotool", "getwindowpid", wid).Output()
	pid := strings.TrimSpace(string(pidOut))

	process := ""
	if pid != "" {
		// /proc/<pid>/exe → symlink to executable
		if exePath, err := filepath.EvalSymlinks("/proc/" + pid + "/exe"); err == nil {
			process = filepath.Base(exePath)
		}
	}

	return &ActiveWindow{Title: title, Process: process}
}

func hyprctlWindow() *ActiveWindow {
	out, err := exec.Command("hyprctl", "activewindow", "-j").Output()
	if err != nil {
		return nil
	}
	// Simple field extraction without importing encoding/json to keep deps light.
	title := jsonField(string(out), "title")
	class := jsonField(string(out), "class")
	return &ActiveWindow{Title: title, Process: class}
}

// jsonField extracts a simple string field value from JSON without full parsing.
func jsonField(json, key string) string {
	needle := `"` + key + `":"`
	idx := strings.Index(json, needle)
	if idx < 0 {
		return ""
	}
	rest := json[idx+len(needle):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}
