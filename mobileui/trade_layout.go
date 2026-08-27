package mobileui

type TradeRowKind uint8

const (
	TradeInventoryRow TradeRowKind = iota + 1
	TradeOwnOfferRow
	TradePartnerOfferRow
)

type TradeRowRect struct {
	Kind  TradeRowKind
	Index int
	Rect  Rect
}

type TradeLayout struct {
	Safe, Panel, Header, BackButton                                             Rect
	InventoryPanel, OwnPanel, PartnerPanel                                      Rect
	InventoryRows, OwnRows, PartnerRows                                         []TradeRowRect
	AddZeny, Conclude, Commit, Cancel                                           Rect
	Notice                                                                      Rect
	RequestModal, RequestAccept, RequestDecline                                 Rect
	QuantityModal, QuantityMinus, QuantityPlus, QuantityConfirm, QuantityCancel Rect
	Portrait                                                                    bool
}

type TradeQuantityAction uint8

const (
	TradeQuantityItem TradeQuantityAction = iota
	TradeQuantityZeny
)

type TradeQuantityState struct {
	Open      bool
	Action    TradeQuantityAction
	ItemIndex uint16
	Maximum   int
	Value     int
}

func (q *TradeQuantityState) OpenItem(item InventoryItemModel) {
	if q == nil {
		return
	}
	maximum := item.Quantity
	if maximum < 1 {
		maximum = 1
	}
	*q = TradeQuantityState{Open: true, Action: TradeQuantityItem, ItemIndex: item.Index, Maximum: maximum, Value: maximum}
}

func (q *TradeQuantityState) OpenZeny(maximum uint32) {
	if q == nil {
		return
	}
	maxInt := int(^uint(0) >> 1)
	if uint64(maximum) > uint64(maxInt) {
		maximum = uint32(maxInt)
	}
	if maximum == 0 {
		return
	}
	*q = TradeQuantityState{Open: true, Action: TradeQuantityZeny, Maximum: int(maximum), Value: 1}
}

func (q *TradeQuantityState) SetValue(value int) {
	if q == nil {
		return
	}
	if value < 1 {
		value = 1
	}
	if value > q.Maximum {
		value = q.Maximum
	}
	q.Value = value
}

func (q *TradeQuantityState) Increment() {
	if q != nil {
		q.SetValue(q.Value + 1)
	}
}
func (q *TradeQuantityState) Decrement() {
	if q != nil {
		q.SetValue(q.Value - 1)
	}
}

func (q *TradeQuantityState) Confirm() (itemIndex uint16, quantity int, zeny bool, ok bool) {
	if q == nil || !q.Open || q.Value < 1 || q.Value > q.Maximum {
		return 0, 0, false, false
	}
	itemIndex, quantity, zeny, ok = q.ItemIndex, q.Value, q.Action == TradeQuantityZeny, true
	*q = TradeQuantityState{}
	return
}

func (q *TradeQuantityState) Cancel() {
	if q != nil {
		*q = TradeQuantityState{}
	}
}

func LayoutTrade(viewport Viewport, model MobileTradeModel, state TradeInteractionState) TradeLayout {
	safe := viewport.SafeRect()
	l := TradeLayout{Safe: safe, Portrait: viewport.IsPortrait()}
	if safe.W <= 0 || safe.H <= 0 {
		return l
	}
	tokens := DefaultTradeTokens()
	contentW := minf(safe.W-2*tokens.Edge, tokens.ContentMaxWidth)
	contentX := safe.X + (safe.W-contentW)/2
	l.Panel = Rect{contentX, safe.Y + tokens.Edge, contentW, maxf(0, safe.H-2*tokens.Edge)}
	l.Header = Rect{contentX, safe.Y + tokens.Edge, contentW, tokens.HeaderHeight}
	l.BackButton = Rect{l.Header.X, l.Header.Y, 112, l.Header.H}
	l.RequestModal, l.RequestAccept, l.RequestDecline = centeredTradeModal(safe, tokens, 760, 380)
	if !model.Open {
		return l
	}
	contentY := l.Header.Bottom() + tokens.Gap
	contentBottom := safe.Bottom() - tokens.Edge
	if l.Portrait {
		inventoryH := minf(330, maxf(220, (contentBottom-contentY)*0.43))
		l.InventoryPanel = Rect{contentX, contentY, contentW, inventoryH}
		offerY := l.InventoryPanel.Bottom() + tokens.Gap
		offerH := maxf(0, contentBottom-offerY)
		l.OwnPanel = Rect{contentX, offerY, contentW, maxf(0, offerH*0.47)}
		l.PartnerPanel = Rect{contentX, l.OwnPanel.Bottom() + tokens.Gap, contentW, maxf(0, contentBottom-l.OwnPanel.Bottom()-tokens.Gap)}
	} else {
		inventoryW := minf(640, maxf(420, contentW*0.34))
		l.InventoryPanel = Rect{contentX, contentY, inventoryW, maxf(0, contentBottom-contentY)}
		offerX := l.InventoryPanel.Right() + tokens.Gap
		offerW := maxf(0, contentX+contentW-offerX)
		l.OwnPanel = Rect{offerX, contentY, offerW, maxf(0, (contentBottom-contentY-tokens.Gap)*0.50)}
		l.PartnerPanel = Rect{offerX, l.OwnPanel.Bottom() + tokens.Gap, offerW, maxf(0, contentBottom-l.OwnPanel.Bottom()-tokens.Gap)}
	}

	rowH := maxf(tokens.MinTouchTarget, 64)
	inventoryArea := Rect{l.InventoryPanel.X + tokens.Gap, l.InventoryPanel.Y + 48, maxf(0, l.InventoryPanel.W-2*tokens.Gap), maxf(0, l.InventoryPanel.H-60)}
	first := int(state.Scroll.Offset / rowH)
	visible := maxInt(1, int(inventoryArea.H/rowH)+2)
	last := minInt(len(model.Inventory), first+visible)
	for i := first; i < last; i++ {
		row := Rect{inventoryArea.X, inventoryArea.Y + float32(i-first)*rowH, inventoryArea.W, rowH - tokens.Gap}
		l.InventoryRows = append(l.InventoryRows, TradeRowRect{Kind: TradeInventoryRow, Index: i, Rect: row})
	}
	addRows := func(area Rect, count int, kind TradeRowKind) []TradeRowRect {
		result := []TradeRowRect{}
		if area.W <= 0 || area.H <= 48 {
			return result
		}
		rowArea := Rect{area.X + tokens.Gap, area.Y + 48, maxf(0, area.W-2*tokens.Gap), maxf(0, area.H-88)}
		maxRows := maxInt(1, int(rowArea.H/rowH))
		if count < maxRows {
			maxRows = count
		}
		for i := 0; i < maxRows; i++ {
			result = append(result, TradeRowRect{Kind: kind, Index: i, Rect: Rect{rowArea.X, rowArea.Y + float32(i)*rowH, rowArea.W, rowH - tokens.Gap}})
		}
		return result
	}
	l.OwnRows = addRows(l.OwnPanel, len(model.OwnOffer), TradeOwnOfferRow)
	l.PartnerRows = addRows(l.PartnerPanel, len(model.PartnerOffer), TradePartnerOfferRow)

	buttonY := l.PartnerPanel.Bottom() - tokens.MinTouchTarget - tokens.Gap
	buttonW := maxf(tokens.MinTouchTarget, (contentW-3*tokens.Gap)/4)
	buttons := [](*Rect){&l.AddZeny, &l.Conclude, &l.Commit, &l.Cancel}
	for i, button := range buttons {
		*button = Rect{contentX + float32(i)*(buttonW+tokens.Gap), buttonY, buttonW, tokens.MinTouchTarget}
	}
	l.Notice = Rect{l.InventoryPanel.X + tokens.Gap, l.InventoryPanel.Bottom() - 28, l.InventoryPanel.W - 2*tokens.Gap, 22}
	if state.Quantity.Open {
		l.QuantityModal, l.QuantityMinus, l.QuantityPlus, l.QuantityConfirm, l.QuantityCancel = LayoutTradeQuantity(safe, tokens)
	}
	return l
}

type TradeTokens struct {
	Edge, Gap, HeaderHeight, ContentMaxWidth, MinTouchTarget float32
}

func DefaultTradeTokens() TradeTokens {
	return TradeTokens{Edge: 20, Gap: 16, HeaderHeight: 64, ContentMaxWidth: 1800, MinTouchTarget: 56}
}

func centeredTradeModal(safe Rect, tokens TradeTokens, width, height float32) (Rect, Rect, Rect) {
	width = minf(width, maxf(tokens.MinTouchTarget*4, safe.W-2*tokens.Edge))
	height = minf(height, maxf(tokens.MinTouchTarget*5, safe.H-2*tokens.Edge))
	modal := Rect{safe.X + (safe.W-width)/2, safe.Y + (safe.H-height)/2, width, height}
	buttonW := maxf(tokens.MinTouchTarget, (width-3*tokens.Gap)/2)
	buttonY := modal.Bottom() - tokens.Gap - tokens.MinTouchTarget
	accept := Rect{modal.X + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	decline := Rect{accept.Right() + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	return modal, accept, decline
}

func LayoutTradeQuantity(safe Rect, tokens TradeTokens) (Rect, Rect, Rect, Rect, Rect) {
	modalW := minf(720, safe.W-2*tokens.Edge)
	modalH := minf(300, safe.H-2*tokens.Edge)
	modal := Rect{safe.X + (safe.W-modalW)/2, safe.Y + (safe.H-modalH)/2, modalW, modalH}
	buttonW := maxf(tokens.MinTouchTarget, (modalW-4*tokens.Gap)/3)
	buttonY := modal.Bottom() - tokens.Gap - tokens.MinTouchTarget
	minus := Rect{modal.X + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	plus := Rect{minus.Right() + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	confirm := Rect{plus.Right() + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	cancel := Rect{modal.Right() - 128 - tokens.Gap, modal.Y + tokens.Gap, 128, tokens.MinTouchTarget}
	return modal, minus, plus, confirm, cancel
}

type TradeInteractionState struct {
	Scroll   ScrollState
	Quantity TradeQuantityState
}

func TradeScrollExtent(model MobileTradeModel, layout TradeLayout, state TradeInteractionState) ScrollState {
	rowH := maxf(DefaultTradeTokens().MinTouchTarget, 64)
	content := float32(len(model.Inventory)) * rowH
	if content > 0 {
		content -= DefaultTradeTokens().Gap
	}
	return ScrollState{ViewportExtent: maxf(0, layout.InventoryPanel.H-60), ContentExtent: content, Offset: state.Scroll.Offset, RowExtent: rowH}
}
