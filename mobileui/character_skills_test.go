package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestCharacterAndSkillsProjection(t *testing.T) {
	character := FixtureCharacter("character-rich")
	if character.Name != "Goro" || character.JobName == "" || character.BaseLevel != 42 || character.JobLevel != 31 {
		t.Fatalf("unexpected character projection: %+v", character)
	}
	if len(character.Stats) != 6 || character.Stats[0].Label != "STR" || character.Attack != 87 || character.Zeny != 12500 {
		t.Fatalf("character stats were not projected: %+v", character)
	}
	skills := FixtureSkills("skills-basic")
	if len(skills.Skills) != 6 || skills.Points != 3 || skills.Skills[0].TargetMode != input.SkillTargetActor || !skills.Skills[0].Upgradable {
		t.Fatalf("unexpected skills projection: %+v", skills)
	}
	if skills.Skills[1].TargetMode != input.SkillTargetGround || skills.Skills[1].IconKey == "" {
		t.Fatalf("skill target/icon projection failed: %+v", skills.Skills[1])
	}
}

func TestCharacterSkillsViewportMatrix(t *testing.T) {
	viewports := []Viewport{
		{Width: 1920, Height: 1080},
		{Width: 2400, Height: 1080},
		{Width: 2560, Height: 1440},
		FoldOuterViewport(),
		{Width: 2268, Height: 832, SafeLeft: 24, SafeRight: 24, SafeTop: 18, SafeBottom: 18},
	}
	for _, viewport := range viewports {
		character := LayoutCharacter(viewport, FixtureCharacter("character-rich"))
		if !insideSafe(character.Panel, character.Safe) || character.SkillsButton.W < 48 || character.SkillsButton.H < 48 {
			t.Errorf("character layout escaped or missed touch target: viewport=%+v layout=%+v", viewport, character)
		}
		skillsModel := FixtureSkills("skills-many")
		skills := LayoutSkills(viewport, skillsModel, 0)
		if !insideSafe(skills.Panel, skills.Safe) || skills.BackButton.H < 48 || skills.CharacterButton.H < 48 || skills.ListViewport.W <= 0 {
			t.Errorf("skills layout invalid: viewport=%+v layout=%+v", viewport, skills)
		}
		skillsModel.Selection = SkillSelectionModel{HasSelection: true, SelectedIndex: 0, Skill: skillsModel.Skills[0]}
		selected := LayoutSkills(viewport, skillsModel, 0)
		if selected.Detail.W <= 0 || !insideSafe(selected.Detail, selected.Safe) {
			t.Errorf("selected skill did not receive a contained detail sheet: viewport=%+v layout=%+v", viewport, selected)
		}
	}
}

func TestPhoneLandscapeUsesWorkspaceAndFoldKeepsSplitPanes(t *testing.T) {
	phoneLandscape := Viewport{Width: 2400, Height: 1080}
	character := LayoutCharacter(phoneLandscape, FixtureCharacter("character-rich"))
	if character.Stacked || character.Panel.W < 0.9*character.Safe.W {
		t.Fatalf("phone landscape should use a full workspace: %+v", character)
	}
	if character.Combat.X <= character.Stats.X || character.Combat.Y != character.Stats.Y {
		t.Fatalf("phone landscape character cards are not a horizontal workspace: stats=%+v combat=%+v content=%+v", character.Stats, character.Combat, character.ContentViewport)
	}

	skillsModel := FixtureSkills("skills-many")
	skills := LayoutSkills(phoneLandscape, skillsModel, 0)
	if skills.Stacked || skills.Detail.W <= 0 || skills.Panel.W < 0.9*skills.Safe.W {
		t.Fatalf("unselected phone landscape skills should use a workspace: list=%+v detail=%+v", skills.ListViewport, skills.Detail)
	}
	skillsModel.Selection = SkillSelectionModel{HasSelection: true, SelectedIndex: 0, Skill: skillsModel.Skills[0]}
	selectedSkills := LayoutSkills(phoneLandscape, skillsModel, 0)
	if selectedSkills.Detail.W <= 0 || selectedSkills.Detail.X <= selectedSkills.ListViewport.X {
		t.Fatalf("selected phone landscape skill detail is not a side pane: list=%+v detail=%+v", selectedSkills.ListViewport, selectedSkills.Detail)
	}

	fold := FoldOuterViewport()
	foldCharacter := LayoutCharacter(fold, FixtureCharacter("character-rich"))
	if foldCharacter.Stacked || foldCharacter.Combat.X <= foldCharacter.Stats.X {
		t.Fatalf("fold outer should retain split character panes: %+v", foldCharacter)
	}
	foldSkills := LayoutSkills(fold, FixtureSkills("skills-many"), 0)
	if foldSkills.Stacked || foldSkills.Detail.X <= foldSkills.ListViewport.X {
		t.Fatalf("fold outer should retain split skill panes: %+v", foldSkills)
	}
}

func TestCharacterSkillsControllerSelectionNavigationAndScroll(t *testing.T) {
	viewport := FoldOuterViewport()
	c := NewCharacterSkillsController(FixtureCharacter("character-rich"), FixtureSkills("skills-many"), viewport)
	if !c.ConsumeTouch(input.TouchPoint{X: 1, Y: 1}) {
		t.Fatal("character screen did not own touch")
	}
	if !c.Tap(c.CharacterLayout.SkillsButton.X+1, c.CharacterLayout.SkillsButton.Y+1) || c.Screen != ScreenSkills {
		t.Fatal("character to skills navigation failed")
	}
	row := c.SkillsLayout.Rows[0]
	if !c.Tap(row.X+2, row.Y+2) || !c.Skills.Selection.HasSelection || c.Skills.Selection.SelectedIndex != 0 {
		t.Fatalf("skill selection failed: %+v", c.Skills.Selection)
	}
	if !c.ScrollBy(10_000) || c.SkillOffset <= 0 {
		t.Fatalf("skill scroll did not advance: offset=%f", c.SkillOffset)
	}
	if c.SkillOffset > float32(len(c.Skills.Skills)*64) {
		t.Fatalf("skill scroll was not clamped: offset=%f", c.SkillOffset)
	}
	if !c.Tap(c.SkillsLayout.CharacterButton.X+1, c.SkillsLayout.CharacterButton.Y+1) || c.Screen != ScreenCharacter {
		t.Fatal("skills to character navigation failed")
	}
	if !c.Back() || c.Screen != ScreenWorldHUD || c.ConsumeTouch(input.TouchPoint{X: 1, Y: 1}) {
		t.Fatal("character back did not return world ownership")
	}
}

func insideSafe(rect, safe Rect) bool {
	return rect.X >= safe.X && rect.Right() <= safe.Right() && rect.Y >= safe.Y && rect.Bottom() <= safe.Bottom()
}
