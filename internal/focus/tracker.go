package focus

// ActiveWindow holds information about the currently focused window.
type ActiveWindow struct {
	Title   string // Window title
	Process string // Executable name (e.g. "code.exe", "spotify")
}

// Tracker is the platform-specific implementation interface.
// Each platform file (windows.go, linux.go, darwin.go) provides getActiveWindow().
type Tracker struct{}

func New() *Tracker { return &Tracker{} }

// Get returns the currently active window, or nil on error / unsupported platform.
func (t *Tracker) Get() *ActiveWindow {
	return getActiveWindow()
}
