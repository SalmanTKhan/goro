package mobileui

import "github.com/kivutar/goro/input"

// CombatPhase describes the presentation state of the mobile combat HUD. It
// does not own attack, skill, cooldown, or damage rules.
type CombatPhase uint8

const (
	CombatReady CombatPhase = iota
	CombatTargetingActor
	CombatTargetingGround
)

// MobileCombatModel is a read-only projection of the current HUD target and
// navigation targeting state. It contains no packet or renderer types.
type MobileCombatModel struct {
	Target            TargetHUDModel
	Phase             CombatPhase
	TargetingMode     input.SkillTargetMode
	PendingSkillID    uint16
	PendingSkillLevel int
	Prompt            string
	CancelAvailable   bool
}

func ProjectCombat(hud MobileHUDModel, navigation Navigation) MobileCombatModel {
	model := MobileCombatModel{Target: hud.Target}
	switch navigation.Targeting.Mode {
	case input.SkillTargetActor:
		model.Phase = CombatTargetingActor
	case input.SkillTargetGround:
		model.Phase = CombatTargetingGround
	default:
		return model
	}
	model.TargetingMode = navigation.Targeting.Mode
	model.PendingSkillID = navigation.Targeting.SkillID
	model.PendingSkillLevel = navigation.Targeting.Level
	model.Prompt = TargetingPrompt(navigation.Targeting.Mode)
	model.CancelAvailable = true
	return model
}

func TargetingPrompt(mode input.SkillTargetMode) string {
	switch mode {
	case input.SkillTargetActor:
		return "SELECT TARGET"
	case input.SkillTargetGround:
		return "SELECT AREA"
	default:
		return ""
	}
}
