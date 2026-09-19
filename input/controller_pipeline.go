package input

import "math"

// ControllerFrame is the immutable controller sample for one application
// update. Only ControllerCoordinator advances temporal state.
type ControllerFrame struct {
	Sequence      uint64
	Connected     bool
	DeviceID      uint64
	Current       ControllerSnapshot
	Previous      ControllerSnapshot
	Pressed       ControllerButtons
	Released      ControllerButtons
	DeviceChanged bool
}

type ControllerCoordinator struct {
	sequence    uint64
	previous    ControllerSnapshot
	deviceID    uint64
	initialized bool
}

// ResetTransient ends the current physical ownership epoch. The next
// connected sample is treated as fresh input, preventing held movement or
// button state from surviving focus loss/disconnect recovery.
func (c *ControllerCoordinator) ResetTransient() {
	if c == nil {
		return
	}
	c.previous = ControllerSnapshot{}
	c.deviceID = 0
	c.initialized = true
}

func (c *ControllerCoordinator) Advance(current ControllerSnapshot) ControllerFrame {
	if c == nil {
		return ControllerFrame{}
	}
	previous := c.previous
	changed := c.initialized && (current.ID != c.deviceID || (!previous.Connected && current.Connected))
	if !current.Connected || changed {
		// A new device starts with no inherited edges or held state.
		previous = ControllerSnapshot{}
	}
	c.sequence++
	currentButtons := controllerDigitalButtons(current)
	previousButtons := controllerDigitalButtons(previous)
	frame := ControllerFrame{Sequence: c.sequence, Connected: current.Connected, DeviceID: current.ID,
		Current: current, Previous: previous, Pressed: currentButtons &^ previousButtons,
		Released: previousButtons &^ currentButtons, DeviceChanged: changed}
	c.previous, c.deviceID, c.initialized = current, current.ID, true
	return frame
}

func controllerDigitalButtons(snapshot ControllerSnapshot) ControllerButtons {
	buttons := snapshot.Buttons
	buttons.Set(ControllerButtonLeftTrigger, snapshot.LeftTrigger >= controllerTriggerButtonThreshold)
	buttons.Set(ControllerButtonRightTrigger, snapshot.RightTrigger >= controllerTriggerButtonThreshold)
	return buttons
}

// ResolveControllerActions is the controller-only resolver. It has no State
// access and therefore cannot observe or regenerate edges through another
// consumer.
func ResolveControllerActions(frame ControllerFrame, settings ControllerSettings) ActionState {
	settings = settings.Normalized()
	if !settings.Enabled || !frame.Connected {
		return ActionState{Source: InputSourceController}
	}
	a := ActionState{Source: InputSourceController}
	x, y := ApplyRadialDeadzone(frame.Current.LeftX, -frame.Current.LeftY, settings.Deadzone, settings.OuterDeadzone)
	if x != 0 || y != 0 {
		a.Move = QuantizeDirection(x, y)
		a.MoveX, a.MoveY = x, y
		a.MoveMagnitude = float32(math.Hypot(float64(x), float64(y)))
	} else {
		dx, dy := float32(0), float32(0)
		if frame.Current.Buttons.Has(ControllerButtonDPadLeft) {
			dx--
		}
		if frame.Current.Buttons.Has(ControllerButtonDPadRight) {
			dx++
		}
		if frame.Current.Buttons.Has(ControllerButtonDPadUp) {
			dy++
		}
		if frame.Current.Buttons.Has(ControllerButtonDPadDown) {
			dy--
		}
		a.Move = QuantizeDirection(dx, dy)
		if dx != 0 || dy != 0 {
			a.MoveX, a.MoveY = dx, dy
			a.MoveMagnitude = 1
		}
	}
	rx, ry := ApplyRadialDeadzone(frame.Current.RightX, frame.Current.RightY, settings.Deadzone, settings.OuterDeadzone)
	a.CameraX, a.CameraY = rx*settings.CameraSensitivity, ry*settings.CameraSensitivity
	if settings.InvertCameraY {
		a.CameraY = -a.CameraY
	}
	setButtonActionsFrame(&a, frame, settings.Bindings)
	return a
}

func setButtonActionsFrame(a *ActionState, f ControllerFrame, b ControllerBindings) {
	currentButtons := controllerDigitalButtons(f.Current)
	previousButtons := controllerDigitalButtons(f.Previous)
	button := func(action Action, physical ControllerButton) {
		down, was := currentButtons.Has(physical), previousButtons.Has(physical)
		a.Held.Set(action, down)
		a.Pressed.Set(action, down && !was)
		a.Released.Set(action, !down && was)
	}
	button(ActionConfirm, b.Confirm)
	button(ActionCancel, b.Cancel)
	button(ActionAttack, b.Attack)
	button(ActionLoot, b.Loot)
	button(ActionTargetPrevious, b.TargetPrevious)
	button(ActionTargetNext, b.TargetNext)
	button(ActionMenu, b.Menu)
	button(ActionMap, b.Map)
	button(ActionResetCamera, b.ResetCamera)
	button(ActionGameMenu, b.GameMenu)
	button(ActionSit, b.Sit)
	// D-pad lane actions are semantic edges. They are ignored by gameplay
	// during ordinary movement and consumed by pending skill targeting.
	buttonDPad := func(action Action, physical ControllerButton) {
		down, was := currentButtons.Has(physical), previousButtons.Has(physical)
		a.Pressed.Set(action, down && !was)
	}
	buttonDPad(ActionTargetSelf, ControllerButtonDPadUp)
	buttonDPad(ActionTargetAlly, ControllerButtonDPadLeft)
	buttonDPad(ActionTargetEnemy, ControllerButtonDPadRight)
	buttonDPad(ActionTargetCompanion, ControllerButtonDPadDown)
	leftModifier := currentButtons.Has(b.LeftModifier)
	rightModifier := currentButtons.Has(b.RightModifier)
	if leftModifier || rightModifier {
		// A modifier layer is exclusive: the same face press cannot also be
		// interpreted as Confirm/Cancel/Attack/Loot.
		for _, physical := range []ControllerButton{b.Confirm, b.Cancel, b.Attack, b.Loot} {
			for _, action := range []Action{ActionConfirm, ActionCancel, ActionAttack, ActionLoot} {
				if b.Get(action) == physical {
					a.Held &^= 1 << action
					a.Pressed &^= 1 << action
					a.Released &^= 1 << action
				}
			}
		}
	}
	faces := []ControllerButton{ControllerButtonSouth, ControllerButtonEast, ControllerButtonWest, ControllerButtonNorth}
	for i, face := range faces {
		action := ActionShortcut1 + Action(i)
		down := leftModifier && currentButtons.Has(face)
		was := previousButtons.Has(b.LeftModifier) && previousButtons.Has(face)
		a.Held.Set(action, down)
		a.Pressed.Set(action, down && !was)
		a.Released.Set(action, !down && was)
		action += 4
		down = rightModifier && currentButtons.Has(face)
		was = previousButtons.Has(b.RightModifier) && previousButtons.Has(face)
		a.Held.Set(action, down)
		a.Pressed.Set(action, down && !was)
		a.Released.Set(action, !down && was)
	}
}
