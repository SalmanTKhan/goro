package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestInventorySelectFilterAndUse(t *testing.T) {
	model := FixtureInventory("inventory-basic")
	var commands input.CommandBuffer
	c := NewInventoryController(model, Viewport{Width: 2400, Height: 1080}, &commands)
	first := c.Layout.Cells[0]
	c.Tap(first.Rect.X+2, first.Rect.Y+2)
	if !c.State.Selection.HasSelection || c.State.Selection.SelectedIndex != 1 || !c.State.DetailOpen {
		t.Fatalf("selection not updated: %+v", c.State.Selection)
	}
	if !c.ConsumeTouch(input.TouchPoint{X: 5, Y: 5}) {
		t.Fatal("screen touch was not consumed")
	}
	c.Tap(c.Layout.PrimaryAction.X+2, c.Layout.PrimaryAction.Y+2)
	if got := commands.Commands(); len(got) != 1 || got[0].Kind != input.CommandUseItem || got[0].ItemIndex != 1 {
		t.Fatalf("unexpected use command: %+v", got)
	}
	c.Tap(c.Layout.Tabs[1].Rect.X+2, c.Layout.Tabs[1].Rect.Y+2)
	if c.State.Category != InventoryCategoryEquipment || len(c.Layout.Cells) == 0 {
		t.Fatalf("equipment filter not applied: category=%v cells=%d", c.State.Category, len(c.Layout.Cells))
	}
}

func TestInventoryEquipDropQuantityAndCancel(t *testing.T) {
	model := FixtureInventory("inventory-basic")
	model.Items[1].Equipped = false
	model.Items[1].EquipmentSlot = 0
	var commands input.CommandBuffer
	c := NewInventoryController(model, Viewport{Width: 2400, Height: 1080}, &commands)
	for _, cell := range c.Layout.Cells {
		if cell.Index == 3 {
			c.Tap(cell.Rect.X+1, cell.Rect.Y+1)
			break
		}
	}
	c.Tap(c.Layout.PrimaryAction.X+1, c.Layout.PrimaryAction.Y+1)
	if got := commands.Commands(); len(got) != 1 || got[0].Kind != input.CommandEquipItem || got[0].ItemIndex != 3 {
		t.Fatalf("unexpected equip command: %+v", got)
	}
	for _, cell := range c.Layout.Cells {
		if cell.Index == 1 {
			c.Tap(cell.Rect.X+1, cell.Rect.Y+1)
			break
		}
	}
	c.Tap(c.Layout.SecondaryAction.X+1, c.Layout.SecondaryAction.Y+1)
	if !c.State.Quantity.Open {
		t.Fatal("drop did not open quantity state")
	}
	c.Tap(c.Layout.QuantityCancel.X+1, c.Layout.QuantityCancel.Y+1)
	if c.State.Quantity.Open || len(commands.Commands()) != 1 {
		t.Fatalf("quantity cancel changed state/commands: %+v", commands.Commands())
	}
}

func TestInventoryDropConfirmAndNavigationPrecedence(t *testing.T) {
	model := FixtureInventory("inventory-large-quantity")
	var commands input.CommandBuffer
	targeting := input.SkillTargetState{}
	c := NewInventoryController(model, Viewport{Width: 2400, Height: 1080}, &commands)
	for _, cell := range c.Layout.Cells {
		if cell.Index == 1 {
			c.Tap(cell.Rect.X+1, cell.Rect.Y+1)
			break
		}
	}
	c.Tap(c.Layout.SecondaryAction.X+1, c.Layout.SecondaryAction.Y+1)
	c.State.Quantity.SetValue(7)
	c.Tap(c.Layout.QuantityConfirm.X+1, c.Layout.QuantityConfirm.Y+1)
	if got := commands.Commands(); len(got) != 1 || got[0].Kind != input.CommandDropItem || got[0].Quantity != 7 {
		t.Fatalf("unexpected drop command: %+v", got)
	}
	c.Targeting = &targeting
	targeting.BeginActor(42, 1)
	if !c.Back() || targeting.Mode != input.SkillTargetIdle || c.State.Screen != ScreenInventory {
		t.Fatal("targeting did not cancel before navigation")
	}
	if got := commands.Commands(); len(got) != 2 || got[1].Kind != input.CommandCancelAction {
		t.Fatalf("missing cancel command: %+v", got)
	}
	if !c.Back() || c.State.DetailOpen || c.State.Screen != ScreenInventory {
		t.Fatal("detail did not close before screen navigation")
	}
	if !c.Back() || c.State.Screen != ScreenWorldHUD {
		t.Fatal("inventory did not navigate to world")
	}
	if c.ConsumeTouch(input.TouchPoint{X: 20, Y: 20}) {
		t.Fatal("world touch was still consumed")
	}
}

func TestEquipmentNavigationAndEmptySlots(t *testing.T) {
	c := NewInventoryController(FixtureInventory("inventory-basic"), Viewport{Width: 2400, Height: 1080}, nil)
	c.Tap(c.Layout.EquipmentButton.X+1, c.Layout.EquipmentButton.Y+1)
	if c.State.Screen != ScreenEquipment || len(c.Layout.EquipmentSlots) == 0 {
		t.Fatal("equipment screen did not open")
	}
	if !c.ConsumeTouch(input.TouchPoint{X: 1, Y: 1}) {
		t.Fatal("equipment screen did not consume touch")
	}
	if !c.Back() || c.State.Screen != ScreenInventory {
		t.Fatal("equipment back did not return to inventory")
	}
}

func TestEquipmentSelectionOpensPaperDollDetail(t *testing.T) {
	c := NewInventoryController(FixtureInventory("equipment-full"), Viewport{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24}, nil)
	if !c.Tap(c.Layout.EquipmentButton.X+1, c.Layout.EquipmentButton.Y+1) || c.State.Screen != ScreenEquipment {
		t.Fatal("equipment screen did not open from the portrait inventory")
	}
	if c.Layout.PaperDoll.W <= 0 || len(c.Layout.PaperDollSlots) == 0 {
		t.Fatalf("paper doll was not laid out: %+v", c.Layout)
	}
	if len(c.Layout.EquipmentSlots) == 0 {
		t.Fatal("equipment list has no touchable slots")
	}
	slot := c.Layout.EquipmentSlots[0]
	if !c.Tap(slot.Rect.X+2, slot.Rect.Y+2) || !c.State.DetailOpen || c.Layout.DetailPanel.W <= 0 {
		t.Fatalf("equipment slot did not open its detail sheet: state=%+v layout=%+v", c.State, c.Layout)
	}
}
