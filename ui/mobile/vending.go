package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// VendingTree builds another player's vending shop: the goods on sale, the
// selected item's detail, and the purchase quantity modal.
func (k Kit) VendingTree(
	model mobileui.MobileVendingModel,
	layout mobileui.VendingLayout,
	selected int,
	quantity mobileui.VendingQuantityState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	k.placeVendingHeader(c, model, layout)
	k.placeVendingRows(c, model, layout, selected)
	k.placeVendingDetail(c, model, layout, selected)
	k.placeVendingQuantity(c, layout, quantity)
	return c
}

func (k Kit) placeVendingHeader(c *Canvas, model mobileui.MobileVendingModel, layout mobileui.VendingLayout) {
	if layout.Back.W > 0 {
		c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.Back)
	}
	if layout.Close.W > 0 {
		c.Place(k.Button("Close", ButtonNormal), layout.Close)
	}
	if layout.Header.W <= 0 {
		return
	}
	pad := k.Theme.Metrics.TableCellPadX
	left := layout.Header.X + pad
	if layout.Back.W > 0 {
		left = layout.Back.Right() + pad
	}
	right := layout.Header.Right() - pad
	if layout.Close.W > 0 {
		right = layout.Close.X - pad
	}
	inner := mobileui.Rect{X: left, Y: layout.Header.Y, W: right - left, H: layout.Header.H}
	if inner.W <= 0 {
		return
	}
	// Shop name over the player's own purse: you need both to decide.
	half := inner.H / 2
	c.Place(k.Centered(vendingTitle(model), RoleTitle), mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: half})
	c.Place(k.Centered(zenyLabel(model.Zeny), RoleMuted), mobileui.Rect{X: inner.X, Y: inner.Y + half, W: inner.W, H: half})
}

func vendingTitle(model mobileui.MobileVendingModel) string {
	if model.ShopName != "" {
		return model.ShopName
	}
	return "Vending"
}

func (k Kit) placeVendingRows(
	c *Canvas,
	model mobileui.MobileVendingModel,
	layout mobileui.VendingLayout,
	selected int,
) {
	if layout.ListViewport.W <= 0 || layout.ListViewport.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.ListViewport)
	if model.Loading {
		c.Place(k.Centered("Loading…", RoleMuted), layout.ListViewport)
		return
	}
	if len(model.Items) == 0 {
		c.Place(k.Centered("Nothing for sale", RoleMuted), layout.ListViewport)
		return
	}

	pad := k.Theme.Metrics.TableCellPadX
	for i, row := range layout.Rows {
		index := vendingRowIndex(layout, i)
		if index < 0 || index >= len(model.Items) || row.W <= 0 || row.H <= 0 {
			continue
		}
		item := model.Items[index]
		c.Place(k.Card(index == selected), row)

		inner := mobileui.Rect{X: row.X + row.H + pad, Y: row.Y, W: row.Right() - pad - (row.X + row.H + pad), H: row.H}
		if inner.W <= 0 {
			continue
		}
		titleRect, subtitleRect := k.RowLines(row, row.H)
		if titleRect.W <= 0 {
			continue
		}
		priceW := titleRect.W * 0.34
		nameW := titleRect.W - priceW
		c.Place(k.Content(vendingItemName(item), RoleValue),
			mobileui.Rect{X: titleRect.X, Y: titleRect.Y, W: nameW, H: titleRect.H})
		c.Place(k.Text(vendingSubtitle(item), RoleMuted),
			mobileui.Rect{X: subtitleRect.X, Y: subtitleRect.Y, W: nameW, H: subtitleRect.H})
		c.Place(k.RightAligned(zenyLabel(item.Price), RoleValue),
			mobileui.Rect{X: titleRect.X + nameW, Y: titleRect.Y, W: priceW, H: titleRect.H + subtitleRect.H})
	}
}

func vendingItemName(item mobileui.VendingItemModel) string {
	if item.Refine > 0 {
		return "+" + strconv.Itoa(int(item.Refine)) + " " + item.Name
	}
	return item.Name
}

func vendingSubtitle(item mobileui.VendingItemModel) string {
	if item.DisabledReason != "" {
		return item.DisabledReason
	}
	if item.Quantity > 0 {
		return "Stock " + strconv.Itoa(item.Quantity)
	}
	return ""
}

func (k Kit) placeVendingDetail(
	c *Canvas,
	model mobileui.MobileVendingModel,
	layout mobileui.VendingLayout,
	selected int,
) {
	if layout.DetailPanel.W <= 0 || layout.DetailPanel.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.DetailPanel)
	pad := k.Theme.Metrics.TableCellPadX
	inner := mobileui.Rect{
		X: layout.DetailPanel.X + pad, Y: layout.DetailPanel.Y + pad,
		W: layout.DetailPanel.W - 2*pad, H: layout.DetailPanel.H - 2*pad,
	}
	if selected < 0 || selected >= len(model.Items) {
		if model.Notice != "" {
			c.Place(k.Wrapped(model.Notice, RoleMuted, 3), inner)
		} else {
			c.Place(k.Centered("Select an item", RoleMuted), inner)
		}
		return
	}
	item := model.Items[selected]
	rowH := k.Theme.Metrics.TableRowHeight
	c.Place(k.Wrapped(vendingItemName(item), RoleTitle, 2),
		mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: rowH})
	c.Place(k.Text(zenyLabel(item.Price)+"   "+vendingSubtitle(item), RoleMuted),
		mobileui.Rect{X: inner.X, Y: inner.Y + rowH, W: inner.W, H: rowH})
	if layout.BuyButton.W > 0 {
		c.Place(k.Button("Buy", buttonStateFor(item.CanBuy)), layout.BuyButton)
	}
}

func (k Kit) placeVendingQuantity(c *Canvas, layout mobileui.VendingLayout, state mobileui.VendingQuantityState) {
	if !state.Open || layout.QuantityModal.W <= 0 {
		return
	}
	c.Place(k.Scrim(), layout.Safe)
	c.Place(k.Window("Buy", nil), layout.QuantityModal)
	for _, control := range []struct {
		rect  mobileui.Rect
		label string
	}{
		{layout.QuantityMinus, "−"},
		{layout.QuantityPlus, "+"},
		{layout.QuantityConfirm, "Confirm"},
		{layout.QuantityCancel, "Cancel"},
	} {
		if control.rect.W > 0 {
			c.Place(k.Button(control.label, ButtonNormal), control.rect)
		}
	}
	body := modalBody(k, layout.QuantityModal, layout.QuantityMinus)
	if body.H <= 0 {
		return
	}
	label := mobileui.Rect{X: body.X, Y: body.Y, W: body.W, H: body.H * 0.4}
	value := mobileui.Rect{X: body.X, Y: body.Y + body.H*0.4, W: body.W, H: body.H * 0.6}
	c.Place(k.Centered("Max "+strconv.Itoa(state.Maximum), RoleMuted), label)
	c.Place(k.Centered(strconv.Itoa(state.Value), RoleTitle), value)
}

func vendingRowIndex(layout mobileui.VendingLayout, row int) int {
	if row < len(layout.RowIndices) {
		return layout.RowIndices[row]
	}
	return row
}

// VendingIconRects reports where the goods' sprites belong.
func VendingIconRects(model mobileui.MobileVendingModel, layout mobileui.VendingLayout) []IconPlacement {
	out := make([]IconPlacement, 0, len(layout.Rows))
	for i, row := range layout.Rows {
		index := vendingRowIndex(layout, i)
		if index < 0 || index >= len(model.Items) || row.W <= 0 {
			continue
		}
		item := model.Items[index]
		out = append(out, IconPlacement{
			Item: mobileui.InventoryItemModel{
				ItemID: item.ItemID, Identified: item.Identified,
				DisplayName: item.Name, Quantity: item.Quantity, IconKey: item.IconKey,
			},
			Rect: mobileui.Rect{X: row.X, Y: row.Y, W: row.H, H: row.H},
		})
	}
	return out
}
