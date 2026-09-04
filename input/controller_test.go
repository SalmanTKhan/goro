package input

import "testing"

func TestApplyRadialDeadzoneNormalizesStickTravel(t *testing.T) {
	x, y := ApplyRadialDeadzone(0.15, 0, 0.15, 0.95)
	if x != 0 || y != 0 {
		t.Fatalf("inner edge = (%f,%f), want zero", x, y)
	}
	x, y = ApplyRadialDeadzone(0.55, 0, 0.15, 0.95)
	if x <= 0 || x >= 1 || y != 0 {
		t.Fatalf("mid travel = (%f,%f), want normalized horizontal value", x, y)
	}
	x, y = ApplyRadialDeadzone(1.2, 0, 0.15, 0.95)
	if x != 1 || y != 0 {
		t.Fatalf("outer travel = (%f,%f), want (1,0)", x, y)
	}
}

func TestQuantizeDirectionUsesWorldSpaceEightWaySectors(t *testing.T) {
	cases := []struct {
		name string
		x, y float32
		want Direction8
	}{
		{"north", 0, 1, DirectionNorth},
		{"northeast", 1, 1, DirectionNorthEast},
		{"east", 1, 0, DirectionEast},
		{"south", 0, -1, DirectionSouth},
		{"southwest", -1, -1, DirectionSouthWest},
		{"west", -1, 0, DirectionWest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuantizeDirection(tc.x, tc.y); got != tc.want {
				t.Fatalf("QuantizeDirection(%v,%v) = %v, want %v", tc.x, tc.y, got, tc.want)
			}
			x, y := tc.want.Vector()
			if x == 0 && y == 0 {
				t.Fatalf("direction %v has no vector", tc.want)
			}
		})
	}
}

func TestControllerButtonEdgesAndHotplugRelease(t *testing.T) {
	state := NewState()
	state.SetController(ControllerSnapshot{
		Connected: true,
		ID:        7,
		Buttons:   ControllerButtons(1 << ControllerButtonSouth),
	})
	if !state.ControllerButtonJustPressed(ControllerButtonSouth) {
		t.Fatal("south press edge was not reported")
	}
	state.EndFrame()
	if state.ControllerButtonJustPressed(ControllerButtonSouth) {
		t.Fatal("south press edge persisted")
	}
	state.SetController(ControllerSnapshot{})
	if !state.ControllerButtonJustReleased(ControllerButtonSouth) {
		t.Fatal("south release on disconnect was not reported")
	}
}

func TestAnalogTriggerBindingsHaveDigitalEdges(t *testing.T) {
	state := NewState()
	state.SetController(ControllerSnapshot{Connected: true, LeftTrigger: 0.8})
	if !state.ControllerButtonJustPressed(ControllerButtonLeftTrigger) {
		t.Fatal("left trigger press edge was not reported")
	}
	state.EndFrame()
	state.SetController(ControllerSnapshot{Connected: true, LeftTrigger: 0.2})
	if !state.ControllerButtonJustReleased(ControllerButtonLeftTrigger) {
		t.Fatal("left trigger release edge was not reported")
	}
}

func TestResolveActionsArbitratesControllerMovementAndBindings(t *testing.T) {
	state := NewState()
	w, _ := KeyCodeFromName("KeyW")
	state.SetKeyCode(w, true)
	actions := ResolveActions(state, DefaultControllerSettings())
	if actions.Move != DirectionNorth || actions.Source != InputSourceKeyboard {
		t.Fatalf("keyboard actions = %#v, want north/keyboard", actions)
	}

	state.EndFrame()
	state.SetController(ControllerSnapshot{
		Connected: true,
		LeftX:     0.8,
		LeftY:     0,
		Buttons:   buttonsWith(ControllerButtonSouth, ControllerButtonRightStick),
	})
	actions = ResolveActions(state, DefaultControllerSettings())
	if actions.Move != DirectionEast || actions.Source != InputSourceController {
		t.Fatalf("controller actions = %#v, want east/controller", actions)
	}
	if !actions.Pressed.Has(ActionConfirm) || !actions.Held.Has(ActionConfirm) {
		t.Fatalf("confirm state = %#v, want held and pressed", actions)
	}
	if !actions.Pressed.Has(ActionResetCamera) || !actions.Held.Has(ActionResetCamera) {
		t.Fatalf("camera reset state = %#v, want held and pressed", actions)
	}

	state.EndFrame()
	state.SetController(ControllerSnapshot{Connected: true, LeftY: -0.8})
	actions = ResolveActions(state, DefaultControllerSettings())
	if actions.Move != DirectionNorth {
		t.Fatalf("negative SDL Y movement = %v, want north", actions.Move)
	}
}

func TestResolveActionsSupportsTriggerShortcutLayers(t *testing.T) {
	state := NewState()
	state.SetController(ControllerSnapshot{
		Connected:    true,
		LeftTrigger:  0.8,
		RightTrigger: 0.8,
		Buttons: ControllerButtons(
			1<<ControllerButtonWest |
				1<<ControllerButtonNorth),
	})
	actions := ResolveActions(state, DefaultControllerSettings())
	if !actions.Pressed.Has(ActionShortcut3) || !actions.Pressed.Has(ActionShortcut8) {
		t.Fatalf("shortcut actions = %#v, want slots 3 and 8", actions.Pressed)
	}
}

func buttonsWith(list ...ControllerButton) ControllerButtons {
	var buttons ControllerButtons
	for _, button := range list {
		buttons.Set(button, true)
	}
	return buttons
}

func TestResolveActionsIgnoresDisabledController(t *testing.T) {
	state := NewState()
	state.SetController(ControllerSnapshot{
		Connected: true,
		Buttons:   buttonsWith(ControllerButtonSouth),
		LeftX:     1,
		RightX:    1,
	})
	keyW, _ := KeyCodeFromName("KeyW")
	state.SetKeyCode(keyW, true)

	settings := DefaultControllerSettings()
	settings.Enabled = false
	actions := ResolveActions(state, settings)

	// The controller contributes nothing...
	if actions.Pressed.Has(ActionConfirm) || actions.Held.Has(ActionConfirm) {
		t.Fatal("disabled controller must not produce button actions")
	}
	if actions.CameraX != 0 || actions.CameraY != 0 {
		t.Fatalf("disabled controller must not produce camera motion: %v,%v", actions.CameraX, actions.CameraY)
	}
	// ...but the keyboard walk path keeps working.
	if actions.Move != DirectionNorth {
		t.Fatalf("keyboard movement = %v want %v", actions.Move, DirectionNorth)
	}
}

func TestControllerBindingsSetIsUnique(t *testing.T) {
	bindings := DefaultControllerBindings()
	// Confirm defaults to South, Cancel to East. Moving Confirm onto East must
	// displace Cancel rather than leaving both on the same button.
	bindings.Set(ActionConfirm, ControllerButtonEast)
	if got := bindings.Get(ActionConfirm); got != ControllerButtonEast {
		t.Fatalf("confirm = %v want east", got)
	}
	if got := bindings.Get(ActionCancel); got == ControllerButtonEast {
		t.Fatal("cancel must be displaced off a reassigned button")
	}
	for _, action := range BindableActions() {
		if other, ok := bindings.Conflict(bindings.Get(action)); ok && other != action {
			// Conflict returns the first action holding the button, so a
			// mismatch here means two actions share one button.
			if bindings.Get(other) == bindings.Get(action) && other != action {
				t.Fatalf("%s and %s share button %v", action.Name(), other.Name(), bindings.Get(action))
			}
		}
	}
}

func TestControllerSettingsNormalizedClamps(t *testing.T) {
	settings := ControllerSettings{Enabled: true, CursorSpeed: 99999, NavRepeatMS: 5, TriggerDeadzone: 4}
	normalized := settings.Normalized()
	defaults := DefaultControllerSettings()
	if normalized.CursorSpeed != defaults.CursorSpeed {
		t.Fatalf("cursor speed = %v want %v", normalized.CursorSpeed, defaults.CursorSpeed)
	}
	if normalized.NavRepeatMS != defaults.NavRepeatMS {
		t.Fatalf("nav repeat = %v want %v", normalized.NavRepeatMS, defaults.NavRepeatMS)
	}
	if normalized.TriggerDeadzone != defaults.TriggerDeadzone {
		t.Fatalf("trigger deadzone = %v want %v", normalized.TriggerDeadzone, defaults.TriggerDeadzone)
	}
}

func TestControllerModeRoundTrip(t *testing.T) {
	for _, raw := range []string{"cursor", "Cursor", "pointer"} {
		if mode, ok := ParseControllerMoveMode(raw); !ok || mode != ControllerMoveCursor {
			t.Fatalf("move mode %q = %v,%v", raw, mode, ok)
		}
	}
	if mode, ok := ParseControllerMoveMode("nonsense"); ok || mode != ControllerMoveCharacter {
		t.Fatalf("invalid move mode = %v,%v", mode, ok)
	}
	if mode, ok := ParseControllerUINavMode("focus_nav"); !ok || mode != ControllerUINavFocus {
		t.Fatalf("ui nav mode = %v,%v", mode, ok)
	}
	if got := ControllerMoveCursor.String(); got != "Cursor" {
		t.Fatalf("move mode string = %q", got)
	}
}
