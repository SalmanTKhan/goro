package game

import (
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/mobileui"
)

// MobileShopModel projects the active online ShopWindow. The packet-driven
// window remains the source of truth so mobile and desktop observe the same
// server list and deal state.
func (m *WorldMode) MobileShopModel(ctx client.Context) mobileui.MobileShopModel {
	if m == nil || ctx.Network == nil {
		return mobileui.MobileShopModel{}
	}
	return m.ui.shopWindow.MobileModel(ctx.Session, ctx.Resources)
}
