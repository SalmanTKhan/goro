package mobileui

// MobileRenderCommand is the renderer-neutral command description used for
// integration diagnostics. Android still owns the actual draw calls, but it
// consumes the same layout/model inputs represented here.
type MobileRenderCommand struct {
	Owner    string
	Rect     Rect
	Text     string
	ImageKey string
	Layer    uint8
}

func InventoryPresentationCommands(layout MobileInventoryLayout, model MobileInventoryModel, state InventoryInteractionState) []MobileRenderCommand {
	commands := []MobileRenderCommand{
		{Owner: "Inventory/Header", Rect: layout.Header, Text: "INVENTORY", Layer: 1},
		{Owner: "Inventory/Grid", Rect: layout.GridViewport, Layer: 1},
	}
	for _, tab := range layout.Tabs {
		commands = append(commands, MobileRenderCommand{Owner: "Inventory/Tab/" + tab.Category.String(), Rect: tab.Rect, Text: tab.Category.String(), Layer: 1})
	}
	if layout.EquipmentButton.W > 0 {
		commands = append(commands, MobileRenderCommand{Owner: "Inventory/Equipment", Rect: layout.EquipmentButton, Text: "EQUIP", Layer: 1})
	}
	for _, cell := range layout.Cells {
		item, ok := inventoryItemByIndex(model.Items, cell.Index)
		if !ok {
			continue
		}
		commands = append(commands, MobileRenderCommand{Owner: "Inventory/Item/" + item.DisplayName, Rect: cell.Rect, Text: item.DisplayName, ImageKey: item.IconKey, Layer: 2})
	}
	if state.Selection.HasSelection && layout.DetailPanel.W > 0 {
		commands = append(commands, MobileRenderCommand{Owner: "Inventory/Detail", Rect: layout.DetailPanel, Text: "ITEM DETAILS", Layer: 3})
	}
	return commands
}

func EquipmentPresentationCommands(layout MobileInventoryLayout, model MobileEquipmentModel, state InventoryInteractionState) []MobileRenderCommand {
	commands := []MobileRenderCommand{
		{Owner: "Equipment/Header", Rect: layout.Header, Text: "EQUIPMENT", Layer: 1},
		{Owner: "Equipment/PaperDoll", Rect: layout.PaperDoll, Text: "PAPER DOLL", Layer: 1},
	}
	if layout.PaperDoll.W > 0 {
		commands = append(commands, MobileRenderCommand{Owner: "Equipment/Character", Rect: EquipmentPreviewRect(layout), ImageKey: "character-preview", Layer: 2})
	}
	seen := map[uint16]bool{}
	for _, slot := range EquipmentPresentationSlots(layout) {
		if seen[slot.Location] {
			continue
		}
		seen[slot.Location] = true
		imageKey := ""
		text := slot.Label
		for _, equipment := range model.Slots {
			if equipment.Location != slot.Location || !equipment.HasItem {
				continue
			}
			imageKey = equipment.Item.IconKey
			text = equipment.Item.DisplayName
			break
		}
		commands = append(commands, MobileRenderCommand{Owner: "Equipment/Slot/" + slot.Label, Rect: slot.Rect, Text: text, ImageKey: imageKey, Layer: 2})
	}
	if state.Selection.HasSelection && layout.DetailPanel.W > 0 {
		commands = append(commands, MobileRenderCommand{Owner: "Equipment/Detail", Rect: layout.DetailPanel, Text: "ITEM DETAILS", Layer: 3})
	}
	return commands
}

func inventoryItemByIndex(items []InventoryItemModel, index uint16) (InventoryItemModel, bool) {
	for _, item := range items {
		if item.Index == index {
			return item, true
		}
	}
	return InventoryItemModel{}, false
}
