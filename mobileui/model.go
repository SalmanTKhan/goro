package mobileui

import (
	"fmt"
	"sort"
	"time"

	"github.com/kivutar/goro/db"
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
	BaseLevel  int
	JobLevel   int
	StatPoints  int
	SkillPoints int
	BaseExp     int64
	NextBaseExp int64
	JobExp      int64
	NextJobExp  int64
	Sitting     bool
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

type ShortcutKind uint8

const (
	ShortcutNone ShortcutKind = iota
	ShortcutSkill
	ShortcutItem
)

type ShortcutSlotModel struct {
	Kind  ShortcutKind
	Skill SkillSlotModel
	Item  InventoryItemModel
}

type StatusEffectModel struct {
	ID         uint16
	IconKey    string
	Name       string
	Remaining  time.Duration
	Beneficial bool
}

type EmoteModel struct {
	ID    uint8
	Label string
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
	Player    PlayerHUDModel
	Target    TargetHUDModel
	Skills    []SkillSlotModel
	Shortcuts []ShortcutSlotModel
	Statuses  []StatusEffectModel
	Emotes    []EmoteModel
	Loot      []LootItemModel
	Minimap   MinimapModel
}

// HUDSource is the small, read-only projection boundary between authoritative
// game state and the mobile presentation model.
type HUDSource struct {
	Player    PlayerHUDModel
	Target    TargetHUDModel
	Skills    []SkillSlotModel
	Shortcuts []ShortcutSlotModel
	Statuses  []StatusEffectModel
	Emotes    []EmoteModel
	Loot      []LootItemModel
	Minimap   MinimapModel
}

func Project(source HUDSource) MobileHUDModel {
	return MobileHUDModel{
		Player: source.Player, Target: source.Target,
		Skills:    append([]SkillSlotModel(nil), source.Skills...),
		Shortcuts: append([]ShortcutSlotModel(nil), source.Shortcuts...),
		Statuses:  append([]StatusEffectModel(nil), source.Statuses...),
		Emotes:   append([]EmoteModel(nil), source.Emotes...),
		Loot:     append([]LootItemModel(nil), source.Loot...),
		Minimap:  projectMinimap(source.Minimap),
	}
}

func ShortcutCount(model MobileHUDModel) int {
	if len(model.Shortcuts) > 0 {
		return len(model.Shortcuts)
	}
	return len(model.Skills)
}

func ShortcutAt(model MobileHUDModel, index int) (ShortcutSlotModel, bool) {
	if index < 0 {
		return ShortcutSlotModel{}, false
	}
	if len(model.Shortcuts) > 0 {
		if index >= len(model.Shortcuts) {
			return ShortcutSlotModel{}, false
		}
		return model.Shortcuts[index], true
	}
	if index >= len(model.Skills) {
		return ShortcutSlotModel{}, false
	}
	return ShortcutSlotModel{Kind: ShortcutSkill, Skill: model.Skills[index]}, true
}

func shortcutItemModel(s *session.Session, itemID uint16) InventoryItemModel {
	out := InventoryItemModel{ItemID: itemID, Identified: true}
	if s == nil || itemID == 0 {
		return out
	}
	for _, item := range s.Inventory.Items {
		if item.ItemID != itemID || item.Amount <= 0 {
			continue
		}
		out.Index = item.Index
		out.ItemID = item.ItemID
		out.Identified = item.Identified
		out.Quantity = item.Amount
		out.Type = item.Type
		out.Usable = db.ItemTypeIsUsable(item.Type)
		return out
	}
	return out
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
		Player: PlayerHUDModel{
			Name: s.SelectedCharacter().Name,
			HP: s.Vitals.HP, MaxHP: s.Vitals.MaxHP,
			SP: s.Vitals.SP, MaxSP: s.Vitals.MaxSP,
			BaseLevel: s.Progress.BaseLevel, JobLevel: s.Progress.JobLevel,
			StatPoints: s.Stats.Points, SkillPoints: s.Skills.Points,
			BaseExp: s.Progress.BaseExp, NextBaseExp: s.Progress.NextBaseExp,
			JobExp: s.Progress.JobExp, NextJobExp: s.Progress.NextJobExp,
		},
		Target:  target,
		Minimap: MinimapModel{Visible: true, MapName: s.Zone.MapName, PlayerX: s.PlayerX, PlayerY: s.PlayerY, PlayerDir: s.PlayerDir},
	}
	for i, slot := range s.Hotkeys.Slots {
		if slot.ID == 0 {
			continue
		}
		switch slot.Type {
		case session.HotkeyTypeItem:
			model.Shortcuts = append(model.Shortcuts, ShortcutSlotModel{Kind: ShortcutItem, Item: shortcutItemModel(s, uint16(slot.ID))})
		case session.HotkeyTypeSkill:
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
			projected := skillSlotModel(i, skill, level, s.Vitals.SP)
			model.Skills = append(model.Skills, projected)
			model.Shortcuts = append(model.Shortcuts, ShortcutSlotModel{Kind: ShortcutSkill, Skill: projected})
		}
	}
	if len(model.Shortcuts) == 0 {
		for i, skill := range s.Skills.List {
			if skill.ID == 0 {
				continue
			}
			projected := skillSlotModel(i, skill, skill.Level, s.Vitals.SP)
			model.Skills = append(model.Skills, projected)
			model.Shortcuts = append(model.Shortcuts, ShortcutSlotModel{Kind: ShortcutSkill, Skill: projected})
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
