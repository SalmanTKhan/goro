package mobileui

import "testing"

func TestPhase1RScreenHeaderRowsDoNotOverlap(t *testing.T) {
	header := LayoutMobileScreenHeader(Rect{X: 16, Y: 16, W: 900, H: 76}, 100, 112)
	if header.Title.W <= 0 || header.Subtitle.W <= 0 {
		t.Fatalf("header title/subtitle lost usable width: %+v", header)
	}
	if header.Title.Bottom() > header.Subtitle.Y || header.Title.Intersects(header.Subtitle) {
		t.Fatalf("header title and subtitle overlap: title=%+v subtitle=%+v", header.Title, header.Subtitle)
	}
	if header.Back.Intersects(header.Title) || header.Action.Intersects(header.Title) {
		t.Fatalf("header controls collide with title: %+v", header)
	}
}

func TestPhase1RHUDChatAndPortraitRegionsAreSeparated(t *testing.T) {
	for _, viewport := range []Viewport{{Width: 390, Height: 844}, {Width: 1080, Height: 2400}} {
		layout := LayoutHUD(viewport, DefaultTokens(), Fixture("normal"), Navigation{})
		for name, rect := range map[string]Rect{
			"chat label": layout.ChatLabel, "chat prompt": layout.ChatPrompt, "chat button": layout.ChatButton,
		} {
			assertInsideRect(t, name, 0, rect, layout.ChatBar)
		}
		if layout.ChatLabel.Intersects(layout.ChatPrompt) || layout.ChatPrompt.Intersects(layout.ChatButton) || layout.ChatLabel.Intersects(layout.ChatButton) {
			t.Fatalf("chat regions overlap at %+v: label=%+v prompt=%+v button=%+v", viewport, layout.ChatLabel, layout.ChatPrompt, layout.ChatButton)
		}
		if viewport.IsPortrait() && layout.PlayerPanel.W > layout.Safe.W*0.70 {
			t.Fatalf("portrait HUD panel still dominates width: panel=%+v safe=%+v", layout.PlayerPanel, layout.Safe)
		}
		if layout.PlayerPanel.Intersects(layout.Minimap) {
			t.Fatalf("HUD player and minimap overlap: player=%+v minimap=%+v", layout.PlayerPanel, layout.Minimap)
		}
		for i, skill := range layout.SkillSlots {
			assertInsideRect(t, "skill", i, skill, layout.Safe)
			if skill.W < 48 || skill.H < 48 || skill.W/skill.H < 0.82 || skill.W/skill.H > 1.18 {
				t.Fatalf("skill control is not a square touch target: %+v", skill)
			}
		}
	}
}

func TestPhase1REquipmentClusterAndSummaryAreBounded(t *testing.T) {
	model := FixtureEquipment("equipment-full")
	portrait := LayoutEquipment(Viewport{Width: 1080, Height: 2400}, DefaultInventoryTokens(), model, InventoryInteractionState{Screen: ScreenEquipment})
	if portrait.PaperDoll.H > portrait.Safe.H*0.70 || portrait.PaperDoll.H < portrait.Safe.H*0.50 {
		t.Fatalf("portrait paper-doll stage is not intentionally bounded: paper=%+v safe=%+v", portrait.PaperDoll, portrait.Safe)
	}
	if portrait.PaperDollCluster.H <= 0 || portrait.PaperDollCluster.H > portrait.Safe.H*0.70 {
		t.Fatalf("portrait cluster is not compact: cluster=%+v safe=%+v", portrait.PaperDollCluster, portrait.Safe)
	}
	preview := EquipmentPreviewRect(portrait)
	assertInsideRect(t, "portrait preview", 0, preview, portrait.PaperDoll)
	seen := map[uint16]bool{}
	for i, slot := range EquipmentPresentationSlots(portrait) {
		assertInsideRect(t, "portrait equipment slot", i, slot.Rect, portrait.PaperDoll)
		if slot.Rect.Intersects(preview) {
			t.Fatalf("portrait slot intersects avatar stage: %q slot=%+v preview=%+v", slot.Label, slot.Rect, preview)
		}
		if seen[slot.Location] {
			t.Fatalf("duplicate equipment location %d", slot.Location)
		}
		seen[slot.Location] = true
	}
	if len(seen) != len(model.Slots) {
		t.Fatalf("portrait slot cluster dropped slots: got=%d want=%d", len(seen), len(model.Slots))
	}

	landscape := LayoutEquipment(Viewport{Width: 2400, Height: 1080}, DefaultInventoryTokens(), model, InventoryInteractionState{Screen: ScreenEquipment})
	if landscape.DetailPanel.W < landscape.Header.W*0.30 || landscape.DetailPanel.W > landscape.Header.W*0.42 {
		t.Fatalf("landscape summary proportion is not bounded: paper=%+v detail=%+v", landscape.PaperDoll, landscape.DetailPanel)
	}
	if landscape.DetailPanel.H > 420 {
		t.Fatalf("unselected landscape summary grew into a blank pane: %+v", landscape.DetailPanel)
	}
}

func TestPhase1RInventoryKeepsPortraitFourColumnGrid(t *testing.T) {
	layout := LayoutInventory(Viewport{Width: 1080, Height: 2400}, DefaultInventoryTokens(), FixtureInventory("inventory-basic"), InventoryInteractionState{Screen: ScreenInventory})
	if layout.GridColumns != 4 {
		t.Fatalf("portrait inventory grid changed column contract: %d", layout.GridColumns)
	}
	for i, cell := range layout.Cells {
		assertInsideRect(t, "inventory cell", i, cell.Rect, layout.GridViewport)
		if cell.Rect.W < 48 || cell.Rect.H < 48 {
			t.Fatalf("inventory cell below touch target: %+v", cell.Rect)
		}
	}
}
