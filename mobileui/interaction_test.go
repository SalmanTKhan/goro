package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestControllerConsumesHUDAndEmitsSkillCommand(t *testing.T) {
	var commands input.CommandBuffer
	c := NewController(Fixture("normal"), Viewport{Width: 2400, Height: 1080}, &commands)
	if !c.ConsumeTouch(input.TouchPoint{X: int(c.Layout.SkillSlots[0].X + 10), Y: int(c.Layout.SkillSlots[0].Y + 10)}) {
		t.Fatal("skill touch was not owned by HUD")
	}
	if !c.Tap(c.Layout.SkillSlots[0].X+10, c.Layout.SkillSlots[0].Y+10) {
		t.Fatal("skill tap was not handled")
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandUseSkill || got[0].SkillID != 100 {
		t.Fatalf("unexpected command: %+v", got)
	}
}

func TestControllerTargetingAndBackCancel(t *testing.T) {
	var commands input.CommandBuffer
	c := NewController(Fixture("monster"), Viewport{Width: 2400, Height: 1080}, &commands)
	c.Tap(c.Layout.SkillSlots[1].X+10, c.Layout.SkillSlots[1].Y+10)
	if c.Navigation.Targeting.Mode != input.SkillTargetActor {
		t.Fatalf("targeting did not begin: %+v", c.Navigation.Targeting)
	}
	if !c.Back() || c.Navigation.Targeting.Mode != input.SkillTargetIdle {
		t.Fatal("back did not cancel targeting first")
	}
	if got := commands.Commands(); len(got) != 1 || got[0].Kind != input.CommandCancelAction {
		t.Fatalf("unexpected cancel commands: %+v", got)
	}
}

func TestControllerTargetingCancelButtonEmitsCancel(t *testing.T) {
	var commands input.CommandBuffer
	c := NewController(Fixture("monster"), FoldOuterViewport(), &commands)
	c.Tap(c.Layout.SkillSlots[1].X+10, c.Layout.SkillSlots[1].Y+10)
	if c.Navigation.Targeting.Mode != input.SkillTargetActor {
		t.Fatalf("targeting did not begin: %+v", c.Navigation.Targeting)
	}
	if !c.ConsumeTouch(input.TouchPoint{X: int(c.Layout.CombatCancel.X + 8), Y: int(c.Layout.CombatCancel.Y + 8)}) {
		t.Fatal("combat cancel touch was not owned")
	}
	if !c.Tap(c.Layout.CombatCancel.X+8, c.Layout.CombatCancel.Y+8) {
		t.Fatal("combat cancel tap was not handled")
	}
	if c.Navigation.Targeting.Mode != input.SkillTargetIdle {
		t.Fatal("combat cancel did not clear targeting")
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandCancelAction {
		t.Fatalf("unexpected cancel commands: %+v", got)
	}
}

func TestControllerConsumesDisabledAndMenuControls(t *testing.T) {
	model := Fixture("cooldowns")
	var commands input.CommandBuffer
	c := NewController(model, Viewport{Width: 2400, Height: 1080}, &commands)
	c.Tap(c.Layout.SkillSlots[0].X+1, c.Layout.SkillSlots[0].Y+1)
	if len(commands.Commands()) != 0 {
		t.Fatal("cooldown tap emitted a command")
	}
	c.Tap(c.Layout.Menu.X+1, c.Layout.Menu.Y+1)
	if !c.Navigation.MenuOpen {
		t.Fatal("menu tap did not open menu")
	}
	c.Layout = LayoutHUD(Viewport{Width: 2400, Height: 1080}, DefaultTokens(), model, c.Navigation)
	if !c.ConsumeTouch(input.TouchPoint{X: int(c.Layout.MenuActions[0].Rect.X + 1), Y: int(c.Layout.MenuActions[0].Rect.Y + 1)}) {
		t.Fatal("menu action was not owned")
	}
	if c.Tap(c.Layout.MenuActions[0].Rect.X+1, c.Layout.MenuActions[0].Rect.Y+1); c.Navigation.Screen != ScreenCharacter {
		t.Fatal("menu action did not navigate")
	}
	if c.Tap(1, 1) {
		t.Fatal("empty world tap was incorrectly consumed")
	}
}
