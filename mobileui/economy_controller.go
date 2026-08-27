package mobileui

import "github.com/kivutar/goro/input"

type EconomyScreen uint8

const (
	EconomyClosed EconomyScreen = iota
	EconomyShop
	EconomyStorage
)

type MobileEconomyController struct {
	Viewport Viewport
	Shop     MobileShopModel
	Storage  MobileStorageModel
	Screen   EconomyScreen
	Tab      ShopTab
	Layout   EconomyLayout
	Scroll   InventoryScrollState
	Quantity EconomyQuantityState
	Sink     input.CommandSink
}

func NewEconomyController(viewport Viewport, sink input.CommandSink) *MobileEconomyController {
	c := &MobileEconomyController{Viewport: viewport, Tab: ShopBuyTab, Sink: sink}
	c.relayout()
	return c
}

func (c *MobileEconomyController) OpenShop(model MobileShopModel) {
	if c == nil {
		return
	}
	c.Shop, c.Screen, c.Tab = model, EconomyShop, ShopBuyTab
	c.Scroll = InventoryScrollState{}
	c.Quantity.Cancel()
	c.relayout()
}

func (c *MobileEconomyController) OpenStorage(model MobileStorageModel) {
	if c == nil {
		return
	}
	c.Storage, c.Screen = model, EconomyStorage
	c.Scroll = InventoryScrollState{}
	c.Quantity.Cancel()
	c.relayout()
}

func (c *MobileEconomyController) SetShop(model MobileShopModel) {
	if c == nil {
		return
	}
	c.Shop = model
	if c.Screen == EconomyShop && !model.Open {
		c.Screen = EconomyClosed
	}
	c.relayout()
}

func (c *MobileEconomyController) SetStorage(model MobileStorageModel) {
	if c == nil {
		return
	}
	c.Storage = model
	if c.Screen == EconomyStorage && !model.Open {
		c.Screen = EconomyClosed
	}
	c.relayout()
}

func (c *MobileEconomyController) Resize(viewport Viewport) {
	if c == nil {
		return
	}
	c.Viewport = viewport
	c.relayout()
}

func (c *MobileEconomyController) Close() bool {
	return c.close(false)
}

func (c *MobileEconomyController) close(sendAuthorityCommand bool) bool {
	if c == nil || c.Screen == EconomyClosed {
		return false
	}
	screen := c.Screen
	npcID := c.Shop.NPCID
	c.Screen = EconomyClosed
	c.Quantity.Cancel()
	c.relayout()
	if sendAuthorityCommand {
		switch screen {
		case EconomyShop:
			if npcID != 0 {
				c.emit(input.PlayerCommand{Kind: input.CommandCloseShop, NPCID: npcID})
			}
		case EconomyStorage:
			c.emit(input.PlayerCommand{Kind: input.CommandCloseStorage})
		}
	}
	return true
}

func (c *MobileEconomyController) ConsumeTouch(point input.TouchPoint) bool {
	_ = point
	return c != nil && c.Screen != EconomyClosed
}

func (c *MobileEconomyController) Tap(x, y float32) bool {
	if c == nil || c.Screen == EconomyClosed {
		return false
	}
	if c.Quantity.Open {
		return c.tapQuantity(x, y)
	}
	if c.Layout.Close.Contains(x, y) {
		c.close(true)
		return true
	}
	if c.Screen == EconomyShop {
		for _, tab := range c.Layout.Tabs {
			if tab.Rect.Contains(x, y) {
				c.Tab = tab.Tab
				c.Scroll.Offset = 0
				c.relayout()
				// An online NPC shop begins with a deal-type packet. The mobile
				// surface has no desktop deal dialog, so selecting a tab is the
				// explicit buy/sell request.
				if c.Shop.NPCID != 0 && len(c.Shop.Items) == 0 && len(c.Shop.SellItems) == 0 {
					c.emit(input.PlayerCommand{Kind: input.CommandOpenShop, NPCID: c.Shop.NPCID, Tab: uint8(tab.Tab)})
				}
				return true
			}
		}
		items := c.Shop.Items
		if c.Tab == ShopSellTab {
			items = c.Shop.SellItems
		}
		for rowNumber, row := range c.Layout.Rows {
			if !row.Contains(x, y) || rowNumber >= len(c.Layout.RowIndices) {
				continue
			}
			itemIndex := c.Layout.RowIndices[rowNumber]
			if itemIndex < 0 || itemIndex >= len(items) {
				return true
			}
			item := items[itemIndex]
			if c.Tab == ShopSellTab {
				if item.CanSell {
					c.Quantity.OpenFor(EconomyQuantitySell, 0, item.Index, economyQuantityMaximum(item, EconomyQuantitySell))
					c.relayout()
				}
				return true
			}
			if item.CanBuy {
				c.Quantity.OpenFor(EconomyQuantityBuy, c.Shop.NPCID, item.Index, economyQuantityMaximum(item, EconomyQuantityBuy))
				c.relayout()
			}
			return true
		}
		return true
	}
	for rowNumber, row := range c.Layout.Rows {
		if row.Contains(x, y) && rowNumber < len(c.Layout.RowIndices) {
			itemIndex := c.Layout.RowIndices[rowNumber]
			if itemIndex < 0 || itemIndex >= len(c.Storage.Items) {
				return true
			}
			item := c.Storage.Items[itemIndex]
			c.Quantity.OpenFor(EconomyQuantityWithdraw, 0, item.Index, item.Quantity)
			c.relayout()
			return true
		}
	}
	return true
}

func (c *MobileEconomyController) Back() bool {
	if c == nil || c.Screen == EconomyClosed {
		return false
	}
	if c.Quantity.Open {
		c.Quantity.Cancel()
		c.relayout()
		return true
	}
	return c.close(true)
}

func (c *MobileEconomyController) ScrollBy(delta float32) bool {
	if c == nil || c.Screen == EconomyClosed || c.Quantity.Open {
		return false
	}
	c.Scroll.ScrollBy(delta)
	c.relayout()
	return true
}

func (c *MobileEconomyController) emit(command input.PlayerCommand) {
	if c.Sink != nil {
		c.Sink.Emit(command)
	}
}

func (c *MobileEconomyController) relayout() {
	if c == nil {
		return
	}
	switch c.Screen {
	case EconomyShop:
		rows := c.Shop.Items
		if c.Tab == ShopSellTab {
			rows = c.Shop.SellItems
		}
		base := LayoutShopScrolled(c.Viewport, len(rows), c.Tab, 0)
		c.Scroll.ViewportExtent, c.Scroll.ContentExtent, c.Scroll.RowExtent = base.ListViewport.H, EconomyScrollExtent(base.ListViewport, len(rows)).ContentExtent, 64
		c.Scroll.SetOffset(c.Scroll.Offset)
		c.Layout = LayoutShopScrolled(c.Viewport, len(rows), c.Tab, c.Scroll.Offset)
	case EconomyStorage:
		base := LayoutStorageScrolled(c.Viewport, len(c.Storage.Items), 0)
		c.Scroll.ViewportExtent, c.Scroll.ContentExtent, c.Scroll.RowExtent = base.ListViewport.H, EconomyScrollExtent(base.ListViewport, len(c.Storage.Items)).ContentExtent, 64
		c.Scroll.SetOffset(c.Scroll.Offset)
		c.Layout = LayoutStorageScrolled(c.Viewport, len(c.Storage.Items), c.Scroll.Offset)
	default:
		c.Layout = EconomyLayout{Safe: c.Viewport.SafeRect()}
	}
	if c.Quantity.Open {
		c.Layout.QuantityModal, c.Layout.QuantityMinus, c.Layout.QuantityPlus, c.Layout.QuantityConfirm, c.Layout.QuantityCancel = LayoutEconomyQuantity(c.Viewport)
	}
}

func (c *MobileEconomyController) tapQuantity(x, y float32) bool {
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
