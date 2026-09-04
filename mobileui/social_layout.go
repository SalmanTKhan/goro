package mobileui

// socialRowExtent is the pitch of one friend or party row. Each stacks a name
// over a status line, so it needs room for two lines of the real UI font.
const socialRowExtent float32 = 96

type SocialRowKind uint8

const (
	SocialRowFriend SocialRowKind = iota + 1
	SocialRowPartyMember
)

type SocialRowRect struct {
	Kind  SocialRowKind
	Index int
	Rect  Rect
}

type SocialLayout struct {
	Safe, Panel, Header, FriendsTab, PartyTab, ListViewport, DetailPanel, BackButton           Rect
	PrimaryAction, SecondaryAction, TertiaryAction, SettingsAction, Notice                     Rect
	RequestModal, RequestAccept, RequestDecline                                                Rect
	SettingsModal, SettingsEach, SettingsEven, SettingsRefuse, SettingsConfirm, SettingsCancel Rect
	TextInputModal, TextInputConfirm, TextInputCancel                                          Rect
	Rows                                                                                       []SocialRowRect
	Portrait                                                                                   bool
}

type SocialInteractionState struct {
	Tab                   SocialTab
	SelectedFriendAccount uint32
	SelectedFriendChar    uint32
	SelectedPartyAccount  uint32
	HasSelection          bool
	SettingsOpen          bool
	SettingsExpShare      uint32
	SettingsRefuseInvites bool
	HandledFriendAccount  uint32
	HandledFriendChar     uint32
	HandledPartyRequest   uint32
	Scroll                ScrollState
}

type SocialTokens struct {
	Edge, Gap, HeaderHeight, TabHeight, ContentMaxWidth, ListWidth, MinTouchTarget float32
}

func DefaultSocialTokens() SocialTokens {
	return SocialTokens{Edge: 20, Gap: 16, HeaderHeight: 64, TabHeight: 56, ContentMaxWidth: 1600, ListWidth: 620, MinTouchTarget: 48}
}

func LayoutSocial(viewport Viewport, tokens SocialTokens, model MobileSocialModel, state SocialInteractionState) SocialLayout {
	if tokens.Edge <= 0 {
		tokens = DefaultSocialTokens()
	}
	safe := viewport.SafeRect()
	l := SocialLayout{Safe: safe, Portrait: viewport.IsPortrait()}
	if safe.W <= 0 || safe.H <= 0 {
		return l
	}
	contentW := minf(safe.W-2*tokens.Edge, tokens.ContentMaxWidth)
	contentX := safe.X + (safe.W-contentW)/2
	l.Panel = Rect{contentX, safe.Y + tokens.Edge, contentW, maxf(0, safe.H-2*tokens.Edge)}
	l.Header = Rect{contentX, safe.Y + tokens.Edge, contentW, tokens.HeaderHeight}
	l.BackButton = Rect{l.Header.X, l.Header.Y, BackButtonWidth(), l.Header.H}
	tabY := l.Header.Bottom() + tokens.Gap
	tabW := minf(180, maxf(tokens.MinTouchTarget, (contentW-tokens.Gap)/2))
	l.FriendsTab = Rect{contentX, tabY, tabW, tokens.TabHeight}
	l.PartyTab = Rect{l.FriendsTab.Right() + tokens.Gap, tabY, tabW, tokens.TabHeight}
	contentY := tabY + tokens.TabHeight + tokens.Gap
	contentBottom := safe.Bottom() - tokens.Edge

	if l.Portrait {
		listH := minf(330, maxf(220, (contentBottom-contentY)*0.48))
		l.ListViewport = Rect{contentX, contentY, contentW, listH}
		l.DetailPanel = Rect{contentX, l.ListViewport.Bottom() + tokens.Gap, contentW, maxf(0, contentBottom-l.ListViewport.Bottom()-tokens.Gap)}
	} else {
		listW := minf(tokens.ListWidth, maxf(360, contentW*0.43))
		l.ListViewport = Rect{contentX, contentY, listW, maxf(0, contentBottom-contentY)}
		l.DetailPanel = Rect{l.ListViewport.Right() + tokens.Gap, contentY, maxf(0, contentW-listW-tokens.Gap), l.ListViewport.H}
	}
	rowArea := Rect{l.ListViewport.X + tokens.Gap, l.ListViewport.Y + 48, maxf(0, l.ListViewport.W-2*tokens.Gap), maxf(0, l.ListViewport.H-56)}
	rowH := maxf(tokens.MinTouchTarget, socialRowExtent)
	count := len(model.Friends)
	kind := SocialRowFriend
	if state.Tab == SocialTabParty {
		count = len(model.PartyMembers)
		kind = SocialRowPartyMember
	}
	state.Scroll.ViewportExtent = rowArea.H
	state.Scroll.ContentExtent = maxf(0, float32(count)*rowH-tokens.Gap)
	state.Scroll.SetOffset(state.Scroll.Offset)
	first := int(state.Scroll.Offset / rowH)
	visible := maxInt(1, int(rowArea.H/rowH)+2)
	last := minInt(count, first+visible)
	for i := first; i < last; i++ {
		row := Rect{rowArea.X, rowArea.Y + float32(i-first)*rowH, rowArea.W, rowH - tokens.Gap}
		l.Rows = append(l.Rows, SocialRowRect{Kind: kind, Index: i, Rect: row})
	}
	if l.DetailPanel.W > 0 && l.DetailPanel.H >= 120 {
		buttonY := l.DetailPanel.Bottom() - tokens.Gap - tokens.MinTouchTarget
		buttonW := maxf(tokens.MinTouchTarget, (l.DetailPanel.W-3*tokens.Gap)/2)
		l.PrimaryAction = Rect{l.DetailPanel.X + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
		l.SecondaryAction = Rect{l.PrimaryAction.Right() + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
		if state.Tab == SocialTabParty && model.PartyActive {
			l.TertiaryAction = Rect{l.DetailPanel.X + tokens.Gap, buttonY - tokens.Gap - tokens.MinTouchTarget, l.DetailPanel.W - 2*tokens.Gap, tokens.MinTouchTarget}
			if model.PartySettings.CanEdit {
				settingsW := minf(220, l.DetailPanel.W-2*tokens.Gap)
				l.SettingsAction = Rect{l.DetailPanel.Right() - tokens.Gap - settingsW, l.DetailPanel.Y + 42, settingsW, tokens.MinTouchTarget}
			}
		}
	}
	l.Notice = Rect{l.ListViewport.X + tokens.Gap, l.ListViewport.Bottom() - 26, l.ListViewport.W - 2*tokens.Gap, 22}
	l.RequestModal, l.RequestAccept, l.RequestDecline = centeredSocialModal(safe, tokens, 720, 360)
	l.SettingsModal, l.SettingsEach, l.SettingsEven, l.SettingsRefuse, l.SettingsConfirm, l.SettingsCancel = centeredPartySettings(safe, tokens)
	l.TextInputModal, l.TextInputConfirm, l.TextInputCancel = centeredSocialModal(safe, tokens, 720, 340)
	return l
}

func centeredSocialModal(safe Rect, tokens SocialTokens, width, height float32) (Rect, Rect, Rect) {
	width = minf(width, maxf(tokens.MinTouchTarget*4, safe.W-2*tokens.Edge))
	height = minf(height, maxf(tokens.MinTouchTarget*5, safe.H-2*tokens.Edge))
	modal := Rect{safe.X + (safe.W-width)/2, safe.Y + (safe.H-height)/2, width, height}
	buttonW := maxf(tokens.MinTouchTarget, (width-3*tokens.Gap)/2)
	buttonY := modal.Bottom() - tokens.Gap - tokens.MinTouchTarget
	accept := Rect{modal.X + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	decline := Rect{accept.Right() + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	return modal, accept, decline
}

func centeredPartySettings(safe Rect, tokens SocialTokens) (Rect, Rect, Rect, Rect, Rect, Rect) {
	modal, _, _ := centeredSocialModal(safe, tokens, 720, 420)
	rowW := maxf(tokens.MinTouchTarget, (modal.W-3*tokens.Gap)/2)
	rowY := modal.Y + 126
	each := Rect{modal.X + tokens.Gap, rowY, rowW, tokens.MinTouchTarget}
	even := Rect{each.Right() + tokens.Gap, rowY, rowW, tokens.MinTouchTarget}
	refuse := Rect{modal.X + tokens.Gap, rowY + tokens.MinTouchTarget + tokens.Gap, modal.W - 2*tokens.Gap, tokens.MinTouchTarget}
	buttonY := modal.Bottom() - tokens.Gap - tokens.MinTouchTarget
	buttonW := maxf(tokens.MinTouchTarget, (modal.W-3*tokens.Gap)/2)
	confirm := Rect{modal.X + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	cancel := Rect{confirm.Right() + tokens.Gap, buttonY, buttonW, tokens.MinTouchTarget}
	return modal, each, even, refuse, confirm, cancel
}

func SocialScrollExtent(model MobileSocialModel, state SocialInteractionState, layout SocialLayout, tokens SocialTokens) ScrollState {
	if tokens.Edge <= 0 {
		tokens = DefaultSocialTokens()
	}
	count := len(model.Friends)
	if state.Tab == SocialTabParty {
		count = len(model.PartyMembers)
	}
	rowH := maxf(tokens.MinTouchTarget, socialRowExtent)
	viewportH := maxf(0, layout.ListViewport.H-56)
	return ScrollState{ViewportExtent: viewportH, ContentExtent: maxf(0, float32(count)*rowH-tokens.Gap), Offset: state.Scroll.Offset, RowExtent: rowH}
}
