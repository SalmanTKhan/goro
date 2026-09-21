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
	if model.ModeReady {
		c.Tab = model.ActiveTab
	}
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
				if c.Tab == tab.Tab {
					return true
				}
				c.Tab = tab.Tab
				c.Scroll.Offset = 0
				c.Quantity.Cancel()
				c.relayout()
				// Online ShopWindow-backed shops expose only the active server
				// deal mode, so changing tabs must request the newly selected
				// mode even when the previous tab already has rows. Offline
				// projected shops already carry both lists and need no request.
				if c.Shop.NPCID != 0 && (c.Shop.CartEnabled || (len(c.Shop.Items) == 0 && len(c.Shop.SellItems) == 0)) {
					c.emit(input.PlayerCommand{Kind: input.CommandOpenShop, NPCID: c.Shop.NPCID, Tab: uint8(tab.Tab)})
				}
				return true
			}
		}
		if c.Shop.CartEnabled && c.Layout.CartConfirm.Contains(x, y) {
			if len(c.Shop.Cart) > 0 {
				c.emit(input.PlayerCommand{Kind: input.CommandShopCartConfirm, NPCID: c.Shop.NPCID})
			}
			return true
		}
		if c.Shop.CartEnabled {
			for rowNumber, row := range c.Layout.CartRows {
				if row.Contains(x, y) && rowNumber < len(c.Shop.Cart) {
					c.emit(input.PlayerCommand{Kind: input.CommandShopCartRemove, NPCID: c.Shop.NPCID, ItemIndex: uint16(rowNumber)})
					return true
				}
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
	before := c.Scroll.Offset
	c.Scroll.ScrollBy(delta)
	if c.Scroll.Offset == before {
		return false
	}
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
		base := LayoutShopCartScrolled(c.Viewport, len(rows), c.Tab, 0, c.Shop.CartEnabled, len(c.Shop.Cart))
		c.Scroll.ViewportExtent, c.Scroll.ContentExtent, c.Scroll.RowExtent = base.ListViewport.H, EconomyScrollExtent(base.ListViewport, len(rows)).ContentExtent, economyRowExtent
		c.Scroll.SetOffset(c.Scroll.Offset)
		c.Layout = LayoutShopCartScrolled(c.Viewport, len(rows), c.Tab, c.Scroll.Offset, c.Shop.CartEnabled, len(c.Shop.Cart))
	case EconomyStorage:
		base := LayoutStorageScrolled(c.Viewport, len(c.Storage.Items), 0)
		c.Scroll.ViewportExtent, c.Scroll.ContentExtent, c.Scroll.RowExtent = base.ListViewport.H, EconomyScrollExtent(base.ListViewport, len(c.Storage.Items)).ContentExtent, 64
		c.Scroll.SetOffset(c.Scroll.Offset)
		c.Layout = LayoutStorageScrolled(c.Viewport, len(c.Storage.Items), c.Scroll.Offset)
	default:
		c.Layout = EconomyLayout{Safe: c.Viewport.SafeRect()}
	}
	if c.Quantity.Open {
		c.Layout.QuantityModal, c.Layout.QuantityMinus, c.Layout.QuantityPlus, c.Layout.QuantityMax, c.Layout.QuantityConfirm, c.Layout.QuantityCancel = LayoutEconomyQuantity(c.Viewport)
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
	case c.Layout.QuantityMax.Contains(x, y):
		c.Quantity.SetMaximum()
	case c.Layout.QuantityConfirm.Contains(x, y):
		if c.Shop.CartEnabled && (c.Quantity.Action == EconomyQuantityBuy || c.Quantity.Action == EconomyQuantitySell) {
			command := input.PlayerCommand{
				Kind: input.CommandShopCartAdd, NPCID: c.Shop.NPCID,
				ItemIndex: c.Quantity.ItemIndex, Quantity: c.Quantity.Value,
			}
			c.Quantity.Cancel()
			c.emit(command)
		} else if command, ok := c.Quantity.Confirm(); ok {
			c.emit(command)
		}
	default:
		return true
	}
	c.relayout()
	return true
}
