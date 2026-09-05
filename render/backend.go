package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
	gogputypes "github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gpucontext"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	uirender "github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
	"github.com/gogpu/wgpu"
	"github.com/kivutar/goro/capture"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/input/gamepad"
	"github.com/kivutar/goro/internal/appicon"
	"github.com/kivutar/goro/internal/buildinfo"
	"github.com/kivutar/goro/ui/rotheme"
)

const BackendName = "gogpu-wgpu"

// A VSync frame normally takes about 16.7ms. Logging every frame above that
// boundary turns normal scheduling jitter into a synchronous stderr write;
// when a window is minimized and the swapchain stops pacing, that can become
// an unbounded log/CPU spiral. Keep diagnostics for actual stalls only.
const slowFrameDiagnosticThreshold = 100 * time.Millisecond

type Game interface {
	Update() error
	Draw(*Frame)
	Resize(width, height int)
	InputState() *input.State
}

type uiScaleProvider interface {
	UISettings() input.UISettings
}

type scaledUIEventSource struct {
	source gpucontext.EventSource
	scale  func() float64
	width  func() int
	height func() int
}

func (s scaledUIEventSource) point(x, y float64) (float64, float64) {
	scale := s.scale()
	if scale <= 0 {
		scale = 1
	}
	// The UI surface is centered and scaled around the physical window center.
	// The framework still receives its normal logical surface coordinates.
	return (x - (float64(s.width())*(1-scale))/2) / scale, (y - (float64(s.height())*(1-scale))/2) / scale
}
func (s scaledUIEventSource) OnKeyPress(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	s.source.OnKeyPress(fn)
}
func (s scaledUIEventSource) OnKeyRelease(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	s.source.OnKeyRelease(fn)
}
func (s scaledUIEventSource) OnTextInput(fn func(string)) { s.source.OnTextInput(fn) }
func (s scaledUIEventSource) OnMouseMove(fn func(float64, float64)) {
	s.source.OnMouseMove(func(x, y float64) { fn(s.point(x, y)) })
}
func (s scaledUIEventSource) OnMousePress(fn func(gpucontext.MouseButton, float64, float64)) {
	s.source.OnMousePress(func(button gpucontext.MouseButton, x, y float64) { lx, ly := s.point(x, y); fn(button, lx, ly) })
}
func (s scaledUIEventSource) OnMouseRelease(fn func(gpucontext.MouseButton, float64, float64)) {
	s.source.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) { lx, ly := s.point(x, y); fn(button, lx, ly) })
}
func (s scaledUIEventSource) OnScroll(fn func(float64, float64)) { s.source.OnScroll(fn) }
func (s scaledUIEventSource) OnResize(fn func(int, int))         { s.source.OnResize(fn) }
func (s scaledUIEventSource) OnFocus(fn func(bool))              { s.source.OnFocus(fn) }
func (s scaledUIEventSource) OnIMECompositionStart(fn func())    { s.source.OnIMECompositionStart(fn) }
func (s scaledUIEventSource) OnIMECompositionUpdate(fn func(gpucontext.IMEState)) {
	s.source.OnIMECompositionUpdate(fn)
}
func (s scaledUIEventSource) OnIMECompositionEnd(fn func(string)) { s.source.OnIMECompositionEnd(fn) }

type quitReceiver interface {
	SetQuitFunc(func())
}

type uiAppReceiver interface {
	SetUIApp(client.UIApp)
}

type uiAppBridge struct {
	*uiapp.App
	runner                 *runner
	uiManager              client.UIManager
	controllerMode         bool
	controllerScope        widget.Widget
	controllerRestoreFocus widget.Focusable
	controllerFocusStack   []controllerFocusFrame
	controllerLastFocus    widget.Focusable
}

type controllerFocusFrame struct {
	scope widget.Widget
	focus widget.Focusable
}

func (b *uiAppBridge) SetUIRoot(root widget.Widget) {
	if b.App != nil {
		b.App.SetRoot(root)
		if b.controllerMode {
			// A newly published overlay must inherit the current input mode even
			// when no new controller event arrives after it is mounted.
			setControllerMode(root, true)
		}
		if empty, ok := root.(interface{ IsUIRootEmpty() bool }); root == nil || ok && empty.IsUIRootEmpty() {
			b.runner.discardPublishedUI()
		}
		if b.App.Window() != nil && b.App.Window().Context() != nil {
			b.App.Window().Context().ResetCursor()
		}
	}
}

func (b uiAppBridge) Invalidate() {
	if b.App == nil || b.App.Window() == nil || b.App.Window().Context() == nil {
		return
	}
	root := b.App.Window().Root()
	if root != nil {
		if bounder, ok := root.(interface{ Bounds() geometry.Rect }); ok {
			if bounds := bounder.Bounds(); !bounds.IsEmpty() {
				b.App.Window().Context().InvalidateRect(bounds)
				return
			}
		}
	}
	b.App.Window().Context().Invalidate()
}

func (b uiAppBridge) InvalidateRect(rect geometry.Rect) {
	if b.App == nil || b.App.Window() == nil || b.App.Window().Context() == nil || rect.IsEmpty() {
		return
	}
	b.App.Window().Context().InvalidateRect(rect)
}

func (b uiAppBridge) InvalidateLayout() {
	if b.App == nil || b.App.Window() == nil || b.App.Window().Context() == nil {
		return
	}
	b.App.Window().Context().Invalidate()
}

func (b uiAppBridge) WidgetContext() widget.Context {
	if b.App == nil || b.App.Window() == nil {
		return nil
	}
	return b.App.Window().Context()
}

func (b *uiAppBridge) FocusControllerWidget(target widget.Widget) bool {
	if b == nil || b.App == nil || b.App.Window() == nil || target == nil {
		return false
	}
	focus, ok := target.(widget.Focusable)
	if !ok || !focus.IsFocusable() {
		return false
	}
	return b.focusControllerWidget(focus)
}

func (b uiAppBridge) Cursor() widget.CursorType {
	if b.App == nil || b.App.Window() == nil || b.App.Window().Context() == nil {
		return widget.CursorDefault
	}
	return b.App.Window().Context().Cursor()
}

func (b uiAppBridge) HoveredWidget() widget.Widget {
	if b.App == nil || b.App.Window() == nil {
		return nil
	}
	return b.App.Window().HoveredWidget()
}

var _ client.UIController = (*uiAppBridge)(nil)

// SetControllerMode enables controller-aware themed controls and focus
// affordances. Keyboard and mouse input switch this back off through
// wireInput's source callback.
func (b *uiAppBridge) SetControllerMode(enabled bool) {
	if b == nil || b.App == nil || b.App.Window() == nil {
		return
	}
	changed := b.controllerMode != enabled
	b.controllerMode = enabled
	setControllerMode(b.App.Window().Root(), enabled)
	if changed {
		b.requestControllerUIRedraw()
	}
}

func (b *uiAppBridge) focusControllerWidget(focus widget.Focusable) bool {
	if b == nil || b.App == nil || b.App.Window() == nil || focus == nil || !focus.IsFocusable() {
		return false
	}
	manager := b.App.Window().FocusManager()
	previous := manager.Focused()
	manager.Focus(focus)
	b.controllerLastFocus = manager.Focused()
	b.controllerFocusChanged(previous, manager.Focused())
	return true
}

func (b *uiAppBridge) controllerFocusChanged(previous, current widget.Focusable) {
	if previous != nil {
		if redraw, ok := previous.(interface{ SetNeedsRedraw(bool) }); ok {
			redraw.SetNeedsRedraw(true)
		}
	}
	if current != nil {
		if redraw, ok := current.(interface{ SetNeedsRedraw(bool) }); ok {
			redraw.SetNeedsRedraw(true)
		}
	}
	b.requestControllerUIRedraw()
}

func (b *uiAppBridge) requestControllerUIRedraw() {
	if b == nil || b.App == nil || b.App.Window() == nil {
		return
	}
	if root := b.App.Window().Root(); root != nil {
		if redraw, ok := root.(interface{ SetNeedsRedraw(bool) }); ok {
			redraw.SetNeedsRedraw(true)
		}
	}
	if ctx := b.App.Window().Context(); ctx != nil {
		ctx.Invalidate()
	}
}

// HandleControllerAction routes a normalized controller UI action through the
// existing widget tree. Directional events are offered to widgets first so
// lists and sliders retain their native keyboard behavior; spatial focus is
// used when the focused widget does not consume the event.
func (b *uiAppBridge) HandleControllerAction(action input.UIAction) bool {
	if b == nil || b.App == nil || b.App.Window() == nil || b.App.Window().Root() == nil {
		return false
	}
	b.SetControllerMode(true)
	window := b.App.Window()
	root := window.Root()
	scope := controllerFocusScope(root)
	b.syncControllerScope(scope)
	focused := window.FocusManager().Focused()
	if focused == nil || !widgetInTree(scope, focusedWidget(focused)) {
		// Some windows establish their initial focus by setting the widget's
		// focused flag while constructing their tree. Reconcile that state with
		// the focus manager before interpreting a controller action. The same
		// check also drops a stale focus pointer after a dynamic window rebuild.
		if candidate := controllerFocusedWidget(scope); candidate != nil {
			b.focusControllerWidget(candidate)
			focused = candidate
		} else {
			focused = b.focusInitialControllerScope(scope)
		}
	}
	// Give packet-driven modal windows (NPC dialogs, trade prompts, and other
	// semantic overlays) first refusal. This keeps their action state local and
	// avoids pretending that a controller Confirm is a physical Enter that can
	// be interpreted by an unrelated widget.
	if handler, ok := scope.(interface{ HandleControllerAction(input.UIAction) bool }); ok && handler.HandleControllerAction(action) {
		return true
	}
	if action == input.UIActionCancel {
		if active, ok := b.uiManager.(interface{ ControllerKeyboardActive() bool }); ok && active.ControllerKeyboardActive() {
			if closer, ok := b.uiManager.(interface{ CloseControllerKeyboard() }); ok {
				closer.CloseControllerKeyboard()
				return true
			}
		}
	}
	if action == input.UIActionConfirm {
		if opener, ok := b.uiManager.(interface{ OpenControllerKeyboard(widget.Widget) bool }); ok {
			if focused != nil {
				if target, ok := focused.(widget.Widget); ok && widgetInTree(scope, target) && opener.OpenControllerKeyboard(target) {
					b.SetControllerMode(true)
					return true
				}
			}
		}
	}
	if action == input.UIActionContext || action == input.UIActionSecondary {
		// These face-button roles have no framework key equivalent. A concrete
		// window may consume them through its semantic handler; otherwise the
		// active overlay still owns the press and prevents gameplay fallthrough.
		return scope != root
	}
	if action == input.UIActionNextFocus || action == input.UIActionPreviousFocus {
		focusables := collectFocusable(scope)
		if len(focusables) == 0 {
			return false
		}
		previous := window.FocusManager().Focused()
		cycleControllerFocus(window.FocusManager(), focusables, action == input.UIActionNextFocus)
		b.controllerLastFocus = window.FocusManager().Focused()
		b.controllerFocusChanged(previous, window.FocusManager().Focused())
		return true
	}
	key, ok := controllerUIKey(action)
	if !ok {
		return false
	}
	ctx := window.Context()
	keyEvent := event.NewKeyEvent(event.KeyPress, key, 0, event.ModNone)
	// Controller UI events are modal at the overlay level. Sending them to the
	// whole overlay root lets an unhandled key fall through into every window
	// underneath the active one (for example, Cross on the keyboard reaching
	// the login form). Keep the event inside the topmost visible scope.
	eventRoot := scope
	if eventRoot == nil {
		eventRoot = root
	}
	if action == input.UIActionConfirm {
		// Buttons accept Enter and Space, while checkboxes intentionally accept
		// Space only. Try Enter first for ordinary controls and fall back to a
		// complete Space press/release pair so controller Confirm also toggles
		// checkboxes without ever reaching a lower window.
		if eventRoot.Event(ctx, keyEvent) {
			// gogpu/ui buttons activate on the release edge. Controller input
			// is sampled as a press edge, so synthesize the matching release
			// after the focused widget accepts the press.
			eventRoot.Event(ctx, event.NewKeyEvent(event.KeyRelease, key, 0, event.ModNone))
			return true
		}
		space := event.NewKeyEvent(event.KeyPress, event.KeySpace, 0, event.ModNone)
		if eventRoot.Event(ctx, space) {
			eventRoot.Event(ctx, event.NewKeyEvent(event.KeyRelease, event.KeySpace, 0, event.ModNone))
			return true
		}
	} else if eventRoot.Event(ctx, keyEvent) {
		return true
	}
	if action == input.UIActionConfirm || action == input.UIActionCancel || action == input.UIActionPageUp || action == input.UIActionPageDown {
		focusables := collectFocusable(scope)
		if len(focusables) == 0 {
			return eventRoot != root
		}
		if window.FocusManager().Focused() == nil {
			b.focusControllerWidget(focusables[0])
		}
		return true
	}
	if b.moveSpatialFocus(action, focusablesForRoot(scope, nil)) {
		return true
	}
	// A visible overlay still owns controller input even when it has no
	// directionally adjacent focus target. Do not leak the event to gameplay or
	// a window below it.
	return eventRoot != root
}

// syncControllerScope makes opening and closing windows deterministic for a
// controller. A newly active scope gets its own initial target; returning to a
// scope restores the focus that was active before the child window opened.
func (b *uiAppBridge) syncControllerScope(scope widget.Widget) {
	if b == nil || b.App == nil || b.App.Window() == nil || scope == b.controllerScope {
		return
	}
	window := b.App.Window()
	previousScope := b.controllerScope
	previousFocus := window.FocusManager().Focused()
	if previousFocus == nil {
		previousFocus = b.controllerLastFocus
	}
	returning := false
	if n := len(b.controllerFocusStack); n > 0 && b.controllerFocusStack[n-1].scope == scope {
		b.controllerRestoreFocus = b.controllerFocusStack[n-1].focus
		b.controllerFocusStack = b.controllerFocusStack[:n-1]
		returning = true
	}
	if !returning && previousScope != nil && previousFocus != nil && widgetInTree(previousScope, focusedWidget(previousFocus)) {
		b.controllerFocusStack = append(b.controllerFocusStack, controllerFocusFrame{scope: previousScope, focus: previousFocus})
	}
	b.controllerScope = scope
	if scope == nil {
		return
	}
	if b.controllerRestoreFocus != nil && widgetInTree(scope, focusedWidget(b.controllerRestoreFocus)) && b.controllerRestoreFocus.IsFocusable() {
		b.focusControllerWidget(b.controllerRestoreFocus)
		b.controllerRestoreFocus = nil
		return
	}
	b.controllerRestoreFocus = nil
	b.focusInitialControllerScope(scope)
}

func (b *uiAppBridge) focusInitialControllerScope(scope widget.Widget) widget.Focusable {
	if b == nil || b.App == nil || b.App.Window() == nil || scope == nil {
		return nil
	}
	if hint, ok := scope.(interface{ ControllerInitialFocus() widget.Widget }); ok {
		if target := hint.ControllerInitialFocus(); target != nil {
			if focus, ok := target.(widget.Focusable); ok && focus.IsFocusable() {
				b.focusControllerWidget(focus)
				return focus
			}
		}
	}
	if focus := controllerFocusedWidget(scope); focus != nil {
		b.focusControllerWidget(focus)
		return focus
	}
	if focusables := collectFocusable(scope); len(focusables) > 0 {
		b.focusControllerWidget(focusables[0])
		return focusables[0]
	}
	return nil
}

func controllerFocusScope(root widget.Widget) widget.Widget {
	if root == nil {
		return nil
	}
	children := root.Children()
	var entryPoint widget.Widget
	for i := len(children) - 1; i >= 0; i-- {
		child := children[i]
		if child == nil {
			continue
		}
		if visible, ok := child.(interface{ IsVisible() bool }); ok && !visible.IsVisible() {
			continue
		}
		if enabled, ok := child.(interface{ IsEnabled() bool }); ok && !enabled.IsEnabled() {
			continue
		}
		// A passive HUD overlay is visible for context but deliberately does
		// not own controller focus. Resolve the modal/interactive overlay below
		// it instead (for example an NPC dialog below the shortcut bar).
		if passive, ok := child.(interface{ ControllerNavigationPassthrough() bool }); ok && passive.ControllerNavigationPassthrough() {
			if entry, ok := child.(interface{ ControllerNavigationEntryPoint() bool }); ok && entry.ControllerNavigationEntryPoint() && entryPoint == nil {
				entryPoint = child
			}
			continue
		}
		return child
	}
	if entryPoint != nil {
		return entryPoint
	}
	return root
}

func widgetInTree(root, target widget.Widget) bool {
	if root == nil || target == nil {
		return false
	}
	if root == target {
		return true
	}
	for _, child := range controllerChildren(root) {
		if widgetInTree(child, target) {
			return true
		}
	}
	return false
}

func controllerUIKey(action input.UIAction) (event.Key, bool) {
	switch action {
	case input.UIActionUp:
		return event.KeyUp, true
	case input.UIActionDown:
		return event.KeyDown, true
	case input.UIActionLeft:
		return event.KeyLeft, true
	case input.UIActionRight:
		return event.KeyRight, true
	case input.UIActionConfirm:
		return event.KeyEnter, true
	case input.UIActionCancel:
		return event.KeyEscape, true
	case input.UIActionPageUp:
		return event.KeyPageUp, true
	case input.UIActionPageDown:
		return event.KeyPageDown, true
	default:
		return 0, false
	}
}

type focusableWidget struct {
	focus  widget.Focusable
	widget widget.Widget
	center geometry.Point
}

func focusablesForRoot(root widget.Widget, out []focusableWidget) []focusableWidget {
	if root == nil {
		return out
	}
	if focus, ok := root.(widget.Focusable); ok && focus.IsFocusable() {
		if bounds, hasBounds := controllerWidgetBounds(root); hasBounds {
			out = append(out, focusableWidget{focus: focus, widget: root, center: bounds.Center()})
		}
	}
	for _, child := range controllerChildren(root) {
		out = focusablesForRoot(child, out)
	}
	return out
}

// controllerWidgetBounds returns screen-space bounds for focus navigation.
// ScreenBounds is authoritative after a draw pass. Before the first draw
// (notably when a controller opens a new overlay), walk the parent chain so
// nested boxes do not all appear to share the same local origin.
func controllerWidgetBounds(w widget.Widget) (geometry.Rect, bool) {
	if w == nil {
		return geometry.Rect{}, false
	}
	if screen, ok := w.(interface {
		ScreenBounds() geometry.Rect
		IsScreenOriginValid() bool
	}); ok && screen.IsScreenOriginValid() {
		return screen.ScreenBounds(), true
	}

	var (
		origin geometry.Point
		size   geometry.Size
		cur    = w
		first  = true
	)
	for cur != nil {
		bounds, ok := cur.(interface{ Bounds() geometry.Rect })
		if !ok {
			return geometry.Rect{}, false
		}
		local := bounds.Bounds()
		origin = origin.Add(local.Min)
		if first {
			size = local.Size()
			first = false
		}
		parent, ok := cur.(interface{ Parent() widget.Widget })
		if !ok {
			break
		}
		cur = parent.Parent()
	}
	return geometry.FromPointSize(origin, size), true
}

func collectFocusable(root widget.Widget) []widget.Focusable {
	items := focusablesForRoot(root, nil)
	out := make([]widget.Focusable, 0, len(items))
	for _, item := range items {
		out = append(out, item.focus)
	}
	return out
}

func controllerFocusedWidget(root widget.Widget) widget.Focusable {
	if root == nil {
		return nil
	}
	if focus, ok := root.(widget.Focusable); ok && focus.IsFocusable() && focus.IsFocused() {
		return focus
	}
	for _, child := range controllerChildren(root) {
		if focus := controllerFocusedWidget(child); focus != nil {
			return focus
		}
	}
	return nil
}

func cycleControllerFocus(manager interface {
	Focused() widget.Focusable
	Focus(widget.Focusable)
}, focusables []widget.Focusable, forward bool) {
	if manager == nil || len(focusables) == 0 {
		return
	}
	current := manager.Focused()
	index := -1
	for i, focus := range focusables {
		if focus == current {
			index = i
			break
		}
	}
	if index < 0 {
		if forward {
			manager.Focus(focusables[0])
		} else {
			manager.Focus(focusables[len(focusables)-1])
		}
		return
	}
	if forward {
		index = (index + 1) % len(focusables)
	} else {
		index = (index - 1 + len(focusables)) % len(focusables)
	}
	manager.Focus(focusables[index])
}

func (b *uiAppBridge) moveSpatialFocus(action input.UIAction, items []focusableWidget) bool {
	if b == nil || b.App == nil || b.App.Window() == nil || len(items) == 0 {
		return false
	}
	current := b.App.Window().FocusManager().Focused()
	if current == nil {
		return b.focusControllerWidget(items[0].focus)
	}
	currentWidget, ok := current.(widget.Widget)
	if !ok {
		return false
	}
	currentBounds, ok := controllerWidgetBounds(currentWidget)
	if !ok {
		return false
	}
	origin := currentBounds.Center()
	dx, dy := float32(0), float32(0)
	switch action {
	case input.UIActionUp:
		dy = -1
	case input.UIActionDown:
		dy = 1
	case input.UIActionLeft:
		dx = -1
	case input.UIActionRight:
		dx = 1
	default:
		return false
	}
	best := -1
	bestPrimary := float32(0)
	bestSecondary := float32(0)
	for i, item := range items {
		if item.focus == current {
			continue
		}
		vx, vy := item.center.X-origin.X, item.center.Y-origin.Y
		primary := vx*dx + vy*dy
		if primary <= 0 {
			continue
		}
		secondary := float32(math.Abs(float64(vx*dy - vy*dx)))
		// Alignment with the current row/column is the primary criterion.
		// Distance along the requested axis is only the tie-breaker; otherwise
		// a nearby control below a row can win over the next key to the right.
		if best < 0 || secondary < bestSecondary || secondary == bestSecondary && primary < bestPrimary {
			best, bestPrimary, bestSecondary = i, primary, secondary
		}
	}
	if best < 0 {
		return false
	}
	return b.focusControllerWidget(items[best].focus)
}

func focusedWidget(focus widget.Focusable) widget.Widget {
	if focus == nil {
		return nil
	}
	widget, _ := focus.(widget.Widget)
	return widget
}

func setControllerMode(root widget.Widget, enabled bool) {
	if root == nil {
		return
	}
	if setter, ok := root.(interface{ SetControllerMode(bool) }); ok {
		setter.SetControllerMode(enabled)
	}
	for _, child := range controllerChildren(root) {
		setControllerMode(child, enabled)
	}
}

func controllerChildren(root widget.Widget) []widget.Widget {
	if root == nil {
		return nil
	}
	if logical, ok := root.(interface{ ControllerChildren() []widget.Widget }); ok {
		return logical.ControllerChildren()
	}
	return root.Children()
}

func (b uiAppBridge) BeginWindowDragLayer(token any, rect geometry.Rect) bool {
	if b.runner == nil {
		return false
	}
	return b.runner.beginUIDragLayer(token, rect)
}

func (b uiAppBridge) MoveWindowDragLayer(token any, rect geometry.Rect) {
	if b.runner != nil {
		b.runner.moveUIDragLayer(token, rect)
	}
}

func (b uiAppBridge) EndWindowDragLayer(token any) {
	if b.runner != nil {
		b.runner.endUIDragLayer(token)
	}
}

func (b uiAppBridge) CancelWindowDragLayer(token any) {
	if b.runner != nil {
		b.runner.cancelUIDragLayer(token)
	}
}

func (b uiAppBridge) WindowDragActive() bool {
	return b.runner != nil && b.runner.uiDrag.active && !b.runner.uiDrag.releasePending
}

type overlayDrawer interface {
	DrawOverlay(*Frame)
}

type uiOverlayDrawer interface {
	DrawUIOverlay(*Frame)
}

type frameSubmittedReceiver interface {
	FrameSubmitted()
}

type runtimeSettingsProvider interface {
	RuntimeFullscreen() bool
	RuntimeVSync() bool
	RuntimeFPS() bool
}

type controllerSettingsProvider interface {
	ControllerSettings() input.ControllerSettings
}

type screenshotRequester interface {
	ConsumeScreenshotRequest() (string, bool)
	CompleteScreenshot(path string, err error)
}

type captureOptionsRequester interface {
	ConsumeCaptureRequest() (capture.ScreenshotOptions, string, bool)
}

type recordingRequester interface {
	ConsumeRecordingStart() (capture.RecordingOptions, bool)
	ConsumeRecordingStop() bool
	CompleteRecording(path string, err error)
	RecordingResize(path string)
}

type cachedOverlayImage struct {
	image  *Image
	width  int
	height int
}

type uiDragLayer struct {
	token          any
	rect           geometry.Rect
	image          *Image
	active         bool
	releasePending bool
	drawOffsetX    float32
	drawOffsetY    float32
	drawWidth      float32
	drawHeight     float32
}

type uiImageRectCapture struct {
	image *Image
	rect  geometry.Rect
}

type uiProfileStats struct {
	start            time.Time
	lastLog          time.Time
	frames           int64
	slowFrames       int64
	uiSlowFrames     int64
	uiWorkFrames     int64
	uiRedrawFrames   int64
	uiFullRepaints   int64
	dirtyRegionSum   int64
	dirtyRegionMax   int
	totalDurSum      time.Duration
	updateDurSum     time.Duration
	gameUpdateDurSum time.Duration
	uiFrameDurSum    time.Duration
	drawDurSum       time.Duration
	uiDrawDurSum     time.Duration
	uiCanvasDurSum   time.Duration
	uiFlushDurSum    time.Duration
	uiImageDurSum    time.Duration
	totalDurMax      time.Duration
	updateDurMax     time.Duration
	gameUpdateDurMax time.Duration
	uiFrameDurMax    time.Duration
	drawDurMax       time.Duration
	uiDrawDurMax     time.Duration
	uiCanvasDurMax   time.Duration
	uiFlushDurMax    time.Duration
	uiImageDurMax    time.Duration
}

type runner struct {
	app             *gogpu.App
	ui              *uiapp.App
	uiImage         *Image
	uiOverlayCanvas *ggcanvas.Canvas
	uiTextCache     map[string]cachedOverlayImage
	uiBubbleCache   map[string]cachedOverlayImage
	game            Game
	screen          *Frame
	gpu             *gpuRenderer
	width           int
	height          int
	duration        time.Duration
	warmup          time.Duration
	renderCfg       config.RenderConfig
	started         time.Time
	measureStarted  time.Time
	lastLog         time.Time
	lastFrame       int64
	frames          int64
	measuredFrames  int64
	fpsStarted      time.Time
	fpsFrames       int64
	fpsDisplay      float64
	frameMSDisplay  float64
	fpsText         string
	uiOverlayScale  float64
	quit            func()
	cpuProfile      *os.File
	fullscreen      bool
	vsync           bool
	fps             bool
	vsyncWarned     bool
	uiDrawnOnce     bool
	uiScale         float64
	uiCanvas        *ggcanvas.Canvas
	uiLogicalWidth  int
	uiLogicalHeight int
	uiAsync         *asyncUIRasterizer
	uiAsyncBusy     bool
	uiPendingLists  []uiDrawList
	uiGeneration    uint64
	uiDrag          uiDragLayer

	lastUpdateDuration   time.Duration
	lastGameUpdateDur    time.Duration
	lastUIFrameDur       time.Duration
	lastUIWork           bool
	lastUIRedraw         bool
	lastUIDrawDur        time.Duration
	lastUICanvasDrawDur  time.Duration
	lastUIFlushDur       time.Duration
	lastUIImageDur       time.Duration
	lastUIDirtyRegions   int
	lastUIFullRepaint    bool
	lastUIDirtyUnion     geometry.Rect
	lastUIDrawStats      widget.DrawStats
	uiProfile            uiProfileStats
	captureCfg           config.CaptureConfig
	capture              *captureRuntime
	controller           input.ControllerBackend
	controllerSettings   input.ControllerSettings
	uiBridge             *uiAppBridge
	controllerNav        navRepeater
	controllerRightNav   navRepeater
	controllerTriggerNav navRepeater
	controllerPollError  bool
	events               *fanoutEventSource
	cursor               input.VirtualCursor
	controllerLastPoll   time.Time
	injectingPointer     bool
	cursorLeftDown       bool
	controllerConnected  bool
}

// windowTitle appends the build identifier to the default window title so the
// running version is visible at a glance. An explicit --title is left untouched.
func windowTitle(base string) string {
	if base == "" || base == "goro" {
		return "goro " + buildinfo.Version()
	}
	return base
}

func Run(game Game, cfg config.WindowConfig, renderCfg config.RenderConfig, captureCfg ...config.CaptureConfig) error {
	configureGogpuVSync(renderCfg)
	appConfig := gogpu.DefaultConfig()
	api, err := graphicsAPI(renderCfg.GraphicsAPI)
	if err != nil {
		return err
	}
	appConfig = appConfig.
		WithGraphicsAPI(api).
		WithTitle(windowTitle(cfg.Title)).
		WithIcon(appicon.Image()).
		WithSize(cfg.Width, cfg.Height).
		WithResizable(true).
		WithContinuousRender(true).
		WithVSync(renderCfg.VSync)
	if cfg.Fullscreen {
		appConfig = appConfig.WithFullscreen()
	}
	gg := gogpu.NewApp(appConfig)
	setCursorApp(gg)
	defer setCursorApp(nil)
	events := newFanoutEventSource(gg.EventSource())
	uiWidth, uiHeight := cfg.Width, cfg.Height
	uiEvents := scaledUIEventSource{
		source: events,
		scale: func() float64 {
			if provider, ok := game.(uiScaleProvider); ok {
				return float64(provider.UISettings().Normalized().Scale)
			}
			return 1
		},
		width: func() int { return uiWidth }, height: func() int { return uiHeight },
	}
	uiTheme := rotheme.Default.AsTheme()
	uiTheme.Colors.Background = widget.RGBA8(0, 0, 0, 0)
	ui := uiapp.New(
		uiapp.WithWindowProvider(gg),
		uiapp.WithPlatformProvider(roCursorPlatformProvider{PlatformProvider: gg}),
		uiapp.WithEventSource(uiEvents),
		uiapp.WithTheme(uiTheme),
		uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged),
	)

	var captureConfig config.CaptureConfig
	if len(captureCfg) > 0 {
		captureConfig = captureCfg[0]
	}
	r := &runner{
		app:                gg,
		ui:                 ui,
		game:               game,
		width:              cfg.Width,
		height:             cfg.Height,
		duration:           time.Duration(renderCfg.BenchSeconds) * time.Second,
		warmup:             time.Duration(renderCfg.BenchWarmupSeconds) * time.Second,
		renderCfg:          renderCfg,
		quit:               gg.Quit,
		fullscreen:         cfg.Fullscreen,
		vsync:              renderCfg.VSync,
		fps:                renderCfg.FPS,
		captureCfg:         captureConfig,
		controllerSettings: input.DefaultControllerSettings(),
	}
	if provider, ok := game.(controllerSettingsProvider); ok {
		r.controllerSettings = provider.ControllerSettings().Normalized()
	}
	r.events = events
	r.cursor.Reset(cfg.Width, cfg.Height)
	r.uiBridge = &uiAppBridge{App: ui, runner: r}
	if provider, ok := game.(interface{ ContextUIManager() client.UIManager }); ok {
		r.uiBridge.uiManager = provider.ContextUIManager()
		// Hand the manager the exact transform applied to real pointer events so
		// its hit tests agree with the widget tree at any UI scale.
		if transform, ok := r.uiBridge.uiManager.(interface {
			SetPointerTransform(func(x, y int) (int, int))
		}); ok {
			transform.SetPointerTransform(func(x, y int) (int, int) {
				lx, ly := uiEvents.point(float64(x), float64(y))
				return int(lx + 0.5), int(ly + 0.5)
			})
		}
	}
	if r.controllerSettings.Enabled {
		controller, controllerErr := gamepad.Open()
		if controllerErr != nil {
			glog.Warnf("controller input unavailable: %v", controllerErr)
		} else {
			r.controller = controller
		}
	}
	if receiver, ok := game.(quitReceiver); ok {
		receiver.SetQuitFunc(gg.Quit)
	}
	if receiver, ok := game.(uiAppReceiver); ok {
		receiver.SetUIApp(r.uiBridge)
	}
	game.Resize(cfg.Width, cfg.Height)
	wireInput(events, game.InputState(), func() bool { return r.injectingPointer }, func(source input.InputSource) {
		if r.uiBridge != nil {
			r.uiBridge.SetControllerMode(source == input.InputSourceController)
		}
	})

	gg.OnResize(func(width, height int) {
		if width <= 0 || height <= 0 {
			return
		}
		if r.capture != nil {
			r.capture.requestStop(true)
		}
		r.width, r.height = width, height
		uiWidth, uiHeight = width, height
		r.cursor.Resize(width, height)
		r.screen = nil
		r.game.Resize(width, height)
	})
	events.OnFocus(func(focused bool) {
		if !focused {
			// Losing focus must not leave a synthetic button held or the
			// character walking, mirroring the safeguard the real window
			// already applies to keyboard state.
			r.releaseControllerPointer()
		}
	})
	gg.OnUpdate(func(float64) {
		applyWindowIcon()
		if err := r.update(); err != nil {
			glog.Errorf("update error: %v", err)
			gg.Quit()
		}
	})
	gg.OnDraw(func(ctx *gogpu.Context) {
		if err := r.draw(ctx); err != nil {
			glog.Errorf("draw error: %v", err)
			gg.Quit()
		}
	})
	gg.OnClose(func() {
		r.closeCapture()
		if r.cpuProfile != nil {
			pprof.StopCPUProfile()
			_ = r.cpuProfile.Close()
			r.cpuProfile = nil
		}
		if r.gpu != nil {
			r.gpu.release()
			r.gpu = nil
		}
		r.stopAsyncUIRasterizer()
		if r.uiCanvas != nil {
			_ = r.uiCanvas.Close()
			r.uiCanvas = nil
		}
		if r.uiOverlayCanvas != nil {
			_ = r.uiOverlayCanvas.Close()
			r.uiOverlayCanvas = nil
		}
		if r.controller != nil {
			_ = r.controller.Close()
			r.controller = nil
		}
	})
	return gg.Run()
}

func configureGogpuVSync(renderCfg config.RenderConfig) {
	if renderCfg.VSync {
		return
	}
	if os.Getenv("GOGPU_WAYLAND_FRAME_CALLBACK") == "" {
		_ = os.Setenv("GOGPU_WAYLAND_FRAME_CALLBACK", "0")
	}
}

func graphicsAPI(name string) (gogputypes.GraphicsAPI, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "vulkan":
		return gogpu.GraphicsAPIVulkan, nil
	case "auto":
		return gogpu.GraphicsAPIAuto, nil
	case "dx12", "directx12", "d3d12":
		return gogpu.GraphicsAPIDX12, nil
	case "metal":
		return gogpu.GraphicsAPIMetal, nil
	case "gles", "opengl", "opengles":
		return gogpu.GraphicsAPIGLES, nil
	case "software", "soft":
		return gogpu.GraphicsAPISoftware, nil
	default:
		return gogpu.GraphicsAPIAuto, fmt.Errorf("unknown graphics api %q", name)
	}
}

type fanoutEventSource struct {
	keyPress             []func(gpucontext.Key, gpucontext.Modifiers)
	keyRelease           []func(gpucontext.Key, gpucontext.Modifiers)
	textInput            []func(string)
	mouseMove            []func(float64, float64)
	mousePress           []func(gpucontext.MouseButton, float64, float64)
	mouseRelease         []func(gpucontext.MouseButton, float64, float64)
	scroll               []func(float64, float64)
	resize               []func(int, int)
	focus                []func(bool)
	imeCompositionStart  []func()
	imeCompositionEnd    []func(string)
	imeCompositionUpdate []func(gpucontext.IMEState)
}

func newFanoutEventSource(source gpucontext.EventSource) *fanoutEventSource {
	f := &fanoutEventSource{}
	source.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		for _, fn := range f.keyPress {
			fn(key, mods)
		}
	})
	source.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		for _, fn := range f.keyRelease {
			fn(key, mods)
		}
	})
	source.OnTextInput(func(text string) {
		for _, fn := range f.textInput {
			fn(text)
		}
	})
	source.OnMouseMove(func(x, y float64) {
		for _, fn := range f.mouseMove {
			fn(x, y)
		}
	})
	source.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
		for _, fn := range f.mousePress {
			fn(button, x, y)
		}
	})
	source.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
		for _, fn := range f.mouseRelease {
			fn(button, x, y)
		}
	})
	source.OnScroll(func(x, y float64) {
		for _, fn := range f.scroll {
			fn(x, y)
		}
	})
	source.OnResize(func(width, height int) {
		for _, fn := range f.resize {
			fn(width, height)
		}
	})
	source.OnFocus(func(focused bool) {
		for _, fn := range f.focus {
			fn(focused)
		}
	})
	source.OnIMECompositionStart(func() {
		for _, fn := range f.imeCompositionStart {
			fn()
		}
	})
	source.OnIMECompositionUpdate(func(state gpucontext.IMEState) {
		for _, fn := range f.imeCompositionUpdate {
			fn(state)
		}
	})
	source.OnIMECompositionEnd(func(committed string) {
		for _, fn := range f.imeCompositionEnd {
			fn(committed)
		}
	})
	return f
}

func (f *fanoutEventSource) OnKeyPress(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	f.keyPress = append(f.keyPress, fn)
}

func (f *fanoutEventSource) OnKeyRelease(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	f.keyRelease = append(f.keyRelease, fn)
}

func (f *fanoutEventSource) OnTextInput(fn func(string)) {
	f.textInput = append(f.textInput, fn)
}

func (f *fanoutEventSource) OnMouseMove(fn func(float64, float64)) {
	f.mouseMove = append(f.mouseMove, fn)
}

func (f *fanoutEventSource) OnMousePress(fn func(gpucontext.MouseButton, float64, float64)) {
	f.mousePress = append(f.mousePress, fn)
}

func (f *fanoutEventSource) OnMouseRelease(fn func(gpucontext.MouseButton, float64, float64)) {
	f.mouseRelease = append(f.mouseRelease, fn)
}

func (f *fanoutEventSource) OnScroll(fn func(float64, float64)) {
	f.scroll = append(f.scroll, fn)
}

func (f *fanoutEventSource) OnResize(fn func(int, int)) {
	f.resize = append(f.resize, fn)
}

func (f *fanoutEventSource) OnFocus(fn func(bool)) {
	f.focus = append(f.focus, fn)
}

func (f *fanoutEventSource) OnIMECompositionStart(fn func()) {
	f.imeCompositionStart = append(f.imeCompositionStart, fn)
}

func (f *fanoutEventSource) OnIMECompositionUpdate(fn func(gpucontext.IMEState)) {
	f.imeCompositionUpdate = append(f.imeCompositionUpdate, fn)
}

func (f *fanoutEventSource) OnIMECompositionEnd(fn func(string)) {
	f.imeCompositionEnd = append(f.imeCompositionEnd, fn)
}

// The Emit* methods inject synthetic pointer events into the same fanout the
// window delivers real ones to. Both pointer consumers — input.State for world
// picking and the gogpu widget tree for windows, drag, and scrolling — are
// downstream of this fork, so one emit reaches everything a real mouse would.

func (f *fanoutEventSource) EmitMouseMove(x, y float64) {
	for _, fn := range f.mouseMove {
		fn(x, y)
	}
}

func (f *fanoutEventSource) EmitMousePress(button gpucontext.MouseButton, x, y float64) {
	for _, fn := range f.mousePress {
		fn(button, x, y)
	}
}

func (f *fanoutEventSource) EmitMouseRelease(button gpucontext.MouseButton, x, y float64) {
	for _, fn := range f.mouseRelease {
		fn(button, x, y)
	}
}

func (f *fanoutEventSource) EmitScroll(x, y float64) {
	for _, fn := range f.scroll {
		fn(x, y)
	}
}

// wireInput copies window events into the shared input.State. synthetic reports
// whether the event currently being delivered was injected by the controller's
// virtual cursor; those must not be attributed to the mouse, or the pad would
// switch controller mode off in the widget tree on every frame it moves the
// pointer. A nil predicate means "everything is real".
func wireInput(events gpucontext.EventSource, state *input.State, synthetic func() bool, sourceChanged ...func(input.InputSource)) {
	if state == nil {
		return
	}
	notifySource := func(source input.InputSource) {
		if synthetic != nil && synthetic() {
			return
		}
		for _, callback := range sourceChanged {
			if callback != nil {
				callback(source)
			}
		}
	}
	events.OnKeyPress(func(key gpucontext.Key, _ gpucontext.Modifiers) {
		state.SetKeyCode(key, true)
		notifySource(input.InputSourceKeyboard)
	})
	events.OnKeyRelease(func(key gpucontext.Key, _ gpucontext.Modifiers) {
		state.SetKeyCode(key, false)
		notifySource(input.InputSourceKeyboard)
	})
	events.OnMouseMove(func(x, y float64) {
		state.SetMousePosition(int(x+0.5), int(y+0.5))
		notifySource(input.InputSourceMouse)
	})
	events.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
		state.SetMousePosition(int(x+0.5), int(y+0.5))
		if mapped, ok := mapMouseButton(button); ok {
			state.SetMouseButton(mapped, true)
		}
		notifySource(input.InputSourceMouse)
	})
	events.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
		state.SetMousePosition(int(x+0.5), int(y+0.5))
		if mapped, ok := mapMouseButton(button); ok {
			state.SetMouseButton(mapped, false)
		}
		notifySource(input.InputSourceMouse)
	})
	events.OnScroll(func(x, y float64) {
		state.AddWheel(x, y)
		notifySource(input.InputSourceMouse)
	})
	events.OnTextInput(func(text string) {
		state.AddTextInput(text)
		notifySource(input.InputSourceKeyboard)
	})
}

func mapMouseButton(button gpucontext.MouseButton) (input.MouseButton, bool) {
	switch button {
	case gpucontext.MouseButtonLeft:
		return input.MouseButtonLeft, true
	case gpucontext.MouseButtonRight:
		return input.MouseButtonRight, true
	default:
		return 0, false
	}
}

func (r *runner) pollController() {
	if r == nil || r.controller == nil || r.game == nil || r.game.InputState() == nil {
		return
	}
	now := time.Now()
	dt := input.ClampFrameDelta(now.Sub(r.controllerLastPoll))
	if r.controllerLastPoll.IsZero() {
		dt = 0
	}
	r.controllerLastPoll = now

	// Settings are re-read every poll rather than captured at startup, so the
	// settings page applies within a frame without extra plumbing.
	if provider, ok := r.game.(controllerSettingsProvider); ok {
		r.controllerSettings = provider.ControllerSettings().Normalized()
	}
	settings := r.controllerSettings

	snapshot, err := r.controller.Poll()
	if err != nil {
		if !r.controllerPollError {
			glog.Warnf("controller poll failed: %v", err)
			r.controllerPollError = true
		}
		r.game.InputState().SetController(input.ControllerSnapshot{})
		r.handleControllerDisconnect()
		return
	}
	r.controllerPollError = false
	state := r.game.InputState()
	state.SetController(snapshot)
	if snapshot.Active() {
		r.uiBridge.SetControllerMode(true)
	}
	if snapshot.Connected != r.controllerConnected {
		if snapshot.Connected {
			r.handleControllerConnect(snapshot)
		} else {
			r.handleControllerDisconnect()
		}
	}
	if !snapshot.Connected {
		return
	}

	actions := input.ResolveActions(state, settings)

	// The on-screen keyboard owns the whole frame while it is open: no pointer
	// motion, no zoom, and no walking behind it.
	if r.controllerKeyboardActive() {
		r.dispatchControllerUI(state, actions, now)
		state.ConsumeControllerMovement()
		state.ConsumeControllerCamera()
		state.ConsumeControllerZoom()
		consumeControllerUIActions(state)
		return
	}
	// A rebinding capture likewise consumes everything, so the button being
	// bound does not also fire the action it is being bound to. The snapshot is
	// still published above, which keeps the diagnostics animating.
	if r.controllerRebindActive() {
		state.ConsumeControllerMovement()
		state.ConsumeControllerCamera()
		state.ConsumeControllerZoom()
		consumeControllerUIActions(state)
		return
	}

	focusNavigation := false
	if nav, ok := r.uiBridge.uiManager.(interface{ ControllerFocusNavigationActive() bool }); ok {
		focusNavigation = nav.ControllerFocusNavigationActive()
	}
	controllerUI := false
	if active, ok := r.uiBridge.uiManager.(interface{ ControllerUIActive() bool }); ok {
		controllerUI = active.ControllerUIActive()
	}
	if controllerUI || focusNavigation || (settings.UINavMode == input.ControllerUINavFocus && r.pointerOverUI()) {
		r.dispatchControllerUI(state, actions, now)
		// Focus navigation owns the movement stick/D-pad while it is active;
		// otherwise the world consumer could interpret the same input as a walk.
		state.ConsumeControllerMovement()
		state.ConsumeControllerCamera()
		state.ConsumeControllerZoom()
		// The active UI scope owns every controller edge for this frame. This
		// closes the remaining leak paths (Map, camera reset, shortcut chords,
		// and unhandled alternate buttons) into the world consumer.
		consumeControllerUIActions(state)
		return
	}
	r.dispatchControllerPointer(state, snapshot, settings, dt)
}

func consumeControllerUIActions(state *input.State) {
	if state == nil {
		return
	}
	for action := input.ActionConfirm; action <= input.ActionResetCamera; action++ {
		state.ConsumeControllerAction(action)
	}
}

// controllerCursorAxes reports the stick vector that should drive the pointer
// this frame, and whether the left stick is therefore unavailable for walking.
// In cursor move mode the left stick always aims. In character move mode the
// right stick always aims, including while the pointer crosses a UI window;
// changing sticks at the window boundary made the virtual cursor feel broken
// and exposed the same stick to gameplay camera handling.
func controllerCursorAxes(snapshot input.ControllerSnapshot, settings input.ControllerSettings, _ bool) (x, y float32, blocksMovement bool) {
	leftX, leftY := input.ApplyRadialDeadzone(snapshot.LeftX, snapshot.LeftY, settings.Deadzone, settings.OuterDeadzone)
	rightX, rightY := input.ApplyRadialDeadzone(snapshot.RightX, snapshot.RightY, settings.Deadzone, settings.OuterDeadzone)
	if settings.MoveMode == input.ControllerMoveCursor {
		return leftX, leftY, true
	}
	return rightX, rightY, false
}

// controllerPointerClickEnabled keeps the virtual pointer's mouse-click
// compatibility path scoped to the modes that actually use it. In direct
// character movement mode, Confirm is a semantic gameplay action (interact,
// pick up, or talk) and must not also become a world left click/attack.
func controllerPointerClickEnabled(settings input.ControllerSettings, overUI bool) bool {
	return settings.MoveMode == input.ControllerMoveCursor || overUI
}

// controllerWheelScale converts full right-stick deflection into wheel notches
// per second while scrolling a UI list.
const controllerWheelScale = 12

func (r *runner) dispatchControllerPointer(state *input.State, snapshot input.ControllerSnapshot, settings input.ControllerSettings, dt time.Duration) {
	if r == nil || r.events == nil || state == nil {
		return
	}
	overUI := r.pointerOverUI()
	axisX, axisY, blocksMovement := controllerCursorAxes(snapshot, settings, overUI)
	if blocksMovement {
		// The left stick is aiming, so gameplay must not also read it as a walk
		// request this frame.
		state.ConsumeControllerMovement()
	} else {
		// The right stick is aiming instead. Gameplay must not also rotate the
		// camera with the same deflection, or the stick would do two jobs at
		// once.
		state.ConsumeControllerCamera()
	}
	if r.cursor.Move(axisX, axisY, dt, settings.CursorSpeed) {
		x, y := r.cursor.Position()
		r.injectPointer(func() { r.events.EmitMouseMove(float64(x), float64(y)) })
	}

	x, y := r.cursor.Position()
	confirm := settings.Bindings.Confirm
	pointerClickEnabled := controllerPointerClickEnabled(settings, overUI)
	switch {
	case pointerClickEnabled && snapshot.ButtonDown(confirm) && !r.cursorLeftDown:
		r.cursorLeftDown = true
		r.injectPointer(func() {
			r.events.EmitMousePress(gpucontext.MouseButtonLeft, float64(x), float64(y))
		})
		state.ConsumeControllerAction(input.ActionConfirm)
	case !snapshot.ButtonDown(confirm) && r.cursorLeftDown:
		r.cursorLeftDown = false
		r.injectPointer(func() {
			r.events.EmitMouseRelease(gpucontext.MouseButtonLeft, float64(x), float64(y))
		})
		if overUI {
			// gogpu buttons activate on release, so a synthetic click on a
			// hovered but unfocused control is ambiguous. Confirm the focused
			// widget as well.
			r.uiBridge.HandleControllerAction(input.UIActionConfirm)
		}
		state.ConsumeControllerAction(input.ActionConfirm)
	}

	if overUI {
		// Over the UI the right stick scrolls. Over the world it stays camera
		// rotation, which gameplay emits as CommandRotateCamera; synthesizing a
		// right-button drag here would double-rotate through MouseDX/MouseDY.
		_, wheelY := input.ApplyRadialDeadzone(snapshot.RightX, snapshot.RightY, settings.Deadzone, settings.OuterDeadzone)
		if wheelY != 0 {
			notches := float64(-wheelY) * controllerWheelScale * dt.Seconds()
			r.injectPointer(func() { r.events.EmitScroll(0, notches) })
		}
	}
}

// injectPointer marks the emission as synthetic so wireInput does not attribute
// it to the mouse and switch controller mode off.
func (r *runner) injectPointer(emit func()) {
	state := r.game.InputState()
	r.injectingPointer = true
	if state != nil {
		state.SetPointerSource(input.InputSourceController)
	}
	emit()
	if state != nil {
		state.SetPointerSource(input.InputSourceMouse)
	}
	r.injectingPointer = false
}

// releaseControllerPointer drops a held synthetic button and clears navigation
// repeats. It runs on disconnect and on window focus loss so neither can leave
// the pointer stuck down.
func (r *runner) releaseControllerPointer() {
	if r == nil {
		return
	}
	r.controllerNav.reset()
	r.controllerRightNav.reset()
	r.controllerTriggerNav.reset()
	if r.cursorLeftDown && r.events != nil {
		x, y := r.cursor.Position()
		r.cursorLeftDown = false
		r.injectPointer(func() {
			r.events.EmitMouseRelease(gpucontext.MouseButtonLeft, float64(x), float64(y))
		})
	}
}

func (r *runner) handleControllerConnect(snapshot input.ControllerSnapshot) {
	r.controllerConnected = true
	r.cursor.Reset(r.width, r.height)
	glog.Infof("controller connected name=%q type=%s", snapshot.Name, snapshot.Kind)
	if !r.controllerSettings.Rumble {
		return
	}
	if rumbler, ok := r.controller.(input.ControllerRumbler); ok {
		// A short buzz on connect proves the haptics path end to end.
		if err := rumbler.Rumble(0.19, 0.4, 200*time.Millisecond); err != nil {
			glog.Debugf("controller connect rumble failed: %v", err)
		}
	}
}

func (r *runner) handleControllerDisconnect() {
	if r == nil || !r.controllerConnected {
		return
	}
	r.controllerConnected = false
	r.releaseControllerPointer()
	glog.Infof("controller disconnected")
}

func (r *runner) pointerOverUI() bool {
	if r == nil || r.uiBridge == nil || r.uiBridge.uiManager == nil {
		return false
	}
	pointer, ok := r.uiBridge.uiManager.(client.UIPointer)
	if !ok {
		return false
	}
	x, y := r.cursor.Position()
	return pointer.PointerOverUI(x, y)
}

func (r *runner) controllerKeyboardActive() bool {
	if r == nil || r.uiBridge == nil || r.uiBridge.uiManager == nil {
		return false
	}
	active, ok := r.uiBridge.uiManager.(interface{ ControllerKeyboardActive() bool })
	return ok && active.ControllerKeyboardActive()
}

func (r *runner) controllerRebindActive() bool {
	if r == nil || r.uiBridge == nil || r.uiBridge.uiManager == nil {
		return false
	}
	active, ok := r.uiBridge.uiManager.(interface{ ControllerRebindActive() bool })
	return ok && active.ControllerRebindActive()
}

func (r *runner) dispatchControllerUI(state *input.State, actions input.ActionState, now time.Time) {
	if r == nil || state == nil || r.uiBridge == nil || !state.Controller().Connected {
		return
	}
	settings := r.controllerSettings
	delay, rate := settings.NavRepeatDelay(), settings.NavRepeatRate()
	move := actions.Move
	if !controllerMoveActive(state.Controller(), settings) || move == input.DirectionNone {
		r.controllerNav.reset()
	} else {
		action := controllerUIActionForDirection(move)
		if r.controllerNav.fire(action, now, delay, rate) && r.uiBridge.HandleControllerAction(action) {
			state.ConsumeControllerMovement()
		}
	}
	r.dispatchControllerAnalogUI(state, actions, now, delay, rate)
	r.dispatchControllerTriggerPaging(state, now, delay, rate)

	buttonActions := []struct {
		action input.Action
		ui     input.UIAction
	}{
		{input.ActionConfirm, input.UIActionConfirm},
		{input.ActionCancel, input.UIActionCancel},
		{input.ActionAttack, input.UIActionContext},
		{input.ActionLoot, input.UIActionSecondary},
		{input.ActionTargetPrevious, input.UIActionPreviousFocus},
		{input.ActionTargetNext, input.UIActionNextFocus},
		{input.ActionMenu, input.UIActionCancel},
	}
	for _, item := range buttonActions {
		if !actions.Pressed.Has(item.action) {
			continue
		}
		if r.uiBridge.HandleControllerAction(item.ui) {
			state.ConsumeControllerAction(item.action)
		}
	}
}

// dispatchControllerTriggerPaging gives bare L2/R2 a page role in a focused
// window while preserving their existing shortcut-modifier role when a face
// button is held with them.
func (r *runner) dispatchControllerTriggerPaging(state *input.State, now time.Time, delay, rate time.Duration) {
	if r == nil || state == nil || r.uiBridge == nil {
		return
	}
	snapshot := state.Controller()
	bindings := r.controllerSettings.Bindings
	faceHeld := snapshot.ButtonDown(input.ControllerButtonSouth) ||
		snapshot.ButtonDown(input.ControllerButtonEast) ||
		snapshot.ButtonDown(input.ControllerButtonWest) ||
		snapshot.ButtonDown(input.ControllerButtonNorth)
	if faceHeld {
		r.controllerTriggerNav.reset()
		return
	}
	page := input.UIAction(0)
	active := false
	if snapshot.ButtonDown(bindings.LeftModifier) {
		page = input.UIActionPageUp
		active = true
	} else if snapshot.ButtonDown(bindings.RightModifier) {
		page = input.UIActionPageDown
		active = true
	}
	if !active {
		r.controllerTriggerNav.reset()
		return
	}
	if r.controllerTriggerNav.fire(page, now, delay, rate) && r.uiBridge.HandleControllerAction(page) {
		state.ConsumeControllerAction(input.ActionLeftModifier)
		state.ConsumeControllerAction(input.ActionRightModifier)
	}
}

func (r *runner) dispatchControllerAnalogUI(state *input.State, actions input.ActionState, now time.Time, delay, rate time.Duration) {
	if r == nil || state == nil || r.uiBridge == nil || !state.Controller().Connected {
		return
	}
	var action input.UIAction
	switch {
	case actions.CameraY < -0.15:
		action = input.UIActionPageUp
	case actions.CameraY > 0.15:
		action = input.UIActionPageDown
	case actions.CameraX < -0.15:
		action = input.UIActionLeft
	case actions.CameraX > 0.15:
		action = input.UIActionRight
	default:
		r.controllerRightNav.reset()
		return
	}
	if r.controllerRightNav.fire(action, now, delay, rate) {
		r.uiBridge.HandleControllerAction(action)
	}
}

func controllerMoveActive(snapshot input.ControllerSnapshot, settings input.ControllerSettings) bool {
	leftX, leftY := input.ApplyRadialDeadzone(snapshot.LeftX, snapshot.LeftY, settings.Deadzone, settings.OuterDeadzone)
	return leftX != 0 || leftY != 0 || snapshot.Buttons.Has(input.ControllerButtonDPadUp) || snapshot.Buttons.Has(input.ControllerButtonDPadDown) || snapshot.Buttons.Has(input.ControllerButtonDPadLeft) || snapshot.Buttons.Has(input.ControllerButtonDPadRight)
}

func controllerUIActionForDirection(direction input.Direction8) input.UIAction {
	switch direction {
	case input.DirectionNorth, input.DirectionNorthEast, input.DirectionNorthWest:
		return input.UIActionUp
	case input.DirectionSouth, input.DirectionSouthEast, input.DirectionSouthWest:
		return input.UIActionDown
	case input.DirectionWest:
		return input.UIActionLeft
	default:
		return input.UIActionRight
	}
}

func (r *runner) update() error {
	updateStart := time.Now()
	r.applyRuntimeSettings()
	r.pollController()
	if r.duration > 0 && r.started.IsZero() {
		r.started = time.Now()
		r.lastLog = r.started
		if path := r.renderCfg.CPUProfile; path != "" {
			file, err := os.Create(path)
			if err != nil {
				glog.Warnf("cpu profile start failed: %v", err)
			} else if err := pprof.StartCPUProfile(file); err != nil {
				glog.Warnf("cpu profile start failed: %v", err)
				_ = file.Close()
			} else {
				r.cpuProfile = file
				glog.Infof("cpu profile writing %s", path)
			}
		}
		glog.Infof("benchmark start duration=%s warmup=%s vsync=%v", r.duration, r.warmup, r.renderCfg.VSync)
	}
	gameStart := time.Now()
	if err := r.game.Update(); err != nil {
		return err
	}
	r.lastGameUpdateDur = time.Since(gameStart)
	if r.ui != nil {
		uiStart := time.Now()
		r.ui.Frame()
		r.lastUIFrameDur = time.Since(uiStart)
	} else {
		r.lastUIFrameDur = 0
	}
	r.lastUpdateDuration = time.Since(updateStart)
	reapplyCursorMode()
	if r.duration <= 0 {
		return nil
	}
	now := time.Now()
	if r.measureStarted.IsZero() && now.Sub(r.started) >= r.warmup {
		r.measureStarted = now
		r.measuredFrames = 0
		glog.Infof("benchmark measure start elapsed=%.3fs", now.Sub(r.started).Seconds())
	}
	if now.Sub(r.lastLog) >= time.Second {
		elapsed := now.Sub(r.started).Seconds()
		interval := now.Sub(r.lastLog).Seconds()
		frames := r.frames - r.lastFrame
		glog.Infof("benchmark fps interval=%.1f average=%.1f frames=%d elapsed=%.1fs", float64(frames)/interval, float64(r.frames)/elapsed, r.frames, elapsed)
		r.lastLog = now
		r.lastFrame = r.frames
	}
	if now.Sub(r.started) >= r.duration {
		elapsed := now.Sub(r.started).Seconds()
		measuredElapsed := elapsed
		measuredFPS := float64(r.frames) / elapsed
		if !r.measureStarted.IsZero() {
			measuredElapsed = now.Sub(r.measureStarted).Seconds()
			if measuredElapsed > 0 {
				measuredFPS = float64(r.measuredFrames) / measuredElapsed
			}
		}
		glog.Infof("benchmark result fps=%.1f measured_fps=%.1f frames=%d measured_frames=%d elapsed=%.3fs measured_elapsed=%.3fs", float64(r.frames)/elapsed, measuredFPS, r.frames, r.measuredFrames, elapsed, measuredElapsed)
		r.logUIProfile(time.Now(), true)
		if r.cpuProfile != nil {
			pprof.StopCPUProfile()
			_ = r.cpuProfile.Close()
			r.cpuProfile = nil
		}
		r.quit()
	}
	return nil
}

func (r *runner) applyRuntimeSettings() {
	provider, ok := r.game.(runtimeSettingsProvider)
	if !ok || provider == nil {
		return
	}
	if fullscreen := provider.RuntimeFullscreen(); fullscreen != r.fullscreen {
		r.app.SetFullscreen(fullscreen)
		r.fullscreen = fullscreen
		r.requestUIRedraw()
	}
	if fps := provider.RuntimeFPS(); fps != r.fps {
		r.fps = fps
		r.renderCfg.FPS = fps
		r.fpsStarted = time.Time{}
		r.fpsFrames = 0
		r.fpsDisplay = 0
		r.frameMSDisplay = 0
		r.fpsText = ""
	}
	if vsync := provider.RuntimeVSync(); vsync != r.vsync {
		r.vsync = vsync
		r.renderCfg.VSync = vsync
		if !r.vsyncWarned {
			glog.Debugf("runtime vsync changed to %v; current gogpu backend applies vsync at startup", vsync)
			r.vsyncWarned = true
		}
	}
}

func (r *runner) draw(ctx *gogpu.Context) error {
	drawStart := time.Now()
	width, height := ctx.Size()
	if width <= 0 || height <= 0 {
		width, height = r.width, r.height
	}
	if width <= 0 || height <= 0 {
		return nil
	}
	if r.screen == nil || r.screen.Bounds().Dx() != width || r.screen.Bounds().Dy() != height {
		r.screen = NewFrame(width, height)
		r.width, r.height = width, height
		r.game.Resize(width, height)
	}
	framebufferW, framebufferH := ctx.FramebufferSize()
	scaleX, scaleY := framebufferScale(width, height, framebufferW, framebufferH)
	deviceScale := ctx.ScaleFactor()
	if deviceScale <= 0 {
		deviceScale = float64(scaleX)
	}
	if r.gpu == nil {
		gpu, err := newGPURenderer(ctx, r.app, r.renderCfg)
		if err != nil {
			return err
		}
		r.gpu = gpu
		glog.Infof("render backend=%s surface_format=%s", ctx.Backend(), r.gpu.format)
	}
	// A minimized native window can temporarily expose no drawable surface.
	// Avoid rebuilding the game and UI frame in that state, and yield so the
	// platform event loop remains responsive until restore supplies a surface.
	surface := ctx.SurfaceView()
	if surface == nil {
		time.Sleep(16 * time.Millisecond)
		return nil
	}
	r.prepareCapture()
	r.screen.BeginFrame()
	r.screen.SetScreenScale(scaleX, scaleY)
	r.resetUIDrawMeasurement()
	r.game.Draw(r.screen)
	if err := r.drawUIOverlay(r.screen, deviceScale); err != nil {
		return err
	}
	if err := r.drawUI(r.screen, width, height, deviceScale); err != nil {
		return err
	}
	if drawer, ok := r.game.(uiOverlayDrawer); ok {
		drawer.DrawUIOverlay(r.screen)
		if err := r.drawUIOverlay(r.screen, deviceScale); err != nil {
			return err
		}
	}
	if drawer, ok := r.game.(overlayDrawer); ok {
		drawer.DrawOverlay(r.screen)
	}
	if err := r.drawFPSMeter(r.screen, deviceScale); err != nil {
		return err
	}
	// Goro redraws the 3D scene every frame; UI canvas damage only scopes UI texture updates.
	ctx.SetDamageRects(nil)
	framebufferW, framebufferH = ctx.FramebufferSize()
	if framebufferW <= 0 || framebufferH <= 0 {
		framebufferW, framebufferH = width, height
	}
	pts := time.Duration(0)
	if r.capture != nil && !r.capture.recordStarted.IsZero() {
		pts = time.Since(r.capture.recordStarted)
	}
	submitted, err := r.gpu.DrawTargetWithCapture(
		FrameTarget{View: surface, Texture: surface.Texture(), Width: framebufferW, Height: framebufferH, Format: r.gpu.format},
		r.screen,
		func(encoder *wgpu.CommandEncoder) error {
			if r.capture != nil {
				r.capture.encode(encoder, surface.Texture(), framebufferW, framebufferH, r.gpu.format, pts)
			}
			return nil
		},
		func() {
			if r.capture != nil {
				r.capture.afterSubmit()
			}
		},
	)
	if err != nil {
		return err
	}
	if submitted {
		if receiver, ok := r.game.(frameSubmittedReceiver); ok {
			receiver.FrameSubmitted()
		}
	}
	drawDur := time.Since(drawStart)
	totalDur := r.lastUpdateDuration + drawDur
	if totalDur > slowFrameDiagnosticThreshold {
		glog.Errorf(
			"slow frame frame=%d total_ms=%.2f threshold_ms=%.2f update_ms=%.2f game_update_ms=%.2f ui_frame_ms=%.2f draw_ms=%.2f ui_work=%t ui_redraw=%t ui_draw_ms=%.2f ui_canvas_ms=%.2f ui_flush_ms=%.2f ui_image_ms=%.2f ui_dirty_regions=%d ui_full_repaint=%t ui_union=%.0f,%.0f %.0fx%.0f",
			r.frames,
			durationMS(totalDur),
			durationMS(slowFrameDiagnosticThreshold),
			durationMS(r.lastUpdateDuration),
			durationMS(r.lastGameUpdateDur),
			durationMS(r.lastUIFrameDur),
			durationMS(drawDur),
			r.lastUIWork,
			r.lastUIRedraw,
			durationMS(r.lastUIDrawDur),
			durationMS(r.lastUICanvasDrawDur),
			durationMS(r.lastUIFlushDur),
			durationMS(r.lastUIImageDur),
			r.lastUIDirtyRegions,
			r.lastUIFullRepaint,
			r.lastUIDirtyUnion.Min.X,
			r.lastUIDirtyUnion.Min.Y,
			r.lastUIDirtyUnion.Width(),
			r.lastUIDirtyUnion.Height(),
		)
	}
	r.recordUIProfile(drawDur, totalDur)
	r.frames++
	if !r.measureStarted.IsZero() {
		r.measuredFrames++
	}
	r.updateFPSCounter(time.Now())
	r.finishCapture()
	return nil
}

func framebufferScale(width, height, framebufferW, framebufferH int) (float32, float32) {
	scaleX, scaleY := float32(1), float32(1)
	if width > 0 && framebufferW > 0 {
		scaleX = float32(framebufferW) / float32(width)
	}
	if height > 0 && framebufferH > 0 {
		scaleY = float32(framebufferH) / float32(height)
	}
	return scaleX, scaleY
}

func durationMS(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func avgDurationMS(sum time.Duration, count int64) float64 {
	if count <= 0 {
		return 0
	}
	return float64(sum.Microseconds()) / 1000 / float64(count)
}

func maxDuration(a, b time.Duration) time.Duration {
	if b > a {
		return b
	}
	return a
}

func (r *runner) recordUIProfile(drawDur, totalDur time.Duration) {
	if r == nil || !r.renderCfg.UIProfile {
		return
	}
	now := time.Now()
	stats := &r.uiProfile
	if stats.frames == 0 {
		stats.start = now
		stats.lastLog = now
	}
	stats.frames++
	if totalDur > 16*time.Millisecond {
		stats.slowFrames++
	}
	if r.lastUIFrameDur+r.lastUIDrawDur > 16*time.Millisecond {
		stats.uiSlowFrames++
	}
	if r.lastUIWork {
		stats.uiWorkFrames++
	}
	if r.lastUIRedraw {
		stats.uiRedrawFrames++
	}
	if r.lastUIFullRepaint {
		stats.uiFullRepaints++
	}
	stats.dirtyRegionSum += int64(r.lastUIDirtyRegions)
	if r.lastUIDirtyRegions > stats.dirtyRegionMax {
		stats.dirtyRegionMax = r.lastUIDirtyRegions
	}
	stats.totalDurSum += totalDur
	stats.updateDurSum += r.lastUpdateDuration
	stats.gameUpdateDurSum += r.lastGameUpdateDur
	stats.uiFrameDurSum += r.lastUIFrameDur
	stats.drawDurSum += drawDur
	stats.uiDrawDurSum += r.lastUIDrawDur
	stats.uiCanvasDurSum += r.lastUICanvasDrawDur
	stats.uiFlushDurSum += r.lastUIFlushDur
	stats.uiImageDurSum += r.lastUIImageDur
	stats.totalDurMax = maxDuration(stats.totalDurMax, totalDur)
	stats.updateDurMax = maxDuration(stats.updateDurMax, r.lastUpdateDuration)
	stats.gameUpdateDurMax = maxDuration(stats.gameUpdateDurMax, r.lastGameUpdateDur)
	stats.uiFrameDurMax = maxDuration(stats.uiFrameDurMax, r.lastUIFrameDur)
	stats.drawDurMax = maxDuration(stats.drawDurMax, drawDur)
	stats.uiDrawDurMax = maxDuration(stats.uiDrawDurMax, r.lastUIDrawDur)
	stats.uiCanvasDurMax = maxDuration(stats.uiCanvasDurMax, r.lastUICanvasDrawDur)
	stats.uiFlushDurMax = maxDuration(stats.uiFlushDurMax, r.lastUIFlushDur)
	stats.uiImageDurMax = maxDuration(stats.uiImageDurMax, r.lastUIImageDur)
	if now.Sub(stats.lastLog) >= time.Second {
		r.logUIProfile(now, false)
		stats.lastLog = now
	}
}

func (r *runner) logUIProfile(now time.Time, final bool) {
	if r == nil || !r.renderCfg.UIProfile || r.uiProfile.frames == 0 {
		return
	}
	stats := &r.uiProfile
	elapsed := now.Sub(stats.start).Seconds()
	fps := 0.0
	if elapsed > 0 {
		fps = float64(stats.frames) / elapsed
	}
	label := "ui profile"
	if final {
		label = "ui profile final"
	}
	glog.Infof(
		"%s elapsed=%.1fs frames=%d fps=%.1f slow_frames=%d ui_slow_frames=%d ui_work_frames=%d ui_redraw_frames=%d ui_full_repaints=%d avg_total_ms=%.2f max_total_ms=%.2f avg_update_ms=%.2f max_update_ms=%.2f avg_game_update_ms=%.2f max_game_update_ms=%.2f avg_draw_ms=%.2f max_draw_ms=%.2f avg_ui_frame_ms=%.2f max_ui_frame_ms=%.2f avg_ui_draw_ms=%.2f max_ui_draw_ms=%.2f avg_ui_draw_work_ms=%.2f avg_ui_canvas_ms=%.2f max_ui_canvas_ms=%.2f avg_ui_flush_ms=%.2f max_ui_flush_ms=%.2f avg_ui_image_ms=%.2f max_ui_image_ms=%.2f avg_dirty_regions=%.2f max_dirty_regions=%d",
		label,
		elapsed,
		stats.frames,
		fps,
		stats.slowFrames,
		stats.uiSlowFrames,
		stats.uiWorkFrames,
		stats.uiRedrawFrames,
		stats.uiFullRepaints,
		avgDurationMS(stats.totalDurSum, stats.frames),
		durationMS(stats.totalDurMax),
		avgDurationMS(stats.updateDurSum, stats.frames),
		durationMS(stats.updateDurMax),
		avgDurationMS(stats.gameUpdateDurSum, stats.frames),
		durationMS(stats.gameUpdateDurMax),
		avgDurationMS(stats.drawDurSum, stats.frames),
		durationMS(stats.drawDurMax),
		avgDurationMS(stats.uiFrameDurSum, stats.frames),
		durationMS(stats.uiFrameDurMax),
		avgDurationMS(stats.uiDrawDurSum, stats.frames),
		durationMS(stats.uiDrawDurMax),
		avgDurationMS(stats.uiDrawDurSum, stats.uiWorkFrames),
		avgDurationMS(stats.uiCanvasDurSum, stats.frames),
		durationMS(stats.uiCanvasDurMax),
		avgDurationMS(stats.uiFlushDurSum, stats.frames),
		durationMS(stats.uiFlushDurMax),
		avgDurationMS(stats.uiImageDurSum, stats.frames),
		durationMS(stats.uiImageDurMax),
		float64(stats.dirtyRegionSum)/float64(stats.frames),
		stats.dirtyRegionMax,
	)
}

func (r *runner) resetUIDrawMeasurement() {
	r.lastUIWork = false
	r.lastUIRedraw = false
	r.lastUIDrawDur = 0
	r.lastUICanvasDrawDur = 0
	r.lastUIFlushDur = 0
	r.lastUIImageDur = 0
	r.lastUIDirtyRegions = 0
	r.lastUIFullRepaint = false
	r.lastUIDirtyUnion = geometry.Rect{}
	r.lastUIDrawStats = widget.DrawStats{}
}

func (r *runner) drawUI(screen *Frame, width, height int, deviceScale float64) error {
	if r.renderCfg.NoUI {
		return nil
	}
	if r.ui == nil || screen == nil || width <= 0 || height <= 0 {
		return nil
	}
	if r.renderCfg.AsyncUI {
		return r.drawUIAsync(screen, width, height, deviceScale)
	}
	return r.drawUISync(screen, width, height, deviceScale)
}

func (r *runner) drawUISync(screen *Frame, width, height int, deviceScale float64) error {
	provider := r.app.GPUContextProvider()
	if provider == nil {
		return nil
	}
	if deviceScale <= 0 {
		deviceScale = 1
	}
	if r.uiCanvas == nil {
		canvas, err := ggcanvas.NewWithScale(provider, width, height, deviceScale)
		if err != nil {
			return fmt.Errorf("create ui canvas: %w", err)
		}
		r.uiCanvas = canvas
		r.uiScale = deviceScale
	}
	canvasW, canvasH := r.uiCanvas.Size()
	if canvasW != width || canvasH != height {
		if err := r.uiCanvas.Resize(width, height); err != nil {
			return fmt.Errorf("resize ui canvas: %w", err)
		}
		r.uiDrawnOnce = false
		r.setUIImage(nil)
		r.requestUIRedraw()
	}
	if !sameUIScale(r.uiScale, deviceScale) {
		r.uiCanvas.SetDeviceScale(deviceScale)
		r.uiScale = deviceScale
		r.uiDrawnOnce = false
		r.setUIImage(nil)
		r.requestUIRedraw()
	}

	win := r.ui.Window()
	if win == nil {
		return nil
	}
	needsWork := !r.uiDrawnOnce || win.NeedsRedraw() || win.HasDirtyBoundaries() || win.NeedsAnimationFrame()
	if needsWork {
		r.lastUIWork = true
		uiStart := time.Now()
		win.ClearAnimationFrame()
		drawn := false
		canvasStart := time.Now()
		if err := r.uiCanvas.Draw(func(cc *gg.Context) {
			baseCanvas := uirender.NewCanvas(cc, width, height)
			canvas := widget.Canvas(scaledImageCanvas{Canvas: baseCanvas, scale: float32(deviceScale)})
			if textMode, ok := baseCanvas.(widget.TextModeController); ok {
				textMode.SetTextMode(widget.TextModeVector)
				defer textMode.SetTextMode(widget.TextModeAuto)
			}
			drawn = win.DrawTo(canvas)
		}); err != nil {
			return fmt.Errorf("draw ui canvas: %w", err)
		}
		canvasDur := time.Since(canvasStart)
		var flushDur time.Duration
		var imageDur time.Duration
		if drawn {
			r.lastUIRedraw = true
			r.uiDrawnOnce = true
			flushStart := time.Now()
			if _, err := r.uiCanvas.Flush(); err != nil {
				return fmt.Errorf("flush ui canvas: %w", err)
			}
			flushDur = time.Since(flushStart)
			imageStart := time.Now()
			r.updateUIImage()
			r.completeUIDragLayerRelease()
			imageDur = time.Since(imageStart)
		}
		uiDur := time.Since(uiStart)
		r.lastUIDrawDur += uiDur
		r.lastUICanvasDrawDur += canvasDur
		r.lastUIFlushDur += flushDur
		r.lastUIImageDur += imageDur
		r.lastUIDirtyRegions = win.DirtyRegionCount()
		r.lastUIFullRepaint = win.WasFullRepaint()
		r.lastUIDirtyUnion = win.LastDirtyUnion()
		r.lastUIDrawStats = win.LastDrawStats()
		if uiDur > slowFrameDiagnosticThreshold {
			union := win.LastDirtyUnion()
			glog.Errorf(
				"slow ui redraw ms=%.2f threshold_ms=%.2f canvas_ms=%.2f flush_ms=%.2f image_ms=%.2f drawn=%t dirty_regions=%d full=%t union=%.0f,%.0f %.0fx%.0f stats=%+v",
				durationMS(uiDur),
				durationMS(slowFrameDiagnosticThreshold),
				durationMS(canvasDur),
				durationMS(flushDur),
				durationMS(imageDur),
				drawn,
				win.DirtyRegionCount(),
				win.WasFullRepaint(),
				union.Min.X,
				union.Min.Y,
				union.Width(),
				union.Height(),
				win.LastDrawStats(),
			)
		}
	}
	return r.drawUIPublishedImage(screen, width, height)
}

func (r *runner) drawUIAsync(screen *Frame, width, height int, deviceScale float64) error {
	if deviceScale <= 0 {
		deviceScale = 1
	}
	win := r.ui.Window()
	if win == nil {
		return nil
	}
	r.updateUIRasterSurface(width, height, deviceScale)

	needsWork := !r.uiDrawnOnce || win.NeedsRedraw() || win.HasDirtyBoundaries() || win.NeedsAnimationFrame()
	if r.shouldRecordAsyncUI(needsWork) {
		r.lastUIWork = true
		uiStart := time.Now()
		win.ClearAnimationFrame()
		drawn := false
		canvasStart := time.Now()
		recorder := newUIDrawRecorder(width, height, deviceScale)
		recorder.setTextMode(widget.TextModeVector)
		drawn = win.DrawTo(recorder)
		recorder.setTextMode(widget.TextModeAuto)
		list := recorder.list()
		recorder.close()
		canvasDur := time.Since(canvasStart)
		if drawn {
			r.lastUIRedraw = true
			r.enqueueUIDrawList(list)
		}
		uiDur := time.Since(uiStart)
		r.lastUIDrawDur += uiDur
		r.lastUICanvasDrawDur += canvasDur
		r.lastUIDirtyRegions = win.DirtyRegionCount()
		r.lastUIFullRepaint = win.WasFullRepaint()
		r.lastUIDirtyUnion = win.LastDirtyUnion()
		r.lastUIDrawStats = win.LastDrawStats()
		if uiDur > slowFrameDiagnosticThreshold {
			union := win.LastDirtyUnion()
			glog.Errorf(
				"slow ui record ms=%.2f threshold_ms=%.2f canvas_ms=%.2f drawn=%t dirty_regions=%d full=%t union=%.0f,%.0f %.0fx%.0f stats=%+v",
				durationMS(uiDur),
				durationMS(slowFrameDiagnosticThreshold),
				durationMS(canvasDur),
				drawn,
				win.DirtyRegionCount(),
				win.WasFullRepaint(),
				union.Min.X,
				union.Min.Y,
				union.Width(),
				union.Height(),
				win.LastDrawStats(),
			)
		}
	}
	// Record current UI changes before collecting the worker's result. If the
	// completed image is now obsolete, enqueueUIDrawList has put its successor
	// in uiPendingLists and collectAsyncUIResults can avoid publishing it.
	r.collectAsyncUIResults(width, height, deviceScale)
	return r.drawUIPublishedImage(screen, width, height)
}

func (r *runner) drawUIPublishedImage(screen *Frame, width, height int) error {
	if r.uiDrawnOnce && r.uiImage != nil {
		var opts DrawImageOptions
		if b := r.uiImage.Bounds(); b.Dx() > 0 && b.Dy() > 0 {
			scale := 1.0
			if provider, ok := r.game.(uiScaleProvider); ok {
				scale = float64(provider.UISettings().Normalized().Scale)
			}
			baseX := float64(width) / float64(b.Dx())
			baseY := float64(height) / float64(b.Dy())
			opts.GeoM.Scale(baseX*scale, baseY*scale)
			opts.GeoM.Translate(float64(width)*(1-scale)/2, float64(height)*(1-scale)/2)
		}
		opts.Filter = FilterNearest
		screen.DrawImage(r.uiImage, &opts)
	}
	r.drawUIDragLayer(screen)
	return nil
}

func (r *runner) updateUIRasterSurface(width, height int, deviceScale float64) {
	if r.uiLogicalWidth == width && r.uiLogicalHeight == height && sameUIScale(r.uiScale, deviceScale) {
		return
	}
	r.uiLogicalWidth = width
	r.uiLogicalHeight = height
	r.uiScale = deviceScale
	r.uiGeneration++
	r.uiDrawnOnce = false
	r.setUIImage(nil)
	r.uiPendingLists = nil
	r.requestUIRedraw()
}

func (r *runner) ensureAsyncUIRasterizer() *asyncUIRasterizer {
	if r.uiAsync == nil {
		r.uiAsync = newAsyncUIRasterizer()
	}
	return r.uiAsync
}

func (r *runner) enqueueUIDrawList(list uiDrawList) {
	if r == nil || len(list.ops) == 0 {
		return
	}
	list.generation = r.uiGeneration
	if r.uiAsyncBusy {
		if len(r.uiPendingLists) == 0 {
			r.uiPendingLists = append(r.uiPendingLists, list)
		} else {
			r.uiPendingLists[0] = list
			r.uiPendingLists = r.uiPendingLists[:1]
		}
		return
	}
	r.submitUIDrawList(list)
}

func (r *runner) submitUIDrawList(list uiDrawList) {
	rasterizer := r.ensureAsyncUIRasterizer()
	job := uiRasterJob{list: list}
	if rasterizer.submit(job) {
		r.uiAsyncBusy = true
		return
	}
	r.requestUIRedraw()
}

func (r *runner) submitPendingUIDrawLists() {
	if r == nil || r.uiAsyncBusy || len(r.uiPendingLists) == 0 {
		return
	}
	list := r.uiPendingLists[len(r.uiPendingLists)-1]
	r.uiPendingLists = nil
	r.submitUIDrawList(list)
}

func (r *runner) collectAsyncUIResults(width, height int, deviceScale float64) {
	if r.uiAsync == nil {
		return
	}
	for {
		select {
		case result := <-r.uiAsync.done:
			r.uiAsyncBusy = false
			if result.err != nil {
				glog.Warnf("async ui raster failed: %v", result.err)
				r.uiGeneration++
				r.uiDrawnOnce = false
				r.setUIImage(nil)
				r.uiPendingLists = nil
				r.requestUIRedraw()
				continue
			}
			if result.generation != r.uiGeneration || result.width != width || result.height != height || !sameUIScale(result.scale, deviceScale) {
				// Stale work still updates the rasterizer's retained canvas. Keep
				// draining queued lists in order, but publish only the current
				// generation.
				r.submitPendingUIDrawLists()
				continue
			}
			// The rasterizer must replay every incremental draw list, but an
			// intermediate image must not reach the screen after the UI has
			// already changed again. Keep displaying the last coherent image
			// until the worker catches up.
			if len(r.uiPendingLists) == 0 {
				imageStart := time.Now()
				r.setUIImage(result.image)
				r.uiDrawnOnce = r.uiImage != nil
				r.completeUIDragLayerRelease()
				r.lastUIImageDur += time.Since(imageStart)
			}
			if r.renderCfg.UIProfile && result.rasterDur > 16*time.Millisecond {
				glog.Debugf(
					"async ui raster ms=%.2f canvas_ms=%.2f flush_ms=%.2f image_ms=%.2f generation=%d size=%dx%d scale=%.2f",
					durationMS(result.rasterDur),
					durationMS(result.canvasDur),
					durationMS(result.flushDur),
					durationMS(result.imageDur),
					result.generation,
					result.width,
					result.height,
					result.scale,
				)
			}
			r.submitPendingUIDrawLists()
		default:
			return
		}
	}
}

func (r *runner) stopAsyncUIRasterizer() {
	if r == nil || r.uiAsync == nil {
		return
	}
	r.uiAsync.stop()
	r.uiAsync = nil
	r.uiAsyncBusy = false
	r.uiPendingLists = nil
}

func (r *runner) beginUIDragLayer(token any, rect geometry.Rect) bool {
	if r == nil || token == nil || rect.IsEmpty() || r.uiDrag.active {
		return false
	}
	capture := r.captureUIImageRect(rect)
	if capture.image == nil {
		return false
	}
	r.setUIDragLayer(uiDragLayer{
		token:       token,
		rect:        rect,
		image:       capture.image,
		active:      true,
		drawOffsetX: capture.rect.Min.X - rect.Min.X,
		drawOffsetY: capture.rect.Min.Y - rect.Min.Y,
		drawWidth:   capture.rect.Width(),
		drawHeight:  capture.rect.Height(),
	})
	return true
}

func (r *runner) moveUIDragLayer(token any, rect geometry.Rect) {
	if r == nil || token == nil || rect.IsEmpty() || !r.uiDrag.active || r.uiDrag.token != token {
		return
	}
	r.uiDrag.rect = rect
}

func (r *runner) endUIDragLayer(token any) {
	if r == nil || token == nil || !r.uiDrag.active || r.uiDrag.token != token {
		return
	}
	if r.uiDrag.releasePending {
		return
	}
	// Keep drawing the captured window until the restored UI has finished
	// rasterizing. Results already in flight contain the hidden overlay, so
	// move to a new generation and reject them during the handoff.
	r.uiGeneration++
	r.uiDrag.releasePending = true
}

func (r *runner) cancelUIDragLayer(token any) {
	if r == nil || token == nil || !r.uiDrag.active || r.uiDrag.token != token {
		return
	}
	r.setUIDragLayer(uiDragLayer{})
}

func (r *runner) completeUIDragLayerRelease() {
	if r != nil && r.uiDrawnOnce && r.uiImage != nil && r.uiDrag.active && r.uiDrag.releasePending {
		r.setUIDragLayer(uiDragLayer{})
	}
}

func (r *runner) captureUIImageRect(rect geometry.Rect) uiImageRectCapture {
	if r.uiImage == nil || r.uiImage.pix == nil {
		return uiImageRectCapture{}
	}
	srcBounds := r.uiImage.Bounds()
	if srcBounds.Empty() {
		return uiImageRectCapture{}
	}
	logicalW, logicalH := r.width, r.height
	if logicalW <= 0 {
		logicalW = srcBounds.Dx()
	}
	if logicalH <= 0 {
		logicalH = srcBounds.Dy()
	}
	scaleX := float64(srcBounds.Dx()) / float64(logicalW)
	scaleY := float64(srcBounds.Dy()) / float64(logicalH)
	crop := image.Rect(
		int(math.Floor(float64(rect.Min.X)*scaleX)),
		int(math.Floor(float64(rect.Min.Y)*scaleY)),
		int(math.Ceil(float64(rect.Max.X)*scaleX)),
		int(math.Ceil(float64(rect.Max.Y)*scaleY)),
	).Intersect(srcBounds)
	if crop.Empty() {
		return uiImageRectCapture{}
	}
	img := NewImage(crop.Dx(), crop.Dy())
	draw.Draw(img.pix, img.pix.Bounds(), r.uiImage.pix, crop.Min, draw.Src)
	img.version++
	logicalRect := geometry.Rect{
		Min: geometry.Pt(float32(float64(crop.Min.X)/scaleX), float32(float64(crop.Min.Y)/scaleY)),
		Max: geometry.Pt(float32(float64(crop.Max.X)/scaleX), float32(float64(crop.Max.Y)/scaleY)),
	}
	return uiImageRectCapture{image: img, rect: logicalRect}
}

func (r *runner) drawUIDragLayer(screen *Frame) {
	if r == nil || screen == nil || !r.uiDrag.active || r.uiDrag.image == nil || r.uiDrag.rect.IsEmpty() {
		return
	}
	bounds := r.uiDrag.image.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	drawRect := r.uiDrag.drawRect()
	if drawRect.IsEmpty() {
		return
	}
	var opts DrawImageOptions
	opts.GeoM.Scale(float64(drawRect.Width())/float64(bounds.Dx()), float64(drawRect.Height())/float64(bounds.Dy()))
	opts.GeoM.Translate(float64(drawRect.Min.X), float64(drawRect.Min.Y))
	opts.Filter = FilterNearest
	screen.DrawImage(r.uiDrag.image, &opts)
}

func (d uiDragLayer) drawRect() geometry.Rect {
	width, height := d.drawWidth, d.drawHeight
	if width <= 0 {
		width = d.rect.Width()
	}
	if height <= 0 {
		height = d.rect.Height()
	}
	return geometry.NewRect(d.rect.Min.X+d.drawOffsetX, d.rect.Min.Y+d.drawOffsetY, width, height)
}

func (r *runner) updateUIImage() {
	r.setUIImage(updateCanvasImage(r.uiCanvas, r.uiImage))
}

func (r *runner) setUIImage(img *Image) {
	if r == nil || r.uiImage == img {
		return
	}
	if r.gpu != nil {
		r.gpu.releaseImageTexture(r.uiImage)
	}
	r.uiImage = img
}

func (r *runner) discardPublishedUI() {
	if r == nil {
		return
	}
	// Do not stop the worker: generation matching cheaply rejects its result,
	// while keeping the async renderer warm for the next non-empty root.
	r.uiGeneration++
	r.uiDrawnOnce = false
	r.uiPendingLists = nil
	r.setUIDragLayer(uiDragLayer{})
	r.setUIImage(nil)
}

func (r *runner) setUIDragLayer(layer uiDragLayer) {
	if r == nil {
		return
	}
	old := r.uiDrag.image
	if old != nil && old != layer.image && old != r.uiImage && r.gpu != nil {
		r.gpu.releaseImageTexture(old)
	}
	r.uiDrag = layer
}

func (r *runner) shouldRecordAsyncUI(needsWork bool) bool {
	if !needsWork {
		return false
	}
	if r != nil && r.uiAsyncBusy && len(r.uiPendingLists) > 0 {
		r.lastUIWork = true
		return false
	}
	return true
}

func (r *runner) requestUIRedraw() {
	if r == nil || r.ui == nil || r.ui.Window() == nil {
		return
	}
	win := r.ui.Window()
	if root := win.Root(); root != nil {
		widget.MarkRedrawInTree(root)
	}
	if ctx := win.Context(); ctx != nil {
		ctx.Invalidate()
	}
}

func updateCanvasImage(canvas *ggcanvas.Canvas, dstImage *Image) *Image {
	if canvas == nil || canvas.Context() == nil {
		return dstImage
	}
	return imageFromGGContext(canvas.Context(), dstImage)
}

func (r *runner) drawUIOverlay(screen *Frame, deviceScale float64) error {
	if screen == nil || (len(screen.uiRects) == 0 && len(screen.uiTextBoxes) == 0 && len(screen.uiTextLabels) == 0 && len(screen.uiActorLabels) == 0) {
		return nil
	}
	defer screen.clearUIOverlayCommands()
	provider := r.app.GPUContextProvider()
	if provider == nil {
		return nil
	}
	if deviceScale <= 0 {
		deviceScale = 1
	}
	if r.uiOverlayScale != deviceScale {
		r.uiOverlayScale = deviceScale
		r.uiTextCache = nil
		r.uiBubbleCache = nil
	}
	for _, rect := range screen.uiRects {
		DrawRect(screen, rect.X, rect.Y, rect.W, rect.H, rect.Color)
	}
	for _, box := range screen.uiTextBoxes {
		cached, err := r.cachedTextBoxImage(provider, box, deviceScale)
		if err != nil {
			return err
		}
		x, y := uiTextBoxPosition(screen, box, cached)
		drawCachedOverlayImage(screen, cached, x, y)
	}
	for _, label := range screen.uiTextLabels {
		cached, err := r.cachedTextLabelImage(provider, label, deviceScale)
		if err != nil {
			return err
		}
		x := label.X
		if label.Centered {
			x -= float64(cached.width) / 2
		}
		drawCachedOverlayImage(screen, cached, x, label.Y)
	}
	for _, label := range screen.uiActorLabels {
		cached, err := r.cachedActorLabelImage(provider, label, deviceScale)
		if err != nil {
			return err
		}
		drawActorLabelOverlay(screen, cached, label)
	}
	return nil
}

func uiTextBoxPosition(screen *Frame, box UITextBoxCommand, cached cachedOverlayImage) (float64, float64) {
	switch box.Anchor {
	case UITextBoxAnchorBottomCenter:
		return box.X - float64(cached.width)/2, box.Y - float64(cached.height)
	case UITextBoxAnchorTooltipCenter:
		screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()
		x := box.X - float64(cached.width)/2
		y := box.Y
		if y+float64(cached.height)+8 > float64(screenH) && box.AltY > 0 {
			y = box.AltY - float64(cached.height)
		}
		x = clampOverlayFloat64(x, 8, maxOverlayFloat64(8, float64(screenW-cached.width-8)))
		y = clampOverlayFloat64(y, 8, maxOverlayFloat64(8, float64(screenH-cached.height-8)))
		return x, y
	default:
		return box.X, box.Y
	}
}

func clampOverlayFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxOverlayFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func drawCachedOverlayImage(screen *Frame, cached cachedOverlayImage, x, y float64) {
	if cached.image == nil || cached.width <= 0 || cached.height <= 0 {
		return
	}
	x, y = snapScreenPoint(screen, x, y)
	var opts DrawImageOptions
	if b := cached.image.Bounds(); b.Dx() > 0 && b.Dy() > 0 {
		opts.GeoM.Scale(float64(cached.width)/float64(b.Dx()), float64(cached.height)/float64(b.Dy()))
	}
	opts.GeoM.Translate(x, y)
	opts.Filter = FilterNearest
	screen.DrawImage(cached.image, &opts)
}

const (
	actorLabelEmblemSize = 24
	actorLabelEmblemGap  = 8
)

func drawActorLabelOverlay(screen *Frame, cached cachedOverlayImage, label UIActorLabelCommand) {
	if screen == nil || cached.image == nil || cached.width <= 0 || cached.height <= 0 {
		return
	}
	emblemWidth := 0
	if label.Emblem != nil {
		emblemWidth = actorLabelEmblemSize + actorLabelEmblemGap
	}
	blockLeft := label.CenterX - float64(cached.width+emblemWidth)/2
	blockLeft, blockTop := snapScreenPoint(screen, blockLeft, label.Y)

	var textOpts DrawImageOptions
	if bounds := cached.image.Bounds(); bounds.Dx() > 0 && bounds.Dy() > 0 {
		textOpts.GeoM.Scale(float64(cached.width)/float64(bounds.Dx()), float64(cached.height)/float64(bounds.Dy()))
	}
	textOpts.GeoM.Translate(blockLeft+float64(emblemWidth), blockTop)
	textOpts.Filter = FilterNearest
	screen.DrawImage(cached.image, &textOpts)

	if label.Emblem == nil {
		return
	}
	bounds := label.Emblem.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	emblemY := blockTop + float64(cached.height-actorLabelEmblemSize)/2
	if cached.height < actorLabelEmblemSize {
		emblemY = blockTop
	}
	var emblemOpts DrawImageOptions
	emblemOpts.GeoM.Scale(float64(actorLabelEmblemSize)/float64(bounds.Dx()), float64(actorLabelEmblemSize)/float64(bounds.Dy()))
	emblemOpts.GeoM.Translate(blockLeft+2, emblemY)
	emblemOpts.Filter = FilterLinear
	screen.DrawImage(label.Emblem, &emblemOpts)
}

func (r *runner) cachedTextLabelImage(provider gpucontext.DeviceProvider, label UITextLabelCommand, deviceScale float64) (cachedOverlayImage, error) {
	size := label.Size
	if size <= 0 {
		size = rotheme.Default.Typography.TextSize
	}
	key := fmt.Sprintf("text|%.3f|%s|%t|%.1f|%08x|%08x", deviceScale, label.Text, label.Bold, size, rgbaKey(label.Foreground), rgbaKey(label.Outline))
	if cached, ok := r.uiTextCache[key]; ok {
		return cached, nil
	}
	measure, err := r.ensureOverlayCanvas(provider, 1, 1, deviceScale)
	if err != nil {
		return cachedOverlayImage{}, err
	}
	var textW float32
	if err := measure.Draw(func(cc *gg.Context) {
		canvas := uirender.NewCanvas(cc, 1, 1)
		textW = rotheme.MeasureText(canvas, label.Text, size, label.Bold)
	}); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("measure ui text overlay: %w", err)
	}
	width := int(textW + 4.999)
	if width < 4 {
		width = 4
	}
	height := int(size + 8)
	if height < 16 {
		height = 16
	}
	canvas, err := r.ensureOverlayCanvas(provider, width, height, deviceScale)
	if err != nil {
		return cachedOverlayImage{}, err
	}
	fg := widget.RGBA8(label.Foreground.R, label.Foreground.G, label.Foreground.B, label.Foreground.A)
	outline := widget.RGBA8(label.Outline.R, label.Outline.G, label.Outline.B, label.Outline.A)
	if err := canvas.Draw(func(cc *gg.Context) {
		uiCanvas := uirender.NewCanvas(cc, width, height)
		uiCanvas.Clear(widget.RGBA8(0, 0, 0, 0))
		if textMode, ok := uiCanvas.(widget.TextModeController); ok {
			textMode.SetTextMode(widget.TextModeVector)
			defer textMode.SetTextMode(widget.TextModeAuto)
		}
		bounds := geometry.NewRect(2, 2, float32(width), float32(height))
		if label.Outline.A != 0 {
			for _, offset := range [][2]float32{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
				rotheme.DrawText(uiCanvas, label.Text, bounds.TranslateXY(offset[0], offset[1]), size, outline, label.Bold, widget.TextAlignLeft)
			}
		}
		rotheme.DrawText(uiCanvas, label.Text, bounds, size, fg, label.Bold, widget.TextAlignLeft)
	}); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("draw ui text overlay: %w", err)
	}
	if _, err := canvas.Flush(); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("flush ui text overlay: %w", err)
	}
	cached := cachedOverlayImage{image: updateCanvasImage(canvas, nil), width: width, height: height}
	if r.uiTextCache == nil {
		r.uiTextCache = make(map[string]cachedOverlayImage)
	}
	r.uiTextCache[key] = cached
	trimOverlayImageCache(r.uiTextCache)
	return cached, nil
}

func (r *runner) cachedActorLabelImage(provider gpucontext.DeviceProvider, label UIActorLabelCommand, deviceScale float64) (cachedOverlayImage, error) {
	size := label.Size
	if size <= 0 {
		size = 12
	}
	key := fmt.Sprintf("actorlabel|%.3f|%s|%.1f|%08x|%08x", deviceScale, strings.Join(label.Labels, "\x00"), size, rgbaKey(label.Foreground), rgbaKey(label.Outline))
	if cached, ok := r.uiTextCache[key]; ok {
		return cached, nil
	}
	measure, err := r.ensureOverlayCanvas(provider, 1, 1, deviceScale)
	if err != nil {
		return cachedOverlayImage{}, err
	}
	var maxTextW float32
	if err := measure.Draw(func(cc *gg.Context) {
		canvas := uirender.NewCanvas(cc, 1, 1)
		for _, line := range label.Labels {
			if w := rotheme.MeasureText(canvas, line, size, true); w > maxTextW {
				maxTextW = w
			}
		}
	}); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("measure actor label overlay: %w", err)
	}
	width := int(maxTextW + 4.999)
	if width < 4 {
		width = 4
	}
	const lineAdvance = 14
	height := 16 + (len(label.Labels)-1)*lineAdvance + 4
	if height < 16 {
		height = 16
	}
	canvas, err := r.ensureOverlayCanvas(provider, width, height, deviceScale)
	if err != nil {
		return cachedOverlayImage{}, err
	}
	fg := widget.RGBA8(label.Foreground.R, label.Foreground.G, label.Foreground.B, label.Foreground.A)
	outline := widget.RGBA8(label.Outline.R, label.Outline.G, label.Outline.B, label.Outline.A)
	if err := canvas.Draw(func(cc *gg.Context) {
		uiCanvas := uirender.NewCanvas(cc, width, height)
		uiCanvas.Clear(widget.RGBA8(0, 0, 0, 0))
		if textMode, ok := uiCanvas.(widget.TextModeController); ok {
			textMode.SetTextMode(widget.TextModeVector)
			defer textMode.SetTextMode(widget.TextModeAuto)
		}
		for i, line := range label.Labels {
			bounds := geometry.NewRect(2, float32(2+i*lineAdvance), float32(width-4), 16)
			if label.Outline.A != 0 {
				for _, offset := range [][2]float32{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
					rotheme.DrawText(uiCanvas, line, bounds.TranslateXY(offset[0], offset[1]), size, outline, true, widget.TextAlignLeft)
				}
			}
			rotheme.DrawText(uiCanvas, line, bounds, size, fg, true, widget.TextAlignLeft)
		}
	}); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("draw actor label overlay: %w", err)
	}
	if _, err := canvas.Flush(); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("flush actor label overlay: %w", err)
	}
	cached := cachedOverlayImage{image: updateCanvasImage(canvas, nil), width: width, height: height}
	if r.uiTextCache == nil {
		r.uiTextCache = make(map[string]cachedOverlayImage)
	}
	r.uiTextCache[key] = cached
	trimOverlayImageCache(r.uiTextCache)
	return cached, nil
}

func (r *runner) cachedTextBoxImage(provider gpucontext.DeviceProvider, box UITextBoxCommand, deviceScale float64) (cachedOverlayImage, error) {
	text := strings.TrimSpace(box.Text)
	if text == "" {
		return cachedOverlayImage{}, nil
	}
	maxWidth := float32(box.MaxWidth)
	if maxWidth <= 0 {
		maxWidth = 0
	}
	maxLines := box.MaxLines
	if maxLines <= 0 {
		maxLines = 1
	}
	key := fmt.Sprintf("box|%.3f|%d|%.1f|%s", deviceScale, maxLines, maxWidth, text)
	style := consoleOverlayTextBoxStyle()
	style.maxLines = maxLines
	if maxWidth > 0 {
		style.minWidth = 28
		style.maxWidth = maxWidth
		style.wrap = true
	}
	return r.cachedOverlayTextBoxImage(provider, key, text, deviceScale, style)
}

type overlayTextBoxStyle struct {
	size       float32
	lineH      float32
	padX       float32
	padY       float32
	minWidth   float32
	maxWidth   float32
	maxLines   int
	wrap       bool
	background widget.Color
	foreground widget.Color
}

func consoleOverlayTextBoxStyle() overlayTextBoxStyle {
	return overlayTextBoxStyle{
		size:       rotheme.Default.Typography.TextSize,
		lineH:      14,
		padX:       8,
		padY:       6,
		minWidth:   1,
		maxLines:   1,
		background: widget.RGBA8(14, 18, 24, 188),
		foreground: widget.RGBA8(235, 242, 250, 255),
	}
}

func (r *runner) cachedOverlayTextBoxImage(provider gpucontext.DeviceProvider, key, text string, deviceScale float64, style overlayTextBoxStyle) (cachedOverlayImage, error) {
	if cached, ok := r.uiBubbleCache[key]; ok {
		return cached, nil
	}
	measure, err := r.ensureOverlayCanvas(provider, 1, 1, deviceScale)
	if err != nil {
		return cachedOverlayImage{}, err
	}
	lines := []string{text}
	if err := measure.Draw(func(cc *gg.Context) {
		canvas := uirender.NewCanvas(cc, 1, 1)
		lines = overlayTextBoxLines(canvas, text, style)
	}); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("measure text box overlay: %w", err)
	}
	if len(lines) == 0 {
		lines = []string{text}
	}
	if style.maxLines > 0 && len(lines) > style.maxLines {
		lines = append(lines[:style.maxLines-1], ellipsizeOverlayText(measure, strings.Join(lines[style.maxLines-1:], " "), style.size, false, style.maxWidth-style.padX*2))
	}
	textWidth := float32(0)
	if err := measure.Draw(func(cc *gg.Context) {
		canvas := uirender.NewCanvas(cc, 1, 1)
		for _, line := range lines {
			if w := rotheme.MeasureText(canvas, line, style.size, false); w > textWidth {
				textWidth = w
			}
		}
	}); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("measure text box width: %w", err)
	}
	width := int(maxFloat32(style.minWidth, textWidth+style.padX*2) + 0.999)
	if style.maxWidth > 0 && width > int(style.maxWidth) {
		width = int(style.maxWidth)
	}
	if width < 1 {
		width = 1
	}
	height := int(float32(len(lines))*style.lineH + style.padY*2 + 0.999)
	canvas, err := r.ensureOverlayCanvas(provider, width, height, deviceScale)
	if err != nil {
		return cachedOverlayImage{}, err
	}
	if err := canvas.Draw(func(cc *gg.Context) {
		uiCanvas := uirender.NewCanvas(cc, width, height)
		uiCanvas.Clear(widget.RGBA8(0, 0, 0, 0))
		if textMode, ok := uiCanvas.(widget.TextModeController); ok {
			textMode.SetTextMode(widget.TextModeVector)
			defer textMode.SetTextMode(widget.TextModeAuto)
		}
		uiCanvas.DrawRect(geometry.NewRect(0, 0, float32(width), float32(height)), style.background)
		for i, line := range lines {
			y := style.padY + float32(i)*style.lineH
			rotheme.DrawText(uiCanvas, line, geometry.NewRect(style.padX, y, float32(width)-style.padX*2, style.lineH), style.size, style.foreground, false, widget.TextAlignLeft)
		}
	}); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("draw text box overlay: %w", err)
	}
	if _, err := canvas.Flush(); err != nil {
		return cachedOverlayImage{}, fmt.Errorf("flush text box overlay: %w", err)
	}
	cached := cachedOverlayImage{image: updateCanvasImage(canvas, nil), width: width, height: height}
	if r.uiBubbleCache == nil {
		r.uiBubbleCache = make(map[string]cachedOverlayImage)
	}
	r.uiBubbleCache[key] = cached
	trimOverlayImageCache(r.uiBubbleCache)
	return cached, nil
}

func trimOverlayImageCache(cache map[string]cachedOverlayImage) {
	if len(cache) <= 512 {
		return
	}
	for key := range cache {
		delete(cache, key)
		if len(cache) <= 384 {
			return
		}
	}
}

func overlayTextBoxLines(canvas widget.Canvas, text string, style overlayTextBoxStyle) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	paragraphs := strings.Split(text, "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		if style.wrap {
			wrapped := wrapOverlayText(canvas, paragraph, style.size, false, style.maxWidth-style.padX*2)
			if len(wrapped) == 0 {
				lines = append(lines, paragraph)
			} else {
				lines = append(lines, wrapped...)
			}
			continue
		}
		lines = append(lines, paragraph)
	}
	return lines
}

func (r *runner) ensureOverlayCanvas(provider gpucontext.DeviceProvider, width, height int, deviceScale float64) (*ggcanvas.Canvas, error) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if r.uiOverlayCanvas == nil {
		canvas, err := ggcanvas.NewWithScale(provider, width, height, deviceScale)
		if err != nil {
			return nil, fmt.Errorf("create ui overlay canvas: %w", err)
		}
		r.uiOverlayCanvas = canvas
		r.uiOverlayScale = deviceScale
		return canvas, nil
	}
	canvasW, canvasH := r.uiOverlayCanvas.Size()
	if canvasW != width || canvasH != height {
		if err := r.uiOverlayCanvas.Resize(width, height); err != nil {
			return nil, fmt.Errorf("resize ui overlay canvas: %w", err)
		}
	}
	if r.uiOverlayScale != deviceScale {
		r.uiOverlayCanvas.SetDeviceScale(deviceScale)
		r.uiOverlayScale = deviceScale
		r.uiTextCache = nil
		r.uiBubbleCache = nil
	}
	return r.uiOverlayCanvas, nil
}

func wrapOverlayText(canvas widget.Canvas, text string, size float32, bold bool, maxWidth float32) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, 2)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line == "" || rotheme.MeasureText(canvas, candidate, size, bold) <= maxWidth {
			line = candidate
			continue
		}
		lines = append(lines, line)
		line = word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func ellipsizeOverlayText(canvas *ggcanvas.Canvas, text string, size float32, bold bool, maxWidth float32) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	result := text
	_ = canvas.Draw(func(cc *gg.Context) {
		uiCanvas := uirender.NewCanvas(cc, 1, 1)
		if rotheme.MeasureText(uiCanvas, result, size, bold) <= maxWidth {
			return
		}
		const suffix = "..."
		runes := []rune(result)
		for len(runes) > 0 {
			candidate := strings.TrimSpace(string(runes)) + suffix
			if rotheme.MeasureText(uiCanvas, candidate, size, bold) <= maxWidth {
				result = candidate
				return
			}
			runes = runes[:len(runes)-1]
		}
		result = suffix
	})
	return result
}

func rgbaKey(c color.RGBA) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

func (r *runner) updateFPSCounter(now time.Time) {
	if !r.renderCfg.FPS {
		return
	}
	if r.fpsStarted.IsZero() {
		r.fpsStarted = now
		r.fpsText = "FPS --"
		return
	}
	r.fpsFrames++
	elapsed := now.Sub(r.fpsStarted)
	if elapsed < time.Second {
		return
	}
	seconds := elapsed.Seconds()
	r.fpsDisplay = float64(r.fpsFrames) / seconds
	r.frameMSDisplay = seconds * 1000 / float64(r.fpsFrames)
	r.fpsFrames = 0
	r.fpsStarted = now
	r.fpsText = fmt.Sprintf("FPS %.1f  %.2f ms", r.fpsDisplay, r.frameMSDisplay)
}

func (r *runner) drawFPSMeter(screen *Frame, deviceScale float64) error {
	if !r.renderCfg.FPS || r.fpsText == "" || screen == nil {
		return nil
	}
	provider := r.app.GPUContextProvider()
	if provider == nil {
		return nil
	}
	if deviceScale <= 0 {
		deviceScale = 1
	}
	box := UITextBoxCommand{
		Text:   r.fpsText,
		X:      6,
		Y:      6,
		Anchor: UITextBoxAnchorTopLeft,
	}
	cached, err := r.cachedTextBoxImage(provider, box, deviceScale)
	if err != nil {
		return fmt.Errorf("draw fps overlay: %w", err)
	}
	x, y := uiTextBoxPosition(screen, box, cached)
	drawCachedOverlayImage(screen, cached, x, y)
	return nil
}
