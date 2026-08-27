package app

import (
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
)

// MobileSettings returns the live values used by the mobile presentation.
// Audio and gameplay values come from their existing runtime owners so the
// settings page never displays stale configuration-file placeholders.
func (g *Game) MobileSettings() input.MobileSettings {
	if g == nil {
		return input.DefaultMobileSettings()
	}
	settings := input.MobileSettings{
		Controls: g.cfg.Mobile,
		Audio: input.MobileAudioSettings{
			BGMEnabled: g.cfg.Audio.BGM,
			BGMVolume:  g.cfg.Audio.BGMVolume,
			SFXVolume:  g.cfg.Audio.SFXVolume,
		},
		Display: input.MobileDisplaySettings{ShowMinimap: g.cfg.MobileDisplay.ShowMinimap},
		Gameplay: input.MobileGameplaySettings{
			NoShift:     g.cfg.Gameplay.NoShift,
			NoCtrl:      g.cfg.Gameplay.NoCtrl,
			LessEffects: g.cfg.Gameplay.LessEffects,
			SnapTargets: g.cfg.Gameplay.SnapTargets,
			SnapItems:   g.cfg.Gameplay.SnapItems,
		},
	}
	return settings.Normalized()
}

// ApplyMobileSettings updates all supported mobile settings at runtime. The
// Android presentation owns persistence, while this method owns the live
// audio, session, and renderer-facing state.
func (g *Game) ApplyMobileSettings(settings input.MobileSettings) bool {
	if g == nil {
		return false
	}
	settings = settings.Normalized()
	g.cfg.Mobile = settings.Controls
	g.cfg.MobileDisplay = settings.Display
	g.cfg.Audio.BGM = settings.Audio.BGMEnabled
	g.cfg.Audio.BGMVolume = settings.Audio.BGMVolume
	g.cfg.Audio.SFXVolume = settings.Audio.SFXVolume
	g.cfg.Gameplay.NoShift = settings.Gameplay.NoShift
	g.cfg.Gameplay.NoCtrl = settings.Gameplay.NoCtrl
	g.cfg.Gameplay.LessEffects = settings.Gameplay.LessEffects
	g.cfg.Gameplay.SnapTargets = settings.Gameplay.SnapTargets
	g.cfg.Gameplay.SnapItems = settings.Gameplay.SnapItems
	if g.runtime != nil {
		g.runtime.SetFPS(g.cfg.Render.FPS)
	}
	if g.audio != nil {
		wasEnabled := g.audio.Enabled()
		g.audio.SetEnabled(settings.Audio.BGMEnabled)
		g.audio.SetBGMVolume(settings.Audio.BGMVolume)
		g.audio.SetSFXVolume(settings.Audio.SFXVolume)
		if settings.Audio.BGMEnabled && !wasEnabled && g.world != nil {
			_, _ = g.audio.PlayMap(g.world.MapName)
		}
	}
	if g.session != nil {
		g.session.NoShift = settings.Gameplay.NoShift
		g.session.NoCtrl = settings.Gameplay.NoCtrl
		g.session.LessEffects = settings.Gameplay.LessEffects
		g.session.SnapTargets = settings.Gameplay.SnapTargets
		g.session.SnapItems = settings.Gameplay.SnapItems
	}
	if g.network != nil {
		_ = g.network.SendLessEffect(settings.Gameplay.LessEffects)
	}
	return true
}

// MobileSettingsSurface is kept as a small app-level seam for UI hosts that
// want to render the same model without depending on the Android package.
func (g *Game) MobileSettingsSurface() mobileui.SurfaceModel {
	return mobileui.SettingsSurfaceForSettings(g.MobileSettings())
}
