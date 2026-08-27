//go:build android && cgo && amd64

package syscall

// goffi v0.6.3 does not provide an Android amd64 errno resolver. A zero
// address disables optional errno capture while preserving the FFI call path.
func ErrnoFnAddr() uintptr { return 0 }
