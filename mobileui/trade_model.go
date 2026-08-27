package mobileui

// TradeOfferItemModel is the renderer-neutral view of one item in an active
// exchange. IsZeny is true for the protocol's special index-zero currency
// offer; packet layouts never cross into mobileui.
type TradeOfferItemModel struct {
	ItemIndex  uint16
	ItemID     uint16
	Name       string
	IconKey    string
	Quantity   int
	Identified bool
	Refine     uint8
	Cards      [4]uint16
	IsZeny     bool
}

type TradeInventoryItemModel struct {
	Item    InventoryItemModel
	Offered bool
	Pending bool
	CanAdd  bool
}

type MobileTradeRequestModel struct {
	TargetID uint32
	Level    uint16
	Name     string
}

// MobileTradeModel is a read-only projection of the current online exchange.
// The server remains authoritative for distance, ownership, quantity, zeny,
// conclusion, and commit validation.
type MobileTradeModel struct {
	Open           bool
	OnlineSession  bool
	PartnerName    string
	Inventory      []TradeInventoryItemModel
	OwnOffer       []TradeOfferItemModel
	PartnerOffer   []TradeOfferItemModel
	OwnZeny        uint32
	AvailableZeny  uint32
	PartnerZeny    uint32
	SelfConcluded  bool
	OtherConcluded bool
	PendingRequest *MobileTradeRequestModel
	CanAddItems    bool
	CanAddZeny     bool
	CanConclude    bool
	CanCommit      bool
	CanCancel      bool
	Notice         string
}
