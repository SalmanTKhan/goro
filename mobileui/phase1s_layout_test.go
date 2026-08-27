package mobileui

import "testing"

func TestItemDetailContentAndActionsNeverOverlap(t *testing.T) {
	viewports := []Viewport{
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
		{Width: 1080, Height: 2400, SafeTop: 48, SafeBottom: 48},
		{Width: 1920, Height: 1080},
		{Width: 2400, Height: 1080},
		{Width: 2560, Height: 1440},
		{Width: 2268, Height: 832, SafeTop: 18, SafeBottom: 18, SafeLeft: 24, SafeRight: 24},
	}
	for _, viewport := range viewports {
		inventoryModel := FixtureInventory("inventory-long-names")
		inventoryState := InventoryInteractionState{Screen: ScreenInventory}
		if !inventoryState.Select(inventoryModel, inventoryModel.Items[0].Index) {
			t.Fatalf("failed to select inventory fixture at %+v", viewport)
		}
		inventory := LayoutInventory(viewport, DefaultInventoryTokens(), inventoryModel, inventoryState)
		assertDetailLayout(t, viewport, inventory, "inventory")

		equipmentModel := FixtureEquipment("equipment-full")
		equipmentState := InventoryInteractionState{
			Screen:     ScreenEquipment,
			Selection:  InventorySelectionModel{HasSelection: true, SelectedIndex: equipmentModel.Slots[3].ItemIndex, Detail: ItemDetailModel{Visible: true, Item: equipmentModel.Slots[3].Item, PrimaryAction: "Unequip", PrimaryEnabled: true, SecondaryAction: "Drop", SecondaryEnabled: true}},
			DetailOpen: true,
		}
		equipment := LayoutEquipment(viewport, DefaultInventoryTokens(), equipmentModel, equipmentState)
		assertDetailLayout(t, viewport, equipment, "equipment")
	}
}

func assertDetailLayout(t *testing.T, viewport Viewport, layout MobileInventoryLayout, name string) {
	t.Helper()
	assertInside(t, name+" detail panel", layout.DetailPanel, layout.Safe)
	assertInside(t, name+" detail icon", layout.DetailIcon, layout.DetailPanel)
	assertInside(t, name+" detail title", layout.DetailTitle, layout.DetailPanel)
	assertInside(t, name+" detail metadata", layout.DetailMeta, layout.DetailPanel)
	assertInside(t, name+" detail description", layout.DetailDescription, layout.DetailPanel)
	assertInside(t, name+" primary action", layout.PrimaryAction, layout.DetailPanel)
	assertInside(t, name+" secondary action", layout.SecondaryAction, layout.DetailPanel)
	assertTouchTarget(t, name+" primary action", layout.PrimaryAction)
	assertTouchTarget(t, name+" secondary action", layout.SecondaryAction)
	if layout.DetailTitle.Intersects(layout.DetailMeta) || layout.DetailTitle.Intersects(layout.DetailDescription) || layout.DetailMeta.Intersects(layout.DetailDescription) {
		t.Fatalf("%s text regions overlap at viewport=%+v: title=%+v meta=%+v description=%+v", name, viewport, layout.DetailTitle, layout.DetailMeta, layout.DetailDescription)
	}
	if layout.DetailDescription.Intersects(layout.PrimaryAction) || layout.DetailDescription.Intersects(layout.SecondaryAction) {
		t.Fatalf("%s description collides with actions at viewport=%+v: description=%+v primary=%+v secondary=%+v", name, viewport, layout.DetailDescription, layout.PrimaryAction, layout.SecondaryAction)
	}
	if layout.PrimaryAction.Intersects(layout.SecondaryAction) {
		t.Fatalf("%s actions overlap at viewport=%+v: primary=%+v secondary=%+v", name, viewport, layout.PrimaryAction, layout.SecondaryAction)
	}
}
