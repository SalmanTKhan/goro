package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineSessionSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "offline-save.json")
	offline := NewOfflineSession("prontera")
	want := New()
	want.PlayerX, want.PlayerY, want.PlayerDir = 81, 99, 3
	want.Vitals = Vitals{HP: 72, MaxHP: 100, SP: 15, MaxSP: 30}
	want.Progress = Progress{BaseLevel: 4, JobLevel: 2}
	want.Inventory = Inventory{Zeny: 42, Items: []InventoryItem{{Index: 1, ItemID: 501, Type: 0, Amount: 7}, {Index: 2, ItemID: 1201, Type: 5, Location: 2, Equipped: true}}}
	if err := offline.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got := New()
	loaded := NewOfflineSession("other")
	if err := loaded.Load(path, got); err != nil {
		t.Fatal(err)
	}
	if loaded.MapName != "prontera" || got.PlayerX != 81 || got.PlayerY != 99 || got.PlayerDir != 3 || got.Vitals.HP != 72 || len(got.Inventory.Items) != 2 || !got.Inventory.Items[1].Equipped {
		t.Fatalf("offline save mismatch: map=%s position=%d,%d,%d hp=%d items=%d equipped=%t", loaded.MapName, got.PlayerX, got.PlayerY, got.PlayerDir, got.Vitals.HP, len(got.Inventory.Items), got.Inventory.Items[1].Equipped)
	}
}

func TestOfflineSessionLegacySavePreservesNewDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-save.json")
	legacy := OfflineSave{MapName: "prontera", PlayerX: 4, PlayerY: 5, Vitals: Vitals{HP: 10, MaxHP: 20}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	state := New()
	state.Skills = Skills{List: []Skill{{ID: 5, Name: "Bash"}}}
	state.Storage = Storage{MaxAmount: 300}
	offline := NewOfflineSession("prontera")
	if err := offline.Load(path, state); err != nil {
		t.Fatal(err)
	}
	if len(state.Skills.List) != 1 || state.Storage.MaxAmount != 300 {
		t.Fatalf("legacy load discarded new defaults: skills=%+v storage=%+v", state.Skills, state.Storage)
	}
}

func TestOfflineSessionSaveLoadPersistsRuntimeWorldState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime-save.json")
	offline := NewOfflineSession("prontera")
	offline.AddMonster(OfflineMonster{ID: 9002, Name: "Poring", X: 75, Y: 101, HP: 17, MaxHP: 45, State: MonsterChase, TargetID: 1})
	offline.Runtime().drops[50001] = OfflineDrop{ID: 50001, ItemID: 501, Amount: 2, X: 75, Y: 101, Identified: true}
	state := New()
	state.PlayerX, state.PlayerY = 78, 98
	if err := offline.Save(path, state); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved OfflineSave
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Version != 5 || len(saved.Runtime.Monsters) != 1 || len(saved.Runtime.Drops) != 1 {
		t.Fatalf("runtime save version/state = %d monsters=%d drops=%d", saved.Version, len(saved.Runtime.Monsters), len(saved.Runtime.Drops))
	}

	loaded := NewOfflineSession("other")
	loaded.AddMonster(OfflineMonster{ID: 1234, Name: "stale"})
	loadedState := New()
	if err := loaded.Load(path, loadedState); err != nil {
		t.Fatal(err)
	}
	monsters := loaded.Monsters()
	drops := loaded.Drops()
	if len(monsters) != 1 || monsters[0].ID != 9002 || monsters[0].HP != 17 || monsters[0].State != MonsterChase || len(drops) != 1 || drops[0].ID != 50001 {
		t.Fatalf("runtime state mismatch: monsters=%+v drops=%+v", monsters, drops)
	}
}

// TestProfileSnapshotStaysSaveable closes a loop where the client rejected its
// own data: the profile's appearance is projected from the world character,
// whose hair may sit outside the range SetProfile validates. Taking it raw
// produced a snapshot that could never be saved back, so the character screen's
// save silently failed for any character with hair below the minimum.
func TestProfileSnapshotStaysSaveable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		hair  int16
		color uint8
	}{
		{"hair below the minimum", 1, 0},
		{"hair above the maximum", 99, 0},
		{"colour past the last palette", 5, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &Session{}
			state.SelectCharacter(Character{
				ID: 1, Name: "Offline Adventurer", Hair: tc.hair, HairColor: tc.color,
			})
			offline := NewOfflineSession("prontera")

			profile, ok := offline.ProfileSnapshot(state)
			if !ok {
				t.Fatal("no profile snapshot")
			}
			if err := offline.SetProfile(profile); err != nil {
				t.Fatalf("the client rejected its own snapshot: %v", err)
			}
		})
	}
}
