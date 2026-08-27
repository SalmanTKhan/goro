package mobileui

import "github.com/kivutar/goro/input"

type VendingSelectionState struct {
	SelectedIndex int
	HasSelection  bool
}

type VendingQuantityState struct {
	Open      bool
	ItemIndex uint16
	Minimum   int
	Maximum   int
	Value     int
}

func (q *VendingQuantityState) OpenFor(itemIndex uint16, maximum int) {
	if q == nil {
		return
	}
	if maximum < 1 {
		maximum = 1
	}
	*q = VendingQuantityState{Open: true, ItemIndex: itemIndex, Minimum: 1, Maximum: maximum, Value: 1}
}

func (q *VendingQuantityState) SetValue(value int) {
	if q == nil {
		return
	}
	if value < q.Minimum {
		value = q.Minimum
	}
	if value > q.Maximum {
		value = q.Maximum
	}
	q.Value = value
}

func (q *VendingQuantityState) Increment() {
	if q != nil {
		q.SetValue(q.Value + 1)
	}
}
func (q *VendingQuantityState) Decrement() {
	if q != nil {
		q.SetValue(q.Value - 1)
	}
}
func (q *VendingQuantityState) Cancel() {
	if q != nil {
		*q = VendingQuantityState{}
	}
}

func (q *VendingQuantityState) Confirm() (input.PlayerCommand, bool) {
	if q == nil || !q.Open || q.Value < 1 {
		return input.PlayerCommand{}, false
	}
	command := input.PlayerCommand{Kind: input.CommandBuyVendingItem, ItemIndex: q.ItemIndex, Quantity: q.Value}
	*q = VendingQuantityState{}
	return command, true
}

type MobileVendingController struct {
	Model    MobileVendingModel
	Viewport Viewport
	State    VendingSelectionState
	Quantity VendingQuantityState
	Scroll   InventoryScrollState
	Layout   VendingLayout
	Visible  bool
	Sink     input.CommandSink
}

func NewVendingController(model MobileVendingModel, viewport Viewport, sink input.CommandSink) *MobileVendingController {
	c := &MobileVendingController{Model: model, Viewport: viewport, Sink: sink}
	c.Visible = model.Open
	c.relayout()
	return c
}

func (c *MobileVendingController) IsOpen() bool { return c != nil && c.Visible }

func (c *MobileVendingController) Open(model MobileVendingModel) {
	if c == nil {
		return
	}
	c.Model, c.Visible = model, true
	c.State = VendingSelectionState{SelectedIndex: -1}
	c.Quantity.Cancel()
	c.Scroll = InventoryScrollState{}
	c.relayout()
}

func (c *MobileVendingController) SetModel(model MobileVendingModel) {
	if c == nil {
		return
	}
	c.Model = model
	if !model.Open {
		c.Visible = false
		c.Quantity.Cancel()
	}
	if c.State.HasSelection && c.State.SelectedIndex >= len(model.Items) {
		c.State = VendingSelectionState{SelectedIndex: -1}
	}
	c.relayout()
}

func (c *MobileVendingController) Resize(viewport Viewport) {
	if c != nil {
		c.Viewport = viewport
		c.relayout()
	}
}

func (c *MobileVendingController) Close() bool {
	if c == nil || !c.Visible {
		return false
	}
	c.Visible = false
	c.Quantity.Cancel()
	c.relayout()
	return true
}

func (c *MobileVendingController) ConsumeTouch(point input.TouchPoint) bool {
	_ = point
	return c != nil && c.Visible
}

func (c *MobileVendingController) Tap(x, y float32) bool {
	if c == nil || !c.Visible {
		return false
	}
	if c.Quantity.Open {
		return c.tapQuantity(x, y)
	}
	if c.Layout.Back.Contains(x, y) || c.Layout.Close.Contains(x, y) {
		c.Close()
		return true
	}
	for rowNumber, row := range c.Layout.Rows {
		if !row.Contains(x, y) || rowNumber >= len(c.Layout.RowIndices) {
			continue
		}
		index := c.Layout.RowIndices[rowNumber]
		if index >= 0 && index < len(c.Model.Items) {
			c.State.SelectedIndex, c.State.HasSelection = index, true
			c.relayout()
		}
		return true
	}
	if c.Layout.BuyButton.Contains(x, y) {
		item, ok := c.SelectedItem()
		if ok && item.CanBuy {
			c.Quantity.OpenFor(item.Index, vendingQuantityMaximum(item, c.Model.Zeny))
			c.relayout()
		}
		return true
	}
	return true
}

func (c *MobileVendingController) SelectedItem() (VendingItemModel, bool) {
	if c == nil || !c.State.HasSelection || c.State.SelectedIndex < 0 || c.State.SelectedIndex >= len(c.Model.Items) {
		return VendingItemModel{}, false
	}
	return c.Model.Items[c.State.SelectedIndex], true
}

func (c *MobileVendingController) Back() bool {
	if c == nil || !c.Visible {
		return false
	}
	if c.Quantity.Open {
		c.Quantity.Cancel()
		c.relayout()
		return true
	}
	return c.Close()
}

func (c *MobileVendingController) ScrollBy(delta float32) bool {
	if c == nil || !c.Visible || c.Quantity.Open {
		return false
	}
	c.Scroll.ScrollBy(delta)
	c.relayout()
	return true
}

func (c *MobileVendingController) emit(command input.PlayerCommand) {
	if c.Sink != nil {
		c.Sink.Emit(command)
	}
}

func (c *MobileVendingController) tapQuantity(x, y float32) bool {
	switch {
	case c.Layout.QuantityCancel.Contains(x, y):
		c.Quantity.Cancel()
	case c.Layout.QuantityMinus.Contains(x, y):
		c.Quantity.Decrement()
	case c.Layout.QuantityPlus.Contains(x, y):
		c.Quantity.Increment()
	case c.Layout.QuantityConfirm.Contains(x, y):
		if command, ok := c.Quantity.Confirm(); ok {
			c.emit(command)
		}
	default:
		return true
	}
	c.relayout()
	return true
}

func (c *MobileVendingController) relayout() {
	if c == nil {
		return
	}
	c.Layout = LayoutVendingScrolled(c.Viewport, len(c.Model.Items), c.Scroll.Offset)
	base := LayoutVending(c.Viewport, len(c.Model.Items))
	c.Scroll.ViewportExtent = base.ListViewport.H
	c.Scroll.ContentExtent = vendingContentExtent(len(c.Model.Items))
	c.Scroll.RowExtent = 68
	c.Scroll.SetOffset(c.Scroll.Offset)
	c.Layout = LayoutVendingScrolled(c.Viewport, len(c.Model.Items), c.Scroll.Offset)
	if c.Quantity.Open {
		c.Layout.QuantityModal, c.Layout.QuantityMinus, c.Layout.QuantityPlus, c.Layout.QuantityConfirm, c.Layout.QuantityCancel = LayoutVendingQuantity(c.Viewport)
	}
}

func vendingQuantityMaximum(item VendingItemModel, zeny int64) int {
	maximum := item.Quantity
	if maximum < 1 {
		return 1
	}
	if item.Price > 0 {
		byZeny := int(zeny / item.Price)
		if byZeny < maximum {
			maximum = byZeny
		}
	}
	if maximum < 1 {
		maximum = 1
	}
	return maximum
}
