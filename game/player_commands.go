package game

import (
	"math"
	"strings"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/network"
)

// ApplyPlayerCommand is the narrow gameplay consumer for the platform-neutral
// input command boundary. Existing helpers remain authoritative for network,
// combat, and skill rules.
func (m *WorldMode) ApplyPlayerCommand(ctx client.Context, command input.PlayerCommand) bool {
	switch command.Kind {
	case input.CommandMoveDirection:
		return m.moveController(ctx, command.Direction)
	case input.CommandTargetPrevious:
		return m.cycleControllerTarget(ctx, true)
	case input.CommandTargetNext:
		return m.cycleControllerTarget(ctx, false)
	case input.CommandAttackFocused:
		return m.controllerAttackFocused(ctx)
	case input.CommandInteractFocused:
		return m.controllerInteractFocused(ctx)
	case input.CommandLootFocused:
		return m.controllerLootFocused(ctx)
	case input.CommandUseShortcut:
		return m.ui.shortcutBar.ActivateSlot(ctx, m, int(command.Slot))
	case input.CommandMoveTo:
		return m.requestWalk(ctx, int(math.Round(command.Position.X)), int(math.Round(command.Position.Y)), "player command")
	case input.CommandAttackActor:
		if ctx.World == nil {
			return false
		}
		actor, ok := ctx.World.Actors[command.ActorID]
		if !ok {
			return false
		}
		if ctx.Offline != nil {
			ctx.Offline.TargetID, ctx.Offline.TargetName = actor.ID, actor.Name
			if name, hp, maxHP, npc, ok := ctx.Offline.TargetForActor(actor.ID); ok {
				ctx.Offline.TargetName, ctx.Offline.TargetHP, ctx.Offline.TargetMaxHP, ctx.Offline.TargetNPC = name, hp, maxHP, npc
			} else {
				ctx.Offline.TargetHP, ctx.Offline.TargetMaxHP = 0, 0
				ctx.Offline.TargetNPC = false
			}
			if actor.Job == 45 || actor.Job == 128 || actor.Job == 129 {
				return false
			}
			return ctx.Offline.HandleCommand(command)
		}
		m.requestAttack(ctx, actor, "player command")
		return true
	case input.CommandInteractActor:
		if ctx.World == nil {
			return false
		}
		actor, ok := ctx.World.Actors[command.ActorID]
		if !ok {
			return false
		}
		if ctx.Offline != nil {
			ctx.Offline.TargetID, ctx.Offline.TargetName = actor.ID, actor.Name
			if name, hp, maxHP, npc, ok := ctx.Offline.TargetForActor(actor.ID); ok {
				ctx.Offline.TargetName, ctx.Offline.TargetHP, ctx.Offline.TargetMaxHP, ctx.Offline.TargetNPC = name, hp, maxHP, npc
			}
		}
		m.requestNPCTalk(ctx, actor, "player command")
		return true
	case input.CommandPickUpItem:
		if ctx.Offline != nil {
			if ctx.World == nil {
				return false
			}
			item, ok := ctx.World.Items[command.ItemID]
			if !ok {
				return false
			}
			return m.requestPickup(ctx, item, "player command")
		}
		if ctx.World == nil {
			return false
		}
		item, ok := ctx.World.Items[command.ItemID]
		if !ok {
			return false
		}
		return m.requestPickup(ctx, item, "player command")
	case input.CommandRotateCamera:
		if cameraRotationLockedForMap(ctx) {
			return false
		}
		m.camera.Rotate(command.DeltaX)
		if command.DeltaY != 0 {
			m.camera.Tilt(command.DeltaY * defaultCameraPitchDragPerPixel)
		}
		return true
	case input.CommandZoomCamera:
		if cameraZoomLockedForMap(ctx) {
			return false
		}
		m.camera.ZoomByDelta(command.DeltaY)
		return true
	case input.CommandResetCamera:
		m.camera.ResetToDefaultOrientation()
		return true
	case input.CommandCancelAction:
		m.cancelControllerAction(ctx)
		if ctx.Offline != nil {
			return ctx.Offline.HandleCommand(command)
		}
		m.skills().Cancel("player command")
		return true
	case input.CommandInspectActor:
		// Inspection is presentation state, but validating the actor here keeps
		// mobile input from displaying stale or fabricated targets.
		_, ok := m.InspectMobileTarget(ctx, command.ActorID)
		return ok
	case input.CommandUseItem:
		if ctx.Offline != nil {
			return ctx.Offline.HandleCommand(command)
		}
		return m.useItemCommand(ctx, command)
	case input.CommandEquipItem:
		if command.ItemIndex == 0 {
			return false
		}
		if ctx.Offline != nil {
			return ctx.Offline.HandleCommand(command)
		}
		if ctx.Network == nil && ctx.Offline != nil {
			return mutateOfflineEquip(ctx, command.ItemIndex, command.EquipmentSlot, true)
		}
		if ctx.Network == nil {
			return false
		}
		return ctx.Network.SendWearEquip(command.ItemIndex, command.EquipmentSlot) == nil
	case input.CommandUnequipItem:
		if command.ItemIndex == 0 {
			return false
		}
		if ctx.Offline != nil {
			return ctx.Offline.HandleCommand(command)
		}
		if ctx.Network == nil && ctx.Offline != nil {
			return mutateOfflineEquip(ctx, command.ItemIndex, 0, false)
		}
		if ctx.Network == nil {
			return false
		}
		return ctx.Network.SendTakeoffEquip(command.ItemIndex) == nil
	case input.CommandDropItem:
		if command.ItemIndex == 0 || command.Quantity <= 0 {
			return false
		}
		if ctx.Offline != nil {
			return ctx.Offline.HandleCommand(command)
		}
		if ctx.Network == nil && ctx.Offline != nil {
			return mutateOfflineDrop(ctx, command.ItemIndex, command.Quantity)
		}
		if ctx.Network == nil {
			return false
		}
		return ctx.Network.SendDropInventoryItem(command.ItemIndex, uint16(command.Quantity)) == nil
	case input.CommandOpenVending:
		if ctx.Network == nil || ctx.World == nil || command.ActorID == 0 {
			return false
		}
		actor, ok := ctx.World.Actors[command.ActorID]
		if !ok || !actorHasVending(actor) {
			return false
		}
		m.requestVendingList(ctx, actor, "mobile command")
		return m.mobileVending.open && m.mobileVending.ownerAID == actor.ID
	case input.CommandBuyVendingItem:
		return m.mobileVendingPurchase(ctx, command.ItemIndex, command.Quantity)
	case input.CommandSaveOfflineProfile:
		return m.applyOfflineProfile(ctx, command)
	case input.CommandSendGlobalChat:
		if ctx.Network == nil || ctx.Session == nil || strings.TrimSpace(command.Text) == "" {
			return false
		}
		name := ctx.Session.SelectedCharacter().Name
		if strings.TrimSpace(name) == "" {
			return false
		}
		return ctx.Network.SendGlobalChat(name, strings.TrimSpace(command.Text)) == nil
	case input.CommandSendWhisper:
		if ctx.Network == nil || strings.TrimSpace(command.TargetName) == "" || strings.TrimSpace(command.Text) == "" {
			return false
		}
		return ctx.Network.SendWhisper(strings.TrimSpace(command.TargetName), strings.TrimSpace(command.Text)) == nil
	case input.CommandDeleteFriend:
		if ctx.Network == nil || command.TargetAccountID == 0 || command.TargetCharID == 0 {
			return false
		}
		return ctx.Network.SendDeleteFriend(command.TargetAccountID, command.TargetCharID) == nil
	case input.CommandCreateParty:
		if ctx.Network == nil || ctx.Session == nil || ctx.Session.Party.Active() || strings.TrimSpace(command.Text) == "" {
			return false
		}
		return ctx.Network.SendMakeParty2(strings.TrimSpace(command.Text), 0, 0) == nil
	case input.CommandInviteParty:
		if ctx.Network == nil || ctx.Session == nil || !partyCanManage(ctx.Session) || strings.TrimSpace(command.Text) == "" {
			return false
		}
		return ctx.Network.SendPartyInvite(0, strings.TrimSpace(command.Text)) == nil
	case input.CommandLeaveParty:
		if ctx.Network == nil || ctx.Session == nil || !ctx.Session.Party.Active() {
			return false
		}
		return ctx.Network.SendLeaveParty() == nil
	case input.CommandExpelPartyMember:
		if ctx.Network == nil || ctx.Session == nil || !partyCanManage(ctx.Session) || command.ActorID == 0 || strings.TrimSpace(command.Text) == "" {
			return false
		}
		return ctx.Network.SendExpelPartyMember(command.ActorID, strings.TrimSpace(command.Text)) == nil
	case input.CommandRespondFriendRequest:
		if ctx.Session == nil || ctx.Network == nil || command.TargetAccountID == 0 || command.TargetCharID == 0 {
			return false
		}
		request := ctx.Session.PendingFriendRequest
		if request == nil || request.AccountID != command.TargetAccountID || request.CharID != command.TargetCharID {
			return false
		}
		return m.respondFriendRequest(ctx, network.FriendRequest{
			AccountID: request.AccountID,
			CharID:    request.CharID,
			Name:      request.Name,
		}, command.Accepted)
	case input.CommandRespondPartyInvite:
		if ctx.Session == nil || ctx.Network == nil || command.RequestID == 0 {
			return false
		}
		invite := ctx.Session.PendingPartyInvite
		if invite == nil || invite.RequestID != command.RequestID {
			return false
		}
		return m.respondPartyInvite(ctx, network.PartyInviteRequest{
			RequestID: invite.RequestID,
			Name:      invite.Name,
		}, command.Accepted)
	case input.CommandSetPartySettings:
		if ctx.Session == nil || ctx.Network == nil || !partyCanManage(ctx.Session) || !ctx.Session.Party.Active() {
			return false
		}
		if err := ctx.Network.SendPartyOption(command.ExpShare); err != nil {
			return false
		}
		if err := ctx.Network.SendPartyInviteConfig(command.RefuseInvites); err != nil {
			return false
		}
		ctx.Session.Party.ExpShare = command.ExpShare
		ctx.Session.Party.RefuseInvites = command.RefuseInvites
		return true
	case input.CommandOpenTrade, input.CommandRespondTradeRequest,
		input.CommandTradeAddItem, input.CommandTradeAddZeny,
		input.CommandTradeConclude, input.CommandTradeCommit, input.CommandTradeCancel:
		return m.applyMobileTradeCommand(ctx, command)
	case input.CommandNPCNext, input.CommandNPCMenuChoice, input.CommandNPCClose:
		return m.ui.npcDialog.ApplyMobileCommand(ctx, command)
	case input.CommandCloseShop:
		if ctx.Network == nil || command.NPCID == 0 {
			return false
		}
		return ctx.Network.SendNPCClose(command.NPCID) == nil
	case input.CommandCloseStorage:
		if ctx.Network == nil || ctx.Session == nil || !ctx.Session.Storage.Open {
			return false
		}
		return ctx.Network.SendCloseStorage() == nil
	case input.CommandUseSkill, input.CommandUseSkillOnActor, input.CommandUseSkillAtPosition,
		input.CommandOpenShop, input.CommandBuyItem, input.CommandSellItem,
		input.CommandOpenStorage, input.CommandDepositItem, input.CommandWithdrawItem:
		if ctx.Offline != nil {
			return ctx.Offline.HandleCommand(command)
		}
		if command.Kind == input.CommandUseSkill || command.Kind == input.CommandUseSkillOnActor || command.Kind == input.CommandUseSkillAtPosition {
			return m.mobileSkill(ctx, command)
		}
		if command.Kind == input.CommandOpenShop && ctx.Network != nil && command.NPCID != 0 {
			return ctx.Network.SendShopDealSelection(command.NPCID, command.Tab) == nil
		}
		if command.Kind == input.CommandOpenStorage {
			return ctx.Session != nil && ctx.Session.Storage.Open
		}
		if command.Kind == input.CommandBuyItem && command.Quantity > 0 {
			return m.mobileShopBuy(ctx, command.ItemIndex, uint16(command.Quantity))
		}
		if command.Kind == input.CommandSellItem && command.Quantity > 0 {
			return m.mobileShopSell(ctx, command.ItemIndex, uint16(command.Quantity))
		}
		if command.Kind == input.CommandDepositItem && command.Quantity > 0 {
			return m.mobileStorageDeposit(ctx, command.ItemIndex, uint32(command.Quantity))
		}
		if command.Kind == input.CommandWithdrawItem && command.Quantity > 0 {
			return m.mobileStorageWithdraw(ctx, command.ItemIndex, uint32(command.Quantity))
		}
		return false
	default:
		return false
	}
}

func (m *WorldMode) useItemCommand(ctx client.Context, command input.PlayerCommand) bool {
	if ctx.Session == nil || command.ItemIndex == 0 {
		return false
	}
	if ctx.Session.Dead {
		return false
	}
	var itemType uint8
	found := false
	for _, item := range ctx.Session.Inventory.Items {
		if item.Index == command.ItemIndex {
			itemType, found = item.Type, true
			break
		}
	}
	if !found || !db.ItemTypeIsUsable(itemType) {
		return false
	}
	if ctx.Network == nil {
		return false
	}
	target := ctx.Session.AccountID
	if target == 0 {
		target = ctx.Session.CharID
	}
	if target == 0 {
		return false
	}
	return ctx.Network.SendUseInventoryItem(command.ItemIndex, target) == nil
}

func mutateOfflineEquip(ctx client.Context, index, slot uint16, equipped bool) bool {
	if ctx.Session == nil {
		return false
	}
	for i := range ctx.Session.Inventory.Items {
		item := &ctx.Session.Inventory.Items[i]
		if item.Index != index {
			continue
		}
		item.Equipped = equipped
		item.Equip = equipped || !db.ItemTypeIsUsable(item.Type)
		if equipped && slot != 0 {
			item.Location = slot
		}
		return true
	}
	return false
}

func mutateOfflineDrop(ctx client.Context, index uint16, quantity int) bool {
	if ctx.Session == nil {
		return false
	}
	for i := range ctx.Session.Inventory.Items {
		item := &ctx.Session.Inventory.Items[i]
		if item.Index != index || item.Amount <= 0 {
			continue
		}
		if quantity >= item.Amount {
			ctx.Session.Inventory.Items = append(ctx.Session.Inventory.Items[:i], ctx.Session.Inventory.Items[i+1:]...)
		} else {
			item.Amount -= quantity
		}
		return true
	}
	return false
}
