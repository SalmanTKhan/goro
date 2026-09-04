package game

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	worldstate "github.com/kivutar/goro/world"
)

const (
	controllerWalkHorizon          = 8
	controllerRefillCells          = 3
	controllerActionRadius         = 8
	controllerTargetFallbackRadius = 24
	controllerCameraScale          = 8
	// controllerZoomInterval throttles analog trigger zoom so a held trigger
	// steps the camera at a readable rate rather than once per frame.
	controllerZoomInterval = 110 * time.Millisecond
	controllerZoomScale    = 120
)

// updateControllerInput is the production keyboard/controller consumer. It
// runs after overlay blocking has been determined and before the legacy Lua
// input callback. A configured Lua profile owns the frame so its movement
// policy remains the sole source of walk requests.
func (m *WorldMode) updateControllerInput(ctx client.Context, dead, blocked bool) {
	if m == nil || ctx.Input == nil {
		return
	}
	if strings.TrimSpace(ctx.Config.Script.Path) != "" {
		return
	}
	settings := ctx.ControllerSettings()
	actions := input.ResolveActions(ctx.Input, settings)

	// Movement is released rather than merely skipped whenever the pad can no
	// longer own it: a disconnect, a switch away from character move mode, or
	// the controller being turned off must never leave the player walking.
	movementOwned := settings.MoveMode == input.ControllerMoveCharacter
	if !movementOwned || dead || blocked || ctx.Input.ControllerMovementConsumed() {
		m.stopControllerMovement(ctx, "controller movement released")
	} else if m.mapPointerBlocked(ctx) {
		// Aiming at the UI must not also walk the player into it.
		m.stopControllerMovement(ctx, "pointer over ui")
	} else {
		m.applyControllerMovement(ctx, actions)
	}

	if dead || blocked {
		return
	}
	m.applyControllerActions(ctx, actions)
}

func (m *WorldMode) applyControllerMovement(ctx client.Context, actions input.ActionState) {
	if actions.Move != input.DirectionNone {
		// A new direction is an explicit resume/re-target and supersedes a stop
		// that was waiting for the walk-request cooldown.
		m.controllerStopPending = false
		m.controllerStopWaitForAck = false
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{
			Kind:      input.CommandMoveDirection,
			Direction: actions.Move,
		})
	} else if m.controllerMoveDir != input.DirectionNone || m.controllerStopPending {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandMoveDirection})
	}
}

func (m *WorldMode) applyControllerActions(ctx client.Context, actions input.ActionState) {
	if (actions.CameraX != 0 || actions.CameraY != 0) && !ctx.Input.ControllerCameraConsumed() {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{
			Kind:   input.CommandRotateCamera,
			DeltaX: float64(actions.CameraX * controllerCameraScale),
			DeltaY: float64(actions.CameraY * controllerCameraScale),
		})
	}

	// The triggers double as shortcut modifiers, so zoom is suppressed on any
	// frame a shortcut fires: shortcuts are face-button edges, zoom is the
	// continuous analog reading.
	if actions.ZoomDelta != 0 && !actions.ShortcutPressed() && !m.mapPointerBlocked(ctx) {
		now := time.Now()
		if m.controllerZoomAt.IsZero() || now.Sub(m.controllerZoomAt) >= controllerZoomInterval {
			m.controllerZoomAt = now
			m.ApplyPlayerCommand(ctx, input.PlayerCommand{
				Kind:   input.CommandZoomCamera,
				DeltaY: float64(actions.ZoomDelta * controllerZoomScale),
			})
		}
	} else if actions.ZoomDelta == 0 {
		m.controllerZoomAt = time.Time{}
	}

	pressed := actions.Pressed
	if pressed.Has(input.ActionTargetPrevious) && !ctx.Input.ControllerActionConsumed(input.ActionTargetPrevious) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandTargetPrevious})
	}
	if pressed.Has(input.ActionTargetNext) && !ctx.Input.ControllerActionConsumed(input.ActionTargetNext) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandTargetNext})
	}
	if pressed.Has(input.ActionConfirm) && !ctx.Input.ControllerActionConsumed(input.ActionConfirm) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandInteractFocused})
	}
	// Confirm is the interaction button. If an unusual controller report or a
	// custom binding makes both face actions edge in one sample, interaction
	// wins so Cross can never silently turn into an attack as well.
	if pressed.Has(input.ActionAttack) && !pressed.Has(input.ActionConfirm) && !ctx.Input.ControllerActionConsumed(input.ActionAttack) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandAttackFocused})
	}
	if pressed.Has(input.ActionLoot) && !ctx.Input.ControllerActionConsumed(input.ActionLoot) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandLootFocused})
	}
	if pressed.Has(input.ActionCancel) && !ctx.Input.ControllerActionConsumed(input.ActionCancel) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandCancelAction})
	}
	m.updateControllerMenuChord(ctx, actions)
	if pressed.Has(input.ActionMap) && !ctx.Input.ControllerActionConsumed(input.ActionMap) {
		m.ui.minimap.Toggle(ctx)
	}
	if pressed.Has(input.ActionResetCamera) && !ctx.Input.ControllerActionConsumed(input.ActionResetCamera) && !m.mapPointerBlocked(ctx) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandResetCamera})
	}
	for slot := 0; slot < 8; slot++ {
		if pressed.Has(input.ActionShortcut1 + input.Action(slot)) {
			m.ApplyPlayerCommand(ctx, input.PlayerCommand{
				Kind: input.CommandUseShortcut,
				Slot: uint16(slot),
			})
		}
	}
}

// preemptControllerCombat handles the intent switch before the normal combat
// queue is processed. World.Update historically processes a locked attack
// before it reaches the device-specific input consumer; without this small
// pre-pass, Cross could emit one stale attack packet before its interaction
// handler had a chance to clear the old chase target.
func (m *WorldMode) preemptControllerCombat(ctx client.Context) {
	if m == nil || ctx.Input == nil || !ctx.ControllerSettings().Enabled || strings.TrimSpace(ctx.Config.Script.Path) != "" {
		return
	}
	actions := input.ResolveActions(ctx.Input, ctx.ControllerSettings())
	for _, action := range []input.Action{input.ActionConfirm, input.ActionLoot, input.ActionCancel} {
		if actions.Pressed.Has(action) && !ctx.Input.ControllerActionConsumed(action) {
			m.clearControllerCombatIntent()
			return
		}
	}
}

func (m *WorldMode) stopControllerMovement(ctx client.Context, source string) {
	if m == nil {
		return
	}
	if m.controllerMoveDir == input.DirectionNone && !m.controllerStopPending {
		return
	}
	// There is no useful stop packet to send when the world is gone, the
	// player is dead, or offline movement is already applied immediately. Do
	// not leave a retry flag behind for the next world/session.
	if ctx.World == nil || playerIsDead(ctx) || ctx.Network == nil {
		m.clearControllerMovementState()
		return
	}
	// The first walk packet can be acknowledged after the input release. Until
	// that acknowledgement updates the local actor path, there is no safe stop
	// destination to send. Keep the target as an outstanding request instead of
	// treating the still-at-origin actor as already stopped.
	if !actorIsMovingAt(ctx.World.Player, time.Now()) {
		if m.controllerStopWaitForAck {
			m.controllerStopPending = true
			m.controllerMoveDir = input.DirectionNone
			return
		}
		playerX, playerY := currentPlayerCell(ctx, time.Now())
		if m.controllerMoveTargetKnown && (m.controllerMoveTargetX != playerX || m.controllerMoveTargetY != playerY) {
			m.controllerStopPending = true
			m.controllerMoveDir = input.DirectionNone
			return
		}
		m.clearControllerMovementState()
		return
	}
	if m.requestWalkStop(ctx, source) {
		m.clearControllerMovementState()
		return
	}
	// Releasing shortly after a horizon request commonly lands inside the
	// request cooldown. Keep the stop alive and retry on subsequent frames;
	// clearing this state here would let the server finish the whole queued
	// path segment.
	m.controllerStopPending = true
	m.controllerMoveDir = input.DirectionNone
}

func (m *WorldMode) clearControllerMovementState() {
	if m == nil {
		return
	}
	m.controllerStopPending = false
	m.controllerStopWaitForAck = false
	m.controllerMoveDir = input.DirectionNone
	m.controllerMoveTargetX = 0
	m.controllerMoveTargetY = 0
	m.controllerMoveTargetKnown = false
}

func (m *WorldMode) moveController(ctx client.Context, direction input.Direction8) bool {
	if direction == input.DirectionNone {
		m.stopControllerMovement(ctx, "controller release")
		return true
	}
	// A fresh held direction is allowed to re-target immediately, even if the
	// previous release could not yet shorten its path because of cooldown.
	m.controllerStopPending = false
	m.controllerStopWaitForAck = false
	if ctx.World == nil || playerIsDead(ctx) {
		return false
	}
	dx, dy := direction.Vector()
	if dx == 0 && dy == 0 {
		return false
	}
	now := time.Now()
	playerX, playerY := currentPlayerCell(ctx, now)
	remaining := maxInt(absInt(m.controllerMoveTargetX-playerX), absInt(m.controllerMoveTargetY-playerY))
	needsTarget := !m.controllerMoveTargetKnown || direction != m.controllerMoveDir || remaining <= controllerRefillCells
	if !needsTarget && (m.controllerMoveTargetX != playerX || m.controllerMoveTargetY != playerY) {
		return true
	}
	if !needsTarget {
		return true
	}
	for distance := controllerWalkHorizon; distance >= 1; distance-- {
		targetX, targetY := playerX+dx*distance, playerY+dy*distance
		if !walkTargetInBounds(ctx, targetX, targetY) {
			continue
		}
		if ctx.World.GAT != nil && !ctx.World.GAT.Walkable(targetX, targetY) {
			continue
		}
		if !m.requestWalk(ctx, targetX, targetY, "controller") {
			continue
		}
		m.cancelAttackIntent()
		m.controllerMoveDir = direction
		m.controllerMoveTargetX, m.controllerMoveTargetY = targetX, targetY
		m.controllerMoveTargetKnown = true
		return true
	}
	// Keep the direction unset when no legal target was found. This lets the
	// next frame retry after a transient collision or map-state change instead
	// of treating the failed request as an already-active walk.
	m.controllerMoveDir = input.DirectionNone
	m.controllerMoveTargetX = 0
	m.controllerMoveTargetY = 0
	m.controllerMoveTargetKnown = false
	return false
}

// cancelControllerAction also stops a server-authoritative walk that was
// started to approach a pickup or attack target. Those walks do not set
// controllerMoveDir, so the normal held-stick release path cannot see them.
func (m *WorldMode) cancelControllerAction(ctx client.Context) {
	if m == nil {
		return
	}
	hadApproachWalk := m.pendingPickup.itemID != 0 || m.pendingAttack.targetID != 0
	hadControllerStop := m.controllerMoveDir != input.DirectionNone || m.controllerStopPending
	m.pendingPickup = pickupIntent{}
	m.cancelAttackIntent()
	if !hadApproachWalk && !hadControllerStop {
		return
	}
	if hadApproachWalk {
		// requestPickup/requestAttack may have sent a walk packet before the
		// local actor path has been acknowledged. Wait for that acknowledgement,
		// then shorten the approved path just like a released stick.
		m.controllerStopPending = true
		m.controllerStopWaitForAck = true
	}
	m.stopControllerMovement(ctx, "controller cancel")
}

type controllerTargetCandidate struct {
	actor  worldstate.Actor
	item   worldstate.FloorItem
	isItem bool
	dist   int
}

// controllerTargetCandidates is deliberately broader than the combat target
// filter. A controller target is something the player can act on: monsters
// and hostile companions can be attacked, while ordinary NPCs can be talked
// to. Keeping this distinction here prevents a generic target cycle from
// accidentally sending an attack request to an NPC.
func (m *WorldMode) controllerTargetCandidates(ctx client.Context, attackOnly bool) []controllerTargetCandidate {
	if ctx.World == nil {
		return nil
	}
	now := time.Now()
	playerX, playerY := currentPlayerCell(ctx, now)
	width, height := ctx.ScreenSize()
	useViewport := width > 0 && height > 0
	var projection sceneProjection
	if useViewport {
		projection = m.sceneProjection(ctx, width, height, now)
	}
	maxDistance := controllerTargetFallbackRadius * controllerTargetFallbackRadius
	candidates := make([]controllerTargetCandidate, 0, len(ctx.World.Actors)+len(ctx.World.Items))
	for id, actor := range ctx.World.Actors {
		if _, dead := m.actorDeaths[id]; dead {
			continue
		}
		if attackOnly {
			if !actorCanBeAttackClicked(ctx, actor) {
				continue
			}
		} else if !actorCanBeAttackClicked(ctx, actor) && !cursorActorCanTalk(actor) {
			continue
		}
		actorX, actorY := actorCurrentCell(actor, now)
		dx, dy := actorX-playerX, actorY-playerY
		distance := dx*dx + dy*dy
		if (!useViewport && distance > maxDistance) || (useViewport && !controllerActorVisible(projection, ctx.World, actor, now, width, height)) {
			continue
		}
		candidates = append(candidates, controllerTargetCandidate{actor: actor, dist: distance})
	}
	if !attackOnly {
		for id, item := range ctx.World.Items {
			if id == 0 || item.ID == 0 {
				continue
			}
			itemX, itemY := item.X, item.Y
			dx, dy := itemX-playerX, itemY-playerY
			distance := dx*dx + dy*dy
			if (!useViewport && distance > maxDistance) || (useViewport && !controllerItemVisible(projection, ctx.World, item, now, width, height)) {
				continue
			}
			candidates = append(candidates, controllerTargetCandidate{item: item, isItem: true, dist: distance})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].dist != candidates[j].dist {
			return candidates[i].dist < candidates[j].dist
		}
		if candidates[i].isItem != candidates[j].isItem {
			return candidates[i].isItem
		}
		return candidates[i].targetID() < candidates[j].targetID()
	})
	return candidates
}

func (c controllerTargetCandidate) targetID() uint32 {
	if c.isItem {
		return c.item.ID
	}
	return c.actor.ID
}

func controllerActorVisible(projection sceneProjection, world *worldstate.World, actor worldstate.Actor, now time.Time, width, height int) bool {
	actorX, actorY := actorRenderPosition(actor, now)
	terrainZ := terrainHeightAt(world, actorX, actorY)
	worldX, worldY := actorWorldAnchor(actor, actorX, actorY)
	point := projection.Project(worldX, worldY, terrainZ)
	scale := actorBillboardScreenScale(projection, worldX, worldY, terrainZ)
	if actorRepresentsPlayer(actor) {
		scale *= playerBodyScaleForJob(actor.Job)
	}
	return !actorAnchorOutsideViewport(float64(point.x), float64(point.y), width, height, scale)
}

func controllerItemVisible(projection sceneProjection, world *worldstate.World, item worldstate.FloorItem, now time.Time, width, height int) bool {
	x, y := floorItemWorldPosition(item)
	z := floorItemRenderHeight(world, item, now)
	point := projection.Project(cellCenter(x), cellCenter(y), z)
	return point.x >= -48 && point.y >= -80 && point.x <= float32(width+48) && point.y <= float32(height+48)
}

func (m *WorldMode) cycleControllerTarget(ctx client.Context, reverse bool) bool {
	candidates := m.controllerTargetCandidates(ctx, false)
	if len(candidates) == 0 {
		m.clearAttackFocus()
		return false
	}
	index := -1
	for i := range candidates {
		if candidates[i].isItem && candidates[i].item.ID == m.controllerFocusItemID {
			index = i
			break
		}
		if !candidates[i].isItem && candidates[i].actor.ID == m.attackFocusID {
			index = i
			break
		}
	}
	if reverse {
		if index <= 0 {
			index = len(candidates) - 1
		} else {
			index--
		}
	} else if index < 0 || index == len(candidates)-1 {
		index = 0
	} else {
		index++
	}
	m.focusControllerTarget(candidates[index], time.Now())
	return true
}

func (m *WorldMode) focusedControllerActor(ctx client.Context) (worldstate.Actor, bool) {
	if ctx.World == nil || m.attackFocusID == 0 || m.controllerFocusItemID != 0 {
		return worldstate.Actor{}, false
	}
	actor, ok := ctx.World.Actors[m.attackFocusID]
	if !ok || (!actorCanBeAttackClicked(ctx, actor) && !cursorActorCanTalk(actor)) {
		m.clearAttackFocus()
		return worldstate.Actor{}, false
	}
	return actor, true
}

func (m *WorldMode) controllerAttackFocused(ctx client.Context) bool {
	actor, ok := m.focusedControllerActor(ctx)
	if !ok || !actorCanBeAttackClicked(ctx, actor) {
		candidates := m.controllerTargetCandidates(ctx, true)
		if len(candidates) == 0 {
			return false
		}
		m.focusAttackTarget(candidates[0].actor.ID, time.Now())
		actor = candidates[0].actor
	}
	m.requestAttack(ctx, actor, "controller")
	return true
}

func (m *WorldMode) controllerInteractFocused(ctx client.Context) bool {
	if item, ok := m.focusedControllerItem(ctx); ok {
		m.clearControllerCombatIntent()
		return m.requestPickup(ctx, item, "controller")
	}
	if actor, ok := m.focusedControllerActor(ctx); ok && cursorActorCanTalk(actor) {
		m.clearControllerCombatIntent()
		m.requestNPCTalk(ctx, actor, "controller")
		return true
	}
	if ctx.World == nil {
		return false
	}
	candidates := m.controllerTargetCandidates(ctx, false)
	for _, candidate := range candidates {
		if candidate.isItem || !cursorActorCanTalk(candidate.actor) {
			continue
		}
		m.focusControllerTarget(candidate, time.Now())
		m.clearControllerCombatIntent()
		m.requestNPCTalk(ctx, candidate.actor, "controller")
		return true
	}
	// Cross is never an attack fallback. If no item or NPC is available, make
	// the explicit interaction a consumed no-op and clear any combat intent
	// that would otherwise continue attacking the previous nearest target.
	m.clearControllerCombatIntent()
	return true
}

func (m *WorldMode) controllerLootFocused(ctx client.Context) bool {
	if ctx.World == nil {
		return false
	}
	if item, ok := m.focusedControllerItem(ctx); ok {
		m.clearControllerCombatIntent()
		return m.requestPickup(ctx, item, "controller")
	}
	playerX, playerY := currentPlayerCell(ctx, time.Now())
	var best worldstate.FloorItem
	bestDistance := math.MaxInt
	for _, item := range ctx.World.Items {
		dx, dy := item.X-playerX, item.Y-playerY
		distance := dx*dx + dy*dy
		if distance <= controllerActionRadius*controllerActionRadius && (distance < bestDistance || distance == bestDistance && item.ID < best.ID) {
			best, bestDistance = item, distance
		}
	}
	if best.ID == 0 {
		return false
	}
	m.focusControllerItem(best.ID, time.Now())
	m.clearControllerCombatIntent()
	return m.requestPickup(ctx, best, "controller")
}

// clearControllerCombatIntent prevents a previously selected attack/chase or
// pickup approach from continuing after the pad explicitly switches to a new
// interaction or loot action. Keep the controller's current target focus
// intact so the selected NPC/item still gets its persistent name label and can
// be acted on again.
func (m *WorldMode) clearControllerCombatIntent() {
	if m == nil {
		return
	}
	m.pendingPickup = pickupIntent{}
	m.pendingAttack = attackIntent{}
	m.clearLockedAttack()
}

func (m *WorldMode) focusControllerTarget(candidate controllerTargetCandidate, now time.Time) {
	if candidate.isItem {
		m.focusControllerItem(candidate.item.ID, now)
		return
	}
	m.focusAttackTarget(candidate.actor.ID, now)
}

func (m *WorldMode) focusControllerItem(itemID uint32, now time.Time) {
	if itemID == 0 {
		m.clearControllerItemFocus()
		return
	}
	m.clearAttackFocus()
	m.controllerFocusItemID = itemID
	m.controllerFocusItemStart = now
}

func (m *WorldMode) clearControllerItemFocus() {
	if m == nil {
		return
	}
	m.controllerFocusItemID = 0
	m.controllerFocusItemStart = time.Time{}
}

func (m *WorldMode) focusedControllerItem(ctx client.Context) (worldstate.FloorItem, bool) {
	if ctx.World == nil || m.controllerFocusItemID == 0 || m.attackFocusID != 0 {
		return worldstate.FloorItem{}, false
	}
	item, ok := ctx.World.Items[m.controllerFocusItemID]
	if !ok || item.ID == 0 {
		m.clearControllerItemFocus()
		return worldstate.FloorItem{}, false
	}
	return item, true
}

// controllerMenuHoldDuration is how long the menu button must be held before it
// opens controller setup instead of the escape menu.
const controllerMenuHoldDuration = 500 * time.Millisecond

// updateControllerMenuChord gives the menu button two jobs: a short press opens
// the escape menu, a long press opens controller setup. The short action fires
// on release so a hold can pre-empt it, and controllerMenuSuppressed stops the
// long press from also firing the short one when the button comes back up.
func (m *WorldMode) updateControllerMenuChord(ctx client.Context, actions input.ActionState) {
	if ctx.Input.ControllerActionConsumed(input.ActionMenu) {
		m.controllerMenuHeldAt = time.Time{}
		m.controllerMenuSuppressed = false
		return
	}
	now := time.Now()
	if actions.Pressed.Has(input.ActionMenu) {
		m.controllerMenuHeldAt = now
		m.controllerMenuSuppressed = false
	}
	if actions.Held.Has(input.ActionMenu) && !m.controllerMenuSuppressed &&
		!m.controllerMenuHeldAt.IsZero() && now.Sub(m.controllerMenuHeldAt) >= controllerMenuHoldDuration {
		m.controllerMenuSuppressed = true
		m.ui.controllerWindow.Toggle(ctx)
		return
	}
	if actions.Released.Has(input.ActionMenu) {
		if !m.controllerMenuSuppressed {
			m.ui.escapeMenu.Toggle(ctx)
		}
		m.controllerMenuHeldAt = time.Time{}
		m.controllerMenuSuppressed = false
	}
}
