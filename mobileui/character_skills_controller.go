package mobileui

import "github.com/kivutar/goro/input"

type CharacterSkillsController struct {
	Viewport        Viewport
	Screen          Screen
	Character       MobileCharacterModel
	Skills          MobileSkillsModel
	CharacterLayout CharacterLayout
	SkillsLayout    SkillsLayout
	CharacterOffset float32
	SkillOffset     float32
	Sink            input.CommandSink
}

func NewCharacterSkillsController(character MobileCharacterModel, skills MobileSkillsModel, viewport Viewport) *CharacterSkillsController {
	c := &CharacterSkillsController{Viewport: viewport, Screen: ScreenCharacter, Character: character, Skills: skills}
	c.relayout()
	return c
}

func (c *CharacterSkillsController) SetModels(character MobileCharacterModel, skills MobileSkillsModel) {
	if c == nil {
		return
	}
	// Selection is controller UI state, not session projection state. Preserve
	// it across the per-frame model refresh so tapping a skill can actually
	// open and keep its description/action sheet visible.
	selection := c.Skills.Selection
	c.Character, c.Skills = character, skills
	if selection.HasSelection {
		index := selection.SelectedIndex
		if index < 0 || index >= len(skills.Skills) {
			selection = SkillSelectionModel{}
		} else {
			selection.Skill = skills.Skills[index]
		}
	}
	c.Skills.Selection = selection
	c.relayout()
}

func (c *CharacterSkillsController) Open(screen Screen) {
	if c == nil {
		return
	}
	if screen != ScreenCharacter && screen != ScreenSkills {
		screen = ScreenCharacter
	}
	c.Screen = screen
	c.relayout()
}

func (c *CharacterSkillsController) Resize(viewport Viewport) {
	if c == nil {
		return
	}
	c.Viewport = viewport
	c.relayout()
}

func (c *CharacterSkillsController) ConsumeTouch(point input.TouchPoint) bool {
	_ = point
	return c != nil && c.Screen != ScreenWorldHUD
}

func (c *CharacterSkillsController) Tap(x, y float32) bool {
	if c == nil || c.Screen == ScreenWorldHUD {
		return false
	}
	if c.CharacterLayout.BackButton.Contains(x, y) || c.SkillsLayout.BackButton.Contains(x, y) {
		c.Screen = ScreenWorldHUD
		c.relayout()
		return true
	}
	if c.Screen == ScreenCharacter {
		if c.CharacterLayout.SkillsButton.Contains(x, y) {
			c.Screen = ScreenSkills
			c.relayout()
			return true
		}
		return true
	}
	if c.SkillsLayout.CharacterButton.Contains(x, y) {
		c.Screen = ScreenCharacter
		c.relayout()
		return true
	}
	if c.Skills.Selection.HasSelection && c.SkillsLayout.HotbarButton.Contains(x, y) {
		if c.Sink != nil {
			c.Sink.Emit(input.PlayerCommand{Kind: input.CommandAssignSkillHotkey, SkillID: c.Skills.Selection.Skill.SkillID})
		}
		return true
	}
	for index, button := range c.SkillsLayout.UpgradeButtons {
		if button.W <= 0 || !button.Contains(x, y) || index >= len(c.Skills.Skills) {
			continue
		}
		if c.Sink != nil {
			c.Sink.Emit(input.PlayerCommand{Kind: input.CommandUpgradeSkill, SkillID: c.Skills.Skills[index].SkillID})
		}
		return true
	}
	for index, row := range c.SkillsLayout.Rows {
		if row.Contains(x, y) && index < len(c.Skills.Skills) {
			c.Skills.Selection = SkillSelectionModel{SelectedIndex: index, HasSelection: true, Skill: c.Skills.Skills[index]}
			c.relayout()
			return true
		}
	}
	return true
}

func (c *CharacterSkillsController) ScrollBy(delta float32) bool {
	if c == nil || (c.Screen != ScreenSkills && c.Screen != ScreenCharacter) {
		return false
	}
	if c.Screen == ScreenCharacter {
		before := c.CharacterOffset
		maxOffset := maxf(0, c.CharacterLayout.ContentExtent-c.CharacterLayout.ContentViewport.H)
		c.CharacterOffset = clampf(c.CharacterOffset+delta, 0, maxOffset)
		if c.CharacterOffset == before {
			return false
		}
		c.relayout()
		return true
	}
	before := c.SkillOffset
	viewportExtent := c.SkillsLayout.ListViewport.H - 16
	if viewportExtent < 0 {
		viewportExtent = 0
	}
	contentExtent := float32(len(c.Skills.Skills) * 64)
	maxOffset := contentExtent - viewportExtent
	if maxOffset < 0 {
		maxOffset = 0
	}
	c.SkillOffset += delta
	if c.SkillOffset < 0 {
		c.SkillOffset = 0
	}
	if c.SkillOffset > maxOffset {
		c.SkillOffset = maxOffset
	}
	if c.SkillOffset == before {
		return false
	}
	c.relayout()
	return true
}

func (c *CharacterSkillsController) Back() bool {
	if c == nil || c.Screen == ScreenWorldHUD {
		return false
	}
	c.Screen = ScreenWorldHUD
	c.relayout()
	return true
}

func (c *CharacterSkillsController) relayout() {
	if c == nil {
		return
	}
	c.CharacterLayout = LayoutCharacterScrolled(c.Viewport, c.Character, c.CharacterOffset)
	c.SkillsLayout = LayoutSkills(c.Viewport, c.Skills, c.SkillOffset)
}
