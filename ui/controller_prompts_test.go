package ui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestControllerPromptFaceLabelsFollowDeviceFamily(t *testing.T) {
	tests := []struct {
		name string
		kind input.ControllerKind
		button input.ControllerButton
		want string
	}{
		{"deck south", input.ControllerKindSteamDeck, input.ControllerButtonSouth, "A"},
		{"deck west", input.ControllerKindSteamDeck, input.ControllerButtonWest, "X"},
		{"dual sense south", input.ControllerKindPlayStation5, input.ControllerButtonSouth, "Cross"},
		{"dual sense west", input.ControllerKindPlayStation5, input.ControllerButtonWest, "Square"},
		{"switch south", input.ControllerKindSwitch, input.ControllerButtonSouth, "B"},
		{"switch east", input.ControllerKindSwitch, input.ControllerButtonEast, "A"},
		{"xbox west", input.ControllerKindXbox, input.ControllerButtonWest, "X"},
		{"generic south", input.ControllerKindStandard, input.ControllerButtonSouth, "South"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := controllerPromptButtonLabel(tt.kind, tt.button); got != tt.want {
				t.Fatalf("controllerPromptButtonLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestControllerShortcutPromptUsesRecommendedModifierLayers(t *testing.T) {
	bindings := input.DefaultControllerBindings()
	tests := []struct {
		slot int
		want string
	}{
		{0, "L2+A"},
		{1, "L2+B"},
		{2, "L2+X"},
		{3, "L2+Y"},
		{4, "R2+A"},
		{5, "R2+B"},
		{6, "R2+X"},
		{7, "R2+Y"},
	}
	for _, tt := range tests {
		if got := controllerShortcutPromptFor(input.ControllerKindSteamDeck, bindings, tt.slot); got != tt.want {
			t.Fatalf("slot %d prompt = %q, want %q", tt.slot, got, tt.want)
		}
	}
}

func TestControllerShortcutPromptFollowsReboundModifiers(t *testing.T) {
	bindings := input.DefaultControllerBindings()
	bindings.LeftModifier = input.ControllerButtonLeftShoulder
	bindings.RightModifier = input.ControllerButtonRightShoulder

	if got := controllerShortcutPromptFor(input.ControllerKindXbox, bindings, 0); got != "LB+A" {
		t.Fatalf("left-layer prompt = %q, want LB+A", got)
	}
	if got := controllerShortcutPromptFor(input.ControllerKindXbox, bindings, 4); got != "RB+A" {
		t.Fatalf("right-layer prompt = %q, want RB+A", got)
	}
}

func TestControllerPromptPlatformShoulderNames(t *testing.T) {
	if got := controllerPromptButtonLabel(input.ControllerKindXbox, input.ControllerButtonLeftTrigger); got != "LT" {
		t.Fatalf("Xbox trigger = %q, want LT", got)
	}
	if got := controllerPromptButtonLabel(input.ControllerKindPlayStation5, input.ControllerButtonLeftTrigger); got != "L2" {
		t.Fatalf("PlayStation trigger = %q, want L2", got)
	}
	if got := controllerPromptButtonLabel(input.ControllerKindSwitch, input.ControllerButtonLeftTrigger); got != "ZL" {
		t.Fatalf("Switch trigger = %q, want ZL", got)
	}
}