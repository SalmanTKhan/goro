package mobileui

import "github.com/kivutar/goro/input"

type Rect struct{ X, Y, W, H float32 }

func (r Rect) Right() float32  { return r.X + r.W }
func (r Rect) Bottom() float32 { return r.Y + r.H }
func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && y >= r.Y && x <= r.Right() && y <= r.Bottom()
}
func (r Rect) Intersects(other Rect) bool {
	return r.X < other.Right() && other.X < r.Right() && r.Y < other.Bottom() && other.Y < r.Bottom()
}

type Viewport struct{ Width, Height, SafeTop, SafeRight, SafeBottom, SafeLeft float32 }

func FoldOuterViewport() Viewport { return Viewport{Width: 2268, Height: 832} }

func (v Viewport) SafeRect() Rect {
	left, top := maxf(0, v.SafeLeft), maxf(0, v.SafeTop)
	right, bottom := maxf(0, v.SafeRight), maxf(0, v.SafeBottom)
	return Rect{X: left, Y: top, W: maxf(0, v.Width-left-right), H: maxf(0, v.Height-top-bottom)}
}

type MobileTokens struct {
	Edge, Gap, MenuSize, SkillSize, PanelHeight, TargetHeight, ChatHeight, MinTouchTarget float32
}

// Named reference-scale roles keep mobile readability independent of raw
// device resolution. The Android adapter applies the final physical scale.
type MobileTypography struct {
	Title, SectionTitle, Body, BodyStrong, Secondary, Button, ItemName, ItemQuantity, HUDPrimary, HUDSecondary float32
}

func DefaultTypography() MobileTypography {
	return MobileTypography{Title: 48, SectionTitle: 40, Body: 32, BodyStrong: 36, Secondary: 28, Button: 32, ItemName: 36, ItemQuantity: 32, HUDPrimary: 32, HUDSecondary: 28}
}

type MobileSpacing struct{ XXS, XS, S, M, L, XL float32 }

func DefaultSpacing() MobileSpacing { return MobileSpacing{XXS: 4, XS: 8, S: 12, M: 16, L: 24, XL: 32} }

// LabelWidth estimates the rendered width of a label at a typography size.
//
// mobileui has no font backend by design, so this is an estimate rather than a
// measurement. It is deliberately generous: reserving slightly too much space
// costs a few pixels, while reserving too little clips the label, which is the
// failure this exists to prevent.
func LabelWidth(label string, size float32) float32 {
	return float32(len([]rune(label))) * size * 0.6
}

// ControlWidth reserves room for a label plus the padding a control draws
// around it.
func ControlWidth(label string, size float32) float32 {
	return LabelWidth(label, size) + 2*DefaultSpacing().M
}

// BackLabel is the canonical back-navigation label. Layouts reserve space for
// it and renderers draw it, so the two cannot disagree about how wide the
// control has to be.
const BackLabel = "‹ Back"

// BackButtonWidth is the width every screen's back control should use.
func BackButtonWidth() float32 {
	return ControlWidth(BackLabel, DefaultTypography().Body)
}

// StackedHeaderHeight is the height a screen header needs when it stacks two
// lines — a name over a job, a map over its coordinates. One line box of the
// real UI font is about 36 pixels, so a 56-pixel header crushed the two
// together.
func StackedHeaderHeight() float32 { return 96 }

func DefaultTokens() MobileTokens {
	return MobileTokens{
		Edge:           16,
		Gap:            12,
		MenuSize:       72,
		SkillSize:      96,
		PanelHeight:    132,
		TargetHeight:   112,
		ChatHeight:     64,
		MinTouchTarget: 48,
	}
}

type MenuAction struct {
	Screen Screen
	Rect   Rect
}

// MobileScreenHeader is the shared geometry contract for full-screen mobile
// surfaces. Title and subtitle have independent rows so screen-specific
// helper text can never collide with the title baseline.
type MobileScreenHeader struct {
	Panel, Back, Title, Subtitle, Action Rect
}

func LayoutMobileScreenHeader(panel Rect, backWidth, actionWidth float32) MobileScreenHeader {
	header := MobileScreenHeader{Panel: panel}
	if panel.W <= 0 || panel.H <= 0 {
		return header
	}
	backWidth = minf(maxf(0, backWidth), panel.W)
	header.Back = Rect{X: panel.X, Y: panel.Y, W: backWidth, H: panel.H}
	titleX := header.Back.Right() + 16
	titleRight := panel.Right() - 16
	if actionWidth > 0 {
		actionWidth = minf(actionWidth, panel.W)
		header.Action = Rect{X: panel.Right() - actionWidth, Y: panel.Y, W: actionWidth, H: panel.H}
		titleRight = header.Action.X - 16
	}
	titleW := maxf(0, titleRight-titleX)
	header.Title = Rect{X: titleX, Y: panel.Y + 10, W: titleW, H: minf(28, maxf(0, panel.H-10))}
	header.Subtitle = Rect{X: titleX, Y: panel.Y + 42, W: titleW, H: minf(22, maxf(0, panel.H-42))}
	return header
}

type HUDLayout struct {
	Safe, PlayerPanel, TargetPanel, LootPanel, Minimap, Menu, StatusArea, ChatBar, ChatLabel, ChatPrompt, ChatButton, SkillBar, SkillPagePrev, SkillPageNext, MenuPanel, CombatBanner, CombatCancel Rect
	SkillSlots                                                                                                                                                                                      []Rect
	SkillStart                                                                                                                                                                                      int
	SkillsPerPage                                                                                                                                                                                   int
	LootRows                                                                                                                                                                                        []Rect
	MenuActions                                                                                                                                                                                     []MenuAction
}

func LayoutHUD(viewport Viewport, tokens MobileTokens, model MobileHUDModel, navigation Navigation) HUDLayout {
	if tokens.Edge <= 0 {
		tokens = DefaultTokens()
	}
	safe := viewport.SafeRect()
	l := HUDLayout{Safe: safe}
	if safe.W == 0 || safe.H == 0 {
		return l
	}
	portrait := viewport.IsPortrait()
	overlayGap := tokens.Gap
	menuSize := tokens.MenuSize
	panelHeight := tokens.PanelHeight
	targetHeight := tokens.TargetHeight
	chatHeight := tokens.ChatHeight
	skillWidth := maxf(tokens.MinTouchTarget, tokens.SkillSize)
	skillHeight := skillWidth
	if !portrait {
		// Landscape has a short physical edge. Keep the overlay compact and
		// reserve the extra horizontal space for the world while retaining
		// readable, touch-safe controls at normal phone distance.
		overlayGap = 12
		menuSize = 72
		panelHeight = 132
		targetHeight = 112
		chatHeight = 64
		skillWidth = 96
		skillHeight = 96
	}
	panelWidth := minf(360, maxf(220, safe.W*0.30))
	if portrait {
		// Keep the status card as a readable overlay rather than letting it
		// consume the entire portrait width. The minimap and menu retain their
		// own right-side safe region.
		panelWidth = minf(640, maxf(220, safe.W*0.64))
	} else if panelWidth > safe.W-2*tokens.Edge {
		panelWidth = maxf(0, safe.W-2*tokens.Edge)
	}
	l.PlayerPanel = Rect{safe.X + tokens.Edge, safe.Y + tokens.Edge, panelWidth, panelHeight}
	l.Menu = Rect{safe.Right() - tokens.Edge - menuSize, safe.Y + tokens.Edge, menuSize, menuSize}
	miniW := minf(232, maxf(156, safe.W*0.22))
	if miniW > safe.W/2 {
		miniW = maxf(0, safe.W/2)
	}
	if portrait {
		miniW = minf(220, maxf(180, safe.W*0.34))
		miniY := maxf(l.Menu.Bottom()+tokens.Gap, l.PlayerPanel.Bottom()+tokens.Gap)
		l.Minimap = Rect{safe.Right() - tokens.Edge - miniW, miniY, miniW, 224}
	} else {
		l.Minimap = Rect{l.Menu.X - overlayGap - miniW, l.Menu.Y, miniW, 216}
	}
	if !model.Minimap.Visible {
		l.Minimap = Rect{}
	}
	statusW := minf(320, maxf(150, safe.W*0.26))
	if portrait {
		statusW = maxf(0, safe.W-2*tokens.Edge)
		l.StatusArea = Rect{safe.X + tokens.Edge, maxf(l.PlayerPanel.Bottom(), l.Minimap.Bottom()) + tokens.Gap, statusW, 64}
	} else {
		if statusW > safe.Right()-l.PlayerPanel.Right()-2*overlayGap {
			statusW = maxf(0, safe.Right()-l.PlayerPanel.Right()-2*overlayGap)
		}
		l.StatusArea = Rect{l.PlayerPanel.Right() + overlayGap, l.PlayerPanel.Y, statusW, 48}
	}
	chatW := minf(420, maxf(240, safe.W*0.34))
	if portrait {
		chatW = minf(360, maxf(200, safe.W*0.52))
	}
	l.ChatBar = Rect{safe.X + tokens.Edge, safe.Bottom() - tokens.Edge - chatHeight, chatW, chatHeight}
	if l.ChatBar.W > safe.W-2*tokens.Edge {
		l.ChatBar.W = maxf(0, safe.W-2*tokens.Edge)
	}
	chatButtonW := minf(112, l.ChatBar.W)
	l.ChatButton = Rect{l.ChatBar.Right() - chatButtonW, l.ChatBar.Y, chatButtonW, l.ChatBar.H}
	chatTextW := maxf(0, l.ChatButton.X-l.ChatBar.X)
	chatLabelW := minf(52, maxf(36, chatTextW*0.34))
	if chatLabelW > chatTextW {
		chatLabelW = chatTextW
	}
	l.ChatLabel = Rect{l.ChatBar.X + 10, l.ChatBar.Y, maxf(0, chatLabelW-10), l.ChatBar.H}
	promptX := l.ChatLabel.Right() + 8
	l.ChatPrompt = Rect{promptX, l.ChatBar.Y, maxf(0, l.ChatButton.X-promptX-8), l.ChatBar.H}
	// Combat controls occupy their own lower-right region on portrait. The
	// target panel is placed above that region rather than behind it.
	skillSize := maxf(tokens.MinTouchTarget, tokens.SkillSize)
	if portrait && safe.W < 600 {
		skillSize = maxf(72, minf(skillSize, safe.W*0.20))
	}
	skillY := l.ChatBar.Y - overlayGap - skillSize
	if !portrait {
		skillY = safe.Bottom() - tokens.Edge - skillHeight
	}
	targetY := skillY - overlayGap - targetHeight
	l.TargetPanel = Rect{safe.X + tokens.Edge, targetY, panelWidth, targetHeight}
	// The player panel occupies the left rail on landscape displays. Keep the
	// loot rail below it even when the status strip is shorter; otherwise the
	// loot panel is drawn underneath the character panel and its touch rows
	// become unreachable.
	lootTop := l.PlayerPanel.Bottom() + overlayGap
	if portrait {
		lootTop = l.StatusArea.Bottom() + tokens.Gap
	}
	lootBottom := l.TargetPanel.Y - overlayGap
	lootHeaderHeight := float32(32)
	lootRowHeight := maxf(tokens.MinTouchTarget, 52)
	maxLootRows := int((lootBottom - lootTop - lootHeaderHeight) / lootRowHeight)
	if maxLootRows < 0 {
		maxLootRows = 0
	}
	visibleLoot := len(model.Loot)
	if visibleLoot > maxLootRows {
		visibleLoot = maxLootRows
	}
	if visibleLoot > 0 {
		lootWidth := minf(360, maxf(240, safe.W*0.30))
		if lootWidth > safe.W-2*tokens.Edge {
			lootWidth = maxf(0, safe.W-2*tokens.Edge)
		}
		l.LootPanel = Rect{safe.X + tokens.Edge, lootTop, lootWidth, lootHeaderHeight + float32(visibleLoot)*lootRowHeight}
		for i := 0; i < visibleLoot; i++ {
			l.LootRows = append(l.LootRows, Rect{l.LootPanel.X + 4, l.LootPanel.Y + lootHeaderHeight + float32(i)*lootRowHeight, l.LootPanel.W - 8, lootRowHeight - 4})
		}
	}
	perPage := visibleSkillCap(safe, portrait)
	l.SkillsPerPage = perPage
	visibleSkills := minInt(perPage, len(model.Skills))
	if visibleSkills > 0 && portrait {
		pages := (len(model.Skills) + perPage - 1) / perPage
		page := navigation.SkillPage
		if page < 0 {
			page = 0
		}
		if page >= pages {
			page = pages - 1
		}
		l.SkillStart = page * perPage
	}
	barW := float32(visibleSkills) * skillWidth
	if visibleSkills > 1 {
		barW += float32(visibleSkills-1) * tokens.Gap
	}
	if portrait && barW > safe.W-2*tokens.Edge {
		barW = safe.W - 2*tokens.Edge
	}
	pageGap := float32(0)
	if portrait && len(model.Skills) > visibleSkills {
		pageGap = tokens.Gap
		buttonSize := maxf(tokens.MinTouchTarget, skillSize)
		available := safe.W - 2*tokens.Edge - 2*(buttonSize+tokens.Gap)
		if available < 0 {
			available = 0
		}
		barW = minf(barW, available)
		groupW := barW + 2*(buttonSize+tokens.Gap)
		groupX := safe.Right() - tokens.Edge - groupW
		l.SkillPagePrev = Rect{groupX, skillY, buttonSize, buttonSize}
		l.SkillBar = Rect{l.SkillPagePrev.Right() + tokens.Gap, skillY, barW, buttonSize}
		l.SkillPageNext = Rect{l.SkillBar.Right() + tokens.Gap, l.SkillBar.Y, buttonSize, buttonSize}
	} else {
		l.SkillBar = Rect{safe.Right() - tokens.Edge - barW, skillY, barW, skillHeight}
	}
	for i := 0; i < visibleSkills; i++ {
		slotW := skillWidth
		if pageGap > 0 {
			slotW = l.SkillBar.W / float32(visibleSkills)
			if visibleSkills > 1 {
				slotW -= tokens.Gap * float32(visibleSkills-1) / float32(visibleSkills)
			}
		}
		layoutX := l.SkillBar.X + float32(i)*(slotW+tokens.Gap)
		l.SkillSlots = append(l.SkillSlots, Rect{layoutX, l.SkillBar.Y, slotW, l.SkillBar.H})
	}
	if navigation.Targeting.Mode != input.SkillTargetIdle {
		bannerW, bannerH := minf(440, maxf(300, safe.W*0.34)), float32(52)
		bannerY := l.TargetPanel.Y - tokens.Gap - bannerH
		if bannerY < safe.Y+tokens.Edge {
			bannerY = l.SkillBar.Y - tokens.Gap - bannerH
		}
		l.CombatBanner = Rect{safe.X + (safe.W-bannerW)/2, bannerY, bannerW, bannerH}
		l.CombatCancel = Rect{l.CombatBanner.Right() - 120, l.CombatBanner.Y + 2, 112, 48}
	}
	if navigation.MenuOpen {
		menuW, rowH := minf(560, maxf(300, safe.W*0.30)), float32(64)
		menuRows := 7
		if portrait {
			menuW = minf(maxf(0, safe.W-2*tokens.Edge), maxf(240, safe.W*0.74))
			menuX := safe.Right() - tokens.Edge - menuW
			menuY := l.Menu.Bottom() + tokens.Gap
			menuH := minf(rowH*float32(menuRows), maxf(0, safe.Bottom()-tokens.Edge-menuY))
			l.MenuPanel = Rect{menuX, menuY, menuW, menuH}
		} else {
			l.MenuPanel = Rect{l.Menu.Right() - menuW, l.Menu.Bottom() + tokens.Gap, menuW, rowH * float32(menuRows)}
		}
		screens := []Screen{ScreenCharacter, ScreenProfile, ScreenInventory, ScreenSkills, ScreenMap, ScreenSocial, ScreenSettings}
		for i, screen := range screens {
			l.MenuActions = append(l.MenuActions, MenuAction{Screen: screen, Rect: Rect{l.MenuPanel.X, l.MenuPanel.Y + float32(i)*rowH, menuW, rowH}})
		}
	}
	return l
}

type ControlID uint8

const (
	ControlNone ControlID = iota
	ControlMenu
	ControlSkill
	ControlMinimap
	ControlTarget
	ControlStatus
	ControlMenuAction
	ControlChat
	ControlCancelAction
	ControlLootItem
	ControlSkillPagePrev
	ControlSkillPageNext
)

type Hit struct {
	Control    ControlID
	SkillIndex int
	LootIndex  int
	Screen     Screen
}

func (l HUDLayout) HitTest(x, y float32) Hit {
	if l.CombatCancel.Contains(x, y) {
		return Hit{Control: ControlCancelAction}
	}
	if l.SkillPagePrev.Contains(x, y) {
		return Hit{Control: ControlSkillPagePrev}
	}
	if l.SkillPageNext.Contains(x, y) {
		return Hit{Control: ControlSkillPageNext}
	}
	for _, action := range l.MenuActions {
		if action.Rect.Contains(x, y) {
			return Hit{Control: ControlMenuAction, Screen: action.Screen}
		}
	}
	if l.Menu.Contains(x, y) || l.MenuPanel.Contains(x, y) {
		return Hit{Control: ControlMenu}
	}
	for i, rect := range l.SkillSlots {
		if rect.Contains(x, y) {
			return Hit{Control: ControlSkill, SkillIndex: i}
		}
	}
	if l.Minimap.Contains(x, y) {
		return Hit{Control: ControlMinimap}
	}
	if l.TargetPanel.Contains(x, y) {
		return Hit{Control: ControlTarget}
	}
	for i, rect := range l.LootRows {
		if rect.Contains(x, y) {
			return Hit{Control: ControlLootItem, LootIndex: i}
		}
	}
	if l.StatusArea.Contains(x, y) {
		return Hit{Control: ControlStatus}
	}
	if l.ChatBar.Contains(x, y) {
		return Hit{Control: ControlChat}
	}
	return Hit{}
}

func (l HUDLayout) ConsumeTouch(point input.TouchPoint) bool {
	return l.HitTest(float32(point.X), float32(point.Y)).Control != ControlNone
}
func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// visibleSkillCap is the number of hotbar slots shown at once, before paging.
// It is the single source of truth for page boundaries; interaction.go and the
// Android host read it back from HUDLayout.SkillsPerPage so they agree.
func visibleSkillCap(safe Rect, portrait bool) int {
	if !portrait {
		return 4
	}
	if safe.W < 600 {
		return 2
	}
	// Tall portrait (roughly 18:10 or taller, e.g. 1080x2340) has room for a
	// wider action cluster closer to the target renders.
	if safe.H >= safe.W*1.8 {
		return 6
	}
	return 4
}
