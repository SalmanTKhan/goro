//go:build (windows || linux || darwin) && !android

package gamepad

import (
	"fmt"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/input"
)

// Backend is the desktop SDL3 gamepad adapter. It intentionally polls the
// standardized gamepad interface rather than depending on vendor-specific
// DualSense HID layouts.
type Backend struct {
	loaded       bool
	gamepad      *sdl.Gamepad
	gamepadID    sdl.JoystickID
	closed       bool
	lastReconcil time.Time
}

// reconcileInterval is how often the backend re-enumerates devices as a
// fallback. Hotplug is event-driven, but a missed add/remove event would
// otherwise wedge the pad until restart.
const reconcileInterval = time.Second

// hidapiHints must be applied before sdl.Init: after initialization the driver
// selection is already fixed, which is the usual reason a DualSense works over
// USB but not over Bluetooth.
var hidapiHints = [][2]string{
	{sdl.HINT_JOYSTICK_HIDAPI, "1"},
	{sdl.HINT_JOYSTICK_HIDAPI_PS5, "1"},
	{sdl.HINT_JOYSTICK_HIDAPI_PS5_PLAYER_LED, "1"},
	{sdl.HINT_JOYSTICK_ENHANCED_REPORTS, "1"},
	{sdl.HINT_JOYSTICK_ALLOW_BACKGROUND_EVENTS, "1"},
}

// Open initializes SDL's gamepad subsystem and returns an adapter even when
// no controller is connected. The latter is normal and keeps keyboard/mouse
// startup independent of controller availability.
func Open() (*Backend, error) {
	// Load the platform SDL library explicitly so a missing optional runtime is
	// a normal error that the renderer can log and ignore. This keeps keyboard
	// and mouse startup independent from SDL/controller availability.
	if err := loadSDL3(); err != nil {
		return nil, err
	}
	for _, hint := range hidapiHints {
		if err := sdl.SetHint(hint[0], hint[1]); err != nil {
			// A rejected hint is not fatal; it only means a device may fall back
			// to a less capable driver.
			glog.Debugf("sdl hint %s failed: %v", hint[0], err)
		}
	}
	if err := sdl.Init(sdl.INIT_GAMEPAD); err != nil {
		_ = sdl.CloseLibrary()
		return nil, fmt.Errorf("initialize SDL gamepad subsystem: %w", err)
	}
	backend := &Backend{loaded: true}
	if _, err := backend.refresh(); err != nil {
		backend.Close()
		return nil, err
	}
	return backend, nil
}

func (b *Backend) Poll() (input.ControllerSnapshot, error) {
	if b == nil || b.closed {
		return input.ControllerSnapshot{}, nil
	}
	sdl.PumpEvents()
	sdl.UpdateGamepads()
	if err := b.drainEvents(); err != nil {
		return input.ControllerSnapshot{}, err
	}
	// Re-enumerate periodically so a dropped add/remove event cannot leave the
	// backend permanently blind, and immediately whenever no pad is open.
	now := time.Now()
	if b.gamepad == nil || now.Sub(b.lastReconcil) >= reconcileInterval {
		b.lastReconcil = now
		if _, err := b.refresh(); err != nil {
			return input.ControllerSnapshot{}, err
		}
	}
	if b.gamepad == nil {
		return input.ControllerSnapshot{}, nil
	}
	return b.snapshot(), nil
}

// drainEvents consumes SDL's event queue, acting only on gamepad arrival and
// removal. Goro initializes SDL with INIT_GAMEPAD alone and takes its window,
// keyboard, and mouse from gogpu, so no other subsystem owns events here. That
// invariant is what makes discarding the rest safe — revisit this if another
// SDL subsystem is ever initialized.
func (b *Backend) drainEvents() error {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type {
		case sdl.EVENT_GAMEPAD_ADDED:
			if b.gamepad == nil {
				if _, err := b.refresh(); err != nil {
					return err
				}
			}
		case sdl.EVENT_GAMEPAD_REMOVED:
			device := event.GamepadDeviceEvent()
			if device != nil && device.Which == b.gamepadID {
				b.closeGamepad()
			}
		}
	}
	return nil
}

func (b *Backend) closeGamepad() {
	if b.gamepad != nil {
		b.gamepad.Close()
		b.gamepad = nil
	}
	b.gamepadID = 0
}

// Rumble drives both motors. Magnitudes are in [0, 1].
func (b *Backend) Rumble(low, high float32, duration time.Duration) error {
	if b == nil || b.closed || b.gamepad == nil {
		return nil
	}
	return b.gamepad.Rumble(rumbleMagnitude(low), rumbleMagnitude(high), uint32(duration.Milliseconds()))
}

// SetLED sets the light bar colour on controllers that have one.
func (b *Backend) SetLED(red, green, blue uint8) error {
	if b == nil || b.closed || b.gamepad == nil {
		return nil
	}
	return b.gamepad.SetLED(red, green, blue)
}

func rumbleMagnitude(value float32) uint16 {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 0xFFFF
	}
	return uint16(value * 0xFFFF)
}

func controllerKind(gamepadType sdl.GamepadType) input.ControllerKind {
	switch gamepadType {
	case sdl.GAMEPAD_TYPE_STANDARD:
		return input.ControllerKindStandard
	case sdl.GAMEPAD_TYPE_XBOX360, sdl.GAMEPAD_TYPE_XBOXONE:
		return input.ControllerKindXbox
	case sdl.GAMEPAD_TYPE_PS3, sdl.GAMEPAD_TYPE_PS4:
		return input.ControllerKindPlayStation4
	case sdl.GAMEPAD_TYPE_PS5:
		return input.ControllerKindPlayStation5
	case sdl.GAMEPAD_TYPE_NINTENDO_SWITCH_PRO,
		sdl.GAMEPAD_TYPE_NINTENDO_SWITCH_JOYCON_LEFT,
		sdl.GAMEPAD_TYPE_NINTENDO_SWITCH_JOYCON_RIGHT,
		sdl.GAMEPAD_TYPE_NINTENDO_SWITCH_JOYCON_PAIR:
		return input.ControllerKindSwitch
	default:
		return input.ControllerKindUnknown
	}
}

func (b *Backend) refresh() (bool, error) {
	ids, err := sdl.GetGamepads()
	if err != nil {
		return false, fmt.Errorf("enumerate SDL gamepads: %w", err)
	}
	var selected sdl.JoystickID
	found := false
	for _, id := range ids {
		if id == b.gamepadID && b.gamepad != nil {
			selected, found = id, true
			break
		}
	}
	if !found && len(ids) > 0 {
		selected, found = ids[0], true
	}
	if !found {
		b.closeGamepad()
		return false, nil
	}
	if b.gamepad != nil && b.gamepadID == selected {
		return true, nil
	}
	if b.gamepad != nil {
		b.gamepad.Close()
	}
	gamepad, err := selected.OpenGamepad()
	if err != nil {
		b.gamepad = nil
		b.gamepadID = 0
		return false, fmt.Errorf("open SDL gamepad %d: %w", selected, err)
	}
	b.gamepad = gamepad
	b.gamepadID = selected
	return true, nil
}

func (b *Backend) snapshot() input.ControllerSnapshot {
	gamepad := b.gamepad
	var buttons input.ControllerButtons
	set := func(dst input.ControllerButton, src sdl.GamepadButton) {
		buttons.Set(dst, gamepad.Button(src))
	}
	set(input.ControllerButtonSouth, sdl.GAMEPAD_BUTTON_SOUTH)
	set(input.ControllerButtonEast, sdl.GAMEPAD_BUTTON_EAST)
	set(input.ControllerButtonWest, sdl.GAMEPAD_BUTTON_WEST)
	set(input.ControllerButtonNorth, sdl.GAMEPAD_BUTTON_NORTH)
	set(input.ControllerButtonBack, sdl.GAMEPAD_BUTTON_BACK)
	set(input.ControllerButtonStart, sdl.GAMEPAD_BUTTON_START)
	set(input.ControllerButtonLeftStick, sdl.GAMEPAD_BUTTON_LEFT_STICK)
	set(input.ControllerButtonRightStick, sdl.GAMEPAD_BUTTON_RIGHT_STICK)
	set(input.ControllerButtonLeftShoulder, sdl.GAMEPAD_BUTTON_LEFT_SHOULDER)
	set(input.ControllerButtonRightShoulder, sdl.GAMEPAD_BUTTON_RIGHT_SHOULDER)
	set(input.ControllerButtonDPadUp, sdl.GAMEPAD_BUTTON_DPAD_UP)
	set(input.ControllerButtonDPadDown, sdl.GAMEPAD_BUTTON_DPAD_DOWN)
	set(input.ControllerButtonDPadLeft, sdl.GAMEPAD_BUTTON_DPAD_LEFT)
	set(input.ControllerButtonDPadRight, sdl.GAMEPAD_BUTTON_DPAD_RIGHT)
	set(input.ControllerButtonTouchpad, sdl.GAMEPAD_BUTTON_TOUCHPAD)

	return input.ControllerSnapshot{
		Connected:    gamepad.Connected(),
		ID:           uint64(b.gamepadID),
		Name:         gamepad.Name(),
		Kind:         controllerKind(gamepad.Type()),
		Buttons:      buttons,
		LeftX:        axis(gamepad.Axis(sdl.GAMEPAD_AXIS_LEFTX)),
		LeftY:        axis(gamepad.Axis(sdl.GAMEPAD_AXIS_LEFTY)),
		RightX:       axis(gamepad.Axis(sdl.GAMEPAD_AXIS_RIGHTX)),
		RightY:       axis(gamepad.Axis(sdl.GAMEPAD_AXIS_RIGHTY)),
		LeftTrigger:  trigger(gamepad.Axis(sdl.GAMEPAD_AXIS_LEFT_TRIGGER)),
		RightTrigger: trigger(gamepad.Axis(sdl.GAMEPAD_AXIS_RIGHT_TRIGGER)),
	}
}

func axis(value int16) float32 {
	if value < 0 {
		return float32(value) / 32768
	}
	return float32(value) / 32767
}

func trigger(value int16) float32 {
	if value <= 0 {
		return 0
	}
	return float32(value) / 32767
}

func (b *Backend) Close() error {
	if b == nil || b.closed {
		return nil
	}
	b.closed = true
	if b.gamepad != nil {
		b.gamepad.Close()
		b.gamepad = nil
	}
	sdl.Quit()
	if b.loaded {
		if err := sdl.CloseLibrary(); err != nil {
			return fmt.Errorf("close SDL3 gamepad library: %w", err)
		}
		b.loaded = false
	}
	return nil
}
