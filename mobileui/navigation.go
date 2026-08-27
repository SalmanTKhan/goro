package mobileui

import "github.com/kivutar/goro/input"

type Screen uint8

const (
	ScreenWorldHUD Screen = iota
	ScreenCharacter
	ScreenInventory
	ScreenSkills
	ScreenMap
	ScreenSettings
	ScreenEquipment
	ScreenSocial
	ScreenTrade
	ScreenVending
	ScreenProfile
)

func (s Screen) String() string {
	switch s {
	case ScreenCharacter:
		return "Character"
	case ScreenInventory:
		return "Inventory"
	case ScreenSkills:
		return "Skills"
	case ScreenMap:
		return "Map"
	case ScreenSettings:
		return "Settings"
	case ScreenEquipment:
		return "Equipment"
	case ScreenSocial:
		return "Social"
	case ScreenTrade:
		return "Trade"
	case ScreenVending:
		return "Vending"
	case ScreenProfile:
		return "Profile"
	default:
		return "World"
	}
}

type Navigation struct {
	Screen    Screen
	MenuOpen  bool
	Targeting input.SkillTargetState
	SkillPage int
	Stack     []NavigationEntry
}

type NavigationLayer uint8

const (
	NavigationFullScreen NavigationLayer = iota
	NavigationDetailSheet
	NavigationConfirmation
	NavigationTransient
)

type NavigationEntry struct {
	Screen Screen
	Layer  NavigationLayer
	ID     string
}

func (n *Navigation) Open(screen Screen) {
	if n == nil {
		return
	}
	n.Screen, n.MenuOpen = screen, false
	n.Stack = []NavigationEntry{{Screen: screen, Layer: NavigationFullScreen}}
}

func (n *Navigation) OpenLayer(screen Screen, layer NavigationLayer, id string) {
	if n == nil {
		return
	}
	if len(n.Stack) == 0 && layer != NavigationFullScreen {
		n.Stack = append(n.Stack, NavigationEntry{Screen: screen, Layer: NavigationFullScreen})
	}
	if len(n.Stack) == 0 || n.Stack[len(n.Stack)-1].Screen != screen || n.Stack[len(n.Stack)-1].Layer != layer || n.Stack[len(n.Stack)-1].ID != id {
		n.Stack = append(n.Stack, NavigationEntry{Screen: screen, Layer: layer, ID: id})
	}
	n.Screen, n.MenuOpen = screen, false
}

func (n *Navigation) Top() NavigationEntry {
	if n == nil || len(n.Stack) == 0 {
		return NavigationEntry{Screen: ScreenWorldHUD, Layer: NavigationFullScreen}
	}
	return n.Stack[len(n.Stack)-1]
}

func (n *Navigation) HasLayer(layer NavigationLayer) bool {
	return n != nil && n.Top().Layer == layer
}

func (n *Navigation) Back() bool {
	if n == nil {
		return false
	}
	if n.Targeting.Mode != input.SkillTargetIdle {
		n.Targeting.Cancel()
		return true
	}
	if n.MenuOpen {
		n.MenuOpen = false
		return true
	}
	if len(n.Stack) > 1 {
		n.Stack = n.Stack[:len(n.Stack)-1]
		top := n.Top()
		n.Screen = top.Screen
		return true
	}
	if n.Screen != ScreenWorldHUD {
		n.Screen = ScreenWorldHUD
		n.Stack = []NavigationEntry{{Screen: ScreenWorldHUD, Layer: NavigationFullScreen}}
		return true
	}
	if len(n.Stack) == 0 {
		n.Stack = []NavigationEntry{{Screen: ScreenWorldHUD, Layer: NavigationFullScreen}}
	}
	return false
}
