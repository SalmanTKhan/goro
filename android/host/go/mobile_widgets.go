//go:build android && cgo

package main

import (
	"fmt"

	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/render"
	uimobile "github.com/kivutar/goro/ui/mobile"
	"github.com/kivutar/goro/ui/rotheme"
)

// mobileWidgets renders the mobile presentation through the shared ui/mobile
// widget layer instead of the host's hand-coded draw calls.
//
// It rasterizes the screen's widget tree once per change and blits the result,
// then layers real item and skill sprites on top from the resource path — the
// same split the preview uses, so what CI renders is what the device shows.
type mobileWidgets struct {
	ui    *uiapp.App
	win   *androidUIWindow
	kit   uimobile.Kit
	baked *render.Image
	w, h  int
	key   string
	dirty bool
	// logged keeps the one-shot diagnostic from repeating every frame.
	logged bool
}

func newMobileWidgets() *mobileWidgets {
	theme := rotheme.Mobile.AsTheme()
	// Transparent ground: the mobile screens float over the live world.
	theme.Colors.Background = widget.RGBA8(0, 0, 0, 0)
	// The app needs a window provider or its window has no size and DrawTo
	// renders nothing, which is exactly how this path first came out black.
	win := &androidUIWindow{width: 1, height: 1}
	return &mobileWidgets{
		ui: uiapp.New(
			uiapp.WithWindowProvider(win),
			uiapp.WithTheme(theme),
			uiapp.WithRenderMode(uiapp.RenderModeHostManaged),
		),
		win:   win,
		kit:   uimobile.NewKit(rotheme.Mobile),
		dirty: true,
	}
}

func (m *mobileWidgets) Invalidate() {
	if m != nil {
		m.dirty = true
	}
}

// sprites is the art a screen wants composited over its raster.
type sprites struct {
	items  []uimobile.IconPlacement
	skills []uimobile.SkillIconPlacement
	// preview is the character sprite's frame on the profile screen. It draws
	// through a different call than item art, so it is reported separately.
	preview      mobileui.Rect
	previewOf    mobileui.MobileProfileModel
	previewValid bool
}

// tree returns the widget tree for the screen the presentation is showing,
// mirroring the dispatch order in Draw. A nil tree means this screen has no
// widget builder yet and the legacy drawing should run instead.
func (p *mobilePresentation) tree(k uimobile.Kit) (widget.Widget, sprites) {
	vp := p.viewport
	switch {
	case p.startup != nil && p.startup.Phase == mobileui.StartupTitle:
		return k.StartupTree(p.startup.Model, mobileui.LayoutStartup(vp)), sprites{}

	case p.tradeController != nil && p.tradeController.IsOpen():
		c := p.tradeController
		return k.TradeTree(c.Model, c.Layout, c.State),
			sprites{items: uimobile.TradeIconRects(c.Model, c.Layout)}

	case p.vendingController != nil && p.vendingController.IsOpen():
		c := p.vendingController
		return k.VendingTree(c.Model, c.Layout, vendingSelection(c.State), c.Quantity),
			sprites{items: uimobile.VendingIconRects(c.Model, c.Layout)}

	// The profile screen is deliberately absent: character selection and
	// creation run the desktop windows through characterWindows (see
	// mobile_character.go), which Draw consults before this dispatch.

	case p.chatController != nil && p.chatController.Model.Open:
		c := p.chatController
		return k.ChatTree(c.Model, c.Layout, c.Draft), sprites{}

	case p.dialogController != nil && p.dialogController.Model.Open:
		c := p.dialogController
		return k.DialogTree(c.Model, c.Layout), sprites{}

	case p.economyController != nil && p.economyController.Screen == mobileui.EconomyShop:
		c := p.economyController
		items := c.Shop.Items
		if c.Tab == mobileui.ShopSellTab {
			items = c.Shop.SellItems
		}
		return k.ShopTree(c.Shop, c.Layout, c.Tab, c.Quantity),
			sprites{items: uimobile.ShopIconRects(items, c.Layout)}

	case p.economyController != nil && p.economyController.Screen == mobileui.EconomyStorage:
		c := p.economyController
		return k.StorageTree(c.Storage, c.Layout, c.Quantity),
			sprites{items: uimobile.StorageIconRects(c.Storage, c.Layout)}

	case p.characterSkills != nil && p.navigation.Screen == mobileui.ScreenCharacter:
		c := p.characterSkills
		return k.CharacterTree(c.Character, c.CharacterLayout), sprites{}

	case p.characterSkills != nil && p.navigation.Screen == mobileui.ScreenSkills:
		c := p.characterSkills
		return k.SkillsTree(c.Skills, c.SkillsLayout),
			sprites{skills: uimobile.SkillIconRects(c.Skills, c.SkillsLayout)}

	case p.mapController != nil && p.navigation.Screen == mobileui.ScreenMap:
		c := p.mapController
		return k.MapTree(c.Model, c.Layout, c.State), sprites{}

	case p.socialController != nil && p.navigation.Screen == mobileui.ScreenSocial && p.socialController.IsOpen():
		c := p.socialController
		return k.SocialTree(c.Model, c.Layout, c.State), sprites{}

	case p.settingsController != nil && p.navigation.Screen == mobileui.ScreenSettings:
		c := p.settingsController
		return k.SurfaceTree(c.Model, c.Layout, c.State), sprites{}

	case p.inventory != nil && p.inventory.State.Screen == mobileui.ScreenInventory:
		c := p.inventory
		return k.InventoryTree(c.Model, c.Layout, c.State),
			sprites{items: uimobile.IconRects(c.Model, c.Layout, c.State)}

	case p.inventory != nil && p.inventory.State.Screen == mobileui.ScreenEquipment:
		c := p.inventory
		return k.EquipmentTree(c.Equipment, c.Layout, c.State),
			sprites{items: uimobile.EquipmentIconRects(c.Equipment, c.Layout)}

	case p.inventory == nil || p.inventory.State.Screen == mobileui.ScreenWorldHUD:
		skills, loot := uimobile.HUDIconRects(p.hudModel, p.hud)
		return k.HUDTree(p.hudModel, p.hud, p.navigation),
			sprites{items: loot, skills: skills}
	}
	return nil, sprites{}
}

// widgetStateKey covers presentation-only state that is not part of the game
// snapshot (navigation and controller visibility). Refresh invalidates the
// raster when the snapshot changes; this key handles screen transitions and
// menu/dialog toggles between snapshots.
func (p *mobilePresentation) widgetStateKey() string {
	if p == nil {
		return ""
	}
	phase := mobileui.StartupWorld
	if p.startup != nil {
		phase = p.startup.Phase
	}
	trade, vending, profile, chat, dialog := false, false, false, false, false
	if p.tradeController != nil {
		trade = p.tradeController.IsOpen()
	}
	if p.vendingController != nil {
		vending = p.vendingController.IsOpen()
	}
	if p.profileController != nil {
		profile = p.profileController.Open
	}
	if p.chatController != nil {
		chat = p.chatController.Model.Open
	}
	if p.dialogController != nil {
		dialog = p.dialogController.Model.Open
	}
	return fmt.Sprintf("%d|%#v|%t|%t|%t|%t|%t|%d|%d", phase, p.navigation, trade, vending, profile, chat, dialog, func() mobileui.EconomyScreen {
		if p.economyController != nil {
			return p.economyController.Screen
		}
		return mobileui.EconomyClosed
	}(), func() mobileui.Screen {
		if p.inventory != nil {
			return p.inventory.State.Screen
		}
		return mobileui.ScreenWorldHUD
	}())
}

// drawWidgets renders the current screen through the widget layer. It reports
// false when the screen has no builder, so the caller can fall back.
func (m *mobileWidgets) drawWidgets(p *mobilePresentation, frame *render.Frame) bool {
	if m == nil || m.ui == nil || frame == nil {
		return false
	}
	tree, art := p.tree(m.kit)
	if tree == nil {
		return false
	}
	w, h := int(p.viewport.Width), int(p.viewport.Height)
	if w <= 0 || h <= 0 {
		return false
	}

	m.win.width, m.win.height = w, h
	key := p.widgetStateKey()
	if key != m.key || m.w != w || m.h != h {
		m.dirty = true
	}
	drawn := false
	if m.dirty || m.baked == nil {
		m.ui.SetRoot(tree)
		// SetRoot only replaces the retained tree. Frame performs the framework
		// layout/update pass that gives the absolute Canvas and its children
		// bounds; without it DrawTo reports success but the tree contributes no
		// pixels.
		m.ui.Frame()
		image, rasterDrawn, err := render.RasterizeUI(m.ui, w, h, m.baked)
		if err != nil {
			androidLog(fmt.Sprintf("stage=mobile-widgets raster-error=%v", err))
			return false
		}
		if rasterDrawn {
			m.baked, m.w, m.h = image, w, h
			drawn = true
		}
		m.key = key
		m.dirty = false
	}
	if !m.logged {
		androidLog(fmt.Sprintf("stage=mobile-widgets size=%dx%d drawn=%t baked=%t", w, h, drawn, m.baked != nil))
		m.logged = true
	}
	if m.baked == nil {
		return false
	}
	var opts render.DrawImageOptions
	opts.Filter = render.FilterNearest
	frame.DrawImage(m.baked, &opts)
	if p.widgetHUDActive() && p.settings.Display.ShowMinimap && p.hud.Minimap.W > 0 {
		mapRect := mobileui.Rect{X: p.hud.Minimap.X + 10, Y: p.hud.Minimap.Y + 34, W: p.hud.Minimap.W - 20, H: p.hud.Minimap.H - 70}
		render.DrawRect(frame, float64(mapRect.X), float64(mapRect.Y), float64(mapRect.W), float64(mapRect.H), mobileColors().mapBackground)
		p.drawMinimapTerrain(frame, mapRect)
	}

	// Real art on top, from the authoritative resource path.
	for _, placement := range art.items {
		side := int(min32(placement.Rect.W, placement.Rect.H))
		if side < 6 {
			continue
		}
		p.game.DrawMobileInventoryItemIcon(frame, placement.Item, int(placement.Rect.X), int(placement.Rect.Y), side)
	}
	if art.previewValid {
		p.game.DrawMobileProfilePreview(frame, art.previewOf,
			int(art.preview.X), int(art.preview.Y), int(art.preview.W), int(art.preview.H))
	}
	for _, placement := range art.skills {
		side := int(min32(placement.Rect.W, placement.Rect.H))
		if side < 6 {
			continue
		}
		p.game.DrawMobileSkillIcon(frame, placement.Skill, int(placement.Rect.X), int(placement.Rect.Y), side)
	}
	return true
}

func (p *mobilePresentation) widgetHUDActive() bool {
	if p == nil || p.navigation.Screen != mobileui.ScreenWorldHUD {
		return false
	}
	if p.startup != nil && p.startup.Phase == mobileui.StartupTitle {
		return false
	}
	if p.game != nil && p.game.Online() && !p.game.SessionPlaying() {
		return false
	}
	if p.tradeController != nil && p.tradeController.IsOpen() ||
		p.vendingController != nil && p.vendingController.IsOpen() ||
		p.profileController != nil && p.profileController.Open ||
		p.chatController != nil && p.chatController.Model.Open ||
		p.dialogController != nil && p.dialogController.Model.Open ||
		p.economyController != nil && p.economyController.Screen != mobileui.EconomyClosed {
		return false
	}
	return p.inventory == nil || p.inventory.State.Screen == mobileui.ScreenWorldHUD
}

// vendingSelection converts the controller's selection into the index the
// renderer expects, using -1 for "nothing selected".
func vendingSelection(state mobileui.VendingSelectionState) int {
	if !state.HasSelection {
		return -1
	}
	return state.SelectedIndex
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
