package mobileui

import (
	"strings"

	"github.com/kivutar/goro/input"
)

// SocialTextInput identifies the social actions that need a native keyboard.
// The mobile surface owns the prompt state while the shared command consumer
// remains authoritative for the resulting network action.
type SocialTextInput uint8

const (
	SocialTextInputNone SocialTextInput = iota
	SocialTextInputPartyName
	SocialTextInputInviteName
)

type MobileSocialController struct {
	Model         MobileSocialModel
	Layout        SocialLayout
	State         SocialInteractionState
	Viewport      Viewport
	Visible       bool
	PartyDraft    string
	InviteDraft   string
	WhisperTarget string
	TextInput     SocialTextInput
	Sink          input.CommandSink
	Targeting     *input.SkillTargetState
}

func NewSocialController(model MobileSocialModel, viewport Viewport, sink input.CommandSink) *MobileSocialController {
	c := &MobileSocialController{Model: model, Viewport: viewport, Sink: sink}
	c.relayout()
	return c
}

func (c *MobileSocialController) Open(tab SocialTab) {
	if c == nil {
		return
	}
	if tab != SocialTabParty {
		tab = SocialTabFriends
	}
	c.Visible = true
	c.State.Tab = tab
	c.State.Scroll = ScrollState{}
	c.State.HasSelection = false
	c.State.SettingsOpen = false
	c.State.HandledFriendAccount, c.State.HandledFriendChar, c.State.HandledPartyRequest = 0, 0, 0
	c.relayout()
}

func (c *MobileSocialController) IsOpen() bool { return c != nil && c.Visible }

func (c *MobileSocialController) Close() bool {
	if c == nil || !c.Visible {
		return false
	}
	c.Visible = false
	c.TextInput = SocialTextInputNone
	c.State.HasSelection = false
	c.relayout()
	return true
}

func (c *MobileSocialController) SetModel(model MobileSocialModel) {
	if c == nil {
		return
	}
	model.Tab = c.State.Tab
	c.Model = model
	if model.FriendRequest == nil {
		c.State.HandledFriendAccount, c.State.HandledFriendChar = 0, 0
	} else if c.State.HandledFriendAccount != model.FriendRequest.AccountID || c.State.HandledFriendChar != model.FriendRequest.CharID {
		c.State.HandledFriendAccount, c.State.HandledFriendChar = 0, 0
	}
	if model.PartyInvite == nil {
		c.State.HandledPartyRequest = 0
	} else if c.State.HandledPartyRequest != model.PartyInvite.RequestID {
		c.State.HandledPartyRequest = 0
	}
	c.retainSelection()
	c.relayout()
}

func (c *MobileSocialController) Resize(viewport Viewport) {
	if c != nil {
		c.Viewport = viewport
		c.relayout()
	}
}

func (c *MobileSocialController) ConsumeTouch(point input.TouchPoint) bool {
	return c != nil && c.Visible && c.Layout.Safe.Contains(float32(point.X), float32(point.Y))
}

func (c *MobileSocialController) ScrollBy(delta float32) bool {
	if c == nil || !c.Visible {
		return false
	}
	c.State.Scroll.ViewportExtent = c.Layout.ListViewport.H - 56
	c.State.Scroll.ContentExtent = SocialScrollExtent(c.Model, c.State, c.Layout, DefaultSocialTokens()).ContentExtent
	c.State.Scroll.ScrollBy(delta)
	c.relayout()
	return true
}

func (c *MobileSocialController) Tap(x, y float32) bool {
	if c == nil || !c.Visible {
		return false
	}
	if c.Layout.BackButton.Contains(x, y) {
		return c.Back()
	}
	if c.TextInputActive() {
		if c.Layout.TextInputConfirm.Contains(x, y) {
			c.SubmitTextInput()
		} else if c.Layout.TextInputCancel.Contains(x, y) {
			c.CancelTextInput()
		}
		return true
	}
	if c.visibleFriendRequest() != nil {
		if c.Layout.RequestAccept.Contains(x, y) {
			c.respondFriendRequest(true)
		} else if c.Layout.RequestDecline.Contains(x, y) {
			c.respondFriendRequest(false)
		}
		return true
	}
	if c.visiblePartyInvite() != nil {
		if c.Layout.RequestAccept.Contains(x, y) {
			c.respondPartyInvite(true)
		} else if c.Layout.RequestDecline.Contains(x, y) {
			c.respondPartyInvite(false)
		}
		return true
	}
	if c.State.SettingsOpen {
		if c.Layout.SettingsEach.Contains(x, y) {
			c.State.SettingsExpShare = 0
		} else if c.Layout.SettingsEven.Contains(x, y) {
			c.State.SettingsExpShare = 1
		} else if c.Layout.SettingsRefuse.Contains(x, y) {
			c.State.SettingsRefuseInvites = !c.State.SettingsRefuseInvites
		} else if c.Layout.SettingsConfirm.Contains(x, y) {
			c.commitPartySettings()
		} else if c.Layout.SettingsCancel.Contains(x, y) {
			c.State.SettingsOpen = false
		}
		c.relayout()
		return true
	}
	if c.Layout.FriendsTab.Contains(x, y) {
		c.setTab(SocialTabFriends)
		return true
	}
	if c.Layout.PartyTab.Contains(x, y) {
		c.setTab(SocialTabParty)
		return true
	}
	for _, row := range c.Layout.Rows {
		if !row.Rect.Contains(x, y) {
			continue
		}
		if row.Kind == SocialRowFriend && row.Index < len(c.Model.Friends) {
			friend := c.Model.Friends[row.Index]
			c.State.SelectedFriendAccount, c.State.SelectedFriendChar = friend.AccountID, friend.CharID
			c.State.SelectedPartyAccount, c.State.HasSelection = 0, true
		} else if row.Kind == SocialRowPartyMember && row.Index < len(c.Model.PartyMembers) {
			member := c.Model.PartyMembers[row.Index]
			c.State.SelectedPartyAccount, c.State.HasSelection = member.AccountID, true
			c.State.SelectedFriendAccount, c.State.SelectedFriendChar = 0, 0
		}
		c.relayout()
		return true
	}
	if c.Layout.PrimaryAction.Contains(x, y) {
		if c.State.Tab == SocialTabFriends {
			c.RequestFriendWhisper()
		} else if c.Model.PartyActive && c.State.HasSelection {
			c.RequestPartyWhisper()
		} else if c.Model.PartyActive {
			c.BeginInviteNameInput()
		} else {
			c.BeginPartyNameInput()
		}
		return true
	}
	if c.Layout.SecondaryAction.Contains(x, y) {
		if c.State.Tab == SocialTabFriends {
			c.DeleteFriend()
		} else if c.Model.PartyActive && c.State.HasSelection {
			c.ExpelPartyMember()
		} else if c.Model.PartyActive {
			c.BeginInviteNameInput()
		}
		return true
	}
	if c.Layout.TertiaryAction.Contains(x, y) {
		c.LeaveParty()
		return true
	}
	if c.Layout.SettingsAction.Contains(x, y) {
		c.openPartySettings()
		return true
	}
	return true
}

func (c *MobileSocialController) Back() bool {
	if c == nil || !c.Visible {
		return false
	}
	if c.Targeting != nil && c.Targeting.Mode != input.SkillTargetIdle {
		c.Targeting.Cancel()
		c.emit(input.PlayerCommand{Kind: input.CommandCancelAction})
		return true
	}
	if c.TextInputActive() {
		c.CancelTextInput()
		return true
	}
	if c.State.SettingsOpen {
		c.State.SettingsOpen = false
		c.relayout()
		return true
	}
	if c.State.HasSelection {
		c.State.HasSelection = false
		c.State.SelectedFriendAccount, c.State.SelectedFriendChar, c.State.SelectedPartyAccount = 0, 0, 0
		c.relayout()
		return true
	}
	return c.Close()
}

func (c *MobileSocialController) SetPartyDraft(value string) {
	if c != nil {
		c.PartyDraft = strings.TrimSpace(value)
	}
}
func (c *MobileSocialController) SetInviteDraft(value string) {
	if c != nil {
		c.InviteDraft = strings.TrimSpace(value)
	}
}

// TextInputActive reports whether the host should expose its native text
// editor for a social action.
func (c *MobileSocialController) TextInputActive() bool {
	return c != nil && c.Visible && c.TextInput != SocialTextInputNone
}

func (c *MobileSocialController) TextInputTitle() string {
	if c == nil {
		return "SOCIAL INPUT"
	}
	switch c.TextInput {
	case SocialTextInputPartyName:
		return "CREATE PARTY"
	case SocialTextInputInviteName:
		return "INVITE PLAYER"
	default:
		return "SOCIAL INPUT"
	}
}

func (c *MobileSocialController) TextInputHint() string {
	if c == nil {
		return "Enter text"
	}
	switch c.TextInput {
	case SocialTextInputPartyName:
		return "Enter a party name"
	case SocialTextInputInviteName:
		return "Enter a character name"
	default:
		return "Enter text"
	}
}

func (c *MobileSocialController) TextInputDraft() string {
	if c == nil {
		return ""
	}
	switch c.TextInput {
	case SocialTextInputPartyName:
		return c.PartyDraft
	case SocialTextInputInviteName:
		return c.InviteDraft
	default:
		return ""
	}
}

// SetTextInputDraft is fed by the host-native editor while a social prompt is
// visible. It intentionally does not emit a command on every keystroke.
func (c *MobileSocialController) SetTextInputDraft(value string) {
	if c == nil {
		return
	}
	switch c.TextInput {
	case SocialTextInputPartyName:
		c.PartyDraft = strings.TrimSpace(value)
	case SocialTextInputInviteName:
		c.InviteDraft = strings.TrimSpace(value)
	}
}

func (c *MobileSocialController) BeginPartyNameInput() {
	if c == nil || !c.Model.CanCreateParty {
		return
	}
	c.PartyDraft = ""
	c.TextInput = SocialTextInputPartyName
	c.relayout()
}

func (c *MobileSocialController) BeginInviteNameInput() {
	if c == nil || !c.Model.CanInvite {
		return
	}
	c.InviteDraft = ""
	c.TextInput = SocialTextInputInviteName
	c.relayout()
}

func (c *MobileSocialController) CancelTextInput() {
	if c == nil {
		return
	}
	c.TextInput = SocialTextInputNone
	c.PartyDraft = ""
	c.InviteDraft = ""
	c.relayout()
}

// SubmitTextInput emits one semantic command after the native editor has
// supplied a non-empty value. A rejected command leaves the prompt open so
// the user can correct or retry it.
func (c *MobileSocialController) SubmitTextInput() bool {
	if c == nil {
		return false
	}
	var command input.PlayerCommand
	switch c.TextInput {
	case SocialTextInputPartyName:
		if !c.Model.CanCreateParty || strings.TrimSpace(c.PartyDraft) == "" {
			return false
		}
		command = input.PlayerCommand{Kind: input.CommandCreateParty, Text: strings.TrimSpace(c.PartyDraft)}
	case SocialTextInputInviteName:
		if !c.Model.CanInvite || strings.TrimSpace(c.InviteDraft) == "" {
			return false
		}
		command = input.PlayerCommand{Kind: input.CommandInviteParty, Text: strings.TrimSpace(c.InviteDraft)}
	default:
		return false
	}
	if !c.emit(command) {
		return false
	}
	c.CancelTextInput()
	return true
}

func (c *MobileSocialController) SelectedFriend() (MobileFriendModel, bool) {
	if c == nil || !c.State.HasSelection || c.State.Tab != SocialTabFriends {
		return MobileFriendModel{}, false
	}
	for _, friend := range c.Model.Friends {
		if friend.AccountID == c.State.SelectedFriendAccount && friend.CharID == c.State.SelectedFriendChar {
			return friend, true
		}
	}
	return MobileFriendModel{}, false
}

func (c *MobileSocialController) SelectedPartyMember() (MobilePartyMemberModel, bool) {
	if c == nil || !c.State.HasSelection || c.State.Tab != SocialTabParty {
		return MobilePartyMemberModel{}, false
	}
	for _, member := range c.Model.PartyMembers {
		if member.AccountID == c.State.SelectedPartyAccount {
			return member, true
		}
	}
	return MobilePartyMemberModel{}, false
}

func (c *MobileSocialController) SelectedFriendCanWhisper() bool {
	friend, ok := c.SelectedFriend()
	return ok && friend.CanWhisper
}

func (c *MobileSocialController) SelectedFriendCanDelete() bool {
	friend, ok := c.SelectedFriend()
	return ok && friend.CanDelete
}

func (c *MobileSocialController) SelectedPartyCanWhisper() bool {
	member, ok := c.SelectedPartyMember()
	return ok && member.Online && !member.IsSelf
}

func (c *MobileSocialController) SelectedPartyIsSelf() bool {
	member, ok := c.SelectedPartyMember()
	return ok && member.IsSelf
}

func (c *MobileSocialController) TakeWhisperTarget() string {
	if c == nil {
		return ""
	}
	target := c.WhisperTarget
	c.WhisperTarget = ""
	return target
}

func (c *MobileSocialController) RequestFriendWhisper() {
	if friend, ok := c.SelectedFriend(); ok && friend.CanWhisper {
		c.WhisperTarget = friend.Name
	}
}

func (c *MobileSocialController) RequestPartyWhisper() {
	if member, ok := c.SelectedPartyMember(); ok && member.Online && !member.IsSelf {
		c.WhisperTarget = member.Name
	}
}

func (c *MobileSocialController) DeleteFriend() {
	friend, ok := c.SelectedFriend()
	if !ok || !friend.CanDelete {
		return
	}
	c.emit(input.PlayerCommand{Kind: input.CommandDeleteFriend, TargetAccountID: friend.AccountID, TargetCharID: friend.CharID})
}

func (c *MobileSocialController) CreateParty() {
	if !c.Model.CanCreateParty || strings.TrimSpace(c.PartyDraft) == "" {
		return
	}
	c.emit(input.PlayerCommand{Kind: input.CommandCreateParty, Text: strings.TrimSpace(c.PartyDraft)})
}

func (c *MobileSocialController) InvitePartyMember() {
	if !c.Model.CanInvite || strings.TrimSpace(c.InviteDraft) == "" {
		return
	}
	c.emit(input.PlayerCommand{Kind: input.CommandInviteParty, Text: strings.TrimSpace(c.InviteDraft)})
}

func (c *MobileSocialController) LeaveParty() {
	if c.Model.CanLeave {
		c.emit(input.PlayerCommand{Kind: input.CommandLeaveParty})
	}
}

func (c *MobileSocialController) ExpelPartyMember() {
	member, ok := c.SelectedPartyMember()
	if !ok || !c.Model.CanExpel || member.IsSelf {
		return
	}
	c.emit(input.PlayerCommand{Kind: input.CommandExpelPartyMember, ActorID: member.AccountID, Text: member.Name})
}

func (c *MobileSocialController) SendWhisper(message string) bool {
	if c == nil || strings.TrimSpace(c.WhisperTarget) == "" || strings.TrimSpace(message) == "" {
		return false
	}
	c.emit(input.PlayerCommand{Kind: input.CommandSendWhisper, TargetName: c.WhisperTarget, Text: strings.TrimSpace(message)})
	return true
}

func (c *MobileSocialController) setTab(tab SocialTab) {
	c.State.Tab = tab
	c.State.Scroll = ScrollState{}
	c.State.HasSelection = false
	c.relayout()
}

func (c *MobileSocialController) retainSelection() {
	if !c.State.HasSelection {
		return
	}
	if c.State.Tab == SocialTabFriends {
		if _, ok := c.SelectedFriend(); !ok {
			c.State.HasSelection = false
		}
	} else if _, ok := c.SelectedPartyMember(); !ok {
		c.State.HasSelection = false
	}
}

func (c *MobileSocialController) emit(command input.PlayerCommand) bool {
	if c.Sink != nil {
		return c.Sink.Emit(command)
	}
	return false
}

func (c *MobileSocialController) visibleFriendRequest() *MobileFriendRequestModel {
	if c == nil || c.Model.FriendRequest == nil ||
		(c.State.HandledFriendAccount == c.Model.FriendRequest.AccountID && c.State.HandledFriendChar == c.Model.FriendRequest.CharID && c.State.HandledFriendAccount != 0) {
		return nil
	}
	return c.Model.FriendRequest
}

// VisibleFriendRequest returns the current request until a response command
// has been accepted by the command sink.
func (c *MobileSocialController) VisibleFriendRequest() *MobileFriendRequestModel {
	return c.visibleFriendRequest()
}

func (c *MobileSocialController) visiblePartyInvite() *MobilePartyInviteModel {
	if c == nil || c.Model.PartyInvite == nil ||
		(c.State.HandledPartyRequest == c.Model.PartyInvite.RequestID && c.State.HandledPartyRequest != 0) {
		return nil
	}
	return c.Model.PartyInvite
}

// VisiblePartyInvite returns the current invitation until a response command
// has been accepted by the command sink.
func (c *MobileSocialController) VisiblePartyInvite() *MobilePartyInviteModel {
	return c.visiblePartyInvite()
}

func (c *MobileSocialController) respondFriendRequest(accepted bool) {
	request := c.visibleFriendRequest()
	if request == nil || !c.Model.OnlineSession {
		return
	}
	if c.emit(input.PlayerCommand{
		Kind:            input.CommandRespondFriendRequest,
		TargetAccountID: request.AccountID,
		TargetCharID:    request.CharID,
		Accepted:        accepted,
	}) {
		c.State.HandledFriendAccount, c.State.HandledFriendChar = request.AccountID, request.CharID
		c.relayout()
	}
}

func (c *MobileSocialController) respondPartyInvite(accepted bool) {
	invite := c.visiblePartyInvite()
	if invite == nil || !c.Model.OnlineSession {
		return
	}
	if c.emit(input.PlayerCommand{
		Kind:      input.CommandRespondPartyInvite,
		RequestID: invite.RequestID,
		Accepted:  accepted,
	}) {
		c.State.HandledPartyRequest = invite.RequestID
		c.relayout()
	}
}

func (c *MobileSocialController) openPartySettings() {
	if c == nil || !c.Model.PartySettings.CanEdit {
		return
	}
	c.State.SettingsOpen = true
	c.State.SettingsExpShare = c.Model.PartySettings.ExpShare
	c.State.SettingsRefuseInvites = c.Model.PartySettings.RefuseInvites
	c.relayout()
}

func (c *MobileSocialController) commitPartySettings() {
	if c == nil || !c.Model.PartySettings.CanEdit {
		return
	}
	if c.emit(input.PlayerCommand{
		Kind:          input.CommandSetPartySettings,
		ExpShare:      c.State.SettingsExpShare,
		RefuseInvites: c.State.SettingsRefuseInvites,
	}) {
		c.State.SettingsOpen = false
	}
}

func (c *MobileSocialController) relayout() {
	if c == nil {
		return
	}
	extent := SocialScrollExtent(c.Model, c.State, LayoutSocial(c.Viewport, DefaultSocialTokens(), c.Model, c.State), DefaultSocialTokens())
	c.State.Scroll.ViewportExtent = extent.ViewportExtent
	c.State.Scroll.ContentExtent = extent.ContentExtent
	c.State.Scroll.SetOffset(c.State.Scroll.Offset)
	c.Layout = LayoutSocial(c.Viewport, DefaultSocialTokens(), c.Model, c.State)
}
