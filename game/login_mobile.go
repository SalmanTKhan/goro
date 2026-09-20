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

// MobileLoginModel projects the complete shared online-login flow for Android.
// Credentials remain inside LoginMode; the mobile model only exposes the
// username and whether a password is already present.
func (m *LoginMode) MobileLoginModel(ctx client.Context) mobileui.MobileOnlineLoginModel {
	model := mobileui.MobileOnlineLoginModel{
		Status:         m.Status(),
		Network:        "offline",
		SelectedSlot:  m.selectedSlot,
		SelectedServer: m.selectedLoginServer,
		Username:       m.username,
		PasswordSet:    m.password != "",
		CanReconnect:  ctx.Network != nil,
		CanDisconnect: ctx.Network != nil,
		CanSwitchMode: true,
	}
	if ctx.Network != nil {
		model.Network = ctx.Network.Status()
	}

	if m.phase == loginPhaseCreate {
		model.Phase = mobileui.OnlineLoginCreate
		model.Notice = "Creating a character"
		return model
	}
	if m.phase == loginPhaseCharacter {
		return m.mobileCharacterLoginModel(ctx, model)
	}

	switch m.accountStep {
	case loginAccountConnection:
		model.Phase = mobileui.OnlineLoginServer
		connections := loginConnections(ctx)
		for i, conn := range connections {
			detail := strings.TrimSpace(conn.Description)
			if detail == "" {
				detail = fmt.Sprintf("%s:%d", conn.Address, conn.Port)
			}
			model.Servers = append(model.Servers, mobileui.OnlineServerOption{
				Index: i, Name: conn.Display, Detail: detail, Selected: i == m.selectedLoginServer,
			})
		}
		model.Notice = "Choose a login server."
		if len(connections) == 0 {
			model.Notice = "No login servers were found in clientinfo.xml."
		}
		return model

	case loginAccountCredentials:
		model.Phase = mobileui.OnlineLoginCredentials
		if conn, ok := m.selectedLoginConnection(ctx); ok {
			model.Server = conn.Display
		}
		model.CanSubmit = strings.TrimSpace(m.username) != "" && m.password != ""
		model.Notice = "Enter your Ragnarok Online account credentials."
		return model

	case loginAccountCharacterService:
		model.Phase = mobileui.OnlineLoginCharacterService
		if ctx.Session != nil {
			model.SelectedServer = ctx.Session.CharServerIndex
			for i, server := range ctx.Session.CharServers {
				name := strings.TrimSpace(server.Name)
				if name == "" {
					name = fmt.Sprintf("Character Server %d", i+1)
				}
				model.Servers = append(model.Servers, mobileui.OnlineServerOption{
					Index: i,
					Name: name,
					Detail: fmt.Sprintf("%s:%d", server.Address, server.Port),
					UserCount: int(server.UserCount),
					Selected: i == ctx.Session.CharServerIndex,
				})
			}
		}
		model.Notice = "Choose a character service."
		return model

	case loginAccountCharacterConnecting:
		model.Phase = mobileui.OnlineLoginConnecting
		model.Notice = "Connecting to the character service…"
		return model

	default:
		model.Phase = mobileui.OnlineLoginConnecting
		model.Notice = "Preparing online login…"
		return model
	}
}

func (m *LoginMode) mobileCharacterLoginModel(ctx client.Context, model mobileui.MobileOnlineLoginModel) mobileui.MobileOnlineLoginModel {
	model.Phase = mobileui.OnlineLoginCharacters
	model.Characters = make([]mobileui.OnlineCharacterSlot, 0, m.maxSlots)
	maxSlots := m.maxSlots
	if maxSlots <= 0 {
		maxSlots = 9
	}
	if ctx.Session == nil {
		model.Notice = "Waiting for character data."
		return model
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
	case input.CommandOnlineSelectLoginServer:
		if m.phase != loginPhaseAccount || m.accountStep != loginAccountConnection {
			return false
		}
		if command.Slot >= uint16(len(loginConnections(ctx))) {
			return false
		}
		m.selectLoginServer(ctx, int(command.Slot))
		return true

	case input.CommandOnlineSubmitCredentials:
		if m.phase != loginPhaseAccount || m.accountStep != loginAccountCredentials {
			return false
		}
		if username := strings.TrimSpace(command.Username); username != "" {
			m.username = username
		}
		if command.Password != "" {
			m.password = command.Password
		}
		if strings.TrimSpace(m.username) == "" || m.password == "" {
			m.status = "username and password required"
			return false
		}
		m.saveLoginID(ctx)
		conn, ok := m.selectedLoginConnection(ctx)
		if !ok || ctx.Network == nil {
			m.status = "login server unavailable"
			return false
		}
		m.connectAndMaybeLogin(ctx, conn, true)
		return true

	case input.CommandOnlineSelectCharacterService:
		if m.phase != loginPhaseAccount || m.accountStep != loginAccountCharacterService || ctx.Session == nil {
			return false
		}
		if int(command.Slot) < 0 || int(command.Slot) >= len(ctx.Session.CharServers) {
			return false
		}
		m.selectCharacterService(ctx, int(command.Slot), true)
		return true

	case input.CommandOnlineSelectCharacter:
		if m.phase != loginPhaseCharacter || ctx.Session == nil {
			return false
		}
		m.selectedSlot = clampCharacterSlot(int(command.Slot), m.maxSlots)
		m.submitSelectedCharacter(ctx)
		return false

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
		m.accountStep = loginAccountConnection
		m.fade = loginFadeState{}
		m.autoAttempted = false
		m.autoCharAttempted = false
		m.connectRequested = false
		m.status = "select a server"
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
