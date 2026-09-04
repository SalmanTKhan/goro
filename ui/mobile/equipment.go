package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// EquipmentTree builds the equipment screen: a paper doll of equip slots with a
// summary or item detail beside or below it, depending on orientation.
//
// Like the inventory it draws no item art — EquipmentIconRects reports where the
// host composites the real sprites.
func (k Kit) EquipmentTree(
	model mobileui.MobileEquipmentModel,
	layout mobileui.MobileInventoryLayout,
	state mobileui.InventoryInteractionState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)

	k.placeEquipmentHeader(c, layout)
	k.placePaperDoll(c, model, layout, state)
	k.placeEquipmentDetail(c, model, layout, state)
	return c
}

func (k Kit) placeEquipmentHeader(c *Canvas, layout mobileui.MobileInventoryLayout) {
	if layout.Header.W <= 0 {
		return
	}
	c.Place(k.Panel(), layout.Header)
	c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)

	title := mobileui.Rect{
		X: layout.BackButton.Right() + k.Theme.Metrics.TableCellPadX,
		Y: layout.Header.Y,
		W: layout.Header.Right() - layout.BackButton.Right() - 2*k.Theme.Metrics.TableCellPadX,
		H: layout.Header.H,
	}
	if title.W > 0 {
		c.Place(k.Centered("Equipment", RoleTitle), title)
	}
}

func (k Kit) placePaperDoll(
	c *Canvas,
	model mobileui.MobileEquipmentModel,
	layout mobileui.MobileInventoryLayout,
	state mobileui.InventoryInteractionState,
) {
	if layout.PaperDoll.W <= 0 || layout.PaperDoll.H <= 0 {
		return
	}
	// The paper doll surface follows its slot cluster rather than filling the
	// content band, so the world stays visible below it.
	c.Place(k.Panel(), paperDollSurface(layout))

	for _, slot := range layout.PaperDollSlots {
		equipped, filled := equipmentSlotAt(model, slot.Location)
		selected := filled && state.Selection.HasSelection && state.Selection.SelectedIndex == equipped.ItemIndex
		c.Place(k.Slot(filled, selected), slot.Rect)

		// An empty slot names itself so the player knows what belongs there; an
		// occupied one gives its label to the sprite instead.
		if !filled {
			c.Place(k.Centered(slot.Label, RoleMuted), slot.Rect)
			continue
		}
		if equipped.Item.Refine > 0 {
			c.Place(k.Centered(refineLabel(equipped.Item.Refine), RoleMuted),
				quantityBadgeRect(slot.Rect, k.Theme.Metrics.TableCellPadX))
		}
	}
}

// paperDollSurface bounds the paper-doll panel to the slots it actually holds.
func paperDollSurface(layout mobileui.MobileInventoryLayout) mobileui.Rect {
	surface := layout.PaperDoll
	var bottom float32
	for _, slot := range layout.PaperDollSlots {
		if slot.Rect.Bottom() > bottom {
			bottom = slot.Rect.Bottom()
		}
	}
	if bottom <= surface.Y {
		return surface
	}
	if h := bottom - surface.Y + 12; h < surface.H {
		surface.H = h
	}
	return surface
}

func (k Kit) placeEquipmentDetail(
	c *Canvas,
	model mobileui.MobileEquipmentModel,
	layout mobileui.MobileInventoryLayout,
	state mobileui.InventoryInteractionState,
) {
	if layout.DetailPanel.W <= 0 || layout.DetailPanel.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.DetailPanel)

	detail := state.Selection.Detail
	if !detail.Visible {
		// Nothing selected: summarise what is worn, which is more useful than an
		// empty card.
		k.placeEquipmentSummary(c, model, layout.DetailPanel)
		return
	}
	c.Place(k.Wrapped(detail.Item.DisplayName, RoleTitle, 2), layout.DetailTitle)
	c.Place(k.Wrapped(itemMeta(detail.Item), RoleMuted, 2), layout.DetailMeta)
	c.Place(k.Wrapped(joinLines(detail.Item.Description), RoleBody, 0), layout.DetailDescription)
	if detail.PrimaryAction != "" {
		c.Place(k.Button(detail.PrimaryAction, buttonStateFor(detail.PrimaryEnabled)), layout.PrimaryAction)
	}
	if detail.SecondaryAction != "" {
		c.Place(k.Button(detail.SecondaryAction, buttonStateFor(detail.SecondaryEnabled)), layout.SecondaryAction)
	}
}

// placeEquipmentSummary lists the worn items as label/value rows.
func (k Kit) placeEquipmentSummary(c *Canvas, model mobileui.MobileEquipmentModel, panel mobileui.Rect) {
	pad := k.Theme.Metrics.TableCellPadX
	rowH := k.Theme.Metrics.TableRowHeight
	y := panel.Y + pad

	c.Place(k.Text("Equipped", RoleTitle), mobileui.Rect{X: panel.X + pad, Y: y, W: panel.W - 2*pad, H: rowH})
	y += rowH

	for _, slot := range model.Slots {
		if y+rowH > panel.Bottom()-pad {
			return
		}
		if !slot.HasItem {
			continue
		}
		labelW := (panel.W - 2*pad) * 0.42
		c.Place(k.Text(slot.Label, RoleLabel), mobileui.Rect{X: panel.X + pad, Y: y, W: labelW, H: rowH})
		c.Place(k.Text(slot.Item.DisplayName, RoleValue),
			mobileui.Rect{X: panel.X + pad + labelW, Y: y, W: panel.W - 2*pad - labelW, H: rowH})
		y += rowH
	}
}

// EquipmentIconRects reports where equipped item sprites belong.
func EquipmentIconRects(
	model mobileui.MobileEquipmentModel,
	layout mobileui.MobileInventoryLayout,
) []IconPlacement {
	out := make([]IconPlacement, 0, len(layout.PaperDollSlots))
	for _, slot := range layout.PaperDollSlots {
		equipped, filled := equipmentSlotAt(model, slot.Location)
		if !filled {
			continue
		}
		out = append(out, IconPlacement{Item: equipped.Item, Rect: slot.Rect})
	}
	return out
}

func equipmentSlotAt(model mobileui.MobileEquipmentModel, location uint16) (mobileui.EquipmentSlotModel, bool) {
	for _, slot := range model.Slots {
		if slot.Location == location && slot.HasItem {
			return slot, true
		}
	}
	return mobileui.EquipmentSlotModel{}, false
}

func refineLabel(refine uint8) string {
	return "+" + strconv.Itoa(int(refine))
}
