package mobileui

import (
	"fmt"

	"github.com/kivutar/goro/session"
)

type ShopTab uint8

const (
	ShopBuyTab ShopTab = iota
	ShopSellTab
)

func (t ShopTab) String() string {
	if t == ShopSellTab {
		return "Sell"
	}
	return "Buy"
}

type ShopItemModel struct {
	Index          uint16
	ItemID         uint16
	Name           string
	Price          int64
	SellPrice      int64
	Stock          int
	Quantity       int
	MaxQuantity    int
	CanBuy         bool
	CanSell        bool
	DisabledReason string
}

type MobileShopModel struct {
	Open      bool
	NPCID     uint32
	Name      string
	Zeny      int64
	Items     []ShopItemModel
	SellItems []ShopItemModel
}

type MobileStorageModel struct {
	Open      bool
	Amount    int
	MaxAmount int
	Items     []InventoryItemModel
}

func ProjectShop(state *session.Session, shops []session.OfflineShop, npcID uint32) MobileShopModel {
	return ProjectShopWithMetadata(state, shops, npcID, nil)
}

func ProjectShopWithMetadata(state *session.Session, shops []session.OfflineShop, npcID uint32, metadata ItemMetadataSource) MobileShopModel {
	model := MobileShopModel{}
	if state != nil {
		model.Zeny = state.Inventory.Zeny
	}
	for _, shop := range shops {
		if shop.NPCID != npcID {
			continue
		}
		model.Open, model.NPCID, model.Name = true, shop.NPCID, shop.Name
		for i, item := range shop.Items {
			maxQuantity := 999
			if item.Price > 0 {
				maxQuantity = int(model.Zeny / item.Price)
			}
			if item.Stock > 0 && item.Stock < maxQuantity {
				maxQuantity = item.Stock
			}
			canBuy := item.Price >= 0 && maxQuantity > 0
			model.Items = append(model.Items, ShopItemModel{Index: uint16(i), ItemID: item.ItemID, Name: item.Name, Price: item.Price, Stock: item.Stock, MaxQuantity: maxQuantity, CanBuy: canBuy})
		}
		break
	}
	if state != nil {
		for _, item := range state.Inventory.Items {
			if item.Equipped || item.Amount <= 0 {
				continue
			}
			name := ""
			if metadata != nil {
				name, _ = metadata.ItemDisplayName(int(item.ItemID), item.Identified)
			}
			if name == "" {
				name = "Item " + formatItemID(item.ItemID)
			}
			model.SellItems = append(model.SellItems, ShopItemModel{Index: item.Index, ItemID: item.ItemID, Name: name, Price: 5, SellPrice: 5, Quantity: item.Amount, Stock: item.Amount, MaxQuantity: item.Amount, CanSell: true})
		}
	}
	return model
}

func formatItemID(itemID uint16) string {
	return fmt.Sprintf("%d", itemID)
}

func ProjectStorage(state *session.Session, metadata ItemMetadataSource) MobileStorageModel {
	if state == nil {
		return MobileStorageModel{}
	}
	items := make([]InventoryItemModel, 0, len(state.Storage.Items))
	for _, item := range state.Storage.Items {
		items = append(items, projectInventoryItem(item, metadata))
	}
	return MobileStorageModel{Open: state.Storage.Open, Amount: state.Storage.Amount, MaxAmount: state.Storage.MaxAmount, Items: items}
}
