package ui

import (
	"fmt"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
)

// controllerPromptFamily is presentation-only. Gameplay continues to use
// positional ControllerButton values and semantic Action bindings.
type controllerPromptFamily uint8

const (
	controllerPromptGeneric controllerPromptFamily = iota
	controllerPromptXbox
	controllerPromptPlayStation
	controllerPromptSwitch
	controllerPromptSteamDeck
)

func controllerPromptFamilyForKind(kind input.ControllerKind) controllerPromptFamily {
	switch kind {
	case input.ControllerKindXbox:
		return controllerPromptXbox
	case input.ControllerKindPlayStation4, input.ControllerKindPlayStation5:
		return controllerPromptPlayStation
	case input.ControllerKindSwitch:
		return controllerPromptSwitch
	case input.ControllerKindSteamDeck:
		return controllerPromptSteamDeck
	default:
		return controllerPromptGeneric
	}
}

func controllerPromptsVisible(ctx client.Context) bool {
	if ctx.Input == nil {
		return false
	}
	snapshot := ctx.Input.Controller()
	return snapshot.Connected && ctx.Input.InputSource() == input.InputSourceController
}

func controllerPromptForAction(ctx client.Context, action input.Action) string {
	if ctx.Input == nil {
		return ""
	}
	snapshot := ctx.Input.Controller()
	if !snapshot.Connected {
		return ""
	}
	return controllerPromptButtonCompact(snapshot.Kind, ctx.ControllerSettings().Bindings.Get(action))
}

// controllerShortcutPrompt resolves the eight recommended controller skill
// slots from the same modifier+face ordering used by ResolveControllerActions.
func controllerShortcutPrompt(ctx client.Context, slot int) string {
	if !controllerPromptsVisible(ctx) || ctx.Input == nil {
		return ""
	}
	snapshot := ctx.Input.Controller()
	return controllerShortcutPromptFor(snapshot.Kind, ctx.ControllerSettings().Bindings, slot)
}

func controllerShortcutPromptFor(kind input.ControllerKind, bindings input.ControllerBindings, slot int) string {
	if slot < 0 || slot >= 8 {
		return ""
	}
	faces := [...]input.ControllerButton{
		input.ControllerButtonSouth,
		input.ControllerButtonEast,
		input.ControllerButtonWest,
		input.ControllerButtonNorth,
	}
	modifier := bindings.LeftModifier
	if slot >= 4 {
		modifier = bindings.RightModifier
	}
	return fmt.Sprintf("%s+%s",
		controllerPromptButtonCompact(kind, modifier),
		controllerPromptButtonCompact(kind, faces[slot%4]),
	)
}

func controllerPromptButtonLabel(kind input.ControllerKind, button input.ControllerButton) string {
	family := controllerPromptFamilyForKind(kind)
	switch button {
	case input.ControllerButtonSouth:
		switch family {
		case controllerPromptXbox, controllerPromptSteamDeck:
			return "A"
		case controllerPromptPlayStation:
			return "Cross"
		case controllerPromptSwitch:
			return "B"
		default:
			return "South"
		}
	case input.ControllerButtonEast:
		switch family {
		case controllerPromptXbox, controllerPromptSteamDeck:
			return "B"
		case controllerPromptPlayStation:
			return "Circle"
		case controllerPromptSwitch:
			return "A"
		default:
			return "East"
		}
	case input.ControllerButtonWest:
		switch family {
		case controllerPromptXbox, controllerPromptSteamDeck:
			return "X"
		case controllerPromptPlayStation:
			return "Square"
		case controllerPromptSwitch:
			return "Y"
		default:
			return "West"
		}
	case input.ControllerButtonNorth:
		switch family {
		case controllerPromptXbox, controllerPromptSteamDeck:
			return "Y"
		case controllerPromptPlayStation:
			return "Triangle"
		case controllerPromptSwitch:
			return "X"
		default:
			return "North"
		}
	case input.ControllerButtonBack:
		switch family {
		case controllerPromptPlayStation:
			if kind == input.ControllerKindPlayStation4 {
				return "Share"
			}
			return "Create"
		case controllerPromptSwitch:
			return "Minus"
		case controllerPromptSteamDeck, controllerPromptXbox:
			return "View"
		default:
			return "Back"
		}
	case input.ControllerButtonStart:
		switch family {
		case controllerPromptPlayStation:
			return "Options"
		case controllerPromptSwitch:
			return "Plus"
		case controllerPromptSteamDeck, controllerPromptXbox:
			return "Menu"
		default:
			return "Start"
		}
	case input.ControllerButtonLeftStick:
		return "L3"
	case input.ControllerButtonRightStick:
		return "R3"
	case input.ControllerButtonLeftShoulder:
		switch family {
		case controllerPromptXbox:
			return "LB"
		case controllerPromptSwitch:
			return "L"
		default:
			return "L1"
		}
	case input.ControllerButtonRightShoulder:
		switch family {
		case controllerPromptXbox:
			return "RB"
		case controllerPromptSwitch:
			return "R"
		default:
			return "R1"
		}
	case input.ControllerButtonDPadUp:
		return "D-Pad Up"
	case input.ControllerButtonDPadDown:
		return "D-Pad Down"
	case input.ControllerButtonDPadLeft:
		return "D-Pad Left"
	case input.ControllerButtonDPadRight:
		return "D-Pad Right"
	case input.ControllerButtonTouchpad:
		switch family {
		case controllerPromptSteamDeck:
			return "Trackpad"
		case controllerPromptPlayStation:
			return "Touchpad"
		default:
			return "Touchpad"
		}
	case input.ControllerButtonLeftTrigger:
		switch family {
		case controllerPromptXbox:
			return "LT"
		case controllerPromptSwitch:
			return "ZL"
		default:
			return "L2"
		}
	case input.ControllerButtonRightTrigger:
		switch family {
		case controllerPromptXbox:
			return "RT"
		case controllerPromptSwitch:
			return "ZR"
		default:
			return "R2"
		}
	case input.ControllerButtonGuide:
		switch family {
		case controllerPromptSteamDeck:
			return "Steam"
		case controllerPromptPlayStation:
			return "PS"
		case controllerPromptSwitch:
			return "Home"
		case controllerPromptXbox:
			return "Xbox"
		default:
			return "Guide"
		}
	case input.ControllerButtonMisc1:
		switch family {
		case controllerPromptSteamDeck:
			return "QAM"
		case controllerPromptSwitch:
			return "Capture"
		default:
			return "Misc"
		}
	case input.ControllerButtonLeftPaddle1:
		return "L4"
	case input.ControllerButtonRightPaddle1:
		return "R4"
	case input.ControllerButtonLeftPaddle2:
		return "L5"
	case input.ControllerButtonRightPaddle2:
		return "R5"
	default:
		return "?"
	}
}

func controllerPromptButtonCompact(kind input.ControllerKind, button input.ControllerButton) string {
	if controllerPromptFamilyForKind(kind) == controllerPromptPlayStation {
		switch button {
		case input.ControllerButtonSouth:
			return "X"
		case input.ControllerButtonEast:
			return "O"
		case input.ControllerButtonWest:
			return "[]"
		case input.ControllerButtonNorth:
			return "/\\"
		}
	}
	switch button {
	case input.ControllerButtonDPadUp:
		return "D↑"
	case input.ControllerButtonDPadDown:
		return "D↓"
	case input.ControllerButtonDPadLeft:
		return "D←"
	case input.ControllerButtonDPadRight:
		return "D→"
	}
	return controllerPromptButtonLabel(kind, button)
}