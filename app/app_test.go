package app

import (
	"testing"

	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/input"
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
		Log: glog.LogConfig{
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


func TestMobileSettingsApplyLiveRenderOptions(t *testing.T) {
	g := &Game{
		cfg: config.Config{
			Render: config.RenderConfig{VSync: true, FPS: false},
			MobileDisplay: input.MobileDisplaySettings{
				ShowMinimap: true, Presentation: input.MobilePresentationMobileUI,
			},
		},
		runtime: newRuntimeSettings(false, true, false),
	}
	settings := g.MobileSettings()
	if !settings.Display.VSync || settings.Display.FPS {
		t.Fatalf("initial live render settings=%+v", settings.Display)
	}

	settings.Display.VSync = false
	settings.Display.FPS = true
	settings.Display.Presentation = input.MobilePresentationDesktop
	if !g.ApplyMobileSettings(settings) {
		t.Fatal("ApplyMobileSettings returned false")
	}
	if g.RuntimeVSync() || !g.RuntimeFPS() {
		t.Fatalf("runtime render settings vsync=%t fps=%t", g.RuntimeVSync(), g.RuntimeFPS())
	}
	if g.cfg.Render.VSync || !g.cfg.Render.FPS || g.cfg.Render.NoUI {
		t.Fatalf("game render config not synchronized: %+v", g.cfg.Render)
	}

	settings.Display.Presentation = input.MobilePresentationMobileUI
	if !g.ApplyMobileSettings(settings) || !g.cfg.Render.NoUI {
		t.Fatalf("mobile presentation did not suppress desktop UI: %+v", g.cfg.Render)
	}
}
