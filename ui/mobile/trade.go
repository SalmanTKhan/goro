package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// TradeTree builds the player-to-player exchange: your inventory, your offer
// and your partner's, with the conclude/commit controls and the request and
// quantity modals.
func (k Kit) TradeTree(
	model mobileui.MobileTradeModel,
	layout mobileui.TradeLayout,
	state mobileui.TradeInteractionState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	k.placeTradeHeader(c, model, layout)
	k.placeTradeInventory(c, model, layout)
	k.placeTradeOffer(c, layout.OwnPanel, layout.OwnRows, model.OwnOffer,
		offerTitle("Your offer", model.OwnZeny, model.SelfConcluded))
	k.placeTradeOffer(c, layout.PartnerPanel, layout.PartnerRows, model.PartnerOffer,
		offerTitle(partnerTitle(model), model.PartnerZeny, model.OtherConcluded))
	k.placeTradeActions(c, model, layout)

	if model.Notice != "" && layout.Notice.W > 0 {
		c.Place(k.Wrapped(model.Notice, RoleMuted, 2), layout.Notice)
	}
	k.placeTradeModals(c, model, layout, state)
	return c
}

func (k Kit) placeTradeHeader(c *Canvas, model mobileui.MobileTradeModel, layout mobileui.TradeLayout) {
	if layout.BackButton.W > 0 {
		c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)
	}
	if layout.Header.W <= 0 {
		return
	}
	pad := k.Theme.Metrics.TableCellPadX
	left := layout.Header.X + pad
	if layout.BackButton.W > 0 {
		left = layout.BackButton.Right() + pad
	}
	title := mobileui.Rect{X: left, Y: layout.Header.Y, W: layout.Header.Right() - pad - left, H: layout.Header.H}
	if title.W > 0 {
		c.Place(k.Centered(partnerTitle(model), RoleTitle), title)
	}
}

func partnerTitle(model mobileui.MobileTradeModel) string {
	if model.PartnerName == "" {
		return "Trade"
	}
	return model.PartnerName
}

// offerTitle names an offer and reports its zeny and conclusion state, which is
// what a player checks before committing.
func offerTitle(name string, zeny uint32, concluded bool) string {
	title := name
	if zeny > 0 {
		title += "  " + zenyLabel(int64(zeny))
	}
	if concluded {
		title += "  (ready)"
	}
	return title
}

func (k Kit) placeTradeInventory(c *Canvas, model mobileui.MobileTradeModel, layout mobileui.TradeLayout) {
	if layout.InventoryPanel.W <= 0 || layout.InventoryPanel.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.InventoryPanel)
	pad := k.Theme.Metrics.TableCellPadX
	rowH := k.Theme.Metrics.TableRowHeight
	c.Place(k.Text("Your items", RoleTitle),
		mobileui.Rect{X: layout.InventoryPanel.X + pad, Y: layout.InventoryPanel.Y + pad, W: layout.InventoryPanel.W - 2*pad, H: rowH})

	for _, row := range layout.InventoryRows {
		// The layout over-produces rows so a scroll has something to reveal;
		// only those wholly inside the panel may be drawn, since nothing clips.
		if row.Index >= len(model.Inventory) || !rowInsidePanel(row.Rect, layout.InventoryPanel) {
			continue
		}
		entry := model.Inventory[row.Index]
		c.Place(k.Card(entry.Offered), row.Rect)
		titleRect, subtitleRect := k.RowLines(row.Rect, row.Rect.H)
		if titleRect.W <= 0 {
			continue
		}
		c.Place(k.Content(entry.Item.DisplayName, RoleValue), titleRect)
		c.Place(k.Text(tradeInventorySubtitle(entry), RoleMuted), subtitleRect)
	}
}

// tradeInventorySubtitle says how many are held and whether the row is already
// committed to the exchange.
func tradeInventorySubtitle(entry mobileui.TradeInventoryItemModel) string {
	subtitle := "x" + strconv.Itoa(entry.Item.Quantity)
	switch {
	case entry.Pending:
		subtitle += "  pending"
	case entry.Offered:
		subtitle += "  offered"
	case !entry.CanAdd:
		subtitle += "  locked"
	}
	return subtitle
}

func (k Kit) placeTradeOffer(
	c *Canvas,
	panel mobileui.Rect,
	rows []mobileui.TradeRowRect,
	items []mobileui.TradeOfferItemModel,
	title string,
) {
	if panel.W <= 0 || panel.H <= 0 {
		return
	}
	c.Place(k.Panel(), panel)
	pad := k.Theme.Metrics.TableCellPadX
	rowH := k.Theme.Metrics.TableRowHeight
	c.Place(k.Text(title, RoleTitle),
		mobileui.Rect{X: panel.X + pad, Y: panel.Y + pad, W: panel.W - 2*pad, H: rowH})

	for _, row := range rows {
		if row.Index >= len(items) || !rowInsidePanel(row.Rect, panel) {
			continue
		}
		item := items[row.Index]
		c.Place(k.Card(false), row.Rect)
		// Zeny has no sprite; every other offer entry reserves an icon square.
		leading := row.Rect.X + pad
		if !item.IsZeny {
			leading = row.Rect.X + row.Rect.H + pad
		}
		inner := mobileui.Rect{X: leading, Y: row.Rect.Y, W: row.Rect.Right() - pad - leading, H: row.Rect.H}
		if inner.W <= 0 {
			continue
		}
		countW := inner.W * 0.24
		c.Place(k.Content(offerItemName(item), RoleBody),
			mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W - countW, H: inner.H})
		if item.Quantity > 1 {
			c.Place(k.RightAligned("x"+strconv.Itoa(item.Quantity), RoleMuted),
				mobileui.Rect{X: inner.X + inner.W - countW, Y: inner.Y, W: countW, H: inner.H})
		}
	}
}

func offerItemName(item mobileui.TradeOfferItemModel) string {
	if item.IsZeny {
		return "Zeny"
	}
	if item.Refine > 0 {
		return "+" + strconv.Itoa(int(item.Refine)) + " " + item.Name
	}
	return item.Name
}

func (k Kit) placeTradeActions(c *Canvas, model mobileui.MobileTradeModel, layout mobileui.TradeLayout) {
	for _, action := range []struct {
		rect    mobileui.Rect
		label   string
		enabled bool
	}{
		{layout.AddZeny, "Add zeny", model.CanAddZeny},
		{layout.Conclude, "Conclude", model.CanConclude},
		{layout.Commit, "Commit", model.CanCommit},
		{layout.Cancel, "Cancel", true},
	} {
		if action.rect.W > 0 && action.rect.H > 0 {
			c.Place(k.Button(action.label, buttonStateFor(action.enabled)), action.rect)
		}
	}
}

func (k Kit) placeTradeModals(
	c *Canvas,
	model mobileui.MobileTradeModel,
	layout mobileui.TradeLayout,
	state mobileui.TradeInteractionState,
) {
	if model.PendingRequest != nil && layout.RequestModal.W > 0 {
		k.placeConfirmModal(c, layout.Safe, layout.RequestModal, "Trade request",
			model.PendingRequest.Name+" wants to trade with you.",
			layout.RequestAccept, "Accept", layout.RequestDecline, "Decline")
		return
	}
	if !state.Quantity.Open || layout.QuantityModal.W <= 0 {
		return
	}
	title := "Add item"
	if state.Quantity.Action == mobileui.TradeQuantityZeny {
		title = "Add zeny"
	}
	c.Place(k.Scrim(), layout.Safe)
	c.Place(k.Window(title, nil), layout.QuantityModal)
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
	c.Place(k.Centered("Max "+strconv.Itoa(state.Quantity.Maximum), RoleMuted), label)
	c.Place(k.Centered(strconv.Itoa(state.Quantity.Value), RoleTitle), value)
}

// TradeIconRects reports where sprites belong across all three trade lists: the
// player's inventory and both sides of the offer.
func TradeIconRects(model mobileui.MobileTradeModel, layout mobileui.TradeLayout) []IconPlacement {
	out := make([]IconPlacement, 0, len(layout.InventoryRows)+len(layout.OwnRows)+len(layout.PartnerRows))
	for _, row := range layout.InventoryRows {
		if row.Index >= len(model.Inventory) || !rowInsidePanel(row.Rect, layout.InventoryPanel) {
			continue
		}
		out = append(out, IconPlacement{
			Item: model.Inventory[row.Index].Item,
			Rect: mobileui.Rect{X: row.Rect.X, Y: row.Rect.Y, W: row.Rect.H, H: row.Rect.H},
		})
	}
	for _, pair := range []struct {
		rows  []mobileui.TradeRowRect
		items []mobileui.TradeOfferItemModel
		panel mobileui.Rect
	}{
		{layout.OwnRows, model.OwnOffer, layout.OwnPanel},
		{layout.PartnerRows, model.PartnerOffer, layout.PartnerPanel},
	} {
		for _, row := range pair.rows {
			if row.Index >= len(pair.items) || !rowInsidePanel(row.Rect, pair.panel) {
				continue
			}
			item := pair.items[row.Index]
			if item.IsZeny {
				continue
			}
			out = append(out, IconPlacement{
				Item: mobileui.InventoryItemModel{
					ItemID: item.ItemID, Identified: item.Identified, DisplayName: item.Name,
					Quantity: item.Quantity, Refine: item.Refine, IconKey: item.IconKey,
				},
				Rect: mobileui.Rect{X: row.Rect.X, Y: row.Rect.Y, W: row.Rect.H, H: row.Rect.H},
			})
		}
	}
	return out
}

// rowInsidePanel reports whether a row is wholly within its panel. Nothing in
// this presentation clips, so a row that merely overlaps would paint over the
// panel below it.
func rowInsidePanel(row, panel mobileui.Rect) bool {
	if row.W <= 0 || row.H <= 0 || panel.H <= 0 {
		return false
	}
	return row.Y >= panel.Y-0.5 && row.Bottom() <= panel.Bottom()+0.5
}
