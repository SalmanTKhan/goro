package rotheme

// Metrics carries the widget geometry a theme paints with. The desktop values
// live in this package as exported constants because several call sites fold
// them into their own constant expressions (see ui/context_menu.go and
// ui/stats_window.go); Metrics mirrors them so a variant theme can supply a
// different set without those constants having to become variables.
//
// DesktopMetrics is pinned to the constants by TestDesktopMetricsMatchConstants.
type Metrics struct {
	ButtonRadius        float32
	ButtonPaddingX      float32
	ButtonPaddingY      float32
	LargeButtonPaddingY float32

	IconButtonSize float32

	CheckboxSize   float32
	CheckboxRadius float32
	CheckboxGap    float32

	RadioSize float32
	RadioGap  float32

	SliderTrackHeight float32
	SliderThumbSize   float32

	TableRowHeight  float32
	TableGap        float32
	TableCellPadX   float32
	TableHeaderPadY float32

	SelectListRowPadX     float32
	ContextMenuItemHeight float32

	// Window chrome. ui/window.go owns the desktop constants; a variant theme
	// supplies its own so the same window builder can produce touch-sized chrome.
	WindowTitleHeight  float32
	WindowFooterHeight float32

	// MinTouchTarget is the smallest interactive edge a theme promises. Desktop
	// leaves it at zero: a mouse does not need one.
	MinTouchTarget float32
}

// DesktopMetrics returns the mouse-sized geometry the desktop client has always
// used. Every value here mirrors the exported constant of the same name.
func DesktopMetrics() Metrics {
	return Metrics{
		ButtonRadius:        ButtonRadius,
		ButtonPaddingX:      ButtonPaddingX,
		ButtonPaddingY:      ButtonPaddingY,
		LargeButtonPaddingY: LargeButtonPaddingY,

		IconButtonSize: IconButtonSize,

		CheckboxSize:   CheckboxSize,
		CheckboxRadius: CheckboxRadius,
		CheckboxGap:    CheckboxGap,

		RadioSize: RadioSize,
		RadioGap:  RadioGap,

		SliderTrackHeight: SliderTrackHeight,
		SliderThumbSize:   SliderThumbSize,

		TableRowHeight:  TableRowHeight,
		TableGap:        TableGap,
		TableCellPadX:   TableCellPadX,
		TableHeaderPadY: TableHeaderPadY,

		SelectListRowPadX:     SelectListRowPadX,
		ContextMenuItemHeight: ContextMenuItemHeight,

		WindowTitleHeight:  28,
		WindowFooterHeight: 42,

		MinTouchTarget: 0,
	}
}

// MobileMetrics returns finger-sized geometry. The values are expressed in the
// same logical space the mobile viewport uses (mobileui.MobileTokens), where a
// touch target is 48 and body text is 28 — not in desktop points.
func MobileMetrics() Metrics {
	return Metrics{
		ButtonRadius:        8,
		ButtonPaddingX:      20,
		ButtonPaddingY:      14,
		LargeButtonPaddingY: 20,

		IconButtonSize: 48,

		CheckboxSize:   44,
		CheckboxRadius: 8,
		CheckboxGap:    16,

		RadioSize: 36,
		RadioGap:  14,

		SliderTrackHeight: 12,
		SliderThumbSize:   44,

		TableRowHeight:  56,
		TableGap:        2,
		TableCellPadX:   12,
		TableHeaderPadY: 12,

		SelectListRowPadX:     16,
		ContextMenuItemHeight: 52,

		WindowTitleHeight:  64,
		WindowFooterHeight: 84,

		MinTouchTarget: 48,
	}
}
