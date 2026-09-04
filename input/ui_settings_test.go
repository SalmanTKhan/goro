package input

import "testing"

func TestUISettingsPresets(t *testing.T) {
	for _, tt := range []struct {
		name, label string
		value, next float32
	}{
		{"small", "Small", UIScaleSmallValue, UIScaleDefaultValue},
		{"default", "Default", UIScaleDefaultValue, UIScaleLargeValue},
		{"large", "Large", UIScaleLargeValue, UIScaleSmallValue},
	} {
		t.Run(tt.name, func(t *testing.T) {
			settings := UISettings{Scale: tt.value}
			if got := settings.Label(); got != tt.label {
				t.Fatalf("Label() = %q, want %q", got, tt.label)
			}
			if got := settings.NextPreset().Scale; got != tt.next {
				t.Fatalf("NextPreset().Scale = %v, want %v", got, tt.next)
			}
		})
	}
}

func TestUISettingsNormalizesInvalidAndRetainsFineGrainedValues(t *testing.T) {
	if got := (UISettings{Scale: 0}).Normalized().Scale; got != UIScaleDefaultValue {
		t.Fatalf("invalid scale = %v, want default", got)
	}
	if got := (UISettings{Scale: 1.07}).Normalized().Scale; got != 1.07 {
		t.Fatalf("custom scale = %v, want 1.07", got)
	}
	if got := (UISettings{Scale: 1.07}).Label(); got != "Custom" {
		t.Fatalf("custom label = %q, want Custom", got)
	}
}
