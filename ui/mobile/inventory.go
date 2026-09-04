package mobile

import (
	"strconv"
	"strings"

	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/mobileui"
)

// InventoryTree builds the inventory screen. It reads only the projected model,
// the layout rectangles and the interaction state — never the session — so the
// same call renders identically in a test, a preview and on a device.
//
// Item sprites are deliberately absent: the Android host draws them from the
// resource path directly onto the frame, over this raster. IconRects reports
// where they go.
func (k Kit) InventoryTree(
	model mobileui.MobileInventoryModel,
	layout mobileui.MobileInventoryLayout,
	state mobileui.InventoryInteractionState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	// No full-screen fill: the mobile screens float over the live game world,
	// the way the desktop client's windows do. Painting the whole safe area
	// would turn every uncovered pixel into blank chrome.
	k.placeInventoryHeader(c, model, layout)
	k.placeInventoryTabs(c, layout, state)
	k.placeInventoryGrid(c, model, layout, state)
	k.placeItemDetail(c, layout, state)
	k.placeQuantityModal(c, layout, state)
	return c
}

func (k Kit) placeInventoryHeader(c *Canvas, model mobileui.MobileInventoryModel, layout mobileui.MobileInventoryLayout) {
	if layout.Header.W <= 0 {
		return
	}
	c.Place(k.Panel(), layout.Header)
	c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)
	c.Place(k.Button("Equipment", ButtonNormal), layout.EquipmentButton)

	// Weight and zeny sit between the two header buttons rather than in the
	// title, which is where a player looks for them in the desktop client.
	between := mobileui.Rect{
		X: layout.BackButton.Right() + k.Theme.Metrics.TableCellPadX,
		Y: layout.Header.Y,
		W: layout.EquipmentButton.X - layout.BackButton.Right() - 2*k.Theme.Metrics.TableCellPadX,
		H: layout.Header.H,
	}
	if between.W <= 0 {
		return
	}
	half := between.H / 2
	c.Place(k.Centered(weightLabel(model), RoleValue), mobileui.Rect{X: between.X, Y: between.Y, W: between.W, H: half})
	c.Place(k.Centered(zenyLabel(model.Zeny), RoleMuted), mobileui.Rect{X: between.X, Y: between.Y + half, W: between.W, H: half})
}

func (k Kit) placeInventoryTabs(c *Canvas, layout mobileui.MobileInventoryLayout, state mobileui.InventoryInteractionState) {
	for _, tab := range layout.Tabs {
		active := tab.Category == state.Category
		label := categoryLabel(tab.Category)
		btn := ButtonNormal
		if active {
			btn = ButtonPressed
		}
		c.Place(k.Button(label, btn), tab.Rect)
	}
}

func (k Kit) placeInventoryGrid(
	c *Canvas,
	model mobileui.MobileInventoryModel,
	layout mobileui.MobileInventoryLayout,
	state mobileui.InventoryInteractionState,
) {
	if layout.GridViewport.W <= 0 || layout.GridViewport.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.GridViewport)

	items := state.FilteredItems(model)
	for _, cell := range layout.Cells {
		// A cell outside the viewport is scrolled away; skip it rather than
		// relying on a clip, so the raster stays cheap.
		if !cell.Rect.Intersects(layout.GridViewport) {
			continue
		}
		item, occupied := inventoryItemAt(items, cell.Index)
		selected := occupied && state.Selection.HasSelection && state.Selection.SelectedIndex == cell.Index
		c.Place(k.Slot(occupied, selected), cell.Rect)
		if occupied && item.Quantity > 1 {
			c.Place(k.quantityBadge(item.Quantity), quantityBadgeRect(cell.Rect, k.Theme.Metrics.TableCellPadX))
		}
	}
}

func (k Kit) placeItemDetail(c *Canvas, layout mobileui.MobileInventoryLayout, state mobileui.InventoryInteractionState) {
	detail := state.Selection.Detail
	if !detail.Visible || layout.DetailPanel.W <= 0 {
		return
	}
	// A plain panel, not a titled window: mobileui's detail layout positions the
	// item name at the top of the panel itself, so window chrome would sit on
	// top of it.
	c.Place(k.Panel(), layout.DetailPanel)
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

func (k Kit) placeQuantityModal(c *Canvas, layout mobileui.MobileInventoryLayout, state mobileui.InventoryInteractionState) {
	if !state.Quantity.Open || layout.QuantityModal.W <= 0 {
		return
	}
	// The scrim covers the safe area so nothing behind the modal reads as
	// tappable, matching mobileui's modal touch gating.
	c.Place(k.Scrim(), layout.Safe)
	c.Place(k.Window("Drop", nil), layout.QuantityModal)
	c.Place(k.Button("−", ButtonNormal), layout.QuantityMinus)
	c.Place(k.Button("+", ButtonNormal), layout.QuantityPlus)
	c.Place(k.Button("Confirm", ButtonNormal), layout.QuantityConfirm)
	c.Place(k.Button("Cancel", ButtonNormal), layout.QuantityCancel)

	// The layout has no rectangle for the amount itself: it belongs in the
	// modal body, between the title bar and the button row.
	bodyTop := layout.QuantityModal.Y + k.Theme.Metrics.WindowTitleHeight
	body := mobileui.Rect{
		X: layout.QuantityModal.X,
		Y: bodyTop,
		W: layout.QuantityModal.W,
		H: layout.QuantityMinus.Y - bodyTop,
	}
	if body.H <= 0 {
		return
	}
	label := mobileui.Rect{X: body.X, Y: body.Y, W: body.W, H: body.H * 0.4}
	value := mobileui.Rect{X: body.X, Y: body.Y + body.H*0.4, W: body.W, H: body.H * 0.6}
	c.Place(k.Centered(quantityPrompt(state), RoleMuted), label)
	c.Place(k.Centered(strconv.Itoa(state.Quantity.Value), RoleTitle), value)
}

// quantityPrompt names what is being dropped and how many are available.
func quantityPrompt(state mobileui.InventoryInteractionState) string {
	name := state.Selection.Detail.Item.DisplayName
	if name == "" {
		return "Amount"
	}
	return name + "  (max " + strconv.Itoa(state.Quantity.Maximum) + ")"
}

// quantityBadge is the small count drawn over an item sprite.
func (k Kit) quantityBadge(quantity int) widget.Widget {
	return k.Centered("x"+strconv.Itoa(quantity), RoleMuted)
}

func quantityBadgeRect(cell mobileui.Rect, pad float32) mobileui.Rect {
	h := cell.H * 0.32
	return mobileui.Rect{X: cell.X, Y: cell.Bottom() - h - pad/2, W: cell.W - pad/2, H: h}
}

// IconRects reports where the host must draw real item sprites, in the order
// the grid presents them. Keeping this beside the builder means the raster and
// the sprite pass cannot disagree about cell geometry.
func IconRects(
	model mobileui.MobileInventoryModel,
	layout mobileui.MobileInventoryLayout,
	state mobileui.InventoryInteractionState,
) []IconPlacement {
	items := state.FilteredItems(model)
	out := make([]IconPlacement, 0, len(layout.Cells))
	for _, cell := range layout.Cells {
		if !cell.Rect.Intersects(layout.GridViewport) {
			continue
		}
		item, occupied := inventoryItemAt(items, cell.Index)
		if !occupied {
			continue
		}
		// The quantity badge occupies the bottom of the cell, so the sprite gets
		// the space above it rather than being scaled down inside the whole cell.
		art := cell.Rect
		art.H = spriteHeightWithinCell(cell.Rect)
		out = append(out, IconPlacement{Item: item, Rect: art})
	}
	return out
}

// IconPlacement pairs an item with the cell its sprite belongs in.
type IconPlacement struct {
	Item mobileui.InventoryItemModel
	Rect mobileui.Rect
}

func inventoryItemAt(items []mobileui.InventoryItemModel, index uint16) (mobileui.InventoryItemModel, bool) {
	for _, item := range items {
		if item.Index == index {
			return item, true
		}
	}
	return mobileui.InventoryItemModel{}, false
}

func buttonStateFor(enabled bool) ButtonState {
	if enabled {
		return ButtonNormal
	}
	return ButtonDisabled
}

func categoryLabel(category mobileui.InventoryCategory) string {
	switch category {
	case mobileui.InventoryCategoryEquipment:
		return "Equip"
	case mobileui.InventoryCategoryUsable:
		return "Use"
	case mobileui.InventoryCategoryEtc:
		return "Etc"
	case mobileui.InventoryCategoryCards:
		return "Cards"
	default:
		return "All"
	}
}

// itemTitle names the detail sub-window. The item's own name is rendered inside
// the body, so the title stays constant the way the desktop item window does.
func itemTitle() string { return "Item Info" }

func itemMeta(item mobileui.InventoryItemModel) string {
	meta := ""
	if item.Refine > 0 {
		meta = "+" + strconv.Itoa(int(item.Refine)) + " "
	}
	if item.Quantity > 1 {
		meta += "x" + strconv.Itoa(item.Quantity)
	}
	if !item.Identified {
		if meta != "" {
			meta += "  "
		}
		meta += "Unidentified"
	}
	return meta
}

// joinLines flattens an item description into one wrappable paragraph. RO
// descriptions arrive as pre-broken lines carrying ^RRGGBB colour codes; the
// codes are stripped (mobile draws descriptions in a single colour) and the
// lines are joined with spaces so the text widget can re-wrap them to the
// panel width rather than inheriting the desktop client's line breaks.
func joinLines(lines []string) string {
	var out strings.Builder
	for _, line := range lines {
		clean := strings.TrimSpace(mobileui.StripROText(line))
		if clean == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteByte(' ')
		}
		out.WriteString(clean)
	}
	return out.String()
}

func weightLabel(model mobileui.MobileInventoryModel) string {
	return "Weight " + strconv.Itoa(model.Weight) + " / " + strconv.Itoa(model.MaxWeight)
}

func zenyLabel(zeny int64) string {
	return groupDigits(zeny) + " z"
}

// groupDigits renders a zeny amount with thousands separators, the way the
// desktop client shows currency.
func groupDigits(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	digits := strconv.FormatInt(value, 10)
	var out []byte
	for i, d := range []byte(digits) {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, d)
	}
	if negative {
		return "-" + string(out)
	}
	return string(out)
}

// spriteHeightWithinCell reserves the quantity badge's strip along the bottom of
// a grid cell, so the art above it is drawn at full size.
func spriteHeightWithinCell(cell mobileui.Rect) float32 {
	return cell.H * 0.68
}
