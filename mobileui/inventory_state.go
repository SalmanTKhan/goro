package mobileui

import "github.com/kivutar/goro/input"

type InventoryScrollState = ScrollState

type QuantityState struct {
	Open      bool
	ItemIndex uint16
	Minimum   int
	Maximum   int
	Value     int
}

func (q QuantityState) Active() bool { return q.Open }
func (q *QuantityState) OpenFor(item InventoryItemModel) {
	if q == nil {
		return
	}
	max := item.Quantity
	if max < 1 {
		max = 1
	}
	*q = QuantityState{Open: true, ItemIndex: item.Index, Minimum: 1, Maximum: max, Value: max}
}
func (q *QuantityState) Increment() {
	if q != nil {
		q.SetValue(q.Value + 1)
	}
}
func (q *QuantityState) Decrement() {
	if q != nil {
		q.SetValue(q.Value - 1)
	}
}
func (q *QuantityState) SetValue(value int) {
	if q == nil {
		return
	}
	if value < q.Minimum {
		value = q.Minimum
	}
	if value > q.Maximum {
		value = q.Maximum
	}
	q.Value = value
}
func (q *QuantityState) Confirm() (input.PlayerCommand, bool) {
	if q == nil || !q.Open || q.ItemIndex == 0 || q.Value < 1 {
		return input.PlayerCommand{}, false
	}
	command := input.PlayerCommand{Kind: input.CommandDropItem, ItemIndex: q.ItemIndex, Quantity: q.Value}
	*q = QuantityState{}
	return command, true
}
func (q *QuantityState) Cancel() {
	if q != nil {
		*q = QuantityState{}
	}
}

type InventoryInteractionState struct {
	Screen           Screen
	Category         InventoryCategory
	Selection        InventorySelectionModel
	Scroll           InventoryScrollState
	Quantity         QuantityState
	DetailOpen       bool
	CategoryMenuOpen bool
}

func (s *InventoryInteractionState) FilteredItems(model MobileInventoryModel) []InventoryItemModel {
	if s == nil || s.Category == InventoryCategoryAll {
		return append([]InventoryItemModel(nil), model.Items...)
	}
	items := make([]InventoryItemModel, 0)
	for _, item := range model.Items {
		if item.Category == s.Category {
			items = append(items, item)
		}
	}
	return items
}

func (s *InventoryInteractionState) Select(model MobileInventoryModel, index uint16) bool {
	if s == nil {
		return false
	}
	for _, item := range model.Items {
		if item.Index != index {
			continue
		}
		s.Selection = InventorySelectionModel{SelectedIndex: index, HasSelection: true, Detail: itemDetail(item)}
		s.DetailOpen = true
		return true
	}
	return false
}

func (s *InventoryInteractionState) SetCategory(category InventoryCategory) {
	if s == nil {
		return
	}
	s.Category, s.Scroll.Offset, s.CategoryMenuOpen = category, 0, false
}

func itemDetail(item InventoryItemModel) ItemDetailModel {
	detail := ItemDetailModel{Visible: true, Item: item}
	switch {
	case item.Equipped:
		detail.PrimaryAction, detail.PrimaryEnabled = "Unequip", true
	case item.Equippable:
		detail.PrimaryAction, detail.PrimaryEnabled = "Equip", true
	case item.Usable:
		detail.PrimaryAction, detail.PrimaryEnabled = "Use", true
	default:
		detail.PrimaryAction = "Unavailable"
	}
	detail.SecondaryAction = "Drop"
	detail.SecondaryEnabled = item.Quantity > 0
	return detail
}

func containsItem(items []InventoryItemModel, index uint16) bool {
	for _, item := range items {
		if item.Index == index {
			return true
		}
	}
	return false
}
func clampf(value, low, high float32) float32 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
