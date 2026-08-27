package session

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"
)

// OfflineMapChange is the local equivalent of a server map-change packet.
// The game package consumes it through ConsumeMapChange and reuses the normal
// world-mode map loading path.
type OfflineMapChange struct {
	Map string
	X   int
	Y   int
}

// LoadContent installs a generated content pack and seeds the selected map.
// It is intentionally separate from NewOfflineSession so tests and tools can
// validate/decode a pack before binding it to a player account.
func (s *OfflineSession) LoadContent(content OfflineContent, mapNames ...string) error {
	if s == nil {
		return fmt.Errorf("offline content requires a session")
	}
	NormalizeOfflineContent(&content)
	if _, err := DecodeOfflineContentMust(content); err != nil {
		return err
	}
	mapName := s.MapName
	if len(mapNames) > 0 && strings.TrimSpace(mapNames[0]) != "" {
		mapName = mapNames[0]
	}
	world, ok := content.Map(mapName)
	if !ok {
		return fmt.Errorf("offline content has no map %q", mapName)
	}
	s.content = &content
	s.MapName = world.Name
	r := s.Runtime()
	r.contentLoaded = true
	r.items = make(map[uint16]OfflineItem, len(content.Items))
	for id, item := range content.Items {
		r.items[id] = item
	}
	r.skills = make(map[uint16]OfflineSkillDef, len(content.Skills))
	for id, skill := range content.Skills {
		r.skills[id] = skill
	}
	return s.activateContentMap(world.Name, 0, 0)
}

// DecodeOfflineContentMust keeps the runtime validation in one place while
// allowing already-decoded content to be checked without a JSON round trip.
func DecodeOfflineContentMust(content OfflineContent) (OfflineContent, error) {
	data, err := json.Marshal(content)
	if err != nil {
		return OfflineContent{}, fmt.Errorf("encode offline content for validation: %w", err)
	}
	return DecodeOfflineContent(data)
}

func (s *OfflineSession) activateContentMap(mapName string, x, y int) error {
	if s == nil || s.content == nil {
		return fmt.Errorf("offline content is not loaded")
	}
	world, ok := s.content.Map(mapName)
	if !ok {
		return fmt.Errorf("offline content has no map %q", mapName)
	}
	s.MapName = world.Name
	if s.state != nil && (x != 0 || y != 0) {
		s.state.PlayerX, s.state.PlayerY = x, y
	}
	r := s.Runtime()
	r.monsters = map[uint32]OfflineMonster{}
	r.drops = map[uint32]OfflineDrop{}
	r.shops = map[uint32]OfflineShop{}
	r.npcs = map[uint32]OfflineNPC{}
	r.warps = map[uint32]OfflineWarp{}
	r.pendingMapChange = nil
	r.attackID, r.attackReady = 0, 0
	for _, npc := range world.NPCs {
		r.npcs[npc.ID] = npc
	}
	for _, warp := range world.Warps {
		if _, ok := s.content.Map(warp.Map); !ok {
			return fmt.Errorf("map %s warp %q points to missing map %q", world.Name, warp.Name, warp.Map)
		}
		r.warps[warp.ID] = warp
	}
	for _, shop := range world.Shops {
		runtimeShop := OfflineShop{NPCID: shop.NPCID, Name: shop.Name}
		for _, item := range shop.Items {
			definition, ok := r.items[item.ItemID]
			if !ok {
				return fmt.Errorf("shop %q references missing item %d", shop.Name, item.ItemID)
			}
			price := item.Price
			if price < 0 {
				price = definition.Buy
			}
			if price < 0 {
				return fmt.Errorf("shop %q has negative price for item %d", shop.Name, item.ItemID)
			}
			runtimeShop.Items = append(runtimeShop.Items, OfflineShopItem{ItemID: item.ItemID, Type: definition.Type, Name: definition.Name, Price: price, Stock: item.Stock})
		}
		r.shops[shop.NPCID] = runtimeShop
	}
	for _, spawn := range world.Spawns {
		definition, ok := s.content.Monsters[spawn.MonsterID]
		if !ok {
			return fmt.Errorf("spawn %d references missing monster %d", spawn.ID, spawn.MonsterID)
		}
		count := spawn.Count
		if count <= 0 {
			count = 1
		}
		for index := 0; index < count; index++ {
			spawnX, spawnY := spawn.X, spawn.Y
			if spawnX == 0 && spawnY == 0 {
				baseX, baseY := x, y
				if s.state != nil {
					baseX, baseY = s.state.PlayerX, s.state.PlayerY
				}
				spawnX, spawnY = baseX+1+(index%5), baseY+(index/5)%5
			}
			if spawn.Radius > 0 {
				offset := int(contentHash(fmt.Sprintf("%d/%d", spawn.ID, index)) % uint64(spawn.Radius*2+1))
				spawnX += offset - spawn.Radius
				spawnY += int(contentHash(fmt.Sprintf("%d/%d/y", spawn.ID, index))%uint64(spawn.Radius*2+1)) - spawn.Radius
			}
			attackDelay := time.Duration(definition.AttackDelay) * time.Millisecond
			if attackDelay <= 0 {
				attackDelay = 1200 * time.Millisecond
			}
			respawn := time.Duration(spawn.RespawnMinMS) * time.Millisecond
			if respawn <= 0 {
				respawn = time.Duration(spawn.RespawnMaxMS) * time.Millisecond
			}
			if respawn <= 0 {
				respawn = 8 * time.Second
			}
			monster := OfflineMonster{ID: contentID(fmt.Sprintf("monster|%s|%d|%d", world.Name, spawn.ID, index)), SourceKey: fmt.Sprintf("%d#%d", spawn.ID, index), Name: definition.Name, Job: int16(definition.ID), Level: definition.Level, X: spawnX, Y: spawnY, SpawnX: spawnX, SpawnY: spawnY, MaxHP: definition.HP, Attack: definition.AttackMin, AttackMax: definition.AttackMax, Defense: definition.Defense, WalkSpeed: definition.WalkSpeed, AttackDelay: attackDelay, RespawnAfter: respawn, BaseEXP: definition.BaseEXP, JobEXP: definition.JobEXP, DropTable: append([]OfflineDropTable(nil), definition.Drops...)}
			monster.AttackRange = definition.AttackRange
			s.addContentMonster(monster)
		}
	}
	return nil
}

func (s *OfflineSession) addContentMonster(monster OfflineMonster) {
	if monster.MaxHP > 0 && monster.HP <= 0 {
		monster.HP = monster.MaxHP
	}
	if monster.State == MonsterDead {
		monster.State = MonsterIdle
	}
	s.Runtime().monsters[monster.ID] = monster
}

func (s *OfflineSession) ActivateMap(mapName string, x, y int) error {
	return s.activateContentMap(mapName, x, y)
}

func (s *OfflineSession) Content() (OfflineContent, bool) {
	if s == nil || s.content == nil {
		return OfflineContent{}, false
	}
	return *s.content, true
}

func (s *OfflineSession) ContentFingerprint() string {
	if s == nil || s.content == nil {
		return ""
	}
	return s.content.ContentFingerprint()
}

func (s *OfflineSession) NPCs() []OfflineNPC {
	r := s.Runtime()
	if r == nil {
		return nil
	}
	result := make([]OfflineNPC, 0, len(r.npcs))
	for _, npc := range r.npcs {
		result = append(result, npc)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *OfflineSession) Warps() []OfflineWarp {
	r := s.Runtime()
	if r == nil {
		return nil
	}
	result := make([]OfflineWarp, 0, len(r.warps))
	for _, warp := range r.warps {
		result = append(result, warp)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *OfflineSession) Item(id uint16) (OfflineItem, bool) {
	r := s.Runtime()
	if r == nil {
		return OfflineItem{}, false
	}
	item, ok := r.items[id]
	return item, ok
}

func (s *OfflineSession) Skill(id uint16) (OfflineSkillDef, bool) {
	r := s.Runtime()
	if r == nil {
		return OfflineSkillDef{}, false
	}
	skill, ok := r.skills[id]
	return skill, ok
}

func (s *OfflineSession) TargetForActor(id uint32) (name string, hp, maxHP int, npc bool, ok bool) {
	r := s.Runtime()
	if r == nil {
		return "", 0, 0, false, false
	}
	if monster, found := r.monsters[id]; found {
		return monster.Name, monster.HP, monster.MaxHP, false, true
	}
	if actor, found := r.npcs[id]; found {
		return actor.Name, 1, 1, true, true
	}
	if warp, found := r.warps[id]; found {
		return warp.Name, 1, 1, true, true
	}
	return "", 0, 0, false, false
}

func (s *OfflineSession) TryWarpAt(x, y int) bool {
	r := s.Runtime()
	if r == nil {
		return false
	}
	for _, warp := range r.warps {
		width, height := warp.Width, warp.Height
		if width <= 0 {
			width = 1
		}
		if height <= 0 {
			height = 1
		}
		if abs(x-warp.X) > width || abs(y-warp.Y) > height {
			continue
		}
		r.pendingMapChange = &OfflineMapChange{Map: warp.Map, X: warp.DestX, Y: warp.DestY}
		return true
	}
	return false
}

func (s *OfflineSession) ConsumeMapChange() (OfflineMapChange, bool) {
	r := s.Runtime()
	if r == nil {
		return OfflineMapChange{}, false
	}
	if r.pendingMapChange == nil {
		return OfflineMapChange{}, false
	}
	change := *r.pendingMapChange
	r.pendingMapChange = nil
	return change, true
}

// PeekMapChange reports the next local transition without consuming it. The
// world mode uses this while waiting for a CDN overlay so a failed or delayed
// optional patch cannot discard the requested map change.
func (s *OfflineSession) PeekMapChange() (OfflineMapChange, bool) {
	r := s.Runtime()
	if r == nil || r.pendingMapChange == nil {
		return OfflineMapChange{}, false
	}
	return *r.pendingMapChange, true
}

func contentID(key string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	value := hash.Sum32()
	if value == 0 {
		return 1
	}
	return value
}

func contentHash(key string) uint64 {
	digest := sha256.Sum256([]byte(key))
	return binary.LittleEndian.Uint64(digest[:8])
}

// deterministicRoll is a small deterministic PRNG step. It is seeded from
// content fingerprint, source entity, and sequence rather than wall-clock or
// process-global randomness, so repeated builds and save/load produce the same
// imported drop decisions.
func deterministicRoll(key string) uint32 {
	state := contentHash(key)
	if state == 0 {
		state = 0x9e3779b97f4a7c15
	}
	state ^= state >> 12
	state ^= state << 25
	state ^= state >> 27
	return uint32((state * 0x2545f4914f6cdd1d) >> 32)
}
