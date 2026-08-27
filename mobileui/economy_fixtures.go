package mobileui

import (
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/session"
)

func FixtureDialog(name string) MobileDialogModel {
	if name == "dialog-speakers" {
		return MobileDialogModel{
			Open:  true,
			NPCID: 9001,
			Title: "NPC",
			Messages: []DialogMessage{
				{Speaker: "PrivateMvpRoom", Text: "Please select a private MVP room."},
				{Speaker: "PrivateMvpRoom", Text: "You can only use the room for 0 minutes."},
				{Speaker: "Tine", Text: "Some married chocolate lovers almost double their experience at trainings!<br/>But everything isn't so simply..."},
			},
			Options: []DialogOption{
				{ID: "next", Label: "Next", Action: DialogNext, Enabled: true},
				{ID: "cancel", Label: "Cancel", Action: DialogNPCClose, Enabled: true},
			},
		}
	}
	message := "Welcome to Prontera. This offline fixture is ready for exploration."
	if name == "dialog-long" {
		message = "Welcome to Prontera. The guide can help you prepare for the road ahead. This deliberately long message verifies wrapping and bounded dialog layout on the Fold outer display."
	}
	return ProjectDialog(9001, "PRONTERA GUIDE", message, true)
}

func FixtureShop(name string) MobileShopModel {
	state := session.New()
	state.Inventory.Zeny = 125
	state.Inventory.Items = []session.InventoryItem{
		{Index: 1, ItemID: 601, Type: db.ItemTypeHealing, Amount: 8, Identified: true},
		{Index: 2, ItemID: 602, Type: db.ItemTypeEtc, Amount: 3, Identified: true},
		{Index: 3, ItemID: 603, Type: db.ItemTypeArmor, Amount: 1, Identified: true, Equipped: true},
	}
	items := []session.OfflineShopItem{
		{ItemID: 501, Type: db.ItemTypeHealing, Name: "Red Potion", Price: 50, Stock: 99},
		{ItemID: 909, Type: db.ItemTypeEtc, Name: "Jellopy", Price: 10, Stock: 99},
	}
	if name == "shop-sell" {
		state.Inventory.Zeny = 500
		items = append(items, session.OfflineShopItem{ItemID: 610, Type: db.ItemTypeHealing, Name: "Orange Potion", Price: 100, Stock: 12})
	}
	if name == "shop-long" {
		for i := 0; i < 32; i++ {
			items = append(items, session.OfflineShopItem{ItemID: uint16(700 + i), Type: db.ItemTypeEtc, Name: "Field Supply " + formatItemID(uint16(i+1)), Price: int64(5 + i), Stock: 99})
		}
	}
	return ProjectShopWithMetadata(state, []session.OfflineShop{{NPCID: 9001, Name: "Prontera Guide Shop", Items: items}}, 9001, fixtureItemMetadata{})
}

func FixtureStorage(name string) MobileStorageModel {
	state := session.New()
	state.Storage = session.Storage{Open: true, Amount: 2, MaxAmount: 300, Items: []session.InventoryItem{
		{Index: 11, ItemID: 909, Type: db.ItemTypeEtc, Amount: 6, Identified: true},
		{Index: 12, ItemID: 501, Type: db.ItemTypeHealing, Amount: 2, Identified: true},
	}}
	if name == "storage-empty" {
		state.Storage.Items = nil
		state.Storage.Amount = 0
	}
	if name == "storage-long" {
		for i := 0; i < 32; i++ {
			state.Storage.Items = append(state.Storage.Items, session.InventoryItem{Index: uint16(20 + i), ItemID: uint16(700 + i), Type: db.ItemTypeEtc, Amount: i + 1, Identified: true})
		}
		state.Storage.Amount = len(state.Storage.Items)
	}
	return ProjectStorage(state, fixtureItemMetadata{})
}
