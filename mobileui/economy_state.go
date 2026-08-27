package mobileui

import "github.com/kivutar/goro/input"

// EconomyQuantityAction identifies the authority operation that a quantity
// modal will confirm. The modal remains presentation state; the resulting
// command is still validated by the offline or online gameplay authority.
type EconomyQuantityAction uint8

const (
	EconomyQuantityBuy EconomyQuantityAction = iota + 1
	EconomyQuantitySell
	EconomyQuantityWithdraw
)

type EconomyQuantityState struct {
	Open      bool
	Action    EconomyQuantityAction
	NPCID     uint32
	ItemIndex uint16
	Minimum   int
	Maximum   int
	Value     int
}

func (q EconomyQuantityState) Active() bool { return q.Open }

func (q *EconomyQuantityState) OpenFor(action EconomyQuantityAction, npcID uint32, itemIndex uint16, maximum int) {
	if q == nil {
		return
	}
	if maximum < 1 {
		maximum = 1
	}
	*q = EconomyQuantityState{Open: true, Action: action, NPCID: npcID, ItemIndex: itemIndex, Minimum: 1, Maximum: maximum, Value: 1}
}

func (q *EconomyQuantityState) Increment() {
	if q != nil {
		q.SetValue(q.Value + 1)
	}
}

func (q *EconomyQuantityState) Decrement() {
	if q != nil {
		q.SetValue(q.Value - 1)
	}
}

func (q *EconomyQuantityState) SetValue(value int) {
	if q == nil {
		return
	}
	if value < q.Minimum {
		value = q.Minimum
	}
	if value > q.Maximum {
		value = q.Maximum
	}
	q.Value = value
}

func (q *EconomyQuantityState) Confirm() (input.PlayerCommand, bool) {
	if q == nil || !q.Open || q.Value < 1 {
		return input.PlayerCommand{}, false
	}
	command := input.PlayerCommand{ItemIndex: q.ItemIndex, Quantity: q.Value}
	switch q.Action {
	case EconomyQuantityBuy:
		command.Kind, command.NPCID = input.CommandBuyItem, q.NPCID
	case EconomyQuantitySell:
		command.Kind = input.CommandSellItem
	case EconomyQuantityWithdraw:
		command.Kind = input.CommandWithdrawItem
	default:
		return input.PlayerCommand{}, false
	}
	*q = EconomyQuantityState{}
	return command, true
}

func (q *EconomyQuantityState) Cancel() {
	if q != nil {
		*q = EconomyQuantityState{}
	}
}

func economyQuantityMaximum(item ShopItemModel, action EconomyQuantityAction) int {
	switch action {
	case EconomyQuantityBuy:
		if item.MaxQuantity > 0 {
			return item.MaxQuantity
		}
		if item.Stock > 0 {
			return item.Stock
		}
		return 1
	case EconomyQuantitySell:
		if item.MaxQuantity > 0 {
			return item.MaxQuantity
		}
		return item.Quantity
	default:
		return item.Quantity
	}
}
