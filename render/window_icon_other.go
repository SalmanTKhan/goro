//go:build !windows

package render

// On X11 the icon is set through gogpu's WithIcon (see Run); Wayland and macOS
// take it from the desktop file / app bundle. Nothing to do here.
func applyWindowIcon() {}
