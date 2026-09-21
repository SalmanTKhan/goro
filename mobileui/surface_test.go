package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestSurfaceIsBoundedAndSafeAreaAwareOnFoldOuter(t *testing.T) {
	viewport := Viewport{Width: 2268, Height: 832, SafeLeft: 24, SafeRight: 24, SafeTop: 18, SafeBottom: 18}
	model := SettingsSurface()
	c := NewSurfaceController(model, viewport, nil)
	l := c.Layout
	if l.Panel.W != 1200 {
		t.Fatalf("panel width=%v, want bounded 1200", l.Panel.W)
	}
	if l.Panel.X < l.Safe.X || l.Panel.Right() > l.Safe.Right() || l.Panel.Y < l.Safe.Y || l.Panel.Bottom() > l.Safe.Bottom() {
		t.Fatalf("surface escaped safe area: panel=%+v safe=%+v", l.Panel, l.Safe)
	}
	for _, row := range l.Rows {
		if row.W < 48 || row.H < 48 {
			t.Fatalf("surface row missed touch target: %+v", row)
		}
	}
	if !c.ConsumeTouch(input.TouchPoint{X: int(l.Panel.X + 4), Y: int(l.Panel.Y + 4)}) {
		t.Fatal("surface did not claim its touch")
	}
}

func TestSurfaceSelectionDetailBackAndScroll(t *testing.T) {
	model := SurfaceModel{Title: "SURFACE", Items: make([]SurfaceItem, 20)}
	for i := range model.Items {
		model.Items[i] = SurfaceItem{ID: string(rune('a' + i)), Label: "Item", Value: "Open", Enabled: true}
	}
	c := NewSurfaceController(model, FoldOuterViewport(), nil)
	row := c.Layout.Rows[0]
	if !c.Tap(row.X+2, row.Y+2) || !c.State.DetailOpen || c.State.SelectedID != model.Items[0].ID {
		t.Fatalf("selection state=%+v", c.State)
	}
	if !c.Back() || c.State.DetailOpen || c.State.SelectedID != "" {
		t.Fatalf("detail back state=%+v", c.State)
	}
	if !c.ScrollBy(100000) || c.Scroll.Offset != c.Scroll.MaxOffset() {
		t.Fatalf("scroll=%+v", c.Scroll)
	}
	if !c.ScrollBy(-100000) || c.Scroll.Offset != 0 {
		t.Fatalf("reverse scroll=%+v", c.Scroll)
	}
}

func TestSurfaceDisabledCameraRotationRemainsPresentationOnly(t *testing.T) {
	model := SettingsSurface()
	c := NewSurfaceController(model, Viewport{Width: 1920, Height: 1080}, nil)

	// Camera rotation intentionally lives in the touch-controls section, which
	// can be below the initial viewport as settings sections evolve. Scroll to
	// its semantic content offset instead of assuming model index == visible row.
	offset := float32(0)
	found := false
	for _, item := range model.Items {
		if item.ID == "camera-rotation" {
			found = true
			break
		}
		offset += SurfaceRowHeight(item) + surfaceRowGap
	}
	if !found {
		t.Fatal("camera rotation setting missing")
	}
	c.ScrollBy(offset)

	var cameraRotation Rect
	for i, id := range c.Layout.RowIDs {
		if id == "camera-rotation" && i < len(c.Layout.Rows) {
			cameraRotation = c.Layout.Rows[i]
			break
		}
	}
	if cameraRotation.W == 0 {
		t.Fatalf("camera rotation row not laid out after scroll: offset=%v scroll=%+v ids=%v", offset, c.Scroll, c.Layout.RowIDs)
	}
	if !c.Tap(cameraRotation.X+2, cameraRotation.Y+2) || c.State.SelectedID != "camera-rotation" {
		t.Fatalf("disabled item did not expose its explanation: state=%+v", c.State)
	}
}

func TestSettingsSurfaceExposesResolvedControlValues(t *testing.T) {
	controls := input.DefaultMobileControls()
	controls.MovementMode = input.MovementTapToMove
	controls.CameraSensitivity = 1.5
	model := SettingsSurfaceForControls(controls)
	values := map[string]string{}
	for _, item := range model.Items {
		values[item.ID] = item.Value
	}
	if values["movement"] != "Tap to move" || values["camera-sensitivity"] != "1.50x" || values["camera-rotation"] != "2-finger drag" {
		t.Fatalf("settings values = %#v", values)
	}
}

func TestSettingsSurfaceExposesLiveAudioDisplayAndGameplayValues(t *testing.T) {
	settings := input.DefaultMobileSettings()
	settings.Audio.BGMEnabled = false
	settings.Audio.BGMVolume = 0.25
	settings.Audio.SFXVolume = 0.75
	settings.Display.ShowMinimap = false
	settings.Display.VSync = false
	settings.Display.FPS = true
	settings.Display.Presentation = input.MobilePresentationDesktop
	settings.Gameplay.NoShift = true
	settings.Gameplay.NoCtrl = false
	settings.Gameplay.LessEffects = true
	model := SettingsSurfaceForSettings(settings)
	values := map[string]string{}
	for _, item := range model.Items {
		values[item.ID] = item.Value
	}
	want := map[string]string{
		"bgm-enabled": "Off", "bgm-volume": "25%", "sfx-volume": "75%",
		"show-minimap": "Off", "vsync": "Off", "fps-meter": "On", "presentation": "Desktop UI",
		"no-shift": "On", "no-ctrl": "Off", "less-effects": "On",
	}
	for id, value := range want {
		if values[id] != value {
			t.Fatalf("settings value %s = %q, want %q", id, values[id], value)
		}
	}
}

func TestSettingsSurfaceAppliesItemCallback(t *testing.T) {
	controller := NewSurfaceController(SettingsSurface(), FoldOuterViewport(), nil)
	var tapped string
	controller.OnItemTap = func(item SurfaceItem) bool {
		tapped = item.ID
		return true
	}
	rowIndex := -1
	for i, id := range controller.Layout.RowIDs {
		if id == "presentation" {
			rowIndex = i
			break
		}
	}
	if rowIndex < 0 {
		t.Fatal("presentation setting was not laid out")
	}
	row := controller.Layout.Rows[rowIndex]
	if !controller.Tap(row.X+2, row.Y+2) || tapped != "presentation" {
		t.Fatalf("tap result=%t tapped=%q", tapped != "", tapped)
	}
	if controller.State.DetailOpen {
		t.Fatal("callback setting opened a placeholder detail sheet")
	}
}

func TestSettingsSurfaceForSessionExposesOnlineDisconnectOnlyOnline(t *testing.T) {
	settings := input.DefaultMobileSettings()
	online := SettingsSurfaceForSession(settings, true, "Local Sabine", "connected to 127.0.0.1:5121")
	offline := SettingsSurfaceForSession(settings, false, "Local Sabine", "offline")

	var onlineDisconnect, offlineDisconnect SurfaceItem
	for _, item := range online.Items {
		if item.ID == "disconnect" {
			onlineDisconnect = item
		}
	}
	for _, item := range offline.Items {
		if item.ID == "disconnect" {
			offlineDisconnect = item
		}
	}
	if !onlineDisconnect.Enabled || offlineDisconnect.Enabled {
		t.Fatalf("disconnect online=%+v offline=%+v", onlineDisconnect, offlineDisconnect)
	}
	if onlineDisconnect.Detail == "" || online.Items[1].Value != "connected to 127.0.0.1:5121" {
		t.Fatalf("online session projection=%+v", online.Items[:4])
	}
}


func TestSettingsSurfaceMatchesDesktopDisplayControls(t *testing.T) {
	model := SettingsSurface()
	ids := map[string]SurfaceItem{}
	for _, item := range model.Items {
		ids[item.ID] = item
	}
	for _, id := range []string{"presentation", "ui-scale", "vsync", "fps-meter", "show-minimap"} {
		item, ok := ids[id]
		if !ok || !item.Enabled {
			t.Fatalf("display setting %q missing or disabled: %+v", id, item)
		}
	}
	if ids["presentation"].Detail == "" || ids["vsync"].Detail == "" || ids["fps-meter"].Detail == "" {
		t.Fatalf("live display settings need explanatory detail: presentation=%+v vsync=%+v fps=%+v", ids["presentation"], ids["vsync"], ids["fps-meter"])
	}
}
