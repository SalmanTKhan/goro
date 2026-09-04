package game

import (
	"testing"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
	worldstate "github.com/kivutar/goro/world"
)

// TestApplyOfflineProfileSurvivesSavingAnExistingProfile covers the save the
// character screen performs when it edits the profile in place, as opposed to
// creating a fresh one. That branch rebuilds the world player from the updated
// character, and had never been exercised: the profile projected from world
// state was rejected by SetProfile before reaching it.
func TestApplyOfflineProfileSurvivesSavingAnExistingProfile(t *testing.T) {
	state := session.New()
	state.SelectCharacter(session.Character{
		ID: 1, Name: "Offline Adventurer", Job: 0, HP: 100, MaxHP: 100, SP: 30, MaxSP: 30, Hair: 1,
	})
	state.Playing = true
	state.PlayerX, state.PlayerY = 10, 10

	offline := session.NewOfflineSession("prontera")
	offline.BindState(state)

	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 1, Name: "Offline Adventurer", X: 10, Y: 10, Job: 0, Appearance: true}

	mode := &WorldMode{}
	ctx := client.Context{Session: state, World: world, Offline: offline}

	if !mode.applyOfflineProfile(ctx, input.PlayerCommand{
		Kind:             input.CommandSaveOfflineProfile,
		Text:             "Offline Adventurer",
		ProfileID:        1,
		ProfileSex:       0,
		ProfileHairStyle: 4,
		ProfileHairColor: 2,
		ProfileStats:     [6]uint8{6, 5, 5, 4, 5, 5},
		ProfileNew:       false,
	}) {
		t.Fatal("saving an existing offline profile was rejected")
	}
	if world.Player.Head != 4 {
		t.Fatalf("world player hair = %d, want the saved 4", world.Player.Head)
	}
	// Rebuilding the actor for its new appearance must not move the player.
	// It did: the rebuilt actor carried no position, so saving teleported the
	// player to 0,0 — off the viewport, which then crashed the actor draw.
	if world.Player.X != 10 || world.Player.Y != 10 {
		t.Fatalf("world player position = (%d,%d), want it left at (10,10)", world.Player.X, world.Player.Y)
	}
}
