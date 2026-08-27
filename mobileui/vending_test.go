package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestVendingControllerSelectBuyAndCancelQuantity(t *testing.T) {
	var commands input.CommandBuffer
	c := NewVendingController(FixtureVending("vending-basic"), FoldOuterViewport(), &commands)
	if !c.IsOpen() || c.Layout.Panel.W > 1280 || c.Layout.Panel.Right() > c.Layout.Safe.Right() {
		t.Fatalf("vending panel is not bounded/safe: %+v safe=%+v", c.Layout.Panel, c.Layout.Safe)
	}
	for _, row := range c.Layout.Rows {
		if row.W < 48 || row.H < 48 {
			t.Fatalf("vending row misses touch target: %+v", row)
		}
	}
	c.Tap(c.Layout.Rows[0].X+2, c.Layout.Rows[0].Y+2)
	if !c.State.HasSelection || c.State.SelectedIndex != 0 {
		t.Fatalf("item was not selected: %+v", c.State)
	}
	c.Tap(c.Layout.BuyButton.X+2, c.Layout.BuyButton.Y+2)
	if !c.Quantity.Open || c.Quantity.Maximum != 3 || len(commands.Commands()) != 0 {
		t.Fatalf("buy quantity did not open: %+v commands=%+v", c.Quantity, commands.Commands())
	}
	c.Quantity.SetValue(2)
	c.Tap(c.Layout.QuantityConfirm.X+2, c.Layout.QuantityConfirm.Y+2)
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandBuyVendingItem || got[0].ItemIndex != 1 || got[0].Quantity != 2 {
		t.Fatalf("unexpected vending command: %+v", got)
	}
	c.Tap(c.Layout.Rows[0].X+2, c.Layout.Rows[0].Y+2)
	c.Tap(c.Layout.BuyButton.X+2, c.Layout.BuyButton.Y+2)
	c.Back()
	if c.Quantity.Open || len(commands.Commands()) != 1 {
		t.Fatalf("quantity cancel changed command state: quantity=%+v commands=%+v", c.Quantity, commands.Commands())
	}
	if !c.Back() || c.IsOpen() {
		t.Fatal("vending back did not close the screen")
	}
}

func TestVendingControllerScrollClampsAndOwnsTouch(t *testing.T) {
	c := NewVendingController(FixtureVending("vending-long"), FoldOuterViewport(), nil)
	if !c.ConsumeTouch(input.TouchPoint{X: 1, Y: 1}) || c.Scroll.MaxOffset() <= 0 || len(c.Layout.Rows) >= len(c.Model.Items) {
		t.Fatalf("long vending list not scrollable/owned: scroll=%+v rows=%d items=%d", c.Scroll, len(c.Layout.Rows), len(c.Model.Items))
	}
	c.ScrollBy(100000)
	if c.Scroll.Offset != c.Scroll.MaxOffset() {
		t.Fatalf("vending scroll not clamped: offset=%v max=%v", c.Scroll.Offset, c.Scroll.MaxOffset())
	}
	if c.Layout.BuyButton.Intersects(c.Layout.ListViewport) {
		t.Fatal("vending buy action overlaps list viewport")
	}
}

func TestVendingLayoutSafeAcrossRepresentativeViewports(t *testing.T) {
	for _, viewport := range []Viewport{{Width: 1920, Height: 1080}, {Width: 2400, Height: 1080}, {Width: 2560, Height: 1440}, FoldOuterViewport(), {Width: 834, Height: 1194}} {
		l := LayoutVending(viewport, 12)
		if l.Panel.X < l.Safe.X || l.Panel.Y < l.Safe.Y || l.Panel.Right() > l.Safe.Right() || l.Panel.Bottom() > l.Safe.Bottom() {
			t.Fatalf("viewport %+v escaped safe area: panel=%+v safe=%+v", viewport, l.Panel, l.Safe)
		}
		if l.BuyButton.W > 0 && l.BuyButton.H < 48 {
			t.Fatalf("viewport %+v buy button too small: %+v", viewport, l.BuyButton)
		}
	}
}
