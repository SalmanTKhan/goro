package mobileui

func FixtureTrade(name string) MobileTradeModel {
	model := MobileTradeModel{
		Open: true, OnlineSession: true, PartnerName: "Alice",
		CanAddItems: true, CanAddZeny: true, CanConclude: true, CanCancel: true,
		Inventory: []TradeInventoryItemModel{
			{Item: InventoryItemModel{Index: 1, ItemID: 501, DisplayName: "Red Potion", IconKey: "item-red-potion", Quantity: 8, Identified: true, Usable: true}, CanAdd: true},
			{Item: InventoryItemModel{Index: 2, ItemID: 1201, DisplayName: "Knife", IconKey: "item-knife", Quantity: 1, Identified: true, Equippable: true}, CanAdd: true},
			{Item: InventoryItemModel{Index: 3, ItemID: 909, DisplayName: "Jellopy", IconKey: "item-jellopy", Quantity: 12, Identified: true}, CanAdd: true},
			{Item: InventoryItemModel{Index: 4, ItemID: 601, DisplayName: "Fly Wing", IconKey: "item-fly-wing", Quantity: 5, Identified: true, Usable: true}, CanAdd: true},
		},
		AvailableZeny: 2500,
		OwnOffer:      []TradeOfferItemModel{{ItemIndex: 1, ItemID: 501, Name: "Red Potion", IconKey: "item-red-potion", Quantity: 3, Identified: true}},
		PartnerOffer:  []TradeOfferItemModel{{ItemID: 909, Name: "Jellopy", IconKey: "item-jellopy", Quantity: 12, Identified: true}},
		PartnerZeny:   1000,
	}
	for i := range model.Inventory {
		if model.Inventory[i].Item.Index == 1 {
			model.Inventory[i].Offered = true
		}
	}
	switch name {
	case "trade-empty":
		model.OwnOffer = nil
		model.PartnerOffer = nil
		model.OwnZeny, model.AvailableZeny, model.PartnerZeny = 0, 2500, 0
		for i := range model.Inventory {
			model.Inventory[i].Offered = false
		}
	case "trade-pending":
		model.Inventory[2].Pending = true
		model.Inventory[2].CanAdd = false
	case "trade-concluded":
		model.SelfConcluded, model.OtherConcluded = true, true
		model.CanAddItems, model.CanAddZeny, model.CanConclude = false, false, false
		model.CanCommit = true
	case "trade-request":
		model.Open = false
		model.PendingRequest = &MobileTradeRequestModel{TargetID: 7001, Level: 42, Name: "Balthasar"}
	case "trade-long-names":
		model.PartnerName = "A Very Long Character Name That Must Stay Inside The Trade Header"
		model.Inventory[0].Item.DisplayName = "A Very Long Item Name That Must Stay Inside The Inventory Row"
		model.PartnerOffer[0].Name = "A Very Long Received Item Name That Must Stay Inside The Offer Row"
	}
	return model
}
