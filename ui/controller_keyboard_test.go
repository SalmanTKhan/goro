package ui

import (
	"testing"

	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
)

type controllerKeyboardTestTarget struct{ text string }

func (t *controllerKeyboardTestTarget) Text() string        { return t.text }
func (t *controllerKeyboardTestTarget) SetText(text string) { t.text = text }

func TestControllerKeyboardSupportsCaseSpaceAndBackspace(t *testing.T) {
	target := &controllerKeyboardTestTarget{}
	keyboard := &ControllerKeyboard{target: target}
	keyboard.insert("a")
	keyboard.shift = true
	keyboard.insert("b")
	keyboard.caps = true
	keyboard.insert("c")
	keyboard.insert(" ")
	keyboard.backspace()
	if target.text != "aBC" {
		t.Fatalf("keyboard text = %q, want %q", target.text, "aBC")
	}
}

func TestControllerKeyboardUsesTextFieldChangePath(t *testing.T) {
	changed := ""
	field := textfield.New(
		textfield.InitialValue("x"),
		textfield.OnChange(func(value string) { changed = value }),
	)
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))})
	keyboard := newControllerKeyboard(manager, field)

	keyboard.insert("a")
	if field.Text() != "xa" || changed != "xa" {
		t.Fatalf("controller insert text=%q callback=%q, want both %q", field.Text(), changed, "xa")
	}
	keyboard.backspace()
	if field.Text() != "x" || changed != "x" {
		t.Fatalf("controller backspace text=%q callback=%q, want both %q", field.Text(), changed, "x")
	}
}

func TestControllerKeyboardFirstFocusableSkipsNonFocusableNodes(t *testing.T) {
	keyboard := newControllerKeyboard(nil, nil)
	if keyboard.firstFocusable() == nil {
		t.Fatal("controller keyboard has no focusable key")
	}
}

func TestControllerKeyboardMouseClickActivatesKey(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	field := textfield.New()
	if !manager.OpenControllerKeyboard(field) {
		t.Fatal("failed to open controller keyboard")
	}
	app.Frame()

	first := manager.controllerKeyboard.firstFocusable()
	if first == nil {
		t.Fatal("controller keyboard has no first key")
	}
	firstFocus := first.(widget.Focusable)
	if focused := app.Window().FocusManager().Focused(); focused != firstFocus {
		t.Fatalf("controller keyboard focus = %T, want first key %T", focused, first)
	}
	if !firstFocus.IsFocused() {
		t.Fatal("first controller keyboard key is not marked focused")
	}
	// A real mouse event switches the UI out of controller mode before the
	// widget receives the click. Verify that the keyboard remains mouse-usable
	// in that mode as well.
	setControllerModeRecursive(manager.controllerKeyboardOverlay, false)
	bounds, ok := first.(interface{ Bounds() geometry.Rect })
	if !ok {
		t.Fatalf("first key %T has no bounds", first)
	}
	overlayBounds := manager.controllerKeyboard.Widget().(interface{ Bounds() geometry.Rect }).Bounds()
	point := overlayBounds.Min.Add(geometry.Pt(10+10, 28+10+10)).Add(bounds.Bounds().Center())
	app.HandleEvent(event.NewMouseEvent(event.MouseMove, event.ButtonNone, 0, point, point, event.ModNone))
	press := event.NewMouseEvent(event.MousePress, event.ButtonLeft, event.ButtonStateLeft, point, point, event.ModNone)
	release := event.NewMouseEvent(event.MouseRelease, event.ButtonLeft, 0, point, point, event.ModNone)
	app.HandleEvent(press)
	app.HandleEvent(release)

	if field.Text() != "1" {
		t.Fatalf("mouse activation text = %q, want %q", field.Text(), "1")
	}
}

func TestControllerKeyboardFocusLifecycleRestoresField(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	host := basicMenuTestApp{app: app}
	manager.SetUIApp(host)

	field := textfield.New()
	field.SetFocused(true)
	manager.AddOverlay(positionedWidget(
		Win(Content(primitives.Box(field).Width(220).Height(48))),
		0, 0, 240, 70,
	))
	app.Frame()

	if !manager.OpenControllerKeyboard(field) {
		t.Fatal("failed to open controller keyboard")
	}
	first := manager.controllerKeyboard.firstFocusable()
	if first == nil {
		t.Fatal("controller keyboard has no focusable key")
	}
	firstFocus := first.(widget.Focusable)
	if field.IsFocused() {
		t.Fatal("underlying text field stayed focused while keyboard was open")
	}
	if focused := app.Window().FocusManager().Focused(); focused != firstFocus {
		t.Fatalf("opened keyboard focus = %T, want first key %T", focused, first)
	}

	manager.CloseControllerKeyboard()
	if manager.ControllerKeyboardActive() {
		t.Fatal("controller keyboard remained active after close")
	}
	if !field.IsFocused() {
		t.Fatal("text field did not regain focus after keyboard close")
	}
	if focused := app.Window().FocusManager().Focused(); focused != field {
		t.Fatalf("restored focus = %T, want text field", focused)
	}
	if !manager.ControllerFocusNavigationActive() {
		t.Fatal("closing the keyboard did not enable controller focus navigation")
	}
}

func TestControllerFocusNavigationExpiresWhenDismissedFieldLeavesTree(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	field := textfield.New()
	field.SetFocused(true)
	overlay := positionedWidget(
		Win(Content(primitives.Box(field).Width(220).Height(48))),
		0, 0, 240, 70,
	)
	manager.AddOverlay(overlay)
	app.Frame()

	if !manager.OpenControllerKeyboard(field) {
		t.Fatal("failed to open controller keyboard")
	}
	manager.CloseControllerKeyboard()
	if !manager.ControllerFocusNavigationActive() {
		t.Fatal("keyboard close did not enable temporary focus navigation")
	}

	manager.RemoveOverlay(overlay)
	if manager.ControllerFocusNavigationActive() {
		t.Fatal("focus navigation remained active after its dismissed field left the UI tree")
	}
}

func TestControllerKeyboardDoneDismissesWithoutSubmittingField(t *testing.T) {
	submits := 0
	field := textfield.New(textfield.OnSubmit(func(string) { submits++ }))
	field.SetFocused(true)
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	manager.AddOverlay(positionedWidget(
		Win(Content(primitives.Box(field).Width(220).Height(48))),
		0, 0, 240, 70,
	))
	app.Frame()

	if !manager.OpenControllerKeyboard(field) {
		t.Fatal("failed to open controller keyboard")
	}
	manager.controllerKeyboard.submit()

	if manager.ControllerKeyboardActive() {
		t.Fatal("Done left the controller keyboard active")
	}
	if submits != 0 {
		t.Fatalf("Done submitted the underlying field %d time(s)", submits)
	}
	if manager.EnsureControllerKeyboard() {
		t.Fatal("Done immediately reopened the controller keyboard")
	}
	if !manager.OpenControllerKeyboard(field) {
		t.Fatal("explicit confirm could not reopen the controller keyboard")
	}
}
