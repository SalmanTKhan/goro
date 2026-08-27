package mobileui

// StartupPhase is the platform-independent front-door flow for the client.
type StartupPhase uint8

const (
	StartupTitle StartupPhase = iota
	StartupProfile
	StartupWorld
)

type StartupModel struct{ Title, Subtitle, Action string }

type StartupLayout struct{ Safe, Panel, Logo, Subtitle, Action, AlternateAction Rect }

type StartupAction uint8

const (
	StartupNoAction StartupAction = iota
	StartupStartOffline
	StartupSwitchOnline
)

func LayoutStartup(v Viewport) StartupLayout {
	s := v.SafeRect()
	w := minf(720, s.W-32)
	h := minf(420, s.H-32)
	p := Rect{X: s.X + (s.W-w)/2, Y: s.Y + (s.H-h)/2, W: maxf(0, w), H: maxf(0, h)}
	return StartupLayout{
		Safe:            s,
		Panel:           p,
		Logo:            Rect{X: p.X + 24, Y: p.Y + 56, W: p.W - 48, H: 100},
		Subtitle:        Rect{X: p.X + 24, Y: p.Y + 174, W: p.W - 48, H: 52},
		Action:          Rect{X: p.X + 24, Y: p.Bottom() - 128, W: p.W - 48, H: 56},
		AlternateAction: Rect{X: p.X + 24, Y: p.Bottom() - 60, W: p.W - 48, H: 48},
	}
}

type StartupController struct {
	Viewport Viewport
	Model    StartupModel
	Phase    StartupPhase
	Layout   StartupLayout
}

func NewStartupController(v Viewport) *StartupController {
	c := &StartupController{Viewport: v, Model: StartupModel{Title: "GORO", Subtitle: "Offline Adventure", Action: "START"}}
	c.relayout()
	return c
}
func (c *StartupController) Resize(v Viewport) {
	if c != nil {
		c.Viewport = v
		c.relayout()
	}
}
func (c *StartupController) ActionAt(x, y float32) StartupAction {
	if c == nil || c.Phase != StartupTitle {
		return StartupNoAction
	}
	if c.Layout.Action.Contains(x, y) {
		return StartupStartOffline
	}
	if c.Layout.AlternateAction.Contains(x, y) {
		return StartupSwitchOnline
	}
	return StartupNoAction
}

func (c *StartupController) Tap(x, y float32) bool {
	if c.ActionAt(x, y) != StartupStartOffline {
		return c != nil && c.Phase == StartupTitle
	}
	c.Phase = StartupProfile
	return true
}
func (c *StartupController) EnterWorld() {
	if c != nil {
		c.Phase = StartupWorld
	}
}
func (c *StartupController) EnterTitle() {
	if c != nil {
		c.Phase = StartupTitle
	}
}
func (c *StartupController) Active() bool { return c != nil && c.Phase != StartupWorld }
func (c *StartupController) relayout()    { c.Layout = LayoutStartup(c.Viewport) }
