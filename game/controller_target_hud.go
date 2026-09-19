package game

import (
	"fmt"
	"image/color"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/render"
	"github.com/kivutar/goro/ui/inputprompt"
)

func (m *WorldMode) drawControllerTargetHUD(screen *render.Frame, ctx client.Context) {
	if m == nil || screen == nil || ctx.Input == nil || ctx.Input.InputSource() != input.InputSourceController || m.pendingSkill.skill.ID == 0 {
		return
	}
	r := inputprompt.NewResolver()
	settings := ctx.ControllerSettings()
	kind := ctx.Input.Controller().Kind
	lane := "Target"
	if isGroundTargetSkill(m.pendingSkill.skill) {
		confirm := r.ForAction(settings, kind, input.ActionConfirm).Text
		cancel := r.ForAction(settings, kind, input.ActionCancel).Text
		text := fmt.Sprintf("Ground Target   Right Stick Move   %s Cast   %s Cancel", confirm, cancel)
		b := screen.Bounds()
		x := b.Min.X + b.Dx()/2 - 180
		if x < b.Min.X+12 {
			x = b.Min.X + 12
		}
		render.DrawOutlinedTextAt(screen, text, x, b.Min.Y+76, color.RGBA{R: 255, G: 255, B: 255, A: 255}, color.RGBA{R: 20, G: 28, B: 45, A: 255})
		return
	}
	switch m.controllerSkillLane {
	case input.ActionTargetSelf:
		lane = "Self"
	case input.ActionTargetAlly:
		lane = "Ally"
	case input.ActionTargetEnemy:
		lane = "Enemy"
	case input.ActionTargetCompanion:
		lane = "Companion"
	}
	prev := r.ForAction(settings, kind, input.ActionTargetPrevious).Text
	next := r.ForAction(settings, kind, input.ActionTargetNext).Text
	confirm := r.ForAction(settings, kind, input.ActionConfirm).Text
	cancel := r.ForAction(settings, kind, input.ActionCancel).Text
	text := fmt.Sprintf("Target: %s   %s/%s Cycle   %s Cast   %s Cancel", lane, prev, next, confirm, cancel)
	b := screen.Bounds()
	// Keep the target hint under the shortcut bar and centered. The basic info
	// panel occupies the upper-left corner of the world HUD.
	x := b.Min.X + b.Dx()/2 - 180
	if x < b.Min.X+12 {
		x = b.Min.X + 12
	}
	y := b.Min.Y + 76
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	outline := color.RGBA{R: 20, G: 28, B: 45, A: 255}
	render.DrawOutlinedTextAt(screen, text, x, y, white, outline)
}
