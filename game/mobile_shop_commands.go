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
