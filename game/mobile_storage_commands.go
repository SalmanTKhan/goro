package game

import "github.com/kivutar/goro/client"

func (m *WorldMode) mobileStorageDeposit(ctx client.Context, index uint16, amount uint32) bool {
	if ctx.Network == nil || ctx.Session == nil || !ctx.Session.Storage.Open || index == 0 || amount == 0 {
		return false
	}
	for _, item := range ctx.Session.Inventory.Items {
		if item.Index != index || item.Amount <= 0 || int64(amount) > int64(item.Amount) {
			continue
		}
		return ctx.Network.SendMoveToStorage(index, amount) == nil
	}
	return false
}

func (m *WorldMode) mobileStorageWithdraw(ctx client.Context, index uint16, amount uint32) bool {
	if ctx.Network == nil || ctx.Session == nil || !ctx.Session.Storage.Open || index == 0 || amount == 0 {
		return false
	}
	for _, item := range ctx.Session.Storage.Items {
		if item.Index != index || item.Amount <= 0 || int64(amount) > int64(item.Amount) {
			continue
		}
		return ctx.Network.SendMoveFromStorage(index, amount) == nil
	}
	return false
}
