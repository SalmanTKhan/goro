package app

import (
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"github.com/gogpu/gpucontext"
	gameaudio "github.com/kivutar/goro/audio"
	"github.com/kivutar/goro/capture"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/game"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/network"
	"github.com/kivutar/goro/render"
	"github.com/kivutar/goro/res"
	"github.com/kivutar/goro/session"
	gameui "github.com/kivutar/goro/ui"
	"github.com/kivutar/goro/world"
)

type Game struct {
	cfg                   config.Config
	input                 *input.State
	resource              *res.Manager
	assets                client.AssetAvailability
	session               *session.Session
	world                 *world.World
	network               *network.Client
	offline               *session.OfflineSession
	audio                 *gameaudio.BGM
	modes                 *game.Manager
	runtime               *runtimeSettings
	uiApp                 client.UIApp
	ui                    *gameui.Manager
	started               time.Time
	lastUpdate            time.Time
	screenW               int
	screenH               int
	uiW                   int
	uiH                   int
	quit                  func()
	quitting              bool
	pendingScreenshot     string
	pendingScreenshotOpts capture.ScreenshotOptions
	pendingRecording      capture.RecordingOptions
	hasPendingRecording   bool
	recordingActive       bool
	pendingRecordingStop  bool
	mobileTarget          mobileui.TargetHUDModel
	mobileSettingsChanged func(input.MobileSettings)
}

func (g *Game) SetMobileSettingsChanged(callback func(input.MobileSettings)) {
	if g != nil {
		g.mobileSettingsChanged = callback
	}
}

func New(cfg config.Config) (*Game, error) {
	resource, err := res.NewManager(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resource manager: %w", err)
	}
if !cfg.Headless {
	loadClientUIFont(resource)
}
server := cfg.MobileSession.Server.Normalized()
packetDate := cfg.Packet.ClientDate
if cfg.MobileSession.Mode == config.SessionModeOnline && server.ClientDate != 0 {
	packetDate = server.ClientDate
}
if cfg.MobileSession.Mode == config.SessionModeOnline && server.Host != "" {
	// A mobile build may not ship clientinfo.xml. Inject the explicitly
	// selected server while retaining the existing resource-driven desktop
	// discovery path when no mobile server is configured.
	resource.ClientInfo.Connections = []res.Connection{{
		Display: server.Name, Address: server.Host, Port: server.AuthPort,
		// The version field is the client date written into CA_LOGIN. The
		// packet profile is a local selection for packet builders and is not
		// the value expected by the account server in this field.
		Version: server.ClientDate,
	}}
}
	}

	g := &Game{
		cfg:        cfg,
		input:      input.NewState(),
		resource:   resource,
		assets:     localAssetAvailability{manager: resource},
		session:    session.New(),
		world:      world.New(),
		network:    network.NewClient(packetDate, cfg.Network.Trace),
		audio:      gameaudio.NewBGM(resource, cfg.Audio.BGM, cfg.Audio.BGMVolume, cfg.Audio.SFXVolume, cfg.Audio.Disabled),
		runtime:    newRuntimeSettings(cfg.Window.Fullscreen, cfg.Render.VSync, cfg.Render.FPS),
		ui:         gameui.NewManager(),
		started:    time.Now(),
		lastUpdate: time.Now(),
		screenW:    cfg.Window.Width,
		screenH:    cfg.Window.Height,
	}
	g.session.KeepLoginID = cfg.Login.KeepID
	g.session.SavedUsername = cfg.Login.SavedUsername
	g.session.NoShift = cfg.Gameplay.NoShift
	g.session.NoCtrl = cfg.Gameplay.NoCtrl
	g.session.LessEffects = cfg.Gameplay.LessEffects
	g.session.SnapTargets = cfg.Gameplay.SnapTargets
	g.session.SnapItems = cfg.Gameplay.SnapItems
	if cfg.Gameplay.ForceUserAI {
		g.session.HomunculusCustomAI = true
		g.session.MercenaryCustomAI = true
	}

	ctx := g.modeContext()
	g.modes = game.NewManager(ctx, game.NewLoginMode())
	return g, nil
}

// NewOffline creates the same client graph as the online runtime, but starts
// directly in the production world mode with local authority. This keeps
// rendering, projections, and mobile commands shared with the online client.
func NewOffline(cfg config.Config) (*Game, error) {
	return NewOfflineAtMap(cfg, "prontera")
}

// NewOfflineAtLogin loads the same local authority as NewOfflineAtMap but
// enters through the shared desktop login and character-select presentation.
func NewOfflineAtLogin(cfg config.Config, startMap string) (*Game, error) {
	g, err := NewOfflineAtMap(cfg, startMap)
	if err != nil {
		return nil, err
	}
	character := g.session.Selected
	character.Slot = 0
	g.session.Characters = []session.Character{character}
	g.session.Playing = false
	g.modes = game.NewManager(g.modeContext(), game.NewOfflineLoginMode())
	return g, nil
}

// NewOfflineAtMap is the profile-aware offline entry point used by mobile
// asset manifests. The legacy NewOffline wrapper retains the Prontera default
// for desktop callers and existing tests.
func NewOfflineAtMap(cfg config.Config, startMap string) (*Game, error) {
	g, err := New(cfg)
	if err != nil {
		return nil, err
	}
	contentData, err := g.resource.ReadFile("offline/content.json")
	if err != nil {
		return nil, fmt.Errorf("offline content pack missing: %w", err)
	}
	content, err := session.DecodeOfflineContent(contentData)
	if err != nil {
		return nil, fmt.Errorf("offline content pack invalid: %w", err)
	}
	if content.Source.Packetver != 0 && cfg.Packet.ClientDate != 0 && content.Source.Packetver != cfg.Packet.ClientDate {
		return nil, fmt.Errorf("offline content pack is stale: packetver=%d client=%d", content.Source.Packetver, cfg.Packet.ClientDate)
	}
	g.network = nil
	startMap = strings.ToLower(strings.TrimSpace(startMap))
	if startMap == "" {
		startMap = "prontera"
	}
	g.offline = session.NewOfflineSession(startMap)
	character := session.Character{
		ID: 1, Name: "Offline Adventurer", Level: 1, JobLevel: 1,
		HP: 100, MaxHP: 100, SP: 30, MaxSP: 30, Job: 0,
		Hair: 1, HairColor: 1, Money: 2500, Str: 5, Agi: 5, Vit: 5, Int: 5, Dex: 5, Luk: 5,
	}
	g.session.SelectCharacter(character)
	g.session.Playing = true
	g.session.Sex = 0
	redPotion, ok := content.Items[501]
	if !ok {
		return nil, fmt.Errorf("offline content pack missing starter item 501")
	}
	weapon, ok := content.Items[1201]
	if !ok {
		return nil, fmt.Errorf("offline content pack missing starter item 1201")
	}
	jellopy, ok := content.Items[909]
	if !ok {
		return nil, fmt.Errorf("offline content pack missing starter item 909")
	}
	starterWeight := 8*redPotion.Weight + weapon.Weight + 12*jellopy.Weight
	// Keep the deterministic starter loadout below its offline fixture capacity
	// so the first combat-to-loot test can pick up the guaranteed Poring drop.
	// Weight checks remain authoritative in OfflineRuntime; this only avoids
	// seeding a new character already over capacity.
	g.session.Inventory = session.Inventory{Zeny: character.Money, Weight: starterWeight, MaxWeight: 2000, Items: []session.InventoryItem{
		{Index: 1, ItemID: redPotion.ID, Type: redPotion.Type, Identified: true, Amount: 8},
		// The starter weapon is part of the visible offline loadout. Mark it as
		// equipped so the mobile paper-doll surface has a real item to render.
		{Index: 2, ItemID: weapon.ID, Type: weapon.Type, Location: db.EquipWeapon, Identified: true, Amount: 1, Equip: true, Equipped: true},
		{Index: 3, ItemID: jellopy.ID, Type: jellopy.Type, Identified: true, Amount: 12},
	}}
	g.session.Storage = session.Storage{MaxAmount: 300}
	if _, ok := content.Skills[db.SkillSMBash]; !ok {
		return nil, fmt.Errorf("offline content pack missing starter skill %d", db.SkillSMBash)
	}
	g.session.Skills, g.session.Hotkeys = session.StarterSkillLoadout(content)
	g.session.PlayerX, g.session.PlayerY, g.session.PlayerDir = 78, 98, 0
	g.offline.BindState(g.session)
	g.offline.EnsureProfile(g.session)
	if err := g.offline.LoadContent(content, startMap); err != nil {
		return nil, fmt.Errorf("load offline content pack: %w", err)
	}
	g.session.Zone.MapName = g.offline.MapName
	g.world.MapName = g.offline.MapName
	g.world.SetPlayerPosition(78, 98, 0)
	g.modes = game.NewManager(g.modeContext(), game.NewWorldMode())
	return g, nil
}

func (g *Game) Update() error {
	defer g.input.EndFrame()
	if g.offline != nil && g.offline.Paused {
		return nil
	}
	now := time.Now()
	if g.offline != nil {
		dt := now.Sub(g.lastUpdate)
		g.offline.Update(dt)
	}
	g.lastUpdate = now
	if g.network != nil {
		g.network.Pump()
	}
	g.modes.UpdateContext(g.modeContext())
	return g.modes.Update()
}

func (g *Game) Draw(screen *render.Frame) {
	if g.cfg.Headless {
		return
	}
	g.modes.Draw(screen)
}

func (g *Game) DrawOverlay(screen *render.Frame) {
	if g.cfg.Headless {
		return
	}
	g.modes.DrawOverlay(screen)
}

func (g *Game) DrawUIOverlay(screen *render.Frame) {
	if g.cfg.Headless {
		return
	}
	g.modes.DrawUIOverlay(screen)
}

// DrawMobileTileCursor renders the shared world tile highlight after the
// world pass and before mobile UI overlays are drawn.
func (g *Game) DrawMobileTileCursor(position input.WorldPosition, screen *render.Frame) {
	if g == nil || g.modes == nil {
		return
	}
	g.modes.DrawMobileTileCursor(screen, position)
}

func (g *Game) FrameSubmitted() {
	g.modes.FrameSubmitted()
}

func (g *Game) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		g.screenW = g.cfg.Window.Width
		g.screenH = g.cfg.Window.Height
		return
	}
	g.screenW = width
	g.screenH = height
}

func (g *Game) InputState() *input.State {
	return g.input
}

// ControllerSettings exposes the desktop controller policy to the renderer
// without making the render package depend on the application configuration
// graph.
func (g *Game) ControllerSettings() input.ControllerSettings {
	if g == nil {
		return input.DefaultControllerSettings()
	}
	return g.cfg.Controller.Normalized()
}

// ApplyControllerSettings updates the controller policy at runtime. The
// renderer re-reads ControllerSettings every poll, so a change here takes
// effect within a frame.
func (g *Game) ApplyControllerSettings(settings input.ControllerSettings) bool {
	if g == nil {
		return false
	}
	g.cfg.Controller = settings.Normalized()
	return true
}

// MountAssetOverlay activates a verified mobile delivery layer at the next
// render-thread command boundary. Callers never modify the embedded base.
func (g *Game) MountAssetOverlay(overlay res.AssetOverlay) error {
	if g == nil || g.resource == nil {
		return fmt.Errorf("asset resource manager is unavailable")
	}
	return g.resource.MountOverlay(overlay)
}

// ReplaceAssetOverlays atomically swaps the active optional release while
// keeping the embedded/base resource containers untouched.
func (g *Game) ReplaceAssetOverlays(overlays []res.AssetOverlay) error {
	if g == nil || g.resource == nil {
		return fmt.Errorf("asset resource manager is unavailable")
	}
	return g.resource.ReplaceOverlays(overlays)
}

func (g *Game) ApplyPlayerCommand(command input.PlayerCommand) bool {
	if g == nil || g.modes == nil {
		return false
	}
	if command.Kind == input.CommandInspectActor {
		target, ok := g.modes.InspectMobileTarget(g.modeContext(), command.ActorID)
		if !ok {
			return false
		}
		g.mobileTarget = target
		return true
	}
	return g.modes.ApplyPlayerCommand(g.modeContext(), command)
}

func (g *Game) MobileControls() input.MobileControls {
	if g == nil {
		return input.DefaultMobileControls()
	}
	return g.cfg.Mobile.Normalized()
}

func (g *Game) PickMobileTarget(position input.WorldPosition) (input.PickedTarget, bool) {
	if g == nil || g.modes == nil {
		return input.PickedTarget{}, false
	}
	return g.modes.PickMobileTarget(g.modeContext(), position)
}

func (g *Game) MobileHUDModel() mobileui.MobileHUDModel {
	if g == nil || g.session == nil {
		return mobileui.MobileHUDModel{}
	}
	target := inputTargetHUD(g.offline)
	if g.mobileTarget.Visible {
		target = g.mobileTarget
	}
	model := mobileui.ProjectSession(g.session, target)
	if g.world != nil && g.world.GND != nil {
		model.Minimap.Raster = mobileMinimapRaster(g.world.GND)
	}
	if g.offline != nil {
		for _, monster := range g.offline.Monsters() {
			if monster.State == session.MonsterDead || monster.State == session.MonsterRespawn {
				continue
			}
			model.Minimap.Markers = append(model.Minimap.Markers, mobileui.MinimapMarkerModel{
				ID: monster.ID, Name: monster.Name, X: monster.X, Y: monster.Y,
				Kind: mobileui.MinimapMarkerHostile, Selected: monster.ID == g.offline.TargetID,
			})
		}
		for _, npc := range g.offline.NPCs() {
			model.Minimap.Markers = append(model.Minimap.Markers, mobileui.MinimapMarkerModel{
				ID: npc.ID, Name: npc.Name, X: npc.X, Y: npc.Y,
				Kind: mobileui.MinimapMarkerNPC, Selected: npc.ID == g.offline.TargetID,
			})
		}
		for _, warp := range g.offline.Warps() {
			model.Minimap.Markers = append(model.Minimap.Markers, mobileui.MinimapMarkerModel{
				ID: warp.ID, Name: warp.Name, X: warp.X, Y: warp.Y,
				Kind: mobileui.MinimapMarkerWarp, Selected: warp.ID == g.offline.TargetID,
			})
		}
		playerX, playerY := g.session.PlayerX, g.session.PlayerY
		for _, drop := range g.offline.Drops() {
			name := fmt.Sprintf("Item %d", drop.ItemID)
			if definition, ok := g.offline.Item(drop.ItemID); ok && strings.TrimSpace(definition.Name) != "" {
				name = definition.Name
			}
			if g.resource != nil {
				if resolved, ok := g.resource.ItemDisplayName(int(drop.ItemID), drop.Identified); ok && strings.TrimSpace(resolved) != "" {
					name = resolved
				}
			}
			distance := mobileLootDistance(playerX, playerY, drop.X, drop.Y)
			model.Loot = append(model.Loot, mobileui.LootItemModel{
				DropID: drop.ID, ItemID: drop.ItemID, Identified: drop.Identified, Name: name, Quantity: drop.Amount,
				X: drop.X, Y: drop.Y, Distance: distance, PickupReady: distance <= 1,
			})
			model.Minimap.Markers = append(model.Minimap.Markers, mobileui.MinimapMarkerModel{
				ID: drop.ID, Name: name, X: drop.X, Y: drop.Y, Kind: mobileui.MinimapMarkerItem,
			})
		}
		sort.Slice(model.Loot, func(i, j int) bool {
			if model.Loot[i].Distance != model.Loot[j].Distance {
				return model.Loot[i].Distance < model.Loot[j].Distance
			}
			return model.Loot[i].DropID < model.Loot[j].DropID
		})
	}
	return model
}

func (g *Game) HandleKeyPress(code input.KeyCode) {
	if g.modes != nil {
		g.modes.HandleKeyPress(g.modeContext(), code)
	}
}

func (g *Game) PrepareTextInput(code input.KeyCode) bool {
	return g.modes != nil && g.modes.PrepareTextInput(g.modeContext(), code)
}

func (g *Game) PrepareKeyInput(code input.KeyCode, mods gpucontext.Modifiers) {
	if g.modes != nil {
		g.modes.PrepareKeyInput(g.modeContext(), code, mods)
	}
}

func (g *Game) SetQuitFunc(quit func()) {
	g.quit = quit
}

func (g *Game) SetUIApp(uiApp client.UIApp) {
	g.uiApp = uiApp
	if g.ui != nil {
		g.ui.SetUIApp(uiApp)
	}
}

func (g *Game) SetUIViewport(width, height int) {
	if g == nil {
		return
	}
	oldWidth, oldHeight := g.uiW, g.uiH
	if oldWidth == width && oldHeight == height {
		return
	}
	g.uiW, g.uiH = width, height
	if responsive, ok := any(g.ui).(client.UIViewportManager); ok {
		responsive.ViewportChanged(oldWidth, oldHeight, width, height)
	}
}

func (g *Game) ContextUIManager() client.UIManager {
	if g == nil {
		return nil
	}
	return g.ui
}

func (g *Game) RequestQuit() {
	if g.quitting {
		return
	}
	g.quitting = true
	if g.network != nil {
		g.network.Close()
	}
	if g.audio != nil {
		g.audio.Stop()
	}
	if g.quit != nil {
		g.quit()
	}
}

func (g *Game) RequestScreenshot() (string, error) {
	return g.RequestScreenshotOptions(capture.ScreenshotOptions{Format: capture.StillPNG})
}

func (g *Game) RequestScreenshotOptions(options capture.ScreenshotOptions) (string, error) {
	if g.pendingScreenshot != "" {
		return "", fmt.Errorf("screenshot is already pending")
	}
	options, err := options.Normalized()
	if err != nil {
		return "", err
	}
	path, err := config.NextScreenshotPathFor(time.Now(), string(options.Format))
	if err != nil {
		return "", err
	}
	g.pendingScreenshot = path
	g.pendingScreenshotOpts = options
	return path, nil
}

func (g *Game) ConsumeScreenshotRequest() (string, bool) {
	if g.pendingScreenshot == "" {
		return "", false
	}
	path := g.pendingScreenshot
	g.pendingScreenshot = ""
	return path, true
}

func (g *Game) ConsumeCaptureRequest() (capture.ScreenshotOptions, string, bool) {
	if g.pendingScreenshot == "" {
		return capture.ScreenshotOptions{}, "", false
	}
	path := g.pendingScreenshot
	options := g.pendingScreenshotOpts
	g.pendingScreenshot = ""
	g.pendingScreenshotOpts = capture.ScreenshotOptions{}
	return options, path, true
}

func (g *Game) CompleteScreenshot(path string, err error) {
	if err != nil {
		glog.Errorf("screenshot failed path=%s error=%v", path, err)
		return
	}
	glog.Infof("screenshot saved path=%s", path)
}

func (g *Game) StartRecording(options capture.RecordingOptions) (string, error) {
	options, err := options.Normalized()
	if err != nil {
		return "", err
	}
	if g.hasPendingRecording || g.recordingActive {
		return "", fmt.Errorf("recording is already active")
	}
	path, err := config.NextCapturePath(time.Now(), string(options.Container))
	if err != nil {
		return "", err
	}
	options.Path = path
	g.pendingRecording = options
	g.hasPendingRecording = true
	g.recordingActive = true
	g.pendingRecordingStop = false
	return path, nil
}

func (g *Game) ConsumeRecordingStart() (capture.RecordingOptions, bool) {
	if !g.hasPendingRecording {
		return capture.RecordingOptions{}, false
	}
	options := g.pendingRecording
	g.pendingRecording = capture.RecordingOptions{}
	g.hasPendingRecording = false
	return options, true
}

func (g *Game) StopRecording() error {
	if !g.hasPendingRecording && !g.recordingActive {
		return fmt.Errorf("no recording is active")
	}
	g.pendingRecordingStop = true
	return nil
}

func (g *Game) ConsumeRecordingStop() bool {
	if !g.pendingRecordingStop {
		return false
	}
	g.pendingRecordingStop = false
	return true
}

func (g *Game) CompleteRecording(path string, err error) {
	g.recordingActive = false
	if err != nil {
		glog.Errorf("recording failed path=%s error=%v", path, err)
		return
	}
	glog.Infof("recording saved path=%s", path)
}

func (g *Game) RecordingResize(path string) {
	g.recordingActive = false
	glog.Infof("recording stopped after framebuffer resize path=%s; start a new recording", path)
}

func (g *Game) RuntimeFullscreen() bool {
	return g.runtime.Fullscreen()
}

func (g *Game) RuntimeVSync() bool {
	return g.runtime.VSync()
}

func (g *Game) RuntimeFPS() bool {
	return g.runtime.FPS()
}

func loadClientUIFont(resource *res.Manager) {
	regular, err := resource.ReadFileExact("System/Font/SCDream4.otf")
	if err != nil {
		return
	}
	bold, err := resource.ReadFileExact("System/Font/SCDream6.otf")
	if err != nil {
		glog.Warnf("ui font regular loaded but bold missing: %v", err)
	}
	if err := render.SetUIFont(regular, bold); err != nil {
		glog.Errorf("ui font load failed: %v", err)
		return
	}
	if len(bold) > 0 {
		glog.Infof("ui font loaded path=System/Font/SCDream4.otf bold=System/Font/SCDream6.otf")
	} else {
		glog.Infof("ui font loaded path=System/Font/SCDream4.otf")
	}
}

func (g *Game) modeContext() client.Context {
	return client.Context{
		Config:                   g.cfg,
		Input:                    g.input,
		Resources:                g.resource,
		Assets:                   g.assets,
		Session:                  g.session,
		World:                    g.world,
		Network:                  g.network,
		Offline:                  g.offline,
		Audio:                    g.audio,
		Started:                  g.started,
		ScreenW:                  g.screenW,
		ScreenH:                  g.screenH,
		UIWidth:                  g.uiW,
		UIHeight:                 g.uiH,
		Runtime:                  g.runtime,
		RequestQuit:              g.RequestQuit,
		RequestScreenshot:        g.RequestScreenshot,
		RequestScreenshotOptions: g.RequestScreenshotOptions,
		StartRecording:           g.StartRecording,
		StopRecording:            g.StopRecording,
		UIApp:                    g.uiApp,
		UIManager:                g.ui,
		MobileSettingsHost:       g,
		UISettingsHost:           g,
		ControllerSettingsHost:   g,
	}
}
