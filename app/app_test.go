package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/res"
	"github.com/kivutar/goro/session"
)

func TestNewForceUserAIEnablesCompanionCustomAI(t *testing.T) {
	g, err := New(config.Config{
		DataDir: t.TempDir(),
		Window: config.WindowConfig{
			Width:  1280,
			Height: 720,
		},
		Packet: config.PacketConfig{
			ClientDate: 20080910,
		},
		Audio: config.AudioConfig{
			BGMVolume: 0.55,
			SFXVolume: 0.55,
		},
		Render: config.RenderConfig{
			GraphicsAPI: "vulkan",
			VSync:       true,
		},
		Gameplay: config.GameplayConfig{
			ForceUserAI: true,
		},
		Log: config.LogConfig{
			Level: "info",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !g.session.HomunculusCustomAI || !g.session.MercenaryCustomAI {
		t.Fatalf("custom AI flags = homunculus:%v mercenary:%v, want both true", g.session.HomunculusCustomAI, g.session.MercenaryCustomAI)
	}
}

func TestNewOnlineUsesClientDateForAccountLoginVersion(t *testing.T) {
	g, err := New(config.Config{
		DataDir: t.TempDir(),
		Window:  config.WindowConfig{Width: 1280, Height: 720},
		Packet:  config.PacketConfig{ClientDate: 20211103},
		MobileSession: config.MobileSessionConfig{
			Mode: config.SessionModeOnline,
			Server: config.ServerConfig{
				Name:       "Sabine",
				Host:       "127.0.0.1",
				AuthPort:   6900,
				ClientDate: 20080910,
				Profile:    23,
			},
		},
		Audio:  config.AudioConfig{BGMVolume: 0.55, SFXVolume: 0.55},
		Render: config.RenderConfig{GraphicsAPI: "vulkan", VSync: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.resource.ClientInfo.Connections) != 1 {
		t.Fatalf("online connections = %d, want 1", len(g.resource.ClientInfo.Connections))
	}
	conn := g.resource.ClientInfo.Connections[0]
	if conn.Version != 20080910 {
		t.Fatalf("account login version = %d, want client date 20080910", conn.Version)
	}
}

func TestApplyMobileSettingsUpdatesLiveOwners(t *testing.T) {
	g, err := New(config.Config{
		DataDir: t.TempDir(),
		Window:  config.WindowConfig{Width: 1280, Height: 720},
		Packet:  config.PacketConfig{ClientDate: 20080910},
		Audio:   config.AudioConfig{BGM: true, BGMVolume: 0.55, SFXVolume: 0.55},
		Render:  config.RenderConfig{GraphicsAPI: "vulkan", VSync: true},
		Gameplay: config.GameplayConfig{
			NoCtrl: true,
		},
		Log: config.LogConfig{Level: "info"},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings := input.DefaultMobileSettings()
	settings.Controls.MovementMode = input.MovementTapToMove
	settings.Audio.BGMEnabled = false
	settings.Audio.BGMVolume = 0.25
	settings.Audio.SFXVolume = 0.75
	settings.Display.ShowMinimap = false
	settings.Gameplay.NoCtrl = false
	if !g.ApplyMobileSettings(settings) {
		t.Fatal("mobile settings were not accepted")
	}
	if got := g.MobileSettings(); got != settings {
		t.Fatalf("live mobile settings = %#v, want %#v", got, settings)
	}
	if g.audio.BGMVolume() != 0.25 || g.audio.SFXVolume() != 0.75 || g.audio.Enabled() {
		t.Fatalf("live audio = enabled:%t bgm:%v sfx:%v", g.audio.Enabled(), g.audio.BGMVolume(), g.audio.SFXVolume())
	}
	if g.session.NoCtrl {
		t.Fatal("No Ctrl gameplay setting was not applied")
	}
}

func TestNewOfflineLoadsValidContentPack(t *testing.T) {
	dataDir := t.TempDir()
	writeOfflineTestContent(t, dataDir, session.OfflineContent{
		Format: session.OfflineContentFormat, Version: session.OfflineContentVersion,
		Source: session.OfflineContentSource{Fingerprint: "startup-test", Packetver: 20080910},
		Maps:   map[string]session.OfflineMap{"prontera": {Name: "prontera"}},
		Items: map[uint16]session.OfflineItem{
			501:  {ID: 501, Name: "Red Potion", Type: db.ItemTypeHealing, UseSupported: true, UseEffect: session.OfflineUseEffect{HP: 45}},
			909:  {ID: 909, Name: "Jellopy", Type: db.ItemTypeEtc},
			1201: {ID: 1201, Name: "Knife", Type: db.ItemTypeWeapon},
		},
		Skills: map[uint16]session.OfflineSkillDef{db.SkillSMBash: {ID: db.SkillSMBash, Name: "Bash", MaxLevel: 10, Range: 6, SPCost: 8}},
	})
	g, err := NewOffline(testOfflineConfig(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	if g.Offline() == nil || g.Offline().ContentFingerprint() != "startup-test" {
		t.Fatalf("offline content was not loaded: %+v", g.Offline())
	}
	if g.session.Inventory.Weight > g.session.Inventory.MaxWeight {
		t.Fatalf("offline starter loadout is overweight: weight=%d max=%d", g.session.Inventory.Weight, g.session.Inventory.MaxWeight)
	}
}

func TestNewOfflineRejectsMissingAndMalformedContentPack(t *testing.T) {
	if _, err := NewOffline(testOfflineConfig(t.TempDir())); err == nil {
		t.Fatal("missing offline content was accepted")
	}
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "offline"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "offline", "content.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewOffline(testOfflineConfig(dataDir)); err == nil {
		t.Fatal("malformed offline content was accepted")
	}
}

func TestNewOfflineRejectsStaleContentPack(t *testing.T) {
	dataDir := t.TempDir()
	writeOfflineTestContent(t, dataDir, session.OfflineContent{
		Format: session.OfflineContentFormat, Version: session.OfflineContentVersion,
		Source: session.OfflineContentSource{Fingerprint: "stale", Packetver: 20080909},
		Maps:   map[string]session.OfflineMap{"prontera": {Name: "prontera"}},
		Items:  map[uint16]session.OfflineItem{501: {}, 909: {}, 1201: {}},
		Skills: map[uint16]session.OfflineSkillDef{db.SkillSMBash: {ID: db.SkillSMBash, Name: "Bash", MaxLevel: 10}},
	})
	if _, err := NewOffline(testOfflineConfig(dataDir)); err == nil {
		t.Fatal("stale offline content was accepted")
	}
}

func testOfflineConfig(dataDir string) config.Config {
	return config.Config{DataDir: dataDir, Window: config.WindowConfig{Width: 1280, Height: 720}, Packet: config.PacketConfig{ClientDate: 20080910}, Audio: config.AudioConfig{BGMVolume: 0.55, SFXVolume: 0.55}, Render: config.RenderConfig{GraphicsAPI: "vulkan", VSync: true}, Log: config.LogConfig{Level: "info"}}
}

func writeOfflineTestContent(t *testing.T, dataDir string, content session.OfflineContent) {
	t.Helper()
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dataDir, "offline", "content.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMobileMinimapRasterProjectsGNDCells(t *testing.T) {
	gnd := &res.GND{
		Width:  3,
		Height: 2,
		Cells: []res.GNDCell{
			{Top: 0}, {Top: -1, Front: 0}, {Top: -1, Right: 0},
			{Top: -1, Front: -1, Right: -1}, {Top: 1}, {Top: -1, Front: 1},
		},
	}
	raster := mobileMinimapRaster(gnd)
	if raster.Width != 3 || raster.Height != 2 {
		t.Fatalf("raster dimensions = %dx%d, want 3x2", raster.Width, raster.Height)
	}
	want := []uint8{1, 2, 2, 0, 1, 2}
	for i, value := range want {
		if raster.Cells[i] != value {
			t.Fatalf("raster cell %d = %d, want %d", i, raster.Cells[i], value)
		}
	}
}
