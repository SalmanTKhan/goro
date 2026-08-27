package mobileui

func FixtureProfile(name string) MobileProfileModel {
	model := MobileProfileModel{
		Available: true,
		Editable:  true,
		ProfileID: 1,
		Name:      "Offline Adventurer",
		Sex:       0,
		SexLabel:  "FEMALE",
		HairStyle: 2,
		HairColor: 0,
		Stats:     [6]uint8{5, 5, 5, 5, 5, 5},
	}
	switch name {
	case "profile-female":
		model.Name = "Aurelia"
		model.Sex, model.SexLabel = 0, ProfileSexLabel(0)
		model.HairStyle, model.HairColor = 12, 4
	case "profile-custom":
		model.Name = "Goro Tester"
		model.HairStyle, model.HairColor = 23, 9
		model.Stats = [6]uint8{9, 7, 3, 3, 4, 4}
	case "profile-empty":
		model.Available = false
		model.Name = ""
		model.Notice = "No offline profile has been created."
	case "profile-long-name":
		model.Name = "A Very Long Adventurer"
		model.Notice = "Name remains bounded by the mobile profile editor."
	}
	return model
}
