package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestSocialProjectionAndTabs(t *testing.T) {
	model := FixtureSocial("social-basic")
	if len(model.Friends) != 3 || !model.PartyActive || !model.CanInvite || !model.CanLeave {
		t.Fatalf("unexpected social projection: %+v", model)
	}
	var commands input.CommandBuffer
	c := NewSocialController(model, FoldOuterViewport(), &commands)
	c.Open(SocialTabFriends)
	if !c.ConsumeTouch(input.TouchPoint{X: 1, Y: 1}) {
		t.Fatal("social screen did not claim safe-area touch")
	}
	c.Tap(c.Layout.Rows[0].Rect.X+1, c.Layout.Rows[0].Rect.Y+1)
	if friend, ok := c.SelectedFriend(); !ok || friend.Name != "Alice" {
		t.Fatalf("friend selection failed: %+v %t", friend, ok)
	}
	c.Tap(c.Layout.PrimaryAction.X+1, c.Layout.PrimaryAction.Y+1)
	if got := c.TakeWhisperTarget(); got != "Alice" {
		t.Fatalf("whisper target=%q", got)
	}
	c.Tap(c.Layout.PartyTab.X+1, c.Layout.PartyTab.Y+1)
	if c.State.Tab != SocialTabParty || c.State.HasSelection {
		t.Fatalf("party tab state=%+v", c.State)
	}
}

func TestSocialCommandsAndNavigationPrecedence(t *testing.T) {
	model := FixtureSocial("social-basic")
	var commands input.CommandBuffer
	c := NewSocialController(model, Viewport{Width: 1920, Height: 1080}, &commands)
	c.Open(SocialTabFriends)
	c.Tap(c.Layout.Rows[0].Rect.X+1, c.Layout.Rows[0].Rect.Y+1)
	c.Tap(c.Layout.SecondaryAction.X+1, c.Layout.SecondaryAction.Y+1)
	c.SetModel(model)
	if got := commands.Commands(); len(got) != 1 || got[0].Kind != input.CommandDeleteFriend || got[0].TargetAccountID != 101 || got[0].TargetCharID != 201 {
		t.Fatalf("delete command=%+v", got)
	}
	c.Open(SocialTabParty)
	c.Tap(c.Layout.Rows[1].Rect.X+1, c.Layout.Rows[1].Rect.Y+1)
	c.Tap(c.Layout.SecondaryAction.X+1, c.Layout.SecondaryAction.Y+1)
	c.Tap(c.Layout.TertiaryAction.X+1, c.Layout.TertiaryAction.Y+1)
	got := commands.Commands()
	if len(got) != 3 || got[1].Kind != input.CommandExpelPartyMember || got[2].Kind != input.CommandLeaveParty {
		t.Fatalf("party commands=%+v", got)
	}
	var targeting input.SkillTargetState
	targeting.BeginActor(10, 1)
	c.Targeting = &targeting
	if !c.Back() || targeting.Mode != input.SkillTargetIdle || len(commands.Commands()) != 4 || commands.Commands()[3].Kind != input.CommandCancelAction {
		t.Fatalf("targeting cancellation failed: %+v", commands.Commands())
	}
	if !c.Back() || c.State.HasSelection {
		t.Fatal("selection did not clear before closing social screen")
	}
	if !c.Back() || c.IsOpen() {
		t.Fatal("social screen did not close")
	}
}

func TestSocialLayoutFoldOuterAndScrollClamp(t *testing.T) {
	model := FixtureSocial("social-long")
	c := NewSocialController(model, FoldOuterViewport(), nil)
	c.Open(SocialTabFriends)
	if c.Layout.Panel.W > 1600 || c.Layout.ListViewport.Right() > c.Layout.Safe.Right()+0.01 || c.Layout.DetailPanel.Right() > c.Layout.Safe.Right()+0.01 {
		t.Fatalf("wide social layout escaped bounds: %+v", c.Layout)
	}
	for _, row := range c.Layout.Rows {
		if row.Rect.W < 48 || row.Rect.H < 48 || row.Rect.X < c.Layout.ListViewport.X || row.Rect.Right() > c.Layout.ListViewport.Right() {
			t.Fatalf("invalid social row: %+v", row)
		}
	}
	c.ScrollBy(100000)
	if c.State.Scroll.Offset != c.State.Scroll.MaxOffset() {
		t.Fatalf("social scroll did not clamp: %+v", c.State.Scroll)
	}
	c.ScrollBy(-100000)
	if c.State.Scroll.Offset != 0 {
		t.Fatalf("social scroll did not clamp at zero: %+v", c.State.Scroll)
	}
}

func TestSocialLayoutRepresentativeViewports(t *testing.T) {
	viewports := []Viewport{
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
		{Width: 1920, Height: 1080},
		{Width: 2400, Height: 1080},
		{Width: 2560, Height: 1440},
		{Width: 1280, Height: 800, SafeLeft: 24, SafeTop: 24, SafeRight: 24, SafeBottom: 32},
	}
	for _, viewport := range viewports {
		model := FixtureSocial("social-party")
		c := NewSocialController(model, viewport, nil)
		c.Open(SocialTabParty)
		safe := viewport.SafeRect()
		for name, rect := range map[string]Rect{
			"panel": c.Layout.Panel, "header": c.Layout.Header, "friends": c.Layout.FriendsTab,
			"party": c.Layout.PartyTab, "list": c.Layout.ListViewport, "detail": c.Layout.DetailPanel,
			"back": c.Layout.BackButton, "primary": c.Layout.PrimaryAction, "secondary": c.Layout.SecondaryAction,
			"tertiary": c.Layout.TertiaryAction,
		} {
			if rect.W <= 0 || rect.H <= 0 {
				continue
			}
			if rect.X < safe.X || rect.Y < safe.Y || rect.Right() > safe.Right() || rect.Bottom() > safe.Bottom() {
				t.Fatalf("%s escaped safe area at %+v: rect=%+v safe=%+v", name, viewport, rect, safe)
			}
		}
		for _, row := range c.Layout.Rows {
			if row.Rect.W < 48 || row.Rect.H < 48 {
				t.Fatalf("row below touch target at %+v: %+v", viewport, row.Rect)
			}
		}
	}
}

func TestSocialOfflineStateDisablesMutations(t *testing.T) {
	model := FixtureSocial("social-offline")
	if model.OnlineSession || model.CanCreateParty || model.CanInvite || model.CanLeave || model.CanExpel || model.Notice == "" {
		t.Fatalf("offline social state=%+v", model)
	}
	var commands input.CommandBuffer
	c := NewSocialController(model, FoldOuterViewport(), &commands)
	c.Open(SocialTabFriends)
	c.Tap(c.Layout.Rows[0].Rect.X+1, c.Layout.Rows[0].Rect.Y+1)
	c.Tap(c.Layout.SecondaryAction.X+1, c.Layout.SecondaryAction.Y+1)
	if len(commands.Commands()) != 0 {
		t.Fatalf("offline social emitted commands: %+v", commands.Commands())
	}
}

func TestSocialIncomingRequestsEmitSemanticResponses(t *testing.T) {
	var commands input.CommandBuffer
	friend := NewSocialController(FixtureSocial("social-request-friend"), FoldOuterViewport(), &commands)
	friend.Open(SocialTabFriends)
	if request := friend.VisibleFriendRequest(); request == nil || request.Name != "Requesting Friend" {
		t.Fatalf("friend request=%+v", friend.VisibleFriendRequest())
	}
	friend.Tap(friend.Layout.RequestAccept.X+1, friend.Layout.RequestAccept.Y+1)
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandRespondFriendRequest || got[0].TargetAccountID != 106 || got[0].TargetCharID != 206 || !got[0].Accepted {
		t.Fatalf("friend response command=%+v", got)
	}
	if friend.VisibleFriendRequest() != nil {
		t.Fatal("accepted friend request remained visible")
	}

	commands.Reset()
	party := NewSocialController(FixtureSocial("social-request-party"), FoldOuterViewport(), &commands)
	party.Open(SocialTabParty)
	party.Tap(party.Layout.RequestDecline.X+1, party.Layout.RequestDecline.Y+1)
	got = commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandRespondPartyInvite || got[0].RequestID != 9001 || got[0].Accepted {
		t.Fatalf("party response command=%+v", got)
	}
}

func TestSocialPartySettingsEmitSemanticCommand(t *testing.T) {
	var commands input.CommandBuffer
	c := NewSocialController(FixtureSocial("social-settings"), FoldOuterViewport(), &commands)
	c.Open(SocialTabParty)
	if !c.Layout.SettingsAction.Contains(c.Layout.SettingsAction.X+1, c.Layout.SettingsAction.Y+1) {
		t.Fatal("party settings action has no hit area")
	}
	c.Tap(c.Layout.SettingsAction.X+1, c.Layout.SettingsAction.Y+1)
	if !c.State.SettingsOpen {
		t.Fatal("party settings did not open")
	}
	c.Tap(c.Layout.SettingsEach.X+1, c.Layout.SettingsEach.Y+1)
	c.Tap(c.Layout.SettingsRefuse.X+1, c.Layout.SettingsRefuse.Y+1)
	c.Tap(c.Layout.SettingsConfirm.X+1, c.Layout.SettingsConfirm.Y+1)
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandSetPartySettings || got[0].ExpShare != 0 || !got[0].RefuseInvites {
		t.Fatalf("party settings command=%+v", got)
	}
	if c.State.SettingsOpen {
		t.Fatal("party settings stayed open after accepted command")
	}
}

func TestSocialRequestAndSettingsLayoutStayInsideFoldSafeArea(t *testing.T) {
	viewports := []Viewport{
		FoldOuterViewport(),
		{Width: 1920, Height: 1080},
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
	}
	for _, viewport := range viewports {
		for _, fixture := range []string{"social-request-friend", "social-request-party", "social-settings"} {
			model := FixtureSocial(fixture)
			state := SocialInteractionState{Tab: SocialTabFriends}
			if fixture != "social-request-friend" {
				state.Tab = SocialTabParty
			}
			layout := LayoutSocial(viewport, DefaultSocialTokens(), model, state)
			safe := viewport.SafeRect()
			for name, rect := range map[string]Rect{
				"request": layout.RequestModal, "accept": layout.RequestAccept, "decline": layout.RequestDecline,
				"settings": layout.SettingsModal, "each": layout.SettingsEach, "even": layout.SettingsEven,
				"refuse": layout.SettingsRefuse, "confirm": layout.SettingsConfirm, "cancel": layout.SettingsCancel,
				"text-input": layout.TextInputModal, "text-confirm": layout.TextInputConfirm, "text-cancel": layout.TextInputCancel,
			} {
				if rect.W <= 0 || rect.H <= 0 {
					continue
				}
				if rect.X < safe.X || rect.Y < safe.Y || rect.Right() > safe.Right() || rect.Bottom() > safe.Bottom() {
					t.Fatalf("%s escaped safe area at %+v: rect=%+v safe=%+v", name, viewport, rect, safe)
				}
				if name != "request" && name != "settings" && (rect.W < 48 || rect.H < 48) {
					t.Fatalf("%s below touch target at %+v: %+v", name, viewport, rect)
				}
			}
		}
	}
}

func TestSocialNativeTextInputCreatesPartyAndInvitesPlayer(t *testing.T) {
	var commands input.CommandBuffer
	party := NewSocialController(FixtureSocial("social-empty"), FoldOuterViewport(), &commands)
	party.Open(SocialTabParty)
	party.Tap(party.Layout.PrimaryAction.X+1, party.Layout.PrimaryAction.Y+1)
	if !party.TextInputActive() || party.TextInput != SocialTextInputPartyName {
		t.Fatalf("party text input did not open: active=%t kind=%d", party.TextInputActive(), party.TextInput)
	}
	party.SetTextInputDraft("  Poring Patrol  ")
	if !party.SubmitTextInput() {
		t.Fatal("party creation was not accepted")
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandCreateParty || got[0].Text != "Poring Patrol" {
		t.Fatalf("party creation command=%+v", got)
	}
	if party.TextInputActive() {
		t.Fatal("party creation prompt remained open")
	}

	commands.Reset()
	invite := NewSocialController(FixtureSocial("social-party"), FoldOuterViewport(), &commands)
	invite.Open(SocialTabParty)
	invite.Tap(invite.Layout.PrimaryAction.X+1, invite.Layout.PrimaryAction.Y+1)
	if !invite.TextInputActive() || invite.TextInput != SocialTextInputInviteName {
		t.Fatalf("invite text input did not open: active=%t kind=%d", invite.TextInputActive(), invite.TextInput)
	}
	invite.SetTextInputDraft(" Alice ")
	if !invite.SubmitTextInput() {
		t.Fatal("party invite was not accepted")
	}
	got = commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandInviteParty || got[0].Text != "Alice" {
		t.Fatalf("party invite command=%+v", got)
	}
}

func TestSocialNativeTextInputCancelAndBack(t *testing.T) {
	c := NewSocialController(FixtureSocial("social-empty"), FoldOuterViewport(), nil)
	c.Open(SocialTabParty)
	c.BeginPartyNameInput()
	c.SetTextInputDraft("discarded")
	if !c.Back() || c.TextInputActive() {
		t.Fatalf("back did not cancel social text input: %+v", c)
	}
	c.BeginPartyNameInput()
	if !c.Layout.TextInputConfirm.Contains(c.Layout.TextInputConfirm.X+1, c.Layout.TextInputConfirm.Y+1) || !c.Layout.TextInputCancel.Contains(c.Layout.TextInputCancel.X+1, c.Layout.TextInputCancel.Y+1) {
		t.Fatal("social text input buttons have no hit area")
	}
	c.Tap(c.Layout.TextInputCancel.X+1, c.Layout.TextInputCancel.Y+1)
	if c.TextInputActive() {
		t.Fatal("cancel button did not close social text input")
	}
}
