package mobileui

import (
	"fmt"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
)

type CharacterStatID uint8

const (
	CharacterStatSTR CharacterStatID = iota
	CharacterStatAGI
	CharacterStatVIT
	CharacterStatINT
	CharacterStatDEX
	CharacterStatLUK
)

type CharacterStatModel struct {
	ID     CharacterStatID
	Label  string
	Value  int
	Bonus  int
	Cost   int
	CanAdd bool
}

type MobileCharacterModel struct {
	Name          string
	JobName       string
	BaseLevel     int
	JobLevel      int
	BaseExp       int64
	NextBaseExp   int64
	JobExp        int64
	NextJobExp    int64
	HP            int
	MaxHP         int
	SP            int
	MaxSP         int
	Zeny          int64
	Weight        int
	MaxWeight     int
	StatPoints    int
	Stats         []CharacterStatModel
	Attack        int
	AttackBonus   int
	Defense       int
	DefenseBonus  int
	MDefense      int
	MDefenseBonus int
	Hit           int
	Flee          int
	FleeBonus     int
	Critical      int
	ASPD          int
	ASPDBonus     int
}

type MobileSkillModel struct {
	Index          int
	SkillID        uint16
	Name           string
	Level          int
	MaxLevel       int
	SPCost         int
	Range          int
	TargetMode     input.SkillTargetMode
	Upgradable     bool
	Usable         bool
	DisabledReason string
	IconKey        string
}

type SkillSelectionModel struct {
	SelectedIndex int
	HasSelection  bool
	Skill         MobileSkillModel
}

type MobileSkillsModel struct {
	Points    int
	Skills    []MobileSkillModel
	Selection SkillSelectionModel
}

func ProjectCharacter(s *session.Session) MobileCharacterModel {
	if s == nil {
		return MobileCharacterModel{}
	}
	character := s.SelectedCharacter()
	model := MobileCharacterModel{
		Name: character.Name, JobName: db.JobDisplayName(int(character.Job)),
		BaseLevel: s.Progress.BaseLevel, JobLevel: s.Progress.JobLevel,
		BaseExp: s.Progress.BaseExp, NextBaseExp: s.Progress.NextBaseExp,
		JobExp: s.Progress.JobExp, NextJobExp: s.Progress.NextJobExp,
		HP: s.Vitals.HP, MaxHP: s.Vitals.MaxHP, SP: s.Vitals.SP, MaxSP: s.Vitals.MaxSP,
		Zeny: s.Inventory.Zeny, Weight: s.Inventory.Weight, MaxWeight: s.Inventory.MaxWeight,
		StatPoints: s.Stats.Points,
		Attack:     s.Stats.Attack, AttackBonus: s.Stats.AttackBonus,
		Defense: s.Stats.Defense, DefenseBonus: s.Stats.DefenseBonus,
		MDefense: s.Stats.MDefense, MDefenseBonus: s.Stats.MDefenseBonus,
		Hit: s.Stats.Hit, Flee: s.Stats.Flee, FleeBonus: s.Stats.FleeBonus,
		Critical: s.Stats.Critical, ASPD: s.Stats.ASPD, ASPDBonus: s.Stats.ASPDBonus,
	}
	if model.Name == "" {
		model.Name = "Player"
	}
	model.Stats = []CharacterStatModel{
		{ID: CharacterStatSTR, Label: "STR", Value: s.Stats.Str, Bonus: s.Stats.StrBonus, Cost: s.Stats.StrCost, CanAdd: s.Stats.Points > 0},
		{ID: CharacterStatAGI, Label: "AGI", Value: s.Stats.Agi, Bonus: s.Stats.AgiBonus, Cost: s.Stats.AgiCost, CanAdd: s.Stats.Points > 0},
		{ID: CharacterStatVIT, Label: "VIT", Value: s.Stats.Vit, Bonus: s.Stats.VitBonus, Cost: s.Stats.VitCost, CanAdd: s.Stats.Points > 0},
		{ID: CharacterStatINT, Label: "INT", Value: s.Stats.Int, Bonus: s.Stats.IntBonus, Cost: s.Stats.IntCost, CanAdd: s.Stats.Points > 0},
		{ID: CharacterStatDEX, Label: "DEX", Value: s.Stats.Dex, Bonus: s.Stats.DexBonus, Cost: s.Stats.DexCost, CanAdd: s.Stats.Points > 0},
		{ID: CharacterStatLUK, Label: "LUK", Value: s.Stats.Luk, Bonus: s.Stats.LukBonus, Cost: s.Stats.LukCost, CanAdd: s.Stats.Points > 0},
	}
	return model
}

func ProjectSkills(s *session.Session) MobileSkillsModel {
	model := MobileSkillsModel{}
	if s == nil {
		return model
	}
	model.Points = s.Skills.Points
	model.Skills = make([]MobileSkillModel, 0, len(s.Skills.List))
	for index, skill := range s.Skills.List {
		targetMode := input.SkillTargetIdle
		switch {
		case skill.Type&1 != 0:
			targetMode = input.SkillTargetActor
		case skill.Type&2 != 0:
			targetMode = input.SkillTargetGround
		}
		usable := skill.SPCost <= 0 || s.Vitals.SP >= skill.SPCost
		entry := MobileSkillModel{
			Index: index, SkillID: skill.ID, Name: skill.Name, Level: skill.Level,
			MaxLevel: skill.MaxLevel, SPCost: skill.SPCost, Range: skill.Range,
			TargetMode: targetMode, Upgradable: skill.Upgradable, Usable: usable,
			IconKey: fmt.Sprintf("skill-%d", skill.ID),
		}
		if !usable {
			entry.DisabledReason = "Not enough SP"
		}
		model.Skills = append(model.Skills, entry)
	}
	return model
}
