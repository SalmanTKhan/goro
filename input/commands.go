package input

// CommandKind identifies player intent independently of the device that
// produced it.
type CommandKind uint8

const (
	CommandMoveTo CommandKind = iota
	CommandSelectActor
	CommandAttackActor
	CommandInteractActor
	CommandPickUpItem
	CommandUseSkill
	CommandUseSkillOnActor
	CommandUseSkillAtPosition
	CommandRotateCamera
	CommandZoomCamera
	CommandOpenInventory
	CommandOpenCharacter
	CommandOpenSkills
	CommandOpenMap
	CommandOpenSettings
	CommandCancelAction
	CommandOpenActorContext
	CommandInspectActor
	CommandSetMobileControls
	CommandResetMobileControls
	CommandSetMobileSettings
	CommandResetMobileSettings
	CommandUseItem
	CommandEquipItem
	CommandUnequipItem
	CommandDropItem
	CommandOpenShop
	CommandBuyItem
	CommandSellItem
	CommandOpenStorage
	CommandDepositItem
	CommandWithdrawItem
	CommandSendGlobalChat
	CommandSendWhisper
	CommandOpenFriends
	CommandOpenParty
	CommandOpenGuild
	CommandOpenTrade
	CommandOpenVending
	CommandOpenCrafting
	CommandOpenRefinement
	CommandOpenCards
	CommandDeleteFriend
	CommandCreateParty
	CommandInviteParty
	CommandLeaveParty
	CommandExpelPartyMember
	CommandRespondFriendRequest
	CommandRespondPartyInvite
	CommandSetPartySettings
	CommandRespondTradeRequest
	CommandTradeAddItem
	CommandTradeAddZeny
	CommandTradeConclude
	CommandTradeCommit
	CommandTradeCancel
	CommandBuyVendingItem
	CommandOpenProfile
	CommandSaveOfflineProfile
	CommandOnlineSelectCharacter
	CommandOnlineCreateCharacter
	CommandOnlineReconnect
	CommandOnlineDisconnect
	CommandCloseShop
	CommandCloseStorage
	CommandNPCNext
	CommandNPCMenuChoice
	CommandNPCClose
	// Controller and physical-keyboard intents are appended so existing
	// command IDs remain stable for callers that persist or inspect them.
	CommandMoveDirection
	CommandTargetPrevious
	CommandTargetNext
	CommandAttackFocused
	CommandInteractFocused
	CommandLootFocused
	CommandUseShortcut
	CommandResetCamera
)

// WorldPosition is a presentation-independent world target. Screen-space
// picking belongs to the input adapter and never crosses the command boundary.
type WorldPosition struct {
	X float64
	Y float64
}

// PlayerCommand carries semantic player intent. Hardware-specific values
// such as MouseButton, KeyCode, TouchID, and screen coordinates are absent.
type PlayerCommand struct {
	Kind             CommandKind
	Position         WorldPosition
	ActorID          uint32
	ItemID           uint32
	ItemIndex        uint16
	NPCID            uint32
	Quantity         int
	EquipmentSlot    uint16
	SkillID          uint16
	Level            int
	Tab              uint8
	Choice           uint8
	Slot             uint16
	Text             string
	TargetName       string
	TargetAccountID  uint32
	TargetCharID     uint32
	RequestID        uint32
	Accepted         bool
	ProfileID        uint32
	ProfileSex       uint8
	ProfileHairStyle int16
	ProfileHairColor uint8
	ProfileStats     [6]uint8
	ProfileNew       bool
	ExpShare         uint32
	RefuseInvites    bool
	DeltaX           float64
	DeltaY           float64
	Direction        Direction8
	MobileControls   MobileControls
	MobileSettings   MobileSettings
}

// CommandSink accepts semantic commands. Returning false allows a consumer to
// indicate that the command was not accepted without introducing a bus.
type CommandSink interface {
	Emit(PlayerCommand) bool
}

// CommandBuffer is a small deterministic sink useful for adapters, tests, and
// a future gameplay consumer.
type CommandBuffer struct {
	commands []PlayerCommand
}

func (b *CommandBuffer) Emit(command PlayerCommand) bool {
	if b == nil {
		return false
	}
	b.commands = append(b.commands, command)
	return true
}

func (b *CommandBuffer) Commands() []PlayerCommand {
	if b == nil {
		return nil
	}
	return append([]PlayerCommand(nil), b.commands...)
}

func (b *CommandBuffer) Reset() {
	if b != nil {
		b.commands = b.commands[:0]
	}
}

type TargetKind uint8

const (
	TargetGround TargetKind = iota
	TargetActor
	TargetNPC
	TargetItem
	TargetVending
)

// PickedTarget is the result of screen-to-world picking. The picker may be
// backed by the current renderer, but the resulting command is not.
type PickedTarget struct {
	Kind     TargetKind
	Position WorldPosition
	ActorID  uint32
	ItemID   uint32
	Hostile  bool
}

type WorldPicker interface {
	Pick(WorldPosition) (PickedTarget, bool)
}

type UIHitTester interface {
	ConsumeTouch(TouchPoint) bool
}

type SkillTargetMode uint8

const (
	SkillTargetIdle SkillTargetMode = iota
	SkillTargetActor
	SkillTargetGround
)

type SkillTargetState struct {
	Mode    SkillTargetMode
	SkillID uint16
	Level   int
}

func (s *SkillTargetState) BeginActor(skillID uint16, level int) {
	*s = SkillTargetState{Mode: SkillTargetActor, SkillID: skillID, Level: level}
}

func (s *SkillTargetState) BeginGround(skillID uint16, level int) {
	*s = SkillTargetState{Mode: SkillTargetGround, SkillID: skillID, Level: level}
}

func (s *SkillTargetState) Cancel() {
	*s = SkillTargetState{}
}

func (s *SkillTargetState) Select(target PickedTarget) (PlayerCommand, bool) {
	if s == nil || s.SkillID == 0 {
		return PlayerCommand{}, false
	}
	command := PlayerCommand{SkillID: s.SkillID, Level: s.Level}
	switch {
	case s.Mode == SkillTargetActor && target.Kind == TargetActor:
		command.Kind = CommandUseSkillOnActor
		command.ActorID = target.ActorID
	case s.Mode == SkillTargetGround && target.Kind == TargetGround:
		command.Kind = CommandUseSkillAtPosition
		command.Position = target.Position
	default:
		return PlayerCommand{}, false
	}
	s.Cancel()
	return command, true
}
