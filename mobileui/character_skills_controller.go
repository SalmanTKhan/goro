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
	c.Character, c.Skills = character, skills
	if c.Skills.Selection.HasSelection {
		index := c.Skills.Selection.SelectedIndex
		if index < 0 || index >= len(skills.Skills) {
			c.Skills.Selection = SkillSelectionModel{}
		} else {
			c.Skills.Selection.Skill = skills.Skills[index]
		}
	}
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
		maxOffset := maxf(0, c.CharacterLayout.ContentExtent-c.CharacterLayout.ContentViewport.H)
		c.CharacterOffset = clampf(c.CharacterOffset+delta, 0, maxOffset)
		c.relayout()
		return true
	}
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
