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


func TestControllerPrimaryActionAttacksHostileTarget(t *testing.T) {
	var commands input.CommandBuffer
	c := NewController(Fixture("monster"), FoldOuterViewport(), &commands)
	if c.Layout.PrimaryAction.W < DefaultTokens().MinTouchTarget || c.Layout.PrimaryAction.H < DefaultTokens().MinTouchTarget {
		t.Fatalf("primary action is not touch-safe: %+v", c.Layout.PrimaryAction)
	}
	if !c.ConsumeTouch(input.TouchPoint{X: int(c.Layout.PrimaryAction.X + 4), Y: int(c.Layout.PrimaryAction.Y + 4)}) {
		t.Fatal("primary action touch was not owned by HUD")
	}
	if !c.Tap(c.Layout.PrimaryAction.X+4, c.Layout.PrimaryAction.Y+4) {
		t.Fatal("primary action tap was not handled")
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandAttackActor || got[0].ActorID != 9001 {
		t.Fatalf("unexpected primary action command: %+v", got)
	}
}

func TestControllerTargetFrameUsesRelationAction(t *testing.T) {
	model := Fixture("long-target")
	var commands input.CommandBuffer
	c := NewController(model, FoldOuterViewport(), &commands)
	if !c.Tap(c.Layout.TargetPanel.X+4, c.Layout.TargetPanel.Y+4) {
		t.Fatal("target frame tap was not handled")
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandInteractActor || got[0].ActorID != model.Target.ID {
		t.Fatalf("unexpected target-frame command: %+v", got)
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

func TestControllerWorldUtilityActionsEmitSemanticCommands(t *testing.T) {
	model := Fixture("loot-basic")
	model.Emotes = []EmoteModel{{ID: 15, Label: "thx"}}
	var commands input.CommandBuffer
	controller := NewController(model, FoldOuterViewport(), &commands)

	if !controller.Tap(controller.Layout.SitAction.X+4, controller.Layout.SitAction.Y+4) {
		t.Fatal("sit action was not handled")
	}
	if !controller.Tap(controller.Layout.LootAction.X+4, controller.Layout.LootAction.Y+4) {
		t.Fatal("loot action was not handled")
	}
	got := commands.Commands()
	if len(got) != 2 || got[0].Kind != input.CommandToggleSit || got[1].Kind != input.CommandLootFocused {
		t.Fatalf("world utility commands = %+v", got)
	}

	if !controller.Tap(controller.Layout.EmoteAction.X+4, controller.Layout.EmoteAction.Y+4) || !controller.Navigation.EmoteOpen {
		t.Fatal("emote action did not open quick picker")
	}
	if len(controller.Layout.EmoteRows) != 1 {
		t.Fatalf("emote rows = %d, want one", len(controller.Layout.EmoteRows))
	}
	row := controller.Layout.EmoteRows[0]
	if !controller.Tap(row.X+4, row.Y+4) {
		t.Fatal("emote row was not handled")
	}
	got = commands.Commands()
	if len(got) != 3 || got[2].Kind != input.CommandEmotion || got[2].EmotionID != 15 {
		t.Fatalf("emote command = %+v", got)
	}
	if controller.Navigation.EmoteOpen {
		t.Fatal("emote picker stayed open after selection")
	}
}

func TestControllerBackClosesQuickEmotesBeforeLeavingHUD(t *testing.T) {
	model := Fixture("normal")
	model.Emotes = []EmoteModel{{ID: 0, Label: "!"}}
	controller := NewController(model, FoldOuterViewport(), nil)
	controller.Tap(controller.Layout.EmoteAction.X+4, controller.Layout.EmoteAction.Y+4)
	if !controller.Navigation.EmoteOpen {
		t.Fatal("emote picker did not open")
	}
	if !controller.Back() || controller.Navigation.EmoteOpen {
		t.Fatal("back did not close emote picker")
	}
	if controller.Navigation.Screen != ScreenWorldHUD {
		t.Fatalf("back left world HUD: %v", controller.Navigation.Screen)
	}
}

func TestControllerUnavailableEmotesStayClosed(t *testing.T) {
	controller := NewController(Fixture("normal"), FoldOuterViewport(), nil)
	if !controller.Tap(controller.Layout.EmoteAction.X+4, controller.Layout.EmoteAction.Y+4) {
		t.Fatal("disabled emote control did not consume its touch")
	}
	if controller.Navigation.EmoteOpen {
		t.Fatal("disabled emote control opened an empty picker")
	}
}


func TestProgressionAlertsOpenCharacterAndSkills(t *testing.T) {
	viewport := Viewport{Width: 840, Height: 2289, SafeTop: 48, SafeBottom: 96}
	c := NewController(Fixture("progression"), viewport, nil)

	level := c.Layout.LevelUpAction
	if !c.Tap(level.X+2, level.Y+2) || c.Navigation.Screen != ScreenCharacter {
		t.Fatalf("level-up did not open character screen: nav=%+v", c.Navigation)
	}

	c.Navigation.Open(ScreenWorldHUD)
	c.relayout()
	skill := c.Layout.SkillUpAction
	if !c.Tap(skill.X+2, skill.Y+2) || c.Navigation.Screen != ScreenSkills {
		t.Fatalf("skill-up did not open skills screen: nav=%+v", c.Navigation)
	}
}
