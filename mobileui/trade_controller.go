package mobileui

import "github.com/kivutar/goro/input"

type MobileTradeController struct {
	Model     MobileTradeModel
	Layout    TradeLayout
	Viewport  Viewport
	State     TradeInteractionState
	Visible   bool
	Sink      input.CommandSink
	Targeting *input.SkillTargetState
}

func NewTradeController(model MobileTradeModel, viewport Viewport, sink input.CommandSink) *MobileTradeController {
	c := &MobileTradeController{Model: model, Viewport: viewport, Sink: sink}
	c.SetModel(model)
	return c
}

func (c *MobileTradeController) SetModel(model MobileTradeModel) {
	if c == nil {
		return
	}
	c.Model = model
	if model.Open || model.PendingRequest != nil {
		c.Visible = true
	} else {
		c.Visible = false
		c.State.Quantity.Cancel()
	}
	c.relayout()
}

func (c *MobileTradeController) Resize(viewport Viewport) {
	if c != nil {
		c.Viewport = viewport
		c.relayout()
	}
}

func (c *MobileTradeController) IsOpen() bool {
	return c != nil && c.Visible && (c.Model.Open || c.Model.PendingRequest != nil)
}

func (c *MobileTradeController) ConsumeTouch(point input.TouchPoint) bool {
	return c != nil && c.IsOpen() && c.Layout.Safe.Contains(float32(point.X), float32(point.Y))
}

func (c *MobileTradeController) Tap(x, y float32) bool {
	if c == nil || !c.IsOpen() {
		return false
	}
	if c.Model.PendingRequest != nil {
		if c.Layout.RequestAccept.Contains(x, y) {
			c.respondRequest(true)
		} else if c.Layout.RequestDecline.Contains(x, y) {
			c.respondRequest(false)
		}
		return true
	}
	if c.State.Quantity.Open {
		return c.tapQuantity(x, y)
	}
	if c.Layout.BackButton.Contains(x, y) || c.Layout.Cancel.Contains(x, y) {
		c.cancel()
		return true
	}
	for _, row := range c.Layout.InventoryRows {
		if !row.Rect.Contains(x, y) || row.Index < 0 || row.Index >= len(c.Model.Inventory) {
			continue
		}
		item := c.Model.Inventory[row.Index]
		if item.CanAdd && !item.Offered && !item.Pending {
			c.State.Quantity.OpenItem(item.Item)
			c.relayout()
		}
		return true
	}
	if c.Layout.AddZeny.Contains(x, y) {
		if c.Model.CanAddZeny {
			c.State.Quantity.OpenZeny(c.Model.AvailableZeny)
			c.relayout()
		}
		return true
	}
	if c.Layout.Conclude.Contains(x, y) {
		if c.Model.CanConclude {
			c.emit(input.PlayerCommand{Kind: input.CommandTradeConclude})
		}
		return true
	}
	if c.Layout.Commit.Contains(x, y) {
		if c.Model.CanCommit {
			c.emit(input.PlayerCommand{Kind: input.CommandTradeCommit})
		}
		return true
	}
	return true
}

func (c *MobileTradeController) tapQuantity(x, y float32) bool {
	switch {
	case c.Layout.QuantityCancel.Contains(x, y):
		c.State.Quantity.Cancel()
	case c.Layout.QuantityMinus.Contains(x, y):
		c.State.Quantity.Decrement()
	case c.Layout.QuantityPlus.Contains(x, y):
		c.State.Quantity.Increment()
	case c.Layout.QuantityConfirm.Contains(x, y):
		itemIndex, quantity, zeny, ok := c.State.Quantity.Confirm()
		if ok {
			kind := input.CommandTradeAddItem
			command := input.PlayerCommand{Kind: kind, ItemIndex: itemIndex, Quantity: quantity}
			if zeny {
				command.Kind = input.CommandTradeAddZeny
			}
			c.emit(command)
		}
	}
	c.relayout()
	return true
}

func (c *MobileTradeController) ScrollBy(delta float32) bool {
	if c == nil || !c.IsOpen() || !c.Model.Open || c.State.Quantity.Open {
		return false
	}
	c.State.Scroll.ScrollBy(delta)
	c.relayout()
	return true
}

func (c *MobileTradeController) Back() bool {
	if c == nil || !c.IsOpen() {
		return false
	}
	if c.Targeting != nil && c.Targeting.Mode != input.SkillTargetIdle {
		c.Targeting.Cancel()
		c.emit(input.PlayerCommand{Kind: input.CommandCancelAction})
		return true
	}
	if c.Model.PendingRequest != nil {
		c.respondRequest(false)
		return true
	}
	if c.State.Quantity.Open {
		c.State.Quantity.Cancel()
		c.relayout()
		return true
	}
	c.cancel()
	return true
}

func (c *MobileTradeController) respondRequest(accepted bool) {
	request := c.Model.PendingRequest
	if request == nil || !c.Model.OnlineSession {
		return
	}
	if c.emit(input.PlayerCommand{Kind: input.CommandRespondTradeRequest, RequestID: request.TargetID, Accepted: accepted}) {
		c.Model.PendingRequest = nil
		c.Model.Open = accepted
		if !accepted {
			c.Visible = false
		}
		c.relayout()
	}
}

func (c *MobileTradeController) cancel() {
	if c.Model.Open {
		c.emit(input.PlayerCommand{Kind: input.CommandTradeCancel})
	}
	c.Visible = false
	c.State.Quantity.Cancel()
	c.relayout()
}

func (c *MobileTradeController) emit(command input.PlayerCommand) bool {
	if c.Sink == nil {
		return false
	}
	return c.Sink.Emit(command)
}

func (c *MobileTradeController) relayout() {
	if c == nil {
		return
	}
	extent := TradeScrollExtent(c.Model, LayoutTrade(c.Viewport, c.Model, c.State), c.State)
	c.State.Scroll.ViewportExtent = extent.ViewportExtent
	c.State.Scroll.ContentExtent = extent.ContentExtent
	c.State.Scroll.RowExtent = extent.RowExtent
	c.State.Scroll.SetOffset(c.State.Scroll.Offset)
	c.Layout = LayoutTrade(c.Viewport, c.Model, c.State)
}
