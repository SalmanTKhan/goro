package mobileui

type MapTokens struct {
	Edge, Gap, HeaderHeight, ContentMaxWidth, WarpListWidth, MinTouchTarget float32
}

func DefaultMapTokens() MapTokens {
	return MapTokens{Edge: 20, Gap: 16, HeaderHeight: 64, ContentMaxWidth: 1800, WarpListWidth: 520, MinTouchTarget: 48}
}

// mapWarpRowExtent is the pitch of one warp row. Each row stacks the exit's
// name over its destination, so it needs room for two lines of the real UI
// font; the previous 56-64 pixel rows were sized for the old bitmap text and
// overlapped the two.
const mapWarpRowExtent float32 = 92

type MapWarpRect struct {
	ID   uint32
	Rect Rect
}

type MobileMapLayout struct {
	Safe, Header, MapViewport, WarpPanel, WarpListViewport, DetailPanel, BackButton, TravelButton, ConfirmModal, ConfirmButton, ConfirmCancel Rect
	WarpRows                                                                                                                                  []MapWarpRect
	Portrait                                                                                                                                  bool
}

type MapInteractionState struct {
	SelectedWarpID uint32
	ConfirmOpen    bool
	ScrollOffset   float32
}

func LayoutMap(viewport Viewport, tokens MapTokens, model MobileMapModel, state MapInteractionState) MobileMapLayout {
	if tokens.Edge <= 0 {
		tokens = DefaultMapTokens()
	}
	safe := viewport.SafeRect()
	layout := MobileMapLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	contentW := maxf(0, safe.W-2*tokens.Edge)
	if viewport.Profile() == LayoutFoldLandscape && tokens.ContentMaxWidth > 0 {
		contentW = minf(contentW, tokens.ContentMaxWidth)
	}
	contentX := safe.X + (safe.W-contentW)/2
	layout.Portrait = viewport.IsPortrait()
	layout.Header = Rect{contentX, safe.Y + tokens.Edge, contentW, tokens.HeaderHeight}
	layout.BackButton = Rect{layout.Header.X, layout.Header.Y, BackButtonWidth(), layout.Header.H}
	contentY := layout.Header.Bottom() + tokens.Gap
	contentH := maxf(0, safe.Bottom()-tokens.Edge-contentY)
	if layout.Portrait {
		mapH := minf(520, maxf(320, contentH*0.48))
		layout.MapViewport = Rect{contentX, contentY, contentW, mapH}
		listY := layout.MapViewport.Bottom() + tokens.Gap
		listBottom := safe.Bottom() - tokens.Edge
		if _, ok := model.Warp(state.SelectedWarpID); ok {
			detailH := minf(220, maxf(180, contentH*0.26))
			layout.DetailPanel = Rect{contentX, listBottom - detailH, contentW, detailH}
			listBottom = layout.DetailPanel.Y - tokens.Gap
		}
		layout.WarpPanel = Rect{contentX, listY, contentW, maxf(0, listBottom-listY)}
		rowArea := Rect{contentX + tokens.Gap, listY + 44, contentW - 2*tokens.Gap, maxf(0, layout.WarpPanel.Bottom()-listY-52)}
		layout.WarpListViewport = rowArea
		rowH := maxf(tokens.MinTouchTarget, mapWarpRowExtent)
		first := int(state.ScrollOffset / rowH)
		visible := maxInt(1, int(rowArea.H/rowH))
		last := minInt(len(model.Warps), first+visible)
		for i := first; i < last; i++ {
			layout.WarpRows = append(layout.WarpRows, MapWarpRect{ID: model.Warps[i].ID, Rect: Rect{rowArea.X, rowArea.Y + float32(i-first)*rowH, rowArea.W, rowH - tokens.Gap}})
		}
		if _, ok := model.Warp(state.SelectedWarpID); ok {
			layout.TravelButton = Rect{layout.DetailPanel.X + tokens.Gap, layout.DetailPanel.Bottom() - 60, layout.DetailPanel.W - 2*tokens.Gap, 52}
		}
		if state.ConfirmOpen {
			modalW := maxf(0, safe.W-2*tokens.Edge)
			modalH := minf(300, safe.H-2*tokens.Edge)
			layout.ConfirmModal = Rect{safe.X + tokens.Edge, safe.Y + (safe.H-modalH)/2, modalW, modalH}
			layout.ConfirmButton = Rect{layout.ConfirmModal.X + tokens.Gap, layout.ConfirmModal.Bottom() - 68, (layout.ConfirmModal.W - 3*tokens.Gap) / 2, 52}
			layout.ConfirmCancel = Rect{layout.ConfirmButton.Right() + tokens.Gap, layout.ConfirmButton.Y, layout.ConfirmButton.W, 52}
		}
		return layout
	}
	listW := minf(tokens.WarpListWidth, maxf(320, contentW*0.30))
	mapW := maxf(320, contentW-listW-tokens.Gap)
	layout.MapViewport = Rect{contentX, contentY, mapW, contentH}
	layout.WarpPanel = Rect{layout.MapViewport.Right() + tokens.Gap, contentY, listW, contentH}

	detailH := maxf(160, minf(220, contentH*0.34))
	layout.DetailPanel = Rect{layout.WarpPanel.X + tokens.Gap, layout.WarpPanel.Bottom() - detailH - tokens.Gap, layout.WarpPanel.W - 2*tokens.Gap, detailH}
	rowArea := Rect{layout.WarpPanel.X + tokens.Gap, layout.WarpPanel.Y + 44, layout.WarpPanel.W - 2*tokens.Gap, maxf(0, layout.DetailPanel.Y-layout.WarpPanel.Y-52)}
	layout.WarpListViewport = rowArea
	rowH := maxf(tokens.MinTouchTarget, mapWarpRowExtent)
	first := int(state.ScrollOffset / rowH)
	visible := maxInt(1, int(rowArea.H/rowH)+1)
	last := minInt(len(model.Warps), first+visible)
	for i := first; i < last; i++ {
		layout.WarpRows = append(layout.WarpRows, MapWarpRect{ID: model.Warps[i].ID, Rect: Rect{rowArea.X, rowArea.Y + float32(i-first)*rowH, rowArea.W, rowH - tokens.Gap}})
	}
	if _, ok := model.Warp(state.SelectedWarpID); ok {
		layout.TravelButton = Rect{layout.DetailPanel.X + tokens.Gap, layout.DetailPanel.Bottom() - 60, layout.DetailPanel.W - 2*tokens.Gap, 52}
	}
	if state.ConfirmOpen {
		modalW := minf(720, safe.W-48)
		modalH := minf(300, safe.H-48)
		layout.ConfirmModal = Rect{safe.X + (safe.W-modalW)/2, safe.Y + (safe.H-modalH)/2, modalW, modalH}
		layout.ConfirmButton = Rect{layout.ConfirmModal.X + tokens.Gap, layout.ConfirmModal.Bottom() - 68, (layout.ConfirmModal.W - 3*tokens.Gap) / 2, 52}
		layout.ConfirmCancel = Rect{layout.ConfirmButton.Right() + tokens.Gap, layout.ConfirmButton.Y, layout.ConfirmButton.W, 52}
	}
	return layout
}

func MapScrollExtent(model MobileMapModel, layout MobileMapLayout, tokens MapTokens) InventoryScrollState {
	if tokens.Edge <= 0 {
		tokens = DefaultMapTokens()
	}
	rowAreaH := layout.WarpListViewport.H
	rowH := maxf(tokens.MinTouchTarget, mapWarpRowExtent)
	return InventoryScrollState{ViewportExtent: rowAreaH, ContentExtent: float32(len(model.Warps)) * rowH, Offset: 0, RowExtent: rowH}
}
