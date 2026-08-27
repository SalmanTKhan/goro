package mobileui

import (
	"sort"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/res"
	"github.com/kivutar/goro/session"
)

type InventoryCategory uint8

const (
	InventoryCategoryAll InventoryCategory = iota
	InventoryCategoryEquipment
	InventoryCategoryUsable
	InventoryCategoryEtc
	InventoryCategoryCards
)

func (c InventoryCategory) String() string {
	switch c {
	case InventoryCategoryEquipment:
		return "Equipment"
	case InventoryCategoryUsable:
		return "Usable"
	case InventoryCategoryEtc:
		return "Etc"
	case InventoryCategoryCards:
		return "Cards"
	default:
		return "All"
	}
}

type InventoryItemModel struct {
	Index            uint16
	ItemID           uint16
	Identified       bool
	DisplayName      string
	IconKey          string
	Description      []string
	Quantity         int
	Type             uint8
	Category         InventoryCategory
	Equipped         bool
	Usable           bool
	Equippable       bool
	Refine           uint8
	Cards            [4]uint16
	EquipmentSlot    uint16
	DisabledReason   string
	NameAvailable    bool
	DescriptionKnown bool
}

type InventoryCategoryModel struct {
	Category InventoryCategory
	Label    string
	Count    int
	Enabled  bool
}

type InventorySelectionModel struct {
	SelectedIndex uint16
	HasSelection  bool
	Detail        ItemDetailModel
}

type ItemDetailModel struct {
	Visible          bool
	Item             InventoryItemModel
	PrimaryAction    string
	SecondaryAction  string
	PrimaryEnabled   bool
	SecondaryEnabled bool
}

type MobileInventoryModel struct {
	Weight     int
	MaxWeight  int
	Zeny       int64
	Items      []InventoryItemModel
	Categories []InventoryCategoryModel
	Selection  InventorySelectionModel
}

type EquipmentSlotModel struct {
	Label     string
	Location  uint16
	ItemIndex uint16
	HasItem   bool
	Item      InventoryItemModel
}

type MobileEquipmentModel struct {
	Slots     []EquipmentSlotModel
	Selection InventorySelectionModel
}

// ItemMetadataSource is implemented by res.Manager. Keeping this small avoids
// exposing resource managers or loaded assets through the mobile model.
type ItemMetadataSource interface {
	ItemDisplayName(itemID int, identified bool) (string, bool)
	ItemResourceName(itemID int, identified bool) (string, bool)
	ItemDescription(itemID int, identified bool) ([]string, bool)
	ItemSlotCount(itemID int) (int, bool)
}

var _ ItemMetadataSource = (*res.Manager)(nil)

func ProjectInventory(s *session.Session, metadata ItemMetadataSource) MobileInventoryModel {
	if s == nil {
		return MobileInventoryModel{Categories: inventoryCategories(nil)}
	}
	items := make([]InventoryItemModel, 0, len(s.Inventory.Items))
	for _, item := range s.Inventory.Items {
		items = append(items, projectInventoryItem(item, metadata))
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Index < items[j].Index })
	return MobileInventoryModel{
		Weight: s.Inventory.Weight, MaxWeight: s.Inventory.MaxWeight, Zeny: s.Inventory.Zeny,
		Items: items, Categories: inventoryCategories(items),
	}
}

func projectInventoryItem(item session.InventoryItem, metadata ItemMetadataSource) InventoryItemModel {
	model := InventoryItemModel{
		Index: item.Index, ItemID: item.ItemID, Identified: item.Identified, Quantity: item.Amount, Type: item.Type,
		Category: inventoryCategoryForType(item.Type), Equipped: item.Equipped,
		Usable: db.ItemTypeIsUsable(item.Type), Equippable: inventoryTypeIsEquippable(item.Type) || item.Equip,
		Refine: item.Refine, Cards: item.Cards, EquipmentSlot: item.Location,
	}
	if model.Quantity < 1 {
		model.Quantity = 1
	}
	if metadata != nil {
		model.DisplayName, model.NameAvailable = metadata.ItemDisplayName(int(item.ItemID), item.Identified)
		model.IconKey, _ = metadata.ItemResourceName(int(item.ItemID), item.Identified)
		model.Description, model.DescriptionKnown = metadata.ItemDescription(int(item.ItemID), item.Identified)
	}
	if !model.Usable && !model.Equippable {
		model.DisabledReason = "This item has no mobile action"
	}
	return model
}

func ProjectEquipment(inventory MobileInventoryModel) MobileEquipmentModel {
	result := MobileEquipmentModel{}
	for _, slot := range mobileEquipmentSlots() {
		entry := EquipmentSlotModel{Label: slot.label, Location: slot.location}
		for _, item := range inventory.Items {
			if item.Equipped && item.EquipmentSlot&slot.location != 0 {
				entry.Item, entry.ItemIndex, entry.HasItem = item, item.Index, true
				break
			}
		}
		result.Slots = append(result.Slots, entry)
	}
	return result
}

type mobileEquipmentSlot struct {
	label    string
	location uint16
}

func mobileEquipmentSlots() []mobileEquipmentSlot {
	return []mobileEquipmentSlot{
		{"Head Top", db.EquipHeadTop}, {"Head Mid", db.EquipHeadMid}, {"Head Low", db.EquipHeadBottom},
		{"Weapon", db.EquipWeapon}, {"Shield", db.EquipShield}, {"Armor", db.EquipArmor},
		{"Garment", db.EquipGarment}, {"Shoes", db.EquipShoes}, {"Accessory 1", db.EquipAccessory1},
		{"Accessory 2", db.EquipAccessory2}, {"Ammo", db.EquipAmmo},
	}
}

func inventoryCategoryForType(itemType uint8) InventoryCategory {
	switch {
	case itemType == db.ItemTypeCard:
		return InventoryCategoryCards
	case inventoryTypeIsEquippable(itemType):
		return InventoryCategoryEquipment
	case db.ItemTypeIsUsable(itemType):
		return InventoryCategoryUsable
	default:
		return InventoryCategoryEtc
	}
}

func inventoryTypeIsEquippable(itemType uint8) bool {
	switch itemType {
	case db.ItemTypeArmor, db.ItemTypeWeapon, db.ItemTypePetEgg, db.ItemTypePetArmor, db.ItemTypeAmmo, db.ItemTypeShadowGear:
		return true
	default:
		return false
	}
}

func inventoryCategories(items []InventoryItemModel) []InventoryCategoryModel {
	counts := map[InventoryCategory]int{}
	for _, item := range items {
		counts[item.Category]++
		counts[InventoryCategoryAll]++
	}
	result := make([]InventoryCategoryModel, 0, 5)
	for _, category := range []InventoryCategory{InventoryCategoryAll, InventoryCategoryEquipment, InventoryCategoryUsable, InventoryCategoryEtc, InventoryCategoryCards} {
		result = append(result, InventoryCategoryModel{Category: category, Label: category.String(), Count: counts[category], Enabled: category == InventoryCategoryAll || counts[category] > 0})
	}
	return result
}
