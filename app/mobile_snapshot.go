package app

import "github.com/kivutar/goro/mobileui"

// MobileSnapshot is the consolidated, renderer-neutral source used by the
// Android presentation. It keeps screen projections together while retaining
// the existing typed model boundary; no widget, renderer handle, or screen
// coordinate crosses into app or game authority.
type MobileSnapshot struct {
	HUD       mobileui.MobileHUDModel
	Character mobileui.MobileCharacterModel
	Skills    mobileui.MobileSkillsModel
	Inventory mobileui.MobileInventoryModel
	Equipment mobileui.MobileEquipmentModel
	Map       mobileui.MobileMapModel
	Storage   mobileui.MobileStorageModel
	Shop      mobileui.MobileShopModel
	Chat      mobileui.MobileChatModel
	Social    mobileui.MobileSocialModel
	Trade     mobileui.MobileTradeModel
	Vending   mobileui.MobileVendingModel
	Profile   mobileui.MobileProfileModel
}

type MobileSnapshotSource interface {
	MobileSnapshot(npcID uint32) MobileSnapshot
}

func (g *Game) MobileSnapshot(npcID uint32) MobileSnapshot {
	if g == nil {
		return MobileSnapshot{}
	}
	inventory := g.MobileInventoryModel()
	return MobileSnapshot{
		HUD:       g.MobileHUDModel(),
		Character: g.MobileCharacterModel(),
		Skills:    g.MobileSkillsModel(),
		Inventory: inventory,
		Equipment: mobileui.ProjectEquipment(inventory),
		Map:       g.MobileMapModel(),
		Storage:   g.MobileStorageModel(),
		Shop:      g.MobileShopModel(npcID),
		Chat:      g.MobileChatModel(),
		Social:    g.MobileSocialModel(),
		Trade:     g.MobileTradeModel(),
		Vending:   g.MobileVendingModel(),
		Profile:   g.MobileProfileModel(),
	}
}
