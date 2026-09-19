// Package inputprompt resolves controller actions to context-appropriate
// prompt glyphs. It is presentation-only: gameplay continues to use the
// positional ControllerButton vocabulary from input.
package inputprompt

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kivutar/goro/input"
)

// Family identifies the visual controller family, not gameplay hardware.
type Family uint8

const (
	FamilyGeneric Family = iota
	FamilyXbox
	FamilyPlayStation
	FamilySwitch
	FamilySteamDeck
)

func FamilyForKind(kind input.ControllerKind) Family {
	switch kind {
	case input.ControllerKindXbox:
		return FamilyXbox
	case input.ControllerKindPlayStation4, input.ControllerKindPlayStation5:
		return FamilyPlayStation
	case input.ControllerKindSwitch:
		return FamilySwitch
	case input.ControllerKindSteamDeck:
		return FamilySteamDeck
	default:
		return FamilyGeneric
	}
}

func (f Family) String() string {
	switch f {
	case FamilyXbox:
		return "xbox"
	case FamilyPlayStation:
		return "playstation"
	case FamilySwitch:
		return "switch"
	case FamilySteamDeck:
		return "steamdeck"
	default:
		return "generic"
	}
}

// Prompt is a glyph plus a text fallback. Image is nil only when the
// requested asset is unavailable; Text remains suitable for diagnostics.
type Prompt struct {
	Image image.Image
	Text  string
}

// FooterText provides a compact text fallback for surfaces that cannot yet
// host image-bearing prompt widgets. It still resolves through bindings and
// therefore remains correct after rebinding or controller-family changes.
func (r *Resolver) FooterText(settings input.ControllerSettings, kind input.ControllerKind, actions ...input.Action) string {
	parts := make([]string, 0, len(actions))
	for _, action := range actions {
		prompt := r.ForAction(settings, kind, action)
		parts = append(parts, prompt.Text+" "+action.Name())
	}
	return strings.Join(parts, "   ")
}

// Resolver owns the optional asset cache. Decoding happens once per asset.
type Resolver struct {
	mu    sync.Mutex
	cache map[string]image.Image
}

func NewResolver() *Resolver { return &Resolver{cache: make(map[string]image.Image)} }

func (r *Resolver) ForAction(settings input.ControllerSettings, kind input.ControllerKind, action input.Action) Prompt {
	button := settings.Bindings.Get(action)
	return r.ForButton(FamilyForKind(kind), button)
}

func (r *Resolver) ForButton(family Family, button input.ControllerButton) Prompt {
	name := glyphName(family, button)
	path := fmt.Sprintf("assets/%s/%s.png", family, name)
	return Prompt{Image: r.load(path), Text: label(family, button)}
}

func (r *Resolver) ForShortcut(settings input.ControllerSettings, kind input.ControllerKind, slot int) []Prompt {
	if slot < 0 || slot >= 8 {
		return nil
	}
	family := FamilyForKind(kind)
	modifier := settings.Bindings.LeftModifier
	if slot >= 4 {
		modifier = settings.Bindings.RightModifier
	}
	faces := []input.ControllerButton{input.ControllerButtonSouth, input.ControllerButtonEast, input.ControllerButtonWest, input.ControllerButtonNorth}
	return []Prompt{r.ForButton(family, modifier), r.ForButton(family, faces[slot%4])}
}

func (r *Resolver) load(path string) image.Image {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if img, ok := r.cache[path]; ok {
		return img
	}
	root := os.Getenv("GORO_INPUT_PROMPTS_DIR")
	if root == "" {
		r.cache[path] = nil
		return nil
	}
	rel := strings.TrimPrefix(path, "assets/")
	parts := strings.SplitN(rel, "/", 2)
	folders := map[string][]string{
		"generic": {"Generic"}, "playstation": {"PlayStation Series"},
		"steamdeck": {"Steam Deck"}, "switch": {"Nintendo Switch"}, "xbox": {"Xbox Series"},
	}[parts[0]]
	if len(folders) == 0 {
		folders = []string{parts[0]}
	}
	var data []byte
	var err error
	for _, folder := range folders {
		for _, variant := range []string{"Default", ""} {
			candidate := filepath.Join(root, folder, variant, filepath.FromSlash(parts[1]))
			data, err = os.ReadFile(candidate)
			if err == nil {
				break
			}
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		r.cache[path] = nil
		return nil
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		r.cache[path] = nil
		return nil
	}
	r.cache[path] = img
	return img
}

func glyphName(f Family, b input.ControllerButton) string {
	prefix := map[Family]string{FamilyXbox: "xbox", FamilyPlayStation: "playstation", FamilySwitch: "switch", FamilySteamDeck: "steamdeck", FamilyGeneric: "generic"}[f]
	if f == FamilyGeneric {
		switch b {
		case input.ControllerButtonLeftTrigger:
			return "generic_button_trigger_a"
		case input.ControllerButtonRightTrigger:
			return "generic_button_trigger_b"
		case input.ControllerButtonLeftStick, input.ControllerButtonRightStick:
			return "generic_stick_press"
		default:
			return "generic_button"
		}
	}
	switch b {
	case input.ControllerButtonSouth:
		name := "a"
		if f == FamilySwitch {
			name = "b"
		} else if f == FamilyPlayStation {
			name = "cross"
		}
		return prefix + "_button_" + name
	case input.ControllerButtonEast:
		name := "b"
		if f == FamilySwitch {
			name = "a"
		} else if f == FamilyPlayStation {
			name = "circle"
		}
		return prefix + "_button_" + name
	case input.ControllerButtonWest:
		name := "x"
		if f == FamilySwitch {
			name = "y"
		} else if f == FamilyPlayStation {
			name = "square"
		}
		return prefix + "_button_" + name
	case input.ControllerButtonNorth:
		name := "y"
		if f == FamilySwitch {
			name = "x"
		} else if f == FamilyPlayStation {
			name = "triangle"
		}
		return prefix + "_button_" + name
	case input.ControllerButtonLeftShoulder:
		if f == FamilyXbox {
			return "xbox_lb"
		}
		if f == FamilyPlayStation {
			return "playstation_trigger_l1"
		}
		if f == FamilySwitch {
			return "switch_button_l"
		}
		return prefix + "_button_l1"
	case input.ControllerButtonRightShoulder:
		if f == FamilyXbox {
			return "xbox_rb"
		}
		if f == FamilyPlayStation {
			return "playstation_trigger_r1"
		}
		if f == FamilySwitch {
			return "switch_button_r"
		}
		return prefix + "_button_r1"
	case input.ControllerButtonLeftTrigger:
		if f == FamilyXbox {
			return "xbox_lt"
		}
		if f == FamilyPlayStation {
			return "playstation_trigger_l2"
		}
		return prefix + "_button_" + map[Family]string{FamilySwitch: "zl", FamilySteamDeck: "l2"}[f]
	case input.ControllerButtonRightTrigger:
		if f == FamilyXbox {
			return "xbox_rt"
		}
		if f == FamilyPlayStation {
			return "playstation_trigger_r2"
		}
		return prefix + "_button_" + map[Family]string{FamilySwitch: "zr", FamilySteamDeck: "r2"}[f]
	case input.ControllerButtonBack:
		if f == FamilyPlayStation {
			return "playstation4_button_share"
		}
		return prefix + "_button_" + map[Family]string{FamilyXbox: "back", FamilySwitch: "minus", FamilySteamDeck: "view"}[f]
	case input.ControllerButtonStart:
		if f == FamilyPlayStation {
			return "playstation5_button_options"
		}
		return prefix + "_button_" + map[Family]string{FamilyXbox: "menu", FamilySwitch: "plus", FamilySteamDeck: "options"}[f]
	case input.ControllerButtonLeftStick:
		return prefix + "_stick_l_press"
	case input.ControllerButtonRightStick:
		return prefix + "_stick_r_press"
	default:
		return prefix + "_dpad"
	}
}

func label(f Family, b input.ControllerButton) string {
	labels := map[Family]map[input.ControllerButton]string{
		FamilyGeneric:     {input.ControllerButtonSouth: "A", input.ControllerButtonEast: "B", input.ControllerButtonWest: "X", input.ControllerButtonNorth: "Y", input.ControllerButtonLeftTrigger: "LT", input.ControllerButtonRightTrigger: "RT", input.ControllerButtonLeftShoulder: "LB", input.ControllerButtonRightShoulder: "RB"},
		FamilyXbox:        {input.ControllerButtonSouth: "A", input.ControllerButtonEast: "B", input.ControllerButtonWest: "X", input.ControllerButtonNorth: "Y", input.ControllerButtonLeftTrigger: "LT", input.ControllerButtonRightTrigger: "RT"},
		FamilySteamDeck:   {input.ControllerButtonSouth: "A", input.ControllerButtonEast: "B", input.ControllerButtonWest: "X", input.ControllerButtonNorth: "Y", input.ControllerButtonLeftTrigger: "L2", input.ControllerButtonRightTrigger: "R2"},
		FamilyPlayStation: {input.ControllerButtonSouth: "Cross", input.ControllerButtonEast: "Circle", input.ControllerButtonWest: "Square", input.ControllerButtonNorth: "Triangle", input.ControllerButtonLeftTrigger: "L2", input.ControllerButtonRightTrigger: "R2"},
		FamilySwitch:      {input.ControllerButtonSouth: "B", input.ControllerButtonEast: "A", input.ControllerButtonWest: "Y", input.ControllerButtonNorth: "X", input.ControllerButtonLeftTrigger: "ZL", input.ControllerButtonRightTrigger: "ZR"},
	}
	if text := labels[f][b]; text != "" {
		return text
	}
	return fmt.Sprintf("Button %d", b)
}
