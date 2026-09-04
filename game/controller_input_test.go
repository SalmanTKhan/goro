package game

import (
	"testing"
	"time"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/network"
	"github.com/kivutar/goro/session"
	worldstate "github.com/kivutar/goro/world"
)

func TestProductionWASDUsesTheExistingWalkRequestPolicy(t *testing.T) {
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	defer networkClient.Close()
	defer serverConn.Close()

	state := input.NewState()
	w, ok := input.KeyCodeFromName("KeyW")
	if !ok {
		t.Fatal("KeyW was not recognized")
	}
	state.SetKeyCode(w, true)
	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 2000000, X: 10, Y: 20}
	world.GAT = flatWalkableGAT(64, 64)
	mode := &WorldMode{}
	ctx := client.Context{
		Config:  client.Context{}.Config,
		Input:   state,
		Network: networkClient,
		Session: &session.Session{AccountID: 2000000, CharID: 150000},
		World:   world,
	}

	mode.updateControllerInput(ctx, false, false)
	want, ok := network.BuildWalkToXYPacketForClientDate(10, 28, 20080910)
	if !ok {
		t.Fatal("failed to build expected walk packet")
	}
	readBotTestPackets(t, serverConn, want)
	if mode.controllerMoveDir != input.DirectionNorth {
		t.Fatalf("controller movement direction = %v, want north", mode.controllerMoveDir)
	}
}

func TestLuaProfileSuppressesProductionMovement(t *testing.T) {
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	defer networkClient.Close()
	defer serverConn.Close()

	state := input.NewState()
	w, _ := input.KeyCodeFromName("KeyW")
	state.SetKeyCode(w, true)
	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 2000000, X: 10, Y: 20}
	world.GAT = flatWalkableGAT(64, 64)
	mode := &WorldMode{}
	ctx := client.Context{
		Config:  client.Context{}.Config,
		Input:   state,
		Network: networkClient,
		Session: &session.Session{AccountID: 2000000, CharID: 150000},
		World:   world,
	}
	ctx.Config.Script.Path = "legacy.lua"

	mode.updateControllerInput(ctx, false, false)
	if err := serverConn.SetReadDeadline(testNetworkReadDeadline()); err != nil {
		t.Fatal(err)
	}
	if _, err := serverConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("Lua-owned input unexpectedly emitted a production walk")
	}
}

func TestControllerTargetCycleIsStableAndWraps(t *testing.T) {
	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 100, X: 10, Y: 10}
	world.Actors[201] = worldstate.Actor{ID: 201, X: 11, Y: 10, ObjectType: actorObjectTypeMob, HasObjectType: true}
	world.Actors[202] = worldstate.Actor{ID: 202, X: 12, Y: 10, ObjectType: actorObjectTypeMob, HasObjectType: true}
	mode := &WorldMode{}
	ctx := client.Context{Session: &session.Session{CharID: 100}, World: world}

	if !mode.cycleControllerTarget(ctx, false) || mode.attackFocusID != 201 {
		t.Fatalf("first target = %d, want 201", mode.attackFocusID)
	}
	if !mode.cycleControllerTarget(ctx, false) || mode.attackFocusID != 202 {
		t.Fatalf("second target = %d, want 202", mode.attackFocusID)
	}
	if !mode.cycleControllerTarget(ctx, false) || mode.attackFocusID != 201 {
		t.Fatalf("wrapped target = %d, want 201", mode.attackFocusID)
	}
	if !mode.cycleControllerTarget(ctx, true) || mode.attackFocusID != 202 {
		t.Fatalf("reverse target = %d, want 202", mode.attackFocusID)
	}
}

func TestControllerTargetCycleIncludesNearestNPCAndMonster(t *testing.T) {
	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 100, X: 10, Y: 10}
	world.Actors[201] = worldstate.Actor{ID: 201, X: 12, Y: 10, ObjectType: actorObjectTypeMob, HasObjectType: true}
	world.Actors[202] = worldstate.Actor{ID: 202, X: 11, Y: 10, ObjectType: actorObjectTypeNPC, HasObjectType: true}
	world.Actors[204] = worldstate.Actor{ID: 204, X: 20, Y: 10, ObjectType: actorObjectTypeMob, HasObjectType: true}
	world.Actors[203] = worldstate.Actor{ID: 203, X: 30, Y: 30, ObjectType: actorObjectTypeMob, HasObjectType: true}
	mode := &WorldMode{}
	ctx := client.Context{Session: &session.Session{CharID: 100}, World: world}

	if !mode.cycleControllerTarget(ctx, false) || mode.attackFocusID != 202 {
		t.Fatalf("nearest target = %d, want NPC 202", mode.attackFocusID)
	}
	if !mode.cycleControllerTarget(ctx, false) || mode.attackFocusID != 201 {
		t.Fatalf("cycled target = %d, want monster 201", mode.attackFocusID)
	}
	if !mode.cycleControllerTarget(ctx, false) || mode.attackFocusID != 204 {
		t.Fatalf("visible distant target = %d, want monster 204", mode.attackFocusID)
	}
	if !mode.cycleControllerTarget(ctx, false) || mode.attackFocusID != 202 {
		t.Fatalf("target cycle wrapped to = %d, want NPC 202", mode.attackFocusID)
	}
}

func TestControllerTargetCycleIncludesFloorItemsAndCrossPicksFocusedItem(t *testing.T) {
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	defer networkClient.Close()
	defer serverConn.Close()

	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 2000000, X: 10, Y: 20}
	world.Items[301] = worldstate.FloorItem{ID: 301, ItemID: 909, X: 11, Y: 20, Amount: 1}
	world.Actors[201] = worldstate.Actor{ID: 201, X: 12, Y: 20, ObjectType: actorObjectTypeMob, HasObjectType: true}
	mode := &WorldMode{}
	ctx := client.Context{
		Network: networkClient,
		Session: &session.Session{AccountID: 2000000, CharID: 100},
		World:   world,
	}

	if !mode.cycleControllerTarget(ctx, false) || mode.controllerFocusItemID != 301 {
		t.Fatalf("first focus item = %d, want 301", mode.controllerFocusItemID)
	}
	if !mode.controllerInteractFocused(ctx) {
		t.Fatal("Cross did not request pickup for focused floor item")
	}
	want := network.BuildItemPickupPacketForClientDate(301, 20080910)
	readBotTestPackets(t, serverConn, want)
}

func TestControllerInteractFocusedTalksToNPCAndCancelsCombat(t *testing.T) {
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	defer networkClient.Close()
	defer serverConn.Close()

	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 2000000, X: 10, Y: 20}
	npc := worldstate.Actor{ID: 302, X: 11, Y: 20, ObjectType: actorObjectTypeNPC, HasObjectType: true}
	world.Actors[npc.ID] = npc
	mode := &WorldMode{
		attackFocusID:  302,
		lockedAttackID: 201,
		pendingAttack:  attackIntent{targetID: 201, expires: time.Now().Add(time.Second)},
	}
	ctx := client.Context{
		Network: networkClient,
		Session: &session.Session{AccountID: 2000000, CharID: 150000},
		World:   world,
	}

	if !mode.controllerInteractFocused(ctx) {
		t.Fatal("Cross did not request contact with the focused NPC")
	}
	readBotTestPackets(t, serverConn, network.BuildNPCContactPacket(npc.ID, 0))
	if mode.pendingAttack.targetID != 0 || mode.lockedAttackID != 0 {
		t.Fatalf("NPC interaction retained combat intent: pending=%d locked=%d", mode.pendingAttack.targetID, mode.lockedAttackID)
	}
}

func TestControllerConfirmPreemptsLockedAttackBeforeCombatProcessing(t *testing.T) {
	state := input.NewState()
	settings := input.DefaultControllerSettings()
	snapshot := input.ControllerSnapshot{Connected: true}
	snapshot.Buttons.Set(settings.Bindings.Confirm, true)
	state.SetController(snapshot)

	mode := &WorldMode{
		lockedAttackID: 201,
		pendingAttack:  attackIntent{targetID: 201, expires: time.Now().Add(time.Second)},
	}
	ctx := client.Context{
		Config: client.Context{}.Config,
		Input:  state,
	}
	ctx.Config.Controller = settings

	mode.preemptControllerCombat(ctx)
	if mode.lockedAttackID != 0 || mode.pendingAttack.targetID != 0 {
		t.Fatalf("confirm left stale combat intent: locked=%d pending=%d", mode.lockedAttackID, mode.pendingAttack.targetID)
	}
}

func TestControllerCancelClearsPendingPickupAndStopsApproachWalk(t *testing.T) {
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	defer networkClient.Close()
	defer serverConn.Close()

	now := time.Now()
	world := worldstate.New()
	world.Player = worldstate.Actor{
		ID:           2000000,
		X:            10,
		Y:            20,
		FromX:        10,
		FromY:        20,
		ToX:          10,
		ToY:          22,
		Moving:       true,
		MoveStarted:  now,
		MoveDuration: 2 * time.Second,
		MovePath: []worldstate.WalkStep{
			{X: 10, Y: 20},
			{X: 10, Y: 21},
			{X: 10, Y: 22},
		},
	}
	world.Items[301] = worldstate.FloorItem{ID: 301, ItemID: 909, X: 10, Y: 24}
	mode := &WorldMode{pendingPickup: pickupIntent{itemID: 301, expires: now.Add(time.Second)}}
	ctx := client.Context{
		Network: networkClient,
		Session: &session.Session{AccountID: 2000000, CharID: 150000},
		World:   world,
	}

	if !mode.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandCancelAction}) {
		t.Fatal("controller cancel was rejected")
	}
	if mode.pendingPickup.itemID != 0 {
		t.Fatalf("pending pickup survived cancel: %d", mode.pendingPickup.itemID)
	}
	want, ok := network.BuildWalkToXYPacketForClientDate(10, 21, 20080910)
	if !ok {
		t.Fatal("failed to build expected shortened stop packet")
	}
	readBotTestPackets(t, serverConn, want)
}

func TestControllerAttackSkipsTalkOnlyFocusedNPC(t *testing.T) {
	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 100, X: 10, Y: 10}
	world.Actors[201] = worldstate.Actor{ID: 201, X: 11, Y: 10, ObjectType: actorObjectTypeNPC, HasObjectType: true}
	world.Actors[202] = worldstate.Actor{ID: 202, X: 12, Y: 10, ObjectType: actorObjectTypeMob, HasObjectType: true}
	mode := &WorldMode{attackFocusID: 201}
	ctx := client.Context{Session: &session.Session{CharID: 100}, World: world}

	if !mode.controllerAttackFocused(ctx) {
		t.Fatal("controller attack did not select a nearby monster")
	}
	if mode.attackFocusID != 202 {
		t.Fatalf("attack focus = %d, want monster 202", mode.attackFocusID)
	}
}

func testNetworkReadDeadline() (deadline time.Time) {
	return time.Now().Add(20 * time.Millisecond)
}

// controllerTestHost supplies live controller settings the way app.Game does.
type controllerTestHost struct {
	settings input.ControllerSettings
}

func (h *controllerTestHost) ControllerSettings() input.ControllerSettings {
	return h.settings
}

func (h *controllerTestHost) ApplyControllerSettings(settings input.ControllerSettings) bool {
	h.settings = settings
	return true
}

func newControllerModeTestContext(t *testing.T, settings input.ControllerSettings) (client.Context, *input.State, func()) {
	t.Helper()
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	state := input.NewState()
	w, ok := input.KeyCodeFromName("KeyW")
	if !ok {
		t.Fatal("KeyW was not recognized")
	}
	state.SetKeyCode(w, true)
	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 2000000, X: 10, Y: 20}
	world.GAT = flatWalkableGAT(64, 64)
	ctx := client.Context{
		Input:                  state,
		Network:                networkClient,
		Session:                &session.Session{AccountID: 2000000, CharID: 150000},
		World:                  world,
		ControllerSettingsHost: &controllerTestHost{settings: settings},
	}
	return ctx, state, func() {
		networkClient.Close()
		serverConn.Close()
	}
}

func TestCursorMoveModeNeverWalksDirectly(t *testing.T) {
	// In cursor move mode walking happens through the synthetic click landing in
	// the ordinary world-click path, never through CommandMoveDirection.
	settings := input.DefaultControllerSettings()
	settings.MoveMode = input.ControllerMoveCursor
	ctx, _, cleanup := newControllerModeTestContext(t, settings)
	defer cleanup()

	mode := &WorldMode{}
	mode.updateControllerInput(ctx, false, false)
	if mode.controllerMoveDir != input.DirectionNone {
		t.Fatalf("cursor mode walked: direction = %v", mode.controllerMoveDir)
	}
}

func TestDisabledControllerReleasesMovement(t *testing.T) {
	settings := input.DefaultControllerSettings()
	ctx, _, cleanup := newControllerModeTestContext(t, settings)
	defer cleanup()

	mode := &WorldMode{}
	mode.updateControllerInput(ctx, false, false)
	if mode.controllerMoveDir == input.DirectionNone {
		t.Fatal("setup did not start the player walking")
	}

	// Turning the controller off mid-walk must release movement, not merely
	// stop feeding it -- otherwise the character keeps walking.
	host := ctx.ControllerSettingsHost.(*controllerTestHost)
	host.settings.MoveMode = input.ControllerMoveCursor
	mode.updateControllerInput(ctx, false, false)
	if mode.controllerMoveDir != input.DirectionNone {
		t.Fatalf("movement not released on mode change: %v", mode.controllerMoveDir)
	}
}

func TestControllerMovementConsumedReleasesMovement(t *testing.T) {
	settings := input.DefaultControllerSettings()
	ctx, state, cleanup := newControllerModeTestContext(t, settings)
	defer cleanup()

	mode := &WorldMode{}
	mode.updateControllerInput(ctx, false, false)
	if mode.controllerMoveDir == input.DirectionNone {
		t.Fatal("setup did not start the player walking")
	}

	// The renderer consumes movement when the left stick is aiming the pointer.
	state.ConsumeControllerMovement()
	mode.updateControllerInput(ctx, false, false)
	if mode.controllerMoveDir != input.DirectionNone {
		t.Fatalf("movement not released when consumed: %v", mode.controllerMoveDir)
	}
}

func TestControllerReleaseRetriesStopAfterWalkCooldown(t *testing.T) {
	settings := input.DefaultControllerSettings()
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	defer networkClient.Close()
	defer serverConn.Close()

	keyW, ok := input.KeyCodeFromName("KeyW")
	if !ok {
		t.Fatal("KeyW was not recognized")
	}
	state := input.NewState()
	world := worldstate.New()
	now := time.Now()
	world.Player = worldstate.Actor{
		ID:           2000000,
		X:            10,
		Y:            28,
		FromX:        10,
		FromY:        20,
		ToX:          10,
		ToY:          28,
		Moving:       true,
		MoveStarted:  now,
		MoveDuration: 8 * time.Second,
		MovePath: []worldstate.WalkStep{
			{X: 10, Y: 20},
			{X: 10, Y: 21},
			{X: 10, Y: 28},
		},
	}
	world.GAT = flatWalkableGAT(64, 64)
	ctx := client.Context{
		Input:                  state,
		Network:                networkClient,
		Session:                &session.Session{AccountID: 2000000, CharID: 150000},
		World:                  world,
		ControllerSettingsHost: &controllerTestHost{settings: settings},
	}
	mode := &WorldMode{
		controllerMoveDir: input.DirectionNorth,
		walkCooldownUntil: now.Add(100 * time.Millisecond),
	}
	state.SetKeyCode(keyW, false)

	mode.updateControllerInput(ctx, false, false)
	if !mode.controllerStopPending {
		t.Fatal("release did not preserve a pending stop while walk cooldown was active")
	}
	if err := serverConn.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err := serverConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("stop request was sent before the walk cooldown expired")
	}

	mode.walkCooldownUntil = time.Time{}
	mode.updateControllerInput(ctx, false, false)
	want, ok := network.BuildWalkToXYPacketForClientDate(10, 21, 20080910)
	if !ok {
		t.Fatal("failed to build expected shortened walk packet")
	}
	readBotTestPackets(t, serverConn, want)
	if mode.controllerStopPending || mode.controllerMoveDir != input.DirectionNone {
		t.Fatalf("movement stop state = pending:%t direction:%v, want cleared", mode.controllerStopPending, mode.controllerMoveDir)
	}
}

func TestControllerReleaseWaitsForMovementAckBeforeStopping(t *testing.T) {
	settings := input.DefaultControllerSettings()
	networkClient, serverConn := newBotTestConnection(t, 20080910)
	defer networkClient.Close()
	defer serverConn.Close()

	state := input.NewState()
	world := worldstate.New()
	world.Player = worldstate.Actor{ID: 2000000, X: 10, Y: 20}
	world.GAT = flatWalkableGAT(64, 64)
	ctx := client.Context{
		Input:                  state,
		Network:                networkClient,
		Session:                &session.Session{AccountID: 2000000, CharID: 150000},
		World:                  world,
		ControllerSettingsHost: &controllerTestHost{settings: settings},
	}
	mode := &WorldMode{
		controllerMoveDir:         input.DirectionNorth,
		controllerMoveTargetX:     10,
		controllerMoveTargetY:     28,
		controllerMoveTargetKnown: true,
	}

	// Releasing before the server's self-move acknowledgement must not issue a
	// correction toward the actor's still-old local position.
	mode.updateControllerInput(ctx, false, false)
	if !mode.controllerStopPending {
		t.Fatal("release did not wait for the outstanding movement acknowledgement")
	}
	if err := serverConn.SetReadDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err := serverConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("stop request was sent before the movement acknowledgement")
	}

	now := time.Now()
	world.Player = worldstate.Actor{
		ID:           2000000,
		X:            10,
		Y:            28,
		FromX:        10,
		FromY:        20,
		ToX:          10,
		ToY:          28,
		Moving:       true,
		MoveStarted:  now,
		MoveDuration: 8 * time.Second,
		MovePath: []worldstate.WalkStep{
			{X: 10, Y: 20},
			{X: 10, Y: 21},
			{X: 10, Y: 28},
		},
	}
	mode.walkCooldownUntil = time.Time{}
	mode.updateControllerInput(ctx, false, false)
	want, ok := network.BuildWalkToXYPacketForClientDate(10, 21, 20080910)
	if !ok {
		t.Fatal("failed to build expected shortened walk packet")
	}
	readBotTestPackets(t, serverConn, want)
	if mode.controllerStopPending || mode.controllerMoveDir != input.DirectionNone {
		t.Fatalf("movement stop state = pending:%t direction:%v, want cleared", mode.controllerStopPending, mode.controllerMoveDir)
	}
}
