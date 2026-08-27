package game

import (
	"fmt"
	"strings"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/session"
)

// MobileLoginModel projects the shared login mode for the Android host. It
// deliberately contains no credentials or network handles; commands are sent
// back through LoginMode so the desktop and mobile paths share authority.
func (m *LoginMode) MobileLoginModel(ctx client.Context) mobileui.MobileOnlineLoginModel {
	model := mobileui.MobileOnlineLoginModel{
		Phase:         mobileui.OnlineLoginAccount,
		Status:        m.Status(),
		Network:       "offline",
		Server:        ctx.Config.MobileSession.Server.Name,
		SelectedSlot:  m.selectedSlot,
		CanReconnect:  true,
		CanDisconnect: true,
		CanSwitchMode: true,
	}
	if ctx.Network != nil {
		model.Network = ctx.Network.Status()
	}
	if model.Server == "" && ctx.Resources != nil && len(ctx.Resources.ClientInfo.Connections) > 0 {
		model.Server = ctx.Resources.ClientInfo.Connections[0].Display
	}
	if m.phase == loginPhaseCreate {
		model.Phase = mobileui.OnlineLoginCreate
		model.Notice = "Creating a character"
		return model
	}
	if m.phase != loginPhaseCharacter {
		model.Notice = "Waiting for the account and character servers."
		return model
	}

	model.Phase = mobileui.OnlineLoginCharacters
	model.Characters = make([]mobileui.OnlineCharacterSlot, 0, m.maxSlots)
	maxSlots := m.maxSlots
	if maxSlots <= 0 {
		maxSlots = 9
	}
	for slot := 0; slot < maxSlots; slot++ {
		entry := mobileui.OnlineCharacterSlot{Slot: slot}
		if character, ok := characterBySlot(ctx.Session.Characters, slot); ok {
			entry.Occupied = true
			entry.Name = character.Name
			entry.Level = int(character.Level)
			entry.JobName = db.JobDisplayName(int(character.Job))
		}
		model.Characters = append(model.Characters, entry)
	}
	model.CanCreate = hasEmptyCharacterSlot(ctx.Session.Characters, maxSlots)
	if len(ctx.Session.Characters) == 0 {
		model.Notice = "No characters yet. Choose an empty slot to create one."
	} else {
		model.Notice = "Choose a character to enter the world."
	}
	return model
}

func hasEmptyCharacterSlot(characters []session.Character, maxSlots int) bool {
	for slot := 0; slot < maxSlots; slot++ {
		if _, ok := characterBySlot(characters, slot); !ok {
			return true
		}
	}
	return false
}

func (m *LoginMode) ApplyPlayerCommand(ctx client.Context, command input.PlayerCommand) bool {
	switch command.Kind {
	case input.CommandOnlineSelectCharacter:
		if m.phase != loginPhaseCharacter || ctx.Session == nil {
			return false
		}
		m.selectedSlot = clampCharacterSlot(int(command.Slot), m.maxSlots)
		return m.submitSelectedCharacter(ctx)
	case input.CommandOnlineCreateCharacter:
		if m.phase != loginPhaseCharacter || ctx.Session == nil || ctx.Network == nil {
			return false
		}
		m.selectedSlot = clampCharacterSlot(int(command.Slot), m.maxSlots)
		if _, occupied := characterBySlot(ctx.Session.Characters, m.selectedSlot); occupied {
			m.status = "character slot occupied"
			return false
		}
		name := strings.TrimSpace(command.Text)
		if name == "" {
			name = fmt.Sprintf("GoroMobile%d", m.selectedSlot)
		}
		m.create = defaultCharCreateState(m.selectedSlot)
		m.create.name = name
		return m.submitCharacterCreate(ctx)
	case input.CommandOnlineReconnect:
		if ctx.Network == nil {
			return false
		}
		ctx.Network.Close()
		m.phase = loginPhaseAccount
		m.fade = loginFadeState{}
		m.autoAttempted = false
		m.autoCharAttempted = false
		m.connectRequested = true
		m.status = "reconnecting"
		if ctx.Session != nil {
			ctx.Session.AccountID = 0
			ctx.Session.CharID = 0
			ctx.Session.AuthCode = 0
			ctx.Session.Playing = false
			ctx.Session.CharServers = nil
			ctx.Session.Characters = nil
			ctx.Session.Selected = session.Character{}
		}
		return true
	case input.CommandOnlineDisconnect:
		if ctx.Network == nil {
			return false
		}
		// Flush the protocol notification before closing the socket. The
		// connection close remains authoritative if the server is already gone.
		_ = ctx.Network.SendQuitGameAndClose()
		m.autoAttempted = true
		m.autoCharAttempted = true
		m.connectRequested = false
		m.status = "disconnected"
		if ctx.Session != nil {
			ctx.Session.Playing = false
		}
		return true
	default:
		return false
	}
}
