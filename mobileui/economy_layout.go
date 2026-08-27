package mobileui

type EconomyLayout struct {
	Safe, Panel, Header, Close, ListViewport                                    Rect
	QuantityModal, QuantityMinus, QuantityPlus, QuantityConfirm, QuantityCancel Rect
	Tabs                                                                        []EconomyTabRect
	Rows                                                                        []Rect
	RowIndices                                                                  []int
	Portrait                                                                    bool
}

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
	tabW := (layout.Panel.W - 56) / 2
	tabY := layout.Header.Bottom() + 8
	if layout.Portrait {
		tabW = (layout.Panel.W - 2*pad - 8) / 2
	}
	layout.Tabs = []EconomyTabRect{
		{Tab: ShopBuyTab, Rect: Rect{X: layout.Panel.X + pad, Y: tabY, W: tabW, H: 52}},
		{Tab: ShopSellTab, Rect: Rect{X: layout.Panel.X + pad + tabW + 8, Y: tabY, W: tabW, H: 52}},
	}
	// Keep the same content inset as the panel header on wide landscape
	// displays so the last row does not sit against the panel edge.
	listBottomPad := float32(20)
	if layout.Portrait {
		listBottomPad = 16
	}
	layout.ListViewport = Rect{X: layout.Panel.X + pad, Y: tabY + 64, W: layout.Panel.W - 2*pad, H: maxf(0, layout.Panel.Bottom()-listBottomPad-(tabY+64))}
	layout.Rows, layout.RowIndices = economyVisibleRows(layout.ListViewport, rowCount, offset)
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

func LayoutEconomyQuantity(viewport Viewport) (modal, minus, plus, confirm, cancel Rect) {
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
	buttonW := (modal.W - 4*12) / 3
	minus = Rect{X: modal.X + 12, Y: buttonY, W: buttonW, H: 56}
	plus = Rect{X: minus.Right() + 12, Y: buttonY, W: buttonW, H: 56}
	confirm = Rect{X: plus.Right() + 12, Y: buttonY, W: buttonW, H: 56}
	cancel = Rect{X: modal.Right() - 112 - 12, Y: modal.Y + 12, W: 112, H: 52}
	return
}

func EconomyScrollExtent(viewport Rect, rowCount int) InventoryScrollState {
	rowExtent := float32(64)
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
	rowExtent := float32(64)
	first := int(offset / rowExtent)
	visible := int(viewport.H/rowExtent) + 2
	last := minInt(rowCount, first+visible)
	rows := make([]Rect, 0, last-first)
	indices := make([]int, 0, last-first)
	for i := first; i < last; i++ {
		row := Rect{X: viewport.X, Y: viewport.Y + float32(i)*rowExtent - offset, W: viewport.W, H: 56}
		if row.Bottom() <= viewport.Y || row.Y >= viewport.Bottom() {
			continue
		}
		rows = append(rows, row)
		indices = append(indices, i)
	}
	return rows, indices
}
