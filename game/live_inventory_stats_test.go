package game

import (
	"testing"
	"time"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/session"
)

func TestDerivedStatParameterPackets(t *testing.T) {
	// Wire IDs from DHXJ's VAR_* and roBrowser's StatusProperty enums.
	tests := []struct {
		name  string
		id    uint16
		field func(*session.Stats) *int
	}{
		{"ATK", 41, func(s *session.Stats) *int { return &s.Attack }},
		{"ATK bonus", 42, func(s *session.Stats) *int { return &s.AttackBonus }},
		{"MATK max", 43, func(s *session.Stats) *int { return &s.MatkMax }},
		{"MATK min", 44, func(s *session.Stats) *int { return &s.MatkMin }},
		{"DEF", 45, func(s *session.Stats) *int { return &s.Defense }},
		{"DEF bonus", 46, func(s *session.Stats) *int { return &s.DefenseBonus }},
		{"MDEF", 47, func(s *session.Stats) *int { return &s.MDefense }},
		{"MDEF bonus", 48, func(s *session.Stats) *int { return &s.MDefenseBonus }},
		{"HIT", 49, func(s *session.Stats) *int { return &s.Hit }},
		{"FLEE", 50, func(s *session.Stats) *int { return &s.Flee }},
		{"FLEE bonus", 51, func(s *session.Stats) *int { return &s.FleeBonus }},
		{"CRIT", 52, func(s *session.Stats) *int { return &s.Critical }},
		{"ASPD", 53, func(s *session.Stats) *int { return &s.ASPD }},
		{"ASPD bonus", 54, func(s *session.Stats) *int { return &s.ASPDBonus }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := session.Stats{Str: 12, StrBonus: 2, Points: 5, Attack: 10, Defense: 3, ASPD: 1500}
			ctx := client.Context{Session: &session.Session{Stats: want}}
			mode := NewWorldMode()
			for _, value := range []int32{37, -2, 0} {
				mode.handleNetworkPacket(ctx, testParameterChangePacket(tc.id, uint32(value)), time.Now())
				*tc.field(&want) = int(value)
				if ctx.Session.Stats != want {
					t.Fatalf("value %d: stats=%+v, want %+v", value, ctx.Session.Stats, want)
				}
			}
		})
	}
}
