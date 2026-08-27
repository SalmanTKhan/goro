package game

import (
	"testing"
	"time"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
	worldstate "github.com/kivutar/goro/world"
)

func TestSyncOfflineProjectionBridgesCombatPresentation(t *testing.T) {
	state := session.New()
	state.SelectCharacter(session.Character{ID: 1, Name: "Offline Adventurer", Job: 0, HP: 100, MaxHP: 100, SP: 30, MaxSP: 30})
	state.Playing = true
	state.PlayerX, state.PlayerY = 10, 10

	offline := session.NewOfflineSession("prontera")
	offline.BindState(state)
	offline.AddMonster(session.OfflineMonster{ID: 9002, Name: "Poring", X: 11, Y: 10, Job: 1002, MaxHP: 1, Attack: 0})
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandAttackActor, ActorID: 9002}) {
		t.Fatal("offline attack command was rejected")
	}
	offline.Update(50 * time.Millisecond)

	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 1, Name: "Offline Adventurer", X: 10, Y: 10, Job: 0, Appearance: true}
	world.UpsertActor(worldstate.Actor{ID: 9002, Name: "Poring", X: 11, Y: 10, Job: 1002, ObjectType: 5, HasObjectType: true, Appearance: true})
	mode := &WorldMode{actorAnims: map[uint32]actorAnimation{}, actorDeaths: map[uint32]time.Time{}, actorLife: map[uint32]actorLife{}}
	mode.syncOfflineProjection(client.Context{Session: state, World: world, Offline: offline})

	if _, ok := mode.actorAnimation(1, time.Now()); !ok {
		t.Fatal("offline damage did not start the local attack animation")
	}
	if _, ok := mode.actorAnimation(9002, time.Now()); !ok {
		t.Fatal("offline lethal damage did not start the monster death animation")
	}
	if _, ok := world.Actors[9002]; !ok {
		t.Fatal("dead monster was removed before its death animation could render")
	}
	if len(mode.damageFloaters) != 1 || mode.damageFloaters[0].text != "8" {
		t.Fatalf("offline damage floaters = %+v, want one 8 floater", mode.damageFloaters)
	}
	drops := offline.Drops()
	if len(drops) != 1 {
		t.Fatalf("offline drops = %+v, want one projected floor item", drops)
	}
	if item, ok := world.Items[drops[0].ID]; !ok || item.ItemID != drops[0].ItemID || item.X != drops[0].X || item.Y != drops[0].Y {
		t.Fatalf("offline floor item projection = %+v, want drop %+v", item, drops[0])
	}
}

func TestSyncOfflineProjectionBridgesOfflineSkillPresentation(t *testing.T) {
	state := session.New()
	state.SelectCharacter(session.Character{ID: 1, Name: "Offline Adventurer", Job: 0, HP: 100, MaxHP: 100, SP: 30, MaxSP: 30})
	state.Playing = true
	state.PlayerX, state.PlayerY = 10, 10
	state.Skills.List = []session.Skill{{ID: db.SkillSMBash, Name: "Bash", Level: 1, MaxLevel: 10, SPCost: 5}}

	offline := session.NewOfflineSession("prontera")
	offline.BindState(state)
	offline.AddMonster(session.OfflineMonster{ID: 9002, Name: "Poring", X: 11, Y: 10, Job: 1002, MaxHP: 45, Attack: 0})
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandUseSkillOnActor, ActorID: 9002, SkillID: db.SkillSMBash, Level: 1}) {
		t.Fatal("offline skill command was rejected")
	}

	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 1, Name: "Offline Adventurer", X: 10, Y: 10, Job: 0, Appearance: true}
	world.UpsertActor(worldstate.Actor{ID: 9002, Name: "Poring", X: 11, Y: 10, Job: 1002, ObjectType: 5, HasObjectType: true, Appearance: true})
	mode := &WorldMode{actorAnims: map[uint32]actorAnimation{}, actorDeaths: map[uint32]time.Time{}, actorLife: map[uint32]actorLife{}, speechBubbles: map[uint32]speechBubble{}}
	mode.syncOfflineProjection(client.Context{Session: state, World: world, Offline: offline})

	anim, ok := mode.actorAnimation(1, time.Now())
	if !ok || anim.actionFamily == spriteActionIdle {
		t.Fatalf("offline skill animation = %+v, want a transient action", anim)
	}
	if len(mode.speechBubbles) != 1 || mode.speechBubbles[1].text != "Bash !!" {
		t.Fatalf("offline skill bubble = %+v, want Bash !!", mode.speechBubbles)
	}
}

func TestOfflinePickupAutoApproachesAndCompletes(t *testing.T) {
	state := session.New()
	state.SelectCharacter(session.Character{ID: 1, Name: "Offline Adventurer", Job: 0, HP: 100, MaxHP: 100, SP: 30, MaxSP: 30, Str: 5})
	state.Playing = true
	state.PlayerX, state.PlayerY = 10, 10
	offline := session.NewOfflineSession("prontera")
	offline.BindState(state)
	offline.AddMonster(session.OfflineMonster{
		ID: 9002, Name: "Poring", X: 14, Y: 10, Job: 1002, MaxHP: 1, Attack: 0,
		DropTable: []session.OfflineDropTable{{ItemID: 909, Amount: 1}},
	})
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandAttackActor, ActorID: 9002}) {
		t.Fatal("offline attack command was rejected")
	}
	offline.Update(50 * time.Millisecond)
	drops := offline.Drops()
	if len(drops) != 1 {
		t.Fatalf("offline drops = %+v, want one drop", drops)
	}

	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 1, X: 10, Y: 10, Job: 0, Appearance: true}
	world.UpsertItem(worldstate.FloorItem{ID: drops[0].ID, ItemID: drops[0].ItemID, X: drops[0].X, Y: drops[0].Y, Amount: uint16(drops[0].Amount)})
	mode := &WorldMode{}
	ctx := client.Context{Session: state, World: world, Offline: offline}
	if !mode.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandPickUpItem, ItemID: drops[0].ID}) {
		t.Fatal("out-of-range pickup did not start an approach")
	}
	if mode.pendingPickup.itemID != drops[0].ID {
		t.Fatalf("pending pickup = %+v, want drop %d", mode.pendingPickup, drops[0].ID)
	}
	if world.Player.X != drops[0].X || world.Player.Y != drops[0].Y {
		t.Fatalf("approach destination = %d,%d, want %d,%d", world.Player.X, world.Player.Y, drops[0].X, drops[0].Y)
	}

	world.Player.Moving = false
	mode.updatePendingPickup(ctx, "test", false)
	mode.pendingPickup.readyAt = time.Now().Add(-time.Second)
	mode.processPendingPickup(ctx)
	if len(offline.Drops()) != 0 {
		t.Fatalf("drop remained after pending pickup: %+v", offline.Drops())
	}
	if len(state.Inventory.Items) != 1 || state.Inventory.Items[0].ItemID != 909 {
		t.Fatalf("inventory after pickup = %+v", state.Inventory.Items)
	}
}
