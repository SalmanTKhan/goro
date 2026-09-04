package render

import (
	"fmt"
	"testing"
	"time"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/input"
)

type controllerScopeTestRoot struct {
	widget.WidgetBase
	children []widget.Widget
}

func (r *controllerScopeTestRoot) Children() []widget.Widget { return r.children }

func (r *controllerScopeTestRoot) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.BiggestFinite(1, 1)
	r.SetBounds(geometry.FromPointSize(r.Position(), size))
	return size
}

func (r *controllerScopeTestRoot) Draw(widget.Context, widget.Canvas) {}

func (r *controllerScopeTestRoot) Event(widget.Context, event.Event) bool { return false }

type controllerScopeTestOverlay struct {
	widget.WidgetBase
	passive bool
}

func (o *controllerScopeTestOverlay) Children() []widget.Widget { return nil }

func (o *controllerScopeTestOverlay) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.BiggestFinite(1, 1)
	o.SetBounds(geometry.FromPointSize(o.Position(), size))
	return size
}

func (o *controllerScopeTestOverlay) Draw(widget.Context, widget.Canvas) {}

func (o *controllerScopeTestOverlay) Event(widget.Context, event.Event) bool { return false }

func (o *controllerScopeTestOverlay) ControllerNavigationPassthrough() bool { return o.passive }

func TestControllerFocusScopeSkipsPassiveOverlay(t *testing.T) {
	lower := &controllerScopeTestOverlay{}
	lower.SetVisible(true)
	lower.SetEnabled(true)
	passive := &controllerScopeTestOverlay{passive: true}
	passive.SetVisible(true)
	passive.SetEnabled(true)
	root := &controllerScopeTestRoot{children: []widget.Widget{lower, passive}}
	root.SetVisible(true)
	root.SetEnabled(true)

	if got := controllerFocusScope(root); got != lower {
		t.Fatalf("controller focus scope = %T, want interactive overlay below passive HUD", got)
	}
}

func TestNavRepeaterFiresOnceThenRepeats(t *testing.T) {
	var repeater navRepeater
	start := time.Now()
	delay, rate := 350*time.Millisecond, 110*time.Millisecond

	if !repeater.fire(input.UIActionDown, start, delay, rate) {
		t.Fatal("first press must fire immediately")
	}
	if repeater.fire(input.UIActionDown, start.Add(200*time.Millisecond), delay, rate) {
		t.Fatal("held action fired before the initial delay elapsed")
	}
	if !repeater.fire(input.UIActionDown, start.Add(360*time.Millisecond), delay, rate) {
		t.Fatal("held action did not repeat after the initial delay")
	}
	if repeater.fire(input.UIActionDown, start.Add(420*time.Millisecond), delay, rate) {
		t.Fatal("held action repeated faster than the repeat rate")
	}
	if !repeater.fire(input.UIActionDown, start.Add(480*time.Millisecond), delay, rate) {
		t.Fatal("held action did not repeat at the repeat rate")
	}
}

func TestNavRepeaterDistinguishesUpFromIdle(t *testing.T) {
	// UIActionUp is the zero value of input.UIAction. A repeater that used the
	// zero value as its "nothing held" sentinel could not tell a held Up from
	// an idle stick, and would re-fire Up on every frame.
	var repeater navRepeater
	start := time.Now()
	delay, rate := 350*time.Millisecond, 110*time.Millisecond

	if !repeater.fire(input.UIActionUp, start, delay, rate) {
		t.Fatal("first Up must fire")
	}
	if repeater.fire(input.UIActionUp, start.Add(10*time.Millisecond), delay, rate) {
		t.Fatal("held Up must not re-fire immediately")
	}
	repeater.reset()
	if !repeater.fire(input.UIActionUp, start.Add(20*time.Millisecond), delay, rate) {
		t.Fatal("Up must fire again after a reset")
	}
}

func TestControllerCursorAxesRespectMoveMode(t *testing.T) {
	settings := input.DefaultControllerSettings().Normalized()
	snapshot := input.ControllerSnapshot{Connected: true, LeftX: 1, RightY: 1}

	// Cursor move mode: the left stick always aims, so it can never also walk.
	settings.MoveMode = input.ControllerMoveCursor
	x, y, blocks := controllerCursorAxes(snapshot, settings, false)
	if x == 0 || y != 0 || !blocks {
		t.Fatalf("cursor mode over world = %v,%v blocks=%v", x, y, blocks)
	}

	// Character move mode over the world: the left stick walks and the right
	// stick aims, so movement is not blocked.
	settings.MoveMode = input.ControllerMoveCharacter
	x, y, blocks = controllerCursorAxes(snapshot, settings, false)
	if x != 0 || y == 0 || blocks {
		t.Fatalf("character mode over world = %v,%v blocks=%v", x, y, blocks)
	}

	// Character move mode over the UI: the left stick aims at buttons instead
	// of walking the player into them.
	x, y, blocks = controllerCursorAxes(snapshot, settings, true)
	if x == 0 || y != 0 || !blocks {
		t.Fatalf("character mode over UI = %v,%v blocks=%v", x, y, blocks)
	}
}

// pointerRecorder captures the synthetic pointer events a fanout emits.
type pointerRecorder struct {
	events []string
}

func newRecordedFanout() (*fanoutEventSource, *pointerRecorder) {
	fanout := &fanoutEventSource{}
	recorder := &pointerRecorder{}
	fanout.OnMouseMove(func(x, y float64) {
		recorder.events = append(recorder.events, fmt.Sprintf("move %.0f,%.0f", x, y))
	})
	fanout.OnMousePress(func(_ gpucontext.MouseButton, x, y float64) {
		recorder.events = append(recorder.events, fmt.Sprintf("press %.0f,%.0f", x, y))
	})
	fanout.OnMouseRelease(func(_ gpucontext.MouseButton, x, y float64) {
		recorder.events = append(recorder.events, fmt.Sprintf("release %.0f,%.0f", x, y))
	})
	fanout.OnScroll(func(x, y float64) {
		recorder.events = append(recorder.events, "scroll")
	})
	return fanout, recorder
}

// pointerTestGame is the minimal Game surface dispatchControllerPointer needs.
type pointerTestGame struct {
	state *input.State
}

func (g *pointerTestGame) Update() error            { return nil }
func (g *pointerTestGame) Draw(*Frame)              {}
func (g *pointerTestGame) Resize(int, int)          {}
func (g *pointerTestGame) InputState() *input.State { return g.state }
func (g *pointerTestGame) ControllerSettings() input.ControllerSettings {
	return input.DefaultControllerSettings()
}

func newPointerTestRunner() (*runner, *pointerRecorder) {
	fanout, recorder := newRecordedFanout()
	state := input.NewState()
	r := &runner{
		width:              800,
		height:             600,
		events:             fanout,
		game:               &pointerTestGame{state: state},
		controllerSettings: input.DefaultControllerSettings().Normalized(),
	}
	r.cursor.Reset(800, 600)
	// Wire the shared input state into the same fanout the real runner uses, so
	// injected events reach input.State exactly as they do in production.
	wireInput(fanout, state, func() bool { return r.injectingPointer })
	return r, recorder
}

func TestDispatchControllerPointerEmitsOneMovePerPixelChange(t *testing.T) {
	r, recorder := newPointerTestRunner()
	settings := r.controllerSettings
	settings.MoveMode = input.ControllerMoveCursor
	snapshot := input.ControllerSnapshot{Connected: true, LeftX: 1}
	state := r.game.InputState()

	// A frame short enough to move less than a pixel emits nothing.
	r.dispatchControllerPointer(state, snapshot, settings, time.Microsecond)
	if len(recorder.events) != 0 {
		t.Fatalf("sub-pixel frame emitted %v", recorder.events)
	}
	// A normal frame emits exactly one move.
	r.dispatchControllerPointer(state, snapshot, settings, 16*time.Millisecond)
	if len(recorder.events) != 1 {
		t.Fatalf("expected one move, got %v", recorder.events)
	}
	// An idle stick emits nothing at all.
	before := len(recorder.events)
	r.dispatchControllerPointer(state, input.ControllerSnapshot{Connected: true}, settings, 16*time.Millisecond)
	if len(recorder.events) != before {
		t.Fatalf("idle stick emitted %v", recorder.events[before:])
	}
}

func TestDispatchControllerPointerConfirmProducesOnePressAndRelease(t *testing.T) {
	r, recorder := newPointerTestRunner()
	settings := r.controllerSettings
	settings.MoveMode = input.ControllerMoveCursor
	state := r.game.InputState()

	held := input.ControllerSnapshot{Connected: true}
	held.Buttons.Set(settings.Bindings.Confirm, true)

	// Holding confirm across several frames must press exactly once.
	for i := 0; i < 3; i++ {
		r.dispatchControllerPointer(state, held, settings, 16*time.Millisecond)
	}
	presses := countEvents(recorder.events, "press")
	if presses != 1 {
		t.Fatalf("expected one press, got %d in %v", presses, recorder.events)
	}
	if !state.ControllerActionConsumed(input.ActionConfirm) {
		t.Fatal("a pointer click must consume the confirm action for gameplay")
	}

	r.dispatchControllerPointer(state, input.ControllerSnapshot{Connected: true}, settings, 16*time.Millisecond)
	if releases := countEvents(recorder.events, "release"); releases != 1 {
		t.Fatalf("expected one release, got %d in %v", releases, recorder.events)
	}
}

func TestCharacterModeConfirmDoesNotSynthesizeWorldClick(t *testing.T) {
	r, recorder := newPointerTestRunner()
	settings := r.controllerSettings
	settings.MoveMode = input.ControllerMoveCharacter
	state := r.game.InputState()
	held := input.ControllerSnapshot{Connected: true}
	held.Buttons.Set(settings.Bindings.Confirm, true)

	r.dispatchControllerPointer(state, held, settings, 16*time.Millisecond)
	if len(recorder.events) != 0 {
		t.Fatalf("character-mode confirm emitted pointer events: %v", recorder.events)
	}
	if state.ControllerActionConsumed(input.ActionConfirm) {
		t.Fatal("character-mode confirm was consumed by the pointer compatibility path")
	}
}

func TestReleaseControllerPointerReleasesHeldButton(t *testing.T) {
	r, recorder := newPointerTestRunner()
	settings := r.controllerSettings
	settings.MoveMode = input.ControllerMoveCursor
	state := r.game.InputState()

	held := input.ControllerSnapshot{Connected: true}
	held.Buttons.Set(settings.Bindings.Confirm, true)
	r.dispatchControllerPointer(state, held, settings, 16*time.Millisecond)
	if countEvents(recorder.events, "press") != 1 {
		t.Fatalf("setup did not press: %v", recorder.events)
	}

	// Disconnecting or losing focus mid-hold must not leave the button down.
	r.controllerConnected = true
	r.handleControllerDisconnect()
	if countEvents(recorder.events, "release") != 1 {
		t.Fatalf("disconnect did not release the pointer: %v", recorder.events)
	}
	if r.cursorLeftDown {
		t.Fatal("pointer button still marked down after disconnect")
	}
}

func TestInjectPointerKeepsControllerAsInputSource(t *testing.T) {
	r, _ := newPointerTestRunner()
	state := r.game.InputState()
	state.SetMousePosition(10, 10)
	if state.InputSource() != input.InputSourceMouse {
		t.Fatalf("real mouse source = %v", state.InputSource())
	}
	r.injectPointer(func() { r.events.EmitMouseMove(100, 100) })
	if state.InputSource() != input.InputSourceController {
		t.Fatalf("injected pointer source = %v want controller", state.InputSource())
	}
	if r.injectingPointer {
		t.Fatal("injection flag leaked past the emit")
	}
}

func countEvents(events []string, prefix string) int {
	count := 0
	for _, event := range events {
		if len(event) >= len(prefix) && event[:len(prefix)] == prefix {
			count++
		}
	}
	return count
}

func TestCharacterModeRightStickAimingConsumesCamera(t *testing.T) {
	// In character move mode the right stick aims the pointer, so it must not
	// also rotate the camera -- otherwise one deflection does two jobs.
	r, _ := newPointerTestRunner()
	settings := r.controllerSettings
	settings.MoveMode = input.ControllerMoveCharacter
	state := r.game.InputState()
	snapshot := input.ControllerSnapshot{Connected: true, RightX: 1}

	r.dispatchControllerPointer(state, snapshot, settings, 16*time.Millisecond)
	if !state.ControllerCameraConsumed() {
		t.Fatal("right-stick aiming did not consume camera rotation")
	}
	if state.ControllerMovementConsumed() {
		t.Fatal("character mode over the world must leave the left stick free to walk")
	}
}

func TestCursorModeLeavesCameraToGameplay(t *testing.T) {
	r, _ := newPointerTestRunner()
	settings := r.controllerSettings
	settings.MoveMode = input.ControllerMoveCursor
	state := r.game.InputState()
	snapshot := input.ControllerSnapshot{Connected: true, LeftX: 1, RightX: 1}

	r.dispatchControllerPointer(state, snapshot, settings, 16*time.Millisecond)
	if state.ControllerCameraConsumed() {
		t.Fatal("cursor mode must leave the right stick to camera rotation")
	}
	if !state.ControllerMovementConsumed() {
		t.Fatal("cursor mode must consume left-stick movement")
	}
}
