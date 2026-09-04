package ui

import (
	"testing"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
)

type controllerHostStub struct {
	settings input.ControllerSettings
}

func (h *controllerHostStub) ControllerSettings() input.ControllerSettings {
	return h.settings
}

func (h *controllerHostStub) ApplyControllerSettings(settings input.ControllerSettings) bool {
	h.settings = settings
	return true
}

func TestControllerBindingCaptureDisplacesDuplicate(t *testing.T) {
	settings := input.DefaultControllerSettings()
	if settings.Bindings.Get(input.ActionCancel) != input.ControllerButtonEast {
		t.Fatalf("test assumes Cancel defaults to East, got %v", settings.Bindings.Get(input.ActionCancel))
	}

	// Binding Confirm to East, which Cancel already holds, must move Cancel off
	// it rather than leaving two actions on one button.
	settings.Bindings.Set(input.ActionConfirm, input.ControllerButtonEast)
	if got := settings.Bindings.Get(input.ActionConfirm); got != input.ControllerButtonEast {
		t.Fatalf("confirm = %v want east", got)
	}
	if got := settings.Bindings.Get(input.ActionCancel); got == input.ControllerButtonEast {
		t.Fatal("cancel was not displaced")
	}
}

func TestControllerWindowRebindActiveGatesDispatch(t *testing.T) {
	var window ControllerWindow
	if window.RebindActive() {
		t.Fatal("fresh window reported a capture in progress")
	}
	host := &controllerHostStub{settings: input.DefaultControllerSettings()}
	ctx := client.Context{Input: input.NewState(), ControllerSettingsHost: host}

	window.beginCapture(ctx, input.ActionConfirm)
	if !window.RebindActive() {
		t.Fatal("capture not reported active")
	}
	window.cancelCapture()
	if window.RebindActive() {
		t.Fatal("capture still active after cancel")
	}
}

func TestControllerWindowRestoreDefaults(t *testing.T) {
	settings := input.DefaultControllerSettings()
	settings.Bindings.Set(input.ActionConfirm, input.ControllerButtonNorth)
	if settings.Bindings.Get(input.ActionConfirm) == input.DefaultControllerBindings().Confirm {
		t.Fatal("setup did not change the binding")
	}
	settings.Bindings = input.DefaultControllerBindings()
	if settings.Bindings.Get(input.ActionConfirm) != input.ControllerButtonSouth {
		t.Fatalf("defaults not restored: %v", settings.Bindings.Get(input.ActionConfirm))
	}
}

func TestSettingsWindowPersistsControllerSettings(t *testing.T) {
	// Regression: UserSettings.Controller was never populated, which made the
	// entire [controller] INI writer unreachable.
	host := &controllerHostStub{settings: input.DefaultControllerSettings()}
	host.settings.MoveMode = input.ControllerMoveCursor
	ctx := client.Context{ControllerSettingsHost: host}

	got := settingsController(ctx)
	if got.MoveMode != input.ControllerMoveCursor {
		t.Fatalf("live controller settings not read: %#v", got)
	}
	if label := controllerMoveModeLabel(ctx); label != "Movement: Cursor" {
		t.Fatalf("move mode label = %q", label)
	}
}
