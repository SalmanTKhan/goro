package game

import (
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/network"
)

// toggleControllerSit requests the server-authoritative sit/stand transition.
// The local Sitting flag is deliberately not changed here; battle.go already
// applies the authoritative actor-action notification.
func (m *WorldMode) toggleControllerSit(ctx client.Context) bool {
	if m == nil || ctx.World == nil || ctx.Network == nil || playerIsDead(ctx) {
		return false
	}
	target := ctx.World.Player.ID
	if target == 0 && ctx.Session != nil {
		target = ctx.Session.CharID
	}
	if target == 0 {
		return false
	}
	m.stopControllerMovement(ctx, "controller sit")
	m.clearControllerCombatIntent()
	action := uint8(network.ActionSitDown)
	if ctx.World.Player.Sitting {
		action = network.ActionStandUp
	}
	return ctx.Network.SendActionRequest(target, action) == nil
}
