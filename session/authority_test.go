package session

import (
	"testing"
	"time"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
)

func TestOfflineRuntimeCombatDropPickupAndSkill(t *testing.T) {
	state := New()
	state.CharID, state.PlayerX, state.PlayerY = 1, 10, 10
	state.Vitals = Vitals{HP: 100, MaxHP: 100, SP: 30, MaxSP: 30}
	state.Stats.Str = 5
	state.Skills.List = []Skill{{ID: db.SkillSMBash, Type: 1, Level: 1, MaxLevel: 10, SPCost: 5, Name: "Bash"}}
	offline := NewOfflineSession("prontera")
	offline.BindState(state)
	offline.AddMonster(OfflineMonster{ID: 9, Name: "Poring", X: 11, Y: 10, MaxHP: 30, Attack: 1, BaseEXP: 25, JobEXP: 10})

	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandAttackActor, ActorID: 9}) {
		t.Fatal("attack target was rejected")
	}
	offline.Update(700 * time.Millisecond)
	monster := offline.Monsters()[0]
	if monster.HP >= monster.MaxHP {
		t.Fatalf("normal attack did not damage monster: %+v", monster)
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandUseSkillOnActor, SkillID: db.SkillSMBash, Level: 1, ActorID: 9}) {
		t.Fatal("skill target was rejected")
	}
	if offline.Monsters()[0].State != MonsterDead {
		t.Fatalf("skill did not kill monster: %+v", offline.Monsters()[0])
	}
	if state.Progress.BaseExp != 25 || state.Progress.JobExp != 10 {
		t.Fatalf("experience not awarded: %+v", state.Progress)
	}
	drops := offline.Drops()
	if len(drops) != 1 || drops[0].ItemID != 501 {
		t.Fatalf("unexpected drops: %+v", drops)
	}
	state.PlayerX, state.PlayerY = drops[0].X, drops[0].Y
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandPickUpItem, ItemID: drops[0].ID}) {
		t.Fatal("drop pickup was rejected")
	}
	if len(offline.Drops()) != 0 || state.Inventory.Items[0].ItemID != 501 || state.Inventory.Items[0].Amount != 1 {
		t.Fatalf("pickup projection mismatch: drops=%+v inventory=%+v", offline.Drops(), state.Inventory.Items)
	}
}

func TestOfflineRuntimeShopAndStorage(t *testing.T) {
	state := New()
	state.CharID, state.PlayerX, state.PlayerY = 1, 1, 1
	state.Inventory = Inventory{Zeny: 100, Items: []InventoryItem{{Index: 1, ItemID: 909, Type: db.ItemTypeEtc, Amount: 2}}}
	offline := NewOfflineSession("prontera")
	offline.BindState(state)
	offline.AddShop(OfflineShop{NPCID: 7, Items: []OfflineShopItem{{ItemID: 501, Type: db.ItemTypeHealing, Price: 25, Stock: 5}}})
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandBuyItem, NPCID: 7, ItemIndex: 0, Quantity: 2}) {
		t.Fatal("shop purchase was rejected")
	}
	if state.Inventory.Zeny != 50 || !hasItemAmount(state.Inventory.Items, 501, 2) {
		t.Fatalf("purchase not applied: zeny=%d items=%+v", state.Inventory.Zeny, state.Inventory.Items)
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandOpenStorage}) {
		t.Fatal("storage open was rejected")
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandDepositItem, ItemIndex: 1, Quantity: 1}) {
		t.Fatal("storage deposit was rejected")
	}
	if state.Storage.Amount != 1 || state.Storage.Items[0].Amount != 1 {
		t.Fatalf("deposit not applied: %+v", state.Storage)
	}
	if !offline.HandleCommand(input.PlayerCommand{Kind: input.CommandWithdrawItem, ItemIndex: 1, Quantity: 1}) {
		t.Fatal("storage withdrawal was rejected")
	}
	if state.Storage.Amount != 0 || !hasItemAmount(state.Inventory.Items, 909, 2) {
		t.Fatalf("withdrawal not applied: storage=%+v inventory=%+v", state.Storage, state.Inventory.Items)
	}
}

func hasItemAmount(items []InventoryItem, itemID uint16, amount int) bool {
	for _, item := range items {
		if item.ItemID == itemID && item.Amount == amount {
			return true
		}
	}
	return false
}
