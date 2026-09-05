package input

import (
	"math"
	"sort"

	"github.com/gogpu/gpucontext"
)

// KeyCode identifies a physical keyboard position. Its names follow the
// browser KeyboardEvent.code convention (KeyW, Digit1, ArrowUp, and so on).
type KeyCode = gpucontext.Key

type Key int

const (
	KeyEnter Key = iota
	KeyEscape
	KeyTab
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyBackspace
	KeyShift
	KeyCtrl
	KeyAlt
	KeyG
	KeyL
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeyQ
	KeyW
	KeyE
	KeyR
	KeyT
	KeyY
	KeyU
	KeyI
	KeyO
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF12
)

type MouseButton int

const (
	MouseButtonLeft MouseButton = iota
	MouseButtonRight
)

type TouchID int64

type State struct {
	keys                     map[Key]bool
	prev                     map[Key]bool
	justKeys                 map[Key]bool
	keyCodes                 map[KeyCode]bool
	prevKeyCodes             map[KeyCode]bool
	justKeyCodes             map[KeyCode]bool
	justKeyCodeUps           map[KeyCode]bool
	buttons                  map[MouseButton]bool
	prevMouse                map[MouseButton]bool
	justMouse                map[MouseButton]bool
	justMouseReleased        map[MouseButton]bool
	controller               ControllerSnapshot
	prevController           ControllerSnapshot
	consumedActions          ActionSet
	controllerMoveConsumed   bool
	controllerCameraConsumed bool
	controllerZoomConsumed   bool
	pointerSource            InputSource
	source                   InputSource

	MouseX   int
	MouseY   int
	MouseDX  int
	MouseDY  int
	hasMouse bool

	WheelX        float64
	WheelY        float64
	textInput     []rune
	TouchPoints   []TouchPoint
	PinchDelta    float64
	touchIDs      []TouchID
	touches       map[TouchID]TouchPoint
	hasPinch      bool
	pinchDistance float64
}

type TouchPoint struct {
	ID TouchID
	X  int
	Y  int
}

func NewState() *State {
	return &State{
		keys:              make(map[Key]bool),
		prev:              make(map[Key]bool),
		justKeys:          make(map[Key]bool),
		keyCodes:          make(map[KeyCode]bool),
		prevKeyCodes:      make(map[KeyCode]bool),
		justKeyCodes:      make(map[KeyCode]bool),
		justKeyCodeUps:    make(map[KeyCode]bool),
		buttons:           make(map[MouseButton]bool),
		prevMouse:         make(map[MouseButton]bool),
		justMouse:         make(map[MouseButton]bool),
		justMouseReleased: make(map[MouseButton]bool),
		touches:           make(map[TouchID]TouchPoint),
		source:            InputSourceUnknown,
	}
}

func (s *State) Update() {
	s.EndFrame()
}

func (s *State) EndFrame() {
	for key, down := range s.keys {
		s.prev[key] = down
	}
	for key := range s.justKeys {
		delete(s.justKeys, key)
	}
	for code, down := range s.keyCodes {
		s.prevKeyCodes[code] = down
	}
	for code := range s.justKeyCodes {
		delete(s.justKeyCodes, code)
	}
	for code := range s.justKeyCodeUps {
		delete(s.justKeyCodeUps, code)
	}
	for button, down := range s.buttons {
		s.prevMouse[button] = down
	}
	for button := range s.justMouse {
		delete(s.justMouse, button)
	}
	for button := range s.justMouseReleased {
		delete(s.justMouseReleased, button)
	}
	s.prevController = s.controller
	s.consumedActions = 0
	s.controllerMoveConsumed = false
	s.controllerCameraConsumed = false
	s.controllerZoomConsumed = false
	s.MouseDX = 0
	s.MouseDY = 0
	s.WheelX = 0
	s.WheelY = 0
	s.textInput = s.textInput[:0]
	s.updateTouches()
}

func (s *State) SetKey(key Key, pressed bool) {
	if pressed && !s.keys[key] {
		s.justKeys[key] = true
	}
	s.keys[key] = pressed
	if pressed {
		s.source = InputSourceKeyboard
	}
}

func (s *State) SetKeyCode(code KeyCode, pressed bool) {
	if pressed && !s.keyCodes[code] {
		s.justKeyCodes[code] = true
	}
	if !pressed && s.keyCodes[code] {
		s.justKeyCodeUps[code] = true
	}
	s.keyCodes[code] = pressed
	if pressed {
		s.source = InputSourceKeyboard
	}
	if key, ok := legacyKeyForCode(code); ok {
		s.SetKey(key, pressed)
	}
}

// SetPointerSource selects which input source pointer events are attributed
// to. The controller's virtual cursor sets this to InputSourceController while
// it injects events so the pad is not misread as a mouse. The zero value keeps
// the historical behavior of attributing pointer events to the mouse.
func (s *State) SetPointerSource(source InputSource) {
	if s == nil {
		return
	}
	s.pointerSource = source
}

func (s *State) pointerInputSource() InputSource {
	if s.pointerSource == InputSourceUnknown {
		return InputSourceMouse
	}
	return s.pointerSource
}

func (s *State) SetMouseButton(button MouseButton, pressed bool) {
	if pressed && !s.buttons[button] {
		s.justMouse[button] = true
	}
	if !pressed && s.buttons[button] {
		s.justMouseReleased[button] = true
	}
	s.buttons[button] = pressed
	if pressed {
		s.source = s.pointerInputSource()
	}
}

func (s *State) SetMousePosition(x, y int) {
	if s.hasMouse {
		s.MouseDX += x - s.MouseX
		s.MouseDY += y - s.MouseY
	} else {
		s.hasMouse = true
	}
	s.MouseX, s.MouseY = x, y
	s.source = s.pointerInputSource()
}

// SetController publishes the current controller snapshot for the next game
// update. Button edge queries compare this snapshot with the previous frame.
func (s *State) SetController(snapshot ControllerSnapshot) {
	if s == nil {
		return
	}
	s.controller = snapshot
	if snapshot.Active() {
		s.source = InputSourceController
	}
}

func (s *State) Controller() ControllerSnapshot {
	if s == nil {
		return ControllerSnapshot{}
	}
	return s.controller
}

func (s *State) InputSource() InputSource {
	if s == nil {
		return InputSourceUnknown
	}
	return s.source
}

func (s *State) ControllerButtonDown(button ControllerButton) bool {
	return s != nil && s.controller.ButtonDown(button)
}

func (s *State) ControllerButtonJustPressed(button ControllerButton) bool {
	if s == nil {
		return false
	}
	return s.controller.ButtonDown(button) && !s.prevController.ButtonDown(button)
}

func (s *State) ControllerButtonJustReleased(button ControllerButton) bool {
	if s == nil {
		return false
	}
	return !s.controller.ButtonDown(button) && s.prevController.ButtonDown(button)
}

func (s *State) ConsumeControllerAction(action Action) {
	if s != nil {
		s.consumedActions.Set(action, true)
	}
}

func (s *State) ControllerActionConsumed(action Action) bool {
	return s != nil && s.consumedActions.Has(action)
}

// ConsumeControllerCamera marks the right stick as already spent this frame.
// The renderer sets it when the right stick is aiming the virtual pointer, so
// gameplay does not also rotate the camera with the same deflection.
func (s *State) ConsumeControllerCamera() {
	if s != nil {
		s.controllerCameraConsumed = true
	}
}

func (s *State) ControllerCameraConsumed() bool {
	return s != nil && s.controllerCameraConsumed
}

// ConsumeControllerZoom marks the trigger zoom channel as already spent this
// frame. Modal controller UI uses it alongside camera consumption so L2/R2
// paging or shortcut chords cannot also zoom the world behind the window.
func (s *State) ConsumeControllerZoom() {
	if s != nil {
		s.controllerZoomConsumed = true
	}
}

func (s *State) ControllerZoomConsumed() bool {
	return s != nil && s.controllerZoomConsumed
}

func (s *State) ConsumeControllerMovement() {
	if s != nil {
		s.controllerMoveConsumed = true
	}
}

func (s *State) ControllerMovementConsumed() bool {
	return s != nil && s.controllerMoveConsumed
}

func (s *State) AddWheel(x, y float64) {
	s.WheelX += x
	s.WheelY += y
}

func (s *State) AddTextInput(text string) {
	for _, r := range text {
		if r >= 0x20 && r != 0x7f {
			s.textInput = append(s.textInput, r)
		}
	}
}

func (s *State) TextInput() string {
	return string(s.textInput)
}

func (s *State) SetTouch(id TouchID, x, y int, pressed bool) {
	if s == nil {
		return
	}
	if pressed {
		s.touches[id] = TouchPoint{ID: id, X: x, Y: y}
		s.source = InputSourceTouch
		return
	}
	delete(s.touches, id)
	s.source = InputSourceTouch
}

func (s *State) Pressed(key Key) bool {
	return s.keys[key]
}

func (s *State) JustPressed(key Key) bool {
	return s.justKeys[key] || (s.keys[key] && !s.prev[key])
}

func (s *State) KeyCodeDown(code KeyCode) bool {
	return s.keyCodes[code]
}

func (s *State) KeyCodeJustPressed(code KeyCode) bool {
	return s.justKeyCodes[code] || (s.keyCodes[code] && !s.prevKeyCodes[code])
}

func (s *State) KeyCodeJustReleased(code KeyCode) bool {
	return s.justKeyCodeUps[code] || (!s.keyCodes[code] && s.prevKeyCodes[code])
}

// ConsumeKeyCodePress reports a new physical key press once and prevents
// other physical-key consumers from acting on the same edge. Held state and
// layout-translated text input are left unchanged.
func (s *State) ConsumeKeyCodePress(code KeyCode) bool {
	if !s.KeyCodeJustPressed(code) {
		return false
	}
	delete(s.justKeyCodes, code)
	s.prevKeyCodes[code] = s.keyCodes[code]
	if key, ok := legacyKeyForCode(code); ok {
		delete(s.justKeys, key)
		s.prev[key] = s.keys[key]
	}
	return true
}

func (s *State) MousePressed(button MouseButton) bool {
	return s.buttons[button]
}

func (s *State) MouseJustPressed(button MouseButton) bool {
	return s.justMouse[button] || (s.buttons[button] && !s.prevMouse[button])
}

func (s *State) MouseJustReleased(button MouseButton) bool {
	return s.justMouseReleased[button] || (!s.buttons[button] && s.prevMouse[button])
}

func (s *State) updateTouches() {
	s.touchIDs = s.touchIDs[:0]
	for id := range s.touches {
		s.touchIDs = append(s.touchIDs, id)
	}
	sort.Slice(s.touchIDs, func(i, j int) bool {
		return s.touchIDs[i] < s.touchIDs[j]
	})
	s.TouchPoints = s.TouchPoints[:0]
	for _, id := range s.touchIDs {
		s.TouchPoints = append(s.TouchPoints, s.touches[id])
	}
	s.PinchDelta = 0
	if len(s.TouchPoints) < 2 {
		s.hasPinch = false
		s.pinchDistance = 0
		return
	}
	distance := touchDistance(s.TouchPoints[0], s.TouchPoints[1])
	if s.hasPinch {
		s.PinchDelta = distance - s.pinchDistance
	} else {
		s.hasPinch = true
	}
	s.pinchDistance = distance
}

func touchDistance(a, b TouchPoint) float64 {
	return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))
}
