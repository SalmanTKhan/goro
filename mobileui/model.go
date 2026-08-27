package mobileui

import (
	"fmt"
	"sort"
	"time"

	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/session"
)

type TargetRelation uint8

const (
	TargetNone TargetRelation = iota
	TargetFriendly
	TargetHostile
	TargetNPC
)

type PlayerHUDModel struct {
	Name      string
	HP        int
	MaxHP     int
	SP        int
	MaxSP     int
	BaseLevel int
	JobLevel  int
}

type TargetHUDModel struct {
	Visible  bool
	ID       uint32
	Name     string
	HP       int
	MaxHP    int
	Relation TargetRelation
}

type SkillSlotModel struct {
	Index             int
	SkillID           uint16
	Name              string
	Level             int
	MaxLevel          int
	IconKey           string
	CooldownRemaining time.Duration
	CooldownTotal     time.Duration
	Usable            bool
	TargetMode        input.SkillTargetMode
}

type StatusEffectModel struct {
	ID         uint16
	IconKey    string
	Name       string
	Remaining  time.Duration
	Beneficial bool
}

// LootItemModel is the read-only mobile projection of one world drop. DropID
// is the runtime identity used by CommandPickUpItem; item metadata remains
// owned by the resource/session layers.
type LootItemModel struct {
	DropID      uint32
	ItemID      uint16
	Identified  bool
	Name        string
	Quantity    int
	X           int
	Y           int
	Distance    int
	PickupReady bool
}

type MinimapModel struct {
	Visible   bool
	MapName   string
	PlayerX   int
	PlayerY   int
	PlayerDir int
	Raster    MinimapRaster
	Markers   []MinimapMarkerModel
}

type MinimapMarkerKind uint8

const (
	MinimapMarkerHostile MinimapMarkerKind = iota + 1
	MinimapMarkerNPC
	MinimapMarkerWarp
	MinimapMarkerItem
)

// MinimapMarkerModel is a renderer-neutral location marker projected from
// the current world. It carries no actor or packet type so the mobile layer
// remains independent of gameplay and renderer packages.
type MinimapMarkerModel struct {
	ID       uint32
	Name     string
	X        int
	Y        int
	Kind     MinimapMarkerKind
	Selected bool
}

// MinimapRaster is a renderer-neutral terrain projection. Cells are indexed
// left-to-right, top-to-bottom; zero is empty, one is walkable/top terrain,
// and two is wall-only terrain.
type MinimapRaster struct {
	Width  int
	Height int
	Cells  []uint8
}

type MobileHUDModel struct {
	Player   PlayerHUDModel
	Target   TargetHUDModel
	Skills   []SkillSlotModel
	Statuses []StatusEffectModel
	Loot     []LootItemModel
	Minimap  MinimapModel
}

// HUDSource is the small, read-only projection boundary between authoritative
// game state and the mobile presentation model.
type HUDSource struct {
	Player   PlayerHUDModel
	Target   TargetHUDModel
	Skills   []SkillSlotModel
	Statuses []StatusEffectModel
	Loot     []LootItemModel
	Minimap  MinimapModel
}

func Project(source HUDSource) MobileHUDModel {
	return MobileHUDModel{
		Player: source.Player, Target: source.Target,
		Skills:   append([]SkillSlotModel(nil), source.Skills...),
		Statuses: append([]StatusEffectModel(nil), source.Statuses...),
		Loot:     append([]LootItemModel(nil), source.Loot...),
		Minimap:  projectMinimap(source.Minimap),
	}
}

func projectMinimap(source MinimapModel) MinimapModel {
	source.Markers = append([]MinimapMarkerModel(nil), source.Markers...)
	source.Raster.Cells = append([]uint8(nil), source.Raster.Cells...)
	return source
}

// skillSlotModel projects one learned skill into a hotbar slot. TargetMode and
// IconKey mirror ProjectSkills (character_skills_model.go). CooldownRemaining /
// CooldownTotal stay zero: the session does not track per-skill cooldown yet, so
// the renderer's cooldown scrim is driven only by Usable here.
func skillSlotModel(index int, skill session.Skill, level, sp int) SkillSlotModel {
	targetMode := input.SkillTargetIdle
	switch {
	case skill.Type&1 != 0:
		targetMode = input.SkillTargetActor
	case skill.Type&2 != 0:
		targetMode = input.SkillTargetGround
	}
	return SkillSlotModel{
		Index:      index,
		SkillID:    skill.ID,
		Name:       skill.Name,
		Level:      level,
		MaxLevel:   skill.MaxLevel,
		IconKey:    fmt.Sprintf("skill-%d", skill.ID),
		Usable:     sp >= skill.SPCost,
		TargetMode: targetMode,
	}
}

// ProjectSession copies only session data needed by the HUD. Target selection,
// cooldown tracking, and world coordinates remain supplied by their owners.
func ProjectSession(s *session.Session, target TargetHUDModel) MobileHUDModel {
	if s == nil {
		return MobileHUDModel{Target: target}
	}
	model := MobileHUDModel{
		Player:  PlayerHUDModel{Name: s.SelectedCharacter().Name, HP: s.Vitals.HP, MaxHP: s.Vitals.MaxHP, SP: s.Vitals.SP, MaxSP: s.Vitals.MaxSP, BaseLevel: s.Progress.BaseLevel, JobLevel: s.Progress.JobLevel},
		Target:  target,
		Minimap: MinimapModel{Visible: true, MapName: s.Zone.MapName, PlayerX: s.PlayerX, PlayerY: s.PlayerY, PlayerDir: s.PlayerDir},
	}
	for i, slot := range s.Hotkeys.Slots {
		if slot.ID == 0 {
			continue
		}
		var skill session.Skill
		var ok bool
		for _, candidate := range s.Skills.List {
			if candidate.ID == uint16(slot.ID) {
				skill, ok = candidate, true
				break
			}
		}
		if !ok {
			continue
		}
		level := int(slot.Level)
		if level <= 0 {
			level = skill.Level
		}
		model.Skills = append(model.Skills, skillSlotModel(i, skill, level, s.Vitals.SP))
	}
	// Fallback: when no explicit hotkey layout exists (common offline, or before
	// the server's ZC_SHORTCUT_KEY_LIST arrives) mirror the learned skills onto
	// the bar so mobile players can still cast more than nothing. An explicit
	// layout, even a single slot, always wins.
	if len(model.Skills) == 0 {
		for i, skill := range s.Skills.List {
			if skill.ID == 0 {
				continue
			}
			model.Skills = append(model.Skills, skillSlotModel(i, skill, skill.Level, s.Vitals.SP))
		}
	}
	ids := make([]int, 0, len(s.Statuses.Active))
	for id := range s.Statuses.Active {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, id := range ids {
		effect := s.Statuses.Active[uint16(id)]
		remaining := time.Duration(0)
		if effect.HasDuration {
			remaining = time.Until(effect.ExpiresAt)
		}
		model.Statuses = append(model.Statuses, StatusEffectModel{ID: effect.ID, Remaining: remaining})
	}
	return model
}
