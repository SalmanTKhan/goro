package mobileui

import "fmt"

func FixtureVending(name string) MobileVendingModel {
	count := 5
	if name == "vending-long" || name == "vending-empty" {
		count = 32
	}
	if name == "vending-empty" {
		count = 0
	}
	items := make([]VendingItemModel, 0, count)
	for i := 0; i < count; i++ {
		item := VendingItemModel{
			Index: uint16(i + 1), ItemID: uint16(501 + i), Name: fmt.Sprintf("Player Shop Item %02d", i+1),
			IconKey: fmt.Sprintf("fixture/item_%d", 501+i), Price: int64(25 + i*10), Quantity: 3 + i%9, Identified: true, CanBuy: true,
		}
		if name == "vending-long" && i == 0 {
			item.Name = "A very long player shop item name that must stay inside the bounded vending list"
		}
		if name == "vending-disabled" && i == 2 {
			item.CanBuy = false
			item.DisabledReason = "Not enough zeny"
		}
		items = append(items, item)
	}
	return ProjectVending(true, false, 4242, "Mira's Supplies", 2500, items, "Tap an item, then BUY.")
}
