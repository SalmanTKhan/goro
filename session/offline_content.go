package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	OfflineContentFormat  = "goro-offline-content"
	OfflineContentVersion = 1
)

// OfflineContent is the generated, server-side content view consumed by the
// local-authority runtime. Client resources remain in the normal GRF/data
// root; this document only describes gameplay and map declarations.
type OfflineContent struct {
	Format   string                       `json:"format"`
	Version  int                          `json:"version"`
	Source   OfflineContentSource         `json:"source"`
	Maps     map[string]OfflineMap        `json:"maps"`
	Items    map[uint16]OfflineItem       `json:"items"`
	Monsters map[uint16]OfflineMonsterDef `json:"monsters"`
	Skills   map[uint16]OfflineSkillDef   `json:"skills"`
	Warnings []string                     `json:"warnings,omitempty"`
}

type OfflineContentSource struct {
	Emulator    string            `json:"emulator"`
	Profile     string            `json:"profile"`
	Packetver   int               `json:"packetver"`
	Commit      string            `json:"commit,omitempty"`
	Fingerprint string            `json:"fingerprint"`
	Files       map[string]string `json:"files,omitempty"`
}

type OfflineMap struct {
	Name   string           `json:"name"`
	NPCs   []OfflineNPC     `json:"npcs,omitempty"`
	Shops  []OfflineShopDef `json:"shops,omitempty"`
	Spawns []OfflineSpawn   `json:"spawns,omitempty"`
	Warps  []OfflineWarp    `json:"warps,omitempty"`
}

type OfflineItem struct {
	ID           uint16           `json:"id"`
	AegisName    string           `json:"aegis_name"`
	Name         string           `json:"name"`
	Type         uint8            `json:"type"`
	Buy          int64            `json:"buy"`
	Sell         int64            `json:"sell"`
	Weight       int              `json:"weight"`
	Locations    uint16           `json:"locations,omitempty"`
	StackAmount  int              `json:"stack_amount,omitempty"`
	NoDrop       bool             `json:"no_drop,omitempty"`
	NoSell       bool             `json:"no_sell,omitempty"`
	NoStorage    bool             `json:"no_storage,omitempty"`
	UseEffect    OfflineUseEffect `json:"use_effect,omitempty"`
	UseSupported bool             `json:"use_supported,omitempty"`
}

type OfflineUseEffect struct {
	HP           int `json:"hp,omitempty"`
	SP           int `json:"sp,omitempty"`
	HPMin        int `json:"hp_min,omitempty"`
	HPMax        int `json:"hp_max,omitempty"`
	SPMin        int `json:"sp_min,omitempty"`
	SPMax        int `json:"sp_max,omitempty"`
	HPPercent    int `json:"hp_percent,omitempty"`
	SPPercent    int `json:"sp_percent,omitempty"`
	HPPercentMin int `json:"hp_percent_min,omitempty"`
	HPPercentMax int `json:"hp_percent_max,omitempty"`
	SPPercentMin int `json:"sp_percent_min,omitempty"`
	SPPercentMax int `json:"sp_percent_max,omitempty"`
}

type OfflineMonsterDef struct {
	ID          uint16             `json:"id"`
	AegisName   string             `json:"aegis_name"`
	Name        string             `json:"name"`
	Level       int                `json:"level"`
	HP          int                `json:"hp"`
	AttackMin   int                `json:"attack_min"`
	AttackMax   int                `json:"attack_max"`
	AttackRange int                `json:"attack_range"`
	Defense     int                `json:"defense"`
	WalkSpeed   int                `json:"walk_speed"`
	AttackDelay int                `json:"attack_delay_ms"`
	BaseEXP     int64              `json:"base_exp"`
	JobEXP      int64              `json:"job_exp"`
	Drops       []OfflineDropTable `json:"drops,omitempty"`
}

type OfflineSkillDef struct {
	ID               uint16 `json:"id"`
	Name             string `json:"name"`
	MaxLevel         int    `json:"max_level"`
	Target           string `json:"target,omitempty"`
	Range            int    `json:"range,omitempty"`
	SPCost           int    `json:"sp_cost,omitempty"`
	CooldownMS       int    `json:"cooldown_ms,omitempty"`
	AfterCastDelayMS int    `json:"after_cast_delay_ms,omitempty"`
}

type OfflineNPC struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Sprite int16  `json:"sprite"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Dir    int    `json:"dir,omitempty"`
	ShopID uint32 `json:"shop_id,omitempty"`
}

type OfflineShopDef struct {
	ID    uint32               `json:"id"`
	NPCID uint32               `json:"npc_id"`
	Name  string               `json:"name"`
	Items []OfflineShopItemDef `json:"items"`
}

type OfflineShopItemDef struct {
	ItemID uint16 `json:"item_id"`
	Price  int64  `json:"price"`
	Stock  int    `json:"stock"`
}

type OfflineSpawn struct {
	ID           uint32 `json:"id"`
	MonsterID    uint16 `json:"monster_id"`
	Name         string `json:"name,omitempty"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	Radius       int    `json:"radius,omitempty"`
	Count        int    `json:"count"`
	RespawnMinMS int    `json:"respawn_min_ms,omitempty"`
	RespawnMaxMS int    `json:"respawn_max_ms,omitempty"`
}

type OfflineWarp struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Map    string `json:"map"`
	DestX  int    `json:"dest_x"`
	DestY  int    `json:"dest_y"`
}

// DecodeOfflineContent validates the generated content contract before it is
// allowed to seed local authority state.
func DecodeOfflineContent(data []byte) (OfflineContent, error) {
	var content OfflineContent
	if err := json.Unmarshal(data, &content); err != nil {
		return OfflineContent{}, fmt.Errorf("decode offline content: %w", err)
	}
	if content.Format != OfflineContentFormat {
		return OfflineContent{}, fmt.Errorf("unsupported offline content format %q", content.Format)
	}
	if content.Version != OfflineContentVersion {
		return OfflineContent{}, fmt.Errorf("unsupported offline content version %d", content.Version)
	}
	if len(content.Maps) == 0 {
		return OfflineContent{}, fmt.Errorf("offline content contains no maps")
	}
	if content.Source.Fingerprint == "" {
		return OfflineContent{}, fmt.Errorf("offline content has no source fingerprint")
	}
	for name, world := range content.Maps {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(world.Name) == "" {
			return OfflineContent{}, fmt.Errorf("offline content contains an unnamed map")
		}
		for _, shop := range world.Shops {
			if shop.ID == 0 || shop.NPCID == 0 || len(shop.Items) == 0 {
				return OfflineContent{}, fmt.Errorf("offline content contains an invalid shop on %s", name)
			}
		}
		for _, spawn := range world.Spawns {
			if spawn.ID == 0 || spawn.MonsterID == 0 || spawn.Count < 0 {
				return OfflineContent{}, fmt.Errorf("offline content contains an invalid spawn on %s", name)
			}
		}
		for _, warp := range world.Warps {
			if warp.ID == 0 || strings.TrimSpace(warp.Map) == "" {
				return OfflineContent{}, fmt.Errorf("offline content contains an invalid warp on %s", name)
			}
		}
	}
	for id, item := range content.Items {
		if id == 0 || item.ID != id || strings.TrimSpace(item.Name) == "" {
			return OfflineContent{}, fmt.Errorf("offline content contains an invalid item %d", id)
		}
	}
	for id, monster := range content.Monsters {
		if id == 0 || monster.ID != id || strings.TrimSpace(monster.Name) == "" || monster.HP <= 0 {
			return OfflineContent{}, fmt.Errorf("offline content contains an invalid monster %d", id)
		}
	}
	for id, skill := range content.Skills {
		if id == 0 || skill.ID != id || strings.TrimSpace(skill.Name) == "" {
			return OfflineContent{}, fmt.Errorf("offline content contains an invalid skill %d", id)
		}
	}
	return content, nil
}

// ContentFingerprint returns the canonical content hash used by save files and
// diagnostics. The source fingerprint is preferred because it remains stable
// across JSON formatting changes.
func (c OfflineContent) ContentFingerprint() string {
	if c.Source.Fingerprint != "" {
		return c.Source.Fingerprint
	}
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (c OfflineContent) Map(name string) (OfflineMap, bool) {
	world, ok := c.Maps[strings.ToLower(strings.TrimSpace(name))]
	if ok {
		return world, true
	}
	for key, candidate := range c.Maps {
		if strings.EqualFold(key, name) || strings.EqualFold(candidate.Name, name) {
			return candidate, true
		}
	}
	return OfflineMap{}, false
}

// NormalizeOfflineContent makes generated/test content deterministic without
// changing gameplay values.
func NormalizeOfflineContent(content *OfflineContent) {
	if content == nil {
		return
	}
	if content.Maps == nil {
		content.Maps = map[string]OfflineMap{}
	}
	for key, world := range content.Maps {
		world.Name = strings.ToLower(strings.TrimSpace(world.Name))
		sort.Slice(world.NPCs, func(i, j int) bool { return world.NPCs[i].ID < world.NPCs[j].ID })
		sort.Slice(world.Shops, func(i, j int) bool { return world.Shops[i].ID < world.Shops[j].ID })
		sort.Slice(world.Spawns, func(i, j int) bool { return world.Spawns[i].ID < world.Spawns[j].ID })
		sort.Slice(world.Warps, func(i, j int) bool { return world.Warps[i].ID < world.Warps[j].ID })
		for i := range world.Shops {
			sort.Slice(world.Shops[i].Items, func(a, b int) bool { return world.Shops[i].Items[a].ItemID < world.Shops[i].Items[b].ItemID })
		}
		content.Maps[strings.ToLower(strings.TrimSpace(key))] = world
		if strings.ToLower(strings.TrimSpace(key)) != key {
			delete(content.Maps, key)
		}
	}
}
