//go:build (windows || linux || darwin) && !android

package gamepad

import (
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kivutar/goro/input"
)

type ownershipSample struct {
	previous input.ControllerSnapshot
	next     input.ControllerSnapshot
}

func selectOwner(active sdl.JoystickID, samples map[sdl.JoystickID]ownershipSample) sdl.JoystickID {
	if active != 0 {
		if _, ok := samples[active]; ok {
			return active
		}
	}
	for id, sample := range samples {
		if meaningfulActivity(sample.previous, sample.next) {
			return id
		}
	}
	return 0
}

func TestAxisNormalizesSDLInt16Range(t *testing.T) {
	if got := axis(-32768); got != -1 {
		t.Fatalf("axis(-32768) = %f, want -1", got)
	}
	if got := axis(32767); got != 1 {
		t.Fatalf("axis(32767) = %f, want 1", got)
	}
}

func TestTriggerClampsSDLNegativeValues(t *testing.T) {
	if got := trigger(-32768); got != 0 {
		t.Fatalf("trigger(-32768) = %f, want 0", got)
	}
	if got := trigger(32767); got != 1 {
		t.Fatalf("trigger(32767) = %f, want 1", got)
	}
}

func TestControllerKindMapsSDLStandardTypes(t *testing.T) {
	tests := []struct {
		name string
		sdl  sdl.GamepadType
		want input.ControllerKind
	}{
		{"standard", sdl.GAMEPAD_TYPE_STANDARD, input.ControllerKindStandard},
		{"xbox", sdl.GAMEPAD_TYPE_XBOXONE, input.ControllerKindXbox},
		{"dualshock", sdl.GAMEPAD_TYPE_PS4, input.ControllerKindPlayStation4},
		{"dualsense", sdl.GAMEPAD_TYPE_PS5, input.ControllerKindPlayStation5},
		{"switch", sdl.GAMEPAD_TYPE_NINTENDO_SWITCH_PRO, input.ControllerKindSwitch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := controllerKind(test.sdl); got != test.want {
				t.Fatalf("controllerKind(%v) = %v, want %v", test.sdl, got, test.want)
			}
		})
	}
}

func TestRumbleMagnitudeClampsAndScales(t *testing.T) {
	for _, test := range []struct {
		value float32
		want  uint16
	}{
		{-1, 0},
		{0, 0},
		{0.5, 32767},
		{1, 0xffff},
		{2, 0xffff},
	} {
		if got := rumbleMagnitude(test.value); got != test.want {
			t.Errorf("rumbleMagnitude(%v) = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestMeaningfulActivityIgnoresJitterAndDetectsOwnershipInput(t *testing.T) {
	base := input.ControllerSnapshot{Connected: true, LeftX: 0.02, RightY: -0.03}
	if meaningfulActivity(input.ControllerSnapshot{Connected: true}, base) {
		t.Fatal("small analog jitter stole ownership")
	}
	if !meaningfulActivity(base, input.ControllerSnapshot{Connected: true, LeftX: 0.2, RightY: -0.03}) {
		t.Fatal("material stick movement did not claim ownership")
	}
	if !meaningfulActivity(base, input.ControllerSnapshot{Connected: true, Touchpads: [2]input.ControllerTouch{{}, {Down: true, X: 0.5, Y: 0.5}}}) {
		t.Fatal("trackpad contact did not claim ownership")
	}
	held := input.ControllerSnapshot{Connected: true, Buttons: buttonsForTest(input.ControllerButtonSouth)}
	if meaningfulActivity(held, held) {
		t.Fatal("held button continuously refreshed ownership")
	}
}

func TestOwnershipAcquiresWhenDevicesExistAndActiveIDIsZero(t *testing.T) {
	deck := sdl.JoystickID(1)
	dualSense := sdl.JoystickID(2)
	neutral := input.ControllerSnapshot{Connected: true}

	if got := selectOwner(0, map[sdl.JoystickID]ownershipSample{
		deck: {previous: input.ControllerSnapshot{}, next: neutral},
	}); got != 0 {
		t.Fatalf("neutral devices acquired owner %d", got)
	}

	deckInput := neutral
	deckInput.Buttons = buttonsForTest(input.ControllerButtonDPadUp)
	if got := selectOwner(0, map[sdl.JoystickID]ownershipSample{
		deck: {previous: neutral, next: deckInput},
	}); got != deck {
		t.Fatalf("D-pad input owner = %d, want Deck %d", got, deck)
	}

	dualInput := neutral
	dualInput.Buttons = buttonsForTest(input.ControllerButtonSouth)
	if got := selectOwner(0, map[sdl.JoystickID]ownershipSample{
		deck:      {previous: neutral, next: neutral},
		dualSense: {previous: neutral, next: dualInput},
	}); got != dualSense {
		t.Fatalf("DualSense input owner = %d, want %d", got, dualSense)
	}
}

func buttonsForTest(button input.ControllerButton) input.ControllerButtons {
	var buttons input.ControllerButtons
	buttons.Set(button, true)
	return buttons
}
