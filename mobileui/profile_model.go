package mobileui

import "github.com/kivutar/goro/session"

// MobileProfileModel is the renderer-neutral projection of the local offline
// character identity. It contains no save-file or renderer handles.
type MobileProfileModel struct {
	Available bool
	Editable  bool
	ProfileID uint32
	Name      string
	Sex       uint8
	SexLabel  string
	HairStyle int
	HairColor int
	Stats     [6]uint8
	Notice    string
}

func ProjectOfflineProfile(profile session.OfflineProfile, available, editable bool, notice string) MobileProfileModel {
	model := MobileProfileModel{
		Available: available,
		Editable:  editable,
		ProfileID: profile.ID,
		Name:      profile.Name,
		Sex:       profile.Sex,
		SexLabel:  profileSexLabel(profile.Sex),
		HairStyle: int(profile.HairStyle),
		HairColor: int(profile.HairColor),
		Stats:     profile.Stats,
		Notice:    notice,
	}
	if model.Name == "" {
		model.Name = "Offline Adventurer"
	}
	if model.HairStyle == 0 {
		model.HairStyle = 2
	}
	return model
}

// The hair styles and palettes the client ships. The appearance controls wrap
// within these bounds rather than running off the end of the sprite set.
const (
	profileMinHairStyle = 2
	profileMaxHairStyle = 23
	profileHairColors   = 10
)

func profileSexLabel(sex uint8) string {
	// The RO resource contract uses sex 0 for the female sprite set and
	// non-zero for the male sprite set. Keep the mobile label derived from the
	// typed value so a stale caller-provided label cannot disagree with the
	// appearance preview.
	if sex == 0 {
		return "FEMALE"
	}
	return "MALE"
}

// ProfileSexLabel is the shared display mapping for profile screens. Sex is
// intentionally kept as the protocol value in the model; callers should not
// duplicate the resource-layer convention in presentation code.
func ProfileSexLabel(sex uint8) string { return profileSexLabel(sex) }

func ProfileStatLabels() [6]string {
	return [6]string{"STR", "AGI", "VIT", "INT", "DEX", "LUK"}
}
