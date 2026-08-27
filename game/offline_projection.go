package game

import (
	"time"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/network"
	"github.com/kivutar/goro/session"
	"github.com/kivutar/goro/world"
)

// syncOfflineProjection keeps the renderer-facing World as a projection of
// local authority. It deliberately does not mutate combat, inventory, or
// monster rules; those live behind OfflineSession.GameSession.
func (m *WorldMode) syncOfflineProjection(ctx client.Context) {
	if m == nil || ctx.Offline == nil || ctx.World == nil {
		return
	}
	// Offline authority emits compact events rather than network packets. Feed
	// those events through the existing renderer-facing action pipeline so the
	// local slice gets the same attack, hit, skill, damage-number, and death
	// presentation as the online path.
	for _, event := range ctx.Offline.DrainEvents() {
		switch event.Kind {
		case session.OfflineEventDamage:
			m.applyOfflineDamageEvent(ctx, event)
		case session.OfflineEventMonsterDied:
			m.startActorDeath(ctx, event.ActorID)
		}
		if event.ActorID == ctx.Offline.TargetID && event.Kind == session.OfflineEventDamage {
			ctx.Offline.TargetHP = event.Amount
		}
	}
	seenActors := make(map[uint32]struct{})
	for _, monster := range ctx.Offline.Monsters() {
		if monster.State == session.MonsterDead {
			// Keep a dead actor projected while the shared death animation/fade
			// owns its removal. The next respawn state is projected normally.
			if _, animating := m.actorDeaths[monster.ID]; !animating {
				ctx.World.RemoveActor(monster.ID)
			}
			continue
		}
		if monster.State == session.MonsterRespawn { // respawn projection is hidden for this transition step
			ctx.World.RemoveActor(monster.ID)
			continue
		}
		seenActors[monster.ID] = struct{}{}
		speed := monster.WalkSpeed
		if speed <= 0 {
			speed = 150
		}
		seenActors[monster.ID] = struct{}{}
		ctx.World.UpsertActor(world.Actor{ID: monster.ID, Name: monster.Name, X: monster.X, Y: monster.Y, Job: monster.Job, ObjectType: 5, HasObjectType: true, Appearance: true, Speed: speed, AttackRange: monster.AttackRange, HasLevel: true, Level: monster.Level})
		if ctx.Offline.TargetID == monster.ID {
			ctx.Offline.TargetHP, ctx.Offline.TargetMaxHP = monster.HP, monster.MaxHP
		}
	}
	for _, npc := range ctx.Offline.NPCs() {
		seenActors[npc.ID] = struct{}{}
		job := npc.Sprite
		if job == 0 {
			job = 1002
		}
		ctx.World.UpsertActor(world.Actor{ID: npc.ID, Name: npc.Name, X: npc.X, Y: npc.Y, Dir: npc.Dir, Job: job, ObjectType: 6, HasObjectType: true, Appearance: true, Speed: 150})
	}
	for _, warp := range ctx.Offline.Warps() {
		seenActors[warp.ID] = struct{}{}
		ctx.World.UpsertActor(world.Actor{ID: warp.ID, Name: warp.Name, X: warp.X, Y: warp.Y, Job: 45, ObjectType: 6, HasObjectType: true, Appearance: true, Speed: 150})
	}
	if m.offlineActorIDs == nil {
		m.offlineActorIDs = map[uint32]struct{}{}
	}
	for id := range m.offlineActorIDs {
		if _, ok := seenActors[id]; !ok {
			ctx.World.RemoveActor(id)
		}
	}
	m.offlineActorIDs = seenActors
	for _, drop := range ctx.Offline.Drops() {
		if m.offlineItemIDs == nil {
			m.offlineItemIDs = map[uint32]struct{}{}
		}
		m.offlineItemIDs[drop.ID] = struct{}{}
		ctx.World.UpsertItem(world.FloorItem{ID: drop.ID, ItemID: drop.ItemID, Identified: drop.Identified, X: drop.X, Y: drop.Y, Amount: uint16(drop.Amount), DroppedAt: time.Now()})
	}
	if m.offlineItemIDs != nil {
		active := map[uint32]struct{}{}
		for _, drop := range ctx.Offline.Drops() {
			active[drop.ID] = struct{}{}
		}
		for id := range m.offlineItemIDs {
			if _, ok := active[id]; !ok {
				ctx.World.RemoveItem(id)
				delete(m.offlineItemIDs, id)
			}
		}
	}
}

func (m *WorldMode) applyOfflineDamageEvent(ctx client.Context, event session.OfflineEvent) {
	if m == nil || ctx.World == nil || ctx.Session == nil || event.ActorID == 0 || event.Damage <= 0 {
		return
	}
	sourceID := ctx.Session.CharID
	if sourceID == 0 {
		sourceID = ctx.Session.AccountID
	}
	if sourceID == 0 {
		sourceID = ctx.World.Player.ID
	}
	if sourceID == 0 {
		return
	}
	notify := network.ActorActionNotify{
		SourceID:    sourceID,
		TargetID:    event.ActorID,
		Damage:      int32(event.Damage),
		HitCount:    1,
		SkillID:     event.SkillID,
		SkillLevel:  1,
		SourceSpeed: 0,
		TargetSpeed: 0,
	}
	if event.SkillID > 0 {
		notify.Action = network.ActorActionSkill
	}
	m.applyActorActionNotify(ctx, notify)
}
