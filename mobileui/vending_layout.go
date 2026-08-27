package mobileui

type VendingLayout struct {
	Safe, Panel, Header, Back, Close, ListViewport, DetailPanel, BuyButton      Rect
	Rows                                                                        []Rect
	RowIndices                                                                  []int
	QuantityModal, QuantityMinus, QuantityPlus, QuantityConfirm, QuantityCancel Rect
}

func LayoutVending(viewport Viewport, rowCount int) VendingLayout {
	return LayoutVendingScrolled(viewport, rowCount, 0)
}

func LayoutVendingScrolled(viewport Viewport, rowCount int, offset float32) VendingLayout {
	safe := viewport.SafeRect()
	l := VendingLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return l
	}
	portrait := viewport.IsPortrait()
	panelW := minf(1280, maxf(520, safe.W-32))
	panelH := maxf(280, safe.H-32)
	if portrait {
		panelW = maxf(0, safe.W-24)
		panelH = maxf(0, safe.H-16)
	}
	if panelH > safe.H {
		panelH = safe.H
	}
	l.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: safe.Y + (safe.H-panelH)/2, W: panelW, H: panelH}
	pad := float32(20)
	if portrait {
		pad = 12
	}
	l.Header = Rect{X: l.Panel.X + pad, Y: l.Panel.Y + 16, W: l.Panel.W - 2*pad, H: 56}
	l.Back = Rect{X: l.Header.X, Y: l.Header.Y, W: 112, H: 52}
	l.Close = Rect{X: l.Header.Right() - 112, Y: l.Header.Y, W: 112, H: 52}
	contentY := l.Header.Bottom() + 12
	if portrait {
		listH := maxf(132, l.Panel.H*0.40)
		l.ListViewport = Rect{X: l.Panel.X + pad, Y: contentY, W: l.Panel.W - 2*pad, H: listH}
		detailY := l.ListViewport.Bottom() + 12
		l.DetailPanel = Rect{X: l.Panel.X + pad, Y: detailY, W: l.Panel.W - 2*pad, H: maxf(0, l.Panel.Bottom()-detailY-16)}
	} else {
		listW := minf(620, maxf(400, l.Panel.W*0.48))
		l.ListViewport = Rect{X: l.Panel.X + pad, Y: contentY, W: listW, H: maxf(0, l.Panel.Bottom()-contentY-20)}
		detailX := l.ListViewport.Right() + 16
		l.DetailPanel = Rect{X: detailX, Y: contentY, W: maxf(0, l.Panel.Right()-pad-detailX), H: l.ListViewport.H}
	}
	rowExtent := float32(68)
	scroll := InventoryScrollState{ViewportExtent: l.ListViewport.H, ContentExtent: vendingContentExtent(rowCount), RowExtent: rowExtent}
	scroll.SetOffset(offset)
	clamped := scroll.Offset
	first := int(clamped / rowExtent)
	visible := int(l.ListViewport.H/rowExtent) + 2
	last := minInt(rowCount, first+visible)
	for i := first; i < last; i++ {
		row := Rect{X: l.ListViewport.X, Y: l.ListViewport.Y + float32(i)*rowExtent - clamped, W: l.ListViewport.W, H: 60}
		if row.Bottom() <= l.ListViewport.Y || row.Y >= l.ListViewport.Bottom() {
			continue
		}
		l.Rows = append(l.Rows, row)
		l.RowIndices = append(l.RowIndices, i)
	}
	if l.DetailPanel.W > 0 && l.DetailPanel.H > 0 {
		l.BuyButton = Rect{X: l.DetailPanel.X + 16, Y: l.DetailPanel.Bottom() - 72, W: l.DetailPanel.W - 32, H: 56}
	}
	return l
}

func vendingContentExtent(rowCount int) float32 {
	if rowCount <= 0 {
		return 0
	}
	return maxf(0, float32(rowCount)*68-8)
}

func LayoutVendingQuantity(viewport Viewport) (modal, minus, plus, confirm, cancel Rect) {
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
