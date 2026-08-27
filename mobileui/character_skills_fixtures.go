package mobileui

import (
	"fmt"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/session"
)

func FixtureCharacter(name string) MobileCharacterModel {
	return ProjectCharacter(fixtureCharacterSession(name))
}

func FixtureSkills(name string) MobileSkillsModel {
	s := fixtureCharacterSession(name)
	count := 6
	if name == "skills-many" {
		count = 24
	}
	s.Skills.Points = 3
	for i := 0; i < count; i++ {
		targetType := uint32(0)
		switch i % 3 {
		case 0:
			targetType = 1
		case 1:
			targetType = 2
		}
		s.Skills.List = append(s.Skills.List, session.Skill{
			ID: uint16(100 + i), Type: targetType, Level: i%5 + 1, MaxLevel: 10,
			SPCost: 5 + i, Range: 3 + i%6, Name: fmt.Sprintf("Fixture Skill %02d", i+1),
			Upgradable: i%2 == 0,
		})
	}
	return ProjectSkills(s)
}

func fixtureCharacterSession(name string) *session.Session {
	s := session.New()
	character := session.Character{
		ID: 1, Name: "Goro", Level: 42, JobLevel: 31, Job: db.JobKnight,
		HP: 742, MaxHP: 1000, SP: 184, MaxSP: 260, Money: 12500,
		Str: 52, Agi: 31, Vit: 44, Int: 18, Dex: 29, Luk: 12,
	}
	if name == "character-empty" {
		character.Name = "New Adventurer"
		character.Level, character.JobLevel = 1, 1
		character.HP, character.MaxHP, character.SP, character.MaxSP = 1, 1, 0, 0
	}
	s.SelectCharacter(character)
	s.Progress = session.Progress{BaseLevel: int(character.Level), JobLevel: int(character.JobLevel), BaseExp: 345_678, NextBaseExp: 1_000_000, JobExp: 12_345, NextJobExp: 100_000}
	s.Vitals = session.Vitals{HP: int(character.HP), MaxHP: int(character.MaxHP), SP: int(character.SP), MaxSP: int(character.MaxSP)}
	s.Inventory = session.Inventory{Zeny: character.Money, Weight: 423, MaxWeight: 1000}
	s.Stats = session.Stats{
		Points: 7,
		Str:    52, Agi: 31, Vit: 44, Int: 18, Dex: 29, Luk: 12,
		StrBonus: 4, AgiBonus: 2, VitBonus: 3, IntBonus: 1, DexBonus: 5, LukBonus: 0,
		StrCost: 6, AgiCost: 5, VitCost: 7, IntCost: 4, DexCost: 5, LukCost: 3,
		Attack: 87, AttackBonus: 14, Defense: 42, DefenseBonus: 8,
		MDefense: 18, MDefenseBonus: 3, Hit: 126, Flee: 94, FleeBonus: 2,
		Critical: 7, ASPD: 176, ASPDBonus: 4,
	}
	if name == "character-empty" {
		s.Progress = session.Progress{BaseLevel: 1, JobLevel: 1}
		s.Inventory = session.Inventory{Zeny: 0, MaxWeight: 1000}
		s.Stats = session.Stats{Str: 1, Agi: 1, Vit: 1, Int: 1, Dex: 1, Luk: 1}
	}
	return s
}
