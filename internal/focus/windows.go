//go:build windows

package focus

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess         = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
)

const (
	processQueryLimitedInformation = 0x1000
)

func getActiveWindow() *ActiveWindow {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil
	}

	// Get window title
	titleBuf := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuf[0])), uintptr(len(titleBuf)))
	title := syscall.UTF16ToString(titleBuf)

	// Get process ID
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return &ActiveWindow{Title: title, Process: ""}
	}

	// Open process to query image name
	hProc, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if hProc == 0 {
		return &ActiveWindow{Title: title, Process: ""}
	}
	defer procCloseHandle.Call(hProc)

	pathBuf := make([]uint16, 1024)
	size := uint32(len(pathBuf))
	procQueryFullProcessImageNameW.Call(hProc, 0, uintptr(unsafe.Pointer(&pathBuf[0])), uintptr(unsafe.Pointer(&size)))
	processPath := syscall.UTF16ToString(pathBuf[:size])
	processName := filepath.Base(processPath)

	return &ActiveWindow{Title: title, Process: processName}
}
