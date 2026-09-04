package input

import (
	"math"
	"strings"
	"time"
)

// InputSource identifies the most recently active input device. It is used by
// the presentation layer to decide whether controller focus affordances should
// be shown without changing gameplay authority.
type InputSource uint8

const (
	InputSourceUnknown InputSource = iota
	InputSourceKeyboard
	InputSourceMouse
	InputSourceController
	InputSourceTouch
)

// ControllerButton is the standardized positional gamepad button vocabulary
// used by SDL's gamepad API. Positional names keep DualSense and Xbox labels
// interchangeable at the gameplay/action boundary.
type ControllerButton uint8

const (
	ControllerButtonSouth ControllerButton = iota
	ControllerButtonEast
	ControllerButtonWest
	ControllerButtonNorth
	ControllerButtonBack
	ControllerButtonStart
	ControllerButtonLeftStick
	ControllerButtonRightStick
	ControllerButtonLeftShoulder
	ControllerButtonRightShoulder
	ControllerButtonDPadUp
	ControllerButtonDPadDown
	ControllerButtonDPadLeft
	ControllerButtonDPadRight
	ControllerButtonTouchpad
	ControllerButtonLeftTrigger
	ControllerButtonRightTrigger
)

// ControllerButtons is a compact set of currently held controller buttons.
type ControllerButtons uint32

func (b ControllerButtons) Has(button ControllerButton) bool {
	if button >= 32 {
		return false
	}
	return b&(1<<button) != 0
}

func (b *ControllerButtons) Set(button ControllerButton, pressed bool) {
	if b == nil || button >= 32 {
		return
	}
	mask := ControllerButtons(1 << button)
	if pressed {
		*b |= mask
	} else {
		*b &^= mask
	}
}

// ControllerSnapshot is the renderer-independent state sampled from the
// active desktop gamepad. Stick axes are in [-1, 1], trigger axes in [0, 1].
// ControllerKind is the device family reported by the backend. The pointer and
// action layers are label-agnostic; this exists so the diagnostics page can
// name the device and so vendor-specific haptics stay gated to the hardware
// that supports them.
type ControllerKind uint8

const (
	ControllerKindUnknown ControllerKind = iota
	ControllerKindStandard
	ControllerKindXbox
	ControllerKindPlayStation4
	ControllerKindPlayStation5
	ControllerKindSwitch
)

func (k ControllerKind) String() string {
	switch k {
	case ControllerKindStandard:
		return "Standard"
	case ControllerKindXbox:
		return "Xbox"
	case ControllerKindPlayStation4:
		return "DualShock 4"
	case ControllerKindPlayStation5:
		return "DualSense"
	case ControllerKindSwitch:
		return "Switch"
	default:
		return "Unknown"
	}
}

type ControllerSnapshot struct {
	Connected    bool
	ID           uint64
	Name         string
	Kind         ControllerKind
	Buttons      ControllerButtons
	LeftX        float32
	LeftY        float32
	RightX       float32
	RightY       float32
	LeftTrigger  float32
	RightTrigger float32
}

const controllerTriggerButtonThreshold = 0.5

// ButtonDown exposes digital buttons and the standardized trigger controls
// through one query. SDL reports L2/R2 as analog axes, so trigger bindings use
// a stable half-travel threshold while retaining the raw analog values for
// future pressure-sensitive actions.
func (s ControllerSnapshot) ButtonDown(button ControllerButton) bool {
	if s.Buttons.Has(button) {
		return true
	}
	switch button {
	case ControllerButtonLeftTrigger:
		return s.LeftTrigger >= controllerTriggerButtonThreshold
	case ControllerButtonRightTrigger:
		return s.RightTrigger >= controllerTriggerButtonThreshold
	default:
		return false
	}
}

func (s ControllerSnapshot) Active() bool {
	return s.Connected && (s.Buttons != 0 ||
		math.Hypot(float64(s.LeftX), float64(s.LeftY)) >= 0.01 ||
		math.Hypot(float64(s.RightX), float64(s.RightY)) >= 0.01 ||
		s.LeftTrigger >= 0.01 || s.RightTrigger >= 0.01)
}

// ControllerBackend is implemented by desktop platform adapters. The
// backend is deliberately independent from the game and UI packages so tests
// can provide deterministic snapshots without loading SDL.
type ControllerBackend interface {
	Poll() (ControllerSnapshot, error)
	Close() error
}

// The capability interfaces below are optional. A backend that cannot drive a
// device's haptics simply does not implement them, so the stub backend and test
// fakes need no changes when a new capability is added.

// ControllerRumbler drives the low- and high-frequency rumble motors. Both
// magnitudes are in [0, 1].
type ControllerRumbler interface {
	Rumble(low, high float32, duration time.Duration) error
}

// ControllerLED sets a controller's light bar colour.
type ControllerLED interface {
	SetLED(red, green, blue uint8) error
}

// Direction8 is the normalized eight-way movement vocabulary shared by WASD,
// D-pad, and analog-stick input.
type Direction8 uint8

const (
	DirectionNone Direction8 = iota
	DirectionNorth
	DirectionNorthEast
	DirectionEast
	DirectionSouthEast
	DirectionSouth
	DirectionSouthWest
	DirectionWest
	DirectionNorthWest
)

func (d Direction8) Vector() (int, int) {
	switch d {
	case DirectionNorth:
		return 0, 1
	case DirectionNorthEast:
		return 1, 1
	case DirectionEast:
		return 1, 0
	case DirectionSouthEast:
		return 1, -1
	case DirectionSouth:
		return 0, -1
	case DirectionSouthWest:
		return -1, -1
	case DirectionWest:
		return -1, 0
	case DirectionNorthWest:
		return -1, 1
	default:
		return 0, 0
	}
}

// QuantizeDirection maps a vector to the nearest eight-way direction. The
// angle sectors are centered on the cardinal and diagonal directions and are
// deterministic at their boundaries.
func QuantizeDirection(x, y float32) Direction8 {
	if x == 0 && y == 0 {
		return DirectionNone
	}
	angle := math.Atan2(float64(y), float64(x))
	sector := int(math.Floor((angle + math.Pi/8) / (math.Pi / 4)))
	sector = ((sector % 8) + 8) % 8
	// The sector order starts at east and proceeds counter-clockwise in world
	// coordinates where positive Y is north.
	return [...]Direction8{
		DirectionEast,
		DirectionNorthEast,
		DirectionNorth,
		DirectionNorthWest,
		DirectionWest,
		DirectionSouthWest,
		DirectionSouth,
		DirectionSouthEast,
	}[sector]
}

// ApplyRadialDeadzone removes the inner stick deadzone and rescales the
// remaining radius so full travel still reaches one. Values outside the outer
// deadzone are clamped rather than amplified.
func ApplyRadialDeadzone(x, y, deadzone, outer float32) (float32, float32) {
	deadzone = clamp01(deadzone)
	outer = clamp01(outer)
	if outer <= deadzone {
		outer = minFloat32(1, deadzone+0.01)
	}
	rawRadius := float32(math.Hypot(float64(x), float64(y)))
	if rawRadius <= deadzone {
		return 0, 0
	}
	radius := rawRadius
	if radius > 1 {
		radius = 1
	}
	unitX, unitY := x/rawRadius, y/rawRadius
	rescaled := (radius - deadzone) / (outer - deadzone)
	if rescaled > 1 {
		rescaled = 1
	}
	return unitX * rescaled, unitY * rescaled
}

type Action uint8

const (
	ActionConfirm Action = iota
	ActionCancel
	ActionAttack
	ActionLoot
	ActionTargetPrevious
	ActionTargetNext
	ActionMenu
	ActionMap
	ActionShortcut1
	ActionShortcut2
	ActionShortcut3
	ActionShortcut4
	ActionShortcut5
	ActionShortcut6
	ActionShortcut7
	ActionShortcut8
	// ActionLeftModifier and ActionRightModifier are not dispatched as actions;
	// they exist so the shortcut-layer modifiers are addressable by the same
	// table-driven binding API the rebinding editor uses.
	ActionLeftModifier
	ActionRightModifier
	ActionResetCamera
)

// BindableActions lists every action whose physical button is configurable, in
// the order the rebinding editor presents them. The shortcut-layer actions are
// deliberately absent: they are produced by a modifier plus a face button
// rather than by a button of their own.
func BindableActions() []Action {
	return []Action{
		ActionConfirm,
		ActionCancel,
		ActionAttack,
		ActionLoot,
		ActionTargetPrevious,
		ActionTargetNext,
		ActionMenu,
		ActionMap,
		ActionResetCamera,
		ActionLeftModifier,
		ActionRightModifier,
	}
}

// Name is the stable identifier used for INI keys and rebinding rows.
func (a Action) Name() string {
	switch a {
	case ActionConfirm:
		return "Confirm"
	case ActionCancel:
		return "Cancel"
	case ActionAttack:
		return "Attack"
	case ActionLoot:
		return "Loot"
	case ActionTargetPrevious:
		return "TargetPrevious"
	case ActionTargetNext:
		return "TargetNext"
	case ActionMenu:
		return "Menu"
	case ActionMap:
		return "Map"
	case ActionLeftModifier:
		return "LeftModifier"
	case ActionRightModifier:
		return "RightModifier"
	case ActionResetCamera:
		return "ResetCamera"
	default:
		if a >= ActionShortcut1 && a <= ActionShortcut8 {
			return "Shortcut" + string(rune('1'+int(a-ActionShortcut1)))
		}
		return "Unknown"
	}
}

type ActionSet uint32

func (s ActionSet) Has(action Action) bool {
	if action >= 32 {
		return false
	}
	return s&(1<<action) != 0
}

func (s *ActionSet) Set(action Action, active bool) {
	if s == nil || action >= 32 {
		return
	}
	mask := ActionSet(1 << action)
	if active {
		*s |= mask
	} else {
		*s &^= mask
	}
}

// ActionState is the device-neutral action frame consumed by gameplay and
// controller UI routing.
type ActionState struct {
	Source  InputSource
	Move    Direction8
	CameraX float32
	CameraY float32
	// ZoomDelta is the analog trigger differential, negative for the left
	// trigger and positive for the right. It coexists with the trigger shortcut
	// layers because shortcuts fire on a face-button edge while zoom is a
	// continuous analog reading; the consumer suppresses zoom on any frame a
	// shortcut fires.
	ZoomDelta float32
	Held      ActionSet
	Pressed   ActionSet
	Released  ActionSet
}

// ShortcutPressed reports whether any shortcut-layer action fired this frame.
func (a ActionState) ShortcutPressed() bool {
	for slot := ActionShortcut1; slot <= ActionShortcut8; slot++ {
		if a.Pressed.Has(slot) {
			return true
		}
	}
	return false
}

// UIAction is the small controller-navigation vocabulary shared by every
// desktop screen. The render bridge translates it into the UI framework's
// focus/key events after giving the active overlay first refusal.
type UIAction uint8

const (
	UIActionUp UIAction = iota
	UIActionDown
	UIActionLeft
	UIActionRight
	UIActionConfirm
	UIActionCancel
	UIActionNextFocus
	UIActionPreviousFocus
	UIActionPageUp
	UIActionPageDown
)

type ControllerBindings struct {
	Confirm        ControllerButton
	Cancel         ControllerButton
	Attack         ControllerButton
	Loot           ControllerButton
	TargetPrevious ControllerButton
	TargetNext     ControllerButton
	Menu           ControllerButton
	Map            ControllerButton
	ResetCamera    ControllerButton
	LeftModifier   ControllerButton
	RightModifier  ControllerButton
}

func DefaultControllerBindings() ControllerBindings {
	return ControllerBindings{
		Confirm:        ControllerButtonSouth,
		Cancel:         ControllerButtonEast,
		Attack:         ControllerButtonWest,
		Loot:           ControllerButtonNorth,
		TargetPrevious: ControllerButtonLeftShoulder,
		TargetNext:     ControllerButtonRightShoulder,
		Menu:           ControllerButtonStart,
		Map:            ControllerButtonTouchpad,
		ResetCamera:    ControllerButtonRightStick,
		LeftModifier:   ControllerButtonLeftTrigger,
		RightModifier:  ControllerButtonRightTrigger,
	}
}

// Get returns the button bound to action, or ControllerButtonSouth for an
// action that carries no binding.
func (b ControllerBindings) Get(action Action) ControllerButton {
	switch action {
	case ActionConfirm:
		return b.Confirm
	case ActionCancel:
		return b.Cancel
	case ActionAttack:
		return b.Attack
	case ActionLoot:
		return b.Loot
	case ActionTargetPrevious:
		return b.TargetPrevious
	case ActionTargetNext:
		return b.TargetNext
	case ActionMenu:
		return b.Menu
	case ActionMap:
		return b.Map
	case ActionResetCamera:
		return b.ResetCamera
	case ActionLeftModifier:
		return b.LeftModifier
	case ActionRightModifier:
		return b.RightModifier
	default:
		return ControllerButtonSouth
	}
}

// Conflict reports the action already bound to button, if any.
func (b ControllerBindings) Conflict(button ControllerButton) (Action, bool) {
	for _, action := range BindableActions() {
		if b.Get(action) == button {
			return action, true
		}
	}
	return 0, false
}

// Set binds button to action, clearing any other action that held it. Bindings
// are kept unique so a rebinding cannot silently shadow an existing control;
// the displaced action falls back to its default, and if that default is the
// button being assigned it is left unbound-looking but harmless because the
// caller can always rebind it explicitly.
func (b *ControllerBindings) Set(action Action, button ControllerButton) {
	if b == nil {
		return
	}
	if previous, ok := b.Conflict(button); ok && previous != action {
		defaults := DefaultControllerBindings()
		freed := defaults.Get(previous)
		if freed == button {
			freed = b.Get(action)
		}
		b.assign(previous, freed)
	}
	b.assign(action, button)
}

func (b *ControllerBindings) assign(action Action, button ControllerButton) {
	switch action {
	case ActionConfirm:
		b.Confirm = button
	case ActionCancel:
		b.Cancel = button
	case ActionAttack:
		b.Attack = button
	case ActionLoot:
		b.Loot = button
	case ActionTargetPrevious:
		b.TargetPrevious = button
	case ActionTargetNext:
		b.TargetNext = button
	case ActionMenu:
		b.Menu = button
	case ActionMap:
		b.Map = button
	case ActionResetCamera:
		b.ResetCamera = button
	case ActionLeftModifier:
		b.LeftModifier = button
	case ActionRightModifier:
		b.RightModifier = button
	}
}

// ControllerMoveMode selects what the left stick does in the world.
// ControllerMoveCharacter walks directly; ControllerMoveCursor drives the
// on-screen pointer instead, so walking happens through the same click path the
// mouse uses.
type ControllerMoveMode uint8

const (
	ControllerMoveCharacter ControllerMoveMode = iota
	ControllerMoveCursor
)

func (m ControllerMoveMode) String() string {
	if m == ControllerMoveCursor {
		return "Cursor"
	}
	return "Character"
}

func ParseControllerMoveMode(raw string) (ControllerMoveMode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "character", "walk", "direct":
		return ControllerMoveCharacter, true
	case "cursor", "pointer":
		return ControllerMoveCursor, true
	default:
		return ControllerMoveCharacter, false
	}
}

// ControllerUINavMode selects how menus are driven. ControllerUINavCursor moves
// the pointer over the UI; ControllerUINavFocus hops between focusable widgets.
type ControllerUINavMode uint8

const (
	ControllerUINavCursor ControllerUINavMode = iota
	ControllerUINavFocus
)

func (m ControllerUINavMode) String() string {
	if m == ControllerUINavFocus {
		return "Focus"
	}
	return "Cursor"
}

func ParseControllerUINavMode(raw string) (ControllerUINavMode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "cursor", "pointer":
		return ControllerUINavCursor, true
	case "focus", "focusnav", "focus_nav":
		return ControllerUINavFocus, true
	default:
		return ControllerUINavCursor, false
	}
}

type ControllerSettings struct {
	Enabled           bool
	Deadzone          float32
	OuterDeadzone     float32
	CameraSensitivity float32
	InvertCameraY     bool
	MoveMode          ControllerMoveMode
	UINavMode         ControllerUINavMode
	// CursorSpeed is the virtual cursor travel in pixels per second at full
	// stick deflection, before the response curve.
	CursorSpeed float32
	// TriggerDeadzone is the analog travel below which a trigger reads as idle.
	TriggerDeadzone float32
	// NavRepeatDelayMS is the hold time before focus navigation starts
	// repeating; NavRepeatMS is the interval between repeats afterwards.
	NavRepeatDelayMS int
	NavRepeatMS      int
	Rumble           bool
	Bindings         ControllerBindings
}

func DefaultControllerSettings() ControllerSettings {
	return ControllerSettings{
		Enabled:           true,
		Deadzone:          0.15,
		OuterDeadzone:     0.95,
		CameraSensitivity: 1,
		MoveMode:          ControllerMoveCharacter,
		UINavMode:         ControllerUINavCursor,
		CursorSpeed:       1400,
		TriggerDeadzone:   0.10,
		NavRepeatDelayMS:  350,
		NavRepeatMS:       110,
		Rumble:            true,
		Bindings:          DefaultControllerBindings(),
	}
}

// NavRepeatDelay and NavRepeatRate expose the normalized repeat timings as
// durations for the renderer's navigation repeaters.
func (s ControllerSettings) NavRepeatDelay() time.Duration {
	return time.Duration(s.NavRepeatDelayMS) * time.Millisecond
}

func (s ControllerSettings) NavRepeatRate() time.Duration {
	return time.Duration(s.NavRepeatMS) * time.Millisecond
}

func (s ControllerSettings) Normalized() ControllerSettings {
	defaults := DefaultControllerSettings()
	if s.Deadzone <= 0 || s.Deadzone >= 1 {
		s.Deadzone = defaults.Deadzone
	}
	if s.OuterDeadzone <= s.Deadzone || s.OuterDeadzone > 1 {
		s.OuterDeadzone = defaults.OuterDeadzone
	}
	if s.CameraSensitivity <= 0 {
		s.CameraSensitivity = defaults.CameraSensitivity
	}
	if s.MoveMode > ControllerMoveCursor {
		s.MoveMode = defaults.MoveMode
	}
	if s.UINavMode > ControllerUINavFocus {
		s.UINavMode = defaults.UINavMode
	}
	if s.CursorSpeed < 200 || s.CursorSpeed > 6000 {
		s.CursorSpeed = defaults.CursorSpeed
	}
	if s.TriggerDeadzone < 0 || s.TriggerDeadzone > 0.9 {
		s.TriggerDeadzone = defaults.TriggerDeadzone
	}
	if s.NavRepeatDelayMS < 50 || s.NavRepeatDelayMS > 1000 {
		s.NavRepeatDelayMS = defaults.NavRepeatDelayMS
	}
	if s.NavRepeatMS < 20 || s.NavRepeatMS > 500 {
		s.NavRepeatMS = defaults.NavRepeatMS
	}
	if s.Bindings == (ControllerBindings{}) {
		s.Bindings = defaults.Bindings
	}
	return s
}

func ResolveActions(state *State, settings ControllerSettings) ActionState {
	if state == nil {
		return ActionState{}
	}
	settings = settings.Normalized()
	snapshot := state.Controller()
	if !settings.Enabled {
		// Zero the snapshot rather than returning early: the keyboard WASD
		// arbitration below is the production keyboard walk path and must keep
		// working with the controller turned off.
		snapshot = ControllerSnapshot{}
	}
	actions := ActionState{Source: state.InputSource()}

	keyboardX, keyboardY := float32(0), float32(0)
	keyA, _ := KeyCodeFromName("KeyA")
	keyD, _ := KeyCodeFromName("KeyD")
	keyW, _ := KeyCodeFromName("KeyW")
	keyS, _ := KeyCodeFromName("KeyS")
	if state.KeyCodeDown(keyA) {
		keyboardX--
	}
	if state.KeyCodeDown(keyD) {
		keyboardX++
	}
	if state.KeyCodeDown(keyW) {
		keyboardY++
	}
	if state.KeyCodeDown(keyS) {
		keyboardY--
	}
	keyboardDirection := QuantizeDirection(keyboardX, keyboardY)

	// SDL's standardized Y axes increase downwards; convert them into Goro's
	// world-space convention before quantizing movement.
	stickX, stickY := ApplyRadialDeadzone(snapshot.LeftX, -snapshot.LeftY, settings.Deadzone, settings.OuterDeadzone)
	if stickX != 0 || stickY != 0 {
		actions.Move = QuantizeDirection(stickX, stickY)
		actions.Source = InputSourceController
	} else {
		dpadX, dpadY := float32(0), float32(0)
		if snapshot.Buttons.Has(ControllerButtonDPadLeft) {
			dpadX--
		}
		if snapshot.Buttons.Has(ControllerButtonDPadRight) {
			dpadX++
		}
		if snapshot.Buttons.Has(ControllerButtonDPadUp) {
			dpadY++
		}
		if snapshot.Buttons.Has(ControllerButtonDPadDown) {
			dpadY--
		}
		if dpadX != 0 || dpadY != 0 {
			actions.Move = QuantizeDirection(dpadX, dpadY)
			actions.Source = InputSourceController
		} else {
			actions.Move = keyboardDirection
		}
	}

	rightX, rightY := ApplyRadialDeadzone(snapshot.RightX, snapshot.RightY, settings.Deadzone, settings.OuterDeadzone)
	actions.CameraX = rightX * settings.CameraSensitivity
	actions.CameraY = rightY * settings.CameraSensitivity
	if settings.InvertCameraY {
		actions.CameraY = -actions.CameraY
	}
	if rightX != 0 || rightY != 0 {
		actions.Source = InputSourceController
	}

	leftTrigger, rightTrigger := snapshot.LeftTrigger, snapshot.RightTrigger
	if leftTrigger < settings.TriggerDeadzone {
		leftTrigger = 0
	}
	if rightTrigger < settings.TriggerDeadzone {
		rightTrigger = 0
	}
	actions.ZoomDelta = rightTrigger - leftTrigger
	if actions.ZoomDelta != 0 {
		actions.Source = InputSourceController
	}

	bindings := settings.Bindings
	previous := state.prevController
	if !settings.Enabled {
		previous = ControllerSnapshot{}
	}
	setButtonActions(&actions, previous, snapshot, bindings)
	return actions
}

func setButtonActions(actions *ActionState, previous, snapshot ControllerSnapshot, bindings ControllerBindings) {
	buttonAction := func(action Action, button ControllerButton) {
		down := snapshot.ButtonDown(button)
		wasDown := previous.ButtonDown(button)
		actions.Held.Set(action, down)
		actions.Pressed.Set(action, down && !wasDown)
		actions.Released.Set(action, !down && wasDown)
	}
	buttonAction(ActionConfirm, bindings.Confirm)
	buttonAction(ActionCancel, bindings.Cancel)
	buttonAction(ActionAttack, bindings.Attack)
	buttonAction(ActionLoot, bindings.Loot)
	buttonAction(ActionTargetPrevious, bindings.TargetPrevious)
	buttonAction(ActionTargetNext, bindings.TargetNext)
	buttonAction(ActionMenu, bindings.Menu)
	buttonAction(ActionMap, bindings.Map)
	buttonAction(ActionResetCamera, bindings.ResetCamera)

	leftModifier := snapshot.ButtonDown(bindings.LeftModifier)
	rightModifier := snapshot.ButtonDown(bindings.RightModifier)
	faces := []ControllerButton{ControllerButtonSouth, ControllerButtonEast, ControllerButtonWest, ControllerButtonNorth}
	for i, face := range faces {
		action := ActionShortcut1 + Action(i)
		shortcutDown := leftModifier && snapshot.ButtonDown(face)
		shortcutWasDown := previous.ButtonDown(bindings.LeftModifier) && previous.ButtonDown(face)
		actions.Held.Set(action, shortcutDown)
		actions.Pressed.Set(action, shortcutDown && !shortcutWasDown)
		actions.Released.Set(action, !shortcutDown && shortcutWasDown)
		action += 4
		shortcutDown = rightModifier && snapshot.ButtonDown(face)
		shortcutWasDown = previous.ButtonDown(bindings.RightModifier) && previous.ButtonDown(face)
		actions.Held.Set(action, shortcutDown)
		actions.Pressed.Set(action, shortcutDown && !shortcutWasDown)
		actions.Released.Set(action, !shortcutDown && shortcutWasDown)
	}
}

func clamp01(value float32) float32 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
