package ui

import (
	"strings"

	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
)

const partyContextMenuWidth = 132

type PartyContextMenu struct {
	Window
	member    session.PartyMember
	canManage bool
	isSelf    bool
	action    FriendsWindowAction
}

func (m *PartyContextMenu) Open(ctx Context, x, y int, member session.PartyMember, canManage, isSelf bool) {
	name := strings.TrimSpace(member.Name)
	if name == "" {
		return
	}
	m.EnsureWindow(partyContextMenuWidth, m.height())
	m.titleHeight = 0
	m.ctx = ctx
	m.member = member
	m.canManage = canManage
	m.isSelf = isSelf
	screenW, screenH := ctx.UIScreenSize()
	height := m.height()
	m.SetSize(partyContextMenuWidth, height)
	x = clampWindowInt(x, windowScreenMargin, maxInt(windowScreenMargin, screenW-partyContextMenuWidth-windowScreenMargin))
	y = clampWindowInt(y, windowScreenMargin, maxInt(windowScreenMargin, screenH-height-windowScreenMargin))
	m.OpenAt(x, y, m.widgetTree())
	m.Publish(ctx)
}

func (m *PartyContextMenu) Update(ctx Context) bool {
	m.EnsureWindow(partyContextMenuWidth, m.height())
	m.titleHeight = 0
	m.ctx = ctx
	if !m.IsOpen() {
		return false
	}
	if ctx.Input != nil {
		inside := pointInRect(ctx.Input.MouseX, ctx.Input.MouseY, m.x, m.y, partyContextMenuWidth, m.height())
		if ctx.Input.JustPressed(input.KeyEscape) || (!inside && (ctx.Input.MouseJustPressed(input.MouseButtonLeft) || ctx.Input.MouseJustPressed(input.MouseButtonRight))) {
			m.Close()
			return true
		}
	}
	consumed := m.Window.Update(ctx)
	m.Publish(ctx)
	return consumed
}

func (m *PartyContextMenu) Rebind(ctx Context) {
	if !m.IsOpen() {
		return
	}
	m.ctx = ctx
	m.SetContent(m.widgetTree())
	m.Publish(ctx)
}

func (m *PartyContextMenu) PopAction() FriendsWindowAction {
	action := m.action
	m.action = FriendsWindowAction{}
	return action
}

func (m *PartyContextMenu) widgetTree() widget.Widget {
	rows := []widget.Widget{
		m.button("Information", FriendsWindowActionPartyMemberInfo),
	}
	if !m.isSelf {
		rows = append(rows, m.button("1:1 Chat", FriendsWindowActionPartyMemberWhisper))
	}
	if m.canManage && !m.isSelf {
		rows = append(rows, m.button("Expel", FriendsWindowActionPartyMemberExpel))
	}
	if m.isSelf {
		rows = append(rows, m.button("Leave Party", FriendsWindowActionPartyLeave))
	}
	return contextMenu(partyContextMenuWidth, rows...)
}

func (m *PartyContextMenu) button(label string, kind FriendsWindowActionKind) widget.Widget {
	return contextMenuItem(label, func() {
		m.action = FriendsWindowAction{Kind: kind, PartyMember: m.member}
		m.Close()
	})
}

func (m *PartyContextMenu) height() int {
	rows := 1
	if !m.isSelf {
		rows++
	}
	if m.canManage && !m.isSelf {
		rows++
	}
	if m.isSelf {
		rows++
	}
	return contextMenuHeight(rows)
}
