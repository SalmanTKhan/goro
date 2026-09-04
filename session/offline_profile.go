package session

import (
	"fmt"
	"strings"
)

const (
	offlineProfileMinNameBytes = 4
	offlineProfileMaxNameBytes = 23
	offlineProfileMinHairStyle = 2
	offlineProfileMaxHairStyle = 23
	offlineProfileHairColors   = 10
)

// clampProfileHairStyle and clampProfileHairColor hold an appearance inside the
// range SetProfile validates, so a profile projected from world state is always
// one the client will accept back.
func clampProfileHairStyle(hair int16) int16 {
	if hair < offlineProfileMinHairStyle {
		return offlineProfileMinHairStyle
	}
	if hair > offlineProfileMaxHairStyle {
		return offlineProfileMaxHairStyle
	}
	return hair
}

func clampProfileHairColor(color uint8) uint8 {
	if int(color) >= offlineProfileHairColors {
		return offlineProfileHairColors - 1
	}
	return color
}

// EnsureProfile upgrades legacy saves and freshly-created offline sessions to
// the profile contract without changing the existing gameplay state.
func (s *OfflineSession) EnsureProfile(state *Session) {
	if s == nil || s.HasProfile || state == nil {
		return
	}
	character := state.SelectedCharacter()
	name := strings.TrimSpace(character.Name)
	if name == "" {
		name = "Offline Adventurer"
	}
	hair := clampProfileHairStyle(character.Hair)
	stats := [6]uint8{5, 5, 5, 5, 5, 5}
	for i, value := range [6]int{state.Stats.Str, state.Stats.Agi, state.Stats.Vit, state.Stats.Int, state.Stats.Dex, state.Stats.Luk} {
		if value >= 1 && value <= 9 {
			stats[i] = uint8(value)
		}
	}
	s.Profile = OfflineProfile{ID: 1, Name: name, Sex: state.Sex, HairStyle: hair, HairColor: character.HairColor, Stats: stats}
	s.HasProfile = true
}

func (s *OfflineSession) syncProfile(state *Session) {
	s.EnsureProfile(state)
	if s == nil || state == nil || !s.HasProfile {
		return
	}
	character := state.SelectedCharacter()
	if strings.TrimSpace(character.Name) != "" {
		s.Profile.Name = strings.TrimSpace(character.Name)
	}
	s.Profile.Sex = state.Sex
	// Clamped for the same reason EnsureProfile clamps: the appearance is copied
	// from the world character, whose hair may sit outside the range SetProfile
	// accepts. Taking it raw produced a profile the client's own validator
	// rejected, so saving from the character screen failed for good.
	s.Profile.HairStyle = clampProfileHairStyle(character.Hair)
	s.Profile.HairColor = clampProfileHairColor(character.HairColor)
	if state.Stats.Str >= 1 && state.Stats.Str <= 9 {
		s.Profile.Stats[0] = uint8(state.Stats.Str)
	}
	if state.Stats.Agi >= 1 && state.Stats.Agi <= 9 {
		s.Profile.Stats[1] = uint8(state.Stats.Agi)
	}
	if state.Stats.Vit >= 1 && state.Stats.Vit <= 9 {
		s.Profile.Stats[2] = uint8(state.Stats.Vit)
	}
	if state.Stats.Int >= 1 && state.Stats.Int <= 9 {
		s.Profile.Stats[3] = uint8(state.Stats.Int)
	}
	if state.Stats.Dex >= 1 && state.Stats.Dex <= 9 {
		s.Profile.Stats[4] = uint8(state.Stats.Dex)
	}
	if state.Stats.Luk >= 1 && state.Stats.Luk <= 9 {
		s.Profile.Stats[5] = uint8(state.Stats.Luk)
	}
}

func (s *OfflineSession) ProfileSnapshot(state *Session) (OfflineProfile, bool) {
	if s == nil {
		return OfflineProfile{}, false
	}
	s.syncProfile(state)
	return s.Profile, s.HasProfile
}

func (s *OfflineSession) SetProfile(profile OfflineProfile) error {
	if s == nil {
		return fmt.Errorf("offline profile is unavailable")
	}
	profile.Name = strings.TrimSpace(profile.Name)
	if len([]byte(profile.Name)) < offlineProfileMinNameBytes || len([]byte(profile.Name)) > offlineProfileMaxNameBytes {
		return fmt.Errorf("profile name must be between %d and %d bytes", offlineProfileMinNameBytes, offlineProfileMaxNameBytes)
	}
	if profile.Sex > 1 {
		return fmt.Errorf("profile sex is invalid")
	}
	if profile.HairStyle < offlineProfileMinHairStyle || profile.HairStyle > offlineProfileMaxHairStyle {
		return fmt.Errorf("profile hair style is invalid")
	}
	if int(profile.HairColor) >= offlineProfileHairColors {
		return fmt.Errorf("profile hair color is invalid")
	}
	total := 0
	for _, value := range profile.Stats {
		if value < 1 || value > 9 {
			return fmt.Errorf("profile starter stats are invalid")
		}
		total += int(value)
	}
	if total != 30 {
		return fmt.Errorf("profile starter stats must total 30")
	}
	if profile.ID == 0 {
		profile.ID = 1
	}
	s.Profile, s.HasProfile = profile, true
	return nil
}
