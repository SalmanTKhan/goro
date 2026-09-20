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
	actions := ctx.ControllerActions
	if !ctx.ControllerActionsValid {
		// Headless/unit callers may not run the renderer; production updates
		// always receive the renderer-owned result.
		actions = input.ResolveActions(ctx.Input, settings)
	}

	// Movement is released rather than merely skipped whenever the pad can no
	// longer own it: a disconnect, a switch away from character move mode, or
	// the controller being turned off must never leave the player walking.
	movementOwned := settings.MoveMode == input.ControllerMoveCharacter
	if !movementOwned || dead || blocked || (!ctx.ControllerActionsValid && ctx.Input.ControllerMovementConsumed()) {
		m.stopControllerMovement(ctx, "controller movement released")
	} else {
		m.applyControllerMovement(ctx, actions)
	}

	if dead || blocked {
		return
	}
	m.applyControllerActions(ctx, actions)
}

// ControllerTargetingActive is queried by the renderer before routing the
// current controller sample. It keeps the right stick on the world targeting
// reticle while a controller skill is pending, including when a trackpad
// pointer override had been used immediately beforehand.
func (m *WorldMode) ControllerTargetingActive() bool {
	return m != nil && m.pendingSkill.skill.ID != 0
}

func (m *WorldMode) applyControllerMovement(ctx client.Context, actions input.ActionState) {
	if ctx.World != nil && ctx.World.Player.Sitting {
		if actions.Move != input.DirectionNone {
			m.rotateSittingController(ctx, actions.Move)
		}
		return
	}
	if ctx.World != nil && m.pendingSkill.skill.ID != 0 {
		return
	}
	if actions.Move != input.DirectionNone {
		// A new direction is an explicit resume/re-target and supersedes a stop
		// that was waiting for the walk-request cooldown.
		m.controllerStopPending = false
		m.controllerStopWaitForAck = false
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{
			Kind:          input.CommandMoveDirection,
			Direction:     actions.Move,
			MoveX:         actions.MoveX,
			MoveY:         actions.MoveY,
			MoveMagnitude: actions.MoveMagnitude,
		})
	} else if m.controllerMoveDir != input.DirectionNone || m.controllerStopPending {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandMoveDirection})
	}
}

func (m *WorldMode) applyControllerActions(ctx client.Context, actions input.ActionState) {
	if m.pendingSkill.skill.ID == 0 {
		m.controllerSkillTargetID = 0
		m.controllerSkillLane = 0
		m.controllerGroundCursorX = 0
		m.controllerGroundCursorY = 0
	}
	if m.pendingSkill.skill.ID != 0 && isGroundTargetSkill(m.pendingSkill.skill) {
		m.updateControllerGroundReticle(ctx, actions)
		// LB/RB use the same target-cycle edges as actor skills, but for a
		// ground skill they reposition the reticle onto the selected actor's
		// current cell. Handle them before the ground-mode early return.
		if actions.Pressed.Has(input.ActionTargetPrevious) {
			m.cycleControllerGroundTarget(ctx, true)
		}
		if actions.Pressed.Has(input.ActionTargetNext) {
			m.cycleControllerGroundTarget(ctx, false)
		}
		if actions.Pressed.Has(input.ActionConfirm) {
			if m.skills().sendTarget(ctx, m.pendingSkill, "controller ground") == nil {
				m.pendingSkill = pendingSkillTarget{}
				m.controllerSkillTargetID = 0
				m.controllerSkillLane = 0
			}
			return
		}
		if actions.Pressed.Has(input.ActionCancel) {
			m.skills().Cancel("controller ground")
			m.controllerSkillTargetID = 0
			m.controllerSkillLane = 0
			return
		}
		return
	}
	if actions.CameraX != 0 || actions.CameraY != 0 {
		now := time.Now()
		dt := time.Second / 60
		if !m.controllerCameraAt.IsZero() {
			dt = now.Sub(m.controllerCameraAt)
			if dt < 0 || dt > 100*time.Millisecond {
				dt = 100 * time.Millisecond
			}
		}
		m.controllerCameraAt = now
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{
			Kind:   input.CommandRotateCamera,
			DeltaX: float64(actions.CameraX*controllerCameraScale) * dt.Seconds(),
			DeltaY: float64(actions.CameraY*controllerCameraScale) * dt.Seconds(),
		})
	} else {
		m.controllerCameraAt = time.Time{}
	}

	// The triggers double as shortcut modifiers, so zoom is suppressed on any
	// frame a shortcut fires: shortcuts are face-button edges, zoom is the
	// continuous analog reading.
	// LT/RT are reserved for shortcut layers in the recommended layout;
	// controller trigger zoom is intentionally disabled.
	m.controllerZoomAt = time.Time{}

	pressed := actions.Pressed
	if pressed.Has(input.ActionTargetPrevious) {
		if isGroundTargetSkill(m.pendingSkill.skill) {
			m.cycleControllerGroundTarget(ctx, true)
		} else if m.pendingSkill.skill.ID != 0 {
			m.cycleControllerSkillTarget(ctx, true)
		} else {
			m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandTargetPrevious})
		}
	}
	if pressed.Has(input.ActionTargetNext) {
		if isGroundTargetSkill(m.pendingSkill.skill) {
			m.cycleControllerGroundTarget(ctx, false)
		} else if m.pendingSkill.skill.ID != 0 {
			m.cycleControllerSkillTarget(ctx, false)
		} else {
			m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandTargetNext})
		}
	}
	for _, lane := range []input.Action{input.ActionTargetSelf, input.ActionTargetAlly, input.ActionTargetEnemy, input.ActionTargetCompanion} {
		if pressed.Has(lane) && m.pendingSkill.skill.ID != 0 {
			m.selectControllerSkillLane(ctx, lane)
		}
	}
	if pressed.Has(input.ActionConfirm) {
		if m.pendingSkill.skill.ID != 0 {
			m.confirmControllerPendingSkill(ctx)
		} else {
			m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandInteractFocused})
		}
	}
	// Confirm is the interaction button. If an unusual controller report or a
	// custom binding makes both face actions edge in one sample, interaction
	// wins so Cross can never silently turn into an attack as well.
	if pressed.Has(input.ActionAttack) && !pressed.Has(input.ActionConfirm) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandAttackFocused})
	}
	if pressed.Has(input.ActionLoot) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandLootFocused})
	}
	if pressed.Has(input.ActionCancel) {
		if m.pendingSkill.skill.ID != 0 {
			m.skills().Cancel("controller")
		} else {
			m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandCancelAction})
		}
	}
	m.updateControllerMenuChord(ctx, actions)
	if pressed.Has(input.ActionMap) {
		m.ui.minimap.Toggle(ctx)
	}
	if pressed.Has(input.ActionResetCamera) && !m.mapPointerBlocked(ctx) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandResetCamera})
	}
	if pressed.Has(input.ActionGameMenu) {
		m.ui.basicMenu.ActivateController(ctx)
	}
	if pressed.Has(input.ActionSit) {
		m.ApplyPlayerCommand(ctx, input.PlayerCommand{Kind: input.CommandToggleSit})
	}
	for slot := 0; slot < 8; slot++ {
		action := input.ActionShortcut1 + input.Action(slot)
		if pressed.Has(action) {
			m.ApplyPlayerCommand(ctx, input.PlayerCommand{
				Kind: input.CommandUseShortcut,
				Slot: uint16(slot),
			})
		}
	}
}

// rotateSittingController shares the mouse sitting behavior: the first valid
// request changes the character's head direction and the next request changes
// the body direction. The existing resolver and network command remain the
// authority for that two-step turn.
func (m *WorldMode) rotateSittingController(ctx client.Context, direction input.Direction8) {
	if m == nil || ctx.World == nil || !ctx.World.Player.Sitting || !m.walkReady(time.Now()) {
		return
	}
	playerX, playerY := currentPlayerCell(ctx, time.Now())
	dx, dy := direction.Vector()
	targetX := playerX + dx
	targetY := playerY + dy
	if targetX == playerX && targetY == playerY {
		return
	}
	m.requestChangeDirection(ctx, targetX, targetY, "controller sitting turn")
}

func (m *WorldMode) updateControllerGroundReticle(ctx client.Context, actions input.ActionState) {
	if ctx.World == nil {
		return
	}
	if m.pendingSkill.x == 0 && m.pendingSkill.y == 0 {
		m.pendingSkill.x, m.pendingSkill.y = currentPlayerCell(ctx, time.Now())
	}
	m.pendingSkill.controllerTargeting = true
	m.pendingSkill.groundTargetSet = true
	m.controllerGroundCursorX += actions.CameraX * 0.75
	m.controllerGroundCursorY += actions.CameraY * 0.75
	stepX, stepY := int(m.controllerGroundCursorX), int(m.controllerGroundCursorY)
	if stepX != 0 || stepY != 0 {
		oldX, oldY := m.pendingSkill.x, m.pendingSkill.y
		m.pendingSkill.x += stepX
		m.pendingSkill.y -= stepY
		m.controllerGroundCursorX -= float32(stepX)
		m.controllerGroundCursorY -= float32(stepY)
		if !walkTargetInBounds(ctx, m.pendingSkill.x, m.pendingSkill.y) {
			m.pendingSkill.x, m.pendingSkill.y = oldX, oldY
		}
	}
}

func (m *WorldMode) cycleControllerGroundTarget(ctx client.Context, reverse bool) {
	// Ground targeting cycles actor positions, not floor-item focus entries.
	// Use the broad candidate set here: the skill-specific legality is handled
	// by the cast path, while this control is selecting a useful world cell.
	allCandidates := m.controllerTargetCandidates(ctx, false)
	candidates := make([]controllerTargetCandidate, 0, len(allCandidates))
	for _, candidate := range allCandidates {
		if !candidate.isItem {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return
	}
	now := time.Now()
	index := -1
	for i := range candidates {
		x, y := actorCurrentCell(candidates[i].actor, now)
		if x == m.pendingSkill.x && y == m.pendingSkill.y {
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
	m.pendingSkill.x, m.pendingSkill.y = actorCurrentCell(candidates[index].actor, now)
	m.controllerGroundCursorX, m.controllerGroundCursorY = 0, 0
}

// confirmControllerPendingSkill completes the explicit controller cast flow:
// shortcut selects the skill, LB/RB selects an actor, and Confirm casts it.
func (m *WorldMode) confirmControllerPendingSkill(ctx client.Context) {
	if m == nil || m.pendingSkill.skill.ID == 0 || m.pendingSkill.targetID != 0 || isGroundTargetSkill(m.pendingSkill.skill) {
		return
	}
	if isSelfTargetSkill(m.pendingSkill.skill) {
		target := ctx.World.Player.ID
		if target == 0 && ctx.Session != nil {
			target = ctx.Session.CharID
		}
		if target != 0 {
			if err := m.skills().SendToID(ctx, m.pendingSkill.skill, target, "controller self"); err == nil {
				m.pendingSkill = pendingSkillTarget{}
				m.controllerSkillTargetID = 0
				m.controllerSkillLane = 0
			}
		}
		return
	}
	actor, ok := m.controllerSkillTargetActor(ctx)
	if !ok || !actorCanBeSkillTargeted(ctx, m.pendingSkill.skill, actor) {
		return
	}
	if err := m.skills().SendToID(ctx, m.pendingSkill.skill, actor.ID, "controller target"); err != nil {
		return
	}
	m.pendingSkill = pendingSkillTarget{}
	m.controllerSkillTargetID = 0
	m.controllerSkillLane = 0
}

func (m *WorldMode) selectControllerSkillLane(ctx client.Context, lane input.Action) {
	m.controllerSkillLane = lane
	m.controllerSkillTargetID = 0
	if lane == input.ActionTargetSelf {
		if ctx.World != nil {
			m.controllerSkillTargetID = ctx.World.Player.ID
		}
		return
	}
	m.cycleControllerSkillTarget(ctx, false)
}

func (m *WorldMode) controllerSkillTargetActor(ctx client.Context) (worldstate.Actor, bool) {
	if ctx.World == nil || m.controllerSkillTargetID == 0 {
		return worldstate.Actor{}, false
	}
	actor, ok := ctx.World.Actors[m.controllerSkillTargetID]
	if !ok || !m.controllerSkillCandidate(ctx, actor) {
		return worldstate.Actor{}, false
	}
	return actor, true
}

func (m *WorldMode) controllerSkillCandidate(ctx client.Context, actor worldstate.Actor) bool {
	if !actorCanBeSkillTargeted(ctx, m.pendingSkill.skill, actor) {
		return false
	}
	flags, known := skillTargetFlagsForActor(ctx, actor)
	if !known {
		return false
	}
	switch m.controllerSkillLane {
	case input.ActionTargetAlly:
		return m.pendingSkill.skill.Type&skillTargetFriend != 0 && actorRepresentsPlayer(actor) && !isLocalActor(ctx, actor.ID)
	case input.ActionTargetEnemy:
		return m.pendingSkill.skill.Type&skillTargetEnemy != 0 && !isLocalActor(ctx, actor.ID)
	case input.ActionTargetCompanion:
		return flags&(skillTargetPet|skillTargetHomun) != 0 && m.pendingSkill.skill.Type&(skillTargetPet|skillTargetHomun) != 0
	default:
		return true
	}
}

func (m *WorldMode) cycleControllerSkillTarget(ctx client.Context, reverse bool) bool {
	if ctx.World == nil || m.pendingSkill.skill.ID == 0 {
		return false
	}
	now := time.Now()
	px, py := currentPlayerCell(ctx, now)
	candidates := make([]worldstate.Actor, 0, len(ctx.World.Actors))
	for _, actor := range ctx.World.Actors {
		if !m.controllerSkillCandidate(ctx, actor) {
			continue
		}
		x, y := actorCurrentCell(actor, now)
		dx, dy := x-px, y-py
		candidates = append(candidates, actor)
		_ = dx
		_ = dy
	}
	if len(candidates) == 0 {
		return false
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	index := -1
	for i := range candidates {
		if candidates[i].ID == m.controllerSkillTargetID {
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
	m.controllerSkillTargetID = candidates[index].ID
	return true
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
	actions := ctx.ControllerActions
	if !ctx.ControllerActionsValid {
		actions = input.ResolveActions(ctx.Input, ctx.ControllerSettings())
	}
	for _, action := range []input.Action{input.ActionConfirm, input.ActionLoot, input.ActionCancel} {
		if actions.Pressed.Has(action) && (ctx.ControllerActionsValid || !ctx.Input.ControllerActionConsumed(action)) {
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

func (m *WorldMode) moveControllerVector(ctx client.Context, direction input.Direction8, vectorX, vectorY, magnitude float32) bool {
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
	if vectorX == 0 && vectorY == 0 {
		// Keyboard/D-pad compatibility callers may provide only Direction.
		// Preserve that digital command while analog callers retain their
		// vector and magnitude above.
		dx, dy := direction.Vector()
		vectorX, vectorY = float32(dx), float32(dy)
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
	if magnitude <= 0 {
		magnitude = 1
	}
	if magnitude > 1 {
		magnitude = 1
	}
	distance := int(math.Round(2 + float64(magnitude)*float64(controllerWalkHorizon-2)))
	if distance < 1 {
		distance = 1
	}
	norm := float32(math.Hypot(float64(vectorX), float64(vectorY)))
	if norm == 0 {
		return false
	}
	targetX, targetY := playerX+int(math.Round(float64(vectorX/norm)*float64(distance))), playerY+int(math.Round(float64(vectorY/norm)*float64(distance)))
	if targetX == playerX && targetY == playerY {
		return false
	}
	for _, candidate := range [][2]int{{targetX, targetY}, {playerX + int(math.Round(float64(vectorX/norm)*float64(distance-1))), playerY + int(math.Round(float64(vectorY/norm)*float64(distance-1)))}} {
		targetX, targetY = candidate[0], candidate[1]
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
	if !ctx.ControllerActionsValid && ctx.Input.ControllerActionConsumed(input.ActionMenu) {
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
