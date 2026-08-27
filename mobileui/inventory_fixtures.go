package mobileui

import (
	"fmt"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/session"
)

type fixtureItemMetadata struct{}

func (fixtureItemMetadata) ItemDisplayName(itemID int, _ bool) (string, bool) {
	return fmt.Sprintf("Fixture Item %d", itemID), true
}
func (fixtureItemMetadata) ItemResourceName(itemID int, _ bool) (string, bool) {
	return fmt.Sprintf("fixture/item_%d", itemID), true
}
func (fixtureItemMetadata) ItemDescription(itemID int, _ bool) ([]string, bool) {
	return []string{fmt.Sprintf("Deterministic fixture description for item %d.", itemID)}, true
}
func (fixtureItemMetadata) ItemSlotCount(itemID int) (int, bool) {
	if itemID%3 == 0 {
		return 2, true
	}
	return 0, false
}

func FixtureInventory(name string) MobileInventoryModel {
	count := 8
	switch name {
	case "inventory-full", "inventory-large-quantity":
		count = 120
	case "equipment-full":
		count = 11
	case "inventory-long-names":
		count = 12
	case "inventory-empty":
		count = 0
	}
	s := &session.Session{}
	s.Inventory.Weight, s.Inventory.MaxWeight, s.Inventory.Zeny = 423, 1000, 12500
	for i := 0; i < count; i++ {
		itemType := []uint8{db.ItemTypeHealing, db.ItemTypeArmor, db.ItemTypeWeapon, db.ItemTypeEtc, db.ItemTypeCard}[i%5]
		item := session.InventoryItem{Index: uint16(i + 1), ItemID: uint16(500 + i), Type: itemType, Identified: true, Amount: (i % 7) + 1}
		if name == "inventory-large-quantity" && i == 0 {
			item.Amount = 999
		}
		if name == "inventory-long-names" && i == 0 {
			item.ItemID = 9999
		}
		if i == 1 || (name == "equipment-full" && i < 11) {
			item.Equipped = true
			item.Equip = true
			item.Location = []uint16{db.EquipHeadTop, db.EquipWeapon, db.EquipArmor, db.EquipShield, db.EquipGarment, db.EquipShoes, db.EquipAccessory1, db.EquipAccessory2, db.EquipHeadMid, db.EquipHeadBottom, db.EquipAmmo}[i%11]
		}
		s.Inventory.Items = append(s.Inventory.Items, item)
	}
	model := ProjectInventory(s, fixtureItemMetadata{})
	if name == "inventory-long-names" && len(model.Items) > 0 {
		model.Items[0].DisplayName = "A very long fixture item name that must remain inside the detail surface"
	}
	return model
}

func FixtureEquipment(name string) MobileEquipmentModel {
	if name == "equipment-partial" {
		return ProjectEquipment(FixtureInventory("normal"))
	}
	return ProjectEquipment(FixtureInventory("equipment-full"))
}
