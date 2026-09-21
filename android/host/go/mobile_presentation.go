//go:build android && cgo

package main

import (
	"fmt"
	"image"
	"image/color"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/kivutar/goro/app"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/render"
)

type mobileCommandSink struct {
	game         *app.Game
	presentation *mobilePresentation
}

func (s mobileCommandSink) Emit(command input.PlayerCommand) bool {
	game := s.game
	if s.presentation != nil && s.presentation.game != nil {
		game = s.presentation.game
	}
	if s.presentation != nil {
		switch command.Kind {
		case input.CommandSetMobileControls, input.CommandResetMobileControls, input.CommandSetMobileSettings, input.CommandResetMobileSettings:
			return s.presentation.ApplyMobileSettingsCommand(command)
		}
	}
	accepted := game != nil && game.ApplyPlayerCommand(command)
	androidLog(fmt.Sprintf("stage=mobile-command kind=%d accepted=%t actor=%d item=%d skill=%d quantity=%d", command.Kind, accepted, command.ActorID, command.ItemID, command.SkillID, command.Quantity))
	if accepted && command.Kind == input.CommandInteractActor && s.presentation != nil && game.Offline() != nil {
		shopAvailable := game.MobileShopModel(command.ActorID).Open
		title := "NPC"
		message := "The NPC has no additional information."
		if offline := game.Offline(); offline != nil {
			title = offline.TargetName
			message = fmt.Sprintf("%s is available on %s.", offline.TargetName, offline.MapName)
		}
		s.presentation.dialogController.Open(mobileui.ProjectDialog(command.ActorID, title, message, shopAvailable))
	}
	if accepted && command.Kind == input.CommandOpenShop && s.presentation != nil {
		model := game.MobileShopModel(command.NPCID)
		if s.presentation.economyController.Screen == mobileui.EconomyShop {
			// Tab switches already update presentation state before emitting the
			// server deal request. Do not reopen the controller and reset it to Buy.
			s.presentation.economyController.SetShop(model)
		} else {
			s.presentation.economyController.OpenShop(model)
		}
		s.presentation.dialogController.Close()
	}
	if accepted && command.Kind == input.CommandOpenStorage && s.presentation != nil {
		s.presentation.economyController.OpenStorage(game.MobileStorageModel())
		s.presentation.dialogController.Close()
	}
	if accepted && command.Kind == input.CommandOpenVending && s.presentation != nil {
		s.presentation.vendingController.Open(game.MobileVendingModel())
		s.presentation.navigation.Open(mobileui.ScreenVending)
	}
	if accepted && command.Kind == input.CommandDepositItem && s.presentation != nil {
		s.presentation.economyController.SetStorage(game.MobileStorageModel())
	}
	if accepted && s.presentation != nil &&
		(command.Kind == input.CommandShopCartAdd || command.Kind == input.CommandShopCartRemove || command.Kind == input.CommandShopCartConfirm) {
		s.presentation.economyController.SetShop(game.MobileShopModel(command.NPCID))
		if s.presentation.widgets != nil {
			s.presentation.widgets.Invalidate()
		}
	}
	if accepted && s.presentation != nil &&
		(command.Kind == input.CommandAssignSkillHotkey || command.Kind == input.CommandAssignItemHotkey) {
		s.presentation.Refresh()
		if s.presentation.widgets != nil {
			s.presentation.widgets.Invalidate()
		}
	}
	return accepted
}

type mobileWorldPicker struct{ presentation *mobilePresentation }

func (p mobileWorldPicker) Pick(position input.WorldPosition) (input.PickedTarget, bool) {
	if p.presentation == nil || p.presentation.game == nil {
		return input.PickedTarget{}, false
	}
	return p.presentation.game.PickMobileTarget(position)
}

type gameWorldPicker struct{ game *app.Game }

func (p gameWorldPicker) Pick(position input.WorldPosition) (input.PickedTarget, bool) {
	if p.game == nil {
		return input.PickedTarget{}, false
	}
	return p.game.PickMobileTarget(position)
}

type mobilePresentation struct {
	game               *app.Game
	modeChanged        func(online bool) bool
	startup            *mobileui.StartupController
	viewport           mobileui.Viewport
	physicalWidth      int
	physicalHeight     int
	navigation         mobileui.Navigation
	hudController      *mobileui.Controller
	hud                mobileui.HUDLayout
	hudModel           mobileui.MobileHUDModel
	inventory          *mobileui.MobileInventoryController
	characterSkills    *mobileui.CharacterSkillsController
	profileController  *mobileui.MobileProfileController
	dialogController   *mobileui.DialogController
	economyController  *mobileui.MobileEconomyController
	mapController      *mobileui.MobileMapController
	settingsController *mobileui.SurfaceController
	settings           input.MobileSettings
	controls           input.MobileControls
	onlineInputMode    uint32
	onlineUsername     string
	onlinePassword     string
	onlineCharacterName string
	settingsChanged    func(input.MobileSettings) bool
	controlsChanged    func(input.MobileControls) bool
	chatController     *mobileui.ChatController
	socialController   *mobileui.MobileSocialController
	tradeController    *mobileui.MobileTradeController
	vendingController  *mobileui.MobileVendingController
	widgets            *mobileWidgets
	characters         *characterWindows
	widgetSnapshot     app.MobileSnapshot
	widgetSnapshotSet  bool
	minimapImage       *render.Image
	minimapSignature   string
	touch              mobileui.TouchSession
	hudTouchStart      mobileui.Hit
	commandDumpKey     string
	lastPlaying        bool
	playingInitialized bool
	safeLeft           float32
	safeTop            float32
	safeRight          float32
	safeBottom         float32
}

func newMobilePresentation(game *app.Game, width, height int) *mobilePresentation {
	p := &mobilePresentation{game: game, widgets: newMobileWidgets(), characters: newCharacterWindows()}
	if game != nil {
		p.settings = game.MobileSettings()
		p.controls = p.settings.Controls
	}
	p.Resize(width, height)
	p.startup = mobileui.NewStartupController(p.viewport)
	// Online mode owns its login/character flow in game.LoginMode. Do not let
	// the offline profile gate obscure that shared flow on Android.
	if game != nil && game.Offline() == nil {
		p.startup.EnterWorld()
	}
	p.hudController = mobileui.NewController(p.hudModel, p.viewport, mobileCommandSink{game: game, presentation: p})
	p.inventory = mobileui.NewInventoryController(game.MobileInventoryModel(), p.viewport, mobileCommandSink{game: game, presentation: p})
	p.characterSkills = mobileui.NewCharacterSkillsController(game.MobileCharacterModel(), game.MobileSkillsModel(), p.viewport)
	p.characterSkills.Sink = mobileCommandSink{game: game, presentation: p}
	p.profileController = mobileui.NewProfileController(game.MobileProfileModel(), p.viewport, mobileCommandSink{game: game, presentation: p})
	p.dialogController = mobileui.NewDialogController(mobileui.MobileDialogModel{}, p.viewport, mobileCommandSink{game: game, presentation: p})
	p.economyController = mobileui.NewEconomyController(p.viewport, mobileCommandSink{game: game, presentation: p})
	p.mapController = mobileui.NewMapController(game.MobileMapModel(), p.viewport, mobileCommandSink{game: game, presentation: p})
	p.settingsController = mobileui.NewSurfaceController(mobileui.SettingsSurface(), p.viewport, mobileCommandSink{game: game, presentation: p})
	p.settingsController.OnItemTap = p.handleSettingsItem
	p.settingsController.SetModel(p.settingsSurfaceModel())
	p.chatController = mobileui.NewChatController(p.viewport, mobileCommandSink{game: game, presentation: p})
	p.socialController = mobileui.NewSocialController(game.MobileSocialModel(), p.viewport, mobileCommandSink{game: game, presentation: p})
	p.tradeController = mobileui.NewTradeController(game.MobileTradeModel(), p.viewport, mobileCommandSink{game: game, presentation: p})
	p.vendingController = mobileui.NewVendingController(game.MobileVendingModel(), p.viewport, mobileCommandSink{game: game, presentation: p})
	p.inventory.Open(mobileui.ScreenWorldHUD)
	return p
}

func (p *mobilePresentation) SetModeChanged(handler func(online bool) bool) {
	if p != nil {
		p.modeChanged = handler
	}
}

// SetGame swaps the authority behind the shared mobile surface. The old game
// is never reused after a mode change, so online and offline state cannot
// silently share session authority.
func (p *mobilePresentation) SetGame(game *app.Game) {
	if p == nil {
		return
	}
	p.game = game
	p.playingInitialized = false
	p.startup = mobileui.NewStartupController(p.viewport)
	if game != nil && game.Online() {
		p.startup.EnterWorld()
	}
	p.navigation = mobileui.Navigation{}
	p.navigation.Open(mobileui.ScreenWorldHUD)
	if p.dialogController != nil {
		p.dialogController.Close()
	}
	if p.economyController != nil {
		p.economyController.Close()
	}
	if p.tradeController != nil {
		p.tradeController.SetModel(mobileui.MobileTradeModel{})
	}
	if p.vendingController != nil {
		p.vendingController.Close()
	}
	if p.profileController != nil {
		p.profileController.Close()
	}
	if p.game != nil {
		p.settings = p.game.MobileSettings()
		p.controls = p.settings.Controls
	}
	p.Refresh()
	if p.widgets != nil {
		p.widgets.Invalidate()
	}
}

// SetChatDraft is fed by the Android IME bridge. The text field remains a
// platform concern; the mobile controller still owns the draft and sends it
// through the normal semantic chat commands.
func (p *mobilePresentation) SetChatDraft(text string) {
	if p != nil && p.chatController != nil && p.chatController.Model.Open {
		p.chatController.SetDraft(text)
	}
}

const (
	androidTextInputNone uint32 = iota
	androidTextInputChat
	androidTextInputSocial
	// androidTextInputCharacter feeds the character create window's name field.
	// That window is a real desktop text field rather than the mobile on-screen
	// keyboard, so the platform editor types into it.
	androidTextInputCharacter
	androidTextInputLoginUsername
	androidTextInputLoginPassword
	androidTextInputLoginCharacterName
)

// SetTextInput routes the single host-native editor to the currently visible
// mobile surface. The editor remains platform-owned; Go retains draft and
// command ownership for both chat and social prompts.
func (p *mobilePresentation) SetTextInput(mode uint32, text string) {
	if p == nil {
		return
	}
	switch mode {
	case androidTextInputCharacter:
		if p.profileController != nil && p.profileController.Editor {
			p.profileController.SetDraftName(text)
			p.characters.Invalidate()
		}
	case androidTextInputSocial:
		if p.socialController != nil && p.socialController.TextInputActive() {
			p.socialController.SetTextInputDraft(text)
		}
	case androidTextInputLoginUsername:
		p.onlineUsername = text
	case androidTextInputLoginPassword:
		p.onlinePassword = text
	case androidTextInputLoginCharacterName:
		p.onlineCharacterName = text
	default:
		p.SetChatDraft(text)
	}
}

func (p *mobilePresentation) syncTextInputState() {
	active := androidTextInputNone
	if p != nil && p.characters.active(p) && p.profileController.Editor && p.characters.TextInputActive() {
		// The character screen is an offline surface, so unlike chat and social
		// this does not depend on being connected.
		active = androidTextInputCharacter
	} else if p != nil && p.game != nil && p.game.Online() && !p.game.SessionPlaying() &&
		(p.onlineInputMode == androidTextInputLoginUsername || p.onlineInputMode == androidTextInputLoginPassword || p.onlineInputMode == androidTextInputLoginCharacterName) {
		phase := p.game.MobileLoginModel().Phase
		if (phase == mobileui.OnlineLoginCredentials && (p.onlineInputMode == androidTextInputLoginUsername || p.onlineInputMode == androidTextInputLoginPassword)) ||
			(phase == mobileui.OnlineLoginCreate && p.onlineInputMode == androidTextInputLoginCharacterName) {
			active = p.onlineInputMode
		} else {
			p.onlineInputMode = androidTextInputNone
		}
	} else if p != nil && p.game != nil && p.game.Online() && p.chatController != nil && p.chatController.Model.Open && p.chatController.Model.CanSend {
		active = androidTextInputChat
	} else if p != nil && p.game != nil && p.game.Online() && p.socialController != nil && p.socialController.TextInputActive() {
		active = androidTextInputSocial
	}
	atomic.StoreUint32(&androidTextInputActive, active)
}

func (p *mobilePresentation) SetControls(controls input.MobileControls) {
	if p == nil {
		return
	}
	settings := p.settings
	settings.Controls = controls
	p.SetSettings(settings)
}

func (p *mobilePresentation) SetControlsChanged(handler func(input.MobileControls) bool) {
	if p != nil {
		p.controlsChanged = handler
	}
}

func (p *mobilePresentation) SetSettings(settings input.MobileSettings) {
	if p == nil {
		return
	}
	p.settings = settings.Normalized()
	p.controls = p.settings.Controls
	if p.widgets != nil {
		p.widgets.Invalidate()
	}
	if p.physicalWidth > 0 && p.physicalHeight > 0 {
		p.Resize(p.physicalWidth, p.physicalHeight)
	}
	if p.settingsController != nil {
		p.settingsController.SetModel(p.settingsSurfaceModel())
	}
}

func (p *mobilePresentation) settingsSurfaceModel() mobileui.SurfaceModel {
	if p == nil {
		return mobileui.SettingsSurface()
	}
	online := p.game != nil && p.game.Online()
	server, status := "", ""
	if p.game != nil {
		status = p.game.NetworkStatus()
		server = p.game.MobileServerName()
	}
	return mobileui.SettingsSurfaceForSession(p.settings, online, server, status)
}

func (p *mobilePresentation) SetSettingsChanged(handler func(input.MobileSettings) bool) {
	if p != nil {
		p.settingsChanged = handler
	}
}

func (p *mobilePresentation) handleSettingsItem(item mobileui.SurfaceItem) bool {
	if p == nil {
		return false
	}
	settings := p.settings
	switch item.ID {
	case "disconnect":
		if p.game == nil || !p.game.Online() {
			return false
		}
		return (mobileCommandSink{game: p.game, presentation: p}).Emit(input.PlayerCommand{Kind: input.CommandOnlineDisconnect})
	case "movement":
		if settings.Controls.MovementMode == input.MovementHoldToMove {
			settings.Controls.MovementMode = input.MovementTapToMove
		} else {
			settings.Controls.MovementMode = input.MovementHoldToMove
		}
	case "camera-sensitivity":
		settings.Controls.CameraSensitivity = nextMobileSensitivity(settings.Controls.CameraSensitivity)
	case "zoom-sensitivity":
		settings.Controls.ZoomSensitivity = nextMobileSensitivity(settings.Controls.ZoomSensitivity)
	case "invert-camera-y":
		settings.Controls.InvertCameraY = !settings.Controls.InvertCameraY
	case "long-press":
		settings.Controls.LongPressMS = nextLongPress(settings.Controls.LongPressMS)
	case "show-target-names":
		settings.Controls.ShowTargetNames = !settings.Controls.ShowTargetNames
	case "bgm-enabled":
		settings.Audio.BGMEnabled = !settings.Audio.BGMEnabled
	case "bgm-volume":
		settings.Audio.BGMVolume = nextMobileVolume(settings.Audio.BGMVolume)
	case "sfx-volume":
		settings.Audio.SFXVolume = nextMobileVolume(settings.Audio.SFXVolume)
	case "show-minimap":
		settings.Display.ShowMinimap = !settings.Display.ShowMinimap
	case "vsync":
		settings.Display.VSync = !settings.Display.VSync
	case "fps-meter":
		settings.Display.FPS = !settings.Display.FPS
	case "ui-scale":
		settings.UI = settings.UI.NextPreset()
	case "presentation":
		if settings.Display.Presentation == input.MobilePresentationDesktop {
			settings.Display.Presentation = input.MobilePresentationMobileUI
		} else {
			settings.Display.Presentation = input.MobilePresentationDesktop
		}
	case "no-shift":
		settings.Gameplay.NoShift = !settings.Gameplay.NoShift
	case "no-ctrl":
		settings.Gameplay.NoCtrl = !settings.Gameplay.NoCtrl
	case "less-effects":
		settings.Gameplay.LessEffects = !settings.Gameplay.LessEffects
	case "snap-targets":
		settings.Gameplay.SnapTargets = !settings.Gameplay.SnapTargets
	case "snap-items":
		settings.Gameplay.SnapItems = !settings.Gameplay.SnapItems
	case "reset-controls":
		settings.Controls = input.DefaultMobileControls()
	case "reset-all":
		return (mobileCommandSink{game: p.game, presentation: p}).Emit(input.PlayerCommand{Kind: input.CommandResetMobileSettings})
	default:
		return false
	}
	command := input.PlayerCommand{Kind: input.CommandSetMobileSettings, MobileSettings: settings}
	return (mobileCommandSink{game: p.game, presentation: p}).Emit(command)
}

func (p *mobilePresentation) ApplyMobileSettingsCommand(command input.PlayerCommand) bool {
	if p == nil {
		return false
	}
	settings := p.settings
	switch command.Kind {
	case input.CommandResetMobileSettings:
		settings = input.DefaultMobileSettings()
	case input.CommandResetMobileControls:
		settings.Controls = input.DefaultMobileControls()
	case input.CommandSetMobileControls:
		settings.Controls = command.MobileControls
	case input.CommandSetMobileSettings:
		settings = command.MobileSettings
	default:
		return false
	}
	settings = settings.Normalized()
	if p.game != nil && !p.game.ApplyMobileSettings(settings) {
		return false
	}
	p.SetSettings(settings)
	if p.settingsChanged != nil {
		return p.settingsChanged(p.settings)
	}
	if p.controlsChanged != nil {
		return p.controlsChanged(p.controls)
	}
	return true
}

func nextMobileSensitivity(value float64) float64 {
	values := []float64{0.5, 0.75, 1, 1.25, 1.5, 2, 2.5, 3}
	for _, candidate := range values {
		if value < candidate-0.001 {
			return candidate
		}
	}
	return values[0]
}

func nextLongPress(value int) int {
	values := []int{300, 400, 550, 700, 900, 1200, 1500}
	for _, candidate := range values {
		if value < candidate {
			return candidate
		}
	}
	return values[0]
}

func nextMobileVolume(value float64) float64 {
	values := []float64{0, 0.25, 0.5, 0.75, 1}
	for _, candidate := range values {
		if value < candidate-0.001 {
			return candidate
		}
	}
	return values[0]
}

func (p *mobilePresentation) layoutHUD() mobileui.HUDLayout {
	if p == nil {
		return mobileui.HUDLayout{}
	}
	layout := mobileui.LayoutHUD(p.viewport, mobileui.DefaultTokens(), p.hudModel, p.navigation)
	if p.game == nil || !p.game.Online() || len(layout.MenuActions) == 0 {
		return layout
	}
	// Profile is an offline-authority feature. Removing it from the online
	// drawer also removes its hit target, so an online player cannot enter the
	// offline character select/create workflow and become stranded there.
	actions := make([]mobileui.MenuAction, 0, len(layout.MenuActions))
	nextY := layout.MenuPanel.Y
	for _, action := range layout.MenuActions {
		if action.Screen == mobileui.ScreenProfile {
			continue
		}
		action.Rect.Y = nextY
		nextY += action.Rect.H
		actions = append(actions, action)
	}
	layout.MenuActions = actions
	if len(actions) == 0 {
		layout.MenuPanel = mobileui.Rect{}
	} else {
		layout.MenuPanel.H = actions[len(actions)-1].Rect.Bottom() - layout.MenuPanel.Y
	}
	return layout
}

func (p *mobilePresentation) Resize(width, height int) {
	if p == nil {
		return
	}
	p.physicalWidth, p.physicalHeight = width, height
	scale := float32(1)
	if p.settings.UI.Scale > 0 {
		scale = p.settings.UI.Normalized().Scale
	}
	p.viewport = mobileui.Viewport{Width: float32(width) / scale, Height: float32(height) / scale, SafeLeft: p.safeLeft / scale, SafeTop: p.safeTop / scale, SafeRight: p.safeRight / scale, SafeBottom: p.safeBottom / scale}
	if p.startup != nil {
		p.startup.Resize(p.viewport)
	}
	snapshot := p.game.MobileSnapshot(0)
	p.hudModel = snapshot.HUD
	p.refreshMinimapImage()
	p.hud = p.layoutHUD()
	if p.hudController != nil {
		p.hudController.Model = p.hudModel
		p.hudController.Navigation = p.navigation
		p.hudController.Layout = p.hud
	}
	if p.inventory != nil {
		p.inventory.Viewport = p.viewport
		p.inventory.SetModel(snapshot.Inventory)
	}
	if p.characterSkills != nil {
		p.characterSkills.Resize(p.viewport)
		p.characterSkills.SetModels(snapshot.Character, snapshot.Skills)
	}
	if p.profileController != nil {
		p.profileController.Resize(p.viewport)
		p.profileController.SetModel(snapshot.Profile)
	}
	if p.dialogController != nil {
		p.dialogController.Resize(p.viewport)
		dialog := p.game.MobileDialogModel()
		if dialog.Open {
			p.dialogController.SetModel(dialog)
		} else if p.game.Online() {
			p.dialogController.Close()
		}
	}
	if p.economyController != nil {
		p.economyController.Resize(p.viewport)
	}
	if p.mapController != nil {
		p.mapController.Resize(p.viewport)
		p.mapController.SetModel(snapshot.Map)
	}
	if p.settingsController != nil {
		p.settingsController.Resize(p.viewport)
		p.settingsController.SetModel(p.settingsSurfaceModel())
	}
	if p.chatController != nil {
		p.chatController.SetModel(snapshot.Chat)
		p.chatController.Resize(p.viewport)
	}
	if p.socialController != nil {
		p.socialController.SetModel(snapshot.Social)
		p.socialController.Resize(p.viewport)
	}
	if p.tradeController != nil {
		p.tradeController.SetModel(snapshot.Trade)
		p.tradeController.Resize(p.viewport)
	}
	if p.vendingController != nil {
		p.vendingController.SetModel(snapshot.Vending)
		p.vendingController.Resize(p.viewport)
		if !snapshot.Vending.Open && p.navigation.Screen == mobileui.ScreenVending {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
	}
	// Orientation/surface changes are a hard retained-layout boundary. Drop
	// touch capture and the old baked widget image before the next frame.
	p.touch.Cancel()
	if p.widgets != nil {
		p.widgets.InvalidateSize()
	}
}

func (p *mobilePresentation) SetSafeInsets(left, top, right, bottom int) {
	if p == nil {
		return
	}
	p.safeLeft, p.safeTop, p.safeRight, p.safeBottom = float32(left), float32(top), float32(right), float32(bottom)
	p.Resize(p.physicalWidth, p.physicalHeight)
}

func (p *mobilePresentation) logicalPoint(x, y int) (int, int) {
	if p == nil {
		return x, y
	}
	scale := p.settings.UI.Normalized().Scale
	if scale <= 0 {
		scale = 1
	}
	return int(float32(x) / scale), int(float32(y) / scale)
}

// Back applies Android's system-back gesture/button to the same priority
// order as the in-game back affordance. In particular, profile name entry
// consumes the first back action to dismiss its keyboard without closing the
// editor or stranding the save/cancel actions behind a modal.
func (p *mobilePresentation) Back() bool {
	if p == nil {
		return false
	}
	if p.onlineInputMode == androidTextInputLoginUsername || p.onlineInputMode == androidTextInputLoginPassword || p.onlineInputMode == androidTextInputLoginCharacterName {
		p.onlineInputMode = androidTextInputNone
		p.syncTextInputState()
		return true
	}
	if p.game != nil && p.game.Online() && !p.game.SessionPlaying() && p.game.MobileLoginModel().Phase == mobileui.OnlineLoginCreate {
		p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineCancelCharacterCreate})
		p.onlineCharacterName = ""
		return true
	}
	if p.tradeController != nil && p.tradeController.IsOpen() {
		if p.tradeController.Back() {
			if !p.tradeController.IsOpen() {
				p.navigation.Open(mobileui.ScreenWorldHUD)
			}
			return true
		}
	}
	if p.vendingController != nil && p.vendingController.IsOpen() {
		if p.vendingController.Back() {
			if !p.vendingController.IsOpen() {
				p.navigation.Open(mobileui.ScreenWorldHUD)
			}
			return true
		}
	}
	if p.profileController != nil && p.profileController.Open {
		if p.profileController.Back() {
			if !p.profileController.Open {
				// The offline profile surface is the startup gate. Match the
				// existing touch BACK behavior and enter the loaded world.
				if p.startup != nil {
					p.startup.EnterWorld()
				}
				p.navigation.Open(mobileui.ScreenWorldHUD)
			}
			return true
		}
	}
	if p.chatController != nil && p.chatController.Model.Open {
		p.chatController.Close()
		p.navigation.Back()
		return true
	}
	if p.dialogController != nil && p.dialogController.Model.Open {
		return p.dialogController.Back()
	}
	if p.economyController != nil && p.economyController.Screen != mobileui.EconomyClosed {
		if p.economyController.Back() {
			if p.economyController.Screen == mobileui.EconomyClosed {
				p.navigation.Open(mobileui.ScreenWorldHUD)
			}
			return true
		}
	}
	switch p.navigation.Screen {
	case mobileui.ScreenCharacter, mobileui.ScreenSkills:
		if p.characterSkills != nil && p.characterSkills.Back() {
			p.navigation.Open(mobileui.ScreenWorldHUD)
			return true
		}
	case mobileui.ScreenSettings:
		if p.settingsController != nil && p.settingsController.Back() {
			p.navigation.Open(mobileui.ScreenWorldHUD)
			return true
		}
	case mobileui.ScreenMap:
		if p.mapController != nil && p.mapController.Back() {
			p.navigation.Open(mobileui.ScreenWorldHUD)
			return true
		}
	case mobileui.ScreenSocial:
		if p.socialController != nil && p.socialController.Back() {
			p.navigation.Open(mobileui.ScreenWorldHUD)
			return true
		}
	case mobileui.ScreenInventory, mobileui.ScreenEquipment:
		if p.inventory != nil && p.inventory.Back() {
			if p.inventory.State.Screen == mobileui.ScreenWorldHUD {
				p.navigation.Open(mobileui.ScreenWorldHUD)
			} else {
				p.navigation.Screen = p.inventory.State.Screen
			}
			return true
		}
	}
	return p.navigation.Back()
}

// visibleWidgetSnapshotChanged limits retained mobile UI invalidation to the
// model actually visible on screen. World/network state can update dozens of
// unrelated snapshot projections every tick; treating the aggregate snapshot
// as one dirty bit caused full-screen UI rasterization and texture uploads even
// while a static Settings/Inventory/etc. surface was open.
func (p *mobilePresentation) visibleWidgetSnapshotChanged(previous, next app.MobileSnapshot) bool {
	if p == nil {
		return false
	}
	if p.tradeController != nil && p.tradeController.IsOpen() {
		return !reflect.DeepEqual(previous.Trade, next.Trade)
	}
	if p.vendingController != nil && p.vendingController.IsOpen() {
		return !reflect.DeepEqual(previous.Vending, next.Vending)
	}
	if p.chatController != nil && p.chatController.Model.Open {
		return !reflect.DeepEqual(previous.Chat, next.Chat)
	}
	if p.economyController != nil {
		switch p.economyController.Screen {
		case mobileui.EconomyShop:
			return !reflect.DeepEqual(previous.Shop, next.Shop)
		case mobileui.EconomyStorage:
			return !reflect.DeepEqual(previous.Storage, next.Storage)
		}
	}
	switch p.navigation.Screen {
	case mobileui.ScreenCharacter:
		return !reflect.DeepEqual(previous.Character, next.Character)
	case mobileui.ScreenSkills:
		return !reflect.DeepEqual(previous.Skills, next.Skills)
	case mobileui.ScreenMap:
		return !reflect.DeepEqual(previous.Map, next.Map)
	case mobileui.ScreenSocial:
		return !reflect.DeepEqual(previous.Social, next.Social)
	case mobileui.ScreenInventory:
		return !reflect.DeepEqual(previous.Inventory, next.Inventory)
	case mobileui.ScreenEquipment:
		return !reflect.DeepEqual(previous.Equipment, next.Equipment)
	case mobileui.ScreenSettings:
		return false
	default:
		return !reflect.DeepEqual(previous.HUD, next.HUD)
	}
}

func (p *mobilePresentation) Refresh() {
	if p == nil || p.game == nil {
		atomic.StoreUint32(&androidTextInputActive, 0)
		return
	}
	snapshot := p.game.MobileSnapshot(0)
	if p.widgetSnapshotSet && p.visibleWidgetSnapshotChanged(p.widgetSnapshot, snapshot) && p.widgets != nil {
		p.widgets.Invalidate()
	}
	p.widgetSnapshot = snapshot
	p.widgetSnapshotSet = true
	playing := p.game.SessionPlaying()
	if p.playingInitialized && playing && !p.lastPlaying {
		// A reconnect may complete while a full-screen surface (for example
		// Settings) is still open. Once the shared session becomes playable,
		// return to the world HUD so the player sees the resumed world.
		p.navigation.Open(mobileui.ScreenWorldHUD)
		if p.dialogController != nil {
			p.dialogController.Close()
		}
		if p.economyController != nil {
			p.economyController.Close()
		}
		if p.chatController != nil {
			p.chatController.Close()
		}
		if p.socialController != nil {
			p.socialController.Close()
		}
		if p.inventory != nil {
			p.inventory.Open(mobileui.ScreenWorldHUD)
		}
	}
	p.lastPlaying = playing
	p.playingInitialized = true
	p.hudModel = snapshot.HUD
	p.hudModel.Minimap.Visible = p.settings.Display.ShowMinimap
	if offline := p.game.Offline(); offline != nil {
		p.hudModel.Minimap.MapName = offline.MapName
	}
	p.refreshMinimapImage()
	p.hud = p.layoutHUD()
	if p.hudController != nil {
		p.hudController.Model = p.hudModel
		p.hudController.Navigation = p.navigation
		p.hudController.Layout = p.hud
	}
	if p.inventory != nil {
		p.inventory.SetModel(snapshot.Inventory)
	}
	if p.characterSkills != nil {
		p.characterSkills.SetModels(snapshot.Character, snapshot.Skills)
	}
	if p.profileController != nil {
		p.profileController.SetModel(snapshot.Profile)
		p.profileController.Resize(p.viewport)
		if p.game.Online() && p.profileController.Open {
			// Profile is the offline character-authority editor. Never carry an
			// accidentally opened profile surface into an online session.
			p.profileController.Close()
			if p.navigation.Screen == mobileui.ScreenProfile {
				p.navigation.Open(mobileui.ScreenWorldHUD)
			}
		}
	}
	if p.dialogController != nil {
		p.dialogController.Resize(p.viewport)
		// NPC dialog state is not part of MobileSnapshot: it is driven directly
		// by zone packets through the shared desktop NPCDialog. Invalidate the
		// retained widget raster whenever that authoritative projection changes;
		// otherwise the hit-test/model can advance to Next/Menu while Android
		// keeps displaying the previous "Waiting for the server" raster.
		previousDialog := p.dialogController.Model
		dialog := p.game.MobileDialogModel()
		if dialog.Open {
			p.dialogController.SetModel(dialog)
		} else if p.game.Online() {
			p.dialogController.Close()
		}
		if !reflect.DeepEqual(previousDialog, p.dialogController.Model) {
			androidLog(fmt.Sprintf("stage=mobile-dialog open=%t npc=%d options=%d notice=%q",
				p.dialogController.Model.Open, p.dialogController.Model.NPCID, len(p.dialogController.Model.Options), p.dialogController.Model.Notice))
			if p.widgets != nil {
				p.widgets.Invalidate()
			}
		}
	}
	if p.economyController != nil {
		shop := p.game.MobileShopModel(0)
		if shop.Open && p.economyController.Screen == mobileui.EconomyClosed {
			p.economyController.OpenShop(shop)
			if shop.NPCID != 0 && len(shop.Items) == 0 && len(shop.SellItems) == 0 {
				p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOpenShop, NPCID: shop.NPCID, Tab: uint8(mobileui.ShopBuyTab)})
			}
		}
		switch p.economyController.Screen {
		case mobileui.EconomyShop:
			p.economyController.SetShop(p.game.MobileShopModel(p.economyController.Shop.NPCID))
		case mobileui.EconomyStorage:
			p.economyController.SetStorage(p.game.MobileStorageModel())
		}
		storage := p.game.MobileStorageModel()
		if storage.Open && p.economyController.Screen == mobileui.EconomyClosed {
			p.economyController.OpenStorage(storage)
		}
	}
	if p.mapController != nil {
		p.mapController.SetModel(snapshot.Map)
	}
	if p.settingsController != nil {
		p.settingsController.Resize(p.viewport)
		p.settingsController.SetModel(p.settingsSurfaceModel())
	}
	if p.chatController != nil {
		p.chatController.SetModel(snapshot.Chat)
		p.chatController.Resize(p.viewport)
	}
	if p.socialController != nil {
		p.socialController.SetModel(snapshot.Social)
		p.socialController.Resize(p.viewport)
	}
	if p.tradeController != nil {
		p.tradeController.SetModel(snapshot.Trade)
		p.tradeController.Resize(p.viewport)
		if snapshot.Trade.Open || snapshot.Trade.PendingRequest != nil {
			if p.inventory != nil {
				p.inventory.Open(mobileui.ScreenWorldHUD)
			}
			if p.economyController != nil {
				p.economyController.Close()
			}
			if p.dialogController != nil {
				p.dialogController.Close()
			}
			if p.chatController != nil {
				p.chatController.Close()
			}
			if p.socialController != nil {
				p.socialController.Close()
			}
			p.navigation.Open(mobileui.ScreenTrade)
		} else if p.navigation.Screen == mobileui.ScreenTrade {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
	}
	if p.vendingController != nil {
		p.vendingController.SetModel(snapshot.Vending)
		p.vendingController.Resize(p.viewport)
		if !snapshot.Vending.Open && p.navigation.Screen == mobileui.ScreenVending {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
	}
	p.syncTextInputState()
}

func (p *mobilePresentation) emitMobileCommand(command input.PlayerCommand) {
	if p == nil || p.game == nil {
		return
	}
	accepted := p.game.ApplyPlayerCommand(command)
	androidLog(fmt.Sprintf("stage=mobile-command kind=%d accepted=%t npc=%d tab=%d", command.Kind, accepted, command.NPCID, command.Tab))
}

func (p *mobilePresentation) ConsumeTouch(point input.TouchPoint) bool {
	if p == nil {
		return false
	}
	if p.startup != nil && p.startup.Active() {
		return true
	}
	if p.game != nil && p.game.Online() && !p.game.SessionPlaying() {
		return true
	}
	point.X, point.Y = p.logicalPoint(point.X, point.Y)
	if p.tradeController != nil && p.tradeController.IsOpen() {
		return p.tradeController.ConsumeTouch(point)
	}
	if p.vendingController != nil && p.vendingController.IsOpen() {
		return p.vendingController.ConsumeTouch(point)
	}
	if p.profileController != nil && p.profileController.Open {
		return p.profileController.ConsumeTouch(point)
	}
	if p.chatController != nil && p.chatController.Model.Open {
		return p.chatController.ConsumeTouch(point)
	}
	if p.dialogController != nil && p.dialogController.ConsumeTouch(point) {
		return true
	}
	if p.economyController != nil && p.economyController.ConsumeTouch(point) {
		return true
	}
	if p.characterSkills != nil && (p.navigation.Screen == mobileui.ScreenCharacter || p.navigation.Screen == mobileui.ScreenSkills) {
		return p.characterSkills.ConsumeTouch(point)
	}
	if p.settingsController != nil && p.navigation.Screen == mobileui.ScreenSettings {
		return p.settingsController.ConsumeTouch(point)
	}
	if p.mapController != nil && p.navigation.Screen == mobileui.ScreenMap {
		return p.mapController.ConsumeTouch(point)
	}
	if p.socialController != nil && p.navigation.Screen == mobileui.ScreenSocial && p.socialController.IsOpen() {
		return p.socialController.ConsumeTouch(point)
	}
	if p.navigation.Targeting.Mode != input.SkillTargetIdle {
		// Target selection is a UI-owned modal interaction even though the
		// selected point is resolved through the shared world picker.
		return true
	}
	if p.inventory != nil && p.inventory.State.Screen != mobileui.ScreenWorldHUD {
		return p.inventory.ConsumeTouch(point)
	}
	return p.hud.ConsumeTouch(point)
}

func (p *mobilePresentation) BeginTouch(id input.TouchID, x, y int) bool {
	if p == nil {
		return false
	}
	lx, ly := p.logicalPoint(x, y)
	point := input.TouchPoint{ID: id, X: lx, Y: ly}
	owner := mobileui.TouchUnclaimed
	switch {
	case p.startup != nil && p.startup.Phase == mobileui.StartupTitle:
		owner = mobileui.TouchStartup
	case p.game != nil && p.game.Online() && !p.game.SessionPlaying():
		owner = mobileui.TouchOnline
	case p.tradeController != nil && p.tradeController.IsOpen():
		owner = mobileui.TouchDialog
	case p.vendingController != nil && p.vendingController.IsOpen():
		owner = mobileui.TouchDialog
	case p.profileController != nil && p.profileController.Open:
		owner = mobileui.TouchProfile
	case p.chatController != nil && p.chatController.Model.Open:
		owner = mobileui.TouchDialog
	case p.dialogController != nil && p.dialogController.Model.Open:
		owner = mobileui.TouchDialog
	case p.economyController != nil && p.economyController.Screen != mobileui.EconomyClosed:
		owner = mobileui.TouchEconomy
	case p.navigation.Screen == mobileui.ScreenCharacter:
		owner = mobileui.TouchCharacter
	case p.navigation.Screen == mobileui.ScreenSkills:
		owner = mobileui.TouchSkills
	case p.navigation.Screen == mobileui.ScreenMap:
		owner = mobileui.TouchMap
	case p.navigation.Screen == mobileui.ScreenSocial:
		owner = mobileui.TouchSocial
	case p.navigation.Screen == mobileui.ScreenSettings:
		owner = mobileui.TouchDialog
	case p.inventory != nil && p.inventory.State.Screen == mobileui.ScreenInventory:
		owner = mobileui.TouchInventory
	case p.inventory != nil && p.inventory.State.Screen == mobileui.ScreenEquipment:
		owner = mobileui.TouchEquipment
	default:
		owner = mobileui.TouchHUD
	}
	if owner == mobileui.TouchUnclaimed || !p.ConsumeTouch(point) {
		return false
	}
	p.hudTouchStart = mobileui.Hit{}
	if owner == mobileui.TouchHUD {
		p.hudTouchStart = p.hud.HitTest(float32(point.X), float32(point.Y))
	}
	if !p.touch.Begin(point, owner, owner == mobileui.TouchDialog || (p.economyController != nil && p.economyController.Quantity.Open) || p.navigation.Targeting.Mode != input.SkillTargetIdle) {
		p.hudTouchStart = mobileui.Hit{}
		return false
	}
	return true
}

func (p *mobilePresentation) CancelTouch() {
	if p != nil {
		p.touch.Cancel()
		p.hudTouchStart = mobileui.Hit{}
	}
}

func (p *mobilePresentation) Move(id input.TouchID, x, y int) {
	if p == nil {
		return
	}
	lx, ly := p.logicalPoint(x, y)
	_, dy, owned := p.touch.Move(input.TouchPoint{ID: id, X: lx, Y: ly})
	if !owned {
		return
	}
	switch p.touch.Owner {
	case mobileui.TouchDialog:
		if p.tradeController != nil && p.tradeController.IsOpen() {
			p.tradeController.ScrollBy(-dy)
		} else if p.vendingController != nil && p.vendingController.IsOpen() {
			p.vendingController.ScrollBy(-dy)
		} else if p.chatController != nil && p.chatController.Model.Open {
			p.chatController.ScrollBy(-dy)
		} else if p.dialogController != nil && p.dialogController.Model.Open {
			if p.dialogController.ScrollBy(-dy) && p.widgets != nil {
				p.widgets.Invalidate()
			}
		} else if p.settingsController != nil && p.navigation.Screen == mobileui.ScreenSettings {
			p.settingsController.ScrollBy(-dy)
		}
	case mobileui.TouchProfile:
		// Profile editing is a bounded form. The on-screen keyboard and
		// appearance controls own taps; dragging does not scroll the world.
	case mobileui.TouchInventory:
		before := p.inventory.State.Scroll.Offset
		p.inventory.ScrollBy(-dy)
		if p.widgets != nil && p.inventory.State.Scroll.Offset != before {
			p.widgets.Invalidate()
		}
	case mobileui.TouchEquipment:
		before := p.inventory.State.Scroll.Offset
		p.inventory.ScrollBy(-dy)
		if p.widgets != nil && p.inventory.State.Scroll.Offset != before {
			p.widgets.Invalidate()
		}
	case mobileui.TouchEconomy:
		if p.economyController.ScrollBy(-dy) && p.widgets != nil {
			p.widgets.Invalidate()
		}
	case mobileui.TouchCharacter, mobileui.TouchSkills:
		if p.characterSkills.ScrollBy(-dy) && p.widgets != nil {
			p.widgets.Invalidate()
		}
	case mobileui.TouchMap:
		p.mapController.ScrollBy(-dy)
	case mobileui.TouchSocial:
		p.socialController.ScrollBy(-dy)
	}
}

func (p *mobilePresentation) Release(id input.TouchID, x, y int) {
	if p == nil {
		return
	}
	rawX, rawY := x, y
	x, y = p.logicalPoint(x, y)
	startHUDHit := p.hudTouchStart
	p.hudTouchStart = mobileui.Hit{}
	owner, moved := p.touch.End(id)
	if owner == mobileui.TouchUnclaimed || moved {
		return
	}
	// Controllers mutate selection, tabs, detail sheets, navigation, and other
	// presentation state synchronously on tap. Their geometry is recomputed
	// immediately, but the retained widget raster is separate state. Always
	// invalidate it after a claimed tap so sprite overlays and widget chrome
	// cannot diverge for a frame (or indefinitely on otherwise-static screens).
	defer func() {
		if p.widgets != nil {
			p.widgets.Invalidate()
		}
	}()
	if owner == mobileui.TouchStartup {
		action := p.startup.ActionAt(float32(x), float32(y))
		if action == mobileui.StartupSwitchOnline {
			if p.modeChanged != nil {
				p.modeChanged(true)
			}
			return
		}
		if p.startup.Tap(float32(x), float32(y)) && p.startup.Phase == mobileui.StartupProfile {
			p.profileController.OpenProfile(p.game.MobileProfileModel())
			p.navigation.Open(mobileui.ScreenProfile)
		}
		return
	}
	if owner == mobileui.TouchOnline {
		p.handleOnlineTouch(float32(x), float32(y))
		return
	}
	if p.tradeController != nil && p.tradeController.IsOpen() {
		p.tradeController.Tap(float32(x), float32(y))
		if !p.tradeController.IsOpen() {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
		return
	}
	if p.vendingController != nil && p.vendingController.IsOpen() {
		p.vendingController.Tap(float32(x), float32(y))
		if !p.vendingController.IsOpen() {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
		return
	}
	if p.profileController != nil && p.profileController.Open {
		// The desktop character windows own this screen, so the tap goes to
		// them rather than to the controller's own hit tests. They drive the
		// same controller, so the outcomes checked below are unchanged.
		wasBack := false
		if p.characters == nil || !p.characters.tap(p, x, y) {
			wasBack = p.profileController.Layout.BackButton.Contains(float32(x), float32(y))
			p.profileController.Tap(float32(x), float32(y))
		}
		if p.profileController.StartRequested {
			p.profileController.StartRequested = false
			p.startup.EnterWorld()
			p.navigation.Open(mobileui.ScreenWorldHUD)
			return
		}
		if wasBack && !p.profileController.Open {
			// The profile screen is the final gate before the offline world.
			// Treat its BACK action as a safe escape into the already-loaded
			// session, so an existing character can never be stranded in the
			// selection surface.
			p.startup.EnterWorld()
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
		return
	}
	if p.chatController != nil && p.chatController.Model.Open {
		wasBack := p.chatController.Layout.Back.Contains(float32(x), float32(y))
		p.chatController.Tap(float32(x), float32(y))
		if wasBack {
			p.navigation.Back()
		}
		return
	}
	if p.dialogController != nil && p.dialogController.Model.Open {
		p.dialogController.Tap(float32(x), float32(y))
		return
	}
	if p.economyController != nil && p.economyController.Screen != mobileui.EconomyClosed {
		p.economyController.Tap(float32(x), float32(y))
		return
	}
	if p.characterSkills != nil && (p.navigation.Screen == mobileui.ScreenCharacter || p.navigation.Screen == mobileui.ScreenSkills) {
		p.characterSkills.Tap(float32(x), float32(y))
		p.navigation.Screen = p.characterSkills.Screen
		return
	}
	if p.settingsController != nil && p.navigation.Screen == mobileui.ScreenSettings {
		wasBack := p.settingsController.Layout.Back.Contains(float32(x), float32(y))
		p.settingsController.Tap(float32(x), float32(y))
		if wasBack {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
		return
	}
	if p.mapController != nil && p.navigation.Screen == mobileui.ScreenMap {
		wasBack := p.mapController.Layout.BackButton.Contains(float32(x), float32(y))
		wasConfirm := p.mapController.Layout.ConfirmButton.Contains(float32(x), float32(y))
		p.mapController.Tap(float32(x), float32(y))
		if (wasBack || wasConfirm) && !p.mapController.State.ConfirmOpen {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
		return
	}
	if p.socialController != nil && p.navigation.Screen == mobileui.ScreenSocial && p.socialController.IsOpen() {
		p.socialController.Tap(float32(x), float32(y))
		if target := p.socialController.TakeWhisperTarget(); target != "" && p.chatController != nil {
			chat := p.game.MobileChatModel()
			chat.Recipient = target
			p.chatController.Open(chat)
			p.navigation.OpenLayer(mobileui.ScreenSocial, mobileui.NavigationDetailSheet, "whisper:"+target)
		} else if !p.socialController.IsOpen() {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
		return
	}
	if p.navigation.Targeting.Mode != input.SkillTargetIdle {
		if p.hudController != nil && p.hudController.Layout.CombatCancel.Contains(float32(x), float32(y)) {
			p.hudController.Tap(float32(x), float32(y))
			p.navigation = p.hudController.Navigation
			p.hud = p.layoutHUD()
			return
		}
		screenPoint := input.WorldPosition{X: float64(rawX), Y: float64(rawY)}
		var (
			target input.PickedTarget
			ok     bool
		)
		if p.navigation.Targeting.Mode == input.SkillTargetGround {
			// Ground skills own the terrain under the finger. Normal world
			// picking prioritizes actors/items, which would otherwise make an
			// occupied cell impossible to select.
			target, ok = p.game.PickMobileGroundTarget(screenPoint)
		} else {
			target, ok = p.game.PickMobileTarget(screenPoint)
		}
		if ok {
			if command, selected := p.navigation.Targeting.Select(target); selected {
				mobileCommandSink{game: p.game, presentation: p}.Emit(command)
			}
		}
		p.hud = p.layoutHUD()
		return
	}
	if p.inventory != nil && p.inventory.State.Screen != mobileui.ScreenWorldHUD {
		p.inventory.Tap(float32(x), float32(y))
		if p.inventory.State.Screen == mobileui.ScreenWorldHUD {
			p.navigation.Open(mobileui.ScreenWorldHUD)
		}
		return
	}
	hit := p.hud.HitTest(float32(x), float32(y))
	if owner == mobileui.TouchHUD {
		// Bind a tap to the control that owned finger-down. Small finger drift
		// may remain inside touch slop; re-hit-testing only at finger-up allowed
		// adjacent actions such as Loot -> Sit to fire accidentally.
		if startHUDHit.Control == mobileui.ControlNone || hit != startHUDHit {
			return
		}
		hit = startHUDHit
	}
	switch hit.Control {
	case mobileui.ControlMenu:
		p.navigation.MenuOpen = !p.navigation.MenuOpen
		if p.navigation.MenuOpen {
			p.navigation.EmoteOpen = false
		}
	case mobileui.ControlMenuAction:
		if hit.Screen == mobileui.ScreenInventory {
			p.inventory.Open(mobileui.ScreenInventory)
			p.navigation.Open(mobileui.ScreenInventory)
		} else if hit.Screen == mobileui.ScreenProfile {
			if p.game != nil && p.game.Online() {
				p.navigation.MenuOpen = false
				break
			}
			p.profileController.OpenProfile(p.game.MobileProfileModel())
			p.navigation.Open(mobileui.ScreenProfile)
		} else if hit.Screen == mobileui.ScreenCharacter || hit.Screen == mobileui.ScreenSkills {
			p.characterSkills.Open(hit.Screen)
			p.navigation.Open(hit.Screen)
		} else if hit.Screen == mobileui.ScreenMap {
			p.mapController.SetModel(p.game.MobileMapModel())
			p.navigation.Open(mobileui.ScreenMap)
		} else if hit.Screen == mobileui.ScreenSettings {
			p.navigation.Open(mobileui.ScreenSettings)
		} else if hit.Screen == mobileui.ScreenSocial {
			p.socialController.Open(mobileui.SocialTabFriends)
			p.navigation.Open(mobileui.ScreenSocial)
		} else {
			p.navigation.Open(hit.Screen)
		}
	case mobileui.ControlChat:
		if p.chatController != nil {
			p.chatController.Open(p.game.MobileChatModel())
			p.navigation.OpenLayer(mobileui.ScreenWorldHUD, mobileui.NavigationDetailSheet, "chat")
		}
	case mobileui.ControlLevelUp:
		if p.characterSkills != nil {
			p.characterSkills.Open(mobileui.ScreenCharacter)
			p.navigation.Open(mobileui.ScreenCharacter)
		}
	case mobileui.ControlSkillUp:
		if p.characterSkills != nil {
			p.characterSkills.Open(mobileui.ScreenSkills)
			p.navigation.Open(mobileui.ScreenSkills)
		}
	case mobileui.ControlSkill, mobileui.ControlLootItem, mobileui.ControlSkillPagePrev, mobileui.ControlSkillPageNext,
		mobileui.ControlPrimaryAction, mobileui.ControlTarget, mobileui.ControlSit, mobileui.ControlLoot,
		mobileui.ControlEmoteToggle, mobileui.ControlEmote:
		if p.hudController != nil {
			p.hudController.Model = p.hudModel
			p.hudController.Navigation = p.navigation
			p.hudController.Layout = p.hud
			p.hudController.Tap(float32(x), float32(y))
			p.navigation = p.hudController.Navigation
		}
	case mobileui.ControlMinimap:
		if p.mapController != nil {
			p.mapController.SetModel(p.game.MobileMapModel())
			p.navigation.Open(mobileui.ScreenMap)
		}
	case mobileui.ControlStatus:
		if p.characterSkills != nil {
			p.characterSkills.Open(mobileui.ScreenCharacter)
			p.navigation.Open(mobileui.ScreenCharacter)
		}
	}
	p.hud = p.layoutHUD()
}

func (p *mobilePresentation) Draw(frame *render.Frame) {
	if p == nil || frame == nil {
		return
	}
	scale := p.settings.UI.Normalized().Scale
	frame.SetScreenScale(scale, scale)
	defer frame.SetScreenScale(1, 1)
	// Character selection and creation run the real desktop windows, scaled to
	// the phone, rather than a mobile redesign of them.
	if p.characters != nil && p.characters.draw(p, frame) {
		return
	}
	if p.game != nil && p.game.Online() && !p.game.SessionPlaying() {
		p.drawOnlineStatus(frame)
		return
	}
	// The desktop world renderer suppresses UI while the native mobile surface
	// is active. Reintroduce only the authoritative NPC cut-in here, beneath the
	// mobile dialog/widget layer. In portrait, anchor it to the dialog sheet.
	if p.game != nil {
		dialogTop := 0
		if p.dialogController != nil && p.dialogController.Model.Open {
			dialogTop = int(p.dialogController.Layout.Panel.Y)
		}
		p.game.DrawMobileNPCCutin(frame, dialogTop)
	}
	// The shared ui/mobile widget layer draws every screen that has a builder.
	// Screens without one still fall through to the host's own drawing below.
	if p.widgets != nil && p.widgets.drawWidgets(p, frame) {
		return
	}
	if p.startup != nil && p.startup.Phase == mobileui.StartupTitle {
		p.drawStartup(frame)
		return
	}
	if p.tradeController != nil && p.tradeController.IsOpen() {
		p.drawTrade(frame)
		return
	}
	if p.vendingController != nil && p.vendingController.IsOpen() {
		p.drawVending(frame)
		return
	}
	if p.profileController != nil && p.profileController.Open {
		p.drawProfile(frame)
		return
	}
	if p.chatController != nil && p.chatController.Model.Open {
		p.drawChat(frame)
		return
	}
	if p.dialogController != nil && p.dialogController.Model.Open {
		p.drawDialog(frame)
		return
	}
	if p.economyController != nil && p.economyController.Screen != mobileui.EconomyClosed {
		p.drawEconomy(frame)
		return
	}
	if p.characterSkills != nil && (p.navigation.Screen == mobileui.ScreenCharacter || p.navigation.Screen == mobileui.ScreenSkills) {
		p.drawCharacterSkills(frame)
		return
	}
	if p.mapController != nil && p.navigation.Screen == mobileui.ScreenMap {
		p.drawMap(frame)
		return
	}
	if p.socialController != nil && p.navigation.Screen == mobileui.ScreenSocial && p.socialController.IsOpen() {
		p.drawSocial(frame)
		return
	}
	if p.settingsController != nil && p.navigation.Screen == mobileui.ScreenSettings {
		p.drawSurface(frame, p.settingsController)
		return
	}
	if p.inventory != nil && p.inventory.State.Screen != mobileui.ScreenWorldHUD {
		p.drawInventory(frame)
		return
	}
	p.drawHUD(frame)
}

func (p *mobilePresentation) drawOnlineStatus(frame *render.Frame) {
	if p == nil || frame == nil || p.game == nil {
		return
	}
	safe := p.viewport.SafeRect()
	model := p.game.MobileLoginModel()
	layout := mobileui.LayoutOnlineLogin(p.viewport, model)
	panel := layout.Panel
	if panel.W < 0 {
		panel.W = 0
	}
	if panel.H < 0 {
		panel.H = 0
	}
	colors := mobileColors()
	scale := p.mobileTextScale()
	render.DrawRect(frame, float64(safe.X), float64(safe.Y), float64(safe.W), float64(safe.H), color.RGBA{R: 10, G: 22, B: 42, A: 210})
	drawMobilePanel(frame, panel)

	title := "GORO ONLINE"
	switch model.Phase {
	case mobileui.OnlineLoginServer:
		title = "SELECT SERVER"
	case mobileui.OnlineLoginCredentials:
		title = "ACCOUNT LOGIN"
	case mobileui.OnlineLoginCharacterService:
		title = "SELECT SERVICE"
	case mobileui.OnlineLoginConnecting:
		title = "CONNECTING"
	case mobileui.OnlineLoginCharacters:
		title = "SELECT CHARACTER"
	case mobileui.OnlineLoginCreate:
		title = "CREATE CHARACTER"
	}
	drawMobileTextCentered(frame, title, layout.Title, colors.text, scale*1.35)
	drawMobileTextCentered(frame, model.Status, layout.Status, colors.title, scale*0.82)
	drawMobileTextCentered(frame, model.Network, layout.Network, colors.muted, scale*0.62)
	if model.Notice != "" {
		drawMobileTextCentered(frame, model.Notice, layout.Notice, colors.muted, scale*0.62)
	}

	switch model.Phase {
	case mobileui.OnlineLoginServer, mobileui.OnlineLoginCharacterService:
		for i, rect := range layout.Options {
			if i >= len(model.Servers) {
				break
			}
			entry := model.Servers[i]
			active := entry.Selected || entry.Index == model.SelectedServer
			drawMobileCard(frame, rect, colors, active)
			if active {
				drawMobileSelectionOutline(frame, rect, colors)
			}
			nameRect := mobileui.Rect{X: rect.X + 14, Y: rect.Y + 6, W: rect.W - 28, H: rect.H * 0.52}
			detailRect := mobileui.Rect{X: rect.X + 14, Y: rect.Y + rect.H*0.50, W: rect.W - 28, H: rect.H * 0.40}
			drawMobileTextFit(frame, entry.Name, nameRect.X, nameRect.Y+4, nameRect.W, colors.text, scale*0.72)
			detail := entry.Detail
			if entry.UserCount > 0 {
				detail = fmt.Sprintf("%s   users %d", detail, entry.UserCount)
			}
			drawMobileTextFit(frame, detail, detailRect.X, detailRect.Y+2, detailRect.W, colors.muted, scale*0.54)
		}
		if model.CanSwitchMode {
			drawMobileTextCentered(frame, "OFFLINE MODE", layout.Mode, colors.accent, scale*0.58)
		}
		return

	case mobileui.OnlineLoginCredentials:
		username := p.onlineUsername
		if strings.TrimSpace(username) == "" {
			username = model.Username
		}
		if strings.TrimSpace(username) == "" {
			username = "tap to enter username"
		}
		password := ""
		if p.onlinePassword != "" {
			password = strings.Repeat("*", len([]rune(p.onlinePassword)))
		} else if model.PasswordSet {
			password = "********"
		} else {
			password = "tap to enter password"
		}
		drawMobileButton(frame, layout.Username, "ID   "+username, colors, scale*0.68, p.onlineInputMode == androidTextInputLoginUsername)
		drawMobileButton(frame, layout.Password, "Password   "+password, colors, scale*0.68, p.onlineInputMode == androidTextInputLoginPassword)
		loginEnabled := strings.TrimSpace(p.onlineUsername) != "" || strings.TrimSpace(model.Username) != ""
		loginEnabled = loginEnabled && (p.onlinePassword != "" || model.PasswordSet)
		drawMobileButton(frame, layout.Submit, "LOGIN", colors, scale*0.78, loginEnabled)
		if model.CanSwitchMode {
			drawMobileTextCentered(frame, "OFFLINE MODE", layout.Mode, colors.accent, scale*0.58)
		}
		return

	case mobileui.OnlineLoginCreate:
		name := p.onlineCharacterName
		if strings.TrimSpace(name) == "" {
			name = model.CreateName
		}
		if strings.TrimSpace(name) == "" {
			name = "tap to enter character name"
		}
		drawMobileButton(frame, layout.Username, "Name   "+name, colors, scale*0.68, p.onlineInputMode == androidTextInputLoginCharacterName)
		enabled := len([]byte(strings.TrimSpace(name))) >= 4 && name != "tap to enter character name"
		drawMobileButton(frame, layout.Submit, "CREATE", colors, scale*0.74, enabled)
		drawMobileButton(frame, layout.Cancel, "CANCEL", colors, scale*0.74, true)
		return

	case mobileui.OnlineLoginCharacters:
		pageStart := (model.SelectedSlot / 3) * 3
		pageCount := (len(model.Characters) + 2) / 3
		if pageCount < 1 {
			pageCount = 1
		}
		page := pageStart / 3
		for i, rect := range layout.Slots {
			index := pageStart + i
			if index < 0 || index >= len(model.Characters) {
				continue
			}
			entry := model.Characters[index]
			active := entry.Slot == model.SelectedSlot
			drawMobileCard(frame, rect, colors, active)
			if active {
				drawMobileSelectionOutline(frame, rect, colors)
			}
			label := fmt.Sprintf("SLOT %d", entry.Slot+1)
			if entry.Occupied {
				label = trimText(entry.Name, 16)
				previewTop := rect.Y + 30
				previewBottom := rect.Bottom() - 24
				if previewBottom > previewTop {
					p.game.DrawMobileLoginCharacterPreview(frame, entry.Slot,
						int(rect.X+6), int(previewTop), int(rect.W-12), int(previewBottom-previewTop))
				}
			}
			drawMobileTextCentered(frame, label, mobileui.Rect{X: rect.X + 6, Y: rect.Y + 4, W: rect.W - 12, H: 26}, colors.text, scale*0.62)
			if entry.Occupied {
				footer := fmt.Sprintf("%s   Lv %d", trimText(entry.JobName, 12), entry.Level)
				drawMobileTextCentered(frame, footer, mobileui.Rect{X: rect.X + 6, Y: rect.Bottom()-23, W: rect.W - 12, H: 18}, colors.muted, scale*0.48)
			} else {
				drawMobileTextCentered(frame, "EMPTY SLOT", mobileui.Rect{X: rect.X + 6, Y: rect.Y + rect.H*0.48, W: rect.W - 12, H: 24}, colors.accent, scale*0.54)
			}
		}

		var selected mobileui.OnlineCharacterSlot
		for _, entry := range model.Characters {
			if entry.Slot == model.SelectedSlot {
				selected = entry
				break
			}
		}
		if layout.CharacterInfo.W > 0 && layout.CharacterInfo.H > 0 {
			drawMobilePanel(frame, layout.CharacterInfo)
			if selected.Occupied {
				pad := float32(14)
				innerX := layout.CharacterInfo.X + pad
				innerW := layout.CharacterInfo.W - 2*pad
				rowH := minf32(32, maxf32(20, (layout.CharacterInfo.H-16)/7))
				y := layout.CharacterInfo.Y + 8
				drawMobileTextFit(frame, trimText(selected.Name, 20), innerX, y, innerW, colors.text, scale*0.68)
				y += rowH
				drawMobileTextFit(frame, trimText(selected.JobName, 20), innerX, y, innerW, colors.muted, scale*0.56)
				y += rowH
				drawMobileTextFit(frame, fmt.Sprintf("Base %d   Job %d   EXP %d", selected.Level, selected.JobLevel, selected.Exp), innerX, y, innerW, colors.muted, scale*0.54)
				y += rowH
				drawMobileTextFit(frame, fmt.Sprintf("HP %d/%d   SP %d/%d", selected.HP, selected.MaxHP, selected.SP, selected.MaxSP), innerX, y, innerW, colors.muted, scale*0.54)
				y += rowH
				drawMobileTextFit(frame, fmt.Sprintf("STR %d   AGI %d   VIT %d", selected.Str, selected.Agi, selected.Vit), innerX, y, innerW, colors.text, scale*0.54)
				y += rowH
				drawMobileTextFit(frame, fmt.Sprintf("INT %d   DEX %d   LUK %d", selected.Int, selected.Dex, selected.Luk), innerX, y, innerW, colors.text, scale*0.54)
				y += rowH
				drawMobileTextFit(frame, fmt.Sprintf("Zeny %d", selected.Zeny), innerX, y, innerW, colors.accent, scale*0.54)
			} else {
				drawMobileTextCentered(frame, fmt.Sprintf("Slot %d is empty", model.SelectedSlot+1), layout.CharacterInfo, colors.muted, scale*0.62)
			}
		}
		drawMobileTextCentered(frame, fmt.Sprintf("PAGE %d / %d", page+1, pageCount), layout.PageLabel, colors.muted, scale*0.54)
		drawMobileButton(frame, layout.PagePrev, "‹ PREV", colors, scale*0.58, page > 0)
		drawMobileButton(frame, layout.PageNext, "NEXT ›", colors, scale*0.58, page+1 < pageCount)
		actionLabel := "CREATE"
		actionEnabled := model.CanCreate
		if selected.Occupied {
			actionLabel = "ENTER WORLD"
			actionEnabled = true
		}
		drawMobileButton(frame, layout.Create, actionLabel, colors, scale*0.62, actionEnabled)
		return

	default:
		drawMobileButton(frame, layout.Reconnect, "RECONNECT", colors, scale*0.64, model.CanReconnect)
		drawMobileButton(frame, layout.Disconnect, "DISCONNECT", colors, scale*0.64, model.CanDisconnect)
	}
}

func (p *mobilePresentation) handleOnlineTouch(x, y float32) {
	if p == nil || p.game == nil {
		return
	}
	model := p.game.MobileLoginModel()
	layout := mobileui.LayoutOnlineLogin(p.viewport, model)

	switch model.Phase {
	case mobileui.OnlineLoginServer, mobileui.OnlineLoginCharacterService:
		for i, rect := range layout.Options {
			if !rect.Contains(x, y) || i >= len(model.Servers) {
				continue
			}
			entry := model.Servers[i]
			kind := input.CommandOnlineSelectLoginServer
			if model.Phase == mobileui.OnlineLoginCharacterService {
				kind = input.CommandOnlineSelectCharacterService
			}
			p.onlineInputMode = androidTextInputNone
			p.syncTextInputState()
			p.emitMobileCommand(input.PlayerCommand{Kind: kind, Slot: uint16(entry.Index)})
			return
		}
		if layout.Mode.Contains(x, y) && model.CanSwitchMode && p.modeChanged != nil {
			p.modeChanged(false)
		}
		return

	case mobileui.OnlineLoginCredentials:
		if layout.Username.Contains(x, y) {
			if strings.TrimSpace(p.onlineUsername) == "" {
				p.onlineUsername = model.Username
			}
			p.onlineInputMode = androidTextInputLoginUsername
			p.syncTextInputState()
			return
		}
		if layout.Password.Contains(x, y) {
			p.onlineInputMode = androidTextInputLoginPassword
			p.syncTextInputState()
			return
		}
		if layout.Submit.Contains(x, y) {
			username := p.onlineUsername
			if strings.TrimSpace(username) == "" {
				username = model.Username
			}
			password := p.onlinePassword
			if strings.TrimSpace(username) == "" || (password == "" && !model.PasswordSet) {
				return
			}
			p.onlineInputMode = androidTextInputNone
			p.syncTextInputState()
			p.emitMobileCommand(input.PlayerCommand{
				Kind: input.CommandOnlineSubmitCredentials,
				Username: username,
				Password: password,
			})
			return
		}
		if layout.Mode.Contains(x, y) && model.CanSwitchMode && p.modeChanged != nil {
			p.onlineInputMode = androidTextInputNone
			p.syncTextInputState()
			p.modeChanged(false)
			return
		}
		// A text field is only active while the player is explicitly editing it.
		// Tapping the background must not behave like another username tap.
		if p.onlineInputMode != androidTextInputNone {
			p.onlineInputMode = androidTextInputNone
			p.syncTextInputState()
		}
		return

	case mobileui.OnlineLoginCreate:
		if layout.Username.Contains(x, y) {
			if strings.TrimSpace(p.onlineCharacterName) == "" {
				p.onlineCharacterName = model.CreateName
			}
			p.onlineInputMode = androidTextInputLoginCharacterName
			p.syncTextInputState()
			return
		}
		if layout.Submit.Contains(x, y) {
			name := strings.TrimSpace(p.onlineCharacterName)
			if name == "" {
				name = strings.TrimSpace(model.CreateName)
			}
			if len([]byte(name)) < 4 {
				return
			}
			p.onlineInputMode = androidTextInputNone
			p.syncTextInputState()
			p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineCreateCharacter, Slot: uint16(model.CreateSlot), Text: name})
			return
		}
		if layout.Cancel.Contains(x, y) {
			p.onlineInputMode = androidTextInputNone
			p.onlineCharacterName = ""
			p.syncTextInputState()
			p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineCancelCharacterCreate})
			return
		}
		if p.onlineInputMode != androidTextInputNone {
			p.onlineInputMode = androidTextInputNone
			p.syncTextInputState()
		}
		return

	case mobileui.OnlineLoginCharacters:
		pageStart := (model.SelectedSlot / 3) * 3
		for i, rect := range layout.Slots {
			index := pageStart + i
			if !rect.Contains(x, y) || index < 0 || index >= len(model.Characters) {
				continue
			}
			entry := model.Characters[index]
			p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineFocusCharacter, Slot: uint16(entry.Slot)})
			return
		}
		if layout.PagePrev.Contains(x, y) && pageStart > 0 {
			p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineFocusCharacter, Slot: uint16(pageStart - 3)})
			return
		}
		if layout.PageNext.Contains(x, y) && pageStart+3 < len(model.Characters) {
			p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineFocusCharacter, Slot: uint16(pageStart + 3)})
			return
		}
		if layout.Create.Contains(x, y) {
			selectedOccupied := false
			for _, entry := range model.Characters {
				if entry.Slot == model.SelectedSlot {
					selectedOccupied = entry.Occupied
					break
				}
			}
			kind := input.CommandOnlineCreateCharacter
			if selectedOccupied {
				kind = input.CommandOnlineSelectCharacter
			} else if !model.CanCreate {
				return
			}
			p.emitMobileCommand(input.PlayerCommand{Kind: kind, Slot: uint16(model.SelectedSlot)})
		}
		return
	}

	if layout.Reconnect.Contains(x, y) && model.CanReconnect {
		p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineReconnect})
		return
	}
	if layout.Disconnect.Contains(x, y) && model.CanDisconnect {
		p.emitMobileCommand(input.PlayerCommand{Kind: input.CommandOnlineDisconnect})
	}
}

func (p *mobilePresentation) WorldActive() bool {
	return p != nil && (p.startup == nil || p.startup.Phase == mobileui.StartupWorld)
}

// WorldTouchAvailable reports whether a live touch can drive the world. It
// deliberately checks the same UI hit-test path as the mobile input adapter,
// so the highlight cannot appear underneath a control-owned touch.
func (p *mobilePresentation) WorldTouchAvailable(point input.TouchPoint) bool {
	if p == nil || !p.WorldActive() || p.navigation.Screen != mobileui.ScreenWorldHUD || p.navigation.MenuOpen {
		return false
	}
	if p.navigation.Targeting.Mode != input.SkillTargetIdle || p.navigation.Top().Layer != mobileui.NavigationFullScreen {
		return false
	}
	return !p.ConsumeTouch(point)
}

func (p *mobilePresentation) drawStartup(frame *render.Frame) {
	l := p.startup.Layout
	c := mobileColors()
	s := p.mobileTextScale()
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 10, G: 22, B: 42, A: 255})
	drawMobilePanel(frame, l.Panel)
	drawMobileTextCentered(frame, "GORO", l.Logo, c.text, s*2.2)
	drawMobileTextCentered(frame, "Offline Adventure", l.Subtitle, c.text, s)
	drawMobileButton(frame, l.Action, "START OFFLINE", c, s, true)
	drawMobileTextCentered(frame, "ONLINE MODE", l.AlternateAction, c.accent, s*0.72)
}

func (p *mobilePresentation) drawProfile(frame *render.Frame) {
	c := p.profileController
	if c == nil || !c.Open {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	model := c.Model
	title := "PROFILE"
	if c.Editor {
		model = c.Draft
		if c.NewProfile {
			title = "CREATE CHARACTER"
		} else {
			title = "CUSTOMIZE PROFILE"
		}
	}
	l := c.Layout
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 35, G: 62, B: 88, A: 150})
	drawMobilePanel(frame, l.Panel)
	drawMobileHeaderWithBack(frame, l.HeaderTitle, l.BackButton, title, colors, textScale, "BACK")

	drawMobilePanel(frame, l.Preview)
	drawMobileHeader(frame, l.Preview, "APPEARANCE PREVIEW", colors, textScale*0.86)
	preview := mobileui.Rect{X: l.Preview.X + 12, Y: l.Preview.Y + 38, W: l.Preview.W - 24, H: l.Preview.H - 50}
	if p.game != nil {
		p.game.DrawMobileProfilePreview(frame, model, int(preview.X), int(preview.Y), int(preview.W), int(preview.H))
	}
	drawMobileTextCentered(frame, fmt.Sprintf("%s  /  %s", trimText(model.Name, 18), mobileui.ProfileSexLabel(model.Sex)), mobileui.Rect{X: l.Preview.X + 12, Y: l.Preview.Bottom() - 38, W: l.Preview.W - 24, H: 24}, colors.text, textScale*0.68)

	drawMobilePanel(frame, l.Identity)
	drawMobileHeader(frame, l.Identity, "IDENTITY", colors, textScale*0.86)
	drawMobileTextFit(frame, "NAME", l.NameLabel.X, l.NameLabel.Y, l.NameLabel.W, colors.muted, textScale*0.62)
	drawMobileButton(frame, l.NameField, trimText(model.Name, 23), colors, textScale*0.82, c.Editor && c.EditingName)
	if !c.Editor {
		drawMobileTextFit(frame, fmt.Sprintf("SEX  %s", mobileui.ProfileSexLabel(model.Sex)), l.SexLabel.X, l.SexLabel.Y, l.SexLabel.W, colors.muted, textScale*0.62)
	}

	drawMobilePanel(frame, l.Appearance)
	drawMobileHeader(frame, l.Appearance, "CUSTOMIZATION", colors, textScale*0.86)
	if c.Editor {
		drawMobileButton(frame, l.SexButton, mobileui.ProfileSexLabel(model.Sex), colors, textScale*0.70, true)
		drawMobileButton(frame, l.HairPrev, "<", colors, textScale*0.92, false)
		drawMobileButton(frame, l.HairNext, ">", colors, textScale*0.92, false)
		drawMobileButton(frame, l.HairColor, fmt.Sprintf("HAIR COLOR %d", model.HairColor+1), colors, textScale*0.62, false)
		drawMobileTextFit(frame, fmt.Sprintf("HAIR STYLE %d", model.HairStyle), l.AppearanceLabel.X, l.AppearanceLabel.Y, l.AppearanceLabel.W, colors.muted, textScale*0.62)
	} else {
		drawMobileTextFit(frame, fmt.Sprintf("SEX  %s", mobileui.ProfileSexLabel(model.Sex)), l.Appearance.X+12, l.Appearance.Y+48, l.Appearance.W-24, colors.text, textScale*0.80)
		drawMobileTextFit(frame, fmt.Sprintf("HAIR STYLE  %d    COLOR %d", model.HairStyle, model.HairColor+1), l.Appearance.X+12, l.Appearance.Y+78, l.Appearance.W-24, colors.muted, textScale*0.66)
	}

	drawMobilePanel(frame, l.Stats)
	statLabels := mobileui.ProfileStatLabels()
	statTotal := 0
	for i, value := range model.Stats {
		statTotal += int(value)
		row := l.StatRows[i]
		if row.W <= 0 || row.H <= 0 {
			continue
		}
		drawMobileTextFit(frame, fmt.Sprintf("%s  %d", statLabels[i], value), row.X+8, row.Y+10, maxf32(32, row.W-112), colors.title, textScale*0.72)
		if c.Editor && l.StatMinus[i].W > 0 {
			drawMobileButton(frame, l.StatMinus[i], "−", colors, textScale*0.76, false)
			drawMobileButton(frame, l.StatPlus[i], "+", colors, textScale*0.76, false)
		}
	}
	drawMobileTextFit(frame, fmt.Sprintf("STARTER TOTAL %d / 30", statTotal), l.StarterTotal.X, l.StarterTotal.Y, l.StarterTotal.W, colors.accent, textScale*0.60)

	if c.Editor {
		drawMobileButton(frame, l.SaveButton, "SAVE", colors, textScale*0.76, true)
		drawMobileButton(frame, l.CancelButton, "CANCEL", colors, textScale*0.76, false)
	} else {
		drawMobileButton(frame, l.EditButton, "EDIT PROFILE", colors, textScale*0.70, true)
		drawMobileButton(frame, l.NewButton, "NEW CHARACTER", colors, textScale*0.64, false)
		drawMobileButton(frame, l.ContinueButton, "CONTINUE", colors, textScale*0.76, true)
	}
	if model.Notice != "" && l.Notice.W > 0 {
		drawMobileTextFit(frame, model.Notice, l.Notice.X, l.Notice.Y, l.Notice.W, colors.accent, textScale*0.58)
	}
	if c.EditingName {
		render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 20, G: 30, B: 44, A: 150})
		drawMobilePanel(frame, l.Keyboard)
		drawMobileHeader(frame, l.Keyboard, "CHARACTER NAME", colors, textScale*0.82)
		for _, key := range l.Keys {
			drawMobileButton(frame, key.Rect, key.Key, colors, textScale*0.58, false)
		}
	}
}

func (p *mobilePresentation) drawHUD(frame *render.Frame) {
	l := p.hud
	colors := mobileColors()
	textScale := p.mobileTextScale()

	drawMobilePanel(frame, l.PlayerPanel)
	drawMobileHeader(frame, l.PlayerPanel, "CHARACTER", colors, textScale)
	// Keep the status panel legible at phone distance. The two bars are given
	// their own rows instead of compressing desktop-sized labels into 96px.
	drawMobileTextFit(frame, trimText(p.hudModel.Player.Name, 18), l.PlayerPanel.X+12, l.PlayerPanel.Y+38, l.PlayerPanel.W*0.54, colors.text, textScale*1.02)
	drawMobileTextFit(frame, fmt.Sprintf("BASE %d   JOB %d", p.hudModel.Player.BaseLevel, p.hudModel.Player.JobLevel), l.PlayerPanel.X+l.PlayerPanel.W*0.58, l.PlayerPanel.Y+39, l.PlayerPanel.W*0.38, colors.muted, textScale*0.84)
	drawMobileBar(frame, "HP", l.PlayerPanel.X+12, l.PlayerPanel.Y+68, l.PlayerPanel.W-24, 22, p.hudModel.Player.HP, p.hudModel.Player.MaxHP, colors.hp, colors, textScale)
	drawMobileBar(frame, "SP", l.PlayerPanel.X+12, l.PlayerPanel.Y+101, l.PlayerPanel.W-24, 22, p.hudModel.Player.SP, p.hudModel.Player.MaxSP, colors.sp, colors, textScale)
	drawMobileStatuses(frame, l.StatusArea, p.hudModel.Statuses, colors, textScale)
	if l.LevelUpAction.W > 0 {
		drawMobileButton(frame, l.LevelUpAction, "LV+", colors, textScale*0.72, true)
	}
	if l.SkillUpAction.W > 0 {
		drawMobileButton(frame, l.SkillUpAction, "SK+", colors, textScale*0.72, true)
	}

	if p.hudModel.Target.Visible && p.settings.Controls.ShowTargetNames {
		drawMobilePanel(frame, l.TargetPanel)
		drawMobileHeader(frame, l.TargetPanel, "TARGET", colors, textScale)
		drawMobileTextFit(frame, trimText(p.hudModel.Target.Name, 18), l.TargetPanel.X+12, l.TargetPanel.Y+39, l.TargetPanel.W*0.60, colors.text, textScale*0.96)
		drawMobileTextFit(frame, targetRelationText(p.hudModel.Target.Relation), l.TargetPanel.X+l.TargetPanel.W*0.64, l.TargetPanel.Y+40, l.TargetPanel.W*0.32, colors.muted, textScale*0.78)
		if p.hudModel.Target.MaxHP > 0 {
			drawMobileBar(frame, "HP", l.TargetPanel.X+12, l.TargetPanel.Y+72, l.TargetPanel.W-24, 22, p.hudModel.Target.HP, p.hudModel.Target.MaxHP, colors.target, colors, textScale)
		} else {
			drawMobileText(frame, "HP --", l.TargetPanel.X+12, l.TargetPanel.Y+78, colors.muted, textScale*0.78)
		}
	}
	if l.LootPanel.W > 0 && len(p.hudModel.Loot) > 0 {
		drawMobilePanel(frame, l.LootPanel)
		drawMobileHeader(frame, l.LootPanel, "LOOT NEARBY", colors, textScale)
		for i, row := range l.LootRows {
			if i >= len(p.hudModel.Loot) {
				break
			}
			loot := p.hudModel.Loot[i]
			drawMobileButton(frame, row, "", colors, textScale, loot.PickupReady)
			if p.game != nil {
				p.game.DrawMobileInventoryItemIcon(frame, mobileui.InventoryItemModel{
					ItemID: loot.ItemID, Identified: loot.Identified, Quantity: loot.Quantity,
				}, int(row.X)+8, int(row.Y)+4, 40)
			}
			drawMobileText(frame, trimText(loot.Name, 15), row.X+56, row.Y+9, colors.text, textScale*0.82)
			quantity := fmt.Sprintf("x%d", loot.Quantity)
			drawMobileText(frame, quantity, row.Right()-72, row.Y+9, colors.title, textScale*0.82)
			action := fmt.Sprintf("%d cells", loot.Distance)
			if loot.PickupReady {
				action = "PICK UP"
			}
			drawMobileText(frame, action, row.X+56, row.Bottom()-18, colors.muted, textScale*0.70)
		}
	}
	if l.ChatBar.W > 0 {
		drawMobilePanel(frame, l.ChatBar)
		drawMobileTextCentered(frame, "CHAT", l.ChatLabel, colors.title, textScale*0.78)
		drawMobileTextCentered(frame, "Tap to chat", l.ChatPrompt, colors.muted, textScale*0.62)
		drawMobileButton(frame, l.ChatButton, "OPEN", colors, textScale*0.78, false)
	}

	if p.settings.Display.ShowMinimap && l.Minimap.W > 0 {
		drawMobilePanel(frame, l.Minimap)
		drawMobileHeader(frame, l.Minimap, "MINI MAP", colors, textScale)
		mapRect := mobileui.Rect{X: l.Minimap.X + 10, Y: l.Minimap.Y + 34, W: l.Minimap.W - 20, H: l.Minimap.H - 70}
		render.DrawRect(frame, float64(mapRect.X), float64(mapRect.Y), float64(mapRect.W), float64(mapRect.H), colors.mapBackground)
		if !p.game.DrawMobileMinimap(frame, mapRect) {
			p.drawMinimapTerrain(frame, mapRect)
		}
		drawMobileText(frame, strings.ToUpper(trimText(p.hudModel.Minimap.MapName, 16)), l.Minimap.X+12, l.Minimap.Bottom()-30, colors.muted, textScale*0.72)
		drawMobileTextFit(frame, fmt.Sprintf("X:%d  Y:%d", p.hudModel.Minimap.PlayerX, p.hudModel.Minimap.PlayerY), l.Minimap.X+l.Minimap.W*0.52, l.Minimap.Bottom()-30, l.Minimap.W*0.42, colors.muted, textScale*0.66)
	}
	drawMobileButton(frame, l.Menu, "MENU", colors, textScale*0.86, false)

	for i, slot := range l.SkillSlots {
		index := i + l.SkillStart
		shortcut, ok := mobileui.ShortcutAt(p.hudModel, index)
		if !ok {
			drawMobileEmptySlot(frame, slot, colors)
			continue
		}
		switch shortcut.Kind {
		case mobileui.ShortcutItem:
			drawMobileItemShortcut(frame, slot, shortcut.Item, i, p.game, colors, textScale)
		case mobileui.ShortcutSkill:
			drawMobileSkill(frame, slot, shortcut.Skill, i, p.game, colors, textScale)
		default:
			drawMobileEmptySlot(frame, slot, colors)
		}
	}
	if l.SkillPagePrev.W > 0 {
		perPage := l.SkillsPerPage
		if perPage <= 0 {
			perPage = 4
		}
		drawMobileButton(frame, l.SkillPagePrev, "‹", colors, textScale*1.05, p.navigation.SkillPage > 0)
		drawMobileButton(frame, l.SkillPageNext, "›", colors, textScale*1.05, (p.navigation.SkillPage+1)*perPage < mobileui.ShortcutCount(p.hudModel))
	}
	if p.navigation.Targeting.Mode != input.SkillTargetIdle {
		drawMobilePanel(frame, l.CombatBanner)
		drawMobileText(frame, mobileui.TargetingPrompt(p.navigation.Targeting.Mode), l.CombatBanner.X+16, l.CombatBanner.Y+31, colors.title, textScale*0.86)
		drawMobileButton(frame, l.CombatCancel, "CANCEL", colors, textScale*0.76, false)
	}

	if p.navigation.MenuOpen {
		// Keep the world visible while making the drawer the clear foreground
		// surface. The drawer is painted again below the scrim.
		render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 74})
		drawMobilePanel(frame, l.MenuPanel)
		for _, action := range l.MenuActions {
			drawMobileButton(frame, action.Rect, strings.ToUpper(action.Screen.String()), colors, textScale*0.92, false)
		}
	}
}

func (p *mobilePresentation) refreshMinimapImage() {
	if p == nil {
		return
	}
	raster := p.hudModel.Minimap.Raster
	checksum := uint64(1469598103934665603)
	for _, cell := range raster.Cells {
		checksum ^= uint64(cell)
		checksum *= 1099511628211
	}
	signature := fmt.Sprintf("%s:%d:%d:%d:%x", p.hudModel.Minimap.MapName, raster.Width, raster.Height, len(raster.Cells), checksum)
	if signature == p.minimapSignature {
		return
	}
	p.minimapSignature = signature
	p.minimapImage = nil
	if raster.Width <= 0 || raster.Height <= 0 || len(raster.Cells) < raster.Width*raster.Height {
		return
	}

	img := image.NewRGBA(image.Rect(0, 0, raster.Width, raster.Height))
	for y := 0; y < raster.Height; y++ {
		for x := 0; x < raster.Width; x++ {
			cell := raster.Cells[x+y*raster.Width]
			fill := color.RGBA{R: 27, G: 52, B: 70, A: 255}
			switch cell {
			case 1:
				fill = color.RGBA{R: 104, G: 143, B: 105, A: 255}
			case 2:
				fill = color.RGBA{R: 151, G: 125, B: 91, A: 255}
			}
			img.SetRGBA(x, y, fill)
		}
	}
	p.minimapImage = render.NewImageFromImage(img)
}

func (p *mobilePresentation) drawMinimapTerrain(frame *render.Frame, rect mobileui.Rect) {
	if p == nil || frame == nil {
		return
	}
	raster := p.hudModel.Minimap.Raster
	if p.minimapImage == nil || raster.Width <= 0 || raster.Height <= 0 || rect.W <= 0 || rect.H <= 0 {
		return
	}

	imageWidth, imageHeight := float64(raster.Width), float64(raster.Height)
	drawWidth, drawHeight := float64(rect.W), float64(rect.H)
	if imageWidth/imageHeight > drawWidth/drawHeight {
		drawHeight = drawWidth * imageHeight / imageWidth
	} else {
		drawWidth = drawHeight * imageWidth / imageHeight
	}
	drawX := float64(rect.X) + (float64(rect.W)-drawWidth)/2
	drawY := float64(rect.Y) + (float64(rect.H)-drawHeight)/2

	var options render.DrawImageOptions
	options.Filter = render.FilterNearest
	options.GeoM.Scale(drawWidth/imageWidth, drawHeight/imageHeight)
	options.GeoM.Translate(drawX, drawY)
	frame.DrawImage(p.minimapImage, &options)

	mapX := clampMinimapCoordinate(float64(p.hudModel.Minimap.PlayerX)*0.5, raster.Width)
	mapY := clampMinimapCoordinate(float64(p.hudModel.Minimap.PlayerY)*0.5, raster.Height)
	markerX := drawX + (mapX+0.5)*drawWidth/imageWidth
	markerY := drawY + drawHeight - (mapY+0.5)*drawHeight/imageHeight
	colors := mobileColors()
	p.drawMinimapMarkers(frame, mobileui.Rect{X: float32(drawX), Y: float32(drawY), W: float32(drawWidth), H: float32(drawHeight)}, raster, p.hudModel.Minimap.Markers)
	// A larger crosshair/arrow remains readable on a phone and is distinct
	// from hostile, NPC, and item markers.
	render.DrawRect(frame, markerX-6, markerY-6, 12, 12, colors.marker)
	render.DrawLine(frame, markerX, markerY-12, markerX, markerY+12, colors.marker)
	render.DrawLine(frame, markerX-12, markerY, markerX+12, markerY, colors.marker)
}

func (p *mobilePresentation) drawMinimapMarkers(frame *render.Frame, rect mobileui.Rect, raster mobileui.MinimapRaster, markers []mobileui.MinimapMarkerModel) {
	if frame == nil || rect.W <= 0 || rect.H <= 0 || raster.Width <= 0 || raster.Height <= 0 {
		return
	}
	colors := mobileColors()
	for _, marker := range markers {
		if marker.Kind == mobileui.MinimapMarkerWarp {
			continue
		}
		x, y := minimapPoint(rect, raster, marker.X, marker.Y)
		fill := colors.accent
		if marker.Kind == mobileui.MinimapMarkerHostile {
			fill = colors.target
		} else if marker.Kind == mobileui.MinimapMarkerItem {
			fill = colors.marker
		}
		size := float64(3)
		if marker.Kind == mobileui.MinimapMarkerHostile {
			size = 5
		}
		if marker.Selected {
			size = 5
		}
		render.DrawRect(frame, x-size, y-size, size*2, size*2, fill)
	}
}

func minimapPoint(rect mobileui.Rect, raster mobileui.MinimapRaster, x, y int) (float64, float64) {
	imageWidth, imageHeight := float64(raster.Width), float64(raster.Height)
	drawWidth, drawHeight := float64(rect.W), float64(rect.H)
	if imageWidth/imageHeight > drawWidth/drawHeight {
		drawHeight = drawWidth * imageHeight / imageWidth
	} else {
		drawWidth = drawHeight * imageWidth / imageHeight
	}
	drawX := float64(rect.X) + (float64(rect.W)-drawWidth)/2
	drawY := float64(rect.Y) + (float64(rect.H)-drawHeight)/2
	mapX := clampMinimapCoordinate(float64(x)*0.5, raster.Width)
	mapY := clampMinimapCoordinate(float64(y)*0.5, raster.Height)
	return drawX + (mapX+0.5)*drawWidth/imageWidth, drawY + drawHeight - (mapY+0.5)*drawHeight/imageHeight
}

func clampMinimapCoordinate(value float64, size int) float64 {
	if size <= 1 {
		return 0
	}
	if value < 0 {
		return 0
	}
	maximum := float64(size - 1)
	if value > maximum {
		return maximum
	}
	return value
}

func (p *mobilePresentation) drawCharacterSkills(frame *render.Frame) {
	c := p.characterSkills
	if c == nil {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	if c.Screen == mobileui.ScreenCharacter {
		l, model := c.CharacterLayout, c.Character
		drawMobilePanel(frame, l.Panel)
		drawMobileHeaderWithBack(frame, l.Header, l.BackButton, "CHARACTER", colors, textScale, "BACK")
		drawMobileText(frame, trimText(model.Name, 24), l.Header.X+126, l.Header.Y+22, colors.title, textScale*1.08)
		if model.JobName != "" {
			drawMobileText(frame, trimText(model.JobName, 18), l.Header.Right()-190, l.Header.Y+23, colors.muted, textScale*0.82)
		}

		if mobileSectionVisible(l.Vitals, l.ContentViewport, l.Stacked) {
			drawMobilePanel(frame, l.Vitals)
			drawMobileHeader(frame, l.Vitals, "VITALS", colors, textScale)
			drawMobileBar(frame, "HP", l.Vitals.X+16, l.Vitals.Y+48, l.Vitals.W/2-24, 16, model.HP, model.MaxHP, colors.hp, colors, textScale)
			drawMobileBar(frame, "SP", l.Vitals.X+l.Vitals.W/2+8, l.Vitals.Y+48, l.Vitals.W/2-24, 16, model.SP, model.MaxSP, colors.sp, colors, textScale)
			drawMobileText(frame, fmt.Sprintf("BASE %d   JOB %d", model.BaseLevel, model.JobLevel), l.Vitals.X+16, l.Vitals.Bottom()-18, colors.muted, textScale*0.78)
			drawMobileText(frame, fmt.Sprintf("WEIGHT %d / %d", model.Weight/10, model.MaxWeight/10), l.Vitals.Right()-190, l.Vitals.Bottom()-18, colors.muted, textScale*0.78)
		}

		if mobileSectionVisible(l.Progress, l.ContentViewport, l.Stacked) {
			drawMobilePanel(frame, l.Progress)
			drawMobileHeader(frame, l.Progress, "EXPERIENCE", colors, textScale)
			drawMobileBar(frame, "BASE", l.Progress.X+16, l.Progress.Y+48, l.Progress.W-32, 14, int(model.BaseExp), int(model.NextBaseExp), colors.accent, colors, textScale)
			drawMobileBar(frame, "JOB", l.Progress.X+16, l.Progress.Y+76, l.Progress.W-32, 14, int(model.JobExp), int(model.NextJobExp), colors.accent, colors, textScale)
		}

		if mobileSectionVisible(l.Stats, l.ContentViewport, l.Stacked) {
			drawMobilePanel(frame, l.Stats)
			drawMobileHeader(frame, l.Stats, fmt.Sprintf("ATTRIBUTES   %d POINTS", model.StatPoints), colors, textScale)
			for i, stat := range model.Stats {
				y := l.Stats.Y + 44 + float32(i)*32
				drawMobileText(frame, stat.Label, l.Stats.X+16, y, colors.title, textScale*0.84)
				drawMobileText(frame, fmt.Sprintf("%d", stat.Value), l.Stats.X+86, y, colors.text, textScale*0.84)
				drawMobileText(frame, fmt.Sprintf("+%d", stat.Bonus), l.Stats.X+154, y, colors.muted, textScale*0.78)
			}
		}

		if mobileSectionVisible(l.Combat, l.ContentViewport, l.Stacked) {
			drawMobilePanel(frame, l.Combat)
			drawMobileHeader(frame, l.Combat, "COMBAT", colors, textScale)
			combatRows := []string{
				fmt.Sprintf("Attack       %d + %d", model.Attack, model.AttackBonus),
				fmt.Sprintf("Defense      %d + %d", model.Defense, model.DefenseBonus),
				fmt.Sprintf("M.Defense    %d + %d", model.MDefense, model.MDefenseBonus),
				fmt.Sprintf("Hit          %d", model.Hit),
				fmt.Sprintf("Flee         %d + %d", model.Flee, model.FleeBonus),
				fmt.Sprintf("Critical     %d", model.Critical),
				fmt.Sprintf("ASPD         %d + %d", model.ASPD, model.ASPDBonus),
			}
			for i, row := range combatRows {
				drawMobileText(frame, row, l.Combat.X+16, l.Combat.Y+46+float32(i)*32, colors.text, textScale*0.80)
			}
			drawMobileText(frame, fmt.Sprintf("ZENY %d", model.Zeny), l.Combat.X+16, l.Combat.Bottom()-20, colors.accent, textScale*0.82)
		}
		if mobileSectionVisible(l.SkillsButton, l.ContentViewport, l.Stacked) {
			drawMobileButton(frame, l.SkillsButton, "SKILLS", colors, textScale*0.80, true)
		}
		return
	}

	l, model := c.SkillsLayout, c.Skills
	drawMobilePanel(frame, l.Panel)
	drawMobileHeaderWithBack(frame, l.Header, l.BackButton, "SKILLS", colors, textScale, "BACK")
	drawMobileButton(frame, l.CharacterButton, "CHARACTER", colors, textScale*0.72, false)
	drawMobileButton(frame, l.Points, fmt.Sprintf("SKILL POINTS %d", model.Points), colors, textScale*0.76, false)
	drawMobilePanel(frame, l.ListViewport)
	for i, row := range l.Rows {
		if i >= len(model.Skills) || !row.Intersects(l.ListViewport) {
			continue
		}
		skill := model.Skills[i]
		selected := model.Selection.HasSelection && model.Selection.SelectedIndex == i
		drawMobileButton(frame, row, "", colors, textScale*0.78, selected)
		iconSize := minf32(42, row.H-12)
		iconX, iconY := row.X+10, row.Y+(row.H-iconSize)/2
		render.DrawRect(frame, float64(iconX), float64(iconY), float64(iconSize), float64(iconSize), colors.header)
		fallback := "?"
		if skill.Name != "" {
			fallback = strings.ToUpper(string([]rune(skill.Name)[0]))
		}
		w, _ := render.BitmapTextSize(fallback)
		drawMobileText(frame, fallback, iconX+(iconSize-float32(w)*float32(textScale*0.82))/2, iconY+10, colors.title, textScale*0.82)
		if p.game != nil {
			p.game.DrawMobileSkillIcon(frame, skill, int(iconX), int(iconY), int(iconSize))
		}
		drawMobileText(frame, trimText(skill.Name, 24), row.X+64, row.Y+10, colors.text, textScale*0.82)
		drawMobileText(frame, fmt.Sprintf("Lv %d/%d", skill.Level, skill.MaxLevel), row.X+64, row.Bottom()-17, colors.muted, textScale*0.68)
		drawMobileText(frame, fmt.Sprintf("SP %d", skill.SPCost), row.Right()-78, row.Y+10, colors.accent, textScale*0.68)
		if i < len(l.UpgradeButtons) && l.UpgradeButtons[i].W > 0 {
			drawMobileButton(frame, l.UpgradeButtons[i], "+", colors, textScale*0.88, true)
		}
	}
	if l.Detail.W <= 0 || l.Detail.H <= 0 {
		return
	}
	drawMobilePanel(frame, l.Detail)
	drawMobileHeader(frame, l.Detail, "SKILL DETAILS", colors, textScale)
	if !model.Selection.HasSelection {
		drawMobileText(frame, "Select a skill", l.Detail.X+18, l.Detail.Y+54, colors.muted, textScale*0.88)
		return
	}
	skill := model.Selection.Skill
	iconSize := minf32(64, l.Detail.H-68)
	if iconSize < 42 {
		iconSize = 42
	}
	iconX, iconY := l.Detail.X+18, l.Detail.Y+48
	render.DrawRect(frame, float64(iconX), float64(iconY), float64(iconSize), float64(iconSize), colors.header)
	fallback := "?"
	if skill.Name != "" {
		fallback = strings.ToUpper(string([]rune(skill.Name)[0]))
	}
	w, _ := render.BitmapTextSize(fallback)
	drawMobileText(frame, fallback, iconX+(iconSize-float32(w)*float32(textScale*0.90))/2, iconY+14, colors.title, textScale*0.90)
	if p.game != nil {
		p.game.DrawMobileSkillIcon(frame, skill, int(iconX), int(iconY), int(iconSize))
	}
	drawMobileText(frame, trimText(skill.Name, 28), iconX+iconSize+14, l.Detail.Y+58, colors.text, textScale*1.0)
	drawMobileText(frame, fmt.Sprintf("Level %d / %d", skill.Level, skill.MaxLevel), iconX+iconSize+14, l.Detail.Y+88, colors.muted, textScale*0.82)
	drawMobileText(frame, fmt.Sprintf("SP cost %d", skill.SPCost), iconX+iconSize+14, l.Detail.Y+118, colors.muted, textScale*0.82)
	drawMobileText(frame, fmt.Sprintf("Range %d   Target %s", skill.Range, skillTargetText(skill.TargetMode)), l.Detail.X+18, l.Detail.Y+iconSize+82, colors.muted, textScale*0.82)
	descriptionY := l.Detail.Y + iconSize + 114
	descriptionBottom := l.Detail.Bottom() - 12
	if l.HotbarButton.W > 0 {
		descriptionBottom = l.HotbarButton.Y - 10
		drawMobileButton(frame, l.HotbarButton, "ADD TO BAR", colors, textScale*0.72, true)
	}
	if len(skill.Description) > 0 && descriptionBottom > descriptionY {
		lineAdvance := float32(22)
		maxLines := int((descriptionBottom - descriptionY) / lineAdvance)
		drawMobileRichWrappedTextLimited(frame, mobileDescriptionText(skill.Description), l.Detail.X+18, descriptionY, l.Detail.W-36, lineAdvance, maxLines, colors.muted, textScale*0.68)
	}
}

func (p *mobilePresentation) drawInventory(frame *render.Frame) {
	c := p.inventory
	l := c.Layout
	colors := mobileColors()
	textScale := p.mobileTextScale()
	p.dumpInventoryCommands()
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 35, G: 62, B: 88, A: 150})
	actionWidth := float32(0)
	if c.State.Screen == mobileui.ScreenInventory {
		actionWidth = l.EquipmentButton.W
	}
	header := mobileui.LayoutMobileScreenHeader(l.Header, l.BackButton.W, actionWidth)
	drawMobilePanel(frame, header.Panel)
	drawMobileButton(frame, header.Back, "< BACK", colors, textScale*0.82, false)
	title := map[mobileui.Screen]string{mobileui.ScreenInventory: "INVENTORY", mobileui.ScreenEquipment: "EQUIPMENT"}[c.State.Screen]
	drawMobileTextFit(frame, title, header.Title.X, header.Title.Y, header.Title.W, colors.title, textScale*1.20)
	if c.State.Screen == mobileui.ScreenInventory {
		for _, tab := range l.Tabs {
			drawMobileButton(frame, tab.Rect, strings.ToUpper(tab.Category.String()), colors, textScale*0.88, tab.Category == c.State.Category)
		}
		if l.EquipmentButton.W > 0 {
			drawMobileButton(frame, header.Action, "EQUIP", colors, textScale*0.80, false)
		}
		drawMobileWorkspace(frame, l.GridViewport)
		for _, empty := range inventoryGridEmptySlots(l, c.State.Scroll.Offset) {
			drawMobileEmptySlot(frame, empty, colors)
		}
		for _, cell := range l.Cells {
			item, ok := itemByIndex(c.Model.Items, cell.Index)
			if !ok {
				continue
			}
			selected := c.State.Selection.SelectedIndex == item.Index
			drawMobileCard(frame, cell.Rect, colors, selected)
			if selected {
				drawMobileSelectionOutline(frame, cell.Rect, colors)
			}
			if l.GridColumns > 1 {
				iconSize := minf32(96, maxf32(48, cell.Rect.W*0.54))
				iconX := cell.Rect.X + (cell.Rect.W-iconSize)/2
				if p.game != nil {
					p.game.DrawMobileInventoryItemIcon(frame, item, int(iconX), int(cell.Rect.Y+12), int(iconSize))
				}
				centerMobileText(frame, trimText(itemLabel(item), 14), cell.Rect.X+8, cell.Rect.Y+iconSize+20, cell.Rect.W-16, colors.text, textScale*0.82)
				centerMobileText(frame, fmt.Sprintf("x%d", item.Quantity), cell.Rect.X+8, cell.Rect.Bottom()-30, cell.Rect.W-16, colors.accent, textScale*0.78)
			} else {
				if p.game != nil {
					p.game.DrawMobileInventoryItemIcon(frame, item, int(cell.Rect.X)+10, int(cell.Rect.Y)+10, 48)
				}
				drawMobileTextFit(frame, trimText(itemLabel(item), 12), cell.Rect.X+66, cell.Rect.Y+18, cell.Rect.W-74, colors.text, textScale*0.74)
				drawMobileText(frame, fmt.Sprintf("x%d", item.Quantity), cell.Rect.X+10, cell.Rect.Bottom()-25, colors.accent, textScale*0.74)
			}
			if item.Equipped {
				drawMobileTextFit(frame, "E", cell.Rect.Right()-30, cell.Rect.Y+10, 20, colors.good, textScale*0.86)
			}
		}
		if l.GridViewport.H >= 100 {
			footer := mobileui.Rect{X: l.GridViewport.X + 12, Y: l.GridViewport.Bottom() - 58, W: l.GridViewport.W - 24, H: 44}
			render.DrawRect(frame, float64(footer.X), float64(footer.Y), float64(footer.W), float64(footer.H), color.RGBA{R: 225, G: 237, B: 247, A: 208})
			drawMobileTextFit(frame, fmt.Sprintf("WEIGHT %d / %d", c.Model.Weight/10, c.Model.MaxWeight/10), footer.X+12, footer.Y+12, footer.W*0.45, colors.title, textScale*0.72)
			drawMobileTextFit(frame, fmt.Sprintf("ZENY %d", c.Model.Zeny), footer.X+footer.W*0.54, footer.Y+12, footer.W*0.42, colors.accent, textScale*0.72)
		}
	} else {
		if l.PaperDoll.W > 0 {
			drawMobileWorkspace(frame, l.PaperDoll)
			drawMobileHeader(frame, l.PaperDoll, "PAPER DOLL", colors, textScale)
			preview := mobileui.EquipmentPreviewRect(l)
			if p.game != nil {
				p.game.DrawMobileEquipmentPreview(frame, int(preview.X), int(preview.Y), int(preview.W), int(preview.H))
			}
			for _, slot := range mobileui.EquipmentPresentationSlots(l) {
				equipment, equipped := equipmentSlotByLocation(c.Equipment, slot.Location)
				selected := equipped && c.State.Selection.HasSelection && c.State.Selection.SelectedIndex == equipment.ItemIndex
				drawMobileEquipmentSlot(frame, slot, equipment, equipped, selected, p.game, colors, textScale)
			}
		}
	}
	if c.State.Screen == mobileui.ScreenEquipment && !c.State.Selection.HasSelection && l.DetailPanel.W > 0 {
		drawMobileEquipmentSummary(frame, l.DetailPanel, c.Equipment, p.game, colors, textScale)
	}
	if c.State.Selection.HasSelection {
		d := c.State.Selection.Detail.Item
		if l.DetailPanel.W > 0 {
			drawMobilePanel(frame, l.DetailPanel)
			drawMobileHeader(frame, l.DetailPanel, "ITEM DETAILS", colors, textScale)
			if p.game != nil {
				p.game.DrawMobileInventoryItemIcon(frame, d, int(l.DetailIcon.X), int(l.DetailIcon.Y), int(l.DetailIcon.W))
			}
			drawMobileTextBoxFit(frame, trimText(itemLabel(d), 30), l.DetailTitle, colors.text, textScale*0.86)
			drawMobileTextBoxFit(frame, fmt.Sprintf("QUANTITY %d   REFINE +%d", d.Quantity, d.Refine), l.DetailMeta, colors.muted, textScale*0.68)
			if len(d.Description) > 0 && l.DetailDescription.H > 0 {
				descriptionScale := textScale * 0.68
				lineAdvance := maxf32(22, float32(24*descriptionScale))
				maxLines := int(l.DetailDescription.H / lineAdvance)
				if maxLines < 1 {
					maxLines = 1
				}
				if maxLines > 4 {
					maxLines = 4
				}
				drawMobileRichWrappedTextLimited(frame, mobileDescriptionText(d.Description), l.DetailDescription.X, l.DetailDescription.Y, l.DetailDescription.W, lineAdvance, maxLines, colors.muted, descriptionScale)
			}
		}
		if l.PrimaryAction.W > 0 {
			drawMobileButton(frame, l.PrimaryAction, strings.ToUpper(c.State.Selection.Detail.PrimaryAction), colors, textScale*0.82, c.State.Selection.Detail.PrimaryEnabled)
		}
		if l.ShortcutAction.W > 0 && d.Usable {
			drawMobileButton(frame, l.ShortcutAction, "ADD TO BAR", colors, textScale*0.72, true)
		}
		if l.SecondaryAction.W > 0 {
			drawMobileButton(frame, l.SecondaryAction, "DROP", colors, textScale*0.82, c.State.Selection.Detail.SecondaryEnabled)
		}
	}
	if c.State.Quantity.Open {
		render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 35, G: 62, B: 88, A: 110})
		drawMobilePanel(frame, l.QuantityModal)
		drawMobileHeader(frame, l.QuantityModal, "DROP QUANTITY", colors, textScale)
		drawMobileText(frame, fmt.Sprintf("How many?   %d", c.State.Quantity.Value), l.QuantityModal.X+18, l.QuantityModal.Y+53, colors.text, textScale*1.0)
		drawMobileButton(frame, l.QuantityMinus, "−", colors, textScale*1.15, false)
		drawMobileButton(frame, l.QuantityPlus, "+", colors, textScale*1.15, false)
		drawMobileButton(frame, l.QuantityConfirm, "CONFIRM", colors, textScale*0.76, true)
		drawMobileButton(frame, l.QuantityCancel, "CANCEL", colors, textScale*0.76, false)
	}
}

func drawMobileEquipmentSlot(frame *render.Frame, slot mobileui.EquipmentSlotRect, equipment mobileui.EquipmentSlotModel, equipped, selected bool, game *app.Game, colors mobilePalette, scale float64) {
	if slot.Rect.W <= 0 || slot.Rect.H <= 0 {
		return
	}
	drawMobileCard(frame, slot.Rect, colors, equipped)
	if selected {
		drawMobileSelectionOutline(frame, slot.Rect, colors)
	}
	if !equipped {
		drawMobileTextFit(frame, trimText(slot.Label, 16), slot.Rect.X+8, slot.Rect.Y+(slot.Rect.H-16)/2, slot.Rect.W-16, colors.muted, scale*0.58)
		return
	}
	if slot.Rect.W < 150 {
		iconSize := minf32(50, maxf32(34, slot.Rect.H-20))
		if game != nil {
			game.DrawMobileInventoryItemIcon(frame, equipment.Item, int(slot.Rect.X+(slot.Rect.W-iconSize)/2), int(slot.Rect.Y+5), int(iconSize))
		}
		name := trimText(itemLabel(equipment.Item), 11)
		if equipment.Item.Refine > 0 {
			name = fmt.Sprintf("+%d %s", equipment.Item.Refine, name)
		}
		centerMobileText(frame, name, slot.Rect.X+4, slot.Rect.Bottom()-18, slot.Rect.W-8, colors.text, scale*0.52)
		return
	}
	iconSize := minf32(72, maxf32(48, slot.Rect.H*0.46))
	if game != nil {
		game.DrawMobileInventoryItemIcon(frame, equipment.Item, int(slot.Rect.X+8), int(slot.Rect.Y+(slot.Rect.H-iconSize)/2), int(iconSize))
	}
	name := trimText(itemLabel(equipment.Item), 14)
	if equipment.Item.Refine > 0 {
		name = fmt.Sprintf("+%d %s", equipment.Item.Refine, name)
	}
	drawMobileTextFit(frame, name, slot.Rect.X+iconSize+16, slot.Rect.Y+slot.Rect.H*0.34, slot.Rect.W-iconSize-24, colors.text, scale*0.66)
	drawMobileTextFit(frame, "EQUIPPED", slot.Rect.X+iconSize+16, slot.Rect.Y+slot.Rect.H*0.61, slot.Rect.W-iconSize-24, colors.good, scale*0.48)
}

func drawMobileEquipmentSummary(frame *render.Frame, panel mobileui.Rect, model mobileui.MobileEquipmentModel, game *app.Game, colors mobilePalette, scale float64) {
	drawMobilePanel(frame, panel)
	drawMobileHeader(frame, panel, "EQUIPMENT SUMMARY", colors, scale)
	drawMobileTextFit(frame, "Tap a slot to inspect equipped gear.", panel.X+16, panel.Y+43, panel.W-32, colors.muted, scale*0.58)
	count := 0
	for _, slot := range model.Slots {
		if slot.HasItem {
			count++
		}
	}
	rowY := panel.Y + 68
	rowH := minf32(64, maxf32(52, panel.W*0.12))
	footerY := panel.Bottom() - 22
	for _, slot := range model.Slots {
		if !slot.HasItem || rowY+rowH > footerY-8 {
			continue
		}
		row := mobileui.Rect{X: panel.X + 12, Y: rowY, W: panel.W - 24, H: rowH}
		drawMobileCard(frame, row, colors, false)
		iconSize := minf32(48, maxf32(34, row.H-12))
		if game != nil {
			game.DrawMobileInventoryItemIcon(frame, slot.Item, int(row.X+8), int(row.Y+(row.H-iconSize)/2), int(iconSize))
		}
		drawMobileTextFit(frame, slot.Label, row.X+iconSize+18, row.Y+row.H*0.30, row.W-iconSize-28, colors.muted, scale*0.54)
		drawMobileTextFit(frame, itemLabel(slot.Item), row.X+iconSize+18, row.Y+row.H*0.60, row.W-iconSize-28, colors.text, scale*0.62)
		rowY += rowH + 8
	}
	if count == 0 {
		drawMobileTextFit(frame, "No equipment equipped.", panel.X+16, panel.Y+76, panel.W-32, colors.muted, scale*0.62)
	}
	drawMobileTextFit(frame, fmt.Sprintf("%d / %d slots equipped", count, len(model.Slots)), panel.X+16, footerY, panel.W-32, colors.accent, scale*0.56)
}

// phase1PCommandDump is a temporary, opt-in diagnostic. It is enabled only
// while collecting Phase 1P command evidence and remains disabled in the
// final APK so normal rendering does not produce logcat noise.
const phase1PCommandDump = false

func (p *mobilePresentation) dumpInventoryCommands() {
	if p == nil || !phase1PCommandDump || p.inventory == nil {
		return
	}
	c := p.inventory
	l := c.Layout
	key := fmt.Sprintf("%d:%d:%d:%d", c.State.Screen, c.State.Selection.SelectedIndex, len(l.Cells), len(mobileui.EquipmentPresentationSlots(l)))
	if p.commandDumpKey == key {
		return
	}
	p.commandDumpKey = key
	if c.State.Screen == mobileui.ScreenInventory {
		for _, command := range mobileui.InventoryPresentationCommands(l, c.Model, c.State) {
			androidLog(fmt.Sprintf("mobile-ui-command owner=%s rect=%s text=%s image=%s layer=%d", command.Owner, mobileCommandRect(command.Rect), command.Text, command.ImageKey, command.Layer))
		}
		return
	}
	for _, command := range mobileui.EquipmentPresentationCommands(l, c.Equipment, c.State) {
		androidLog(fmt.Sprintf("mobile-ui-command owner=%s rect=%s text=%s image=%s layer=%d", command.Owner, mobileCommandRect(command.Rect), command.Text, command.ImageKey, command.Layer))
	}
}

func inventoryGridEmptySlots(layout mobileui.MobileInventoryLayout, offset float32) []mobileui.Rect {
	if layout.GridViewport.W <= 0 || layout.GridViewport.H <= 0 || layout.GridColumns <= 0 || layout.GridCellWidth <= 0 || layout.GridCellHeight <= 0 {
		return nil
	}
	rows := int((layout.GridViewport.H - layout.GridOuterPadding*2 + layout.GridVerticalGap) / (layout.GridCellHeight + layout.GridVerticalGap))
	if rows < 1 {
		rows = 1
	}
	result := make([]mobileui.Rect, 0, rows*layout.GridColumns)
	for row := 0; row < rows; row++ {
		for column := 0; column < layout.GridColumns; column++ {
			rect := mobileui.Rect{
				X: layout.GridViewport.X + layout.GridOuterPadding + float32(column)*(layout.GridCellWidth+layout.GridHorizontalGap),
				Y: layout.GridViewport.Y + layout.GridOuterPadding + float32(row)*(layout.GridCellHeight+layout.GridVerticalGap) - offset,
				W: layout.GridCellWidth,
				H: layout.GridCellHeight,
			}
			if rect.Bottom() <= layout.GridViewport.Y || rect.Y >= layout.GridViewport.Bottom() {
				continue
			}
			result = append(result, rect)
		}
	}
	return result
}

func drawMobileEmptySlot(frame *render.Frame, rect mobileui.Rect, colors mobilePalette) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	render.DrawRect(frame, float64(rect.X), float64(rect.Y), float64(rect.W), float64(rect.H), colors.emptySlot)
	render.DrawRect(frame, float64(rect.X+2), float64(rect.Y+2), float64(rect.W-4), float64(rect.H-4), color.RGBA{R: 215, G: 230, B: 243, A: 42})
}

func mobileCommandRect(rect mobileui.Rect) string {
	return fmt.Sprintf("x=%.0f,y=%.0f,w=%.0f,h=%.0f", rect.X, rect.Y, rect.W, rect.H)
}

func (p *mobilePresentation) drawMap(frame *render.Frame) {
	c := p.mapController
	if c == nil {
		return
	}
	l := c.Layout
	model := c.Model
	colors := mobileColors()
	textScale := p.mobileTextScale()
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 35, G: 62, B: 88, A: 150})
	drawMobilePanel(frame, l.Header)
	drawMobileButton(frame, l.BackButton, "BACK", colors, textScale*0.88, false)
	drawMobileText(frame, "MAP", l.Header.X+124, l.Header.Y+22, colors.title, textScale*1.18)
	drawMobileText(frame, strings.ToUpper(trimText(model.MapName, 24)), l.Header.Right()-220, l.Header.Y+22, colors.muted, textScale*0.84)

	drawMobilePanel(frame, l.MapViewport)
	mapRect := mobileui.Rect{X: l.MapViewport.X + 12, Y: l.MapViewport.Y + 12, W: l.MapViewport.W - 24, H: l.MapViewport.H - 24}
	render.DrawRect(frame, float64(mapRect.X), float64(mapRect.Y), float64(mapRect.W), float64(mapRect.H), colors.mapBackground)
	if !p.game.DrawMobileMinimap(frame, mapRect) {
		p.drawMinimapTerrain(frame, mapRect)
	}
	for _, warp := range model.Warps {
		markerX, markerY, ok := mapMarkerPosition(mapRect, model.Raster, warp.X, warp.Y)
		if !ok {
			continue
		}
		fill := colors.accent
		if warp.ID == c.State.SelectedWarpID {
			fill = colors.marker
		}
		render.DrawRect(frame, markerX-5, markerY-5, 10, 10, fill)
	}
	drawMobileText(frame, "PLAYER", mapRect.X+12, mapRect.Y+22, colors.marker, textScale*0.70)

	drawMobilePanel(frame, l.WarpPanel)
	drawMobileHeader(frame, l.WarpPanel, "EXITS / DESTINATIONS", colors, textScale)
	if len(model.Warps) == 0 {
		drawMobileText(frame, "No local exits in this content pack.", l.WarpPanel.X+20, l.WarpPanel.Y+82, colors.muted, textScale*0.82)
		drawMobileText(frame, "Walk through the world or rebuild with", l.WarpPanel.X+20, l.WarpPanel.Y+116, colors.muted, textScale*0.72)
		drawMobileText(frame, "a multi-map offline content selection.", l.WarpPanel.X+20, l.WarpPanel.Y+140, colors.muted, textScale*0.72)
	}
	for _, row := range l.WarpRows {
		warp, ok := model.Warp(row.ID)
		if !ok {
			continue
		}
		drawMobileButton(frame, row.Rect, trimText(warp.Name, 30), colors, textScale*0.80, warp.ID == c.State.SelectedWarpID)
		drawMobileText(frame, fmt.Sprintf("%s  %d,%d", trimText(warp.DestinationMap, 12), warp.DestinationX, warp.DestinationY), row.Rect.X+12, row.Rect.Bottom()-19, colors.muted, textScale*0.62)
	}
	if warp, ok := model.Warp(c.State.SelectedWarpID); ok {
		drawMobilePanel(frame, l.DetailPanel)
		drawMobileHeader(frame, l.DetailPanel, "DESTINATION", colors, textScale)
		drawMobileText(frame, trimText(warp.Name, 32), l.DetailPanel.X+16, l.DetailPanel.Y+44, colors.text, textScale*0.98)
		drawMobileText(frame, fmt.Sprintf("%s at %d,%d", trimText(warp.DestinationMap, 18), warp.DestinationX, warp.DestinationY), l.DetailPanel.X+16, l.DetailPanel.Y+72, colors.muted, textScale*0.80)
		drawMobileButton(frame, l.TravelButton, "CONFIRM TRAVEL", colors, textScale*0.78, true)
	} else if l.DetailPanel.W > 0 {
		drawMobileText(frame, "Select an exit to see its destination.", l.DetailPanel.X+16, l.DetailPanel.Y+42, colors.muted, textScale*0.78)
	}
	if c.State.ConfirmOpen {
		render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 145})
		drawMobilePanel(frame, l.ConfirmModal)
		drawMobileHeader(frame, l.ConfirmModal, "CONFIRM DESTINATION", colors, textScale)
		if warp, ok := model.Warp(c.State.SelectedWarpID); ok {
			drawMobileText(frame, fmt.Sprintf("Travel to %s?", trimText(warp.DestinationMap, 24)), l.ConfirmModal.X+20, l.ConfirmModal.Y+72, colors.text, textScale*1.02)
			drawMobileText(frame, fmt.Sprintf("The player will walk to %s.", trimText(warp.Name, 26)), l.ConfirmModal.X+20, l.ConfirmModal.Y+102, colors.muted, textScale*0.78)
		}
		drawMobileButton(frame, l.ConfirmButton, "CONFIRM", colors, textScale*0.78, true)
		drawMobileButton(frame, l.ConfirmCancel, "CANCEL", colors, textScale*0.78, false)
	}
}

func (p *mobilePresentation) drawSurface(frame *render.Frame, controller *mobileui.SurfaceController) {
	if controller == nil {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	l := controller.Layout
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 35, G: 62, B: 88, A: 150})
	drawMobilePanel(frame, l.Panel)
	// Keep the title in the header region to the right of BACK. Drawing it
	// across the full header lets the back button cover the first letters on
	// wide landscape screens.
	drawMobileHeader(frame, l.Header, "", colors, textScale)
	titleRect := l.Header
	titleRect.X = l.Back.Right() + 16
	titleRect.W = maxf32(0, l.Header.Right()-titleRect.X)
	drawMobileTextFit(frame, controller.Model.Title, titleRect.X, titleRect.Y+7, titleRect.W, colors.title, textScale*0.98)
	drawMobileButton(frame, l.Back, "BACK", colors, textScale*0.82, false)
	drawMobilePanel(frame, l.ListViewport)
	for i, row := range l.Rows {
		if i >= len(l.RowIDs) {
			continue
		}
		item, ok := surfaceItem(controller.Model, l.RowIDs[i])
		if !ok {
			continue
		}
		labelH := mobileui.SurfaceRowLabelHeight()
		if item.Kind == mobileui.SurfaceItemSection {
			drawMobileText(frame, item.Label, row.X+12, row.Y+28, colors.accent, textScale*0.68)
			if item.Detail != "" && row.H > labelH {
				drawMobileWrappedTextLimited(
					frame, item.Detail,
					row.X+12, row.Y+labelH+4,
					int(maxf32(20, (row.W-24)/float32(11*textScale))),
					int(22*textScale), 1,
					colors.muted, textScale*0.62,
				)
			}
			render.DrawLine(frame, float64(row.X), float64(row.Bottom()-2), float64(row.Right()), float64(row.Bottom()-2), colors.border)
			continue
		}

		selected := controller.State.SelectedID == item.ID
		// Paint the desktop-like row chrome first, then lay out the label/value
		// line and explanatory text explicitly. drawMobileButton's centered
		// label would otherwise waste the detail space the layout reserves.
		drawMobileButton(frame, row, "", colors, textScale*0.80, selected)
		valueW := float32(148)
		labelW := maxf32(40, row.W-36-valueW)
		drawMobileTextFit(frame, trimText(item.Label, 34), row.X+12, row.Y+20, labelW, colors.text, textScale*0.76)
		if item.Value != "" {
			drawMobileTextFit(frame, item.Value, row.Right()-valueW, row.Y+20, valueW-12, colors.muted, textScale*0.70)
		}
		if item.Detail != "" && row.H > labelH {
			drawMobileWrappedTextLimited(
				frame, item.Detail,
				row.X+12, row.Y+labelH+4,
				int(maxf32(20, (row.W-24)/float32(11*textScale))),
				int(22*textScale), 2,
				colors.muted, textScale*0.62,
			)
		}
	}
	if controller.Model.Notice != "" {
		drawMobileTextFit(frame, controller.Model.Notice, l.Notice.X, l.Notice.Y+16, l.Notice.W, colors.muted, textScale*0.64)
	}
	if item, ok := controller.Selected(); ok && l.DetailSheet.W > 0 {
		drawMobilePanel(frame, l.DetailSheet)
		drawMobileHeader(frame, l.DetailSheet, strings.ToUpper(item.Label), colors, textScale)
		message := item.Detail
		if message == "" {
			message = "Select this section to view its mobile controls."
		}
		drawMobileWrappedText(frame, message, l.DetailSheet.X+16, l.DetailSheet.Y+58, int(maxf32(20, l.DetailSheet.W/float32(11*textScale))), int(24*textScale), colors.text, textScale*0.78)
	}
}

func (p *mobilePresentation) drawSocial(frame *render.Frame) {
	c := p.socialController
	if c == nil || !c.IsOpen() {
		return
	}
	l := c.Layout
	colors := mobileColors()
	textScale := p.mobileTextScale()
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 35, G: 62, B: 88, A: 150})
	drawMobilePanel(frame, l.Panel)
	drawMobileHeaderWithBack(frame, l.Header, l.BackButton, "SOCIAL", colors, textScale, "BACK")
	drawMobileButton(frame, l.FriendsTab, "FRIENDS", colors, textScale*0.80, c.State.Tab == mobileui.SocialTabFriends)
	drawMobileButton(frame, l.PartyTab, "PARTY", colors, textScale*0.80, c.State.Tab == mobileui.SocialTabParty)
	drawMobilePanel(frame, l.ListViewport)
	if c.State.Tab == mobileui.SocialTabFriends {
		for _, row := range l.Rows {
			if row.Index < 0 || row.Index >= len(c.Model.Friends) {
				continue
			}
			friend := c.Model.Friends[row.Index]
			selected := false
			if selectedFriend, ok := c.SelectedFriend(); ok {
				selected = selectedFriend.AccountID == friend.AccountID && selectedFriend.CharID == friend.CharID
			}
			drawMobileButton(frame, row.Rect, "", colors, textScale*0.72, selected)
			drawMobileText(frame, trimText(friend.Name, 34), row.Rect.X+14, row.Rect.Y+14, colors.text, textScale*0.92)
			statusColor := colors.muted
			if friend.Online {
				statusColor = colors.good
			}
			drawMobileText(frame, strings.ToUpper(friend.Status), row.Rect.Right()-122, row.Rect.Y+16, statusColor, textScale*0.70)
			drawMobileText(frame, fmt.Sprintf("AID %d", friend.AccountID), row.Rect.X+14, row.Rect.Bottom()-19, colors.muted, textScale*0.66)
		}
		if len(c.Model.Friends) == 0 {
			drawMobileText(frame, "NO FRIENDS", l.ListViewport.X+18, l.ListViewport.Y+82, colors.muted, textScale*0.86)
		}
	} else {
		for _, row := range l.Rows {
			if row.Index < 0 || row.Index >= len(c.Model.PartyMembers) {
				continue
			}
			member := c.Model.PartyMembers[row.Index]
			selected := false
			if selectedMember, ok := c.SelectedPartyMember(); ok {
				selected = selectedMember.AccountID == member.AccountID
			}
			drawMobileButton(frame, row.Rect, "", colors, textScale*0.72, selected)
			name := trimText(member.Name, 30)
			if member.Leader {
				name += " *"
			}
			drawMobileText(frame, name, row.Rect.X+14, row.Rect.Y+12, colors.text, textScale*0.88)
			status := member.MapName
			if !member.Online {
				status = "OFFLINE"
			}
			drawMobileText(frame, strings.ToUpper(trimText(status, 18)), row.Rect.Right()-154, row.Rect.Y+14, map[bool]color.RGBA{true: colors.good, false: colors.muted}[member.Online], textScale*0.66)
			if member.MaxHP > 0 {
				drawMobileBar(frame, "HP", row.Rect.X+14, row.Rect.Bottom()-24, row.Rect.W-28, 10, member.HP, member.MaxHP, colors.hp, colors, textScale*0.72)
			}
		}
		if !c.Model.PartyActive {
			drawMobileText(frame, "NO PARTY", l.ListViewport.X+18, l.ListViewport.Y+82, colors.muted, textScale*0.86)
		} else if len(c.Model.PartyMembers) == 0 {
			drawMobileText(frame, "NO PARTY MEMBERS", l.ListViewport.X+18, l.ListViewport.Y+82, colors.muted, textScale*0.86)
		}
	}
	if c.Model.Notice != "" {
		drawMobileText(frame, c.Model.Notice, l.Notice.X, l.Notice.Y, colors.muted, textScale*0.62)
	}

	drawMobilePanel(frame, l.DetailPanel)
	drawMobileHeader(frame, l.DetailPanel, "DETAILS", colors, textScale)
	if c.State.Tab == mobileui.SocialTabFriends {
		if friend, ok := c.SelectedFriend(); ok {
			drawMobileText(frame, trimText(friend.Name, 30), l.DetailPanel.X+18, l.DetailPanel.Y+58, colors.text, textScale*1.04)
			drawMobileText(frame, strings.ToUpper(friend.Status), l.DetailPanel.X+18, l.DetailPanel.Y+88, colors.muted, textScale*0.78)
			drawMobileText(frame, fmt.Sprintf("Account %d   Character %d", friend.AccountID, friend.CharID), l.DetailPanel.X+18, l.DetailPanel.Y+116, colors.muted, textScale*0.70)
		} else {
			drawMobileText(frame, "Select a friend", l.DetailPanel.X+18, l.DetailPanel.Y+62, colors.muted, textScale*0.88)
		}
		if l.PrimaryAction.W > 0 {
			drawMobileButton(frame, l.PrimaryAction, "WHISPER", colors, textScale*0.74, c.SelectedFriendCanWhisper())
		}
		if l.SecondaryAction.W > 0 {
			drawMobileButton(frame, l.SecondaryAction, "DELETE", colors, textScale*0.74, c.SelectedFriendCanDelete())
		}
	} else {
		partyTitle := c.Model.PartyName
		if partyTitle == "" {
			partyTitle = "Party"
		}
		drawMobileText(frame, trimText(partyTitle, 30), l.DetailPanel.X+18, l.DetailPanel.Y+58, colors.text, textScale*1.04)
		if member, ok := c.SelectedPartyMember(); ok {
			state := "OFFLINE"
			if member.Online {
				state = strings.ToUpper(member.MapName)
			}
			drawMobileText(frame, trimText(member.Name, 28), l.DetailPanel.X+18, l.DetailPanel.Y+88, colors.title, textScale*0.86)
			drawMobileText(frame, trimText(state, 22), l.DetailPanel.X+18, l.DetailPanel.Y+114, colors.muted, textScale*0.72)
		} else if !c.Model.PartyActive {
			drawMobileText(frame, "Set a party name to create a party.", l.DetailPanel.X+18, l.DetailPanel.Y+90, colors.muted, textScale*0.76)
		} else {
			drawMobileText(frame, "Select a member or invite a player.", l.DetailPanel.X+18, l.DetailPanel.Y+90, colors.muted, textScale*0.76)
		}
		if l.PrimaryAction.W > 0 {
			label := "CREATE PARTY"
			active := c.Model.CanCreateParty
			if c.Model.PartyActive {
				if c.State.HasSelection {
					label = "WHISPER"
					active = c.SelectedPartyCanWhisper()
				} else {
					label = "INVITE"
					active = c.Model.CanInvite
				}
			}
			drawMobileButton(frame, l.PrimaryAction, label, colors, textScale*0.70, active)
		}
		if l.SecondaryAction.W > 0 && c.Model.PartyActive && c.State.HasSelection {
			drawMobileButton(frame, l.SecondaryAction, "EXPEL", colors, textScale*0.74, c.Model.CanExpel && !c.SelectedPartyIsSelf())
		}
		if l.TertiaryAction.W > 0 {
			drawMobileButton(frame, l.TertiaryAction, "LEAVE PARTY", colors, textScale*0.74, c.Model.CanLeave)
		}
		if l.SettingsAction.W > 0 {
			drawMobileButton(frame, l.SettingsAction, "PARTY SETTINGS", colors, textScale*0.70, c.Model.PartySettings.CanEdit)
		}
	}
	if request := c.VisibleFriendRequest(); request != nil {
		drawMobilePanel(frame, l.RequestModal)
		drawMobileHeader(frame, l.RequestModal, "FRIEND REQUEST", colors, textScale)
		drawMobileWrappedText(frame, fmt.Sprintf("%s wants to be friends with you.", trimText(request.Name, 42)), l.RequestModal.X+24, l.RequestModal.Y+86, int(maxf32(20, l.RequestModal.W/float32(11*textScale))), int(28*textScale), colors.text, textScale*0.92)
		drawMobileButton(frame, l.RequestAccept, "ACCEPT", colors, textScale*0.78, true)
		drawMobileButton(frame, l.RequestDecline, "DECLINE", colors, textScale*0.78, false)
	} else if invite := c.VisiblePartyInvite(); invite != nil {
		drawMobilePanel(frame, l.RequestModal)
		drawMobileHeader(frame, l.RequestModal, "PARTY INVITATION", colors, textScale)
		drawMobileWrappedText(frame, fmt.Sprintf("%s invited you to join a party.", trimText(invite.Name, 42)), l.RequestModal.X+24, l.RequestModal.Y+86, int(maxf32(20, l.RequestModal.W/float32(11*textScale))), int(28*textScale), colors.text, textScale*0.92)
		drawMobileButton(frame, l.RequestAccept, "ACCEPT", colors, textScale*0.78, true)
		drawMobileButton(frame, l.RequestDecline, "DECLINE", colors, textScale*0.78, false)
	}
	if c.State.SettingsOpen {
		drawMobilePanel(frame, l.SettingsModal)
		drawMobileHeader(frame, l.SettingsModal, "PARTY SETTINGS", colors, textScale)
		drawMobileText(frame, "EXP SHARE", l.SettingsModal.X+24, l.SettingsModal.Y+84, colors.text, textScale*0.86)
		drawMobileButton(frame, l.SettingsEach, "EACH TAKE", colors, textScale*0.72, c.State.SettingsExpShare == 0)
		drawMobileButton(frame, l.SettingsEven, "EVEN SHARE", colors, textScale*0.72, c.State.SettingsExpShare == 1)
		drawMobileButton(frame, l.SettingsRefuse, "REFUSE PARTY INVITES", colors, textScale*0.72, c.State.SettingsRefuseInvites)
		drawMobileButton(frame, l.SettingsConfirm, "APPLY", colors, textScale*0.78, true)
		drawMobileButton(frame, l.SettingsCancel, "CANCEL", colors, textScale*0.78, false)
	}
	if c.TextInputActive() {
		drawMobilePanel(frame, l.TextInputModal)
		drawMobileHeader(frame, l.TextInputModal, c.TextInputTitle(), colors, textScale)
		drawMobileText(frame, c.TextInputHint(), l.TextInputModal.X+24, l.TextInputModal.Y+88, colors.muted, textScale*0.82)
		drawMobilePanel(frame, mobileui.Rect{X: l.TextInputModal.X + 24, Y: l.TextInputModal.Y + 122, W: l.TextInputModal.W - 48, H: 64})
		draft := c.TextInputDraft()
		if draft == "" {
			draft = "TYPE USING THE NATIVE KEYBOARD"
		}
		drawMobileText(frame, trimText(draft, 48), l.TextInputModal.X+42, l.TextInputModal.Y+144, colors.text, textScale*0.82)
		drawMobileButton(frame, l.TextInputConfirm, "SEND", colors, textScale*0.78, c.TextInputDraft() != "")
		drawMobileButton(frame, l.TextInputCancel, "CANCEL", colors, textScale*0.78, false)
	}
}

func (p *mobilePresentation) drawTrade(frame *render.Frame) {
	c := p.tradeController
	if c == nil || !c.IsOpen() {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	l := c.Layout
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 150})
	drawMobilePanel(frame, l.Panel)
	drawMobileHeaderWithBack(frame, l.Header, l.BackButton, "TRADE WITH "+trimText(c.Model.PartnerName, 36), colors, textScale, "BACK")
	if request := c.Model.PendingRequest; request != nil {
		drawMobilePanel(frame, l.RequestModal)
		drawMobileHeader(frame, l.RequestModal, "TRADE REQUEST", colors, textScale)
		message := fmt.Sprintf("%s wants to trade with you.", trimText(request.Name, 42))
		if request.Level > 0 {
			message += fmt.Sprintf("  Level %d", request.Level)
		}
		drawMobileWrappedText(frame, message, l.RequestModal.X+24, l.RequestModal.Y+88, int(maxf32(20, l.RequestModal.W/float32(11*textScale))), int(28*textScale), colors.text, textScale*0.92)
		drawMobileButton(frame, l.RequestAccept, "ACCEPT", colors, textScale*0.78, true)
		drawMobileButton(frame, l.RequestDecline, "DECLINE", colors, textScale*0.78, false)
		return
	}

	drawMobilePanel(frame, l.InventoryPanel)
	drawMobileHeader(frame, l.InventoryPanel, "MY INVENTORY", colors, textScale)
	for _, row := range l.InventoryRows {
		if row.Index < 0 || row.Index >= len(c.Model.Inventory) {
			continue
		}
		item := c.Model.Inventory[row.Index]
		drawMobileCard(frame, row.Rect, colors, item.Offered)
		if p.game != nil {
			p.game.DrawMobileInventoryItemIcon(frame, item.Item, int(row.Rect.X+8), int(row.Rect.Y+8), int(minf32(46, row.Rect.H-16)))
		}
		name := trimText(item.Item.DisplayName, 28)
		if name == "" {
			name = fmt.Sprintf("Item %d", item.Item.ItemID)
		}
		drawMobileTextFit(frame, name, row.Rect.X+64, row.Rect.Y+10, row.Rect.W-170, colors.text, textScale*0.82)
		drawMobileTextFit(frame, fmt.Sprintf("x%d", item.Item.Quantity), row.Rect.Right()-98, row.Rect.Y+10, 84, colors.muted, textScale*0.70)
		status := "TAP TO OFFER"
		active := item.CanAdd
		if item.Offered {
			status, active = "OFFERED", true
		} else if item.Pending {
			status, active = "WAITING", false
		}
		drawMobileTextFit(frame, status, row.Rect.X+64, row.Rect.Bottom()-22, row.Rect.W-76, map[bool]color.RGBA{true: colors.good, false: colors.muted}[active], textScale*0.62)
	}
	if c.Model.Notice != "" {
		drawMobileText(frame, c.Model.Notice, l.Notice.X, l.Notice.Y, colors.muted, textScale*0.62)
	}

	drawMobileTradePanel(frame, l.OwnPanel, "MY OFFER", c.Model.OwnOffer, c.Model.OwnZeny, colors, textScale, p.game)
	drawMobileTradePanel(frame, l.PartnerPanel, trimText(c.Model.PartnerName, 28), c.Model.PartnerOffer, c.Model.PartnerZeny, colors, textScale, p.game)
	drawMobileButton(frame, l.AddZeny, "ADD ZENY", colors, textScale*0.64, c.Model.CanAddZeny)
	drawMobileButton(frame, l.Conclude, "OK", colors, textScale*0.78, c.Model.CanConclude)
	drawMobileButton(frame, l.Commit, "TRADE", colors, textScale*0.68, c.Model.CanCommit)
	drawMobileButton(frame, l.Cancel, "CANCEL", colors, textScale*0.68, c.Model.CanCancel)
	if c.State.Quantity.Open {
		render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 140})
		drawMobilePanel(frame, l.QuantityModal)
		title := "OFFER ITEM"
		if c.State.Quantity.Action == mobileui.TradeQuantityZeny {
			title = "OFFER ZENY"
		}
		drawMobileHeader(frame, l.QuantityModal, title, colors, textScale)
		drawMobileText(frame, fmt.Sprintf("%d / %d", c.State.Quantity.Value, c.State.Quantity.Maximum), l.QuantityModal.X+24, l.QuantityModal.Y+86, colors.text, textScale*1.08)
		drawMobileButton(frame, l.QuantityMinus, "−", colors, textScale*1.15, false)
		drawMobileButton(frame, l.QuantityPlus, "+", colors, textScale*1.15, false)
		drawMobileButton(frame, l.QuantityConfirm, "OFFER", colors, textScale*0.70, true)
		drawMobileButton(frame, l.QuantityCancel, "CANCEL", colors, textScale*0.70, false)
	}
}

func drawMobileTradePanel(frame *render.Frame, rect mobileui.Rect, title string, items []mobileui.TradeOfferItemModel, zeny uint32, colors mobilePalette, scale float64, game *app.Game) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	drawMobilePanel(frame, rect)
	drawMobileHeader(frame, rect, title, colors, scale)
	rowH := minf32(52, maxf32(48, rect.H-86))
	maxRows := int(maxf32(0, rect.H-86) / rowH)
	if maxRows > len(items) {
		maxRows = len(items)
	}
	for i := 0; i < maxRows; i++ {
		row := mobileui.Rect{X: rect.X + 12, Y: rect.Y + 52 + float32(i)*rowH, W: rect.W - 24, H: rowH - 4}
		item := items[i]
		drawMobileCard(frame, row, colors, false)
		if item.IsZeny {
			drawMobileText(frame, "Z", row.X+10, row.Y+10, colors.accent, scale*0.90)
		} else if game != nil {
			game.DrawMobileInventoryItemIcon(frame, mobileui.InventoryItemModel{ItemID: item.ItemID, Identified: item.Identified, IconKey: item.IconKey}, int(row.X+8), int(row.Y+4), int(minf32(42, row.H-8)))
		}
		name := item.Name
		if item.IsZeny {
			name = "Zeny"
		}
		drawMobileTextFit(frame, name, row.X+58, row.Y+9, row.W-130, colors.text, scale*0.74)
		drawMobileTextFit(frame, fmt.Sprintf("x%d", item.Quantity), row.Right()-66, row.Y+9, 56, colors.muted, scale*0.68)
	}
	drawMobileTextFit(frame, fmt.Sprintf("ZENY %d", zeny), rect.X+12, rect.Bottom()-26, rect.W-24, colors.accent, scale*0.70)
}

func (p *mobilePresentation) drawVending(frame *render.Frame) {
	c := p.vendingController
	if c == nil || !c.IsOpen() {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	l := c.Layout
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 150})
	drawMobilePanel(frame, l.Panel)
	drawMobileHeaderWithBack(frame, l.Header, l.Back, "VENDING", colors, textScale, "BACK")
	drawMobileButton(frame, l.Close, "CLOSE", colors, textScale*0.76, false)

	if c.Model.Loading {
		drawMobileText(frame, "Loading player shop…", l.ListViewport.X+16, l.ListViewport.Y+34, colors.muted, textScale*0.90)
	} else if len(c.Model.Items) == 0 {
		drawMobileText(frame, "This shop has no items.", l.ListViewport.X+16, l.ListViewport.Y+34, colors.muted, textScale*0.90)
	} else {
		for rowNumber, row := range l.Rows {
			if rowNumber >= len(l.RowIndices) {
				break
			}
			itemIndex := l.RowIndices[rowNumber]
			if itemIndex < 0 || itemIndex >= len(c.Model.Items) {
				continue
			}
			item := c.Model.Items[itemIndex]
			selected := c.State.HasSelection && c.State.SelectedIndex == itemIndex
			drawMobileButton(frame, row, "", colors, textScale*0.80, selected)
			if p.game != nil {
				p.game.DrawMobileInventoryItemIcon(frame, mobileui.InventoryItemModel{ItemID: item.ItemID, Identified: item.Identified, IconKey: item.IconKey}, int(row.X+10), int(row.Y+8), int(minf32(44, row.H-12)))
			}
			drawMobileTextFit(frame, mobileui.VendingItemName(item), row.X+66, row.Y+10, row.W-190, colors.text, textScale*0.82)
			drawMobileTextFit(frame, fmt.Sprintf("x%d", item.Quantity), row.Right()-118, row.Y+10, 54, colors.muted, textScale*0.72)
			drawMobileTextFit(frame, fmt.Sprintf("%d Z", item.Price), row.Right()-118, row.Y+34, 106, colors.accent, textScale*0.72)
			if !item.CanBuy {
				drawMobileTextFit(frame, trimText(item.DisabledReason, 18), row.X+66, row.Y+36, row.W-190, colors.muted, textScale*0.64)
			}
		}
	}

	drawMobilePanel(frame, l.DetailPanel)
	drawMobileHeader(frame, l.DetailPanel, strings.ToUpper(trimText(c.Model.ShopName, 24)), colors, textScale)
	if c.Model.Notice != "" {
		drawMobileWrappedTextLimited(frame, c.Model.Notice, l.DetailPanel.X+18, l.DetailPanel.Y+54, int(maxf32(18, l.DetailPanel.W/float32(11*textScale))), int(20*textScale), 2, colors.muted, textScale*0.68)
	}
	if item, ok := c.SelectedItem(); ok {
		if p.game != nil {
			p.game.DrawMobileInventoryItemIcon(frame, mobileui.InventoryItemModel{ItemID: item.ItemID, Identified: item.Identified, IconKey: item.IconKey}, int(l.DetailPanel.X+18), int(l.DetailPanel.Y+84), 72)
		}
		drawMobileTextFit(frame, mobileui.VendingItemName(item), l.DetailPanel.X+106, l.DetailPanel.Y+86, l.DetailPanel.W-124, colors.text, textScale*1.00)
		drawMobileTextFit(frame, fmt.Sprintf("Price %d Z   Stock %d", item.Price, item.Quantity), l.DetailPanel.X+106, l.DetailPanel.Y+116, l.DetailPanel.W-124, colors.muted, textScale*0.76)
		drawMobileTextFit(frame, fmt.Sprintf("Your zeny: %d", c.Model.Zeny), l.DetailPanel.X+18, l.DetailPanel.Y+172, l.DetailPanel.W-36, colors.accent, textScale*0.82)
		if item.DisabledReason != "" {
			drawMobileWrappedTextLimited(frame, item.DisabledReason, l.DetailPanel.X+18, l.DetailPanel.Y+206, int(maxf32(18, l.DetailPanel.W/float32(11*textScale))), int(20*textScale), 2, colors.muted, textScale*0.72)
		}
	} else if !c.Model.Loading {
		drawMobileText(frame, "Select an item to buy.", l.DetailPanel.X+18, l.DetailPanel.Y+84, colors.muted, textScale*0.90)
	}
	drawMobileButton(frame, l.BuyButton, "BUY", colors, textScale*0.82, func() bool { item, ok := c.SelectedItem(); return ok && item.CanBuy }())

	if c.Quantity.Open {
		render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 140})
		drawMobilePanel(frame, l.QuantityModal)
		drawMobileHeader(frame, l.QuantityModal, "BUY QUANTITY", colors, textScale)
		drawMobileText(frame, fmt.Sprintf("How many?   %d / %d", c.Quantity.Value, c.Quantity.Maximum), l.QuantityModal.X+20, l.QuantityModal.Y+76, colors.text, textScale*1.08)
		drawMobileButton(frame, l.QuantityMinus, "−", colors, textScale*1.15, false)
		drawMobileButton(frame, l.QuantityPlus, "+", colors, textScale*1.15, false)
		drawMobileButton(frame, l.QuantityConfirm, "BUY", colors, textScale*0.76, true)
		drawMobileButton(frame, l.QuantityCancel, "CANCEL", colors, textScale*0.76, false)
	}
}

func surfaceItem(model mobileui.SurfaceModel, id string) (mobileui.SurfaceItem, bool) {
	for _, item := range model.Items {
		if item.ID == id {
			return item, true
		}
	}
	return mobileui.SurfaceItem{}, false
}

func (p *mobilePresentation) drawChat(frame *render.Frame) {
	c := p.chatController
	if c == nil || !c.Model.Open {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	l := c.Layout
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 150})
	drawMobilePanel(frame, l.Panel)
	chatTitle := c.Model.Channel + " CHAT"
	if c.Model.Recipient != "" {
		chatTitle = "WHISPER: " + trimText(c.Model.Recipient, 22)
	}
	drawMobileHeaderWithBack(frame, l.Header, l.Back, chatTitle, colors, textScale, "BACK")
	drawMobilePanel(frame, l.MessageViewport)
	for i, row := range l.Rows {
		if i >= len(l.RowIndices) || l.RowIndices[i] >= len(c.Model.Messages) {
			continue
		}
		message := c.Model.Messages[l.RowIndices[i]]
		drawMobileText(frame, trimText(message.Sender, 16), row.X+12, row.Y+8, colors.title, textScale*0.70)
		drawMobileWrappedText(frame, message.Text, row.X+12, row.Y+28, int(maxf32(16, row.W/float32(11*textScale))), int(20*textScale), colors.text, textScale*0.68)
	}
	composerLabel := c.Draft
	if composerLabel == "" {
		composerLabel = "CHAT IS AVAILABLE ONLINE"
		if c.Model.CanSend {
			composerLabel = "Tap to type…"
		}
	}
	drawMobileButton(frame, l.Composer, composerLabel, colors, textScale*0.72, false)
	if c.Model.Notice != "" {
		drawMobileText(frame, c.Model.Notice, l.MessageViewport.X+12, l.Composer.Y-18, colors.muted, textScale*0.64)
	}
	drawMobileButton(frame, l.Send, "SEND", colors, textScale*0.76, c.Model.CanSend && c.Draft != "")
}

func mapMarkerPosition(rect mobileui.Rect, raster mobileui.MinimapRaster, x, y int) (float64, float64, bool) {
	if raster.Width <= 0 || raster.Height <= 0 || len(raster.Cells) < raster.Width*raster.Height || rect.W <= 0 || rect.H <= 0 {
		return 0, 0, false
	}
	imageWidth, imageHeight := float64(raster.Width), float64(raster.Height)
	drawWidth, drawHeight := float64(rect.W), float64(rect.H)
	if imageWidth/imageHeight > drawWidth/drawHeight {
		drawHeight = drawWidth * imageHeight / imageWidth
	} else {
		drawWidth = drawHeight * imageWidth / imageHeight
	}
	drawX := float64(rect.X) + (float64(rect.W)-drawWidth)/2
	drawY := float64(rect.Y) + (float64(rect.H)-drawHeight)/2
	mapX := clampMinimapCoordinate(float64(x)*0.5, raster.Width)
	mapY := clampMinimapCoordinate(float64(y)*0.5, raster.Height)
	return drawX + (mapX+0.5)*drawWidth/imageWidth, drawY + drawHeight - (mapY+0.5)*drawHeight/imageHeight, true
}

func (p *mobilePresentation) drawDialog(frame *render.Frame) {
	c := p.dialogController
	if c == nil || !c.Model.Open {
		return
	}
	l := c.Layout
	model := c.Model
	colors := mobileColors()
	textScale := p.mobileTextScale()
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 135})
	drawMobilePanel(frame, l.Panel)
	dialogTitle := strings.ToUpper(trimText(model.Title, 28))
	if len(model.Messages) > 0 {
		for _, message := range model.Messages {
			if speaker := strings.TrimSpace(message.Speaker); speaker != "" {
				// Preserve the script's spelling/casing for identity labels such as
				// [Tine] and [PrivateMvpRoom]. Generic fallback titles retain the
				// existing all-caps mobile chrome convention.
				dialogTitle = trimText("["+speaker+"]", 28)
				break
			}
		}
	}
	drawMobileHeader(frame, l.Header, dialogTitle, colors, textScale)
	messageScale := textScale * 1.08
	lineAdvance := maxf32(24, float32(27*textScale))
	if len(l.MessageBlocks) > 0 {
		for _, block := range l.MessageBlocks {
			if block.SpeakerRect.W > 0 && block.SpeakerRect.H > 0 {
				drawMobileTextBoxFit(frame, "["+block.Speaker+"]", block.SpeakerRect, colors.accent, textScale*0.72)
			}
			maxLines := int(block.TextRect.H / lineAdvance)
			if maxLines < 1 {
				maxLines = 1
			}
			drawMobileRichWrappedTextLimited(frame, block.Text, block.TextRect.X, block.TextRect.Y, block.TextRect.W, lineAdvance, maxLines, colors.text, messageScale)
		}
	} else {
		maxLines := int(l.Message.H / lineAdvance)
		if maxLines < 1 {
			maxLines = 1
		}
		drawMobileRichWrappedTextLimited(frame, model.Message, l.Message.X, l.Message.Y, l.Message.W, lineAdvance, maxLines, colors.text, messageScale)
	}
	if model.Notice != "" && l.Notice.W > 0 {
		drawMobileTextBoxFit(frame, model.Notice, l.Notice, colors.accent, textScale*0.64)
	}
	for i, option := range model.Options {
		if i >= len(l.Options) {
			break
		}
		active := option.Enabled && option.Action != mobileui.DialogClose
		drawMobileButton(frame, l.Options[i], strings.ToUpper(option.Label), colors, textScale*0.76, active)
	}
}

func (p *mobilePresentation) drawEconomy(frame *render.Frame) {
	c := p.economyController
	if c == nil || c.Screen == mobileui.EconomyClosed {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	layout := c.Layout
	if c.Screen == mobileui.EconomyStorage {
		storage := c.Storage
		drawMobilePanel(frame, layout.Panel)
		drawMobileHeaderWithAction(frame, layout.Header, fmt.Sprintf("STORAGE   %d / %d", storage.Amount, storage.MaxAmount), layout.Close, colors, textScale)
		drawMobileButton(frame, layout.Close, "CLOSE", colors, textScale*0.76, false)
		for rowNumber, row := range layout.Rows {
			if rowNumber >= len(layout.RowIndices) {
				break
			}
			itemIndex := layout.RowIndices[rowNumber]
			if itemIndex < 0 || itemIndex >= len(storage.Items) {
				continue
			}
			item := storage.Items[itemIndex]
			drawMobileActionRow(frame, row, fmt.Sprintf("%s   x%d", trimText(itemLabel(item), 26), item.Quantity), "WITHDRAW", colors, textScale*0.86, textScale*0.72, false)
		}
		p.drawEconomyQuantity(frame)
		return
	}
	shop := c.Shop
	items := shop.Items
	if c.Tab == mobileui.ShopSellTab {
		items = shop.SellItems
	}
	drawMobilePanel(frame, layout.Panel)
	drawMobileHeaderWithAction(frame, layout.Header, fmt.Sprintf("%s   ZENY %d", trimText(shop.Name, 28), shop.Zeny), layout.Close, colors, textScale)
	drawMobileButton(frame, layout.Close, "CLOSE", colors, textScale*0.76, false)
	for _, tab := range layout.Tabs {
		drawMobileButton(frame, tab.Rect, strings.ToUpper(tab.Tab.String()), colors, textScale*0.76, tab.Tab == c.Tab)
	}
	if len(items) == 0 {
		label := "NO ITEMS AVAILABLE"
		if c.Tab == mobileui.ShopSellTab {
			label = "NO SELLABLE ITEMS"
		}
		centerMobileText(frame, label, layout.ListViewport.X, layout.ListViewport.Y+24, layout.ListViewport.W, colors.muted, textScale*0.72)
	}
	for rowNumber, row := range layout.Rows {
		if rowNumber >= len(layout.RowIndices) {
			break
		}
		itemIndex := layout.RowIndices[rowNumber]
		if itemIndex < 0 || itemIndex >= len(items) {
			continue
		}
		item := items[itemIndex]
		if c.Tab == mobileui.ShopSellTab {
			drawMobileActionRow(frame, row, fmt.Sprintf("%s   x%d", trimText(item.Name, 28), item.Quantity), fmt.Sprintf("SELL %d", item.SellPrice), colors, textScale*0.86, textScale*0.70, item.CanSell)
			continue
		}
		drawMobileActionRow(frame, row, fmt.Sprintf("%s   %d zeny", trimText(item.Name, 28), item.Price), "BUY", colors, textScale*0.86, textScale*0.78, item.CanBuy)
	}
	if shop.CartEnabled && layout.CartPanel.W > 0 {
		drawMobilePanel(frame, layout.CartPanel)
		title := "BUYING"
		action := "BUY"
		if c.Tab == mobileui.ShopSellTab {
			title, action = "SELLING", "SELL"
		}
		drawMobileTextFit(frame, title, layout.CartPanel.X+10, layout.CartPanel.Y+7, layout.CartPanel.W-20, colors.title, textScale*0.72)
		drawMobileTextFit(frame, "TAP ITEM TO REMOVE", layout.CartPanel.X+10, layout.CartPanel.Y+31, layout.CartPanel.W-20, colors.muted, textScale*0.54)
		for i, row := range layout.CartRows {
			if i >= len(shop.Cart) {
				break
			}
			entry := shop.Cart[i]
			drawMobileActionRow(frame, row, fmt.Sprintf("%s  x%d", trimText(entry.Name, 18), entry.Quantity), fmt.Sprintf("%d z", entry.Total), colors, textScale*0.70, textScale*0.62, true)
		}
		drawMobileTextBoxFit(frame, fmt.Sprintf("SUBTOTAL %d z", shop.CartTotal), layout.CartSubtotal, colors.title, textScale*0.68)
		drawMobileButton(frame, layout.CartConfirm, action, colors, textScale*0.72, len(shop.Cart) > 0)
	}
	p.drawEconomyQuantity(frame)
}

func (p *mobilePresentation) drawEconomyQuantity(frame *render.Frame) {
	c := p.economyController
	if c == nil || !c.Quantity.Open {
		return
	}
	colors := mobileColors()
	textScale := p.mobileTextScale()
	l := c.Layout
	render.DrawRect(frame, float64(l.Safe.X), float64(l.Safe.Y), float64(l.Safe.W), float64(l.Safe.H), color.RGBA{R: 18, G: 38, B: 62, A: 140})
	drawMobilePanel(frame, l.QuantityModal)
	title := "Withdraw"
	switch c.Quantity.Action {
	case mobileui.EconomyQuantityBuy:
		title = "Buy"
	case mobileui.EconomyQuantitySell:
		title = "Sell"
	}
	drawMobileHeaderWithAction(frame, l.QuantityModal, title, l.QuantityCancel, colors, textScale)
	drawMobileButton(frame, l.QuantityCancel, "Cancel", colors, textScale*0.72, false)

	bodyTop := l.QuantityModal.Y + 36
	bodyBottom := l.QuantityMinus.Y
	bodyH := maxf32(0, bodyBottom-bodyTop)
	labelRect := mobileui.Rect{X: l.QuantityModal.X, Y: bodyTop, W: l.QuantityModal.W, H: bodyH * 0.42}
	valueRect := mobileui.Rect{X: l.QuantityModal.X, Y: bodyTop + bodyH*0.42, W: l.QuantityModal.W, H: bodyH * 0.58}
	drawMobileTextCentered(frame, fmt.Sprintf("Max %d", c.Quantity.Maximum), labelRect, colors.muted, textScale*0.72)
	drawMobileTextCentered(frame, fmt.Sprintf("%d", c.Quantity.Value), valueRect, colors.title, textScale*1.05)

	drawMobileButton(frame, l.QuantityMinus, "−", colors, textScale*1.15, false)
	drawMobileButton(frame, l.QuantityPlus, "+", colors, textScale*1.15, false)
	drawMobileButton(frame, l.QuantityMax, "Max", colors, textScale*0.72, false)
	confirmLabel := "Confirm"
	if c.Shop.CartEnabled && (c.Quantity.Action == mobileui.EconomyQuantityBuy || c.Quantity.Action == mobileui.EconomyQuantitySell) {
		confirmLabel = "Add"
	}
	drawMobileButton(frame, l.QuantityConfirm, confirmLabel, colors, textScale*0.72, true)
}

type mobilePalette struct {
	body, workspace, headerTop, header, border, button, buttonActive, card, cardActive, emptySlot, text, title, muted, accent, hp, sp, target, good, shadow, mapBackground, mapPath, marker color.RGBA
}

func mobileColors() mobilePalette {
	// These values intentionally mirror ui/rotheme: light blue title bars,
	// white window bodies, blue borders, and dark readable text. Alpha is kept
	// on the window body so the map remains visible beneath the HUD.
	return mobilePalette{
		body:          color.RGBA{R: 250, G: 252, B: 255, A: 232},
		workspace:     color.RGBA{R: 215, G: 230, B: 243, A: 166},
		headerTop:     color.RGBA{R: 226, G: 240, B: 252, A: 250},
		header:        color.RGBA{R: 184, G: 214, B: 242, A: 250},
		border:        color.RGBA{R: 118, G: 160, B: 206, A: 255},
		button:        color.RGBA{R: 236, G: 244, B: 252, A: 245},
		buttonActive:  color.RGBA{R: 198, G: 222, B: 245, A: 255},
		card:          color.RGBA{R: 239, G: 246, B: 253, A: 235},
		cardActive:    color.RGBA{R: 184, G: 216, B: 247, A: 255},
		emptySlot:     color.RGBA{R: 191, G: 211, B: 229, A: 68},
		text:          color.RGBA{R: 38, G: 48, B: 58, A: 255},
		title:         color.RGBA{R: 22, G: 54, B: 88, A: 255},
		muted:         color.RGBA{R: 98, G: 112, B: 126, A: 255},
		accent:        color.RGBA{R: 56, G: 112, B: 166, A: 255},
		hp:            color.RGBA{R: 33, G: 190, B: 74, A: 255},
		sp:            color.RGBA{R: 55, G: 112, B: 218, A: 255},
		target:        color.RGBA{R: 220, G: 112, B: 54, A: 255},
		good:          color.RGBA{R: 34, G: 140, B: 88, A: 255},
		shadow:        color.RGBA{R: 28, G: 57, B: 84, A: 110},
		mapBackground: color.RGBA{R: 34, G: 64, B: 82, A: 245},
		mapPath:       color.RGBA{R: 119, G: 161, B: 113, A: 255},
		marker:        color.RGBA{R: 255, G: 232, B: 88, A: 255},
	}
}

func (p *mobilePresentation) mobileTextScale() float64 {
	if p == nil || p.viewport.Height <= 0 {
		return 2.50
	}
	// The Android bitmap font is authored at desktop-sized pixels. The Fold
	// outer display is physically wide but only 832 logical pixels tall, so
	// height-based scaling keeps labels readable without making ultrawide
	// panels grow with the unused horizontal space.
	// Reference against the short physical edge, with a floor that keeps Quad
	// HD phone captures readable. Width is deliberately excluded so ultrawide
	// landscape cannot create tiny labels.
	shortEdge := p.viewport.Height
	if p.viewport.Width < shortEdge {
		shortEdge = p.viewport.Width
	}
	scale := float64(shortEdge) / 320
	if scale < 2.50 {
		scale = 2.50
	}
	if scale > 2.75 {
		scale = 2.75
	}
	return scale
}

func drawMobilePanel(frame *render.Frame, rect mobileui.Rect) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	colors := mobileColors()
	render.DrawRect(frame, float64(rect.X+3), float64(rect.Y+4), float64(rect.W), float64(rect.H), colors.shadow)
	render.DrawRect(frame, float64(rect.X), float64(rect.Y), float64(rect.W), float64(rect.H), colors.border)
	render.DrawRect(frame, float64(rect.X+2), float64(rect.Y+2), float64(rect.W-4), float64(rect.H-4), colors.body)
}

func drawMobileWorkspace(frame *render.Frame, rect mobileui.Rect) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	colors := mobileColors()
	render.DrawRect(frame, float64(rect.X+3), float64(rect.Y+4), float64(rect.W), float64(rect.H), colors.shadow)
	render.DrawRect(frame, float64(rect.X), float64(rect.Y), float64(rect.W), float64(rect.H), colors.border)
	render.DrawRect(frame, float64(rect.X+2), float64(rect.Y+2), float64(rect.W-4), float64(rect.H-4), colors.workspace)
}

func drawMobileCard(frame *render.Frame, rect mobileui.Rect, colors mobilePalette, active bool) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	render.DrawRect(frame, float64(rect.X+3), float64(rect.Y+4), float64(rect.W), float64(rect.H), colors.shadow)
	render.DrawRect(frame, float64(rect.X), float64(rect.Y), float64(rect.W), float64(rect.H), colors.border)
	fill := colors.card
	if active {
		fill = colors.cardActive
	}
	render.DrawRect(frame, float64(rect.X+3), float64(rect.Y+3), float64(rect.W-6), float64(rect.H-6), fill)
}

func drawMobileSelectionOutline(frame *render.Frame, rect mobileui.Rect, colors mobilePalette) {
	if rect.W < 8 || rect.H < 8 {
		return
	}
	thickness := float32(4)
	render.DrawRect(frame, float64(rect.X), float64(rect.Y), float64(rect.W), float64(thickness), colors.accent)
	render.DrawRect(frame, float64(rect.X), float64(rect.Bottom()-thickness), float64(rect.W), float64(thickness), colors.accent)
	render.DrawRect(frame, float64(rect.X), float64(rect.Y), float64(thickness), float64(rect.H), colors.accent)
	render.DrawRect(frame, float64(rect.Right()-thickness), float64(rect.Y), float64(thickness), float64(rect.H), colors.accent)
}

func drawMobileHeader(frame *render.Frame, rect mobileui.Rect, title string, colors mobilePalette, scale float64) {
	if rect.W <= 4 || rect.H <= 4 {
		return
	}
	height := minf32(30, rect.H-4)
	render.DrawRect(frame, float64(rect.X+2), float64(rect.Y+2), float64(rect.W-4), float64(height), colors.header)
	render.DrawRect(frame, float64(rect.X+2), float64(rect.Y+2), float64(rect.W-4), float64(height/2), colors.headerTop)
	drawMobileTextBoxFit(frame, title, mobileui.Rect{X: rect.X + 10, Y: rect.Y + 4, W: rect.W - 20, H: height - 8}, colors.title, scale*0.92)
}

func drawMobileHeaderWithAction(frame *render.Frame, rect mobileui.Rect, title string, action mobileui.Rect, colors mobilePalette, scale float64) {
	if rect.W <= 4 || rect.H <= 4 {
		return
	}
	drawMobileHeader(frame, rect, "", colors, scale)
	height := minf32(30, rect.H-4)
	titleRight := rect.Right() - 10
	if action.W > 0 && action.X > rect.X && action.X < titleRight {
		titleRight = action.X - 10
	}
	titleRect := mobileui.Rect{X: rect.X + 10, Y: rect.Y + 4, W: maxf32(0, titleRight-(rect.X+10)), H: maxf32(0, height-8)}
	drawMobileTextBoxFit(frame, title, titleRect, colors.title, scale*0.92)
}

func drawMobileHeaderWithBack(frame *render.Frame, header, back mobileui.Rect, title string, colors mobilePalette, scale float64, backLabel string) {
	if header.W <= 4 || header.H <= 4 {
		return
	}
	drawMobileHeader(frame, header, "", colors, scale)
	titleRect := header
	headerHeight := minf32(30, header.H-4)
	titleRect.X = back.Right() + 16
	titleRect.W = maxf32(0, header.Right()-titleRect.X)
	titleRect.Y += 4
	titleRect.H = maxf32(0, headerHeight-8)
	drawMobileTextBoxFit(frame, title, titleRect, colors.title, scale*0.92)
	drawMobileButton(frame, back, backLabel, colors, scale*0.82, false)
}

func drawMobileButton(frame *render.Frame, rect mobileui.Rect, label string, colors mobilePalette, scale float64, active bool) {
	if rect.W <= 0 || rect.H <= 0 {
		return
	}
	drawMobilePanel(frame, rect)
	fill := colors.button
	if active {
		fill = colors.buttonActive
	}
	render.DrawRect(frame, float64(rect.X+3), float64(rect.Y+3), float64(rect.W-6), float64(rect.H-6), fill)
	drawMobileButtonLabel(frame, label, rect, colors.text, scale)
}

func drawMobileButtonLabel(frame *render.Frame, label string, rect mobileui.Rect, c color.RGBA, scale float64) {
	if label == "" || rect.W <= 12 || rect.H <= 8 {
		return
	}
	availableW := maxf32(8, rect.W-12)
	availableH := maxf32(8, rect.H-8)
	lines := mobileRichTextLines(label, availableW, scale, c)
	if len(lines) == 0 {
		return
	}
	if len(lines) > 2 {
		// Two lines preserve a readable action target. Extremely long labels
		// still use the same width-aware fallback rather than creating a third
		// line that would collide with the button chrome.
		merged := make([]mobileRichRune, 0)
		for _, line := range lines[1:] {
			if len(merged) > 0 {
				merged = append(merged, mobileRichRune{Rune: ' ', Color: c})
			}
			merged = append(merged, line...)
		}
		lines = [][]mobileRichRune{lines[0], merged}
	}
	_, glyphH := render.BitmapTextSize(stringFromMobileRichRunes(lines[0]))
	if glyphH <= 0 {
		return
	}
	lineGap := float32(4)
	if len(lines) == 1 {
		lineGap = 0
	}
	widest := 0
	for _, line := range lines {
		w := int(mobileRichLineWidth(line, 1))
		if w > widest {
			widest = w
		}
	}
	if widest > 0 && float32(widest)*float32(scale) > availableW {
		scale = float64(availableW) / float64(widest)
	}
	totalH := float32(len(lines)*glyphH)*float32(scale) + float32(len(lines)-1)*lineGap
	if totalH > availableH {
		scale = minf64(scale, float64(maxf32(8, availableH-float32(len(lines)-1)*lineGap))/float64(len(lines)*glyphH))
	}
	lineH := float32(glyphH) * float32(scale)
	startY := rect.Y + (rect.H-(lineH*float32(len(lines))+lineGap*float32(len(lines)-1)))/2
	for i, line := range lines {
		lineW := mobileRichLineWidth(line, scale)
		cursorX := rect.X + (rect.W-lineW)/2
		for _, run := range mobileRichRuns(line) {
			drawMobileText(frame, run.Text, cursorX, startY+float32(i)*(lineH+lineGap), run.Color, scale)
			w, _ := render.BitmapTextSize(run.Text)
			cursorX += float32(w) * float32(scale)
		}
	}
}

func stringFromMobileRichRunes(line []mobileRichRune) string {
	var builder strings.Builder
	for _, char := range line {
		builder.WriteRune(char.Rune)
	}
	return builder.String()
}

func drawMobileActionRow(frame *render.Frame, row mobileui.Rect, mainLabel, actionLabel string, colors mobilePalette, mainScale, actionScale float64, mainActive bool) {
	if row.W <= 0 || row.H <= 0 {
		return
	}
	drawMobilePanel(frame, row)
	actionW := minf32(180, maxf32(112, row.W*0.25))
	if actionW > row.W-8 {
		actionW = maxf32(0, row.W-8)
	}
	action := mobileui.Rect{X: row.Right() - actionW, Y: row.Y, W: actionW, H: row.H}
	main := mobileui.Rect{X: row.X, Y: row.Y, W: maxf32(0, action.X-row.X-8), H: row.H}
	if main.W > 0 {
		drawMobileButton(frame, main, mainLabel, colors, mainScale, mainActive)
	}
	if action.W > 0 {
		drawMobileButton(frame, action, actionLabel, colors, actionScale, false)
	}
}

func drawMobileTextFit(frame *render.Frame, text string, x, y, width float32, c color.RGBA, scale float64) {
	if text == "" || width <= 0 {
		return
	}
	w, _ := render.BitmapTextSize(text)
	if w > 0 && float32(w)*float32(scale) > width {
		scale = float64(width) / float64(w)
	}
	drawMobileText(frame, text, x, y, c, scale)
}

func drawMobileTextBoxFit(frame *render.Frame, text string, rect mobileui.Rect, c color.RGBA, scale float64) {
	if text == "" || rect.W <= 0 || rect.H <= 0 {
		return
	}
	w, h := render.BitmapTextSize(text)
	if w > 0 && float32(w)*float32(scale) > rect.W {
		scale = float64(rect.W) / float64(w)
	}
	if h > 0 && float32(h)*float32(scale) > rect.H {
		scale = minf64(scale, float64(rect.H)/float64(h))
	}
	drawMobileText(frame, text, rect.X, rect.Y, c, scale)
}

func drawMobileTextCentered(frame *render.Frame, text string, rect mobileui.Rect, c color.RGBA, scale float64) {
	if text == "" || rect.W <= 0 || rect.H <= 0 {
		return
	}
	w, h := render.BitmapTextSize(text)
	if w <= 0 || h <= 0 {
		return
	}
	if float32(w)*float32(scale) > rect.W {
		scale = float64(rect.W) / float64(w)
	}
	textW := float32(w) * float32(scale)
	textH := float32(h) * float32(scale)
	drawMobileText(frame, text, rect.X+(rect.W-textW)/2, rect.Y+(rect.H-textH)/2, c, scale)
}

func drawMobileText(frame *render.Frame, text string, x, y float32, c color.RGBA, scale float64) {
	if text == "" {
		return
	}
	render.DrawBitmapTextScaledAtColor(frame, text, int(x), int(y), c, scale)
}

func centerMobileText(frame *render.Frame, text string, x, y, width float32, c color.RGBA, scale float64) {
	if text == "" || width <= 0 {
		return
	}
	w, _ := render.BitmapTextSize(text)
	textWidth := float32(w) * float32(scale)
	if textWidth > width {
		textWidth = width
	}
	drawMobileText(frame, text, x+(width-textWidth)/2, y, c, scale)
}

func drawMobileBar(frame *render.Frame, label string, x, y, w, h float32, value, maximum int, fill color.RGBA, colors mobilePalette, scale float64) {
	labelW := maxf32(40, minf32(54, h*2.2))
	drawMobileTextFit(frame, label, x, y-1, labelW-4, colors.muted, scale*0.82)
	barX, barW := x+labelW, w-labelW
	render.DrawRect(frame, float64(barX), float64(y), float64(barW), float64(h), colors.border)
	render.DrawRect(frame, float64(barX+2), float64(y+2), float64(barW-4), float64(h-4), color.RGBA{R: 225, G: 231, B: 238, A: 255})
	amount := float32(0)
	if maximum > 0 {
		amount = float32(value) / float32(maximum)
	}
	if amount < 0 {
		amount = 0
	}
	if amount > 1 {
		amount = 1
	}
	if amount > 0 {
		render.DrawRect(frame, float64(barX+2), float64(y+2), float64((barW-4)*amount), float64(h-4), fill)
	}
	valueText := fmt.Sprintf("%d / %d", value, maximum)
	valueW := float32(100)
	drawMobileTextFit(frame, valueText, barX+barW-valueW, y+2, valueW-4, colors.text, scale*0.72)
}

func drawMobileStatuses(frame *render.Frame, rect mobileui.Rect, statuses []mobileui.StatusEffectModel, colors mobilePalette, scale float64) {
	if rect.W <= 0 || rect.H <= 0 || len(statuses) == 0 {
		return
	}
	drawMobilePanel(frame, rect)
	maxVisible := int((rect.W - 12) / 46)
	if maxVisible < 1 {
		return
	}
	if maxVisible > len(statuses) {
		maxVisible = len(statuses)
	}
	for i := 0; i < maxVisible; i++ {
		box := mobileui.Rect{X: rect.X + 6 + float32(i)*46, Y: rect.Y + 6, W: 40, H: rect.H - 12}
		fill := colors.header
		if statuses[i].Beneficial {
			fill = color.RGBA{R: 210, G: 238, B: 216, A: 255}
		}
		render.DrawRect(frame, float64(box.X), float64(box.Y), float64(box.W), float64(box.H), fill)
		label := fmt.Sprintf("%d", i+1)
		if statuses[i].IconKey != "" {
			label = trimText(statuses[i].IconKey, 4)
		}
		drawMobileText(frame, label, box.X+4, box.Y+7, colors.title, scale*0.64)
		if statuses[i].Remaining > 0 {
			drawMobileText(frame, fmt.Sprintf("%ds", int(statuses[i].Remaining.Seconds()+0.99)), box.X+4, box.Bottom()-15, colors.muted, scale*0.56)
		}
	}
	if maxVisible < len(statuses) {
		drawMobileText(frame, fmt.Sprintf("+%d", len(statuses)-maxVisible), rect.Right()-36, rect.Y+16, colors.accent, scale*0.72)
	}
}

func drawMobileItemShortcut(frame *render.Frame, rect mobileui.Rect, item mobileui.InventoryItemModel, _ int, game *app.Game, colors mobilePalette, scale float64) {
	active := item.Usable && item.Index != 0 && item.Quantity > 0
	drawMobileCard(frame, rect, colors, active)
	iconSize := minf32(64, maxf32(40, minf32(rect.W, rect.H)-10))
	iconX := rect.X + (rect.W-iconSize)/2
	iconY := rect.Y + (rect.H-iconSize)/2
	if game != nil && item.ItemID != 0 {
		game.DrawMobileInventoryItemIcon(frame, item, int(iconX), int(iconY), int(iconSize))
	}
	if !active {
		render.DrawRect(frame, float64(rect.X+3), float64(rect.Y+3), float64(rect.W-6), float64(rect.H-6), color.RGBA{R: 30, G: 46, B: 66, A: 120})
	}
	if item.Quantity > 1 {
		drawMobileShortcutBadge(frame, strconv.Itoa(item.Quantity), rect, true, scale)
	}
}

func drawMobileSkill(frame *render.Frame, rect mobileui.Rect, skill mobileui.SkillSlotModel, _ int, game *app.Game, colors mobilePalette, scale float64) {
	active := skill.Usable && skill.CooldownRemaining <= 0
	drawMobileCard(frame, rect, colors, active)
	iconSize := minf32(64, maxf32(40, minf32(rect.W, rect.H)-10))
	iconX := rect.X + (rect.W-iconSize)/2
	iconY := rect.Y + (rect.H-iconSize)/2
	render.DrawRect(frame, float64(iconX), float64(iconY), float64(iconSize), float64(iconSize), colors.header)
	glyph := "?"
	if skill.Name != "" {
		glyph = strings.ToUpper(string([]rune(skill.Name)[0]))
	}
	w, _ := render.BitmapTextSize(glyph)
	drawMobileText(frame, glyph, iconX+(iconSize-float32(w)*float32(scale))/2, iconY+7, colors.title, scale*1.05)
	if game != nil {
		game.DrawMobileSkillIcon(frame, mobileui.MobileSkillModel{SkillID: skill.SkillID}, int(iconX), int(iconY), int(iconSize))
	}
	if !active {
		render.DrawRect(frame, float64(rect.X+3), float64(rect.Y+3), float64(rect.W-6), float64(rect.H-6), color.RGBA{R: 30, G: 46, B: 66, A: 120})
	}
	if skill.Level > 0 {
		drawMobileShortcutBadge(frame, "Lv"+strconv.Itoa(skill.Level), rect, false, scale)
	}
	if skill.CooldownRemaining > 0 {
		seconds := int(skill.CooldownRemaining.Seconds() + 0.99)
		drawMobileTextCentered(frame, fmt.Sprintf("%ds", seconds), rect, color.RGBA{R: 255, G: 255, B: 255, A: 255}, scale*0.78)
	}
}

func mobileSkillDisplayName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	parts := strings.Fields(name)
	if len(parts) > 1 && (strings.EqualFold(parts[0], "SM") || strings.EqualFold(parts[0], "JOB")) {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return "SKILL"
	}
	return trimText(strings.Join(parts, " "), 12)
}

func drawMobileWrappedText(frame *render.Frame, value string, x, y float32, maxChars, lineAdvance int, c color.RGBA, scale float64) {
	drawMobileWrappedTextLimited(frame, value, x, y, maxChars, lineAdvance, 0, c, scale)
}

func drawMobileWrappedTextLimited(frame *render.Frame, value string, x, y float32, maxChars, lineAdvance, maxLines int, c color.RGBA, scale float64) {
	if maxChars < 8 {
		maxChars = 8
	}
	drawMobileRichWrappedTextLimited(frame, value, x, y, float32(maxChars)*11*float32(scale), float32(lineAdvance), maxLines, c, scale)
}

func mobileDescriptionText(lines []string) string {
	return strings.Join(lines, "\n")
}

type mobileRichRune struct {
	Rune  rune
	Color color.RGBA
}

func drawMobileRichWrappedTextLimited(frame *render.Frame, value string, x, y, maxWidth, lineAdvance float32, maxLines int, base color.RGBA, scale float64) {
	if value == "" || maxWidth <= 0 || lineAdvance <= 0 {
		return
	}
	lines := mobileRichTextLines(value, maxWidth, scale, base)
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for row, line := range lines {
		cursorX := x
		for _, run := range mobileRichRuns(line) {
			if run.Text == "" {
				continue
			}
			drawMobileText(frame, run.Text, cursorX, y+float32(row)*lineAdvance, run.Color, scale)
			width, _ := render.BitmapTextSize(run.Text)
			cursorX += float32(width) * float32(scale)
		}
	}
}

func mobileRichTextLines(value string, maxWidth float32, scale float64, base color.RGBA) [][]mobileRichRune {
	var lines [][]mobileRichRune
	var current []mobileRichRune
	var word []mobileRichRune
	flushWord := func() {
		if len(word) == 0 {
			return
		}
		if len(current) == 0 {
			current = appendRichWordWithBreaks(&lines, current, word, maxWidth, scale)
			word = nil
			return
		}
		candidate := append(append([]mobileRichRune(nil), current...), mobileRichRune{Rune: ' ', Color: word[0].Color})
		candidate = append(candidate, word...)
		if mobileRichLineWidth(candidate, scale) <= maxWidth {
			current = candidate
			word = nil
			return
		}
		lines = append(lines, current)
		current = nil
		current = appendRichWordWithBreaks(&lines, current, word, maxWidth, scale)
		word = nil
	}
	for _, run := range mobileui.ParseROText(value, base) {
		for _, r := range run.Text {
			if r == '\n' {
				flushWord()
				lines = append(lines, current)
				current = nil
				continue
			}
			if unicode.IsSpace(r) {
				flushWord()
				continue
			}
			word = append(word, mobileRichRune{Rune: r, Color: run.Color})
		}
	}
	flushWord()
	if len(current) > 0 || len(lines) == 0 {
		lines = append(lines, current)
	}
	return lines
}

func appendRichWordWithBreaks(lines *[][]mobileRichRune, current, word []mobileRichRune, maxWidth float32, scale float64) []mobileRichRune {
	for len(word) > 0 && mobileRichLineWidth(word, scale) > maxWidth {
		cut := len(word)
		for cut > 1 && mobileRichLineWidth(word[:cut], scale) > maxWidth {
			cut--
		}
		if cut == 0 {
			cut = 1
		}
		*lines = append(*lines, append([]mobileRichRune(nil), word[:cut]...))
		word = word[cut:]
	}
	return append(current, word...)
}

func mobileRichLineWidth(line []mobileRichRune, scale float64) float32 {
	width := float32(0)
	for _, run := range mobileRichRuns(line) {
		w, _ := render.BitmapTextSize(run.Text)
		width += float32(w) * float32(scale)
	}
	return width
}

func mobileRichRuns(line []mobileRichRune) []mobileui.MobileTextRun {
	if len(line) == 0 {
		return nil
	}
	runs := make([]mobileui.MobileTextRun, 0, len(line))
	for _, char := range line {
		if len(runs) > 0 && runs[len(runs)-1].Color == char.Color {
			runs[len(runs)-1].Text += string(char.Rune)
			continue
		}
		runs = append(runs, mobileui.MobileTextRun{Text: string(char.Rune), Color: char.Color})
	}
	return runs
}

func targetRelationText(relation mobileui.TargetRelation) string {
	switch relation {
	case mobileui.TargetHostile:
		return "HOSTILE"
	case mobileui.TargetFriendly:
		return "FRIEND"
	case mobileui.TargetNPC:
		return "NPC"
	default:
		return "TARGET"
	}
}

func skillTargetText(mode input.SkillTargetMode) string {
	switch mode {
	case input.SkillTargetActor:
		return "ACTOR"
	case input.SkillTargetGround:
		return "GROUND"
	default:
		return "SELF"
	}
}

func minf32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func minf64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxf32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func mobileSectionVisible(rect, viewport mobileui.Rect, portrait bool) bool {
	if rect.W <= 0 || rect.H <= 0 {
		return false
	}
	if !portrait {
		return true
	}
	// The bitmap renderer has no scissor stack. Whole-card visibility keeps a
	// vertically scrolling portrait section inside its safe content viewport.
	return rect.Y >= viewport.Y && rect.Bottom() <= viewport.Bottom()
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max < 2 || len(value) <= max {
		return value
	}
	return value[:max-1] + "…"
}

func itemByIndex(items []mobileui.InventoryItemModel, index uint16) (mobileui.InventoryItemModel, bool) {
	for _, item := range items {
		if item.Index == index {
			return item, true
		}
	}
	return mobileui.InventoryItemModel{}, false
}

func equipmentSlotByLocation(model mobileui.MobileEquipmentModel, location uint16) (mobileui.EquipmentSlotModel, bool) {
	for _, slot := range model.Slots {
		if slot.Location == location {
			return slot, slot.HasItem
		}
	}
	return mobileui.EquipmentSlotModel{}, false
}

func slotItemIndex(slot mobileui.EquipmentSlotModel) uint16 {
	if !slot.HasItem {
		return 0
	}
	return slot.ItemIndex
}

func itemLabel(item mobileui.InventoryItemModel) string {
	if strings.TrimSpace(item.DisplayName) != "" {
		return item.DisplayName
	}
	return fmt.Sprintf("Item %d", item.ItemID)
}
