package game

import (
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/network"
)

func (m *WorldMode) mobileShopBuy(ctx client.Context, index, amount uint16) bool {
	if m == nil || ctx.Network == nil {
		return false
	}
	item, ok := m.ui.shopWindow.MobileBuyRequest(index, amount)
	if !ok {
		return false
	}
	return ctx.Network.SendShopBuyItems([]network.BuyRequestItem{item}) == nil
}

func (m *WorldMode) mobileShopSell(ctx client.Context, index, amount uint16) bool {
	if m == nil || ctx.Network == nil {
		return false
	}
	item, ok := m.ui.shopWindow.MobileSellRequest(index, amount)
	if !ok {
		return false
	}
	return ctx.Network.SendShopSellItems([]network.SellRequestItem{item}) == nil
}


func (m *WorldMode) mobileShopStage(ctx client.Context, index, amount uint16) bool {
	if m == nil || amount == 0 {
		return false
	}
	return m.ui.shopWindow.MobileStage(ctx, index, amount)
}

func (m *WorldMode) mobileShopRemoveCart(row uint16) bool {
	if m == nil {
		return false
	}
	return m.ui.shopWindow.MobileRemoveCart(int(row))
}

func (m *WorldMode) mobileShopSubmit(ctx client.Context) bool {
	if m == nil {
		return false
	}
	return m.ui.shopWindow.MobileSubmit(ctx)
}

func (m *WorldMode) mobileShopClose(ctx client.Context, npcID uint32) bool {
	if m == nil {
		return false
	}
	if m.ui.shopWindow.MobileClose(ctx) {
		return true
	}
	if ctx.Network == nil || npcID == 0 {
		return false
	}
	return ctx.Network.SendNPCClose(npcID) == nil
}
