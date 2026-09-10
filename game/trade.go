package game

import (
	"fmt"
	"strings"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/network"
	"github.com/kivutar/goro/session"
)

type mobileTradeRequestState struct {
	TargetID uint32
	Level    uint16
	Name     string
}

type mobileTradeState struct {
	open           bool
	partnerName    string
	ownOffer       []mobileui.TradeOfferItemModel
	partnerOffer   []mobileui.TradeOfferItemModel
	pending        map[uint16]mobileui.TradeOfferItemModel
	ownZeny        uint32
	partnerZeny    uint32
	selfConcluded  bool
	otherConcluded bool
}

func (m *WorldMode) sendTradeRequest(ctx client.Context, actorID uint32, name string) bool {
	if actorID == 0 {
		return false
	}
	if ctx.Network == nil {
		glog.Warnf("trade request failed target=%d name=%q: not connected", actorID, name)
		return false
	}
	if err := ctx.Network.SendTradeRequest(actorID); err != nil {
		glog.Warnf("trade request failed target=%d name=%q: %v", actorID, name, err)
		return false
	}
	m.pendingTradeName = strings.TrimSpace(name)
	return true
}

func (m *WorldMode) openTradeRequest(ctx client.Context, request network.TradeRequest) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "Player"
	}
	m.mobileTradeRequest = &mobileTradeRequestState{TargetID: request.TargetID, Level: request.Level, Name: name}
	mobileMessage := fmt.Sprintf("%s wants to trade with you.", name)
	if request.TargetID != 0 || request.Level != 0 {
		mobileMessage = fmt.Sprintf("%s\nLv.%d", mobileMessage, request.Level)
	}
	m.ui.tradeRequest.Open(ctx, "Trade Request", mobileMessage, func() {
		if m.respondTradeRequest(ctx, true) {
			m.ui.tradeWindow.Open(ctx, name)
		}
	}, func() {
		m.respondTradeRequest(ctx, false)
	})
}

func (m *WorldMode) respondTradeRequest(ctx client.Context, accepted bool) bool {
	request := m.mobileTradeRequest
	if request == nil || ctx.Network == nil {
		return false
	}
	if err := ctx.Network.SendTradeAck(accepted); err != nil {
		glog.Warnf("trade request response failed name=%q accepted=%t: %v", request.Name, accepted, err)
		return false
	}
	m.mobileTradeRequest = nil
	m.ui.tradeRequest.Close(ctx)
	if accepted {
		m.openMobileTrade(request.Name)
	}
	return true
}

func (m *WorldMode) openMobileTrade(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Player"
	}
	m.mobileTrade = mobileTradeState{
		open: true, partnerName: name, pending: make(map[uint16]mobileui.TradeOfferItemModel),
	}
}

func (m *WorldMode) closeMobileTrade() {
	m.mobileTrade = mobileTradeState{}
}

func (m *WorldMode) handleTradeResponse(ctx client.Context, response network.TradeResponse) {
	name := m.pendingTradeName
	m.pendingTradeName = ""
	switch response.Result {
	case 0:
		m.ui.tradeWindow.Close(ctx)
		m.closeMobileTrade()
		m.ui.console.AddErrorMessage("That character is too far away.")
	case 1:
		m.ui.tradeWindow.Close(ctx)
		m.closeMobileTrade()
		m.ui.console.AddErrorMessage("Character does not exist.")
	case 2:
		m.ui.tradeWindow.Close(ctx)
		m.closeMobileTrade()
		m.ui.console.AddErrorMessage("Trade failed.")
	case 3:
		// The server acknowledges both sides of an accepted trade with the
		// same legacy packet. For an incoming request there is no pending
		// outbound name, so preserve the name captured from ZC_REQ_EXCHANGE_ITEM
		// instead of replacing it with the generic fallback.
		if name == "" && m.mobileTrade.open {
			name = m.mobileTrade.partnerName
		}
		if name == "" {
			name = "Player"
		}
		m.ui.tradeWindow.Open(ctx, name)
		m.openMobileTrade(name)
	case 4:
		m.ui.tradeWindow.Close(ctx)
		m.closeMobileTrade()
		m.ui.console.AddErrorMessage("Trade canceled.")
	case 5:
		m.ui.tradeWindow.Close(ctx)
		m.closeMobileTrade()
		m.ui.console.AddErrorMessage("That character is busy.")
	default:
		m.ui.tradeWindow.Close(ctx)
		m.closeMobileTrade()
		m.ui.console.AddErrorMessage("Trade failed.")
	}
}

func (m *WorldMode) handleTradeExec(ctx client.Context, exec network.TradeExec) {
	m.ui.tradeWindow.Close(ctx)
	m.closeMobileTrade()
	if exec.Result == 0 {
		m.ui.console.AddBlueMessage("Trade completed.")
		return
	}
	m.ui.console.AddErrorMessage("Trade failed.")
}

//lint:ignore U1000 retained for mobile session integration
func (m *WorldMode) handleMobileTradeItem(ctx client.Context, item network.TradeItem) {
	if !m.mobileTrade.open {
		return
	}
	if item.ItemID == 0 {
		m.mobileTrade.partnerZeny = item.Amount
		return
	}
	m.mobileTrade.partnerOffer = append(m.mobileTrade.partnerOffer, mobileTradeOfferFromNetwork(ctx, item))
}

//lint:ignore U1000 retained for mobile session integration
func (m *WorldMode) handleMobileTradeAddAck(ack network.TradeAddItemAck) {
	if !m.mobileTrade.open {
		return
	}
	if ack.Result != 0 {
		if m.mobileTrade.pending != nil {
			delete(m.mobileTrade.pending, ack.Index)
		}
		return
	}
	offer, ok := m.mobileTrade.pending[ack.Index]
	if !ok {
		return
	}
	delete(m.mobileTrade.pending, ack.Index)
	if ack.Index == 0 {
		m.mobileTrade.ownZeny = uint32(offer.Quantity)
		return
	}
	m.mobileTrade.ownOffer = append(m.mobileTrade.ownOffer, offer)
}

//lint:ignore U1000 retained for mobile session integration
func (m *WorldMode) handleMobileTradeUndo() {
	if !m.mobileTrade.open {
		return
	}
	m.mobileTrade.selfConcluded = false
	m.mobileTrade.ownOffer = nil
	m.mobileTrade.ownZeny = 0
	m.mobileTrade.pending = make(map[uint16]mobileui.TradeOfferItemModel)
}

// MobileTradeModel projects the active exchange and the pending request. It
// is intentionally owned by WorldMode because the online trade lifecycle is
// transient and already follows the existing packet handlers.
func (m *WorldMode) MobileTradeModel(ctx client.Context) mobileui.MobileTradeModel {
	online := ctx.Network != nil
	model := mobileui.MobileTradeModel{OnlineSession: online}
	if !online {
		model.Notice = "Trade is available in an online session."
	}
	if request := m.mobileTradeRequest; request != nil && online {
		model.PendingRequest = &mobileui.MobileTradeRequestModel{TargetID: request.TargetID, Level: request.Level, Name: request.Name}
	}
	if !m.mobileTrade.open {
		return model
	}
	model.Open = online
	model.PartnerName = mobileTradeDisplayName(m.mobileTrade.partnerName)
	model.OwnOffer = append([]mobileui.TradeOfferItemModel(nil), m.mobileTrade.ownOffer...)
	model.PartnerOffer = append([]mobileui.TradeOfferItemModel(nil), m.mobileTrade.partnerOffer...)
	model.OwnZeny = m.mobileTrade.ownZeny
	model.PartnerZeny = m.mobileTrade.partnerZeny
	model.SelfConcluded = m.mobileTrade.selfConcluded
	model.OtherConcluded = m.mobileTrade.otherConcluded
	model.CanAddItems = online && !m.mobileTrade.selfConcluded
	model.CanAddZeny = model.CanAddItems && m.mobileTrade.ownZeny == 0 && !mobileTradePending(m.mobileTrade.pending, 0)
	model.CanConclude = online && !m.mobileTrade.selfConcluded
	model.CanCommit = online && m.mobileTrade.selfConcluded && m.mobileTrade.otherConcluded
	model.CanCancel = online
	if ctx.Session != nil {
		inventory := mobileui.ProjectInventory(ctx.Session, ctx.Resources)
		offered := make(map[uint16]bool, len(m.mobileTrade.ownOffer))
		for _, offer := range m.mobileTrade.ownOffer {
			offered[offer.ItemIndex] = true
		}
		pending := make(map[uint16]bool, len(m.mobileTrade.pending))
		for index := range m.mobileTrade.pending {
			pending[index] = true
		}
		model.Inventory = make([]mobileui.TradeInventoryItemModel, 0, len(inventory.Items))
		for _, item := range inventory.Items {
			if item.Index == 0 || item.Quantity <= 0 || item.Equipped {
				continue
			}
			isOffered, isPending := offered[item.Index], pending[item.Index]
			model.Inventory = append(model.Inventory, mobileui.TradeInventoryItemModel{
				Item: item, Offered: isOffered, Pending: isPending,
				CanAdd: model.CanAddItems && !isOffered && !isPending,
			})
		}
		model.AvailableZeny = uint32(maxInt64(0, inventory.Zeny-int64(model.OwnZeny)))
		model.CanAddZeny = model.CanAddZeny && model.AvailableZeny > 0
	}
	return model
}

func (m *WorldMode) applyMobileTradeCommand(ctx client.Context, command input.PlayerCommand) bool {
	if ctx.Network == nil {
		return false
	}
	switch command.Kind {
	case input.CommandRespondTradeRequest:
		if m.mobileTradeRequest == nil || (command.RequestID != 0 && command.RequestID != m.mobileTradeRequest.TargetID) {
			return false
		}
		return m.respondTradeRequest(ctx, command.Accepted)
	case input.CommandOpenTrade:
		return m.sendTradeRequest(ctx, command.ActorID, command.TargetName)
	case input.CommandTradeAddItem:
		if !m.mobileTrade.open || m.mobileTrade.selfConcluded || command.ItemIndex == 0 || command.Quantity <= 0 || mobileTradePending(m.mobileTrade.pending, command.ItemIndex) {
			return false
		}
		var item session.InventoryItem
		found := false
		if ctx.Session != nil {
			for _, candidate := range ctx.Session.Inventory.Items {
				if candidate.Index == command.ItemIndex {
					item, found = candidate, true
					break
				}
			}
		}
		if !found || item.Equipped || item.Amount <= 0 || command.Quantity > item.Amount {
			return false
		}
		inventory := mobileui.ProjectInventory(ctx.Session, ctx.Resources)
		var projected mobileui.InventoryItemModel
		for _, candidate := range inventory.Items {
			if candidate.Index == item.Index {
				projected = candidate
				break
			}
		}
		if projected.Index == 0 {
			return false
		}
		if err := ctx.Network.SendTradeAddItem(command.ItemIndex, uint32(command.Quantity)); err != nil {
			return false
		}
		if m.mobileTrade.pending == nil {
			m.mobileTrade.pending = make(map[uint16]mobileui.TradeOfferItemModel)
		}
		m.mobileTrade.pending[command.ItemIndex] = mobileTradeOfferFromInventory(projected, command.Quantity)
		return true
	case input.CommandTradeAddZeny:
		if !m.mobileTrade.open || m.mobileTrade.selfConcluded || command.Quantity <= 0 || m.mobileTrade.ownZeny != 0 || mobileTradePending(m.mobileTrade.pending, 0) || ctx.Session == nil {
			return false
		}
		available := ctx.Session.Inventory.Zeny - int64(m.mobileTrade.ownZeny)
		if available <= 0 || int64(command.Quantity) > available {
			return false
		}
		if err := ctx.Network.SendTradeAddItem(0, uint32(command.Quantity)); err != nil {
			return false
		}
		if m.mobileTrade.pending == nil {
			m.mobileTrade.pending = make(map[uint16]mobileui.TradeOfferItemModel)
		}
		m.mobileTrade.pending[0] = mobileui.TradeOfferItemModel{ItemIndex: 0, Quantity: command.Quantity, IsZeny: true, Name: "Zeny"}
		return true
	case input.CommandTradeConclude:
		if !m.mobileTrade.open || m.mobileTrade.selfConcluded {
			return false
		}
		return ctx.Network.SendTradeConclude() == nil
	case input.CommandTradeCommit:
		if !m.mobileTrade.open || !m.mobileTrade.selfConcluded || !m.mobileTrade.otherConcluded {
			return false
		}
		return ctx.Network.SendTradeCommit() == nil
	case input.CommandTradeCancel:
		if !m.mobileTrade.open {
			return false
		}
		err := ctx.Network.SendTradeCancel()
		m.closeMobileTrade()
		return err == nil
	default:
		return false
	}
}

func mobileTradePending(pending map[uint16]mobileui.TradeOfferItemModel, index uint16) bool {
	if pending == nil {
		return false
	}
	_, ok := pending[index]
	return ok
}

func mobileTradeOfferFromInventory(item mobileui.InventoryItemModel, quantity int) mobileui.TradeOfferItemModel {
	return mobileui.TradeOfferItemModel{ItemIndex: item.Index, ItemID: item.ItemID, Name: item.DisplayName, IconKey: item.IconKey, Quantity: quantity, Identified: item.Identified, Refine: item.Refine, Cards: item.Cards}
}

//lint:ignore U1000 retained for mobile session integration
func mobileTradeOfferFromNetwork(ctx client.Context, item network.TradeItem) mobileui.TradeOfferItemModel {
	name, iconKey := "", ""
	if ctx.Resources != nil {
		name, _ = ctx.Resources.ItemDisplayName(int(item.ItemID), item.Identified)
		iconKey, _ = ctx.Resources.ItemResourceName(int(item.ItemID), item.Identified)
	}
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("Item %d", item.ItemID)
	}
	return mobileui.TradeOfferItemModel{ItemID: item.ItemID, Name: name, IconKey: iconKey, Quantity: int(item.Amount), Identified: item.Identified, Refine: item.Refine, Cards: item.Cards}
}

func mobileTradeDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Player"
	}
	return name
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
