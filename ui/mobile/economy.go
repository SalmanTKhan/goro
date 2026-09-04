package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// ShopTree builds the NPC shop: buy and sell tabs over a list of goods, with
// the quantity modal on top when a purchase is being sized.
func (k Kit) ShopTree(
	model mobileui.MobileShopModel,
	layout mobileui.EconomyLayout,
	tab mobileui.ShopTab,
	quantity mobileui.EconomyQuantityState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), economyPanelSurface(layout))
	k.placeEconomyHeader(c, layout, shopTitle(model), zenyLabel(model.Zeny))
	k.placeEconomyTabs(c, layout, tab)

	items := model.Items
	if tab == mobileui.ShopSellTab {
		items = model.SellItems
	}
	k.placeShopRows(c, items, layout, tab)
	k.placeEconomyQuantity(c, layout, quantity)
	return c
}

func shopTitle(model mobileui.MobileShopModel) string {
	if model.Name == "" {
		return "Shop"
	}
	return model.Name
}

// StorageTree builds the Kafra storage list.
func (k Kit) StorageTree(
	model mobileui.MobileStorageModel,
	layout mobileui.EconomyLayout,
	quantity mobileui.EconomyQuantityState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), economyPanelSurface(layout))
	k.placeEconomyHeader(c, layout, "Storage", storageCapacity(model))
	k.placeStorageRows(c, model.Items, layout)
	k.placeEconomyQuantity(c, layout, quantity)
	return c
}

func storageCapacity(model mobileui.MobileStorageModel) string {
	return strconv.Itoa(model.Amount) + " / " + strconv.Itoa(model.MaxAmount)
}

// placeEconomyHeader draws the title, a summary figure and the close control.
func (k Kit) placeEconomyHeader(c *Canvas, layout mobileui.EconomyLayout, title, summary string) {
	if layout.Header.W <= 0 {
		return
	}
	pad := k.Theme.Metrics.TableCellPadX
	if layout.Close.W > 0 {
		c.Place(k.Button("Close", ButtonNormal), layout.Close)
	}
	right := layout.Header.Right() - pad
	if layout.Close.W > 0 && layout.Close.Y < layout.Header.Bottom() {
		right = layout.Close.X - pad
	}
	inner := mobileui.Rect{X: layout.Header.X + pad, Y: layout.Header.Y, H: layout.Header.H}
	inner.W = max32(0, right-inner.X)
	if inner.W <= 0 {
		return
	}
	// Title on the left, the money or capacity figure on the right: both are
	// read at a glance before touching anything.
	summaryW := inner.W * 0.4
	c.Place(k.Text(title, RoleTitle), mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W - summaryW, H: inner.H})
	if summary != "" {
		c.Place(k.RightAligned(summary, RoleValue),
			mobileui.Rect{X: inner.X + inner.W - summaryW, Y: inner.Y, W: summaryW, H: inner.H})
	}
}

func (k Kit) placeEconomyTabs(c *Canvas, layout mobileui.EconomyLayout, selected mobileui.ShopTab) {
	for _, tab := range layout.Tabs {
		if tab.Rect.W <= 0 || tab.Rect.H <= 0 {
			continue
		}
		c.Place(k.Button(shopTabLabel(tab.Tab), buttonStateForOpen(tab.Tab == selected)), tab.Rect)
	}
}

func shopTabLabel(tab mobileui.ShopTab) string {
	if tab == mobileui.ShopSellTab {
		return "Sell"
	}
	return "Buy"
}

// placeShopRows lists goods as name, price and availability.
func (k Kit) placeShopRows(
	c *Canvas,
	items []mobileui.ShopItemModel,
	layout mobileui.EconomyLayout,
	tab mobileui.ShopTab,
) {
	if layout.ListViewport.W > 0 && layout.ListViewport.H > 0 {
		c.Place(k.Panel(), listPanel(layout))
	}
	pad := k.Theme.Metrics.TableCellPadX
	for i, row := range layout.Rows {
		index := rowIndex(layout, i)
		if index < 0 || index >= len(items) || row.W <= 0 || row.H <= 0 {
			continue
		}
		item := items[index]
		c.Place(k.Card(false), row)

		// A square at the leading edge holds the item sprite; the host draws it
		// over this raster from the resource path.
		iconW := row.H
		inner := mobileui.Rect{X: row.X + iconW + pad, Y: row.Y, W: row.Right() - pad - (row.X + iconW + pad), H: row.H}
		if inner.W <= 0 {
			continue
		}
		titleRect, subtitleRect := k.RowLines(row, iconW)
		if titleRect.W <= 0 {
			continue
		}
		priceW := titleRect.W * 0.34
		nameW := titleRect.W - priceW
		c.Place(k.Content(item.Name, RoleValue),
			mobileui.Rect{X: titleRect.X, Y: titleRect.Y, W: nameW, H: titleRect.H})
		c.Place(k.Text(shopSubtitle(item, tab), RoleMuted),
			mobileui.Rect{X: subtitleRect.X, Y: subtitleRect.Y, W: nameW, H: subtitleRect.H})
		c.Place(k.RightAligned(zenyLabel(shopPrice(item, tab)), RoleValue),
			mobileui.Rect{X: titleRect.X + nameW, Y: titleRect.Y, W: priceW, H: titleRect.H + subtitleRect.H})
	}
}

func shopPrice(item mobileui.ShopItemModel, tab mobileui.ShopTab) int64 {
	if tab == mobileui.ShopSellTab {
		return item.SellPrice
	}
	return item.Price
}

// shopSubtitle explains availability, or why the row cannot be acted on.
func shopSubtitle(item mobileui.ShopItemModel, tab mobileui.ShopTab) string {
	if item.DisabledReason != "" {
		return item.DisabledReason
	}
	if tab == mobileui.ShopSellTab {
		if item.Quantity > 0 {
			return "You have " + strconv.Itoa(item.Quantity)
		}
		return ""
	}
	if item.Stock > 0 {
		return "Stock " + strconv.Itoa(item.Stock)
	}
	return ""
}

// placeStorageRows lists stored items with their counts.
func (k Kit) placeStorageRows(
	c *Canvas,
	items []mobileui.InventoryItemModel,
	layout mobileui.EconomyLayout,
) {
	if layout.ListViewport.W > 0 && layout.ListViewport.H > 0 {
		c.Place(k.Panel(), listPanel(layout))
	}
	pad := k.Theme.Metrics.TableCellPadX
	for i, row := range layout.Rows {
		index := rowIndex(layout, i)
		if index < 0 || index >= len(items) || row.W <= 0 || row.H <= 0 {
			continue
		}
		item := items[index]
		c.Place(k.Card(false), row)

		// A square at the leading edge holds the item sprite.
		iconW := row.H
		inner := mobileui.Rect{X: row.X + iconW + pad, Y: row.Y, W: row.Right() - pad - (row.X + iconW + pad), H: row.H}
		if inner.W <= 0 {
			continue
		}
		countW := inner.W * 0.24
		c.Place(k.Content(item.DisplayName, RoleValue),
			mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W - countW, H: inner.H})
		if item.Quantity > 1 {
			c.Place(k.RightAligned("x"+strconv.Itoa(item.Quantity), RoleMuted),
				mobileui.Rect{X: inner.X + inner.W - countW, Y: inner.Y, W: countW, H: inner.H})
		}
	}
}

// placeEconomyQuantity draws the amount modal shared by buy, sell and withdraw.
func (k Kit) placeEconomyQuantity(c *Canvas, layout mobileui.EconomyLayout, state mobileui.EconomyQuantityState) {
	if !state.Open || layout.QuantityModal.W <= 0 {
		return
	}
	c.Place(k.Scrim(), layout.Safe)
	c.Place(k.Window(quantityTitle(state.Action), nil), layout.QuantityModal)
	if layout.QuantityMinus.W > 0 {
		c.Place(k.Button("−", ButtonNormal), layout.QuantityMinus)
	}
	if layout.QuantityPlus.W > 0 {
		c.Place(k.Button("+", ButtonNormal), layout.QuantityPlus)
	}
	if layout.QuantityConfirm.W > 0 {
		c.Place(k.Button("Confirm", ButtonNormal), layout.QuantityConfirm)
	}
	if layout.QuantityCancel.W > 0 {
		c.Place(k.Button("Cancel", ButtonNormal), layout.QuantityCancel)
	}

	// The amount belongs in the modal body, between the title bar and buttons.
	bodyTop := layout.QuantityModal.Y + k.Theme.Metrics.WindowTitleHeight
	bodyBottom := layout.QuantityModal.Bottom()
	if layout.QuantityMinus.H > 0 {
		bodyBottom = layout.QuantityMinus.Y
	}
	body := mobileui.Rect{X: layout.QuantityModal.X, Y: bodyTop, W: layout.QuantityModal.W, H: bodyBottom - bodyTop}
	if body.H <= 0 {
		return
	}
	label := mobileui.Rect{X: body.X, Y: body.Y, W: body.W, H: body.H * 0.4}
	value := mobileui.Rect{X: body.X, Y: body.Y + body.H*0.4, W: body.W, H: body.H * 0.6}
	c.Place(k.Centered("Max "+strconv.Itoa(state.Maximum), RoleMuted), label)
	c.Place(k.Centered(strconv.Itoa(state.Value), RoleTitle), value)
}

func quantityTitle(action mobileui.EconomyQuantityAction) string {
	switch action {
	case mobileui.EconomyQuantitySell:
		return "Sell"
	case mobileui.EconomyQuantityWithdraw:
		return "Withdraw"
	default:
		return "Buy"
	}
}

// economyPanelSurface bounds the backing sheet to its content, so a short shop
// does not paint a full-height slab over the world.
func economyPanelSurface(layout mobileui.EconomyLayout) mobileui.Rect {
	panel := layout.Panel
	bottom := listPanel(layout).Bottom() + 12
	if bottom > panel.Y && bottom < panel.Bottom() {
		panel.H = bottom - panel.Y
	}
	return panel
}

// listPanel bounds the list surface to the rows actually present, so a short
// shop does not paint a full-height sheet.
func listPanel(layout mobileui.EconomyLayout) mobileui.Rect {
	surface := layout.ListViewport
	var bottom float32
	for _, row := range layout.Rows {
		if row.Intersects(layout.ListViewport) && row.Bottom() > bottom {
			bottom = row.Bottom()
		}
	}
	if bottom <= surface.Y {
		return surface
	}
	if h := bottom - surface.Y + 8; h < surface.H {
		surface.H = h
	}
	return surface
}

// rowIndex maps a visible row back to its model index. The layout publishes
// RowIndices when the visible window is a subset of the model.
func rowIndex(layout mobileui.EconomyLayout, row int) int {
	if row < len(layout.RowIndices) {
		return layout.RowIndices[row]
	}
	return row
}

// ShopIconRects reports where the goods' sprites belong.
func ShopIconRects(items []mobileui.ShopItemModel, layout mobileui.EconomyLayout) []IconPlacement {
	out := make([]IconPlacement, 0, len(layout.Rows))
	for i, row := range layout.Rows {
		index := rowIndex(layout, i)
		if index < 0 || index >= len(items) || row.W <= 0 {
			continue
		}
		item := items[index]
		out = append(out, IconPlacement{
			Item: mobileui.InventoryItemModel{
				ItemID: item.ItemID, Identified: true, DisplayName: item.Name, Quantity: item.Quantity,
			},
			Rect: mobileui.Rect{X: row.X, Y: row.Y, W: row.H, H: row.H},
		})
	}
	return out
}

// StorageIconRects reports where stored item sprites belong.
func StorageIconRects(model mobileui.MobileStorageModel, layout mobileui.EconomyLayout) []IconPlacement {
	out := make([]IconPlacement, 0, len(layout.Rows))
	for i, row := range layout.Rows {
		index := rowIndex(layout, i)
		if index < 0 || index >= len(model.Items) || row.W <= 0 {
			continue
		}
		out = append(out, IconPlacement{
			Item: model.Items[index],
			Rect: mobileui.Rect{X: row.X, Y: row.Y, W: row.H, H: row.H},
		})
	}
	return out
}
