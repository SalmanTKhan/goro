package mobileui

import "testing"

func TestPhase1QInventoryTilesAreSquareAndPortraitUsesFourColumns(t *testing.T) {
	model := FixtureInventory("inventory-basic")
	for _, viewport := range []Viewport{
		{Width: 390, Height: 844},
		{Width: 1080, Height: 2400},
		{Width: 2400, Height: 1080},
	} {
		layout := LayoutInventory(viewport, DefaultInventoryTokens(), model, InventoryInteractionState{Screen: ScreenInventory})
		if viewport.Width == 1080 && layout.GridColumns != 4 {
			t.Fatalf("1080 portrait must use four columns: %+v", layout)
		}
		for i, cell := range layout.Cells {
			ratio := cell.Rect.H / cell.Rect.W
			if ratio < 0.90 || ratio > 1.15 {
				t.Fatalf("viewport %+v cell %d is not square-ish: rect=%+v ratio=%v", viewport, i, cell.Rect, ratio)
			}
		}
	}
}

func TestPhase1QEquipmentCompositionIsBoundedAndUnique(t *testing.T) {
	model := FixtureEquipment("equipment-full")
	for _, viewport := range []Viewport{{Width: 1080, Height: 2400}, {Width: 2400, Height: 1080}, FoldOuterViewport()} {
		layout := LayoutEquipment(viewport, DefaultInventoryTokens(), model, InventoryInteractionState{Screen: ScreenEquipment})
		preview := EquipmentPreviewRect(layout)
		assertInsideRect(t, "equipment preview", 0, preview, layout.PaperDoll)
		seen := map[uint16]bool{}
		for i, slot := range EquipmentPresentationSlots(layout) {
			assertInsideRect(t, "paper-doll slot", i, slot.Rect, layout.PaperDoll)
			assertTouchTarget(t, "paper-doll slot", slot.Rect)
			if seen[slot.Location] {
				t.Fatalf("viewport %+v repeats equipment location %d", viewport, slot.Location)
			}
			seen[slot.Location] = true
			if slot.Rect.Intersects(preview) {
				t.Fatalf("viewport %+v slot %q intersects avatar stage: slot=%+v preview=%+v", viewport, slot.Label, slot.Rect, preview)
			}
		}
		if len(seen) != len(model.Slots) {
			t.Fatalf("viewport %+v lost paper-doll slots: got=%d want=%d", viewport, len(seen), len(model.Slots))
		}
		if !viewport.IsPortrait() {
			if layout.DetailPanel.W <= 0 || layout.DetailPanel.Intersects(layout.PaperDoll) {
				t.Fatalf("viewport %+v has no bounded landscape summary rail: paper=%+v detail=%+v", viewport, layout.PaperDoll, layout.DetailPanel)
			}
		}
	}
}

func TestPhase1QHUDControlsUseReadableSafeRegions(t *testing.T) {
	for _, viewport := range []Viewport{
		{Width: 390, Height: 844},
		{Width: 1080, Height: 2400},
		{Width: 2400, Height: 1080},
		FoldOuterViewport(),
	} {
		layout := LayoutHUD(viewport, DefaultTokens(), Fixture("monster"), Navigation{})
		for name, rect := range map[string]Rect{
			"player": layout.PlayerPanel, "target": layout.TargetPanel, "minimap": layout.Minimap,
			"menu": layout.Menu, "chat": layout.ChatBar, "skill": layout.SkillBar,
		} {
			assertInside(t, name, rect, layout.Safe)
		}
		if layout.ChatBar.Intersects(layout.SkillBar) || layout.TargetPanel.Intersects(layout.SkillBar) {
			t.Fatalf("viewport %+v HUD controls overlap: chat=%+v target=%+v skill=%+v", viewport, layout.ChatBar, layout.TargetPanel, layout.SkillBar)
		}
		menu := LayoutHUD(viewport, DefaultTokens(), Fixture("monster"), Navigation{MenuOpen: true})
		assertInside(t, "menu drawer", menu.MenuPanel, menu.Safe)
		if viewport.IsPortrait() && menu.MenuPanel.W >= menu.Safe.W*0.90 {
			t.Fatalf("portrait menu still behaves like a full-width dropdown: %+v safe=%+v", menu.MenuPanel, menu.Safe)
		}
		for i, action := range menu.MenuActions {
			assertTouchTarget(t, "menu action", action.Rect)
			assertInsideRect(t, "menu action", i, action.Rect, menu.MenuPanel)
		}
	}
}
