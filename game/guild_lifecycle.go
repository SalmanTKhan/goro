package game

import (
	"strings"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/network"
	"github.com/kivutar/goro/session"
	gameui "github.com/kivutar/goro/ui"
)

const (
	guildPermissionExpel = 0x10
	guildReasonMaxLength = 39
)

func (m *WorldMode) openGuildMemberPrompt(ctx client.Context, action gameui.GuildMemberAction) {
	if !guildMemberActionAllowed(ctx.Session, action) {
		return
	}
	m.guildAction = action
	switch action.Kind {
	case gameui.GuildMemberActionLeave:
		m.ui.guildMemberPrompt.Open(ctx, "Leave Guild", "Reason", "Reason for leaving", guildReasonMaxLength)
	case gameui.GuildMemberActionExpel:
		m.ui.guildMemberPrompt.Open(ctx, "Expel Guild Member", "Reason", "Reason for expulsion", guildReasonMaxLength)
	default:
		m.guildAction = gameui.GuildMemberAction{}
	}
}

func guildMemberActionAllowed(s *session.Session, action gameui.GuildMemberAction) bool {
	if s == nil || localGuildIDFromSession(s) == 0 {
		return false
	}
	isSelf := action.Member.AccountID == s.AccountID && action.Member.CharID == s.CharID
	switch action.Kind {
	case gameui.GuildMemberActionLeave:
		return isSelf && !s.Guild.IsMaster
	case gameui.GuildMemberActionExpel:
		return !isSelf && (s.Guild.IsMaster || s.Guild.Right&guildPermissionExpel != 0)
	default:
		return false
	}
}

func (m *WorldMode) updateGuildMemberPrompt(ctx client.Context) bool {
	consumed := m.ui.guildMemberPrompt.Update(ctx)
	action := m.ui.guildMemberPrompt.PopAction()
	if action.Submitted {
		m.sendPendingGuildMemberAction(ctx, action.Text)
		return true
	}
	if m.guildAction.Kind != gameui.GuildMemberActionNone && !m.ui.guildMemberPrompt.IsOpen() {
		m.guildAction = gameui.GuildMemberAction{}
	}
	return consumed
}

func (m *WorldMode) sendPendingGuildMemberAction(ctx client.Context, reason string) {
	action := m.guildAction
	m.guildAction = gameui.GuildMemberAction{}
	if !guildMemberActionAllowed(ctx.Session, action) {
		return
	}
	if ctx.Network == nil {
		m.ui.console.AddErrorMessage("Guild member update failed: not connected.")
		return
	}
	guildID := localGuildIDFromSession(ctx.Session)
	var err error
	switch action.Kind {
	case gameui.GuildMemberActionLeave:
		err = ctx.Network.SendLeaveGuild(guildID, ctx.Session.AccountID, ctx.Session.CharID, reason)
	case gameui.GuildMemberActionExpel:
		err = ctx.Network.SendExpelGuildMember(guildID, action.Member.AccountID, action.Member.CharID, reason)
	}
	if err != nil {
		glog.Warnf("guild member update failed kind=%d guild=%d account=%d char=%d: %v", action.Kind, guildID, action.Member.AccountID, action.Member.CharID, err)
		m.ui.console.AddErrorMessage("Guild member update failed.")
	}
}

func localGuildIDFromSession(s *session.Session) uint32 {
	if s == nil {
		return 0
	}
	if s.Guild.ID != 0 {
		return s.Guild.ID
	}
	return s.GuildID
}

func (m *WorldMode) handleGuildMemberDeparture(ctx client.Context, departure network.GuildMemberDeparture) {
	member, local := removeLocalGuildMemberByName(ctx, departure.CharName)
	m.ui.console.AddGuildMessage("%s has withdrawn from the guild.", guildDisplayName(departure.CharName))
	if reason := strings.TrimSpace(departure.Reason); reason != "" {
		m.ui.console.AddGuildMessage("Secession reason: %s", reason)
	}
	if member.AccountID != 0 {
		m.ui.minimap.ApplyGuildMemberPosition(member.AccountID, -1, -1)
	}
	clearVisibleGuildMember(ctx, member, departure.CharName)
	if local {
		m.clearLocalGuildState(ctx, false)
		return
	}
	m.ui.guildWindow.Refresh(ctx)
}

func (m *WorldMode) handleGuildMemberExpulsion(ctx client.Context, expulsion network.GuildMemberExpulsion) {
	member, local := removeLocalGuildMemberByName(ctx, expulsion.CharName)
	m.ui.console.AddGuildMessage("%s has been expelled from the guild.", guildDisplayName(expulsion.CharName))
	if reason := strings.TrimSpace(expulsion.Reason); reason != "" {
		m.ui.console.AddGuildMessage("Expulsion reason: %s", reason)
	}
	if member.AccountID != 0 {
		m.ui.minimap.ApplyGuildMemberPosition(member.AccountID, -1, -1)
	}
	clearVisibleGuildMember(ctx, member, expulsion.CharName)
	if local {
		m.clearLocalGuildState(ctx, false)
		return
	}
	m.ui.guildWindow.Refresh(ctx)
}

func (m *WorldMode) handleGuildDisbandResult(ctx client.Context, result network.GuildDisbandResult) {
	switch result.Result {
	case 0:
		m.ui.console.AddBlueMessage("The guild has been disbanded.")
		m.clearLocalGuildState(ctx, true)
	case 1:
		m.ui.console.AddErrorMessage("The guild name is incorrect.")
	case 2:
		m.ui.console.AddErrorMessage("Expel every other member before disbanding the guild.")
	default:
		m.ui.console.AddErrorMessage("Guild disband failed.")
	}
}

func removeLocalGuildMemberByName(ctx client.Context, name string) (session.GuildMember, bool) {
	if ctx.Session == nil {
		return session.GuildMember{}, false
	}
	name = strings.TrimSpace(name)
	local := name != "" && strings.EqualFold(name, strings.TrimSpace(ctx.Session.SelectedCharacter().Name))
	if !local && ctx.World != nil {
		local = strings.EqualFold(name, strings.TrimSpace(ctx.World.Player.Name))
	}
	members := ctx.Session.Guild.Members
	for i, member := range members {
		if !strings.EqualFold(strings.TrimSpace(member.CharName), name) {
			continue
		}
		copy(members[i:], members[i+1:])
		ctx.Session.Guild.Members = members[:len(members)-1]
		recountGuildOnlineMembers(&ctx.Session.Guild)
		return member, local || member.AccountID == ctx.Session.AccountID && member.CharID == ctx.Session.CharID
	}
	return session.GuildMember{}, local
}

func clearVisibleGuildMember(ctx client.Context, member session.GuildMember, name string) {
	if ctx.World == nil {
		return
	}
	name = strings.TrimSpace(name)
	for id, actor := range ctx.World.Actors {
		matchesID := member.AccountID != 0 && (id == member.AccountID || actor.ID == member.AccountID)
		matchesID = matchesID || member.CharID != 0 && (id == member.CharID || actor.ID == member.CharID)
		if !matchesID && (name == "" || !strings.EqualFold(strings.TrimSpace(actor.Name), name)) {
			continue
		}
		actor.GuildID = 0
		actor.EmblemVersion = 0
		actor.GuildName = ""
		ctx.World.Actors[id] = actor
	}
}

func (m *WorldMode) clearLocalGuildState(ctx client.Context, disbanded bool) {
	oldGuildID := localGuildIDFromSession(ctx.Session)
	m.guildOpenPending = false
	if ctx.Session != nil {
		ctx.Session.GuildID = 0
		ctx.Session.EmblemVersion = 0
		ctx.Session.GuildName = ""
		ctx.Session.Guild = session.Guild{}
		ctx.Session.PendingGuildName = ""
	}
	if ctx.World != nil {
		ctx.World.Player.GuildID = 0
		ctx.World.Player.EmblemVersion = 0
		ctx.World.Player.GuildName = ""
		if disbanded && oldGuildID != 0 {
			for id, actor := range ctx.World.Actors {
				if actor.GuildID != oldGuildID {
					continue
				}
				actor.GuildID = 0
				actor.EmblemVersion = 0
				actor.GuildName = ""
				ctx.World.Actors[id] = actor
			}
		}
	}
	m.ui.minimap.ClearGuildMemberPositions()
	m.guildAction = gameui.GuildMemberAction{}
	m.ui.guildMemberPrompt.Close()
	m.ui.guildWindow.Close()
}
