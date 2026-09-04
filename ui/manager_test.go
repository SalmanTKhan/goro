package ui

import (
	"testing"

	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/uitest"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/ui/rotheme"
)

type countingOverlay struct {
	widget.WidgetBase
	events int
	draws  int
}

func newCountingOverlay() *countingOverlay {
	w := &countingOverlay{}
	w.SetVisible(true)
	w.SetEnabled(true)
	return w
}

func (w *countingOverlay) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.BiggestFinite(1, 1)
	w.SetBounds(geometry.FromPointSize(w.Position(), size))
	return size
}

func (w *countingOverlay) Draw(widget.Context, widget.Canvas) { w.draws++ }

func (w *countingOverlay) Event(widget.Context, event.Event) bool {
	w.events++
	return false
}

func (w *countingOverlay) Children() []widget.Widget { return nil }

func TestOverlayRootReportsWhetherItIsEmpty(t *testing.T) {
	if root := newOverlayRoot(nil); !root.IsUIRootEmpty() {
		t.Fatal("root without overlays did not report itself empty")
	}
	if root := newOverlayRoot([]widget.Widget{newCountingOverlay()}); root.IsUIRootEmpty() {
		t.Fatal("root with an overlay reported itself empty")
	}
}

func TestManagerPointerBlockedUsesOverlayBounds(t *testing.T) {
	app := uiapp.New()
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	manager.AddOverlay(positionedWidget(newInertOverlay(), 100, 120, 80, 40))
	app.Frame()
	app.Window().DrawTo(&uitest.MockCanvas{})

	if !manager.PointerBlocked(120, 130) {
		t.Fatal("pointer over overlay was not blocked")
	}
	if manager.PointerBlocked(90, 130) {
		t.Fatal("pointer outside overlay was blocked")
	}
}

func TestTopOverlayBlocksLowerOverlayEvents(t *testing.T) {
	app := uiapp.New()
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	lower := newCountingOverlay()
	top := newCountingOverlay()
	manager.AddOverlay(positionedWidget(lower, 100, 100, 80, 80))
	manager.AddOverlay(positionedWidget(top, 120, 120, 80, 80))
	app.Frame()
	app.Window().DrawTo(&uitest.MockCanvas{})

	point := geometry.Pt(130, 130)
	app.Window().HandleEvent(event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, point, point, event.ModNone))
	if top.events != 1 {
		t.Fatalf("top overlay events = %d, want 1", top.events)
	}
	if lower.events != 0 {
		t.Fatalf("lower overlay events = %d, want 0", lower.events)
	}
}

func TestClickingWindowRaisesItAboveOtherWindows(t *testing.T) {
	manager := NewManager()
	ctx := client.Context{UIManager: manager}
	lowerContent := newCountingOverlay()
	topContent := newCountingOverlay()
	lower := NewWindow(100, 80)
	top := NewWindow(100, 80)
	lower.OpenAt(100, 100, lowerContent)
	top.OpenAt(150, 100, topContent)
	lower.Publish(ctx)
	top.Publish(ctx)
	root := manager.root
	root.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(300, 240)))
	widget.ClearRedrawInTree(root)

	lowerOnly := geometry.Pt(125, 125)
	root.Event(widget.NewContext(), event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, lowerOnly, lowerOnly, event.ModNone))
	if manager.root != root {
		t.Fatal("raising a window replaced the UI root")
	}
	if got := manager.overlays[len(manager.overlays)-1]; got != lower.published {
		t.Fatal("clicked window was not raised in the manager order")
	}
	if got := root.children[len(root.children)-1]; got != lower.published {
		t.Fatal("clicked window was not raised in the active root")
	}
	if !root.NeedsRedraw() {
		t.Fatal("raising a window did not invalidate the overlay root")
	}

	overlap := geometry.Pt(175, 125)
	root.Event(widget.NewContext(), event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, overlap, overlap, event.ModNone))
	if lowerContent.events != 2 {
		t.Fatalf("raised window events = %d, want 2", lowerContent.events)
	}
	if topContent.events != 0 {
		t.Fatalf("covered window events = %d, want 0", topContent.events)
	}
}

func TestClickingPlainOverlayDoesNotRaiseIt(t *testing.T) {
	manager := NewManager()
	lower := positionedWidget(newCountingOverlay(), 100, 100, 100, 80)
	top := positionedWidget(newCountingOverlay(), 150, 100, 100, 80)
	manager.AddOverlay(lower)
	manager.AddOverlay(top)
	root := manager.root
	root.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(300, 240)))

	point := geometry.Pt(125, 125)
	root.Event(widget.NewContext(), event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, point, point, event.ModNone))
	if got := manager.overlays[len(manager.overlays)-1]; got != top {
		t.Fatal("plain overlay changed stacking order after a click")
	}
}

func TestForegroundOverlayStaysAboveNewWindows(t *testing.T) {
	manager := NewManager()
	foregroundContent := newCountingOverlay()
	foreground := positionedWidget(foregroundContent, 100, 100, 80, 80)
	lower := positionedWidget(newCountingOverlay(), 100, 100, 80, 80)
	window := positionedWidget(newCountingOverlay(), 100, 100, 80, 80)

	manager.AddForegroundOverlay(foreground)
	manager.AddOverlay(lower)
	manager.AddOverlay(window)
	manager.RaiseOverlay(lower)
	if len(manager.overlays) != 3 || manager.overlays[0] != window || manager.overlays[1] != lower || manager.overlays[2] != foreground {
		t.Fatalf("overlay order = %+v", manager.overlays)
	}

	manager.root.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(300, 240)))
	point := geometry.Pt(120, 120)
	manager.root.Event(widget.NewContext(), event.NewMouseEvent(event.MousePress, event.ButtonLeft, 0, point, point, event.ModNone))
	if foregroundContent.events != 1 {
		t.Fatalf("foreground events = %d, want 1", foregroundContent.events)
	}
}

func TestOverlayRootDrawSkipsChildrenOutsideClip(t *testing.T) {
	left := newCountingOverlay()
	right := newCountingOverlay()
	root := newOverlayRoot([]widget.Widget{
		positionedWidget(left, 10, 20, 40, 30),
		positionedWidget(right, 200, 20, 40, 30),
	})
	root.Layout(nil, geometry.Tight(geometry.Sz(300, 100)))

	canvas := clippedOverlayCanvas{
		MockCanvas: uitest.MockCanvas{},
		clip:       geometry.NewRect(190, 0, 80, 80),
	}
	root.Draw(nil, &canvas)

	if left.draws != 0 {
		t.Fatalf("left overlay draws = %d, want 0", left.draws)
	}
	if right.draws != 1 {
		t.Fatalf("right overlay draws = %d, want 1", right.draws)
	}
}

type clippedOverlayCanvas struct {
	uitest.MockCanvas
	clip geometry.Rect
}

func (c *clippedOverlayCanvas) ClipBounds() geometry.Rect {
	return c.clip
}

func TestManagerPointerBlockedAppliesPointerTransform(t *testing.T) {
	app := uiapp.New()
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	manager.AddOverlay(positionedWidget(newInertOverlay(), 100, 120, 80, 40))
	app.Frame()
	app.Window().DrawTo(&uitest.MockCanvas{})

	// Mirrors render's scaledUIEventSource.point for a 800x600 surface at 1.20
	// scale: the UI is scaled about the window center, so physical coordinates
	// must be converted before they are compared with logical widget bounds.
	const scale = 1.20
	const width, height = 800.0, 600.0
	manager.SetPointerTransform(func(x, y int) (int, int) {
		lx := (float64(x) - (width*(1-scale))/2) / scale
		ly := (float64(y) - (height*(1-scale))/2) / scale
		return int(lx + 0.5), int(ly + 0.5)
	})

	// The logical point 120,130 now lives at a different physical coordinate.
	physicalX := int(120*scale + (width*(1-scale))/2)
	physicalY := int(130*scale + (height*(1-scale))/2)
	if !manager.PointerBlocked(physicalX, physicalY) {
		t.Fatalf("scaled pointer at %d,%d was not blocked", physicalX, physicalY)
	}
	// Without the transform this same physical point would have missed the
	// overlay, which is the bug the transform exists to fix.
	manager.SetPointerTransform(nil)
	if manager.PointerBlocked(physicalX, physicalY) {
		t.Fatal("untransformed scaled pointer unexpectedly hit the overlay")
	}
}

func TestManagerControllerUIActiveFindsCharacterStyleFocusableOverlay(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	manager.AddOverlay(positionedWidget(
		primitives.Box(rotheme.Button("OK", nil)).Width(100).Height(40),
		100, 100, 100, 40,
	))
	app.Frame()

	if !manager.ControllerUIActive() {
		t.Fatal("focusable overlay was not reported as controller-active")
	}
}

func TestManagerControllerUIActiveIgnoresPassiveHUDOverlay(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	overlay := positionedWidget(
		primitives.Box(rotheme.Button("Rows", nil)).Width(100).Height(40),
		100, 100, 100, 40,
	).(*positionedOverlay)
	overlay.controllerPassthrough = true
	manager.AddOverlay(overlay)
	app.Frame()

	if manager.ControllerUIActive() {
		t.Fatal("passive HUD overlay incorrectly activated controller focus navigation")
	}
}

func TestManagerControllerUIActiveFindsOverlayBelowPassiveHUD(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	modal := positionedWidget(
		primitives.Box(rotheme.Button("Next", nil)).Width(100).Height(40),
		100, 100, 100, 40,
	)
	passiveHUD := positionedWidget(
		primitives.Box(rotheme.Button("Rows", nil)).Width(100).Height(40),
		100, 100, 100, 40,
	).(*positionedOverlay)
	passiveHUD.controllerPassthrough = true
	manager.AddOverlay(modal)
	manager.AddOverlay(passiveHUD)
	app.Frame()

	if !manager.ControllerUIActive() {
		t.Fatal("interactive overlay below passive HUD was not reported as controller-active")
	}
}

func TestManagerTextInputActiveUsesPredicate(t *testing.T) {
	manager := NewManager()
	if manager.TextInputActive() {
		t.Fatal("fresh manager reported text input active")
	}
	active := false
	manager.SetTextInputPredicate(func() bool { return active })
	if manager.TextInputActive() {
		t.Fatal("inactive predicate reported text input active")
	}
	active = true
	if !manager.TextInputActive() {
		t.Fatal("active predicate not honored")
	}
	if !manager.PointerOverUI(0, 0) {
		t.Fatal("text input must claim the pointer for the UI")
	}
}
