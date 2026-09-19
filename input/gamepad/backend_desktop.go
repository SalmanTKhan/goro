//go:build (windows || linux || darwin) && !android

package gamepad

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/input"
)

// Backend is the desktop SDL3 gamepad adapter. It intentionally polls the
// standardized gamepad interface rather than depending on vendor-specific
// DualSense HID layouts.
type Backend struct {
	mu           sync.Mutex
	loaded       bool
	devices      map[sdl.JoystickID]*device
	activeID     sdl.JoystickID
	resetRequest bool
	closed       bool
	lastReconcil time.Time
	noGamepadLog time.Time
	trace        bool
}

type device struct {
	id       sdl.JoystickID
	gamepad  *sdl.Gamepad
	snapshot input.ControllerSnapshot
	lastUsed time.Time
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
	{sdl.HINT_JOYSTICK_HIDAPI_STEAMDECK, "1"},
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
	backend := &Backend{loaded: true, devices: make(map[sdl.JoystickID]*device), trace: os.Getenv("GORO_CONTROLLER_TRACE") == "1"}
	found, err := backend.refresh()
	if err != nil {
		backend.Close()
		return nil, err
	}
	if !found {
		glog.Warnf("SDL3 initialized but no standardized gamepad was found; controller input will remain unavailable until a gamepad is connected")
	}
	return backend, nil
}

func (b *Backend) Poll() (input.ControllerSnapshot, error) {
	if b == nil {
		return input.ControllerSnapshot{}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return input.ControllerSnapshot{}, nil
	}
	if b.resetRequest {
		b.resetRequest = false
		b.lastReconcil = time.Time{}
		b.tracef("controller reconcile applied reason=focus-restored devices=%d", len(b.devices))
	}
	sdl.PumpEvents()
	sdl.UpdateGamepads()
	if err := b.drainEvents(); err != nil {
		return input.ControllerSnapshot{}, err
	}
	// Re-enumerate periodically so missed hotplug events and suspend/resume
	// handle invalidation cannot leave the backend permanently blind.
	now := time.Now()
	if len(b.devices) == 0 || now.Sub(b.lastReconcil) >= reconcileInterval {
		b.lastReconcil = now
		if _, err := b.refresh(); err != nil {
			return input.ControllerSnapshot{}, err
		}
	}
	if len(b.devices) == 0 {
		if b.noGamepadLog.IsZero() || now.Sub(b.noGamepadLog) >= 10*time.Second {
			glog.Debugf("SDL3 gamepad poll: no standardized gamepad available")
			b.noGamepadLog = now
		}
		return input.ControllerSnapshot{}, nil
	}
	for id, current := range b.devices {
		next := b.snapshot(current.gamepad, id)
		if meaningfulActivity(current.snapshot, next) {
			current.lastUsed = now
			if b.activeID != id {
				b.tracef("controller active %d -> %d reason=%s axis=(%.2f,%.2f)", b.activeID, id, activityReason(current.snapshot, next), next.LeftX, next.LeftY)
			}
			b.activeID = id
		}
		current.snapshot = next
	}
	if active, ok := b.devices[b.activeID]; ok {
		return active.snapshot, nil
	}
	return input.ControllerSnapshot{}, nil
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
			glog.Infof("SDL3 gamepad added event")
			if _, err := b.refresh(); err != nil {
				return err
			}
		case sdl.EVENT_GAMEPAD_REMOVED:
			device := event.GamepadDeviceEvent()
			if device != nil {
				glog.Warnf("SDL3 gamepad removed event id=%d", device.Which)
			}
			if device != nil {
				// Steam Input can replace its virtual Deck pad during profile
				// activation. Re-enumerate before closing the handle: a transient
				// removal event must not make the controller disappear when the
				// same device is still present (or has already been recreated).
				if _, err := b.refresh(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (b *Backend) closeGamepad() {
	for id, current := range b.devices {
		current.gamepad.Close()
		delete(b.devices, id)
	}
	b.activeID = 0
}

// Rumble drives both motors. Magnitudes are in [0, 1].
func (b *Backend) Rumble(low, high float32, duration time.Duration) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	active := b.devices[b.activeID]
	if active == nil {
		return nil
	}
	err := active.gamepad.Rumble(rumbleMagnitude(low), rumbleMagnitude(high), uint32(duration.Milliseconds()))
	b.tracef("rumble id=%d success=%t", active.id, err == nil)
	return err
}

// SetLED sets the light bar colour on controllers that have one.
func (b *Backend) SetLED(red, green, blue uint8) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	active := b.devices[b.activeID]
	if active == nil {
		return nil
	}
	err := active.gamepad.SetLED(red, green, blue)
	b.tracef("led id=%d success=%t", active.id, err == nil)
	return err
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
	present := make(map[sdl.JoystickID]bool, len(ids))
	for _, id := range ids {
		present[id] = true
		if _, ok := b.devices[id]; ok {
			continue
		}
		gamepad, err := id.OpenGamepad()
		if err != nil {
			glog.Warnf("open SDL gamepad %d failed: %v", id, err)
			continue
		}
		b.devices[id] = &device{id: id, gamepad: gamepad}
		kind := diagnosticKind(gamepad.Name(), gamepad.Type())
		glog.Infof("SDL3 gamepad opened id=%d name=%q type=%s", id, gamepad.Name(), kind)
		b.tracef("controller add id=%d name=%q kind=%s touchpads=%d", id, gamepad.Name(), kind, gamepad.NumTouchpadFingers(0))
	}
	for id, current := range b.devices {
		if present[id] {
			continue
		}
		glog.Warnf("SDL3 gamepad enumeration no longer contains id=%d", id)
		current.gamepad.Close()
		delete(b.devices, id)
		if b.activeID == id {
			b.tracef("controller remove id=%d active=true", id)
			b.activeID = 0
		} else {
			b.tracef("controller remove id=%d active=false", id)
		}
	}
	b.noGamepadLog = time.Time{}
	return len(b.devices) > 0, nil
}

func (b *Backend) snapshot(gamepad *sdl.Gamepad, id sdl.JoystickID) input.ControllerSnapshot {
	name := gamepad.Name()
	kind := diagnosticKind(name, gamepad.Type())
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
	set(input.ControllerButtonGuide, sdl.GAMEPAD_BUTTON_GUIDE)
	set(input.ControllerButtonMisc1, sdl.GAMEPAD_BUTTON_MISC1)
	set(input.ControllerButtonRightPaddle1, sdl.GAMEPAD_BUTTON_RIGHT_PADDLE1)
	set(input.ControllerButtonLeftPaddle1, sdl.GAMEPAD_BUTTON_LEFT_PADDLE1)
	set(input.ControllerButtonRightPaddle2, sdl.GAMEPAD_BUTTON_RIGHT_PADDLE2)
	set(input.ControllerButtonLeftPaddle2, sdl.GAMEPAD_BUTTON_LEFT_PADDLE2)
	set(input.ControllerButtonDPadUp, sdl.GAMEPAD_BUTTON_DPAD_UP)
	set(input.ControllerButtonDPadDown, sdl.GAMEPAD_BUTTON_DPAD_DOWN)
	set(input.ControllerButtonDPadLeft, sdl.GAMEPAD_BUTTON_DPAD_LEFT)
	set(input.ControllerButtonDPadRight, sdl.GAMEPAD_BUTTON_DPAD_RIGHT)
	set(input.ControllerButtonTouchpad, sdl.GAMEPAD_BUTTON_TOUCHPAD)
	var touchpads [2]input.ControllerTouch
	for touchpad := int32(0); touchpad < int32(len(touchpads)); touchpad++ {
		if gamepad.NumTouchpadFingers(touchpad) <= 0 {
			continue
		}
		var down bool
		var x, y, pressure float32
		if gamepad.TouchpadFingerState(touchpad, 0, &down, &x, &y, &pressure) {
			touchpads[touchpad] = input.ControllerTouch{Down: down, X: x, Y: y, Pressure: pressure}
		}
	}

	// Keep the logical connection alive while the handle is open. SDL can
	// briefly report Connected()==false for Steam Input's virtual Deck pad
	// during focus/profile transitions; treating that transient value as a
	// disconnect drops all actions until the next reconnect event. Removal is
	// still authoritative through SDL's removal event and refresh enumeration.
	connected := gamepad.Connected()
	if !connected {
		glog.Debugf("SDL3 gamepad handle reported disconnected; retaining it until removal reconciliation")
	}
	return input.ControllerSnapshot{
		Connected:    true,
		ID:           uint64(id),
		Name:         name,
		Kind:         kind,
		Buttons:      buttons,
		LeftX:        axis(gamepad.Axis(sdl.GAMEPAD_AXIS_LEFTX)),
		LeftY:        axis(gamepad.Axis(sdl.GAMEPAD_AXIS_LEFTY)),
		RightX:       axis(gamepad.Axis(sdl.GAMEPAD_AXIS_RIGHTX)),
		RightY:       axis(gamepad.Axis(sdl.GAMEPAD_AXIS_RIGHTY)),
		LeftTrigger:  trigger(gamepad.Axis(sdl.GAMEPAD_AXIS_LEFT_TRIGGER)),
		RightTrigger: trigger(gamepad.Axis(sdl.GAMEPAD_AXIS_RIGHT_TRIGGER)),
		Touchpads:    touchpads,
	}
}

func diagnosticKind(name string, gamepadType sdl.GamepadType) input.ControllerKind {
	if strings.Contains(strings.ToLower(name), "steam deck") {
		return input.ControllerKindSteamDeck
	}
	return controllerKind(gamepadType)
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

func meaningfulActivity(previous, next input.ControllerSnapshot) bool {
	if previous.Buttons != next.Buttons {
		return true
	}
	const axisDelta = 0.08
	axisChanged := func(a, b float32) bool {
		return float32Abs(a-b) > axisDelta
	}
	if axisChanged(previous.LeftX, next.LeftX) || axisChanged(previous.LeftY, next.LeftY) ||
		axisChanged(previous.RightX, next.RightX) || axisChanged(previous.RightY, next.RightY) ||
		axisChanged(previous.LeftTrigger, next.LeftTrigger) || axisChanged(previous.RightTrigger, next.RightTrigger) {
		return true
	}
	for i := range next.Touchpads {
		if previous.Touchpads[i].Down != next.Touchpads[i].Down {
			return true
		}
		if next.Touchpads[i].Down && (axisChanged(previous.Touchpads[i].X, next.Touchpads[i].X) || axisChanged(previous.Touchpads[i].Y, next.Touchpads[i].Y)) {
			return true
		}
	}
	return (float32Abs(next.LeftX)+float32Abs(next.LeftY) >= 0.15) ||
		(float32Abs(next.RightX)+float32Abs(next.RightY) >= 0.15) ||
		next.LeftTrigger >= 0.15 || next.RightTrigger >= 0.15
}

func activityReason(previous, next input.ControllerSnapshot) string {
	if previous.Buttons != next.Buttons {
		return "button"
	}
	if previous.Touchpads[0].Down != next.Touchpads[0].Down || previous.Touchpads[1].Down != next.Touchpads[1].Down {
		return "trackpad"
	}
	if float32Abs(next.LeftTrigger-previous.LeftTrigger) > 0.08 || float32Abs(next.RightTrigger-previous.RightTrigger) > 0.08 {
		return "trigger"
	}
	if float32Abs(next.LeftX-previous.LeftX) > 0.08 || float32Abs(next.LeftY-previous.LeftY) > 0.08 {
		return "left-stick"
	}
	return "right-stick"
}

func float32Abs(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func (b *Backend) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for id, current := range b.devices {
		current.gamepad.Close()
		delete(b.devices, id)
	}
	b.activeID = 0
	sdl.Quit()
	if b.loaded {
		if err := sdl.CloseLibrary(); err != nil {
			return fmt.Errorf("close SDL3 gamepad library: %w", err)
		}
		b.loaded = false
	}
	return nil
}

// Reset discards SDL handles and ownership so a suspend/resume or compositor
// focus restoration is handled as a fresh topology build on the next Poll.
func (b *Backend) Reset() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	if b.resetRequest {
		b.tracef("controller reconcile coalesced reason=focus-restored")
		return nil
	}
	b.resetRequest = true
	b.tracef("controller reconcile requested reason=focus-restored devices=%d", len(b.devices))
	return nil
}

func (b *Backend) tracef(format string, args ...any) {
	if b != nil && b.trace {
		glog.Infof("controller trace: "+format, args...)
	}
}