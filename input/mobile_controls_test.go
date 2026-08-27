package input

import "testing"

func TestDefaultMobileControls(t *testing.T) {
	controls := DefaultMobileControls()
	if controls.MovementMode != MovementHoldToMove || controls.CameraSensitivity != 1 || controls.ZoomSensitivity != 1 || controls.LongPressMS != 550 || !controls.ShowTargetNames {
		t.Fatalf("defaults = %#v", controls)
	}
	if err := controls.Validate(); err != nil {
		t.Fatalf("defaults rejected: %v", err)
	}
	if got := controls.GestureConfig().LongPressDuration.Milliseconds(); got != 550 {
		t.Fatalf("long press duration = %d, want 550", got)
	}
}

func TestMobileControlsValidationAndNormalization(t *testing.T) {
	controls := DefaultMobileControls()
	controls.CameraSensitivity = 0
	if err := controls.Validate(); err == nil {
		t.Fatal("expected invalid camera sensitivity")
	}
	if got := controls.Normalized(); got != DefaultMobileControls() {
		t.Fatalf("normalized invalid controls = %#v", got)
	}
	controls = DefaultMobileControls()
	controls.LongPressMS = 1501
	if err := controls.Validate(); err == nil {
		t.Fatal("expected invalid long press")
	}
}

func TestDefaultMobileSettingsAreCompleteAndNormalized(t *testing.T) {
	settings := DefaultMobileSettings()
	if !settings.Audio.BGMEnabled || settings.Audio.BGMVolume != 0.55 || settings.Audio.SFXVolume != 0.55 {
		t.Fatalf("audio defaults = %+v", settings.Audio)
	}
	if !settings.Display.ShowMinimap || !settings.Gameplay.NoCtrl {
		t.Fatalf("display/gameplay defaults = %+v / %+v", settings.Display, settings.Gameplay)
	}
	if got := settings.Normalized(); got != settings {
		t.Fatalf("normalized defaults = %+v, want %+v", got, settings)
	}
	settings.Audio.BGMVolume = -1
	settings.Audio.SFXVolume = 2
	if got := settings.Normalized(); got.Audio.BGMVolume != 0.55 || got.Audio.SFXVolume != 0.55 {
		t.Fatalf("invalid volume normalization = %+v", got.Audio)
	}
}
