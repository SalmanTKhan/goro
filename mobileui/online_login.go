package mobileui

// OnlineLoginPhase is the renderer-neutral state machine exposed by the
// mobile online front door. Network/login authority remains in game.LoginMode.
type OnlineLoginPhase uint8

const (
	OnlineLoginServer OnlineLoginPhase = iota
	OnlineLoginCredentials
	OnlineLoginCharacterService
	OnlineLoginConnecting
	OnlineLoginCharacters
	OnlineLoginCreate
)

type OnlineServerOption struct {
	Index     int
	Name      string
	Detail    string
	UserCount int
	Selected  bool
}

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
	Servers       []OnlineServerOption
	Characters    []OnlineCharacterSlot
	SelectedSlot  int
	SelectedServer int
	Username      string
	PasswordSet   bool
	CreateName    string
	CreateSlot    int
	CanSubmit     bool
	CanReconnect  bool
	CanDisconnect bool
	CanCreate     bool
	CanSwitchMode bool
	Notice        string
}

type OnlineLoginLayout struct {
	Safe, Panel, Title, Status, Network, Notice Rect
	Options                                      []Rect
	Username, Password, Submit, Cancel           Rect
	Slots                                        []Rect
	Reconnect, Disconnect, Create, Mode          Rect
}

func LayoutOnlineLogin(viewport Viewport, model MobileOnlineLoginModel) OnlineLoginLayout {
	safe := viewport.SafeRect()
	layout := OnlineLoginLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}

	portrait := viewport.IsPortrait()
	panelW := minf(1120, maxf(0, safe.W-32))
	if !portrait {
		panelW = minf(920, maxf(0, safe.W*0.72))
	}
	panelH := maxf(0, safe.H-32)
	layout.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: safe.Y + (safe.H-panelH)/2, W: panelW, H: panelH}
	pad := float32(24)
	layout.Title = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + 20, W: layout.Panel.W - 2*pad, H: 48}
	layout.Status = Rect{X: layout.Panel.X + pad, Y: layout.Title.Bottom() + 6, W: layout.Panel.W - 2*pad, H: 42}
	layout.Network = Rect{X: layout.Panel.X + pad, Y: layout.Status.Bottom(), W: layout.Panel.W - 2*pad, H: 28}
	layout.Notice = Rect{X: layout.Panel.X + pad, Y: layout.Network.Bottom() + 4, W: layout.Panel.W - 2*pad, H: 42}

	switch model.Phase {
	case OnlineLoginServer, OnlineLoginCharacterService:
		top := layout.Notice.Bottom() + 12
		bottom := layout.Panel.Bottom() - 86
		rowGap := float32(8)
		rowH := float32(60)
		if portrait {
			rowH = 72
		}
		count := len(model.Servers)
		maxRows := int((bottom - top + rowGap) / (rowH + rowGap))
		if maxRows < 0 {
			maxRows = 0
		}
		if count > maxRows {
			count = maxRows
		}
		for i := 0; i < count; i++ {
			layout.Options = append(layout.Options, Rect{
				X: layout.Panel.X + pad,
				Y: top + float32(i)*(rowH+rowGap),
				W: layout.Panel.W - 2*pad,
				H: rowH,
			})
		}
		layout.Mode = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Bottom() - 62, W: layout.Panel.W - 2*pad, H: 44}

	case OnlineLoginCredentials:
		formW := minf(620, layout.Panel.W-2*pad)
		formX := layout.Panel.X + (layout.Panel.W-formW)/2
		top := layout.Notice.Bottom() + 22
		fieldH := float32(60)
		gap := float32(12)
		layout.Username = Rect{X: formX, Y: top, W: formW, H: fieldH}
		layout.Password = Rect{X: formX, Y: layout.Username.Bottom() + gap, W: formW, H: fieldH}
		layout.Submit = Rect{X: formX, Y: layout.Password.Bottom() + 18, W: formW, H: 60}
		layout.Mode = Rect{X: formX, Y: layout.Submit.Bottom() + 10, W: formW, H: 44}

	case OnlineLoginCharacters:
		count := len(model.Characters)
		if count == 0 {
			count = 9
		}
		gridTop := layout.Notice.Bottom() + 14
		gridBottom := layout.Panel.Bottom() - 94
		gap := float32(10)
		cellW := (layout.Panel.W - 2*pad - 2*gap) / 3
		cellH := (gridBottom - gridTop - 2*gap) / 3
		cellH = minf(154, maxf(76, cellH))
		for i := 0; i < count && i < 9; i++ {
			layout.Slots = append(layout.Slots, Rect{
				X: layout.Panel.X + pad + float32(i%3)*(cellW+gap),
				Y: gridTop + float32(i/3)*(cellH+gap), W: cellW, H: cellH,
			})
		}
		layout.Create = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Bottom() - 72, W: (layout.Panel.W - 2*pad - 2*gap) / 3, H: 52}

	case OnlineLoginCreate:
		formW := minf(620, layout.Panel.W-2*pad)
		formX := layout.Panel.X + (layout.Panel.W-formW)/2
		top := layout.Notice.Bottom() + 28
		layout.Username = Rect{X: formX, Y: top, W: formW, H: 60}
		buttonGap := float32(12)
		buttonW := (formW-buttonGap)/2
		layout.Submit = Rect{X: formX, Y: layout.Username.Bottom() + 18, W: buttonW, H: 60}
		layout.Cancel = Rect{X: layout.Submit.Right() + buttonGap, Y: layout.Submit.Y, W: buttonW, H: 60}
	case OnlineLoginConnecting:
		buttonY := layout.Panel.Bottom() - 74
		buttonGap := float32(12)
		buttonW := (layout.Panel.W - 2*pad - buttonGap) / 2
		layout.Reconnect = Rect{X: layout.Panel.X + pad, Y: buttonY, W: buttonW, H: 52}
		layout.Disconnect = Rect{X: layout.Reconnect.Right() + buttonGap, Y: buttonY, W: buttonW, H: 52}
	}
	return layout
}
