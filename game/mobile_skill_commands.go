package game

import (
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
)

func (m *WorldMode) mobileSkill(ctx client.Context, command input.PlayerCommand) bool {
	if m == nil || ctx.Network == nil || ctx.Session == nil || command.SkillID == 0 {
		return false
	}
	skill, ok := mobileSessionSkill(ctx.Session, command.SkillID)
	if !ok {
		return false
	}
	if command.Level > 0 {
		skill.Level = command.Level
	}
	switch command.Kind {
	case input.CommandUseSkill:
		return m.skills().Use(ctx, skill, "mobile") == nil
	case input.CommandUseSkillOnActor:
		if ctx.World == nil {
			return false
		}
		actor, ok := ctx.World.Actors[command.ActorID]
		if !ok {
			return false
		}
		return m.skills().UseTarget(ctx, skill, actor, "mobile") == nil
	case input.CommandUseSkillAtPosition:
		return m.skills().SendToGround(ctx, skill, int(command.Position.X), int(command.Position.Y), "mobile") == nil
	default:
		return false
	}
}

func mobileSessionSkill(s *session.Session, id uint16) (session.Skill, bool) {
	if s == nil {
		return session.Skill{}, false
	}
	for _, skill := range s.Skills.List {
		if skill.ID == id {
			return skill, true
		}
	}
	return session.Skill{}, false
}
