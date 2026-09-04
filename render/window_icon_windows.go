package render

import (
	"syscall"
	"unsafe"
)

// GoGPU v0.44.6 leaves the Win32 window-class icon unset and documents WithIcon
// as a no-op on Windows, so the window shows the generic application icon. Push
// the icon embedded in the executable (goro_windows_*.syso, group 32512 — the
// title-bar icon goversioninfo emits) onto the window with WM_SETICON.

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procFindWindowW      = modUser32.NewProc("FindWindowW")
	procLoadImageW       = modUser32.NewProc("LoadImageW")
	procSendMessageW     = modUser32.NewProc("SendMessageW")
	procGetModuleHandleW = modKernel32.NewProc("GetModuleHandleW")
)

const (
	wmSetIcon     = 0x0080
	iconSmall     = 0
	iconBig       = 1
	imageIcon     = 1
	lrDefaultSize = 0x00000040
	lrShared      = 0x00008000

	// Resource ID goversioninfo assigns to the application/title-bar icon group.
	appIconGroupID = 32512
)

// iconApplied is read and written only from the gogpu update callback, which
// runs on a single goroutine, so it needs no synchronization.
var iconApplied bool

// applyWindowIcon is safe to call every frame; it retries until the window
// exists, then acts once.
func applyWindowIcon() {
	if iconApplied {
		return
	}
	classPtr, err := syscall.UTF16PtrFromString("GoGPUWindow")
	if err != nil {
		iconApplied = true // nothing we can do; stop retrying
		return
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(classPtr)), 0)
	if hwnd == 0 {
		return // window not up yet; a later frame retries
	}
	iconApplied = true

	hInst, _, _ := procGetModuleHandleW.Call(0)
	hIcon, _, _ := procLoadImageW.Call(hInst, appIconGroupID, imageIcon, 0, 0, lrDefaultSize|lrShared)
	if hIcon == 0 {
		return
	}
	procSendMessageW.Call(hwnd, wmSetIcon, iconBig, hIcon)
	procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, hIcon)
}
