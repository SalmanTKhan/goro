package mobileui

import "testing"

func TestInventoryLayoutFitsLandscapeViewports(t *testing.T) {
	model := FixtureInventory("inventory-full")
	viewports := []Viewport{
		{Width: 1920, Height: 1080}, {Width: 2400, Height: 1080}, {Width: 2560, Height: 1440},
		{Width: 1280, Height: 800}, FoldOuterViewport(), {Width: 2400, Height: 1080, SafeTop: 48, SafeRight: 32, SafeBottom: 36, SafeLeft: 32},
	}
	for _, viewport := range viewports {
		state := InventoryInteractionState{Screen: ScreenInventory}
		layout := LayoutInventory(viewport, DefaultInventoryTokens(), model, state)
		for i, tab := range layout.Tabs {
			assertInsideRect(t, "tab", i, tab.Rect, layout.Safe)
			if tab.Rect.W < DefaultInventoryTokens().MinTouchTarget || tab.Rect.H < DefaultInventoryTokens().MinTouchTarget {
				t.Fatalf("tab %d is too small: %+v", i, tab.Rect)
			}
		}
		assertInsideRect(t, "grid", 0, layout.GridViewport, layout.Safe)
		if layout.DetailPanel.W > 0 && layout.DetailPanel.H > 0 {
			assertInsideRect(t, "detail", 0, layout.DetailPanel, layout.Safe)
		}
		if layout.DetailPanel.W > 0 && layout.DetailPanel.H > 0 && layout.GridViewport.Intersects(layout.DetailPanel) {
			t.Fatal("grid intersects detail panel")
		}
		for i, cell := range layout.Cells {
			assertInsideRect(t, "cell", i, cell.Rect, layout.GridViewport)
			if cell.Rect.W < DefaultInventoryTokens().MinTouchTarget || cell.Rect.H < DefaultInventoryTokens().MinTouchTarget {
				t.Fatalf("cell %d is too small", i)
			}
		}
	}
}

func TestFoldOuterInventoryUsesBoundedContent(t *testing.T) {
	model := FixtureInventory("inventory-long-names")
	tokens := DefaultInventoryTokens()
	state := InventoryInteractionState{Screen: ScreenInventory}
	layout := LayoutInventory(FoldOuterViewport(), tokens, model, state)
	if layout.Header.W != tokens.ContentMaxWidth || layout.Header.X != 334 {
		t.Fatalf("Fold inventory rail is not bounded/centered: %+v", layout.Header)
	}
	if layout.GridViewport.W > 1100 || layout.DetailPanel.W > 520 {
		t.Fatalf("Fold inventory panels grew with width: grid=%+v detail=%+v", layout.GridViewport, layout.DetailPanel)
	}
	if layout.GridViewport.Intersects(layout.DetailPanel) {
		t.Fatal("Fold inventory grid/detail overlap")
	}
	model.Items[0].DisplayName = "An intentionally very long item name that must be clipped or wrapped by the renderer"
	if LayoutInventory(FoldOuterViewport(), tokens, model, state).DetailPanel != layout.DetailPanel {
		t.Fatal("long item name changed Fold detail geometry")
	}
	quantityState := state
	quantityState.Quantity.Open = true
	quantity := LayoutInventory(FoldOuterViewport(), tokens, model, quantityState)
	if quantity.QuantityModal.X < quantity.Safe.X || quantity.QuantityModal.Right() > quantity.Safe.Right() || quantity.QuantityModal.Y < quantity.Safe.Y || quantity.QuantityModal.Bottom() > quantity.Safe.Bottom() {
		t.Fatalf("Fold quantity modal escapes safe area: %+v", quantity.QuantityModal)
	}
	for name, button := range map[string]Rect{"minus": quantity.QuantityMinus, "plus": quantity.QuantityPlus, "confirm": quantity.QuantityConfirm, "cancel": quantity.QuantityCancel} {
		if button.W < 48 || button.H < 48 {
			t.Fatalf("Fold quantity %s below touch target: %+v", name, button)
		}
		assertInsideRect(t, "quantity "+name, 0, button, quantity.QuantityModal)
	}
	selected := state
	selected.Selection = InventorySelectionModel{HasSelection: true, Detail: ItemDetailModel{Visible: true, Item: model.Items[0]}}
	selectedLayout := LayoutInventory(FoldOuterViewport(), tokens, model, selected)
	assertInsideRect(t, "primary action", 0, selectedLayout.PrimaryAction, selectedLayout.Safe)
	assertInsideRect(t, "secondary action", 0, selectedLayout.SecondaryAction, selectedLayout.Safe)
}

func TestEquipmentLayoutSlotsDoNotOverlap(t *testing.T) {
	model := FixtureEquipment("equipment-full")
	layout := LayoutEquipment(Viewport{Width: 2400, Height: 1080, SafeTop: 48, SafeBottom: 36}, DefaultInventoryTokens(), model, InventoryInteractionState{Screen: ScreenEquipment})
	for i, slot := range layout.EquipmentSlots {
		assertInsideRect(t, "equipment slot", i, slot.Rect, layout.Safe)
		if slot.Rect.W < 48 || slot.Rect.H < 48 {
			t.Fatalf("equipment slot %d below touch target: %+v", i, slot.Rect)
		}
		for j := 0; j < i; j++ {
			if slot.Rect.Intersects(layout.EquipmentSlots[j].Rect) {
				t.Fatalf("equipment slots %d and %d overlap", i, j)
			}
		}
	}
}

func TestInventoryScrollIsClampedAndPaged(t *testing.T) {
	model := FixtureInventory("inventory-full")
	state := InventoryInteractionState{Category: InventoryCategoryAll}
	layout := LayoutInventory(Viewport{Width: 1280, Height: 800}, DefaultInventoryTokens(), model, state)
	scroll := ScrollExtent(model, state, DefaultInventoryTokens(), layout.GridViewport)
	if scroll.ContentExtent <= scroll.ViewportExtent {
		t.Fatal("full fixture did not exceed viewport")
	}
	scroll.SetOffset(999999)
	if scroll.Offset != scroll.MaxOffset() {
		t.Fatalf("scroll not clamped at max: %+v", scroll)
	}
	if scroll.Page() <= 0 {
		t.Fatalf("scroll page did not advance: %+v", scroll)
	}
	scroll.SetOffset(-1)
	if scroll.Offset != 0 {
		t.Fatalf("scroll not clamped at zero: %+v", scroll)
	}
}

func TestPortraitShortListsDoNotPaintDeadSpace(t *testing.T) {
	viewport := Viewport{Width: 1080, Height: 2400, SafeTop: 48, SafeBottom: 48}
	layout := LayoutInventory(viewport, DefaultInventoryTokens(), FixtureInventory("inventory-basic"), InventoryInteractionState{Screen: ScreenInventory})
	if layout.GridViewport.H >= layout.Safe.H/2 {
		t.Fatalf("short portrait inventory still paints dead space: grid=%+v safe=%+v", layout.GridViewport, layout.Safe)
	}

	character := LayoutCharacter(viewport, FixtureCharacter("character-rich"))
	if character.Panel.H < character.Safe.H/2 {
		t.Fatalf("short character profile did not claim the mobile workspace: panel=%+v safe=%+v", character.Panel, character.Safe)
	}

	skills := LayoutSkills(viewport, FixtureSkills("skills-basic"), 0)
	if skills.Panel.H < skills.Safe.H/2 || skills.Detail.W != 0 {
		t.Fatalf("unselected short skill list did not claim the mobile workspace: panel=%+v detail=%+v safe=%+v", skills.Panel, skills.Detail, skills.Safe)
	}
}

func TestPortraitInventoryUsesDenseSpriteGrid(t *testing.T) {
	model := FixtureInventory("inventory-basic")
	for _, viewport := range []Viewport{{Width: 390, Height: 844}, {Width: 1080, Height: 2400}} {
		layout := LayoutInventory(viewport, DefaultInventoryTokens(), model, InventoryInteractionState{Screen: ScreenInventory})
		if layout.GridColumns < 4 {
			t.Fatalf("portrait inventory did not use a dense grid at %+v: columns=%d layout=%+v", viewport, layout.GridColumns, layout)
		}
		for i, cell := range layout.Cells {
			if cell.Rect.W < 48 || cell.Rect.H < 48 {
				t.Fatalf("portrait inventory cell %d is below touch target at %+v: %+v", i, viewport, cell.Rect)
			}
			assertInsideRect(t, "portrait sprite cell", i, cell.Rect, layout.GridViewport)
		}
	}
}

func assertInsideRect(t *testing.T, label string, index int, child, parent Rect) {
	t.Helper()
	if child.X < parent.X || child.Y < parent.Y || child.Right() > parent.Right()+0.01 || child.Bottom() > parent.Bottom()+0.01 {
		t.Fatalf("%s %d %+v escapes %+v", label, index, child, parent)
	}
}
