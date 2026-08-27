package mobileui

// OnlineLoginPhase is the small, renderer-neutral state machine exposed by
// the mobile online front door. The network/login mode remains authoritative;
// this model only gives the mobile host a touch-safe projection of it.
type OnlineLoginPhase uint8

const (
	OnlineLoginAccount OnlineLoginPhase = iota
	OnlineLoginCharacters
	OnlineLoginCreate
)

type OnlineCharacterSlot struct {
	Slot     int
	Name     string
	JobName  string
	Level    int
	Occupied bool
}

type MobileOnlineLoginModel struct {
	Phase         OnlineLoginPhase
	Status        string
	Network       string
	Server        string
	Characters    []OnlineCharacterSlot
	SelectedSlot  int
	CanReconnect  bool
	CanDisconnect bool
	CanCreate     bool
	CanSwitchMode bool
	Notice        string
}

type OnlineLoginLayout struct {
	Safe, Panel, Title, Status, Network, Notice Rect
	Slots                                       []Rect
	Reconnect, Disconnect, Create, Mode         Rect
}

func LayoutOnlineLogin(viewport Viewport, model MobileOnlineLoginModel) OnlineLoginLayout {
	safe := viewport.SafeRect()
	layout := OnlineLoginLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	panelW := minf(1120, maxf(0, safe.W-32))
	panelH := maxf(0, safe.H-32)
	layout.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: safe.Y + (safe.H-panelH)/2, W: panelW, H: panelH}
	pad := float32(24)
	layout.Title = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + 28, W: layout.Panel.W - 2*pad, H: 56}
	layout.Status = Rect{X: layout.Panel.X + pad, Y: layout.Title.Bottom() + 12, W: layout.Panel.W - 2*pad, H: 52}
	layout.Network = Rect{X: layout.Panel.X + pad, Y: layout.Status.Bottom(), W: layout.Panel.W - 2*pad, H: 34}
	layout.Notice = Rect{X: layout.Panel.X + pad, Y: layout.Network.Bottom() + 6, W: layout.Panel.W - 2*pad, H: 48}

	if model.Phase == OnlineLoginCharacters {
		count := len(model.Characters)
		if count == 0 {
			count = 9
		}
		gridTop := layout.Notice.Bottom() + 18
		gridBottom := layout.Panel.Bottom() - 112
		gap := float32(12)
		cellW := (layout.Panel.W - 2*pad - 2*gap) / 3
		cellH := (gridBottom - gridTop - 2*gap) / 3
		cellH = minf(170, maxf(88, cellH))
		for i := 0; i < count && i < 9; i++ {
			layout.Slots = append(layout.Slots, Rect{
				X: layout.Panel.X + pad + float32(i%3)*(cellW+gap),
				Y: gridTop + float32(i/3)*(cellH+gap), W: cellW, H: cellH,
			})
		}
		layout.Create = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Bottom() - 84, W: (layout.Panel.W - 2*pad - 2*gap) / 3, H: 56}
	} else {
		buttonY := layout.Panel.Bottom() - 84
		buttonGap := float32(12)
		buttonW := (layout.Panel.W - 2*pad - buttonGap) / 2
		layout.Reconnect = Rect{X: layout.Panel.X + pad, Y: buttonY, W: buttonW, H: 56}
		layout.Disconnect = Rect{X: layout.Reconnect.Right() + buttonGap, Y: buttonY, W: buttonW, H: 56}
		layout.Mode = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Bottom() - 60, W: layout.Panel.W - 2*pad, H: 48}
	}
	return layout
}
