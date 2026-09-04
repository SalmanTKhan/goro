package game

import (
	"image"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/render"
	"github.com/kivutar/goro/session"
)

type Mode interface {
	Name() string
	Enter(client.Context)
	Update(client.Context) (Mode, error)
	Draw(client.Context, *render.Frame)
}

type overlayMode interface {
	DrawOverlay(client.Context, *render.Frame)
}

type uiOverlayMode interface {
	DrawUIOverlay(client.Context, *render.Frame)
}

type frameSubmittedMode interface {
	FrameSubmitted()
}

type Manager struct {
	ctx  client.Context
	mode Mode
}

func NewManager(ctx client.Context, mode Mode) *Manager {
	m := &Manager{ctx: ctx, mode: mode}
	if m.mode != nil {
		m.mode.Enter(ctx)
	}
	return m
}

func (m *Manager) UpdateContext(ctx client.Context) {
	m.ctx = ctx
}

func (m *Manager) Update() error {
	if m.mode == nil {
		return nil
	}

	next, err := m.mode.Update(m.ctx)
	if err != nil {
		return err
	}
	if next != nil {
		m.mode = next
		m.mode.Enter(m.ctx)
	}
	return nil
}

func (m *Manager) Draw(screen *render.Frame) {
	if m.mode != nil {
		m.mode.Draw(m.ctx, screen)
	}
}

func (m *Manager) DrawOverlay(screen *render.Frame) {
	if mode, ok := m.mode.(overlayMode); ok {
		mode.DrawOverlay(m.ctx, screen)
	}
}

func (m *Manager) DrawUIOverlay(screen *render.Frame) {
	if mode, ok := m.mode.(uiOverlayMode); ok {
		mode.DrawUIOverlay(m.ctx, screen)
	}
}

func (m *Manager) FrameSubmitted() {
	if mode, ok := m.mode.(frameSubmittedMode); ok {
		mode.FrameSubmitted()
	}
}

func (m *Manager) ApplyPlayerCommand(ctx client.Context, command input.PlayerCommand) bool {
	if command.Kind == input.CommandOnlineDisconnect {
		if ctx.Network == nil {
			return false
		}
		// Keep the clean-disconnect packet on the shared network path, then
		// return to character selection without creating an offline authority.
		_ = ctx.Network.SendQuitGameAndClose()
		if world, ok := m.mode.(*WorldMode); ok {
			next := world.nextCharacterSelectMode(ctx)
			// Disconnect is an explicit stop action. Do not immediately select
			// the configured slot again just because the initial mobile login
			// used AutoLogin to cross the credential-less front door.
			next.autoCharAttempted = true
			m.mode = next
			m.mode.Enter(ctx)
		} else {
			ctx.Network.Close()
			if ctx.Session != nil {
				ctx.Session.Playing = false
			}
		}
		return true
	}
	if mode, ok := m.mode.(interface {
		ApplyPlayerCommand(client.Context, input.PlayerCommand) bool
	}); ok {
		return mode.ApplyPlayerCommand(ctx, command)
	}
	return false
}

func (m *Manager) MobileLoginModel(ctx client.Context) mobileui.MobileOnlineLoginModel {
	if m == nil {
		return mobileui.MobileOnlineLoginModel{}
	}
	if mode, ok := m.mode.(*LoginMode); ok {
		return mode.MobileLoginModel(ctx)
	}
	return mobileui.MobileOnlineLoginModel{}
}

func (m *Manager) MobileDialogModel() mobileui.MobileDialogModel {
	if m == nil {
		return mobileui.MobileDialogModel{}
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		return mode.ui.npcDialog.MobileModel(m.ctx)
	}
	return mobileui.MobileDialogModel{}
}

func (m *Manager) PickMobileTarget(ctx client.Context, position input.WorldPosition) (input.PickedTarget, bool) {
	if mode, ok := m.mode.(interface {
		PickMobileTarget(client.Context, input.WorldPosition) (input.PickedTarget, bool)
	}); ok {
		return mode.PickMobileTarget(ctx, position)
	}
	return input.PickedTarget{}, false
}

func (m *Manager) InspectMobileTarget(ctx client.Context, actorID uint32) (mobileui.TargetHUDModel, bool) {
	if m == nil {
		return mobileui.TargetHUDModel{}, false
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		return mode.InspectMobileTarget(ctx, actorID)
	}
	return mobileui.TargetHUDModel{}, false
}

func (m *Manager) DrawMobileTileCursor(screen *render.Frame, position input.WorldPosition) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawMobileTileCursor(m.ctx, screen, position)
	}
}

func (m *Manager) DrawInventoryItemIcon(screen *render.Frame, itemID uint16, identified bool, x, y, size int) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawInventoryItemIconSized(screen, m.ctx.Resources, session.InventoryItem{ItemID: itemID, Identified: identified}, x, y, size)
	}
}

func (m *Manager) DrawMobileInventoryItemIcon(screen *render.Frame, itemID uint16, identified bool, x, y, size int) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawMobileInventoryItemIcon(screen, m.ctx.Resources, session.InventoryItem{ItemID: itemID, Identified: identified}, x, y, size)
	}
}

func (m *Manager) DrawSkillIcon(screen *render.Frame, skillID uint16, x, y, size int) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawSkillIcon(screen, m.ctx.Resources, session.Skill{ID: skillID}, x, y, size)
	}
}

func (m *Manager) DrawMobileSkillIcon(screen *render.Frame, skillID uint16, x, y, size int) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawMobileSkillIcon(screen, m.ctx.Resources, session.Skill{ID: skillID}, x, y, size)
	}
}

func (m *Manager) DrawEquipmentPreview(screen *render.Frame, x, y, width, height int) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawEquipmentPreview(screen, m.ctx, x, y, width, height)
	}
}

func (m *Manager) DrawMobileEquipmentPreview(screen *render.Frame, x, y, width, height int) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawMobileEquipmentPreview(screen, m.ctx, x, y, width, height)
	}
}

func (m *Manager) DrawMobileProfilePreview(screen *render.Frame, character session.Character, sex byte, x, y, width, height int) {
	if m == nil {
		return
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		mode.DrawMobileProfilePreview(screen, m.ctx.Resources, character, sex, x, y, width, height)
	}
}

func (m *Manager) MobileProfilePreviewImage(character session.Character, sex byte, width, height int) image.Image {
	if m == nil {
		return nil
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		return mode.MobileProfilePreviewImage(m.ctx.Resources, character, sex, width, height)
	}
	return nil
}

func (m *Manager) MobileChatModel() mobileui.MobileChatModel {
	if m == nil {
		return mobileui.MobileChatModel{}
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		return mode.MobileChatModel()
	}
	return mobileui.MobileChatModel{Channel: "WORLD"}
}

func (m *Manager) MobileTradeModel() mobileui.MobileTradeModel {
	if m == nil {
		return mobileui.MobileTradeModel{}
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		return mode.MobileTradeModel(m.ctx)
	}
	return mobileui.MobileTradeModel{}
}

func (m *Manager) MobileShopModel(ctx client.Context) mobileui.MobileShopModel {
	if m == nil {
		return mobileui.MobileShopModel{}
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		return mode.MobileShopModel(ctx)
	}
	return mobileui.MobileShopModel{}
}

func (m *Manager) MobileVendingModel() mobileui.MobileVendingModel {
	if m == nil {
		return mobileui.MobileVendingModel{}
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		return mode.MobileVendingModel(m.ctx)
	}
	return mobileui.MobileVendingModel{}
}

func (m *Manager) RenderMetrics() RenderMetrics {
	if m == nil {
		return RenderMetrics{}
	}
	if mode, ok := m.mode.(*WorldMode); ok {
		metrics := mode.RenderMetrics()
		metrics.TerrainTextureFallbacks = mode.terrainTextureFallbacks
		metrics.RSMTextureFallbacks = mode.rsmTextureFallbacks
		metrics.RSMEmptyTextureFallbacks = mode.rsmEmptyTextureFallbacks
		metrics.RSMTextureFallbackExamples = append([]string(nil), mode.rsmTextureFallbackExamples...)
		return metrics
	}
	return RenderMetrics{}
}

func (m *Manager) LoginStatus() string {
	if m == nil {
		return "waiting for login"
	}
	if mode, ok := m.mode.(*LoginMode); ok {
		return mode.Status()
	}
	return "world"
}
