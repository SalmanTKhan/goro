package mobileui

import (
	"fmt"
	"time"

	"github.com/kivutar/goro/input"
)

func Fixture(name string) MobileHUDModel {
	m := MobileHUDModel{Player: PlayerHUDModel{Name: "Goro", HP: 742, MaxHP: 1000, SP: 184, MaxSP: 260, BaseLevel: 42, JobLevel: 31}, Minimap: MinimapModel{Visible: true, MapName: "prt_fild08"}}
	for i := 0; i < 5; i++ {
		m.Skills = append(m.Skills, SkillSlotModel{Index: i, SkillID: uint16(100 + i), Name: fmt.Sprintf("Skill %d", i+1), Level: i%3 + 1, MaxLevel: 5, IconKey: fmt.Sprintf("skill-%d", i+1), Usable: true})
	}
	switch name {
	case "monster":
		m.Target = TargetHUDModel{Visible: true, ID: 9001, Name: "Poring", HP: 38, MaxHP: 120, Relation: TargetHostile}
		m.Skills[1].TargetMode = input.SkillTargetActor
	case "low-hp":
		m.Player.HP = 74
		m.Player.SP = 22
	case "cooldowns":
		m.Skills[0].CooldownRemaining = 1500 * time.Millisecond
		m.Skills[0].CooldownTotal = 5000 * time.Millisecond
	case "skills-paged":
		for i := 5; i < 9; i++ {
			m.Skills = append(m.Skills, SkillSlotModel{Index: i, SkillID: uint16(100 + i), Name: fmt.Sprintf("Skill %d", i+1), Level: i%3 + 1, MaxLevel: 5, IconKey: fmt.Sprintf("skill-%d", i+1), Usable: true})
		}
		m.Skills[1].TargetMode = input.SkillTargetActor
		m.Skills[3].TargetMode = input.SkillTargetGround
	case "many-status":
		for i := 0; i < 8; i++ {
			m.Statuses = append(m.Statuses, StatusEffectModel{ID: uint16(i + 1), IconKey: fmt.Sprintf("status-%d", i+1), Name: "Status"})
		}
	case "long-target":
		m.Target = TargetHUDModel{Visible: true, ID: 9002, Name: "A very long target name that must remain clipped by the renderer", HP: 400, MaxHP: 600, Relation: TargetNPC}
	case "combat-cooldown":
		m.Target = TargetHUDModel{Visible: true, ID: 9001, Name: "Poring", HP: 38, MaxHP: 120, Relation: TargetHostile}
		m.Skills[0].CooldownRemaining = 1500 * time.Millisecond
		m.Skills[0].CooldownTotal = 5000 * time.Millisecond
	case "loot-basic":
		m.Loot = []LootItemModel{{DropID: 50001, ItemID: 909, Identified: true, Name: "Jellopy", Quantity: 1, X: 82, Y: 98, Distance: 4}}
	case "loot-many":
		for i := 0; i < 6; i++ {
			m.Loot = append(m.Loot, LootItemModel{DropID: uint32(50001 + i), ItemID: uint16(909 + i), Identified: true, Name: fmt.Sprintf("Drop Item %02d", i+1), Quantity: i + 1, X: 82 + i, Y: 98, Distance: i + 1})
		}
	case "loot-long-names":
		m.Loot = []LootItemModel{{DropID: 50001, ItemID: 909, Identified: true, Name: "A very long item name that must remain clipped by the mobile pickup rail", Quantity: 99, X: 82, Y: 98, Distance: 4}}
	}
	return m
}
