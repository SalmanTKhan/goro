package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestProfileLayoutFitsRepresentativeViewports(t *testing.T) {
	for _, viewport := range []Viewport{{Width: 390, Height: 844}, {Width: 1920, Height: 1080}, FoldOuterViewport()} {
		model := FixtureProfile("profile-custom")
		for _, editing := range []bool{false, true} {
			layout := LayoutMobileProfile(viewport, model, editing, false)
			assertInsideRect(t, "profile panel", 0, layout.Panel, layout.Safe)
			assertInsideRect(t, "profile preview", 0, layout.Preview, layout.Panel)
			assertInsideRect(t, "profile identity", 0, layout.Identity, layout.Panel)
			assertInsideRect(t, "profile appearance", 0, layout.Appearance, layout.Panel)
			assertInsideRect(t, "profile stats", 0, layout.Stats, layout.Panel)
			if editing {
				for i, key := range LayoutMobileProfile(viewport, model, true, true).Keys {
					assertInsideRect(t, "profile keyboard key", i, key.Rect, layout.Panel)
					if key.Rect.W < 48 || key.Rect.H < 48 {
						t.Fatalf("keyboard key below touch target: %+v", key.Rect)
					}
				}
			}
		}
	}
}

func TestProfileControllerEditsNameAndEmitsSave(t *testing.T) {
	var sink input.CommandBuffer
	c := NewProfileController(FixtureProfile("profile-custom"), Viewport{Width: 390, Height: 844}, &sink)
	c.OpenProfile(c.Model)
	if !c.Tap(c.Layout.EditButton.X+1, c.Layout.EditButton.Y+1) || !c.Editor {
		t.Fatal("profile editor did not open")
	}
	if !c.Tap(c.Layout.NameField.X+1, c.Layout.NameField.Y+1) || !c.EditingName {
		t.Fatal("name editor did not open")
	}
	for _, key := range c.Layout.Keys {
		if key.Key == "CLEAR" {
			c.Tap(key.Rect.X+1, key.Rect.Y+1)
			break
		}
	}
	if c.Draft.Name != "" {
		t.Fatalf("clear key did not clear name: %q", c.Draft.Name)
	}
	for _, want := range []string{"G", "O", "R", "O"} {
		for _, key := range c.Layout.Keys {
			if key.Key == want {
				c.Tap(key.Rect.X+1, key.Rect.Y+1)
				break
			}
		}
	}
	if !c.Tap(c.Layout.KeyboardDone.X+1, c.Layout.KeyboardDone.Y+1) {
		t.Fatal("keyboard done key did not claim touch")
	}
	if c.EditingName {
		t.Fatal("keyboard done key did not close name editor")
	}
	c.Tap(c.Layout.SaveButton.X+1, c.Layout.SaveButton.Y+1)
	commands := sink.Commands()
	if len(commands) != 1 || commands[0].Kind != input.CommandSaveOfflineProfile || commands[0].Text != "GORO" {
		t.Fatalf("profile save command = %#v", commands)
	}
}

func TestProfileControllerBackDismissesKeyboardWithoutClosingEditor(t *testing.T) {
	c := NewProfileController(FixtureProfile("profile-basic"), Viewport{Width: 390, Height: 844}, nil)
	c.OpenProfile(c.Model)
	c.Tap(c.Layout.EditButton.X+1, c.Layout.EditButton.Y+1)
	c.Tap(c.Layout.NameField.X+1, c.Layout.NameField.Y+1)
	if !c.Back() {
		t.Fatal("back did not dismiss keyboard")
	}
	if c.EditingName || !c.Editor || !c.Open {
		t.Fatalf("back changed editor state while dismissing keyboard: open=%t editor=%t editing=%t", c.Open, c.Editor, c.EditingName)
	}
	if c.Layout.SaveButton.W < 48 || c.Layout.CancelButton.W < 48 {
		t.Fatalf("save/cancel actions are not available after keyboard dismissal: save=%+v cancel=%+v", c.Layout.SaveButton, c.Layout.CancelButton)
	}
}

func TestProfileControllerStatPairsStayAtStarterTotal(t *testing.T) {
	c := NewProfileController(FixtureProfile("profile-basic"), Viewport{Width: 1920, Height: 1080}, nil)
	c.OpenProfile(c.Model)
	c.Tap(c.Layout.EditButton.X+1, c.Layout.EditButton.Y+1)
	before := c.Draft.Stats
	c.Tap(c.Layout.StatPlus[0].X+1, c.Layout.StatPlus[0].Y+1)
	if c.Draft.Stats[0] != before[0]+1 || c.Draft.Stats[3] != before[3]-1 {
		t.Fatalf("starter stat pair did not move together: before=%v after=%v", before, c.Draft.Stats)
	}
}

func TestProfileLayoutReservesTextAndActionRows(t *testing.T) {
	if got := ProfileSexLabel(0); got != "FEMALE" {
		t.Fatalf("sex 0 label = %q, want FEMALE", got)
	}
	if got := ProfileSexLabel(1); got != "MALE" {
		t.Fatalf("sex 1 label = %q, want MALE", got)
	}

	for _, viewport := range []Viewport{{Width: 390, Height: 844}, {Width: 2400, Height: 1080}, FoldOuterViewport()} {
		model := FixtureProfile("profile-basic")
		view := LayoutMobileProfile(viewport, model, false, false)
		if view.NameLabel.Intersects(view.NameField) || view.SexLabel.Intersects(view.NameField) {
			t.Fatalf("profile identity rows overlap at %v: name=%+v field=%+v sex=%+v", viewport, view.NameLabel, view.NameField, view.SexLabel)
		}
		if view.StarterTotal.Intersects(view.EditButton) || view.StarterTotal.Intersects(view.NewButton) || view.StarterTotal.Intersects(view.ContinueButton) {
			t.Fatalf("profile footer rows overlap at %v: total=%+v edit=%+v new=%+v continue=%+v", viewport, view.StarterTotal, view.EditButton, view.NewButton, view.ContinueButton)
		}
		if !view.Stacked && view.Stats.X != view.Appearance.X {
			t.Fatalf("landscape stats escaped right rail at %v: stats=%+v appearance=%+v", viewport, view.Stats, view.Appearance)
		}

		editor := LayoutMobileProfile(viewport, model, true, false)
		if editor.AppearanceLabel.Intersects(editor.SexButton) || editor.SexButton.Intersects(editor.HairPrev) || editor.SexButton.Intersects(editor.HairNext) || editor.SexButton.Intersects(editor.HairColor) {
			t.Fatalf("profile editor controls overlap at %v: sex=%+v hair=%+v/%+v color=%+v", viewport, editor.SexButton, editor.HairPrev, editor.HairNext, editor.HairColor)
		}
		for i := 0; i < len(editor.StatRows); i++ {
			if editor.StatRows[i].W <= 0 {
				continue
			}
			if !editor.Stats.Contains(editor.StatMinus[i].X, editor.StatMinus[i].Y) || !editor.Stats.Contains(editor.StatPlus[i].Right(), editor.StatPlus[i].Bottom()) {
				t.Fatalf("profile stat touch controls escape card at %v index=%d row=%+v minus=%+v plus=%+v", viewport, i, editor.StatRows[i], editor.StatMinus[i], editor.StatPlus[i])
			}
		}
	}
}
