package mobileui

import (
	"testing"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
)

func TestEconomyProjectionAndBoundedLayout(t *testing.T) {
	state := session.New()
	state.Inventory.Zeny = 75
	state.Inventory.Items = []session.InventoryItem{{Index: 5, ItemID: 501, Type: db.ItemTypeHealing, Amount: 3}}
	state.Storage = session.Storage{Open: true, Amount: 1, MaxAmount: 300, Items: []session.InventoryItem{{Index: 4, ItemID: 909, Type: db.ItemTypeEtc, Amount: 2}}}
	shop := ProjectShop(state, []session.OfflineShop{{NPCID: 8, Name: "Guide Shop", Items: []session.OfflineShopItem{{ItemID: 501, Price: 50, Stock: 2}}}}, 8)
	if !shop.Open || len(shop.Items) != 1 || !shop.Items[0].CanBuy {
		t.Fatalf("unexpected shop model: %+v", shop)
	}
	if len(shop.SellItems) != 1 || !shop.SellItems[0].CanSell || shop.SellItems[0].Quantity != 3 {
		t.Fatalf("unexpected sell model: %+v", shop.SellItems)
	}
	storage := ProjectStorage(state, nil)
	if !storage.Open || len(storage.Items) != 1 {
		t.Fatalf("unexpected storage model: %+v", storage)
	}
	layout := LayoutEconomy(FoldOuterViewport(), len(shop.Items))
	if layout.Panel.Right() > layout.Safe.Right() || layout.Panel.Bottom() > layout.Safe.Bottom() || layout.Rows[0].W < 48 || layout.Rows[0].H < 48 {
		t.Fatalf("economy layout escapes or misses touch target: %+v safe=%+v", layout, layout.Safe)
	}
}

func TestEconomyControllerBuySellWithdrawAndOwnership(t *testing.T) {
	var commands input.CommandBuffer
	viewport := Viewport{Width: 2268, Height: 832, SafeLeft: 12, SafeRight: 12, SafeTop: 8, SafeBottom: 8}
	c := NewEconomyController(viewport, &commands)
	shop := FixtureShop("shop-sell")
	c.OpenShop(shop)
	if !c.ConsumeTouch(input.TouchPoint{X: int(c.Layout.Panel.X + 1), Y: int(c.Layout.Panel.Y + 1)}) {
		t.Fatal("shop touch was not consumed")
	}
	if c.Layout.Panel.Right() > c.Layout.Safe.Right() || c.Layout.Panel.Bottom() > c.Layout.Safe.Bottom() {
		t.Fatalf("shop escaped safe area: %+v safe=%+v", c.Layout.Panel, c.Layout.Safe)
	}
	for _, row := range c.Layout.Rows {
		if row.W < 48 || row.H < 48 {
			t.Fatalf("shop row missed touch target: %+v", row)
		}
	}
	c.Tap(c.Layout.Rows[0].X+2, c.Layout.Rows[0].Y+2)
	if !c.Quantity.Open || c.Quantity.Maximum != 10 || len(commands.Commands()) != 0 {
		t.Fatalf("buy quantity did not open: state=%+v commands=%+v", c.Quantity, commands.Commands())
	}
	c.Tap(c.Layout.QuantityPlus.X+2, c.Layout.QuantityPlus.Y+2)
	c.Tap(c.Layout.QuantityConfirm.X+2, c.Layout.QuantityConfirm.Y+2)
	if got := commands.Commands(); len(got) != 1 || got[0].Kind != input.CommandBuyItem || got[0].NPCID != 9001 || got[0].Quantity != 2 {
		t.Fatalf("unexpected buy command: %+v", got)
	}
	c.Tap(c.Layout.Tabs[1].Rect.X+2, c.Layout.Tabs[1].Rect.Y+2)
	c.Tap(c.Layout.Rows[0].X+2, c.Layout.Rows[0].Y+2)
	if !c.Quantity.Open || c.Quantity.Maximum != shop.SellItems[0].Quantity {
		t.Fatalf("sell quantity did not open: %+v", c.Quantity)
	}
	c.Tap(c.Layout.QuantityPlus.X+2, c.Layout.QuantityPlus.Y+2)
	c.Tap(c.Layout.QuantityConfirm.X+2, c.Layout.QuantityConfirm.Y+2)
	got := commands.Commands()
	if len(got) != 2 || got[1].Kind != input.CommandSellItem || got[1].ItemIndex != shop.SellItems[0].Index || got[1].Quantity != 2 {
		t.Fatalf("unexpected sell command: %+v", got)
	}
	c.OpenStorage(FixtureStorage("storage-basic"))
	c.Tap(c.Layout.Rows[0].X+2, c.Layout.Rows[0].Y+2)
	if !c.Quantity.Open || c.Quantity.Maximum != 6 {
		t.Fatalf("withdraw quantity did not open: %+v", c.Quantity)
	}
	c.Quantity.SetValue(6)
	c.Tap(c.Layout.QuantityConfirm.X+2, c.Layout.QuantityConfirm.Y+2)
	got = commands.Commands()
	if len(got) != 3 || got[2].Kind != input.CommandWithdrawItem || got[2].Quantity != 6 {
		t.Fatalf("unexpected withdraw command: %+v", got)
	}
	if !c.Tap(c.Layout.Close.X+1, c.Layout.Close.Y+1) || c.Screen != EconomyClosed {
		t.Fatal("economy close did not return to world")
	}
}

func TestEconomyQuantityCancelAndScroll(t *testing.T) {
	var commands input.CommandBuffer
	viewport := FoldOuterViewport()
	c := NewEconomyController(viewport, &commands)
	c.OpenShop(FixtureShop("shop-long"))
	if c.Scroll.MaxOffset() <= 0 || len(c.Layout.Rows) >= len(c.Shop.Items) {
		t.Fatalf("long shop did not create a bounded scroll surface: scroll=%+v rows=%d items=%d", c.Scroll, len(c.Layout.Rows), len(c.Shop.Items))
	}
	first := c.Layout.RowIndices[0]
	c.ScrollBy(100000)
	if c.Scroll.Offset != c.Scroll.MaxOffset() {
		t.Fatalf("shop scroll was not clamped: offset=%v max=%v", c.Scroll.Offset, c.Scroll.MaxOffset())
	}
	if c.Layout.RowIndices[0] <= first {
		t.Fatalf("scrolled row index did not advance: first=%d now=%d", first, c.Layout.RowIndices[0])
	}
	c.Tap(c.Layout.Rows[0].X+2, c.Layout.Rows[0].Y+2)
	if !c.Quantity.Open {
		t.Fatal("scrolled shop row did not open quantity")
	}
	c.Back()
	if c.Quantity.Open || len(commands.Commands()) != 0 {
		t.Fatalf("quantity cancel emitted or left modal open: quantity=%+v commands=%+v", c.Quantity, commands.Commands())
	}
	c.OpenStorage(FixtureStorage("storage-long"))
	if c.Scroll.MaxOffset() <= 0 || len(c.Layout.Rows) >= len(c.Storage.Items) {
		t.Fatalf("long storage did not create a bounded scroll surface: scroll=%+v rows=%d items=%d", c.Scroll, len(c.Layout.Rows), len(c.Storage.Items))
	}
}


func TestEconomyShopTabAlwaysRequestsAuthoritativeDealMode(t *testing.T) {
	var commands input.CommandBuffer
	shop := FixtureShop("shop-sell")
	shop.CartEnabled = true
	c := NewEconomyController(Viewport{Width: 1200, Height: 700}, &commands)
	c.OpenShop(shop)

	sell := c.Layout.Tabs[1].Rect
	if !c.Tap(sell.X+2, sell.Y+2) {
		t.Fatal("sell tab tap was not consumed")
	}
	if c.Tab != ShopSellTab {
		t.Fatalf("tab=%v, want Sell", c.Tab)
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandOpenShop || got[0].NPCID != shop.NPCID || got[0].Tab != uint8(ShopSellTab) {
		t.Fatalf("sell tab did not request server deal mode: %+v", got)
	}
}

func TestEconomyOnlineCartStagesThenConfirms(t *testing.T) {
	var commands input.CommandBuffer
	shop := FixtureShop("shop-sell")
	shop.CartEnabled = true
	c := NewEconomyController(Viewport{Width: 1200, Height: 700}, &commands)
	c.OpenShop(shop)

	row := c.Layout.Rows[0]
	c.Tap(row.X+2, row.Y+2)
	if !c.Quantity.Open {
		t.Fatal("buy row did not open quantity")
	}
	c.Quantity.SetValue(2)
	c.Tap(c.Layout.QuantityConfirm.X+2, c.Layout.QuantityConfirm.Y+2)
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandShopCartAdd || got[0].Quantity != 2 || got[0].ItemIndex != shop.Items[0].Index {
		t.Fatalf("quantity confirm did not stage cart item: %+v", got)
	}

	shop.Cart = []ShopCartItemModel{{
		ItemID: shop.Items[0].ItemID, Name: shop.Items[0].Name,
		Quantity: 2, UnitPrice: shop.Items[0].Price, Total: shop.Items[0].Price * 2,
	}}
	shop.CartTotal = shop.Cart[0].Total
	c.SetShop(shop)
	if len(c.Layout.CartRows) == 0 || c.Layout.CartConfirm.W <= 0 {
		t.Fatalf("cart tray was not laid out: %+v", c.Layout)
	}
	c.Tap(c.Layout.CartConfirm.X+2, c.Layout.CartConfirm.Y+2)
	got = commands.Commands()
	if len(got) != 2 || got[1].Kind != input.CommandShopCartConfirm || got[1].NPCID != shop.NPCID {
		t.Fatalf("cart confirm command=%+v", got)
	}
}

func TestEconomyCartTrayResponsivePlacement(t *testing.T) {
	landscape := LayoutShopCartScrolled(
		Viewport{Width: 1600, Height: 800}, 20, ShopBuyTab, 0, true, 3,
	)
	if landscape.CartPanel.W <= 0 || landscape.CartPanel.X <= landscape.ListViewport.Right() {
		t.Fatalf("landscape cart is not beside list: list=%+v cart=%+v", landscape.ListViewport, landscape.CartPanel)
	}
	if landscape.CartPanel.Intersects(landscape.ListViewport) {
		t.Fatalf("landscape cart overlaps list: list=%+v cart=%+v", landscape.ListViewport, landscape.CartPanel)
	}

	portrait := LayoutShopCartScrolled(
		Viewport{Width: 420, Height: 900, SafeTop: 24, SafeBottom: 24}, 20, ShopBuyTab, 0, true, 3,
	)
	if portrait.CartPanel.W <= 0 || portrait.CartPanel.Y <= portrait.ListViewport.Bottom() {
		t.Fatalf("portrait cart is not below list: list=%+v cart=%+v", portrait.ListViewport, portrait.CartPanel)
	}
	if portrait.CartPanel.Intersects(portrait.ListViewport) {
		t.Fatalf("portrait cart overlaps list: list=%+v cart=%+v", portrait.ListViewport, portrait.CartPanel)
	}
}

func TestEconomyScrollReportsOnlyVisualChanges(t *testing.T) {
	c := NewEconomyController(FoldOuterViewport(), nil)
	c.OpenShop(FixtureShop("shop-long"))
	if !c.ScrollBy(100000) {
		t.Fatal("scroll to bottom did not report a change")
	}
	atBottom := c.Scroll.Offset
	if c.ScrollBy(100000) || c.Scroll.Offset != atBottom {
		t.Fatalf("scroll past bottom reported a change: before=%f after=%f", atBottom, c.Scroll.Offset)
	}
}


func TestEconomyQuantityMaxControl(t *testing.T) {
	var commands input.CommandBuffer
	shop := FixtureShop("shop-sell")
	c := NewEconomyController(Viewport{Width: 420, Height: 900, SafeTop: 24, SafeBottom: 24}, &commands)
	c.OpenShop(shop)
	c.Tab = ShopSellTab
	c.relayout()

	// Use the fixture's first sellable stack and force a larger limit so this
	// exercises the phone use case where repeated '+' taps are impractical.
	c.Tap(c.Layout.Rows[0].X+2, c.Layout.Rows[0].Y+2)
	if !c.Quantity.Open {
		t.Fatal("sell quantity did not open")
	}
	c.Quantity.Maximum = 68
	c.Quantity.SetValue(1)
	c.relayout()

	if c.Layout.QuantityMax.W < 48 || c.Layout.QuantityMax.H < 48 {
		t.Fatalf("max control misses touch target: %+v", c.Layout.QuantityMax)
	}
	c.Tap(c.Layout.QuantityMax.X+2, c.Layout.QuantityMax.Y+2)
	if c.Quantity.Value != 68 {
		t.Fatalf("max quantity value=%d, want 68", c.Quantity.Value)
	}
	if len(commands.Commands()) != 0 {
		t.Fatalf("max control emitted authority command: %+v", commands.Commands())
	}
}

func TestEconomyQuantityFourButtonRowFitsPortrait(t *testing.T) {
	modal, minus, plus, maximum, confirm, cancel := LayoutEconomyQuantity(
		Viewport{Width: 420, Height: 900, SafeTop: 24, SafeBottom: 24},
	)
	if modal.W <= 0 || cancel.W < 48 {
		t.Fatalf("quantity modal invalid: modal=%+v cancel=%+v", modal, cancel)
	}
	buttons := []Rect{minus, plus, maximum, confirm}
	for i, button := range buttons {
		if button.W < 48 || button.H < 48 {
			t.Fatalf("button %d misses touch target: %+v", i, button)
		}
		if button.X < modal.X || button.Right() > modal.Right() || button.Y < modal.Y || button.Bottom() > modal.Bottom() {
			t.Fatalf("button %d escapes modal: button=%+v modal=%+v", i, button, modal)
		}
		if i > 0 && buttons[i-1].Intersects(button) {
			t.Fatalf("quantity buttons overlap: left=%+v right=%+v", buttons[i-1], button)
		}
	}
}
