package input

import "math"

type UIScalePreset string

const (
	UIScaleSmall   UIScalePreset = "small"
	UIScaleDefault UIScalePreset = "default"
	UIScaleLarge   UIScalePreset = "large"
)

const (
	UIScaleSmallValue   float32 = 0.85
	UIScaleDefaultValue float32 = 1.0
	UIScaleLargeValue   float32 = 1.20
)

type UISettings struct{ Scale float32 }

func DefaultUISettings() UISettings { return UISettings{Scale: UIScaleDefaultValue} }

func (s UISettings) Normalized() UISettings {
	if math.IsNaN(float64(s.Scale)) || math.IsInf(float64(s.Scale), 0) || s.Scale < 0.75 || s.Scale > 1.5 {
		return DefaultUISettings()
	}
	return s
}

func (s UISettings) Preset() UIScalePreset {
	s = s.Normalized()
	switch {
	case math.Abs(float64(s.Scale-UIScaleSmallValue)) < 0.001:
		return UIScaleSmall
	case math.Abs(float64(s.Scale-UIScaleLargeValue)) < 0.001:
		return UIScaleLarge
	default:
		return UIScaleDefault
	}
}

func (s UISettings) Label() string {
	s = s.Normalized()
	switch s.Preset() {
	case UIScaleSmall:
		return "Small"
	case UIScaleLarge:
		return "Large"
	default:
		if math.Abs(float64(s.Scale-UIScaleDefaultValue)) < 0.001 {
			return "Default"
		}
		return "Custom"
	}
}

func (s UISettings) NextPreset() UISettings {
	switch s.Preset() {
	case UIScaleSmall:
		return UISettings{Scale: UIScaleDefaultValue}
	case UIScaleDefault:
		return UISettings{Scale: UIScaleLargeValue}
	default:
		return UISettings{Scale: UIScaleSmallValue}
	}
}
