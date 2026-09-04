package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// SocialTree builds the friends and party screen: a tabbed list with a detail
// panel of actions, plus the request, party-settings and text-entry modals.
func (k Kit) SocialTree(
	model mobileui.MobileSocialModel,
	layout mobileui.SocialLayout,
	state mobileui.SocialInteractionState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	k.placeSocialHeader(c, layout, state)
	k.placeSocialRows(c, model, layout, state)
	k.placeSocialDetail(c, model, layout, state)
	if model.Notice != "" && layout.Notice.W > 0 {
		c.Place(k.Wrapped(model.Notice, RoleMuted, 2), layout.Notice)
	}
	k.placeSocialModals(c, model, layout, state)
	return c
}

func (k Kit) placeSocialHeader(c *Canvas, layout mobileui.SocialLayout, state mobileui.SocialInteractionState) {
	if layout.BackButton.W > 0 {
		c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)
	}
	if layout.FriendsTab.W > 0 {
		c.Place(k.Button("Friends", buttonStateForOpen(state.Tab == mobileui.SocialTabFriends)), layout.FriendsTab)
	}
	if layout.PartyTab.W > 0 {
		c.Place(k.Button("Party", buttonStateForOpen(state.Tab == mobileui.SocialTabParty)), layout.PartyTab)
	}
}

func (k Kit) placeSocialRows(
	c *Canvas,
	model mobileui.MobileSocialModel,
	layout mobileui.SocialLayout,
	state mobileui.SocialInteractionState,
) {
	if layout.ListViewport.W <= 0 || layout.ListViewport.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.ListViewport)
	if len(layout.Rows) == 0 {
		c.Place(k.Centered(emptySocialText(state.Tab), RoleMuted), layout.ListViewport)
		return
	}

	for _, row := range layout.Rows {
		if row.Rect.W <= 0 || row.Rect.H <= 0 || !row.Rect.Intersects(layout.ListViewport) {
			continue
		}
		var title, subtitle string
		var selected bool
		switch row.Kind {
		case mobileui.SocialRowFriend:
			if row.Index >= len(model.Friends) {
				continue
			}
			friend := model.Friends[row.Index]
			title = friend.Name
			subtitle = friendStatus(friend)
			selected = state.HasSelection && state.SelectedFriendAccount == friend.AccountID &&
				state.SelectedFriendChar == friend.CharID
		case mobileui.SocialRowPartyMember:
			if row.Index >= len(model.PartyMembers) {
				continue
			}
			member := model.PartyMembers[row.Index]
			title = partyMemberName(member)
			subtitle = partyMemberStatus(member)
			selected = state.HasSelection && state.SelectedPartyAccount == member.AccountID
		default:
			continue
		}
		c.Place(k.Card(selected), row.Rect)
		titleRect, subtitleRect := k.RowLines(row.Rect, 0)
		if titleRect.W <= 0 {
			continue
		}
		c.Place(k.Content(title, RoleValue), titleRect)
		if subtitle != "" {
			c.Place(k.Text(subtitle, RoleMuted), subtitleRect)
		}
	}
}

func emptySocialText(tab mobileui.SocialTab) string {
	if tab == mobileui.SocialTabParty {
		return "You are not in a party"
	}
	return "No friends yet"
}

func friendStatus(friend mobileui.MobileFriendModel) string {
	if friend.Status != "" {
		return friend.Status
	}
	if friend.Online {
		return "Online"
	}
	return "Offline"
}

func partyMemberName(member mobileui.MobilePartyMemberModel) string {
	if member.Leader {
		return member.Name + "  (leader)"
	}
	return member.Name
}

// partyMemberStatus reports where a member is and how they are doing, which is
// what the party list exists to answer.
func partyMemberStatus(member mobileui.MobilePartyMemberModel) string {
	if !member.Online {
		return "Offline"
	}
	status := member.MapName
	if member.Dead {
		if status != "" {
			status += "  "
		}
		status += "Dead"
	} else if member.MaxHP > 0 {
		if status != "" {
			status += "  "
		}
		status += "HP " + strconv.Itoa(member.HP) + " / " + strconv.Itoa(member.MaxHP)
	}
	return status
}

// placeSocialDetail shows the party name or selected friend, with the actions
// available for it.
func (k Kit) placeSocialDetail(
	c *Canvas,
	model mobileui.MobileSocialModel,
	layout mobileui.SocialLayout,
	state mobileui.SocialInteractionState,
) {
	if layout.DetailPanel.W <= 0 || layout.DetailPanel.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.DetailPanel)
	pad := k.Theme.Metrics.TableCellPadX
	rowH := k.Theme.Metrics.TableRowHeight
	c.Place(k.Text(socialDetailTitle(model, state), RoleTitle),
		mobileui.Rect{X: layout.DetailPanel.X + pad, Y: layout.DetailPanel.Y + pad, W: layout.DetailPanel.W - 2*pad, H: rowH})

	for _, action := range []struct {
		rect  mobileui.Rect
		label string
	}{
		{layout.PrimaryAction, socialPrimaryLabel(model, state)},
		{layout.SecondaryAction, socialSecondaryLabel(model, state)},
		{layout.TertiaryAction, "Leave party"},
		{layout.SettingsAction, "Settings"},
	} {
		if action.rect.W > 0 && action.rect.H > 0 && action.label != "" {
			c.Place(k.Button(action.label, ButtonNormal), action.rect)
		}
	}
}

func socialDetailTitle(model mobileui.MobileSocialModel, state mobileui.SocialInteractionState) string {
	if state.Tab == mobileui.SocialTabParty {
		if model.PartyActive && model.PartyName != "" {
			return model.PartyName
		}
		return "Party"
	}
	return "Friends"
}

func socialPrimaryLabel(model mobileui.MobileSocialModel, state mobileui.SocialInteractionState) string {
	if state.Tab == mobileui.SocialTabParty {
		if !model.PartyActive {
			return "Create party"
		}
		return "Invite"
	}
	return "Whisper"
}

func socialSecondaryLabel(model mobileui.MobileSocialModel, state mobileui.SocialInteractionState) string {
	if state.Tab == mobileui.SocialTabParty {
		return "Expel"
	}
	return "Remove friend"
}

// placeSocialModals draws whichever modal is open: an incoming request, party
// settings, or a name prompt.
func (k Kit) placeSocialModals(
	c *Canvas,
	model mobileui.MobileSocialModel,
	layout mobileui.SocialLayout,
	state mobileui.SocialInteractionState,
) {
	switch {
	case model.FriendRequest != nil && layout.RequestModal.W > 0:
		k.placeConfirmModal(c, layout.Safe, layout.RequestModal, "Friend request",
			model.FriendRequest.Name+" wants to be your friend.",
			layout.RequestAccept, "Accept", layout.RequestDecline, "Decline")
	case model.PartyInvite != nil && layout.RequestModal.W > 0:
		k.placeConfirmModal(c, layout.Safe, layout.RequestModal, "Party invite",
			model.PartyInvite.Name+" invited you to a party.",
			layout.RequestAccept, "Accept", layout.RequestDecline, "Decline")
	case state.SettingsOpen && layout.SettingsModal.W > 0:
		k.placePartySettings(c, layout, state)
	}
}

// placeConfirmModal is the shared two-button prompt.
func (k Kit) placeConfirmModal(
	c *Canvas,
	safe, modal mobileui.Rect,
	title, message string,
	acceptRect mobileui.Rect, acceptLabel string,
	declineRect mobileui.Rect, declineLabel string,
) {
	c.Place(k.Scrim(), safe)
	c.Place(k.Window(title, nil), modal)
	if acceptRect.W > 0 {
		c.Place(k.Button(acceptLabel, ButtonNormal), acceptRect)
	}
	if declineRect.W > 0 {
		c.Place(k.Button(declineLabel, ButtonNormal), declineRect)
	}
	body := modalBody(k, modal, acceptRect)
	if body.H > 0 {
		c.Place(k.Wrapped(message, RoleBody, 3), body)
	}
}

// modalBody is the region between a window's title bar and its button row.
func modalBody(k Kit, modal, buttons mobileui.Rect) mobileui.Rect {
	pad := k.Theme.Metrics.TableCellPadX
	top := modal.Y + k.Theme.Metrics.WindowTitleHeight
	bottom := modal.Bottom() - pad
	if buttons.H > 0 && buttons.Y > top {
		bottom = buttons.Y - pad
	}
	return mobileui.Rect{X: modal.X + pad, Y: top, W: modal.W - 2*pad, H: bottom - top}
}

func (k Kit) placePartySettings(c *Canvas, layout mobileui.SocialLayout, state mobileui.SocialInteractionState) {
	c.Place(k.Scrim(), layout.Safe)
	c.Place(k.Window("Party settings", nil), layout.SettingsModal)
	for _, option := range []struct {
		rect   mobileui.Rect
		label  string
		active bool
	}{
		{layout.SettingsEach, "Each takes own", state.SettingsExpShare == 0},
		{layout.SettingsEven, "Share evenly", state.SettingsExpShare != 0},
		{layout.SettingsRefuse, "Refuse invites", state.SettingsRefuseInvites},
	} {
		if option.rect.W > 0 {
			c.Place(k.Button(option.label, buttonStateForOpen(option.active)), option.rect)
		}
	}
	if layout.SettingsConfirm.W > 0 {
		c.Place(k.Button("Apply", ButtonNormal), layout.SettingsConfirm)
	}
	if layout.SettingsCancel.W > 0 {
		c.Place(k.Button("Cancel", ButtonNormal), layout.SettingsCancel)
	}
}
