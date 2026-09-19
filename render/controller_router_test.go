package render

import (
	"github.com/kivutar/goro/input"
	"testing"
)

func TestRouteControllerMovementOwnership(t *testing.T) {
	a := input.ActionState{Move: input.DirectionNorth}
	got := RouteController(a, input.ControllerFrame{}, ControllerRouteContext{NavigationMode: input.ControllerUINavCursor}, 0, 0)
	if got.Gameplay.Move != input.DirectionNorth || len(got.UI.Actions) != 0 {
		t.Fatalf("cursor route=%#v", got)
	}
	got = RouteController(a, input.ControllerFrame{}, ControllerRouteContext{NavigationMode: input.ControllerUINavFocus, FocusNavigationActive: true}, 0, 0)
	if got.Gameplay.Move != input.DirectionNone || len(got.UI.Actions) != 1 || got.UI.Actions[0] != input.UIActionUp {
		t.Fatalf("focus route=%#v", got)
	}
	got = RouteController(a, input.ControllerFrame{}, ControllerRouteContext{NavigationMode: input.ControllerUINavFocus}, 0, 0)
	if got.Gameplay.Move != input.DirectionNorth || len(got.UI.Actions) != 0 {
		t.Fatalf("inactive focus route=%#v", got)
	}
}

func TestRouteControllerConfirmHasOneOwner(t *testing.T) {
	a := input.ActionState{}
	a.Pressed.Set(input.ActionConfirm, true)
	for _, tc := range []struct {
		name              string
		ctx               ControllerRouteContext
		pointer, gameplay bool
		ui                int
	}{
		{"cursor", ControllerRouteContext{NavigationMode: input.ControllerUINavCursor, PointerOverUI: true}, false, false, 0},
		{"focus", ControllerRouteContext{NavigationMode: input.ControllerUINavFocus, FocusNavigationActive: true}, false, false, 1},
		{"gameplay", ControllerRouteContext{}, false, true, 0},
	} {
		got := RouteController(a, input.ControllerFrame{}, tc.ctx, 0, 0)
		if tc.name == "cursor" {
			if !got.UI.PointerClick || got.Gameplay.Pressed.Has(input.ActionConfirm) {
				t.Fatalf("%s route=%#v", tc.name, got)
			}
		} else if tc.name == "focus" {
			if len(got.UI.Actions) != tc.ui || got.Gameplay.Pressed.Has(input.ActionConfirm) {
				t.Fatalf("%s route=%#v", tc.name, got)
			}
		} else if !got.Gameplay.Pressed.Has(input.ActionConfirm) {
			t.Fatalf("%s route=%#v", tc.name, got)
		}
	}
}

func TestRouteControllerPointerOwnsConfirmEdges(t *testing.T) {
	a := input.ActionState{}
	a.Held.Set(input.ActionConfirm, true)
	a.Pressed.Set(input.ActionConfirm, true)
	got := RouteController(a, input.ControllerFrame{}, ControllerRouteContext{
		NavigationMode: input.ControllerUINavCursor, PointerOverUI: true,
	}, 0.5, -0.25)
	if got.Pointer.MoveX != 0.5 || got.Pointer.MoveY != -0.25 || !got.Pointer.LeftDown || !got.Pointer.LeftPressed || got.Gameplay.Pressed.Has(input.ActionConfirm) {
		t.Fatalf("pointer press route=%#v", got)
	}
	a = input.ActionState{}
	a.Released.Set(input.ActionConfirm, true)
	got = RouteController(a, input.ControllerFrame{}, ControllerRouteContext{
		NavigationMode: input.ControllerUINavCursor, PointerOverUI: true,
	}, 0, 0)
	if !got.Pointer.LeftReleased || got.Gameplay.Pressed.Has(input.ActionConfirm) {
		t.Fatalf("pointer release route=%#v", got)
	}
}

func TestRouteControllerPointerOwnershipSuppressesCamera(t *testing.T) {
	a := input.ActionState{CameraX: 1, CameraY: -1}
	got := RouteController(a, input.ControllerFrame{}, ControllerRouteContext{PointerOverUI: true}, 0.25, 0.5)
	if got.Gameplay.CameraX != 0 || got.Gameplay.CameraY != 0 || got.Pointer.MoveX != 0.25 || got.Pointer.MoveY != 0.5 {
		t.Fatalf("pointer did not exclusively own right-stick sample: %#v", got)
	}
}

func TestRouteControllerSkillTargetingKeepsRightStickForWorld(t *testing.T) {
	a := input.ActionState{CameraX: 0.8, CameraY: -0.4}
	got := RouteController(a, input.ControllerFrame{}, ControllerRouteContext{
		PointerOverUI:        true,
		SkillTargetingActive: true,
	}, 0, 0)
	if got.Gameplay.CameraX != a.CameraX || got.Gameplay.CameraY != a.CameraY {
		t.Fatalf("skill targeting lost right-stick vector: %#v", got.Gameplay)
	}
}

func TestRouteControllerCaptureBlocksGameplay(t *testing.T) {
	a := input.ActionState{Move: input.DirectionNorth, CameraX: 1, ZoomDelta: 1}
	a.Pressed.Set(input.ActionConfirm, true)
	for _, ctx := range []ControllerRouteContext{
		{ModalUIActive: true}, {TextInputActive: true}, {RebindActive: true},
	} {
		got := RouteController(a, input.ControllerFrame{}, ctx, 0, 0)
		if got.Gameplay.Move != input.DirectionNone || got.Gameplay.CameraX != 0 || got.Gameplay.ZoomDelta != 0 || got.Gameplay.Pressed != 0 {
			t.Fatalf("captured gameplay leaked: %#v", got)
		}
	}
}
