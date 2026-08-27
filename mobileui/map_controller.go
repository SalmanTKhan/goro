package mobileui

import "github.com/kivutar/goro/input"

type MobileMapController struct {
	Model    MobileMapModel
	State    MapInteractionState
	Layout   MobileMapLayout
	Viewport Viewport
	Tokens   MapTokens
	Sink     input.CommandSink
}

func NewMapController(model MobileMapModel, viewport Viewport, sink input.CommandSink) *MobileMapController {
	c := &MobileMapController{Model: model, Viewport: viewport, Tokens: DefaultMapTokens(), Sink: sink}
	c.relayout()
	return c
}

func (c *MobileMapController) SetModel(model MobileMapModel) {
	if c == nil {
		return
	}
	c.Model = model
	if _, ok := model.Warp(c.State.SelectedWarpID); !ok {
		c.State.SelectedWarpID = 0
		c.State.ConfirmOpen = false
	}
	c.relayout()
}

func (c *MobileMapController) Resize(viewport Viewport) {
	if c == nil {
		return
	}
	c.Viewport = viewport
	c.relayout()
}

func (c *MobileMapController) ConsumeTouch(point input.TouchPoint) bool {
	return c != nil && c.Layout.Safe.Contains(float32(point.X), float32(point.Y))
}

func (c *MobileMapController) Tap(x, y float32) bool {
	if c == nil || !c.Layout.Safe.Contains(x, y) {
		return false
	}
	if c.State.ConfirmOpen {
		if c.Layout.ConfirmButton.Contains(x, y) {
			return c.confirmTravel()
		}
		if c.Layout.ConfirmCancel.Contains(x, y) {
			c.State.ConfirmOpen = false
			c.relayout()
			return true
		}
		return true
	}
	if c.Layout.BackButton.Contains(x, y) {
		return c.Back()
	}
	for _, row := range c.Layout.WarpRows {
		if row.Rect.Contains(x, y) {
			c.State.SelectedWarpID = row.ID
			c.relayout()
			return true
		}
	}
	if c.Layout.TravelButton.Contains(x, y) {
		c.State.ConfirmOpen = true
		c.relayout()
		return true
	}
	return true
}

func (c *MobileMapController) ScrollBy(delta float32) bool {
	if c == nil || c.State.ConfirmOpen {
		return false
	}
	extent := MapScrollExtent(c.Model, c.Layout, c.Tokens)
	extent.Offset = c.State.ScrollOffset
	extent.ScrollBy(delta)
	c.State.ScrollOffset = extent.Offset
	c.relayout()
	return true
}

func (c *MobileMapController) Back() bool {
	if c == nil {
		return false
	}
	if c.State.ConfirmOpen {
		c.State.ConfirmOpen = false
		c.relayout()
		return true
	}
	c.State.SelectedWarpID = 0
	c.relayout()
	return true
}

func (c *MobileMapController) confirmTravel() bool {
	warp, ok := c.Model.Warp(c.State.SelectedWarpID)
	if !ok {
		c.State.ConfirmOpen = false
		c.relayout()
		return true
	}
	accepted := false
	if c.Sink != nil {
		accepted = c.Sink.Emit(input.PlayerCommand{Kind: input.CommandMoveTo, Position: input.WorldPosition{X: float64(warp.X), Y: float64(warp.Y)}})
	}
	if accepted {
		c.State.ConfirmOpen = false
		c.State.SelectedWarpID = 0
	}
	c.relayout()
	return true
}

func (c *MobileMapController) relayout() {
	if c == nil {
		return
	}
	c.Layout = LayoutMap(c.Viewport, c.Tokens, c.Model, c.State)
}
