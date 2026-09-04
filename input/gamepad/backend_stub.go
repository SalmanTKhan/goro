//go:build android || (!windows && !linux && !darwin)

package gamepad

import "github.com/kivutar/goro/input"

// Backend is a no-op on Android and other unsupported targets. Keeping this
// package available lets renderer code remain platform-neutral without
// importing SDL into the Android host.
type Backend struct{}

func Open() (*Backend, error) { return &Backend{}, nil }

func (*Backend) Poll() (input.ControllerSnapshot, error) { return input.ControllerSnapshot{}, nil }

func (*Backend) Close() error { return nil }
