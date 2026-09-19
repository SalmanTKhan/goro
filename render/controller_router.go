package render

import "github.com/kivutar/goro/input"

// ControllerRouteContext is explicit UI ownership state. Presence of a HUD
// or bridge is intentionally not part of this decision.
type ControllerRouteContext struct {
	NavigationMode        input.ControllerUINavMode
	PointerActive         bool
	PointerOverUI         bool
	FocusNavigationActive bool
	ModalUIActive         bool
	TextInputActive       bool
	RebindActive          bool
	// SkillTargetingActive gives the world the right-stick vector even when a
	// UI pointer was previously active. While a skill is pending the stick is
	// an explicit world-targeting control, never a mouse/cursor control.
	SkillTargetingActive bool
}

type ControllerGameplayIntent struct {
	Move                        input.Direction8
	CameraX, CameraY, ZoomDelta float32
	Held, Pressed, Released     input.ActionSet
}
type ControllerPointerIntent struct {
	MoveX, MoveY                        float32
	LeftDown, LeftPressed, LeftReleased bool
}
type ControllerUIIntent struct {
	Actions      []input.UIAction
	PointerClick bool
}
type ControllerDispatch struct {
	Gameplay ControllerGameplayIntent
	Pointer  ControllerPointerIntent
	UI       ControllerUIIntent
}

// Route decides ownership before dispatch. It never calls UI or gameplay and
// therefore cannot create a second interpretation of an edge.
func RouteController(actions input.ActionState, frame input.ControllerFrame, ctx ControllerRouteContext, pointerAxesX, pointerAxesY float32) ControllerDispatch {
	d := ControllerDispatch{Pointer: ControllerPointerIntent{MoveX: pointerAxesX, MoveY: pointerAxesY}}
	d.Gameplay = ControllerGameplayIntent{Move: actions.Move, CameraX: actions.CameraX, CameraY: actions.CameraY, ZoomDelta: actions.ZoomDelta, Held: actions.Held, Pressed: actions.Pressed, Released: actions.Released}
	if ctx.ModalUIActive || ctx.TextInputActive || ctx.RebindActive {
		// Higher-level UI capture is decided before dispatch; no gameplay
		// intent may leak behind a modal, text editor, or rebinding capture.
		d.Gameplay = ControllerGameplayIntent{}
		return d
	}
	if ctx.PointerActive {
		if actions.Pressed.Has(input.ActionMap) {
			d.Pointer.LeftDown, d.Pointer.LeftPressed = true, true
			d.Gameplay.Pressed &^= 1 << input.ActionMap
		}
		if actions.Released.Has(input.ActionMap) {
			d.Pointer.LeftReleased = true
			d.Gameplay.Released &^= 1 << input.ActionMap
		}
	}
	// The right stick is the virtual pointer while it owns UI hit testing; do
	// not also interpret that same sample as world camera motion.
	if ctx.PointerOverUI && !ctx.SkillTargetingActive {
		d.Gameplay.CameraX, d.Gameplay.CameraY = 0, 0
	}
	// A pending skill is a world interaction and takes priority over any
	// passive/focused HUD scope. Confirm, cancel, and LB/RB must reach the
	// targeting state rather than being translated into menu events.
	focus := ctx.NavigationMode == input.ControllerUINavFocus && ctx.FocusNavigationActive && !ctx.SkillTargetingActive
	if focus {
		d.Gameplay.Move = input.DirectionNone
		if actions.Move != input.DirectionNone {
			d.UI.Actions = append(d.UI.Actions, directionUIAction(actions.Move))
		}
	}
	if actions.Pressed.Has(input.ActionConfirm) {
		switch {
		case ctx.NavigationMode == input.ControllerUINavCursor && ctx.PointerOverUI:
			d.UI.PointerClick = true
			d.Pointer.LeftDown, d.Pointer.LeftPressed = true, true
			d.Gameplay.Pressed &^= 1 << input.ActionConfirm
		case focus:
			d.UI.Actions = append(d.UI.Actions, input.UIActionConfirm)
			d.Gameplay.Pressed &^= 1 << input.ActionConfirm
		}
	}
	if ctx.NavigationMode == input.ControllerUINavCursor && ctx.PointerOverUI && actions.Released.Has(input.ActionConfirm) {
		d.Pointer.LeftReleased = true
	}
	if ctx.NavigationMode == input.ControllerUINavCursor && ctx.PointerOverUI {
		d.Pointer.LeftDown = actions.Held.Has(input.ActionConfirm)
	}
	if focus {
		for _, item := range []struct {
			action input.Action
			ui     input.UIAction
		}{
			{input.ActionCancel, input.UIActionCancel},
			{input.ActionAttack, input.UIActionContext},
			{input.ActionLoot, input.UIActionSecondary},
			{input.ActionTargetPrevious, input.UIActionPreviousFocus},
			{input.ActionTargetNext, input.UIActionNextFocus},
			{input.ActionMenu, input.UIActionCancel},
		} {
			if actions.Pressed.Has(item.action) {
				d.UI.Actions = append(d.UI.Actions, item.ui)
				d.Gameplay.Pressed &^= 1 << item.action
			}
		}
	}
	if focus {
		d.Gameplay.Pressed &^= 1 << input.ActionConfirm
	}
	_ = frame
	return d
}

func directionUIAction(d input.Direction8) input.UIAction {
	switch d {
	case input.DirectionNorth:
		return input.UIActionUp
	case input.DirectionSouth:
		return input.UIActionDown
	case input.DirectionWest:
		return input.UIActionLeft
	default:
		return input.UIActionRight
	}
}
