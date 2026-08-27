package game

import (
	"time"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
)

// PickMobileTarget reuses the production scene projection and hit testing for
// touch input. The Android host supplies only surface coordinates; no mobile
// renderer or alternate world math is introduced here.
func (m *WorldMode) PickMobileTarget(ctx client.Context, position input.WorldPosition) (input.PickedTarget, bool) {
	if m == nil || ctx.World == nil {
		return input.PickedTarget{}, false
	}
	width, height := ctx.ScreenSize()
	projection := m.sceneProjection(ctx, width, height, time.Now())
	mx, my := int(position.X), int(position.Y)
	now := time.Now()
	if actor, ok := m.hoveredVendingBoard(ctx, projection, mx, my, now); ok {
		return input.PickedTarget{Kind: input.TargetVending, ActorID: actor.ID, Position: position}, true
	}
	if actor, ok := hoveredCursorActor(ctx, projection, mx, my, now, m.actorDeaths); ok && isWarpActor(actor) {
		return input.PickedTarget{Kind: input.TargetGround, Position: input.WorldPosition{X: float64(actor.X), Y: float64(actor.Y)}}, true
	}
	if actor, ok := clickedTalkTarget(ctx, projection, mx, my, now, m.actorDeaths); ok {
		return input.PickedTarget{Kind: input.TargetNPC, ActorID: actor.ID, Position: position}, true
	}
	if actor, ok := clickedAttackTarget(ctx, projection, mx, my, now, m.actorDeaths); ok {
		return input.PickedTarget{Kind: input.TargetActor, ActorID: actor.ID, Hostile: true, Position: position}, true
	}
	if item, ok := clickedGroundItem(ctx, projection, mx, my, now); ok {
		return input.PickedTarget{Kind: input.TargetItem, ItemID: item.ID, Position: position}, true
	}
	if x, y, ok := clickedWalkTarget(ctx, projection, mx, my); ok {
		return input.PickedTarget{Kind: input.TargetGround, Position: input.WorldPosition{X: float64(x), Y: float64(y)}}, true
	}
	return input.PickedTarget{}, false
}

// InspectMobileTarget resolves the same display-name and life-bar data used
// by the desktop hover label, but returns a renderer-neutral mobile model.
func (m *WorldMode) InspectMobileTarget(ctx client.Context, actorID uint32) (mobileui.TargetHUDModel, bool) {
	if m == nil || ctx.World == nil || actorID == 0 {
		return mobileui.TargetHUDModel{}, false
	}
	actor, ok := ctx.World.Actors[actorID]
	if !ok {
		return mobileui.TargetHUDModel{}, false
	}
	model := mobileui.TargetHUDModel{Visible: true, ID: actor.ID, Name: m.hoveredActorDisplayName(ctx, actor, time.Now()), Relation: mobileui.TargetFriendly}
	if ctx.Offline != nil {
		if name, hp, maxHP, npc, found := ctx.Offline.TargetForActor(actorID); found {
			model.Name, model.HP, model.MaxHP = name, hp, maxHP
			if npc {
				model.Relation = mobileui.TargetNPC
			}
		}
	}
	if model.Name == "" {
		model.Name = actor.Name
	}
	if model.Relation != mobileui.TargetNPC {
		switch {
		case isMonsterLikeHoverActor(actor):
			model.Relation = mobileui.TargetHostile
		case cursorActorCanTalk(actor):
			model.Relation = mobileui.TargetNPC
		}
	}
	if life, found := m.actorLifeForDisplay(ctx, actor); found {
		model.HP, model.MaxHP = life.hp, life.maxHP
	}
	return model, true
}
