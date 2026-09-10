package game

import (
	"fmt"
	"strings"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/network"
)

type mobileVendingState struct {
	open     bool
	loading  bool
	ownerAID uint32
	shopName string
	items    []network.VendingItem
	notice   string
}

// MobileVendingModel projects the existing online vending list without
// exposing packet or desktop-window types to the mobile presentation layer.
func (m *WorldMode) MobileVendingModel(ctx client.Context) mobileui.MobileVendingModel {
	if m == nil {
		return mobileui.MobileVendingModel{}
	}
	online := ctx.Network != nil
	if !online {
		return mobileui.ProjectVending(false, false, 0, "", 0, nil, "Vending is available in an online session.")
	}
	items := make([]mobileui.VendingItemModel, 0, len(m.mobileVending.items))
	for _, item := range m.mobileVending.items {
		name := ""
		iconKey := ""
		if ctx.Resources != nil {
			name, _ = ctx.Resources.ItemDisplayName(int(item.ItemID), item.Identified)
			iconKey, _ = ctx.Resources.ItemResourceName(int(item.ItemID), item.Identified)
		}
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("Item %d", item.ItemID)
		}
		model := mobileui.VendingItemModel{
			Index: item.Index, ItemID: item.ItemID, Name: name, IconKey: iconKey,
			Price: int64(item.Price), Quantity: int(item.Amount), Identified: item.Identified,
			Damaged: item.Damaged, Refine: item.Refine, Cards: item.Cards,
			CanBuy: item.Amount > 0,
		}
		if item.Amount == 0 {
			model.DisabledReason = "Sold out"
		}
		if model.Price > 0 && ctx.Session != nil && int64(ctx.Session.Inventory.Zeny) < model.Price {
			model.CanBuy = false
			model.DisabledReason = "Not enough zeny"
		}
		items = append(items, model)
	}
	zeny := int64(0)
	if ctx.Session != nil {
		zeny = ctx.Session.Inventory.Zeny
	}
	return mobileui.ProjectVending(true, m.mobileVending.loading, m.mobileVending.ownerAID, m.mobileVending.shopName, zeny, items, m.mobileVending.notice)
}

//lint:ignore U1000 retained for mobile session integration
func (m *WorldMode) applyMobileVendingList(ctx client.Context, list network.VendingItemList) {
	if m == nil || list.Own {
		return
	}
	name := "Player Shop"
	if ctx.World != nil {
		if actor, ok := ctx.World.Actors[list.OwnerAID]; ok && strings.TrimSpace(actor.VendingName) != "" {
			name = actor.VendingName
		}
	}
	m.mobileVending = mobileVendingState{
		open: true, ownerAID: list.OwnerAID, shopName: name,
		items: append([]network.VendingItem(nil), list.Items...),
	}
}

//lint:ignore U1000 retained for mobile session integration
func (m *WorldMode) applyMobileVendingResult(result network.VendingPurchaseResult) {
	if m == nil || !m.mobileVending.open {
		return
	}
	if result.Result == 0 {
		m.mobileVending = mobileVendingState{}
		return
	}
	m.mobileVending.notice = fmt.Sprintf("Purchase failed (server result %d).", result.Result)
}

func (m *WorldMode) mobileVendingPurchase(ctx client.Context, commandIndex uint16, quantity int) bool {
	if m == nil || ctx.Network == nil || ctx.Session == nil || !m.mobileVending.open || m.mobileVending.ownerAID == 0 || commandIndex == 0 || quantity <= 0 {
		return false
	}
	var selected *network.VendingItem
	for i := range m.mobileVending.items {
		if m.mobileVending.items[i].Index == commandIndex {
			selected = &m.mobileVending.items[i]
			break
		}
	}
	if selected == nil || selected.Amount == 0 || quantity > int(selected.Amount) {
		return false
	}
	if selected.Price > 0 && int64(quantity)*int64(selected.Price) > ctx.Session.Inventory.Zeny {
		return false
	}
	if err := ctx.Network.SendVendingPurchase(m.mobileVending.ownerAID, []network.VendingPurchaseItem{{Index: commandIndex, Amount: uint16(quantity)}}); err != nil {
		return false
	}
	m.mobileVending.notice = fmt.Sprintf("Purchase requested: %d x %s.", quantity, mobileui.VendingItemName(mobileui.VendingItemModel{Name: selectedItemName(ctx, *selected), ItemID: selected.ItemID}))
	return true
}

func selectedItemName(ctx client.Context, item network.VendingItem) string {
	if ctx.Resources != nil {
		if name, ok := ctx.Resources.ItemDisplayName(int(item.ItemID), item.Identified); ok && strings.TrimSpace(name) != "" {
			return name
		}
	}
	return fmt.Sprintf("Item %d", item.ItemID)
}
