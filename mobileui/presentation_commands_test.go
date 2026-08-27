package mobileui

import "testing"

func TestInventoryPresentationUsesGridWorkspaceAndCompactChrome(t *testing.T) {
	for _, viewport := range []Viewport{{Width: 390, Height: 844}, {Width: 1080, Height: 2400}, {Width: 2400, Height: 1080}} {
		model := FixtureInventory("inventory-basic")
		state := InventoryInteractionState{Screen: ScreenInventory}
		layout := LayoutInventory(viewport, DefaultInventoryTokens(), model, state)
		commands := InventoryPresentationCommands(layout, model, state)
		if len(layout.Tabs) != 5 {
			t.Fatalf("viewport %+v: got %d category tabs", viewport, len(layout.Tabs))
		}
		if layout.CategoryButton.W != 0 || layout.StorageButton.W != 0 {
			t.Fatalf("viewport %+v: obsolete inventory bars remain: category=%+v storage=%+v", viewport, layout.CategoryButton, layout.StorageButton)
		}
		if layout.GridViewport.W <= 0 || layout.GridViewport.H <= 0 || layout.GridViewport.Right() > layout.Safe.Right()+0.01 {
			t.Fatalf("viewport %+v: invalid grid workspace: grid=%+v safe=%+v", viewport, layout.GridViewport, layout.Safe)
		}
		if layout.GridColumns < 4 {
			t.Fatalf("viewport %+v: grid has only %d columns", viewport, layout.GridColumns)
		}
		for _, item := range model.Items {
			found := false
			for _, command := range commands {
				if command.Owner == "Inventory/Item/"+item.DisplayName {
					found = true
					if command.ImageKey == "" {
						t.Fatalf("viewport %+v: item %q has no sprite command", viewport, item.DisplayName)
					}
				}
			}
			if !found {
				t.Fatalf("viewport %+v: item %q missing from presentation commands", viewport, item.DisplayName)
			}
		}
		for _, command := range commands {
			if command.Owner == "Inventory/Equipment" && command.Text != "EQUIP" {
				t.Fatalf("viewport %+v: unexpected equipment command: %+v", viewport, command)
			}
			if command.Owner == "Inventory/Storage" || command.Text == "SELECT AN ITEM TO STORE" {
				t.Fatalf("viewport %+v: obsolete storage command: %+v", viewport, command)
			}
		}
	}
}

func TestEquipmentPresentationHasOneCommandPerSlot(t *testing.T) {
	for _, viewport := range []Viewport{{Width: 390, Height: 844}, {Width: 1080, Height: 2400}, {Width: 2400, Height: 1080}, FoldOuterViewport()} {
		inventory := FixtureInventory("equipment-full")
		model := ProjectEquipment(inventory)
		layout := LayoutEquipment(viewport, DefaultInventoryTokens(), model, InventoryInteractionState{Screen: ScreenEquipment})
		commands := EquipmentPresentationCommands(layout, model, InventoryInteractionState{Screen: ScreenEquipment})
		counts := map[uint16]int{}
		for _, command := range commands {
			if len(command.Owner) < len("Equipment/Slot/") || command.Owner[:len("Equipment/Slot/")] != "Equipment/Slot/" {
				continue
			}
			for _, slot := range model.Slots {
				if command.Owner == "Equipment/Slot/"+slot.Label {
					counts[slot.Location]++
				}
			}
		}
		if len(counts) != len(model.Slots) {
			t.Fatalf("viewport %+v: missing equipment commands: got=%d want=%d commands=%+v", viewport, len(counts), len(model.Slots), commands)
		}
		for _, slot := range model.Slots {
			if counts[slot.Location] != 1 {
				t.Fatalf("viewport %+v: location %d rendered %d times", viewport, slot.Location, counts[slot.Location])
			}
		}
	}
}
