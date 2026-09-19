package inputprompt

import (
	"os"
	"testing"

	"github.com/kivutar/goro/input"
)

func TestFamilyFaceLabels(t *testing.T) {
	settings := input.DefaultControllerSettings()
	for _, tc := range []struct {
		kind input.ControllerKind
		b    input.ControllerButton
		want string
	}{
		{input.ControllerKindSteamDeck, input.ControllerButtonSouth, "A"},
		{input.ControllerKindSteamDeck, input.ControllerButtonWest, "X"},
		{input.ControllerKindPlayStation5, input.ControllerButtonSouth, "Cross"},
		{input.ControllerKindPlayStation5, input.ControllerButtonWest, "Square"},
		{input.ControllerKindXbox, input.ControllerButtonSouth, "A"},
		{input.ControllerKindSwitch, input.ControllerButtonSouth, "B"},
		{input.ControllerKindSwitch, input.ControllerButtonEast, "A"},
	} {
		got := NewResolver().ForButton(FamilyForKind(tc.kind), tc.b)
		if got.Text != tc.want {
			t.Fatalf("kind=%v button=%v label=%q want %q", tc.kind, tc.b, got.Text, tc.want)
		}
		_ = settings
	}
}

func TestSemanticBindingAndShortcutModifiers(t *testing.T) {
	settings := input.DefaultControllerSettings()
	settings.Bindings.Attack = input.ControllerButtonNorth
	settings.Bindings.LeftModifier = input.ControllerButtonLeftShoulder
	r := NewResolver()
	if got := r.ForAction(settings, input.ControllerKindPlayStation5, input.ActionAttack).Text; got != "Triangle" {
		t.Fatalf("rebound attack = %q", got)
	}
	short := r.ForShortcut(settings, input.ControllerKindXbox, 0)
	if len(short) != 2 || short[0].Text != "Button 8" || short[1].Text != "A" {
		t.Fatalf("shortcut prompts = %#v", short)
	}
}

func TestPromptFallbackIsSafeWithoutOptionalAssets(t *testing.T) {
	r := NewResolver()
	settings := input.DefaultControllerSettings()
	families := []input.ControllerKind{input.ControllerKindSteamDeck, input.ControllerKindPlayStation5, input.ControllerKindXbox, input.ControllerKindSwitch, input.ControllerKindStandard}
	buttons := []input.ControllerButton{input.ControllerButtonSouth, input.ControllerButtonEast, input.ControllerButtonWest, input.ControllerButtonNorth, input.ControllerButtonLeftShoulder, input.ControllerButtonRightShoulder, input.ControllerButtonLeftTrigger, input.ControllerButtonRightTrigger, input.ControllerButtonBack, input.ControllerButtonStart, input.ControllerButtonLeftStick, input.ControllerButtonRightStick}
	for _, kind := range families {
		for _, button := range buttons {
			prompt := r.ForButton(FamilyForKind(kind), button)
			if prompt.Text == "" {
				t.Fatalf("missing fallback label kind=%v button=%v", kind, button)
			}
		}
		for slot := 0; slot < 8; slot++ {
			for _, prompt := range r.ForShortcut(settings, kind, slot) {
				if prompt.Text == "" {
					t.Fatalf("missing shortcut fallback kind=%v slot=%d", kind, slot)
				}
			}
		}
	}
	if os.Getenv("GORO_INPUT_PROMPTS_DIR") != "" {
		if r.ForButton(FamilyPlayStation, input.ControllerButtonSouth).Image == nil {
			t.Fatal("optional PlayStation prompt asset did not resolve")
		}
	}
}
