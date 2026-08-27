package mobileui

import "github.com/kivutar/goro/input"

type MobileInventoryController struct {
	Model     MobileInventoryModel
	Equipment MobileEquipmentModel
	State     InventoryInteractionState
	Layout    MobileInventoryLayout
	Viewport  Viewport
	Tokens    InventoryTokens
	Sink      input.CommandSink
	Targeting *input.SkillTargetState
}

func NewInventoryController(model MobileInventoryModel, viewport Viewport, sink input.CommandSink) *MobileInventoryController {
	c := &MobileInventoryController{Model: model, Equipment: ProjectEquipment(model), Viewport: viewport, Tokens: DefaultInventoryTokens(), Sink: sink}
	c.State.Screen = ScreenInventory
	c.relayout()
	return c
}

func (c *MobileInventoryController) SetModel(model MobileInventoryModel) {
	if c == nil {
		return
	}
	c.Model = model
	c.Equipment = ProjectEquipment(model)
	if c.State.Selection.HasSelection && !containsItem(model.Items, c.State.Selection.SelectedIndex) {
		c.State.Selection = InventorySelectionModel{}
		c.State.DetailOpen = false
	}
	c.relayout()
}

func (c *MobileInventoryController) Open(screen Screen) {
	if c == nil {
		return
	}
	c.State.Screen = screen
	c.State.Quantity.Cancel()
	c.relayout()
}

func (c *MobileInventoryController) ConsumeTouch(point input.TouchPoint) bool {
	return c != nil && c.State.Screen != ScreenWorldHUD && c.Layout.Safe.Contains(float32(point.X), float32(point.Y))
}

func (c *MobileInventoryController) Tap(x, y float32) bool {
	if c == nil || c.State.Screen == ScreenWorldHUD {
		return false
	}
	if c.State.Quantity.Open {
		return c.tapQuantity(x, y)
	}
	if c.Layout.BackButton.Contains(x, y) {
		c.Back()
		return true
	}
	if c.State.Screen == ScreenInventory {
		return c.tapInventory(x, y)
	}
	if c.State.Screen == ScreenEquipment {
		return c.tapEquipment(x, y)
	}
	return true
}

func (c *MobileInventoryController) tapInventory(x, y float32) bool {
	if c.Layout.CategoryButton.Contains(x, y) {
		c.State.CategoryMenuOpen = !c.State.CategoryMenuOpen
		c.relayout()
		return true
	}
	for _, option := range c.Layout.CategoryOptions {
		if option.Rect.Contains(x, y) {
			c.State.SetCategory(option.Category)
			c.relayout()
			return true
		}
	}
	for _, tab := range c.Layout.Tabs {
		if tab.Rect.Contains(x, y) {
			c.State.SetCategory(tab.Category)
			c.relayout()
			return true
		}
	}
	for _, cell := range c.Layout.Cells {
		if cell.Rect.Contains(x, y) {
			c.State.Select(c.Model, cell.Index)
			c.Model.Selection = c.State.Selection
			c.relayout()
			return true
		}
	}
	if c.Layout.EquipmentButton.Contains(x, y) {
		c.State.Screen = ScreenEquipment
		c.relayout()
		return true
	}
	if c.State.Selection.HasSelection && c.Layout.StorageButton.Contains(x, y) {
		c.emit(input.PlayerCommand{Kind: input.CommandDepositItem, ItemIndex: c.State.Selection.SelectedIndex, Quantity: c.State.Selection.Detail.Item.Quantity})
		return true
	}
	if c.State.Selection.HasSelection && c.Layout.PrimaryAction.Contains(x, y) {
		return c.primaryAction()
	}
	if c.State.Selection.HasSelection && c.Layout.SecondaryAction.Contains(x, y) {
		c.State.Quantity.OpenFor(c.State.Selection.Detail.Item)
		c.relayout()
		return true
	}
	return true
}

func (c *MobileInventoryController) tapEquipment(x, y float32) bool {
	for _, slot := range EquipmentPresentationSlots(c.Layout) {
		if !slot.Rect.Contains(x, y) {
			continue
		}
		for _, equipment := range c.Equipment.Slots {
			if equipment.Location == slot.Location && equipment.HasItem {
				c.State.Select(c.Model, equipment.ItemIndex)
				c.Model.Selection = c.State.Selection
				c.relayout()
				return true
			}
		}
		return true
	}
	if c.State.Selection.HasSelection && c.Layout.PrimaryAction.Contains(x, y) {
		return c.primaryAction()
	}
	if c.State.Selection.HasSelection && c.Layout.SecondaryAction.Contains(x, y) {
		return c.Back()
	}
	return true
}

func (c *MobileInventoryController) tapQuantity(x, y float32) bool {
	switch {
	case c.Layout.QuantityCancel.Contains(x, y):
		c.State.Quantity.Cancel()
	case c.Layout.QuantityMinus.Contains(x, y):
		c.State.Quantity.Decrement()
	case c.Layout.QuantityPlus.Contains(x, y):
		c.State.Quantity.Increment()
	case c.Layout.QuantityConfirm.Contains(x, y):
		if command, ok := c.State.Quantity.Confirm(); ok {
			c.emit(command)
		}
	default:
		return true
	}
	c.relayout()
	return true
}

func (c *MobileInventoryController) primaryAction() bool {
	item := c.State.Selection.Detail.Item
	switch {
	case item.Equipped:
		c.emit(input.PlayerCommand{Kind: input.CommandUnequipItem, ItemIndex: item.Index})
	case item.Equippable:
		c.emit(input.PlayerCommand{Kind: input.CommandEquipItem, ItemIndex: item.Index, EquipmentSlot: item.EquipmentSlot})
	case item.Usable:
		c.emit(input.PlayerCommand{Kind: input.CommandUseItem, ItemIndex: item.Index})
	default:
		return true
	}
	return true
}

func (c *MobileInventoryController) ScrollBy(delta float32) bool {
	if c == nil || (c.State.Screen != ScreenInventory && c.State.Screen != ScreenEquipment) || c.State.Quantity.Open {
		return false
	}
	if c.State.Screen == ScreenEquipment {
		extent := EquipmentScrollExtent(c.Layout, len(c.Equipment.Slots), c.State, c.Tokens)
		extent.ScrollBy(delta)
		c.State.Scroll.Offset = extent.Offset
	} else {
		c.State.Scroll.ScrollBy(delta)
	}
	c.relayout()
	return true
}

func (c *MobileInventoryController) Back() bool {
	if c == nil {
		return false
	}
	if c.Targeting != nil && c.Targeting.Mode != input.SkillTargetIdle {
		c.Targeting.Cancel()
		c.emit(input.PlayerCommand{Kind: input.CommandCancelAction})
		return true
	}
	if c.State.Quantity.Open {
		c.State.Quantity.Cancel()
		c.relayout()
		return true
	}
	if c.State.DetailOpen {
		c.State.DetailOpen = false
		c.State.Selection = InventorySelectionModel{}
		c.Model.Selection = c.State.Selection
		c.relayout()
		return true
	}
	if c.State.Screen == ScreenEquipment {
		c.State.Screen = ScreenInventory
		c.relayout()
		return true
	}
	if c.State.Screen == ScreenInventory {
		c.State.Screen = ScreenWorldHUD
		c.relayout()
		return true
	}
	return false
}

func (c *MobileInventoryController) relayout() {
	if c.State.Screen == ScreenEquipment {
		base := LayoutEquipment(c.Viewport, c.Tokens, c.Equipment, c.State)
		extent := EquipmentScrollExtent(base, len(c.Equipment.Slots), c.State, c.Tokens)
		c.State.Scroll.ViewportExtent, c.State.Scroll.ContentExtent, c.State.Scroll.RowExtent = extent.ViewportExtent, extent.ContentExtent, extent.RowExtent
		c.State.Scroll.SetOffset(c.State.Scroll.Offset)
		c.Layout = LayoutEquipment(c.Viewport, c.Tokens, c.Equipment, c.State)
		return
	}
	if c.State.Screen == ScreenInventory {
		initial := LayoutInventory(c.Viewport, c.Tokens, c.Model, c.State)
		scroll := ScrollExtentForLayout(c.Model, c.State, c.Tokens, initial)
		c.State.Scroll.ViewportExtent, c.State.Scroll.ContentExtent, c.State.Scroll.RowExtent = scroll.ViewportExtent, scroll.ContentExtent, scroll.RowExtent
		c.State.Scroll.SetOffset(c.State.Scroll.Offset)
		c.Layout = LayoutInventory(c.Viewport, c.Tokens, c.Model, c.State)
		return
	}
	c.Layout = MobileInventoryLayout{Safe: c.Viewport.SafeRect()}
}

func (c *MobileInventoryController) emit(command input.PlayerCommand) {
	if c.Sink != nil {
		c.Sink.Emit(command)
	}
}
