package mobileui

type EconomyLayout struct {
	Safe, Panel, Header, Close, ListViewport                                    Rect
	CartPanel, CartSubtotal, CartConfirm                                         Rect
	QuantityModal, QuantityMinus, QuantityPlus, QuantityMax, QuantityConfirm, QuantityCancel Rect
	Tabs                                                                        []EconomyTabRect
	Rows                                                                        []Rect
	RowIndices                                                                  []int
	CartRows                                                                    []Rect
	Portrait                                                                    bool
}

const (
	// economyRowHeight fits two lines: an item's name over its stock or the
	// quantity already held. A 56-pixel row was sized for the old bitmap text
	// and collided the two lines at the real font size.
	economyRowHeight float32 = 84
	// economyRowExtent is the row height plus the gap between rows.
	economyRowExtent float32 = economyRowHeight + 8
)

type EconomyTabRect struct {
	Tab  ShopTab
	Rect Rect
}

func LayoutEconomy(viewport Viewport, rowCount int) EconomyLayout {
	return LayoutStorage(viewport, rowCount)
}

func LayoutShop(viewport Viewport, rowCount int, selected ShopTab) EconomyLayout {
	return LayoutShopScrolled(viewport, rowCount, selected, 0)
}

func LayoutShopScrolled(viewport Viewport, rowCount int, selected ShopTab, offset float32) EconomyLayout {
	return LayoutShopCartScrolled(viewport, rowCount, selected, offset, false, 0)
}

func LayoutShopCartScrolled(viewport Viewport, rowCount int, selected ShopTab, offset float32, cartEnabled bool, cartCount int) EconomyLayout {
	safe := viewport.SafeRect()
	layout := EconomyLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	layout.Portrait = viewport.IsPortrait()
	panelW := maxf(0, safe.W-2*16)
	if viewport.Profile() == LayoutFoldLandscape {
		panelW = minf(1200, panelW)
	}
	panelH := maxf(280, safe.H-32)
	if layout.Portrait {
		panelW = maxf(0, safe.W-2*16)
		panelH = maxf(0, safe.H-16)
	}
	if panelH > safe.H {
		panelH = safe.H
	}
	layout.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: safe.Y + (safe.H-panelH)/2, W: panelW, H: panelH}
	pad := float32(20)
	if layout.Portrait {
		pad = 12
	}
	layout.Header = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + 16, W: layout.Panel.W - 2*pad, H: 56}
	layout.Close = Rect{X: layout.Header.Right() - 112, Y: layout.Header.Y, W: 112, H: 52}

	contentX := layout.Panel.X + pad
	contentW := maxf(0, layout.Panel.W-2*pad)
	tabY := layout.Header.Bottom() + 8
	listBottomPad := float32(20)
	if layout.Portrait {
		listBottomPad = 16
	}

	listW := contentW
	if cartEnabled && !layout.Portrait {
		cartGap := float32(12)
		cartW := minf(420, maxf(280, contentW*0.34))
		if contentW-cartW-cartGap >= 280 {
			listW = contentW - cartW - cartGap
			layout.CartPanel = Rect{
				X: contentX + listW + cartGap,
				Y: tabY,
				W: cartW,
				H: maxf(0, layout.Panel.Bottom()-listBottomPad-tabY),
			}
		}
	}

	tabW := (listW - 8) / 2
	layout.Tabs = []EconomyTabRect{
		{Tab: ShopBuyTab, Rect: Rect{X: contentX, Y: tabY, W: tabW, H: 52}},
		{Tab: ShopSellTab, Rect: Rect{X: contentX + tabW + 8, Y: tabY, W: tabW, H: 52}},
	}
	listY := tabY + 64
	listBottom := layout.Panel.Bottom() - listBottomPad

	if cartEnabled && layout.Portrait {
		// Keep the phone shop visually stable: the transaction preview owns the
		// bottom quarter of the shop panel, while Buy/Sell owns the upper
		// three-quarters. This avoids the cart jumping in size with item count.
		cartGap := float32(12)
		cartH := layout.Panel.H * 0.25
		minCartH := economyRowHeight + 116 // 58px heading + one full row + 58px footer
		if cartH < minCartH {
			cartH = minCartH
		}
		maxCartH := maxf(0, listBottom-listY-cartGap-180)
		if cartH > maxCartH {
			cartH = maxCartH
		}
		if cartH > 0 {
			layout.CartPanel = Rect{
				X: contentX,
				Y: listBottom - cartH,
				W: contentW,
				H: cartH,
			}
			listBottom = layout.CartPanel.Y - cartGap
		}
	}

	layout.ListViewport = Rect{X: contentX, Y: listY, W: listW, H: maxf(0, listBottom-listY)}
	layout.Rows, layout.RowIndices = economyVisibleRows(layout.ListViewport, rowCount, offset)

	if layout.CartPanel.W > 0 && layout.CartPanel.H > 0 {
		cartPad := float32(10)
		titleH := float32(58)
		footerH := float32(58)
		bodyY := layout.CartPanel.Y + titleH
		bodyBottom := layout.CartPanel.Bottom() - footerH
		rowGap := float32(8)
		rowH := economyRowHeight
		available := maxf(0, bodyBottom-bodyY)
		maxRows := int((available + rowGap) / (rowH + rowGap))
		if maxRows < 0 {
			maxRows = 0
		}
		visible := minInt(cartCount, maxRows)
		for i := 0; i < visible; i++ {
			layout.CartRows = append(layout.CartRows, Rect{
				X: layout.CartPanel.X + cartPad,
				Y: bodyY + float32(i)*(rowH+rowGap),
				W: layout.CartPanel.W - 2*cartPad,
				H: rowH,
			})
		}
		footerY := layout.CartPanel.Bottom() - footerH + 3
		confirmW := minf(150, maxf(112, layout.CartPanel.W*0.38))
		layout.CartConfirm = Rect{
			X: layout.CartPanel.Right() - cartPad - confirmW,
			Y: footerY,
			W: confirmW,
			H: 48,
		}
		layout.CartSubtotal = Rect{
			X: layout.CartPanel.X + cartPad,
			Y: footerY,
			W: maxf(0, layout.CartConfirm.X-layout.CartPanel.X-2*cartPad),
			H: 48,
		}
	}
	return layout
}

func LayoutStorage(viewport Viewport, rowCount int) EconomyLayout {
	return LayoutStorageScrolled(viewport, rowCount, 0)
}

func LayoutStorageScrolled(viewport Viewport, rowCount int, offset float32) EconomyLayout {
	safe := viewport.SafeRect()
	layout := EconomyLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	layout.Portrait = viewport.IsPortrait()
	panelW := maxf(520, safe.W-2*16)
	if viewport.Profile() == LayoutFoldLandscape {
		panelW = minf(1200, panelW)
	}
	panelH := maxf(220, safe.H-32)
	if layout.Portrait {
		panelW = maxf(0, safe.W-2*16)
		panelH = maxf(0, safe.H-16)
	}
	if panelH > safe.H {
		panelH = safe.H
	}
	layout.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: safe.Y + (safe.H-panelH)/2, W: panelW, H: panelH}
	pad := float32(20)
	if layout.Portrait {
		pad = 12
	}
	layout.Header = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + 16, W: layout.Panel.W - 2*pad, H: 56}
	layout.Close = Rect{X: layout.Header.Right() - 112, Y: layout.Header.Y, W: 112, H: 52}
	listBottomPad := float32(20)
	if layout.Portrait {
		listBottomPad = 16
	}
	layout.ListViewport = Rect{X: layout.Panel.X + pad, Y: layout.Header.Bottom() + 12, W: layout.Panel.W - 2*pad, H: maxf(0, layout.Panel.Bottom()-listBottomPad-(layout.Header.Bottom()+12))}
	layout.Rows, layout.RowIndices = economyVisibleRows(layout.ListViewport, rowCount, offset)
	return layout
}

func LayoutEconomyQuantity(viewport Viewport) (modal, minus, plus, maximum, confirm, cancel Rect) {
	safe := viewport.SafeRect()
	if safe.W <= 0 || safe.H <= 0 {
		return
	}
	modalW := minf(720, maxf(420, safe.W*0.42))
	if viewport.IsPortrait() {
		modalW = maxf(0, safe.W-32)
	}
	modalH := minf(300, maxf(240, safe.H*0.40))
	modal = Rect{X: safe.X + (safe.W-modalW)/2, Y: safe.Y + (safe.H-modalH)/2, W: modalW, H: modalH}
	buttonY := modal.Bottom() - 68
	gap := float32(8)
	sidePad := float32(12)
	buttonW := (modal.W - 2*sidePad - 3*gap) / 4
	minus = Rect{X: modal.X + sidePad, Y: buttonY, W: buttonW, H: 56}
	plus = Rect{X: minus.Right() + gap, Y: buttonY, W: buttonW, H: 56}
	maximum = Rect{X: plus.Right() + gap, Y: buttonY, W: buttonW, H: 56}
	confirm = Rect{X: maximum.Right() + gap, Y: buttonY, W: buttonW, H: 56}
	cancel = Rect{X: modal.Right() - 112 - 12, Y: modal.Y + 12, W: 112, H: 52}
	return
}

func EconomyScrollExtent(viewport Rect, rowCount int) InventoryScrollState {
	rowExtent := economyRowExtent
	content := float32(rowCount) * rowExtent
	if content > 0 {
		content -= 8
	}
	return InventoryScrollState{ViewportExtent: viewport.H, ContentExtent: content, RowExtent: rowExtent}
}

func economyVisibleRows(viewport Rect, rowCount int, offset float32) ([]Rect, []int) {
	if rowCount <= 0 || viewport.W <= 0 || viewport.H <= 0 {
		return nil, nil
	}
	maxOffset := EconomyScrollExtent(viewport, rowCount).MaxOffset()
	offset = clampf(offset, 0, maxOffset)
	rowExtent := economyRowExtent
	first := int(offset / rowExtent)
	visible := int(viewport.H/rowExtent) + 2
	last := minInt(rowCount, first+visible)
	rows := make([]Rect, 0, last-first)
	indices := make([]int, 0, last-first)
	for i := first; i < last; i++ {
		row := Rect{X: viewport.X, Y: viewport.Y + float32(i)*rowExtent - offset, W: viewport.W, H: economyRowHeight}
		// Only fully visible rows are emitted. A row straddling the edge is
		// drawn unclipped by the renderer, so it would spill past the panel.
		if row.Y < viewport.Y || row.Bottom() > viewport.Bottom() {
			continue
		}
		rows = append(rows, row)
		indices = append(indices, i)
	}
	return rows, indices
}
