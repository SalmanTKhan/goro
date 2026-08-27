package mobileui

type LayoutOption uint8

const (
	LayoutOptionStacked LayoutOption = iota
	LayoutOptionBottomTabs
)

func (o LayoutOption) String() string {
	if o == LayoutOptionBottomTabs {
		return "bottom-tabs"
	}
	return "stacked-cards"
}

type BottomTabRect struct {
	ID    string
	Label string
	Rect  Rect
}

type BottomTabShellLayout struct {
	Safe, Content, TabBar Rect
	Tabs                  []BottomTabRect
}

// LayoutBottomTabShell is the alternative approval candidate. It is kept
// separate from the active stacked-card route until visual approval promotes
// it, so option review cannot silently change production composition.
func LayoutBottomTabShell(viewport Viewport, labels []string) BottomTabShellLayout {
	safe := viewport.SafeRect()
	layout := BottomTabShellLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	tabHeight := float32(72)
	if safe.H < 500 {
		tabHeight = 64
	}
	layout.TabBar = Rect{X: safe.X, Y: safe.Bottom() - tabHeight, W: safe.W, H: tabHeight}
	layout.Content = Rect{X: safe.X, Y: safe.Y, W: safe.W, H: safe.H - tabHeight}
	if len(labels) == 0 {
		return layout
	}
	tabW := safe.W / float32(len(labels))
	for i, label := range labels {
		layout.Tabs = append(layout.Tabs, BottomTabRect{ID: label, Label: label, Rect: Rect{X: safe.X + float32(i)*tabW, Y: layout.TabBar.Y, W: tabW, H: tabHeight}})
	}
	return layout
}
