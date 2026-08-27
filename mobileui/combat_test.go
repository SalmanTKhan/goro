package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestProjectCombatTargetingStates(t *testing.T) {
	hud := Fixture("monster")
	actor := Navigation{}
	actor.Targeting.BeginActor(101, 2)
	model := ProjectCombat(hud, actor)
	if model.Phase != CombatTargetingActor || model.TargetingMode != input.SkillTargetActor || model.PendingSkillID != 101 || model.Prompt != "SELECT TARGET" || !model.CancelAvailable {
		t.Fatalf("unexpected actor targeting model: %+v", model)
	}
	ground := Navigation{}
	ground.Targeting.BeginGround(102, 1)
	model = ProjectCombat(hud, ground)
	if model.Phase != CombatTargetingGround || model.Prompt != "SELECT AREA" {
		t.Fatalf("unexpected ground targeting model: %+v", model)
	}
	idle := ProjectCombat(hud, Navigation{})
	if idle.Phase != CombatReady || idle.CancelAvailable || idle.Prompt != "" {
		t.Fatalf("unexpected idle combat model: %+v", idle)
	}
}
