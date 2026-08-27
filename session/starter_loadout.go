package session

import "github.com/kivutar/goro/db"

// starterSkill is one curated entry in the default offline loadout. typ mirrors
// the rAthena skill inf bitfield used by session.Skill.Type (1 = enemy target,
// 2 = ground/place, 4 = self).
type starterSkill struct {
	id  uint16
	typ uint32
}

// starterSwordmanSkills is the default skill bar for a freshly created offline
// character. It is a small Swordman-tree mix of enemy-target and self-cast
// skills so the mobile hotbar demonstrates multi-skill casting out of the box.
var starterSwordmanSkills = []starterSkill{
	{db.SkillSMBash, 1},    // Attack
	{db.SkillSMProvoke, 1}, // Attack
	{db.SkillSMMagnum, 4},  // Self
	{db.SkillSMEndure, 4},  // Self
}

// StarterSkillLoadout builds the default offline Skills list and a matching
// hotkey bar from the curated starter set. Skills missing from the content pack
// are skipped, so a partial pack still yields a usable (possibly shorter)
// loadout. The returned Hotkeys is marked Loaded only when at least one slot was
// populated.
func StarterSkillLoadout(content OfflineContent) (Skills, Hotkeys) {
	var skills Skills
	var hotkeys Hotkeys
	for _, entry := range starterSwordmanSkills {
		def, ok := content.Skills[entry.id]
		if !ok {
			continue
		}
		skillRange := def.Range
		if skillRange <= 0 {
			skillRange = 1
		}
		skills.List = append(skills.List, Skill{
			ID:       def.ID,
			Type:     entry.typ,
			Level:    1,
			MaxLevel: def.MaxLevel,
			SPCost:   def.SPCost,
			Range:    skillRange,
			Name:     def.Name,
		})
		hotkeys.Slots = append(hotkeys.Slots, HotkeySlot{Type: 1 /* skill */, ID: uint32(def.ID), Level: 1})
	}
	if len(hotkeys.Slots) > 0 {
		hotkeys.Loaded = true
		hotkeys.Version = 1
	}
	return skills, hotkeys
}
