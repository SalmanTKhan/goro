package session

import (
	"fmt"
	"time"

	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
)

// GameSession is the authority boundary consumed by presentation and input.
// OnlineSession can implement the same contract later without making mobile
// code aware of transport details.
type GameSession interface {
	State() *Session
	HandleCommand(input.PlayerCommand) bool
	Update(time.Duration)
}

type OfflineMonsterState uint8

const (
	MonsterIdle OfflineMonsterState = iota
	MonsterWander
	MonsterAcquireTarget
	MonsterChase
	MonsterAttack
	MonsterDead
	MonsterRespawn
)

type OfflineMonster struct {
	ID            uint32
	SourceKey     string
	Name          string
	Job           int16
	Level         int
	X             int
	Y             int
	SpawnX        int
	SpawnY        int
	HP            int
	MaxHP         int
	Attack        int
	AttackMax     int
	AttackRange   int
	Defense       int
	WalkSpeed     int
	AttackDelay   time.Duration
	AttackReady   time.Duration
	State         OfflineMonsterState
	TargetID      uint32
	RespawnAfter  time.Duration
	RespawnRemain time.Duration
	BaseEXP       int64
	JobEXP        int64
	DropTable     []OfflineDropTable
}

type OfflineDropTable struct {
	ItemID uint16 `json:"item_id"`
	Type   uint8  `json:"type"`
	Amount int    `json:"amount"`
	Rate   int    `json:"rate,omitempty"`
}

type OfflineDrop struct {
	ID         uint32
	ItemID     uint16
	Type       uint8
	Amount     int
	X          int
	Y          int
	Identified bool
}

type OfflineShopItem struct {
	ItemID uint16
	Type   uint8
	Name   string
	Price  int64
	Stock  int
}

type OfflineShop struct {
	NPCID uint32
	Name  string
	Items []OfflineShopItem
}

type OfflineEventKind uint8

const (
	OfflineEventDamage OfflineEventKind = iota + 1
	OfflineEventMonsterDied
	OfflineEventDropCreated
	OfflineEventPickup
	OfflineEventLevelUp
	OfflineEventShopOpened
	OfflineEventStorageOpened
)

type OfflineEvent struct {
	Kind     OfflineEventKind
	ActorID  uint32
	ItemID   uint16
	DropID   uint32
	SkillID  uint16
	Damage   int
	Amount   int
	BaseEXP  int64
	JobEXP   int64
	Position input.WorldPosition
}

const offlineFixedStep = 50 * time.Millisecond

// The runtime owns local authority, while Session remains the single shared
// player-state projection used by desktop UI, mobile UI, save/load, and the
// renderer. Monster/drop maps are authority state with projection snapshots.
type OfflineRuntime struct {
	owner            *OfflineSession
	state            *Session
	monsters         map[uint32]OfflineMonster
	drops            map[uint32]OfflineDrop
	shops            map[uint32]OfflineShop
	npcs             map[uint32]OfflineNPC
	warps            map[uint32]OfflineWarp
	items            map[uint16]OfflineItem
	skills           map[uint16]OfflineSkillDef
	contentLoaded    bool
	pendingMapChange *OfflineMapChange
	nextDropID       uint32
	attackID         uint32
	attackReady      time.Duration
	cooldowns        map[uint16]time.Duration
	accumulator      time.Duration
	events           []OfflineEvent
}

func (s *OfflineSession) State() *Session {
	if s == nil {
		return nil
	}
	return s.state
}

func (s *OfflineSession) BindState(state *Session) {
	if s != nil {
		s.state = state
	}
}

func (s *OfflineSession) Runtime() *OfflineRuntime {
	if s == nil {
		return nil
	}
	if s.runtime == nil {
		s.runtime = &OfflineRuntime{owner: s, monsters: map[uint32]OfflineMonster{}, drops: map[uint32]OfflineDrop{}, shops: map[uint32]OfflineShop{}, npcs: map[uint32]OfflineNPC{}, warps: map[uint32]OfflineWarp{}, items: map[uint16]OfflineItem{}, skills: map[uint16]OfflineSkillDef{}, cooldowns: map[uint16]time.Duration{}, nextDropID: 50000}
	}
	s.runtime.owner = s
	s.runtime.state = s.state
	return s.runtime
}

func (s *OfflineSession) AddMonster(monster OfflineMonster) {
	r := s.Runtime()
	if r == nil || monster.ID == 0 {
		return
	}
	if monster.MaxHP <= 0 {
		monster.MaxHP = 30
	}
	if monster.AttackMax <= 0 {
		monster.AttackMax = monster.Attack
	}
	if monster.AttackRange <= 0 {
		monster.AttackRange = 1
	}
	if monster.WalkSpeed <= 0 {
		monster.WalkSpeed = 150
	}
	if monster.AttackDelay <= 0 {
		monster.AttackDelay = 1200 * time.Millisecond
	}
	if monster.HP <= 0 {
		monster.HP = monster.MaxHP
	}
	if monster.SpawnX == 0 && monster.SpawnY == 0 {
		monster.SpawnX, monster.SpawnY = monster.X, monster.Y
	}
	if monster.State == MonsterDead {
		monster.State = MonsterIdle
	}
	if monster.RespawnAfter <= 0 {
		monster.RespawnAfter = 8 * time.Second
	}
	if monster.BaseEXP <= 0 {
		monster.BaseEXP = 25
	}
	if monster.JobEXP <= 0 {
		monster.JobEXP = 10
	}
	if len(monster.DropTable) == 0 && !r.contentLoaded {
		monster.DropTable = []OfflineDropTable{{ItemID: 501, Type: db.ItemTypeHealing, Amount: 1}}
	}
	r.monsters[monster.ID] = monster
}

func (s *OfflineSession) AddShop(shop OfflineShop) {
	r := s.Runtime()
	if r == nil || shop.NPCID == 0 {
		return
	}
	r.shops[shop.NPCID] = shop
}

func (s *OfflineSession) Monsters() []OfflineMonster {
	r := s.Runtime()
	if r == nil {
		return nil
	}
	result := make([]OfflineMonster, 0, len(r.monsters))
	for _, monster := range r.monsters {
		monster.DropTable = append([]OfflineDropTable(nil), monster.DropTable...)
		result = append(result, monster)
	}
	return result
}

func (s *OfflineSession) Drops() []OfflineDrop {
	r := s.Runtime()
	if r == nil {
		return nil
	}
	result := make([]OfflineDrop, 0, len(r.drops))
	for _, drop := range r.drops {
		result = append(result, drop)
	}
	return result
}

func (s *OfflineSession) Shops() []OfflineShop {
	r := s.Runtime()
	if r == nil {
		return nil
	}
	result := make([]OfflineShop, 0, len(r.shops))
	for _, shop := range r.shops {
		shop.Items = append([]OfflineShopItem(nil), shop.Items...)
		result = append(result, shop)
	}
	return result
}

func (s *OfflineSession) DrainEvents() []OfflineEvent {
	r := s.Runtime()
	if r == nil || len(r.events) == 0 {
		return nil
	}
	events := append([]OfflineEvent(nil), r.events...)
	r.events = r.events[:0]
	return events
}

func (s *OfflineSession) Update(dt time.Duration) {
	if s == nil || s.Paused || dt <= 0 {
		return
	}
	r := s.Runtime()
	if r == nil || r.state == nil {
		return
	}
	if dt > 250*time.Millisecond {
		dt = 250 * time.Millisecond
	}
	r.accumulator += dt
	for r.accumulator >= offlineFixedStep {
		r.accumulator -= offlineFixedStep
		r.step(offlineFixedStep)
	}
}

func (r *OfflineRuntime) step(dt time.Duration) {
	for id, cooldown := range r.cooldowns {
		cooldown -= dt
		if cooldown <= 0 {
			delete(r.cooldowns, id)
		} else {
			r.cooldowns[id] = cooldown
		}
	}
	if r.attackID != 0 {
		r.attackReady -= dt
		if r.attackReady <= 0 {
			r.attackReady = 700 * time.Millisecond
			r.normalAttack(r.attackID)
		}
	}
	for id, monster := range r.monsters {
		switch monster.State {
		case MonsterDead:
			monster.RespawnRemain -= dt
			if monster.RespawnRemain <= 0 {
				monster.X, monster.Y = monster.SpawnX, monster.SpawnY
				monster.HP, monster.TargetID = monster.MaxHP, 0
				monster.State = MonsterRespawn
			}
		case MonsterRespawn:
			monster.State = MonsterIdle
		default:
			if r.distanceToPlayer(monster.X, monster.Y) <= 5 {
				if monster.TargetID == 0 {
					// Acquire is visible state, but the fixture monster does not
					// overlap the player until the player explicitly attacks it.
					monster.State = MonsterAcquireTarget
					r.monsters[id] = monster
					continue
				}
				if r.distanceToPlayer(monster.X, monster.Y) <= monster.AttackRange {
					monster.State = MonsterAttack
				} else {
					monster.State = MonsterChase
					monster.X, monster.Y = stepToward(monster.X, monster.Y, r.state.PlayerX, r.state.PlayerY)
				}
				if monster.State == MonsterAttack && monster.Attack > 0 {
					// Monsters use a deliberately slow local cadence for a readable
					// vertical slice; fixed-step simulation keeps it deterministic.
					monster.AttackReady -= dt
					if monster.AttackReady <= 0 {
						monster.AttackReady = monster.AttackDelay
						damage := monster.Attack
						if monster.AttackMax > monster.Attack {
							damage += int(deterministicRoll(fmt.Sprintf("attack/%d/%d", monster.ID, r.nextDropID)) % uint32(monster.AttackMax-monster.Attack+1))
						}
						r.state.Vitals.HP = max(0, r.state.Vitals.HP-damage)
					}
				}
			} else {
				monster.State = MonsterIdle
			}
		}
		r.monsters[id] = monster
	}
}

func (r *OfflineRuntime) HandleCommand(command input.PlayerCommand) bool {
	if r == nil || r.state == nil || r.state.Dead {
		return false
	}
	switch command.Kind {
	case input.CommandAttackActor:
		monster, ok := r.monsters[command.ActorID]
		if !ok || monster.State == MonsterDead || monster.State == MonsterRespawn {
			return false
		}
		if r.distanceToPlayer(monster.X, monster.Y) > 6 {
			return false
		}
		r.attackID, r.attackReady = command.ActorID, 0
		return true
	case input.CommandUseSkill:
		return r.useSkill(command.SkillID, command.Level, 0, input.WorldPosition{}, false)
	case input.CommandUseSkillOnActor:
		return r.useSkill(command.SkillID, command.Level, command.ActorID, input.WorldPosition{}, false)
	case input.CommandUseSkillAtPosition:
		return r.useSkill(command.SkillID, command.Level, 0, command.Position, true)
	case input.CommandCancelAction:
		r.attackID, r.attackReady = 0, 0
		return true
	case input.CommandPickUpItem:
		return r.pickup(command.ItemID)
	case input.CommandUseItem:
		return r.useItem(command.ItemIndex)
	case input.CommandEquipItem:
		return r.equip(command.ItemIndex, command.EquipmentSlot, true)
	case input.CommandUnequipItem:
		return r.equip(command.ItemIndex, 0, false)
	case input.CommandDropItem:
		return r.dropInventory(command.ItemIndex, command.Quantity)
	case input.CommandOpenShop:
		if _, ok := r.shops[command.NPCID]; !ok {
			return false
		}
		r.events = append(r.events, OfflineEvent{Kind: OfflineEventShopOpened, ActorID: command.NPCID})
		if r.owner != nil {
			if r.owner.NPCFlags == nil {
				r.owner.NPCFlags = map[uint32]bool{}
			}
			r.owner.NPCFlags[command.NPCID] = true
		}
		return true
	case input.CommandBuyItem:
		return r.buy(command.NPCID, command.ItemIndex, command.Quantity)
	case input.CommandSellItem:
		return r.sell(command.ItemIndex, command.Quantity)
	case input.CommandOpenStorage:
		r.state.Storage.Open = true
		r.events = append(r.events, OfflineEvent{Kind: OfflineEventStorageOpened})
		return true
	case input.CommandDepositItem:
		return r.deposit(command.ItemIndex, command.Quantity)
	case input.CommandWithdrawItem:
		return r.withdraw(command.ItemIndex, command.Quantity)
	default:
		return false
	}
}

func (s *OfflineSession) HandleCommand(command input.PlayerCommand) bool {
	if s == nil {
		return false
	}
	return s.Runtime().HandleCommand(command)
}

func (r *OfflineRuntime) normalAttack(id uint32) {
	monster, ok := r.monsters[id]
	if !ok || monster.State == MonsterDead || monster.State == MonsterRespawn || r.distanceToPlayer(monster.X, monster.Y) > 6 {
		return
	}
	damage := 8 + r.state.Stats.Str - monster.Defense
	if damage < 1 {
		damage = 1
	}
	r.damageMonster(&monster, damage, 0)
	r.monsters[id] = monster
}

func (r *OfflineRuntime) useSkill(skillID uint16, level int, actorID uint32, position input.WorldPosition, ground bool) bool {
	if skillID == 0 {
		return false
	}
	if level <= 0 {
		level = 1
	}
	for _, skill := range r.state.Skills.List {
		if skill.ID != skillID {
			continue
		}
		if skill.Level > 0 && level > skill.Level {
			level = skill.Level
		}
		if cooldown, ok := r.cooldowns[skillID]; ok && cooldown > 0 {
			return false
		}
		attackRange := 9
		cost := skill.SPCost
		cooldown := 2 * time.Second
		if r.contentLoaded {
			definition, exists := r.skills[skillID]
			if !exists {
				return false
			}
			cost = definition.SPCost
			cooldown = time.Duration(definition.CooldownMS) * time.Millisecond
			attackRange = 1
			if definition.Range > 0 {
				attackRange = definition.Range
			}
		}
		if !ground && actorID != 0 {
			monster, ok := r.monsters[actorID]
			if !ok || monster.State == MonsterDead || r.distanceToPlayer(monster.X, monster.Y) > attackRange {
				return false
			}
		}
		if cost <= 0 && !r.contentLoaded {
			cost = 5
		}
		if r.state.Vitals.SP < cost {
			return false
		}
		r.state.Vitals.SP -= cost
		if cooldown > 0 {
			r.cooldowns[skillID] = cooldown
		}
		if skill.Type&4 != 0 || skillID == 28 { // self-target/heal rows
			r.state.Vitals.HP = min(r.state.Vitals.MaxHP, r.state.Vitals.HP+20*level)
			return true
		}
		if ground {
			for id, monster := range r.monsters {
				if monster.State == MonsterDead || distance(monster.X, monster.Y, int(position.X), int(position.Y)) > 1 {
					continue
				}
				r.damageMonster(&monster, 18*level, skillID)
				r.monsters[id] = monster
			}
			return true
		}
		if actorID == 0 {
			return false
		}
		monster := r.monsters[actorID]
		r.damageMonster(&monster, 25*level, skillID)
		r.monsters[actorID] = monster
		return true
	}
	return false
}

func (r *OfflineRuntime) damageMonster(monster *OfflineMonster, damage int, skillID uint16) {
	if monster == nil || monster.State == MonsterDead {
		return
	}
	if damage < 1 {
		damage = 1
	}
	monster.HP -= damage
	if monster.HP < 0 {
		monster.HP = 0
	}
	if r.attackID == monster.ID && monster.HP == 0 {
		r.attackID, r.attackReady = 0, 0
	}
	r.events = append(r.events, OfflineEvent{Kind: OfflineEventDamage, ActorID: monster.ID, Damage: damage, SkillID: skillID, Amount: monster.HP, Position: input.WorldPosition{X: float64(monster.X), Y: float64(monster.Y)}})
	if monster.HP > 0 {
		return
	}
	monster.State = MonsterDead
	monster.RespawnRemain = monster.RespawnAfter
	r.state.Progress.BaseExp += monster.BaseEXP
	r.state.Progress.JobExp += monster.JobEXP
	if r.state.Progress.NextBaseExp <= 0 {
		r.state.Progress.NextBaseExp = 100
	}
	if r.state.Progress.NextJobExp <= 0 {
		r.state.Progress.NextJobExp = 50
	}
	for r.state.Progress.BaseExp >= r.state.Progress.NextBaseExp {
		r.state.Progress.BaseExp -= r.state.Progress.NextBaseExp
		r.state.Progress.BaseLevel++
		r.state.Progress.NextBaseExp += 50
		r.state.Vitals.MaxHP += 10
		r.state.Vitals.HP = r.state.Vitals.MaxHP
	}
	for r.state.Progress.JobExp >= r.state.Progress.NextJobExp {
		r.state.Progress.JobExp -= r.state.Progress.NextJobExp
		r.state.Progress.JobLevel++
		r.state.Progress.NextJobExp += 25
	}
	r.events = append(r.events, OfflineEvent{Kind: OfflineEventMonsterDied, ActorID: monster.ID, BaseEXP: monster.BaseEXP, JobEXP: monster.JobEXP})
	for index, table := range monster.DropTable {
		if r.contentLoaded && (table.Rate <= 0 || int(deterministicRoll(fmt.Sprintf("drop/%s/%d/%d/%d", r.owner.content.ContentFingerprint(), monster.ID, r.nextDropID, index))%10000) >= table.Rate) {
			continue
		}
		amount := table.Amount
		if amount <= 0 {
			amount = 1
		}
		r.nextDropID++
		drop := OfflineDrop{ID: r.nextDropID, ItemID: table.ItemID, Type: table.Type, Amount: amount, X: monster.X, Y: monster.Y, Identified: true}
		r.drops[drop.ID] = drop
		r.events = append(r.events, OfflineEvent{Kind: OfflineEventDropCreated, DropID: drop.ID, ItemID: drop.ItemID, Amount: drop.Amount, Position: input.WorldPosition{X: float64(drop.X), Y: float64(drop.Y)}})
	}
}

func (r *OfflineRuntime) pickup(id uint32) bool {
	drop, ok := r.drops[id]
	if !ok || distance(r.state.PlayerX, r.state.PlayerY, drop.X, drop.Y) > 1 {
		return false
	}
	if !r.addInventory(drop.ItemID, drop.Type, drop.Amount) {
		return false
	}
	delete(r.drops, id)
	r.events = append(r.events, OfflineEvent{Kind: OfflineEventPickup, DropID: id, ItemID: drop.ItemID, Amount: drop.Amount})
	return true
}

func (r *OfflineRuntime) addInventory(itemID uint16, itemType uint8, amount int) bool {
	if amount <= 0 {
		return false
	}
	if r.contentLoaded {
		definition, ok := r.items[itemID]
		if !ok || (r.state.Inventory.MaxWeight > 0 && r.state.Inventory.Weight+r.itemWeight(itemID)*amount > r.state.Inventory.MaxWeight) {
			return false
		}
		itemType = definition.Type
	}
	for i := range r.state.Inventory.Items {
		item := &r.state.Inventory.Items[i]
		if item.ItemID == itemID && !item.Equipped {
			item.Amount += amount
			r.state.Inventory.Weight += r.itemWeight(itemID) * amount
			return true
		}
	}
	maxIndex := uint16(0)
	for _, item := range r.state.Inventory.Items {
		if item.Index > maxIndex {
			maxIndex = item.Index
		}
	}
	r.state.Inventory.Items = append(r.state.Inventory.Items, InventoryItem{Index: maxIndex + 1, ItemID: itemID, Type: itemType, Amount: amount, Identified: true})
	r.state.Inventory.Weight += r.itemWeight(itemID) * amount
	return true
}

func (r *OfflineRuntime) itemWeight(itemID uint16) int {
	if item, ok := r.items[itemID]; ok && item.Weight > 0 {
		return item.Weight
	}
	return 1
}

func (r *OfflineRuntime) useItem(index uint16) bool {
	for i := range r.state.Inventory.Items {
		item := &r.state.Inventory.Items[i]
		if item.Index != index || item.Amount <= 0 || !db.ItemTypeIsUsable(item.Type) {
			continue
		}
		definition, hasDefinition := r.items[item.ItemID]
		if !hasDefinition || !definition.UseSupported {
			return false
		}
		item.Amount--
		r.state.Inventory.Weight -= r.itemWeight(item.ItemID)
		if r.state.Inventory.Weight < 0 {
			r.state.Inventory.Weight = 0
		}
		if item.Amount == 0 {
			r.state.Inventory.Items = append(r.state.Inventory.Items[:i], r.state.Inventory.Items[i+1:]...)
		}
		effect := definition.UseEffect
		r.nextDropID++ // also advances the deterministic local PRNG sequence
		hp := effect.HP
		sp := effect.SP
		hpPercent := effect.HPPercent
		spPercent := effect.SPPercent
		if effect.HPMax > effect.HPMin {
			hp = effect.HPMin + int(deterministicRoll(fmt.Sprintf("item-hp/%d/%d", item.ItemID, r.nextDropID))%uint32(effect.HPMax-effect.HPMin+1))
		}
		if effect.SPMax > effect.SPMin {
			sp = effect.SPMin + int(deterministicRoll(fmt.Sprintf("item-sp/%d/%d", item.ItemID, r.nextDropID))%uint32(effect.SPMax-effect.SPMin+1))
		}
		if effect.HPPercentMax > effect.HPPercentMin {
			hpPercent = effect.HPPercentMin + int(deterministicRoll(fmt.Sprintf("item-hp-percent/%d/%d", item.ItemID, r.nextDropID))%uint32(effect.HPPercentMax-effect.HPPercentMin+1))
		}
		if effect.SPPercentMax > effect.SPPercentMin {
			spPercent = effect.SPPercentMin + int(deterministicRoll(fmt.Sprintf("item-sp-percent/%d/%d", item.ItemID, r.nextDropID))%uint32(effect.SPPercentMax-effect.SPPercentMin+1))
		}
		r.state.Vitals.HP = min(r.state.Vitals.MaxHP, r.state.Vitals.HP+hp+r.state.Vitals.MaxHP*hpPercent/100)
		r.state.Vitals.SP = min(r.state.Vitals.MaxSP, r.state.Vitals.SP+sp+r.state.Vitals.MaxSP*spPercent/100)
		return true
	}
	return false
}

func (r *OfflineRuntime) equip(index, slot uint16, equipped bool) bool {
	for i := range r.state.Inventory.Items {
		item := &r.state.Inventory.Items[i]
		if item.Index != index {
			continue
		}
		item.Equipped = equipped
		item.Equip = equipped || !db.ItemTypeIsUsable(item.Type)
		if equipped && slot != 0 {
			item.Location = slot
		}
		return true
	}
	return false
}

func (r *OfflineRuntime) dropInventory(index uint16, quantity int) bool {
	if quantity <= 0 {
		return false
	}
	for i := range r.state.Inventory.Items {
		item := &r.state.Inventory.Items[i]
		if item.Index != index || item.Amount <= 0 {
			continue
		}
		if r.contentLoaded {
			if definition, ok := r.items[item.ItemID]; !ok || definition.NoDrop {
				return false
			}
		}
		if quantity > item.Amount {
			quantity = item.Amount
		}
		r.state.Inventory.Weight -= r.itemWeight(item.ItemID) * quantity
		if r.state.Inventory.Weight < 0 {
			r.state.Inventory.Weight = 0
		}
		item.Amount -= quantity
		if item.Amount == 0 {
			r.state.Inventory.Items = append(r.state.Inventory.Items[:i], r.state.Inventory.Items[i+1:]...)
		}
		r.nextDropID++
		drop := OfflineDrop{ID: r.nextDropID, ItemID: item.ItemID, Type: item.Type, Amount: quantity, X: r.state.PlayerX, Y: r.state.PlayerY, Identified: item.Identified}
		r.drops[drop.ID] = drop
		return true
	}
	return false
}

func (r *OfflineRuntime) buy(npcID uint32, shopIndex uint16, quantity int) bool {
	shop, ok := r.shops[npcID]
	if !ok || int(shopIndex) >= len(shop.Items) || quantity <= 0 {
		return false
	}
	item := &shop.Items[shopIndex]
	if item.Stock > 0 && item.Stock < quantity {
		return false
	}
	total := item.Price * int64(quantity)
	if r.state.Inventory.Zeny < total {
		return false
	}
	if !r.addInventory(item.ItemID, item.Type, quantity) {
		return false
	}
	r.state.Inventory.Zeny -= total
	if item.Stock > 0 {
		item.Stock -= quantity
	}
	return true
}

func (r *OfflineRuntime) sell(index uint16, quantity int) bool {
	if quantity <= 0 {
		return false
	}
	for i := range r.state.Inventory.Items {
		item := &r.state.Inventory.Items[i]
		if item.Index != index || item.Amount < quantity {
			continue
		}
		if definition, ok := r.items[item.ItemID]; r.contentLoaded && (!ok || definition.NoSell) {
			return false
		}
		item.Amount -= quantity
		if item.Amount == 0 {
			r.state.Inventory.Items = append(r.state.Inventory.Items[:i], r.state.Inventory.Items[i+1:]...)
		}
		sellPrice := int64(5)
		if definition, ok := r.items[item.ItemID]; ok {
			sellPrice = definition.Sell
		}
		r.state.Inventory.Zeny += int64(quantity) * sellPrice
		return true
	}
	return false
}

func (r *OfflineRuntime) deposit(index uint16, quantity int) bool {
	if quantity <= 0 {
		return false
	}
	for i := range r.state.Inventory.Items {
		item := &r.state.Inventory.Items[i]
		if item.Index != index || item.Amount < quantity {
			continue
		}
		if r.contentLoaded {
			if definition, ok := r.items[item.ItemID]; !ok || definition.NoStorage {
				return false
			}
		}
		item.Amount -= quantity
		r.state.Inventory.Weight -= r.itemWeight(item.ItemID) * quantity
		if r.state.Inventory.Weight < 0 {
			r.state.Inventory.Weight = 0
		}
		copy := *item
		copy.Amount = quantity
		copy.Equipped, copy.Equip = false, false
		r.state.Storage.Items = append(r.state.Storage.Items, copy)
		if item.Amount == 0 {
			r.state.Inventory.Items = append(r.state.Inventory.Items[:i], r.state.Inventory.Items[i+1:]...)
		}
		r.state.Storage.Amount = len(r.state.Storage.Items)
		return true
	}
	return false
}

func (r *OfflineRuntime) withdraw(index uint16, quantity int) bool {
	if quantity <= 0 {
		return false
	}
	for i := range r.state.Storage.Items {
		item := &r.state.Storage.Items[i]
		if item.Index != index || item.Amount < quantity {
			continue
		}
		if !r.addInventory(item.ItemID, item.Type, quantity) {
			return false
		}
		item.Amount -= quantity
		if item.Amount == 0 {
			r.state.Storage.Items = append(r.state.Storage.Items[:i], r.state.Storage.Items[i+1:]...)
		}
		r.state.Storage.Amount = len(r.state.Storage.Items)
		return true
	}
	return false
}

func (r *OfflineRuntime) distanceToPlayer(x, y int) int {
	return distance(r.state.PlayerX, r.state.PlayerY, x, y)
}

func distance(ax, ay, bx, by int) int {
	return max(abs(ax-bx), abs(ay-by))
}

func stepToward(x, y, targetX, targetY int) (int, int) {
	if x < targetX {
		x++
	} else if x > targetX {
		x--
	}
	if y < targetY {
		y++
	} else if y > targetY {
		y--
	}
	return x, y
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
