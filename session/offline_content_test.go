package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kivutar/goro/input"
)

func testOfflineContent() OfflineContent {
	return OfflineContent{
		Format: OfflineContentFormat, Version: OfflineContentVersion,
		Source: OfflineContentSource{Emulator: "test", Profile: "pre-re", Packetver: 20080910, Fingerprint: "content-a"},
		Items: map[uint16]OfflineItem{
			501:  {ID: 501, AegisName: "Red_Potion", Name: "Red Potion", Type: 0, Buy: 50, Sell: 25, Weight: 10, UseSupported: true, UseEffect: OfflineUseEffect{HP: 45, HPMin: 45, HPMax: 65}},
			909:  {ID: 909, AegisName: "Jellopy", Name: "Jellopy", Type: 3, Buy: 10, Sell: 5, Weight: 1},
			1201: {ID: 1201, AegisName: "Knife_", Name: "Knife", Type: 5, Buy: 50, Sell: 25, Weight: 60},
		},
		Monsters: map[uint16]OfflineMonsterDef{1002: {ID: 1002, AegisName: "PORING", Name: "Poring", Level: 1, HP: 20, AttackMin: 1, AttackMax: 2, AttackRange: 1, Defense: 1, WalkSpeed: 400, AttackDelay: 1000, BaseEXP: 2, JobEXP: 1, Drops: []OfflineDropTable{{ItemID: 909, Type: 3, Amount: 1, Rate: 10000}}}},
		Skills:   map[uint16]OfflineSkillDef{5: {ID: 5, Name: "Bash", MaxLevel: 10, Range: 6, SPCost: 8, CooldownMS: 350}},
		Maps: map[string]OfflineMap{
			"prontera":   {Name: "prontera", NPCs: []OfflineNPC{{ID: 10, Name: "Tool Dealer", X: 2, Y: 2, ShopID: 11}}, Shops: []OfflineShopDef{{ID: 11, NPCID: 10, Name: "Tool Dealer", Items: []OfflineShopItemDef{{ItemID: 501, Price: 50}}}}, Warps: []OfflineWarp{{ID: 12, Name: "South Gate", X: 5, Y: 5, Width: 1, Height: 1, Map: "prt_fild00", DestX: 10, DestY: 10}}},
			"prt_fild00": {Name: "prt_fild00", Spawns: []OfflineSpawn{{ID: 20, MonsterID: 1002, X: 11, Y: 10, Count: 1}}},
		},
	}
}

func TestOfflineContentAuthorityUsesImportedDefinitions(t *testing.T) {
	content := testOfflineContent()
	state := New()
	state.CharID, state.PlayerX, state.PlayerY = 1, 10, 10
	state.Vitals = Vitals{HP: 100, MaxHP: 100, SP: 30, MaxSP: 30}
	state.Stats.Str = 5
	state.Skills.List = []Skill{{ID: 5, Type: 1, Level: 1, MaxLevel: 10, Name: "Bash"}}
	offline := NewOfflineSession("prontera")
	offline.BindState(state)
	if err := offline.LoadContent(content, "prt_fild00"); err != nil {
		t.Fatal(err)
	}
	monster := offline.Monsters()[0]
	if monster.ID == 9002 || monster.MaxHP != 20 || monster.AttackMax != 2 || monster.AttackDelay != time.Second {
		t.Fatalf("imported monster = %+v", monster)
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandAttackActor, ActorID: monster.ID}) {
		t.Fatal("imported monster attack rejected")
	}
	offline.Update(700 * time.Millisecond)
	if offline.Monsters()[0].HP >= 20 {
		t.Fatal("normal attack did not use imported monster")
	}
	state.Inventory = Inventory{Zeny: 100, Items: []InventoryItem{{Index: 1, ItemID: 501, Type: 0, Amount: 1}}}
	state.Vitals.HP = 20
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandUseItem, ItemIndex: 1}) || state.Vitals.HP < 65 || state.Vitals.HP > 85 {
		t.Fatalf("imported use effect not applied: %+v", state.Vitals)
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandUseSkillOnActor, SkillID: 5, Level: 1, ActorID: monster.ID}) || state.Vitals.SP != 22 {
		t.Fatalf("imported skill cost not applied: sp=%d", state.Vitals.SP)
	}
	if len(offline.Drops()) != 1 || offline.Drops()[0].ItemID != 909 {
		t.Fatalf("imported drop table = %+v", offline.Drops())
	}
	state.PlayerX, state.PlayerY = offline.Drops()[0].X, offline.Drops()[0].Y
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandPickUpItem, ItemID: offline.Drops()[0].ID}) {
		t.Fatal("imported drop pickup was rejected")
	}
	if err := offline.ActivateMap("prontera", 5, 5); err != nil {
		t.Fatal(err)
	}
	if !offline.TryWarpAt(5, 5) {
		t.Fatal("imported warp was not accepted")
	}
	change, ok := offline.ConsumeMapChange()
	if !ok || change.Map != "prt_fild00" || change.X != 10 || change.Y != 10 {
		t.Fatalf("map change = %+v, %t", change, ok)
	}
	if err := offline.ActivateMap(change.Map, change.X, change.Y); err != nil {
		t.Fatal(err)
	}
	if offline.MapName != "prt_fild00" || len(offline.Monsters()) != 1 {
		t.Fatalf("activated map = %s monsters=%+v", offline.MapName, offline.Monsters())
	}
}

func TestOfflineContentAuthorityUsesImportedShopPrices(t *testing.T) {
	content := testOfflineContent()
	state := New()
	state.CharID, state.PlayerX, state.PlayerY = 1, 2, 2
	state.Inventory.Zeny = 100
	offline := NewOfflineSession("prontera")
	offline.BindState(state)
	if err := offline.LoadContent(content); err != nil {
		t.Fatal(err)
	}
	if len(offline.Shops()) != 1 || offline.Shops()[0].Items[0].Price != 50 {
		t.Fatalf("imported shop = %+v", offline.Shops())
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandBuyItem, NPCID: 10, ItemIndex: 0, Quantity: 1}) {
		t.Fatal("imported shop purchase was rejected")
	}
	if state.Inventory.Zeny != 50 || !hasItemAmount(state.Inventory.Items, 501, 1) {
		t.Fatalf("imported purchase = zeny %d inventory %+v", state.Inventory.Zeny, state.Inventory.Items)
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandSellItem, ItemIndex: state.Inventory.Items[0].Index, Quantity: 1}) || state.Inventory.Zeny != 75 {
		t.Fatalf("imported sell price not applied: zeny=%d inventory=%+v", state.Inventory.Zeny, state.Inventory.Items)
	}
}

func TestOfflineContentFingerprintReseedsWorldPreservingProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	content := testOfflineContent()
	state := New()
	state.PlayerX, state.PlayerY, state.Progress.BaseExp = 10, 10, 77
	offline := NewOfflineSession("prontera")
	offline.BindState(state)
	if err := offline.LoadContent(content, "prt_fild00"); err != nil {
		t.Fatal(err)
	}
	state.Progress.BaseExp = 77
	if err := offline.Save(path, state); err != nil {
		t.Fatal(err)
	}
	changed := testOfflineContent()
	changed.Source.Fingerprint = "content-b"
	loaded := NewOfflineSession("prontera")
	loadedState := New()
	loaded.BindState(loadedState)
	if err := loaded.LoadContent(changed, "prt_fild00"); err != nil {
		t.Fatal(err)
	}
	if err := loaded.Load(path, loadedState); err != nil {
		t.Fatal(err)
	}
	if loadedState.Progress.BaseExp != 77 || len(loaded.Monsters()) != 1 || len(loaded.Drops()) != 0 {
		t.Fatalf("changed content load lost progress or failed to reseed: progress=%+v monsters=%+v drops=%+v", loadedState.Progress, loaded.Monsters(), loaded.Drops())
	}
}
