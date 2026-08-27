package mobileui

import (
	"strings"

	"github.com/kivutar/goro/session"
)

// SocialTab identifies the two tabs exposed by the desktop Friends window.
// The mobile surface keeps both tabs in one bounded screen so switching does
// not create a second, subtly different social authority.
type SocialTab uint8

const (
	SocialTabFriends SocialTab = iota
	SocialTabParty
)

func (t SocialTab) String() string {
	if t == SocialTabParty {
		return "Party"
	}
	return "Friends"
}

type MobileFriendModel struct {
	AccountID  uint32
	CharID     uint32
	Name       string
	Online     bool
	Status     string
	CanWhisper bool
	CanDelete  bool
}

type MobilePartyMemberModel struct {
	AccountID uint32
	Name      string
	MapName   string
	Online    bool
	Leader    bool
	Dead      bool
	HP        int
	MaxHP     int
	IsSelf    bool
}

type MobileFriendRequestModel struct {
	AccountID uint32
	CharID    uint32
	Name      string
}

type MobilePartyInviteModel struct {
	RequestID uint32
	Name      string
}

type MobilePartySettingsModel struct {
	ExpShare      uint32
	RefuseInvites bool
	CanEdit       bool
}

// MobileSocialModel is a read-only projection. It deliberately contains no
// packet or desktop widget types; command authority remains in WorldMode and
// the existing network helpers.
type MobileSocialModel struct {
	Tab            SocialTab
	Friends        []MobileFriendModel
	PartyName      string
	PartyActive    bool
	PartyMembers   []MobilePartyMemberModel
	FriendRequest  *MobileFriendRequestModel
	PartyInvite    *MobilePartyInviteModel
	PartySettings  MobilePartySettingsModel
	CanCreateParty bool
	CanInvite      bool
	CanLeave       bool
	CanExpel       bool
	OnlineSession  bool
	Notice         string
}

func ProjectSocial(s *session.Session, online bool) MobileSocialModel {
	model := MobileSocialModel{OnlineSession: online, Tab: SocialTabFriends}
	if !online {
		model.Notice = "Social actions are available in an online session."
	}
	if s == nil {
		return model
	}
	for _, friend := range s.Friends.List {
		name := strings.TrimSpace(friend.Name)
		if name == "" {
			name = "Unknown"
		}
		model.Friends = append(model.Friends, MobileFriendModel{
			AccountID:  friend.AccountID,
			CharID:     friend.CharID,
			Name:       name,
			Online:     friend.Online(),
			Status:     friendStatus(friend.Online()),
			CanWhisper: online && friend.Online(),
			CanDelete:  online,
		})
	}
	model.PartyName = strings.TrimSpace(s.Party.Name)
	model.PartyActive = s.Party.Active()
	model.CanCreateParty = online && !model.PartyActive
	model.CanInvite = online && model.PartyActive && partyCanManageMobile(s)
	model.CanLeave = online && model.PartyActive
	model.CanExpel = online && model.PartyActive && partyCanManageMobile(s)
	model.PartySettings = MobilePartySettingsModel{
		ExpShare:      s.Party.ExpShare,
		RefuseInvites: s.Party.RefuseInvites,
		CanEdit:       online && model.PartyActive && partyCanManageMobile(s),
	}
	if online {
		if request := s.PendingFriendRequest; request != nil {
			model.FriendRequest = &MobileFriendRequestModel{
				AccountID: request.AccountID,
				CharID:    request.CharID,
				Name:      socialDisplayName(request.Name),
			}
		}
	}
	if online {
		if invite := s.PendingPartyInvite; invite != nil {
			model.PartyInvite = &MobilePartyInviteModel{
				RequestID: invite.RequestID,
				Name:      socialDisplayName(invite.Name),
			}
		}
	}
	for _, member := range s.Party.Members {
		name := strings.TrimSpace(member.Name)
		if name == "" {
			name = "Player"
		}
		mapName := strings.TrimSpace(member.MapName)
		if mapName == "" && member.Online() {
			mapName = "Online"
		}
		model.PartyMembers = append(model.PartyMembers, MobilePartyMemberModel{
			AccountID: member.AccountID,
			Name:      name,
			MapName:   mapName,
			Online:    member.Online(),
			Leader:    member.Leader(),
			Dead:      member.Dead,
			HP:        member.HP,
			MaxHP:     member.MaxHP,
			IsSelf:    member.AccountID != 0 && member.AccountID == s.AccountID,
		})
	}
	return model
}

func friendStatus(online bool) string {
	if online {
		return "Online"
	}
	return "Offline"
}

func socialDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Someone"
	}
	return name
}

func partyCanManageMobile(s *session.Session) bool {
	if s == nil || !s.Party.Active() {
		return false
	}
	if len(s.Party.Members) == 0 {
		return true
	}
	for _, member := range s.Party.Members {
		if member.AccountID == s.AccountID {
			return member.Leader()
		}
	}
	return false
}
