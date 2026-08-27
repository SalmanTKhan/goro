package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// OfflineSession marks the local-authority runtime without duplicating the
// gameplay state already projected by Session. The world, player, inventory,
// and equipment remain in the normal Session/World structures so presentation
// code has one source of truth in both online and offline modes.
type OfflineSession struct {
	MapName     string
	Paused      bool
	Started     time.Time
	TargetID    uint32
	TargetName  string
	TargetHP    int
	TargetMaxHP int
	TargetNPC   bool
	SavePath    string
	NPCFlags    map[uint32]bool
	Profile     OfflineProfile
	HasProfile  bool
	state       *Session
	runtime     *OfflineRuntime
	content     *OfflineContent
}

// OfflineProfile is the durable local identity and appearance record for the
// offline character. Runtime progression remains in Session and is saved
// alongside this record; the mobile presentation only receives a projection.
type OfflineProfile struct {
	ID        uint32
	Name      string
	Sex       byte
	HairStyle int16
	HairColor uint8
	Stats     [6]uint8
}

type OfflineSave struct {
	Version            int
	ContentFingerprint string
	MapName            string
	PlayerX            int
	PlayerY            int
	PlayerDir          int
	Vitals             Vitals
	Progress           Progress
	Inventory          Inventory
	Storage            Storage
	Stats              Stats
	Skills             Skills
	Hotkeys            Hotkeys
	Statuses           Statuses
	TargetID           uint32
	TargetName         string
	TargetHP           int
	TargetMaxHP        int
	TargetNPC          bool
	Selected           Character
	Sex                byte
	Profile            OfflineProfile
	HasProfile         bool
	Cooldowns          map[uint16]int64
	NPCFlags           map[uint32]bool
	Runtime            OfflineRuntimeSave
}

// OfflineRuntimeSave contains durable local-world state. Events, the fixed
// step accumulator, and wall-clock scheduling are deliberately transient and
// are recreated on load.
type OfflineRuntimeSave struct {
	Monsters   []OfflineMonster `json:"monsters,omitempty"`
	Drops      []OfflineDrop    `json:"drops,omitempty"`
	NextDropID uint32           `json:"next_drop_id,omitempty"`
}

func NewOfflineSession(mapName string) *OfflineSession {
	if mapName == "" {
		mapName = "prontera"
	}
	return &OfflineSession{MapName: mapName, Started: time.Now()}
}

func (s *OfflineSession) Pause() {
	if s != nil {
		s.Paused = true
	}
}

func (s *OfflineSession) Resume() {
	if s != nil {
		s.Paused = false
	}
}

func (s *OfflineSession) Save(path string, state *Session) error {
	if s == nil || state == nil || path == "" {
		return fmt.Errorf("offline save requires session and path")
	}
	var cooldowns map[uint16]int64
	runtimeSave := OfflineRuntimeSave{}
	if runtime := s.Runtime(); runtime != nil {
		cooldowns = make(map[uint16]int64, len(runtime.cooldowns))
		for id, remaining := range runtime.cooldowns {
			cooldowns[id] = remaining.Milliseconds()
		}
		runtimeSave.NextDropID = runtime.nextDropID
		runtimeSave.Monsters = make([]OfflineMonster, 0, len(runtime.monsters))
		for _, monster := range runtime.monsters {
			monster.DropTable = append([]OfflineDropTable(nil), monster.DropTable...)
			runtimeSave.Monsters = append(runtimeSave.Monsters, monster)
		}
		sort.Slice(runtimeSave.Monsters, func(i, j int) bool { return runtimeSave.Monsters[i].ID < runtimeSave.Monsters[j].ID })
		runtimeSave.Drops = make([]OfflineDrop, 0, len(runtime.drops))
		for _, drop := range runtime.drops {
			runtimeSave.Drops = append(runtimeSave.Drops, drop)
		}
		sort.Slice(runtimeSave.Drops, func(i, j int) bool { return runtimeSave.Drops[i].ID < runtimeSave.Drops[j].ID })
	}
	contentFingerprint := ""
	if s.content != nil {
		contentFingerprint = s.content.ContentFingerprint()
	}
	s.syncProfile(state)
	snapshot := OfflineSave{Version: 5, ContentFingerprint: contentFingerprint, MapName: s.MapName, PlayerX: state.PlayerX, PlayerY: state.PlayerY, PlayerDir: state.PlayerDir, Vitals: state.Vitals, Progress: state.Progress, Inventory: state.Inventory, Storage: state.Storage, Stats: state.Stats, Skills: state.Skills, Hotkeys: state.Hotkeys, Statuses: state.Statuses, TargetID: s.TargetID, TargetName: s.TargetName, TargetHP: s.TargetHP, TargetMaxHP: s.TargetMaxHP, TargetNPC: s.TargetNPC, Selected: state.Selected, Sex: state.Sex, Profile: s.Profile, HasProfile: s.HasProfile, Cooldowns: cooldowns, NPCFlags: s.NPCFlags, Runtime: runtimeSave}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode offline save: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create offline save directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write offline save: %w", err)
	}
	s.SavePath = path
	return nil
}

func (s *OfflineSession) Load(path string, state *Session) error {
	if s == nil || state == nil || path == "" {
		return fmt.Errorf("offline load requires session and path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var snapshot OfflineSave
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("decode offline save: %w", err)
	}
	if snapshot.MapName != "" {
		s.MapName = snapshot.MapName
	}
	state.PlayerX, state.PlayerY, state.PlayerDir = snapshot.PlayerX, snapshot.PlayerY, snapshot.PlayerDir
	state.Vitals, state.Progress, state.Inventory, state.Stats = snapshot.Vitals, snapshot.Progress, snapshot.Inventory, snapshot.Stats
	if snapshot.Version >= 5 && (snapshot.HasProfile || snapshot.Profile.Name != "") {
		s.Profile, s.HasProfile = snapshot.Profile, true
		state.Selected, state.Sex = snapshot.Selected, snapshot.Sex
		if state.Selected.ID != 0 {
			state.CharID = state.Selected.ID
		}
	}
	if snapshot.Version >= 2 && (snapshot.Storage.MaxAmount > 0 || snapshot.Storage.Amount > 0 || len(snapshot.Storage.Items) > 0) {
		state.Storage = snapshot.Storage
	}
	if len(snapshot.Skills.List) > 0 || snapshot.Skills.Points > 0 {
		state.Skills = snapshot.Skills
	}
	if snapshot.Hotkeys.Loaded || len(snapshot.Hotkeys.Slots) > 0 {
		state.Hotkeys = snapshot.Hotkeys
	}
	if snapshot.Statuses.Active != nil {
		state.Statuses = snapshot.Statuses
	}
	s.TargetID, s.TargetName, s.TargetHP, s.TargetMaxHP, s.TargetNPC = snapshot.TargetID, snapshot.TargetName, snapshot.TargetHP, snapshot.TargetMaxHP, snapshot.TargetNPC
	if snapshot.NPCFlags != nil {
		s.NPCFlags = snapshot.NPCFlags
	}
	if runtime := s.Runtime(); runtime != nil {
		contentChanged := s.content != nil && (snapshot.Version < 4 || snapshot.ContentFingerprint == "" || snapshot.ContentFingerprint != s.content.ContentFingerprint())
		if s.content != nil {
			if err := s.activateContentMap(s.MapName, state.PlayerX, state.PlayerY); err != nil {
				return err
			}
		}
		runtime.cooldowns = make(map[uint16]time.Duration, len(snapshot.Cooldowns))
		for id, milliseconds := range snapshot.Cooldowns {
			runtime.cooldowns[id] = time.Duration(milliseconds) * time.Millisecond
		}
		if snapshot.Version >= 3 && !contentChanged {
			runtime.monsters = make(map[uint32]OfflineMonster, len(snapshot.Runtime.Monsters))
			for _, monster := range snapshot.Runtime.Monsters {
				monster.DropTable = append([]OfflineDropTable(nil), monster.DropTable...)
				runtime.monsters[monster.ID] = monster
			}
			runtime.drops = make(map[uint32]OfflineDrop, len(snapshot.Runtime.Drops))
			for _, drop := range snapshot.Runtime.Drops {
				runtime.drops[drop.ID] = drop
			}
			if snapshot.Runtime.NextDropID != 0 {
				runtime.nextDropID = snapshot.Runtime.NextDropID
			}
			runtime.events = nil
			runtime.accumulator = 0
		}
	}
	s.SavePath = path
	s.EnsureProfile(state)
	return nil
}
