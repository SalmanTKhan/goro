package mobileui

type InventoryTokens struct {
	Edge, Gap, HeaderHeight, TabHeight, DetailWidth, ContentMaxWidth, CellSize, MinTouchTarget float32
}

func DefaultInventoryTokens() InventoryTokens {
	return InventoryTokens{Edge: 16, Gap: 12, HeaderHeight: 76, TabHeight: 56, DetailWidth: 0.34, ContentMaxWidth: 1600, CellSize: 168, MinTouchTarget: 56}
}

type InventoryTabRect struct {
	Category InventoryCategory
	Rect     Rect
}
type InventoryCellRect struct {
	Index uint16
	Rect  Rect
}
type EquipmentSlotRect struct {
	Index    int
	Location uint16
	Label    string
	Rect     Rect
}

type MobileInventoryLayout struct {
	Safe, Header, TabsArea, CategoryButton, GridViewport, EquipmentViewport, PaperDoll, PaperDollCluster, DetailPanel, DetailIcon, DetailTitle, DetailMeta, DetailDescription, PrimaryAction, SecondaryAction, EquipmentButton, StorageButton, BackButton, QuantityModal Rect
	QuantityMinus, QuantityPlus, QuantityConfirm, QuantityCancel                                                                                                                                                                                                         Rect
	Tabs, CategoryOptions                                                                                                                                                                                                                                                []InventoryTabRect
	Cells                                                                                                                                                                                                                                                                []InventoryCellRect
	PaperDollSlots, EquipmentSlots                                                                                                                                                                                                                                       []EquipmentSlotRect
	Portrait, SingleColumn                                                                                                                                                                                                                                               bool
	GridColumns                                                                                                                                                                                                                                                          int
	GridRowExtent, GridContentExtent, GridCellWidth, GridCellHeight, GridHorizontalGap, GridVerticalGap, GridOuterPadding                                                                                                                                                float32
}

func LayoutInventory(viewport Viewport, tokens InventoryTokens, model MobileInventoryModel, state InventoryInteractionState) MobileInventoryLayout {
	if tokens.Edge <= 0 {
		tokens = DefaultInventoryTokens()
	}
	safe := viewport.SafeRect()
	layout := MobileInventoryLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	contentW := maxf(0, safe.W-2*tokens.Edge)
	if !viewport.IsPortrait() && tokens.ContentMaxWidth > 0 {
		contentW = minf(contentW, tokens.ContentMaxWidth)
	}
	contentX := safe.X + (safe.W-contentW)/2
	layout.Portrait = viewport.IsPortrait()
	layout.Header = Rect{contentX, safe.Y + tokens.Edge, contentW, tokens.HeaderHeight}
	layout.BackButton = Rect{layout.Header.X, layout.Header.Y, 100, layout.Header.H}

	// Browsing uses the same compact category tabs in portrait and landscape.
	// Equipment is header navigation, not a full-width content row.
	tabY := layout.Header.Bottom() + tokens.Gap
	tabGap := minf(tokens.Gap, 10)
	tabW := maxf(tokens.MinTouchTarget, (contentW-4*tabGap)/5)
	layout.TabsArea = Rect{contentX, tabY, contentW, tokens.TabHeight}
	for i, category := range []InventoryCategory{InventoryCategoryAll, InventoryCategoryEquipment, InventoryCategoryUsable, InventoryCategoryEtc, InventoryCategoryCards} {
		layout.Tabs = append(layout.Tabs, InventoryTabRect{Category: category, Rect: Rect{layout.TabsArea.X + float32(i)*(tabW+tabGap), tabY, tabW, tokens.TabHeight}})
	}
	layout.EquipmentButton = Rect{layout.Header.Right() - 112, layout.Header.Y, 112, layout.Header.H}

	contentY := layout.TabsArea.Bottom() + tokens.Gap
	contentH := maxf(0, safe.Bottom()-tokens.Edge-contentY)
	items := state.FilteredItems(model)
	selected := state.Selection.HasSelection && state.DetailOpen
	fold := viewport.Profile() == LayoutFoldLandscape
	detailW := float32(0)
	if fold || (!layout.Portrait && selected) {
		detailW = minf(520, maxf(300, contentW*tokens.DetailWidth))
	}
	gridW := contentW
	if detailW > 0 {
		gridW = maxf(tokens.MinTouchTarget, contentW-tokens.Gap-detailW)
	}
	detailH := float32(0)
	if layout.Portrait && selected {
		detailH = minf(300, maxf(220, safe.H*0.28))
	}
	gridH := contentH
	if detailH > 0 {
		gridH = maxf(0, gridH-detailH-tokens.Gap)
	}
	layout.GridViewport = Rect{contentX, contentY, gridW, gridH}
	if detailH > 0 {
		layout.DetailPanel = Rect{contentX, layout.GridViewport.Bottom() + tokens.Gap, contentW, detailH}
	} else if detailW > 0 {
		panelH := minf(contentH, float32(180))
		if selected {
			panelH = contentH
		}
		layout.DetailPanel = Rect{layout.GridViewport.Right() + tokens.Gap, contentY, detailW, panelH}
	}

	// LayoutGrid is the single source of truth for inventory cell geometry.
	// Keep sparse inventories compact. The body still has two rows of faint
	// empty slots so the surface reads as an inventory, but it must not become a
	// full-height pale rectangle just because the device is tall. When the
	// actual item rows exceed the available space, the viewport naturally
	// expands to the full scrollable height.
	grid := LayoutGrid(layout.GridViewport, len(items), inventoryGridSpec(layout.Portrait, fold))
	minimumRows := maxInt(2, grid.Rows)
	minimumGridH := grid.OuterPadding + float32(minimumRows)*grid.CellHeight + float32(maxInt(0, minimumRows-1))*grid.VerticalGap + tokens.Gap + 58
	if gridH > minimumGridH {
		layout.GridViewport.H = minimumGridH
		grid = LayoutGrid(layout.GridViewport, len(items), inventoryGridSpec(layout.Portrait, fold))
	}
	if detailH > 0 {
		layout.DetailPanel = Rect{contentX, layout.GridViewport.Bottom() + tokens.Gap, contentW, detailH}
	}
	layout.GridColumns = grid.Columns
	layout.SingleColumn = grid.Columns == 1
	layout.GridRowExtent = grid.Scroll.RowExtent
	layout.GridContentExtent = grid.Scroll.ContentExtent
	layout.GridCellWidth = grid.CellWidth
	layout.GridCellHeight = grid.CellHeight
	layout.GridHorizontalGap = grid.HorizontalGap
	layout.GridVerticalGap = grid.VerticalGap
	layout.GridOuterPadding = grid.OuterPadding
	for i, cell := range grid.Cells {
		cell.Y -= state.Scroll.Offset
		if cell.Y < layout.GridViewport.Y || cell.Bottom() > layout.GridViewport.Bottom() {
			continue
		}
		layout.Cells = append(layout.Cells, InventoryCellRect{Index: items[i].Index, Rect: cell})
	}
	if state.Selection.HasSelection && layout.DetailPanel.W > 0 {
		buttonW := maxf(tokens.MinTouchTarget, (layout.DetailPanel.W-3*tokens.Gap)/2)
		actionEdge, actionHeight := detailActionMetrics(layout.DetailPanel, tokens)
		layout.PrimaryAction = Rect{layout.DetailPanel.X + tokens.Gap, layout.DetailPanel.Bottom() - actionEdge - actionHeight, buttonW, actionHeight}
		layout.SecondaryAction = Rect{layout.PrimaryAction.Right() + tokens.Gap, layout.PrimaryAction.Y, layout.PrimaryAction.W, layout.PrimaryAction.H}
		layoutDetailContent(&layout, tokens)
	}
	if state.Quantity.Open {
		modalW := safe.W * 0.40
		modalX := safe.X + safe.W*0.30
		if layout.Portrait {
			modalW = safe.W - 2*tokens.Edge
			modalX = safe.X + tokens.Edge
		}
		layout.QuantityModal = Rect{modalX, safe.Y + safe.H*0.25, modalW, minf(260, safe.H*0.50)}
		buttonY := layout.QuantityModal.Bottom() - 64
		buttonW := (layout.QuantityModal.W - 4*tokens.Gap) / 3
		layout.QuantityMinus = Rect{layout.QuantityModal.X + tokens.Gap, buttonY, buttonW, 52}
		layout.QuantityPlus = Rect{layout.QuantityMinus.Right() + tokens.Gap, buttonY, buttonW, 52}
		layout.QuantityConfirm = Rect{layout.QuantityPlus.Right() + tokens.Gap, buttonY, buttonW, 52}
		layout.QuantityCancel = Rect{layout.QuantityModal.Right() - 112 - tokens.Gap, layout.QuantityModal.Y + tokens.Gap, 112, 48}
	}
	return layout
}

func inventoryGridSpec(portrait, fold bool) GridSpec {
	if portrait {
		return GridSpec{MinCellWidth: 48, MaxCellWidth: 260, MinRowHeight: 0, HorizontalGap: 16, VerticalGap: 16, PreferredColumns: 4, MinColumns: 4, MaxColumns: 4, AspectRatio: 1, FillWidth: true, OuterPadding: 16}
	}
	preferred := 6
	if fold {
		preferred = 5
	}
	return GridSpec{MinCellWidth: 96, MaxCellWidth: 248, MinRowHeight: 0, HorizontalGap: 16, VerticalGap: 16, PreferredColumns: preferred, MinColumns: 4, MaxColumns: 6, AspectRatio: 1, FillWidth: false, OuterPadding: 16}
}

func ScrollExtent(model MobileInventoryModel, state InventoryInteractionState, tokens InventoryTokens, viewport Rect) InventoryScrollState {
	items := state.FilteredItems(model)
	columns := maxInt(1, int((viewport.W+tokens.Gap)/(tokens.CellSize+tokens.Gap)))
	rowExtent := tokens.CellSize + tokens.Gap
	rows := (len(items) + columns - 1) / columns
	content := float32(rows) * rowExtent
	if content > 0 {
		content -= tokens.Gap
	}
	return InventoryScrollState{ViewportExtent: viewport.H, ContentExtent: content, Offset: state.Scroll.Offset, RowExtent: rowExtent}
}

func ScrollExtentForLayout(model MobileInventoryModel, state InventoryInteractionState, tokens InventoryTokens, layout MobileInventoryLayout) InventoryScrollState {
	if layout.GridRowExtent > 0 || layout.GridContentExtent > 0 {
		return InventoryScrollState{ViewportExtent: layout.GridViewport.H, ContentExtent: layout.GridContentExtent, Offset: state.Scroll.Offset, RowExtent: layout.GridRowExtent}
	}
	items := state.FilteredItems(model)
	grid := LayoutGrid(layout.GridViewport, len(items), inventoryGridSpec(layout.Portrait, layout.Safe.W > layout.Safe.H && layout.Safe.H <= 900))
	return InventoryScrollState{ViewportExtent: layout.GridViewport.H, ContentExtent: grid.Scroll.ContentExtent, Offset: state.Scroll.Offset, RowExtent: grid.Scroll.RowExtent}
}

func portraitInventoryColumns(width float32) int {
	switch {
	case width >= 720:
		return 5
	default:
		return 4
	}
}

func LayoutEquipment(viewport Viewport, tokens InventoryTokens, model MobileEquipmentModel, state InventoryInteractionState) MobileInventoryLayout {
	if tokens.Edge <= 0 {
		tokens = DefaultInventoryTokens()
	}
	safe := viewport.SafeRect()
	layout := MobileInventoryLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	contentW := minf(safe.W-2*tokens.Edge, tokens.ContentMaxWidth)
	contentX := safe.X + (safe.W-contentW)/2
	layout.Header = Rect{contentX, safe.Y + tokens.Edge, contentW, tokens.HeaderHeight}
	layout.BackButton = Rect{layout.Header.X, layout.Header.Y, 100, layout.Header.H}
	content := Rect{contentX, layout.Header.Bottom() + tokens.Gap, contentW, safe.Bottom() - layout.Header.Bottom() - tokens.Gap - tokens.Edge}
	layout.Portrait = viewport.IsPortrait()
	// Portrait uses a compact paper-doll cluster followed by a bounded summary
	// region. Landscape keeps the paper doll and summary side by side, but the
	// default summary height is content-sized so a single equipped item does
	// not create a giant empty canvas.
	if layout.Portrait {
		detailH := equipmentSummaryHeight(model, content.H)
		if state.Selection.HasSelection && state.DetailOpen {
			detailH = minf(300, maxf(220, safe.H*0.28))
		}
		minimumPaperH := equipmentPaperDollMinHeight(content.W, tokens)
		paperH := minf(content.H-detailH-tokens.Gap, maxf(minimumPaperH, content.H*0.55))
		if paperH < minimumPaperH {
			paperH = minf(content.H, minimumPaperH)
			detailH = maxf(0, content.H-paperH-tokens.Gap)
		}
		layout.PaperDoll = Rect{content.X, content.Y, content.W, paperH}
		layout.EquipmentViewport = layout.PaperDoll
		layout.PaperDollSlots, layout.PaperDollCluster = paperDollSlotRects(layout.PaperDoll, model.Slots, tokens)
		layout.EquipmentSlots = append([]EquipmentSlotRect(nil), layout.PaperDollSlots...)
		if detailH > 0 {
			layout.DetailPanel = Rect{content.X, layout.PaperDoll.Bottom() + tokens.Gap, content.W, detailH}
		}
	} else {
		// Keep the paper-doll rail bounded on ultrawide screens. The remaining
		// safe width becomes the detail rail instead of inflating every slot.
		paperW := minf(1000, maxf(720, content.W*0.625))
		layout.PaperDoll = Rect{content.X, content.Y, paperW, content.H}
		slotArea := Rect{layout.PaperDoll.Right() + tokens.Gap, content.Y, maxf(0, content.Right()-layout.PaperDoll.Right()-tokens.Gap), content.H}
		layout.EquipmentViewport = layout.PaperDoll
		layout.PaperDollSlots, layout.PaperDollCluster = paperDollSlotRects(layout.PaperDoll, model.Slots, tokens)
		layout.EquipmentSlots = append([]EquipmentSlotRect(nil), layout.PaperDollSlots...)
		layout.DetailPanel = Rect{slotArea.X, slotArea.Y, slotArea.W, minf(slotArea.H, equipmentSummaryHeight(model, slotArea.H))}
		if state.Selection.HasSelection && state.DetailOpen {
			layout.DetailPanel.H = minf(slotArea.H, 320)
		}
	}
	if state.Selection.HasSelection && layout.DetailPanel.W > 0 {
		actionEdge, actionHeight := detailActionMetrics(layout.DetailPanel, tokens)
		if layout.Portrait {
			buttonW := (layout.DetailPanel.W - 3*tokens.Gap) / 2
			layout.PrimaryAction = Rect{layout.DetailPanel.X + tokens.Gap, layout.DetailPanel.Bottom() - actionEdge - actionHeight, buttonW, actionHeight}
			layout.SecondaryAction = Rect{layout.PrimaryAction.Right() + tokens.Gap, layout.PrimaryAction.Y, buttonW, layout.PrimaryAction.H}
		} else {
			buttonW := maxf(tokens.MinTouchTarget, (layout.DetailPanel.W-3*tokens.Gap)/2)
			layout.PrimaryAction = Rect{layout.DetailPanel.X + tokens.Gap, layout.DetailPanel.Bottom() - actionEdge - actionHeight, buttonW, actionHeight}
			layout.SecondaryAction = Rect{layout.PrimaryAction.Right() + tokens.Gap, layout.PrimaryAction.Y, buttonW, layout.PrimaryAction.H}
		}
		layoutDetailContent(&layout, tokens)
	}
	return layout
}

func layoutDetailContent(layout *MobileInventoryLayout, tokens InventoryTokens) {
	if layout == nil || layout.DetailPanel.W <= 0 || layout.DetailPanel.H <= 0 {
		return
	}
	compact := layout.DetailPanel.H < 260
	iconSize := minf(48, maxf(40, layout.DetailPanel.H*0.18))
	titleY, titleHeight, metaGap, metaHeight, descriptionGap := float32(50), float32(40), float32(8), float32(24), float32(12)
	if compact {
		iconSize = 40
		titleY, titleHeight, metaGap, metaHeight, descriptionGap = 46, 32, 4, 20, 8
	}
	layout.DetailIcon = Rect{X: layout.DetailPanel.X + tokens.Gap, Y: layout.DetailPanel.Y + titleY + maxf(0, (titleHeight-iconSize)/2), W: iconSize, H: iconSize}
	textX := layout.DetailIcon.Right() + tokens.Gap
	textW := maxf(0, layout.DetailPanel.Right()-textX-tokens.Gap)
	// Keep the title and metadata on deliberately separate rows. Bitmap text
	// has less forgiving ascent/descent than a font layout, so a merely
	// non-overlapping rectangle still reads as collided on smaller screens.
	layout.DetailTitle = Rect{X: textX, Y: layout.DetailPanel.Y + titleY, W: textW, H: titleHeight}
	layout.DetailMeta = Rect{X: textX, Y: layout.DetailTitle.Bottom() + metaGap, W: textW, H: metaHeight}
	descriptionY := maxf(layout.DetailIcon.Bottom(), layout.DetailMeta.Bottom()) + descriptionGap
	descriptionBottom := layout.DetailPanel.Bottom() - tokens.Gap
	if layout.PrimaryAction.H > 0 {
		descriptionBottom = layout.PrimaryAction.Y - tokens.Gap
	}
	layout.DetailDescription = Rect{X: layout.DetailPanel.X + tokens.Gap, Y: descriptionY, W: maxf(0, layout.DetailPanel.W-2*tokens.Gap), H: maxf(0, descriptionBottom-descriptionY)}
}

func detailActionMetrics(panel Rect, tokens InventoryTokens) (edge, height float32) {
	// A short portrait sheet still needs room for a readable description. Use
	// the 48px touch minimum and a compact inset before falling back to the
	// roomier desktop-scale footer.
	if panel.H < 260 {
		return maxf(8, tokens.Gap/2), 48
	}
	return tokens.Edge, 56
}

func equipmentSummaryHeight(model MobileEquipmentModel, available float32) float32 {
	if available <= 0 {
		return 0
	}
	equipped := 0
	for _, slot := range model.Slots {
		if slot.HasItem {
			equipped++
		}
	}
	// Header, helper, footer, and one 52-64px row fit in the compact base
	// height. Additional equipped rows earn space up to a bounded rail.
	wanted := 150 + float32(equipped)*72
	return minf(320, maxf(200, wanted))
}

// EquipmentPresentationSlots is the single slot collection consumed by a
// renderer. EquipmentSlots remains available for legacy hit-test callers,
// but it must not become a second visual representation.
func EquipmentPresentationSlots(layout MobileInventoryLayout) []EquipmentSlotRect {
	if len(layout.PaperDollSlots) > 0 {
		return layout.PaperDollSlots
	}
	return layout.EquipmentSlots
}

func paperDollSlotRects(area Rect, slots []EquipmentSlotModel, tokens InventoryTokens) ([]EquipmentSlotRect, Rect) {
	if area.W <= 0 || area.H <= 0 {
		return nil, Rect{}
	}
	result := make([]EquipmentSlotRect, 0, len(slots))
	cluster := Rect{}
	if area.W < 600 {
		gap := minf(tokens.Gap, 12)
		headH := minf(70, maxf(58, area.W*0.18))
		headW := maxf(48, (area.W-4*gap)/3)
		headY := area.Y + 36
		for i := 0; i < minInt(3, len(slots)); i++ {
			x := area.X + gap + float32(i)*(headW+gap)
			result = append(result, equipmentSlotRectAt(slots[i], i, Rect{x, headY, headW, headH}))
		}
		previewW := minf(150, maxf(112, area.W*0.36))
		sideW := minf(112, maxf(90, (area.W-previewW-2*gap)/2))
		groupW := 2*sideW + previewW + 2*gap
		groupX := area.X + maxf(0, (area.W-groupW)/2)
		leftX := groupX
		previewX := leftX + sideW + gap
		rightX := previewX + previewW + gap
		sideH := minf(70, maxf(58, area.W*0.18))
		sideY := headY + headH + gap
		for i := 3; i < minInt(7, len(slots)); i++ {
			pair := i - 3
			x := leftX
			if pair%2 == 1 {
				x = rightX
			}
			y := sideY + float32(pair/2)*(sideH+gap)
			result = append(result, equipmentSlotRectAt(slots[i], i, Rect{x, y, sideW, sideH}))
		}
		previewH := minf(168, maxf(128, area.H*0.40))
		preview := Rect{previewX, sideY, previewW, previewH}
		lowerY := maxf(sideY+2*(sideH+gap)-gap, preview.Bottom()+gap)
		lowerW := maxf(48, (area.W-4*gap)/3)
		lowerH := minf(64, maxf(52, area.W*0.16))
		for i := 7; i < minInt(10, len(slots)); i++ {
			col := i - 7
			x := area.X + gap + float32(col)*(lowerW+gap)
			result = append(result, equipmentSlotRectAt(slots[i], i, Rect{x, lowerY, lowerW, lowerH}))
		}
		if len(slots) > 10 {
			ammoX := area.X + (area.W-lowerW)/2
			result = append(result, equipmentSlotRectAt(slots[10], 10, Rect{ammoX, lowerY + lowerH + gap, lowerW, lowerH}))
		}
		cluster = equipmentClusterBounds(result, preview)
		return result, cluster
	}
	centerW := minf(420, maxf(280, area.W*0.34))
	leftW := minf(210, maxf(150, area.W*0.20))
	groupGap := minf(28, maxf(16, area.W*0.02))
	groupW := leftW*2 + centerW + groupGap*2
	groupX := area.X + maxf(0, (area.W-groupW)/2)
	leftX := groupX
	rightX := groupX + leftW + centerW + groupGap
	centerX := groupX + leftW + groupGap
	centerY := area.Y + 36
	headH := minf(112, maxf(76, area.H*0.09))
	headW := minf(190, maxf(118, centerW*0.55))
	if len(slots) > 0 {
		result = append(result, equipmentSlotRectAt(slots[0], 0, Rect{centerX + (centerW-headW)/2, centerY, headW, headH}))
	}
	if len(slots) > 1 {
		result = append(result, equipmentSlotRectAt(slots[1], 1, Rect{leftX, centerY, leftW, headH}))
	}
	if len(slots) > 2 {
		result = append(result, equipmentSlotRectAt(slots[2], 2, Rect{rightX, centerY, leftW, headH}))
	}
	sideY := centerY + headH + groupGap
	availableSlotH := (area.H - (sideY - area.Y) - 3*tokens.Gap) / 4
	slotH := minf(210, maxf(80, minf(area.W*0.18, availableSlotH)))
	for i := 3; i < len(slots); i++ {
		pair := i - 3
		x := leftX
		if pair%2 == 1 {
			x = rightX
		}
		rect := Rect{x, sideY + float32(pair/2)*(slotH+tokens.Gap), leftW, slotH}
		if rect.Right() <= area.Right() && rect.Bottom() <= area.Bottom() {
			result = append(result, equipmentSlotRectAt(slots[i], i, rect))
		}
	}
	preview := Rect{centerX, sideY + 4, centerW, minf(460, maxf(280, area.H*0.30))}
	cluster = equipmentClusterBounds(result, preview)
	return result, cluster
}

func equipmentSlotRectAt(slot EquipmentSlotModel, index int, rect Rect) EquipmentSlotRect {
	return EquipmentSlotRect{Index: index, Location: slot.Location, Label: slot.Label, Rect: rect}
}

func equipmentClusterBounds(slots []EquipmentSlotRect, preview Rect) Rect {
	cluster := preview
	for _, slot := range slots {
		if cluster.W <= 0 || cluster.H <= 0 {
			cluster = slot.Rect
			continue
		}
		left := minf(cluster.X, slot.Rect.X)
		top := minf(cluster.Y, slot.Rect.Y)
		right := maxf(cluster.Right(), slot.Rect.Right())
		bottom := maxf(cluster.Bottom(), slot.Rect.Bottom())
		cluster = Rect{left, top, right - left, bottom - top}
	}
	return cluster
}

func equipmentPaperDollMinHeight(width float32, tokens InventoryTokens) float32 {
	if width < 600 {
		gap := minf(tokens.Gap, 12)
		headH := minf(70, maxf(58, width*0.18))
		sideH := minf(70, maxf(58, width*0.18))
		previewH := minf(168, maxf(128, width*0.40))
		lowerH := minf(64, maxf(52, width*0.16))
		upperH := maxf(2*sideH+gap, previewH)
		// The mobile equipment model includes Ammo after the three lower slots.
		// Reserve that final row so the last slot remains inside the paper-doll
		// rail on narrow portrait screens.
		return 36 + headH + gap + upperH + gap + 3*lowerH + 2*gap
	}
	headH := minf(112, maxf(76, width*0.09))
	slotH := minf(210, maxf(96, width*0.18))
	return 36 + headH + minf(28, maxf(16, width*0.02)) + 4*slotH + 3*tokens.Gap
}

// EquipmentPreviewRect is shared by the renderer-neutral command stream and
// the Android renderer so the avatar bounds are not inferred twice.
func EquipmentPreviewRect(layout MobileInventoryLayout) Rect {
	if layout.PaperDollCluster.W <= 0 || layout.PaperDollCluster.H <= 0 {
		return Rect{}
	}
	// The preview is the central stage used by paperDollSlotRects. Rebuild its
	// compact geometry from the presentation area rather than stretching it to
	// the full paper-doll panel.
	_, preview := paperDollPreviewRect(layout.PaperDoll)
	return preview
}

func paperDollPreviewRect(area Rect) ([]EquipmentSlotRect, Rect) {
	if area.W < 600 {
		gap := float32(12)
		previewW := minf(150, maxf(112, area.W*0.36))
		sideW := minf(112, maxf(90, (area.W-previewW-2*gap)/2))
		groupW := 2*sideW + previewW + 2*gap
		groupX := area.X + maxf(gap, (area.W-groupW)/2)
		previewX := groupX + sideW + gap
		headH := minf(70, maxf(58, area.W*0.18))
		sideY := area.Y + 36 + headH + gap
		previewH := minf(168, maxf(128, area.H*0.40))
		return nil, Rect{previewX, sideY, previewW, previewH}
	}
	centerW := minf(420, maxf(280, area.W*0.34))
	leftW := minf(210, maxf(150, area.W*0.20))
	groupGap := minf(28, maxf(16, area.W*0.02))
	groupW := leftW*2 + centerW + groupGap*2
	groupX := area.X + maxf(0, (area.W-groupW)/2)
	centerX := groupX + leftW + groupGap
	headH := minf(112, maxf(76, area.H*0.09))
	return nil, Rect{centerX, area.Y + 36 + headH + groupGap + 4, centerW, minf(460, maxf(280, area.H*0.30))}
}

func EquipmentScrollExtent(layout MobileInventoryLayout, slotCount int, state InventoryInteractionState, tokens InventoryTokens) InventoryScrollState {
	if !layout.Portrait {
		return InventoryScrollState{ViewportExtent: layout.EquipmentViewport.H, ContentExtent: layout.EquipmentViewport.H, Offset: state.Scroll.Offset, RowExtent: 0}
	}
	// The compact paper doll fits as one composed stage; scrolling belongs to
	// the summary/detail sheet rather than separating slots from the avatar.
	return InventoryScrollState{ViewportExtent: layout.EquipmentViewport.H, ContentExtent: layout.EquipmentViewport.H, Offset: state.Scroll.Offset, RowExtent: 0}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
