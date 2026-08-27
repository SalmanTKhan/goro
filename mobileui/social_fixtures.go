package mobileui

import "github.com/kivutar/goro/session"

func FixtureSocial(name string) MobileSocialModel {
	s := session.New()
	s.AccountID = 100
	s.Friends.List = []session.Friend{
		{AccountID: 101, CharID: 201, Name: "Alice", State: 0},
		{AccountID: 102, CharID: 202, Name: "Balthasar", State: 1},
		{AccountID: 103, CharID: 203, Name: "Long Named Friend That Must Stay Inside The Mobile Row", State: 0},
	}
	s.Party = session.Party{Name: "Poring Patrol", Members: []session.PartyMember{
		{AccountID: 100, Name: "Goro", MapName: "prontera", Role: 0, State: 0, HP: 742, MaxHP: 1000},
		{AccountID: 104, Name: "Alice", MapName: "prt_fild05", Role: 1, State: 0, HP: 310, MaxHP: 500},
		{AccountID: 105, Name: "Offline Friend", MapName: "prontera", Role: 1, State: 1, HP: 0, MaxHP: 500},
	}}
	online := true
	switch name {
	case "social-empty":
		s.Friends.List = nil
		s.Party = session.Party{}
	case "social-party":
		// Keep the full party fixture and open it through the preview controller.
		s.PendingFriendRequest = nil
		s.PendingPartyInvite = nil
	case "social-request-friend":
		s.PendingFriendRequest = &session.PendingFriendRequest{AccountID: 106, CharID: 206, Name: "Requesting Friend"}
		s.PendingPartyInvite = nil
	case "social-request-party":
		s.PendingPartyInvite = &session.PendingPartyInvite{RequestID: 9001, Name: "Party Leader"}
		s.PendingFriendRequest = nil
	case "social-settings":
		s.PendingFriendRequest = nil
		s.PendingPartyInvite = nil
		s.Party.ExpShare = 1
	case "social-long":
		for i := 0; i < 12; i++ {
			s.Friends.List = append(s.Friends.List, session.Friend{AccountID: uint32(200 + i), CharID: uint32(300 + i), Name: "Friend With A Long Deterministic Name", State: uint8(i % 2)})
		}
	case "social-offline":
		online = false
	}
	return ProjectSocial(s, online)
}
