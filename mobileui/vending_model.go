package mobileui

import "fmt"

// VendingItemModel is the renderer-neutral projection of one player-shop
// entry. Index is the server-side vending slot, not a session inventory slot.
type VendingItemModel struct {
	Index          uint16
	ItemID         uint16
	Name           string
	IconKey        string
	Price          int64
	Quantity       int
	Identified     bool
	Damaged        bool
	Refine         uint8
	Cards          [4]uint16
	CanBuy         bool
	DisabledReason string
}

// MobileVendingModel is a read-only projection of the currently requested
// player shop. Loading and Notice make request failures visible without
// moving network state or purchase rules into mobileui.
type MobileVendingModel struct {
	Open          bool
	Loading       bool
	OnlineSession bool
	OwnerID       uint32
	ShopName      string
	Zeny          int64
	Items         []VendingItemModel
	Notice        string
}

func ProjectVending(online bool, loading bool, ownerID uint32, shopName string, zeny int64, items []VendingItemModel, notice string) MobileVendingModel {
	model := MobileVendingModel{
		Open:          online && (loading || ownerID != 0 || len(items) > 0),
		Loading:       online && loading,
		OnlineSession: online,
		OwnerID:       ownerID,
		ShopName:      shopName,
		Zeny:          zeny,
		Items:         append([]VendingItemModel(nil), items...),
		Notice:        notice,
	}
	return model
}

func VendingItemName(item VendingItemModel) string {
	if item.Name != "" {
		return item.Name
	}
	return fmt.Sprintf("Item %d", item.ItemID)
}
