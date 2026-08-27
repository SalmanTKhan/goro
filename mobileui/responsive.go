package mobileui

// LayoutProfile describes the composition space available to a mobile
// surface.  The profile is deliberately based on the safe viewport rather
// than Android device names so previews and future mobile hosts use the same
// geometry decisions.
type LayoutProfile uint8

const (
	LayoutPortrait LayoutProfile = iota
	LayoutLandscape
	LayoutFoldLandscape
)

func (p LayoutProfile) String() string {
	switch p {
	case LayoutLandscape:
		return "landscape"
	case LayoutFoldLandscape:
		return "fold-landscape"
	default:
		return "portrait"
	}
}

func (v Viewport) Profile() LayoutProfile {
	safe := v.SafeRect()
	if safe.W <= 0 || safe.H <= 0 || safe.W <= safe.H {
		return LayoutPortrait
	}
	if safe.H <= 900 && safe.W/safe.H >= 2.0 {
		return LayoutFoldLandscape
	}
	return LayoutLandscape
}

func (v Viewport) IsPortrait() bool { return v.Profile() == LayoutPortrait }

// UsesStackedCards reports whether a surface should use the mobile-first
// vertical card composition. A normal phone in landscape still has a short
// vertical viewport, so a side-by-side desktop pane wastes the scarce height
// and makes the content feel like a scaled desktop window. The Fold outer
// profile is the wide-screen exception where split panes remain readable.
func (v Viewport) UsesStackedCards() bool { return v.IsPortrait() }

// ScrollState is shared by every vertically scrolling mobile surface.  The
// old InventoryScrollState name remains an alias for source compatibility.
type ScrollState struct {
	ViewportExtent float32
	ContentExtent  float32
	Offset         float32
	RowExtent      float32
}

func (s *ScrollState) SetOffset(offset float32) {
	if s == nil {
		return
	}
	s.Offset = clampf(offset, 0, s.MaxOffset())
}

func (s *ScrollState) ScrollBy(delta float32) {
	if s != nil {
		s.SetOffset(s.Offset + delta)
	}
}

func (s ScrollState) MaxOffset() float32 { return maxf(0, s.ContentExtent-s.ViewportExtent) }

func (s ScrollState) Page() int {
	if s.RowExtent <= 0 {
		return 0
	}
	return int(s.Offset / s.RowExtent)
}

func centeredMobileRail(safe Rect, maxWidth, edge float32) Rect {
	width := maxf(0, safe.W-2*edge)
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	return Rect{X: safe.X + (safe.W-width)/2, Y: safe.Y + edge, W: width, H: maxf(0, safe.H-2*edge)}
}
