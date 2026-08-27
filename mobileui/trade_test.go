package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestTradeRequestRespondsWithSemanticCommand(t *testing.T) {
	var commands input.CommandBuffer
	c := NewTradeController(FixtureTrade("trade-request"), FoldOuterViewport(), &commands)
	if !c.IsOpen() || c.Model.PendingRequest == nil {
		t.Fatalf("request controller not open: %+v", c)
	}
	c.Tap(c.Layout.RequestAccept.X+1, c.Layout.RequestAccept.Y+1)
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandRespondTradeRequest || got[0].RequestID != 7001 || !got[0].Accepted {
		t.Fatalf("request command=%+v", got)
	}
	if c.Visible == false {
		t.Fatal("accepted request unexpectedly closed trade surface")
	}

	commands.Reset()
	c.SetModel(FixtureTrade("trade-request"))
	c.Tap(c.Layout.RequestDecline.X+1, c.Layout.RequestDecline.Y+1)
	got = commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandRespondTradeRequest || got[0].Accepted {
		t.Fatalf("decline command=%+v", got)
	}
}

func TestTradeItemAndZenyQuantityCommands(t *testing.T) {
	var commands input.CommandBuffer
	c := NewTradeController(FixtureTrade("trade-empty"), FoldOuterViewport(), &commands)
	row := c.Layout.InventoryRows[0].Rect
	c.Tap(row.X+1, row.Y+1)
	if !c.State.Quantity.Open || c.State.Quantity.Maximum != 8 {
		t.Fatalf("item quantity state=%+v", c.State.Quantity)
	}
	c.State.Quantity.Decrement()
	c.Tap(c.Layout.QuantityConfirm.X+1, c.Layout.QuantityConfirm.Y+1)
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandTradeAddItem || got[0].ItemIndex != 1 || got[0].Quantity != 7 {
		t.Fatalf("item command=%+v", got)
	}

	c.Tap(c.Layout.AddZeny.X+1, c.Layout.AddZeny.Y+1)
	if !c.State.Quantity.Open || c.State.Quantity.Maximum != 2500 {
		t.Fatalf("zeny quantity state=%+v", c.State.Quantity)
	}
	c.Tap(c.Layout.QuantityConfirm.X+1, c.Layout.QuantityConfirm.Y+1)
	got = commands.Commands()
	if len(got) != 2 || got[1].Kind != input.CommandTradeAddZeny || got[1].Quantity != 1 {
		t.Fatalf("zeny command=%+v", got)
	}
}

func TestTradeActionsAndBackPrecedence(t *testing.T) {
	var commands input.CommandBuffer
	c := NewTradeController(FixtureTrade("trade-empty"), FoldOuterViewport(), &commands)
	c.Tap(c.Layout.InventoryRows[0].Rect.X+1, c.Layout.InventoryRows[0].Rect.Y+1)
	if !c.Back() || c.State.Quantity.Open {
		t.Fatal("back did not close quantity first")
	}
	c.Tap(c.Layout.Conclude.X+1, c.Layout.Conclude.Y+1)
	c.SetModel(FixtureTrade("trade-concluded"))
	c.Tap(c.Layout.Commit.X+1, c.Layout.Commit.Y+1)
	c.Tap(c.Layout.Cancel.X+1, c.Layout.Cancel.Y+1)
	got := commands.Commands()
	if len(got) != 3 || got[0].Kind != input.CommandTradeConclude || got[1].Kind != input.CommandTradeCommit || got[2].Kind != input.CommandTradeCancel {
		t.Fatalf("trade action commands=%+v", got)
	}
}

func TestTradeLayoutRepresentativeViewports(t *testing.T) {
	viewports := []Viewport{
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
		{Width: 1920, Height: 1080},
		{Width: 2400, Height: 1080},
		{Width: 2560, Height: 1440},
		FoldOuterViewport(),
		{Width: 1280, Height: 800, SafeLeft: 24, SafeTop: 24, SafeRight: 24, SafeBottom: 32},
	}
	for _, viewport := range viewports {
		model := FixtureTrade("trade-long-names")
		controller := NewTradeController(model, viewport, nil)
		safe := viewport.SafeRect()
		for name, rect := range map[string]Rect{
			"panel": controller.Layout.Panel, "header": controller.Layout.Header,
			"inventory": controller.Layout.InventoryPanel, "own": controller.Layout.OwnPanel,
			"partner": controller.Layout.PartnerPanel, "add-zeny": controller.Layout.AddZeny,
			"conclude": controller.Layout.Conclude, "commit": controller.Layout.Commit,
			"cancel": controller.Layout.Cancel,
		} {
			if rect.W <= 0 || rect.H <= 0 {
				continue
			}
			if rect.X < safe.X || rect.Y < safe.Y || rect.Right() > safe.Right() || rect.Bottom() > safe.Bottom() {
				t.Fatalf("%s escaped safe area at %+v: rect=%+v safe=%+v", name, viewport, rect, safe)
			}
		}
		for _, row := range controller.Layout.InventoryRows {
			if row.Rect.W < 48 || row.Rect.H < 48 || row.Rect.Right() > controller.Layout.InventoryPanel.Right() {
				t.Fatalf("invalid inventory row at %+v: %+v", viewport, row)
			}
		}
	}
}

func TestTradeScrollClamps(t *testing.T) {
	model := FixtureTrade("trade-empty")
	for i := 0; i < 30; i++ {
		model.Inventory = append(model.Inventory, TradeInventoryItemModel{Item: InventoryItemModel{Index: uint16(10 + i), ItemID: 909, DisplayName: "Jellopy", Quantity: 1}, CanAdd: true})
	}
	c := NewTradeController(model, Viewport{Width: 390, Height: 844}, nil)
	c.ScrollBy(100000)
	if c.State.Scroll.Offset != c.State.Scroll.MaxOffset() {
		t.Fatalf("scroll exceeded max: %+v", c.State.Scroll)
	}
	c.ScrollBy(-100000)
	if c.State.Scroll.Offset != 0 {
		t.Fatalf("scroll exceeded minimum: %+v", c.State.Scroll)
	}
}
