package input

// MobileAudioSettings contains the audio preferences exposed by the mobile
// settings page. Volumes use the shared client range of 0..1.
type MobileAudioSettings struct {
	BGMEnabled bool
	BGMVolume  float64
	SFXVolume  float64
}

// MobileDisplaySettings contains mobile-only display preferences that have a
// real presentation effect. The target-name option remains part of
// MobileControls because it belongs to touch inspection.
type MobileDisplaySettings struct {
	ShowMinimap bool
}

// MobileGameplaySettings mirrors the existing desktop gameplay switches so
// the mobile page does not expose dead or mobile-specific shadow settings.
type MobileGameplaySettings struct {
	NoShift     bool
	NoCtrl      bool
	LessEffects bool
	SnapTargets bool
	SnapItems   bool
}

// MobileSettings is the complete persisted state edited by the mobile
// settings page. It contains no touch IDs, screen coordinates, or renderer
// objects.
type MobileSettings struct {
	Controls MobileControls
	Audio    MobileAudioSettings
	Display  MobileDisplaySettings
	Gameplay MobileGameplaySettings
}

func DefaultMobileSettings() MobileSettings {
	return MobileSettings{
		Controls: DefaultMobileControls(),
		Audio: MobileAudioSettings{
			BGMEnabled: true,
			BGMVolume:  0.55,
			SFXVolume:  0.55,
		},
		Display: MobileDisplaySettings{ShowMinimap: true},
		Gameplay: MobileGameplaySettings{
			NoCtrl: true,
		},
	}
}

func (s MobileSettings) Normalized() MobileSettings {
	defaults := DefaultMobileSettings()
	s.Controls = s.Controls.Normalized()
	s.Audio.BGMVolume = clampMobileVolume(s.Audio.BGMVolume, defaults.Audio.BGMVolume)
	s.Audio.SFXVolume = clampMobileVolume(s.Audio.SFXVolume, defaults.Audio.SFXVolume)
	return s
}

func clampMobileVolume(value, fallback float64) float64 {
	if value < 0 || value > 1 || value != value {
		return fallback
	}
	return value
}
