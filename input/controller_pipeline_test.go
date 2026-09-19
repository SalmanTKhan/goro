package input

import "testing"

func TestControllerCoordinatorEdgesAreStable(t *testing.T) {
	var c ControllerCoordinator
	up := c.Advance(ControllerSnapshot{Connected: true, ID: 1})
	down := c.Advance(ControllerSnapshot{Connected: true, ID: 1, Buttons: ControllerButtons(1 << ControllerButtonSouth)})
	if up.Pressed != 0 || !down.Pressed.Has(ControllerButtonSouth) {
		t.Fatalf("unexpected edge: %#v", down)
	}
	held := c.Advance(down.Current)
	if held.Pressed != 0 || !held.Current.Buttons.Has(ControllerButtonSouth) {
		t.Fatalf("edge repeated: %#v", held)
	}
}

func TestControllerCoordinatorHandoffClearsEdges(t *testing.T) {
	var c ControllerCoordinator
	c.Advance(ControllerSnapshot{Connected: true, ID: 1, Buttons: ControllerButtons(1 << ControllerButtonSouth)})
	f := c.Advance(ControllerSnapshot{Connected: true, ID: 2, Buttons: ControllerButtons(1 << ControllerButtonNorth)})
	if !f.DeviceChanged || f.Pressed.Has(ControllerButtonSouth) || !f.Pressed.Has(ControllerButtonNorth) {
		t.Fatalf("handoff leaked state: %#v", f)
	}
}

func TestControllerCoordinatorResetMakesRecoveryFresh(t *testing.T) {
	var c ControllerCoordinator
	c.Advance(ControllerSnapshot{Connected: true, ID: 1, Buttons: ControllerButtons(1 << ControllerButtonSouth)})
	c.ResetTransient()
	f := c.Advance(ControllerSnapshot{Connected: true, ID: 1, Buttons: ControllerButtons(1 << ControllerButtonSouth)})
	if !f.Pressed.Has(ControllerButtonSouth) || !f.DeviceChanged {
		t.Fatalf("recovery did not start a fresh ownership epoch: %#v", f)
	}
}

func TestControllerCoordinatorCalculatesTriggerEdges(t *testing.T) {
	var c ControllerCoordinator
	c.Advance(ControllerSnapshot{Connected: true, ID: 1})
	f := c.Advance(ControllerSnapshot{Connected: true, ID: 1, LeftTrigger: 0.8})
	if !f.Pressed.Has(ControllerButtonLeftTrigger) {
		t.Fatal("trigger threshold crossing was not published as a frame edge")
	}
	f = c.Advance(ControllerSnapshot{Connected: true, ID: 1, LeftTrigger: 0.2})
	if !f.Released.Has(ControllerButtonLeftTrigger) {
		t.Fatal("trigger threshold release was not published as a frame edge")
	}
}

func TestResolveControllerActionsMovement(t *testing.T) {
	s := DefaultControllerSettings()
	s.UINavMode = ControllerUINavCursor
	for _, tc := range []struct {
		name string
		snap ControllerSnapshot
	}{
		{"dpad", ControllerSnapshot{Connected: true, Buttons: ControllerButtons(1 << ControllerButtonDPadUp)}},
		{"stick", ControllerSnapshot{Connected: true, LeftY: -1}},
	} {
		c := ControllerCoordinator{}
		a := ResolveControllerActions(c.Advance(tc.snap), s)
		if a.Move != DirectionNorth {
			t.Errorf("%s move=%v", tc.name, a.Move)
		}
	}
}

func TestResolveControllerActionsAnalogVectorAndExclusiveShortcut(t *testing.T) {
	s := DefaultControllerSettings()
	c := ControllerCoordinator{}
	c.Advance(ControllerSnapshot{Connected: true, ID: 1})
	f := c.Advance(ControllerSnapshot{Connected: true, ID: 1, LeftX: 0.5, LeftY: -0.5,
		LeftTrigger: 1, Buttons: ControllerButtons(1 << ControllerButtonSouth)})
	a := ResolveControllerActions(f, s)
	if a.MoveX == 0 || a.MoveY == 0 || a.MoveMagnitude <= 0 {
		t.Fatalf("analog vector was lost: %#v", a)
	}
	if a.Pressed.Has(ActionConfirm) || a.Held.Has(ActionConfirm) {
		t.Fatal("modifier face press leaked the base confirm action")
	}
	if !a.Pressed.Has(ActionShortcut1) {
		t.Fatal("LT+South did not resolve to shortcut 1")
	}
	if a.ZoomDelta != 0 {
		t.Fatalf("trigger zoom = %v, want disabled", a.ZoomDelta)
	}
}

func TestControllerSchemesApplyDeterministicPresets(t *testing.T) {
	base := DefaultControllerSettings()
	classic := ApplyControllerScheme(base, ControllerSchemeClassic)
	if classic.MoveMode != ControllerMoveCharacter || classic.UINavMode != ControllerUINavFocus {
		t.Fatalf("classic scheme = %#v", classic)
	}
	twin := ApplyControllerScheme(base, ControllerSchemeTwinStick)
	if twin.MoveMode != ControllerMoveCharacter || twin.UINavMode != ControllerUINavCursor || twin.CameraSensitivity != 1.25 {
		t.Fatalf("twin-stick scheme = %#v", twin)
	}
	parity := ApplyControllerScheme(base, ControllerSchemeKeyboardParity)
	if parity.MoveMode != ControllerMoveCharacter || parity.UINavMode != ControllerUINavFocus || parity.CameraSensitivity != 1 {
		t.Fatalf("keyboard-parity scheme = %#v", parity)
	}
}
