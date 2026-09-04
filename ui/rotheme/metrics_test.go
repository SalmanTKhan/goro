package rotheme

import (
	"reflect"
	"testing"
)

// TestDesktopMetricsMatchConstants pins DesktopMetrics to the exported
// constants. Several call sites outside this package fold those constants into
// their own constant expressions, so the constants cannot become variables;
// this test is what keeps the two representations from drifting apart.
func TestDesktopMetricsMatchConstants(t *testing.T) {
	m := DesktopMetrics()
	for _, tc := range []struct {
		name string
		got  float32
		want float32
	}{
		{"ButtonRadius", m.ButtonRadius, ButtonRadius},
		{"ButtonPaddingX", m.ButtonPaddingX, ButtonPaddingX},
		{"ButtonPaddingY", m.ButtonPaddingY, ButtonPaddingY},
		{"LargeButtonPaddingY", m.LargeButtonPaddingY, LargeButtonPaddingY},
		{"IconButtonSize", m.IconButtonSize, IconButtonSize},
		{"CheckboxSize", m.CheckboxSize, CheckboxSize},
		{"CheckboxRadius", m.CheckboxRadius, CheckboxRadius},
		{"CheckboxGap", m.CheckboxGap, CheckboxGap},
		{"RadioSize", m.RadioSize, RadioSize},
		{"RadioGap", m.RadioGap, RadioGap},
		{"SliderTrackHeight", m.SliderTrackHeight, SliderTrackHeight},
		{"SliderThumbSize", m.SliderThumbSize, SliderThumbSize},
		{"TableRowHeight", m.TableRowHeight, TableRowHeight},
		{"TableGap", m.TableGap, TableGap},
		{"TableCellPadX", m.TableCellPadX, TableCellPadX},
		{"TableHeaderPadY", m.TableHeaderPadY, TableHeaderPadY},
		{"SelectListRowPadX", m.SelectListRowPadX, SelectListRowPadX},
		{"ContextMenuItemHeight", m.ContextMenuItemHeight, ContextMenuItemHeight},
	} {
		if tc.got != tc.want {
			t.Errorf("DesktopMetrics().%s = %v, want the %s constant %v", tc.name, tc.got, tc.name, tc.want)
		}
	}
}

// TestDefaultThemeUsesDesktopMetrics guards the wiring: a Theme with a zero
// Metrics would silently paint everything at size zero.
func TestDefaultThemeUsesDesktopMetrics(t *testing.T) {
	if Default.Metrics != DesktopMetrics() {
		t.Errorf("Default.Metrics = %+v, want DesktopMetrics()", Default.Metrics)
	}
	if Mobile.Metrics != MobileMetrics() {
		t.Errorf("Mobile.Metrics = %+v, want MobileMetrics()", Mobile.Metrics)
	}
}

// TestNoMetricIsZero catches a field added to Metrics but forgotten in one of
// the presets. MinTouchTarget is exempt: desktop deliberately leaves it unset.
func TestNoMetricIsZero(t *testing.T) {
	for _, preset := range []struct {
		name string
		m    Metrics
	}{
		{"DesktopMetrics", DesktopMetrics()},
		{"MobileMetrics", MobileMetrics()},
	} {
		v := reflect.ValueOf(preset.m)
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			if name == "MinTouchTarget" && preset.name == "DesktopMetrics" {
				continue
			}
			if v.Field(i).Float() == 0 {
				t.Errorf("%s().%s is zero — new field left unset?", preset.name, name)
			}
		}
	}
}

// TestMobileMetricsMeetTouchTarget is the reason the mobile variant exists: a
// 17px close button and a 14px table row are not hittable with a finger.
func TestMobileMetricsMeetTouchTarget(t *testing.T) {
	m := MobileMetrics()
	min := m.MinTouchTarget
	if min < 48 {
		t.Fatalf("MinTouchTarget = %v, want at least 48", min)
	}
	for _, tc := range []struct {
		name string
		got  float32
	}{
		{"IconButtonSize", m.IconButtonSize},
		{"TableRowHeight", m.TableRowHeight},
		{"ContextMenuItemHeight", m.ContextMenuItemHeight},
		{"WindowTitleHeight", m.WindowTitleHeight},
	} {
		if tc.got < min {
			t.Errorf("MobileMetrics().%s = %v, below the %v minimum touch target", tc.name, tc.got, min)
		}
	}
	// A button's height is text plus padding on both sides; it has to clear the
	// same bar even though no single metric states it.
	if h := Mobile.Typography.TextSize + 2*m.ButtonPaddingY; h < min {
		t.Errorf("mobile button height = %v, below the %v minimum touch target", h, min)
	}
}

// TestMobileKeepsDesktopPalette states the intent explicitly: the mobile
// presentation is the same game, so only geometry and text size may differ.
func TestMobileKeepsDesktopPalette(t *testing.T) {
	if Mobile.Colors != Default.Colors {
		t.Error("Mobile must reuse the Default palette; only metrics and text size may differ")
	}
	if Mobile.Typography.TextSize <= Default.Typography.TextSize {
		t.Errorf("Mobile text size %v should exceed desktop %v", Mobile.Typography.TextSize, Default.Typography.TextSize)
	}
}
