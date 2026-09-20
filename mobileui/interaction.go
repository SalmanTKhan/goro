package mobileui

import "github.com/kivutar/goro/input"

type Controller struct {
	Model      MobileHUDModel
	Layout     HUDLayout
	Navigation Navigation
	Sink       input.CommandSink
	viewport   Viewport
	tokens     MobileTokens
}

func NewController(model MobileHUDModel, viewport Viewport, sink input.CommandSink) *Controller {
	c := &Controller{Model: model, Sink: sink, viewport: viewport, tokens: DefaultTokens()}
	c.Layout = LayoutHUD(viewport, c.tokens, model, c.Navigation)
	return c
}

func (c *Controller) ConsumeTouch(point input.TouchPoint) bool {
	return c != nil && c.Layout.ConsumeTouch(point)
}

func (c *Controller) Tap(x, y float32) bool {
	if c == nil {
		return false
	}
	hit := c.Layout.HitTest(x, y)
	switch hit.Control {
	case ControlNone:
		return false
	case ControlMenu:
		c.Navigation.MenuOpen = !c.Navigation.MenuOpen
		if c.Navigation.MenuOpen {
			c.Navigation.EmoteOpen = false
		}
		c.relayout()
		return true
	case ControlMenuAction:
		c.Navigation.Open(hit.Screen)
		c.relayout()
		return true
	case ControlCancelAction:
		if c.Navigation.Targeting.Mode == input.SkillTargetIdle {
			return true
		}
		c.Navigation.Targeting.Cancel()
		c.emit(input.PlayerCommand{Kind: input.CommandCancelAction})
		c.relayout()
		return true
	case ControlSkillPagePrev:
		if c.Navigation.SkillPage > 0 {
			c.Navigation.SkillPage--
			c.relayout()
		}
		return true
	case ControlSkillPageNext:
		perPage := c.Layout.SkillsPerPage
		if perPage <= 0 {
			perPage = 4
		}
		if len(c.Model.Skills) > 0 && c.Navigation.SkillPage < (len(c.Model.Skills)-1)/perPage {
			c.Navigation.SkillPage++
			c.relayout()
		}
		return true
	case ControlSkill:
		skillIndex := hit.SkillIndex + c.Layout.SkillStart
		if skillIndex >= len(c.Model.Skills) {
			return true
		}
		skill := c.Model.Skills[skillIndex]
		if !skill.Usable || skill.SkillID == 0 || skill.CooldownRemaining > 0 {
			return true
		}
		switch skill.TargetMode {
		case input.SkillTargetActor:
			c.Navigation.Targeting.BeginActor(skill.SkillID, skill.Level)
		case input.SkillTargetGround:
			c.Navigation.Targeting.BeginGround(skill.SkillID, skill.Level)
		default:
			c.emit(input.PlayerCommand{Kind: input.CommandUseSkill, SkillID: skill.SkillID, Level: skill.Level})
		}
		c.relayout()
		return true
	case ControlSit:
		c.emit(input.PlayerCommand{Kind: input.CommandToggleSit})
		return true
	case ControlLoot:
		if len(c.Model.Loot) > 0 {
			c.emit(input.PlayerCommand{Kind: input.CommandLootFocused})
		}
		return true
	case ControlEmoteToggle:
		if len(c.Model.Emotes) == 0 {
			return true
		}
		c.Navigation.EmoteOpen = !c.Navigation.EmoteOpen
		if c.Navigation.EmoteOpen {
			c.Navigation.MenuOpen = false
		}
		c.relayout()
		return true
	case ControlEmote:
		if hit.EmoteIndex < 0 || hit.EmoteIndex >= len(c.Model.Emotes) {
			return true
		}
		emote := c.Model.Emotes[hit.EmoteIndex]
		c.emit(input.PlayerCommand{Kind: input.CommandEmotion, EmotionID: emote.ID})
		c.Navigation.EmoteOpen = false
		c.relayout()
		return true
	case ControlPrimaryAction, ControlTarget:
		target := c.Model.Target
		if !target.Visible || target.ID == 0 {
			return true
		}
		switch target.Relation {
		case TargetHostile:
			c.emit(input.PlayerCommand{Kind: input.CommandAttackActor, ActorID: target.ID})
		case TargetNPC:
			c.emit(input.PlayerCommand{Kind: input.CommandInteractActor, ActorID: target.ID})
		default:
			c.emit(input.PlayerCommand{Kind: input.CommandSelectActor, ActorID: target.ID})
		}
		return true
	case ControlLootItem:
		if hit.LootIndex < 0 || hit.LootIndex >= len(c.Model.Loot) {
			return true
		}
		loot := c.Model.Loot[hit.LootIndex]
		if loot.DropID == 0 {
			return true
		}
		c.emit(input.PlayerCommand{Kind: input.CommandPickUpItem, ItemID: loot.DropID})
		return true
	default:
		return true
	}
}

func (c *Controller) Back() bool {
	if c == nil {
		return false
	}
	wasTargeting := c.Navigation.Targeting.Mode != input.SkillTargetIdle
	if !c.Navigation.Back() {
		return false
	}
	if wasTargeting {
		c.emit(input.PlayerCommand{Kind: input.CommandCancelAction})
	}
	c.relayout()
	return true
}

func (c *Controller) emit(command input.PlayerCommand) {
	if c.Sink != nil {
		c.Sink.Emit(command)
	}
}
func (c *Controller) relayout() {
	c.Layout = LayoutHUD(c.viewport, c.tokens, c.Model, c.Navigation)
}
