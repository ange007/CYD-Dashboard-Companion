//go:build darwin

package focus

import (
	"os/exec"
	"strings"
)

func getActiveWindow() *ActiveWindow {
	// Use osascript to query the frontmost application.
	// Does not require CGWindowListCopyWindowInfo entitlement on modern macOS.
	script := `tell application "System Events" to get {name, unix id} of first process whose frontmost is true`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return nil
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), ", ", 2)
	name := ""
	if len(parts) > 0 {
		name = strings.TrimSpace(parts[0])
	}
	return &ActiveWindow{Title: name, Process: name}
}
