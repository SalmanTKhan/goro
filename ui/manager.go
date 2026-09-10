package ui

import (
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
)

type Manager struct {
	app                              client.UIApp
	root                             *overlayRoot
	overlays                         []widget.Widget
	foreground                       []widget.Widget
	controllerKeyboard               *ControllerKeyboard
	controllerKeyboardOverlay        widget.Widget
	controllerKeyboardDismissedField *textfield.Widget
	controllerFocusNavigation        bool
	pointerTransform                 func(x, y int) (int, int)
	textInputActive                  func() bool
	controllerRebindActive           func() bool
}

func NewManager() *Manager {
	return &Manager{root: newOverlayRoot(nil)}
}

func (m *Manager) SetUIApp(app client.UIApp) {
	if m == nil || m.app == app {
		return
	}
	m.app = app
	m.apply()
}

func (m *Manager) AddOverlay(root widget.Widget) {
	if m == nil || root == nil {
		return
	}
	for _, child := range m.overlays {
		if child == root {
			return
		}
	}
	disableRootRepaintBoundary(root)
	insert := len(m.overlays) - len(m.foreground)
	m.overlays = append(m.overlays, nil)
	copy(m.overlays[insert+1:], m.overlays[insert:])
	m.overlays[insert] = root
	m.apply()
}

// AddForegroundOverlay publishes an overlay above ordinary windows. New and
// raised windows remain below it until the foreground overlay is removed.
func (m *Manager) AddForegroundOverlay(root widget.Widget) {
	if m == nil || root == nil {
		return
	}
	for _, child := range m.overlays {
		if child == root {
			return
		}
	}
	disableRootRepaintBoundary(root)
	m.overlays = append(m.overlays, root)
	m.foreground = append(m.foreground, root)
	m.apply()
}

func (m *Manager) RemoveOverlay(root widget.Widget) {
	if m == nil || root == nil {
		return
	}
	for i, child := range m.overlays {
		if child == root {
			m.overlays = append(m.overlays[:i], m.overlays[i+1:]...)
			for j, foreground := range m.foreground {
				if foreground == root {
					m.foreground = append(m.foreground[:j], m.foreground[j+1:]...)
					break
				}
			}
			m.apply()
			return
		}
	}
}

func (m *Manager) Clear() {
	if m == nil {
		return
	}
	m.overlays = nil
	m.foreground = nil
	m.controllerKeyboardDismissedField = nil
	m.controllerFocusNavigation = false
	m.apply()
}

// OpenControllerKeyboard attaches a foreground keyboard to a focused text
// field. The render bridge calls this only for controller confirmation of a
// text field, so normal mouse, keyboard, and IME flows are untouched.
func (m *Manager) OpenControllerKeyboard(target widget.Widget) bool {
	if m == nil {
		return false
	}
	field, ok := target.(*textfield.Widget)
	if !ok || !field.IsFocusable() {
		return false
	}
	if m.controllerKeyboard != nil {
		m.closeControllerKeyboard(m.controllerKeyboard)
	}
	// An explicit confirm on a field is an intentional reopen after Done or
	// Cancel, so clear the one-shot auto-open suppression recorded by close.
	m.controllerKeyboardDismissedField = nil
	m.controllerFocusNavigation = false
	k := newControllerKeyboard(m, field)
	m.controllerKeyboard = k
	m.controllerKeyboardOverlay = k.Widget()
	m.AddForegroundOverlay(m.controllerKeyboardOverlay)
	// The field remains in the underlying tree while the keyboard is open, but
	// it must not render as the active control or receive controller text.
	field.SetFocused(false)
	setControllerModeRecursive(m.controllerKeyboardOverlay, true)
	if host, ok := m.app.(interface{ FocusControllerWidget(widget.Widget) bool }); ok {
		if focus := k.firstFocusable(); focus != nil {
			host.FocusControllerWidget(focus)
		}
	}
	return true
}

// EnsureControllerKeyboard opens the on-screen keyboard for whatever text field
// currently holds focus. The renderer calls this whenever text entry becomes
// active, so a pad reaches text input without first having to focus the field
// and press confirm.
func (m *Manager) EnsureControllerKeyboard() bool {
	if m == nil || m.controllerKeyboard != nil {
		return false
	}
	field := focusedTextField(m.root)
	if field == nil {
		m.controllerKeyboardDismissedField = nil
		return false
	}
	if field == m.controllerKeyboardDismissedField {
		// Done and Cancel return focus to the field. Do not immediately reopen
		// the modal on the next update; the next explicit confirm is what opts
		// back into controller text entry.
		return false
	}
	m.controllerKeyboardDismissedField = nil
	return m.OpenControllerKeyboard(field)
}

func (m *Manager) closeControllerKeyboard(keyboard *ControllerKeyboard) {
	if m == nil || keyboard == nil || m.controllerKeyboard != keyboard {
		return
	}
	overlay := m.controllerKeyboardOverlay
	m.controllerKeyboardDismissedField = keyboard.field
	m.controllerFocusNavigation = true
	m.controllerKeyboard = nil
	m.controllerKeyboardOverlay = nil
	keyboard.Window.Close()
	if overlay != nil {
		m.RemoveOverlay(overlay)
	}
	if host, ok := m.app.(interface{ FocusControllerWidget(widget.Widget) bool }); ok && keyboard.field != nil {
		host.FocusControllerWidget(keyboard.field)
	}
}

func (m *Manager) CloseControllerKeyboard() {
	if m != nil && m.controllerKeyboard != nil {
		m.closeControllerKeyboard(m.controllerKeyboard)
	}
}

func (m *Manager) ControllerKeyboardActive() bool {
	return m != nil && m.controllerKeyboard != nil
}

// ControllerFocusNavigationActive keeps the underlying UI in focus-navigation
// mode after the modal keyboard is dismissed. This lets D-pad/stick movement
// reach the next field or checkbox without reopening the keyboard until the
// user explicitly confirms a text field again.
func (m *Manager) ControllerFocusNavigationActive() bool {
	if m == nil || !m.controllerFocusNavigation || m.controllerKeyboard != nil {
		return false
	}
	// The field that dismissed the keyboard is the lifetime anchor for this
	// temporary focus-navigation mode. Once its window is replaced (login to
	// character select, or character select to world), retaining the flag would
	// make the next world HUD steal the movement stick.
	if m.controllerKeyboardDismissedField == nil || !controllerTreeContains(m.root, m.controllerKeyboardDismissedField) {
		m.controllerFocusNavigation = false
		m.controllerKeyboardDismissedField = nil
		return false
	}
	return true
}

// ControllerUIActive reports whether the topmost visible overlay exposes at
// least one focusable control. The renderer uses this to enter focus
// navigation even when the persisted UI preference is cursor mode; controller
// screens such as character selection must be usable before a virtual pointer
// has been positioned over a button.
func (m *Manager) ControllerUIActive() bool {
	if m == nil {
		return false
	}
	for i := len(m.overlays) - 1; i >= 0; i-- {
		overlay := m.overlays[i]
		if overlay == nil {
			continue
		}
		if visible, ok := overlay.(interface{ IsVisible() bool }); ok && !visible.IsVisible() {
			continue
		}
		if enabled, ok := overlay.(interface{ IsEnabled() bool }); ok && !enabled.IsEnabled() {
			continue
		}
		// HUD overlays such as the shortcut bar remain visible above modal
		// windows, but explicitly do not own controller navigation. Continue
		// looking for the first non-passive overlay underneath them.
		if passive, ok := overlay.(interface{ ControllerNavigationPassthrough() bool }); ok && passive.ControllerNavigationPassthrough() {
			continue
		}
		if controllerTreeHasFocusable(overlay) {
			return true
		}
		// Some overlays (for example the map) intentionally expose no
		// focusable child but still own semantic controller actions such as
		// Circle-to-close. Keep those overlays modal for the controller so the
		// same press cannot fall through to gameplay.
		_, semantic := overlay.(interface {
			HandleControllerAction(input.UIAction) bool
		})
		return semantic
	}
	return false
}

func controllerTreeHasFocusable(root widget.Widget) bool {
	if root == nil {
		return false
	}
	if focus, ok := root.(widget.Focusable); ok && focus.IsFocusable() {
		return true
	}
	for _, child := range controllerChildren(root) {
		if controllerTreeHasFocusable(child) {
			return true
		}
	}
	return false
}

func controllerTreeContains(root widget.Widget, target widget.Widget) bool {
	if root == nil || target == nil {
		return false
	}
	if root == target {
		return true
	}
	for _, child := range controllerChildren(root) {
		if controllerTreeContains(child, target) {
			return true
		}
	}
	return false
}

func setControllerModeRecursive(root widget.Widget, enabled bool) {
	if root == nil {
		return
	}
	if setter, ok := root.(interface{ SetControllerMode(bool) }); ok {
		setter.SetControllerMode(enabled)
	}
	for _, child := range root.Children() {
		setControllerModeRecursive(child, enabled)
	}
}

// SetPointerTransform installs the physical-to-logical pointer conversion used
// by the UI surface. Callers pass physical window coordinates while widget
// bounds are in logical coordinates, and the two diverge whenever the UI scale
// is not 1. The renderer installs the same function it applies to real pointer
// events so the two can never disagree.
func (m *Manager) SetPointerTransform(transform func(x, y int) (int, int)) {
	if m == nil {
		return
	}
	m.pointerTransform = transform
}

// SetTextInputPredicate registers an additional source of "text entry is
// active", such as the chat console, which lives outside the widget tree. The
// predicate is injected rather than imported so ui keeps no dependency on game.
func (m *Manager) SetTextInputPredicate(active func() bool) {
	if m == nil {
		return
	}
	m.textInputActive = active
}

// SetControllerRebindPredicate registers the source of "a rebinding capture is
// waiting for a button press". The renderer suppresses controller dispatch
// while it is true so the captured press does not also fire its action. Like
// the text-input predicate this is injected, keeping ui free of a game import.
func (m *Manager) SetControllerRebindPredicate(active func() bool) {
	if m == nil {
		return
	}
	m.controllerRebindActive = active
}

// ControllerRebindActive reports whether a rebinding capture is in progress.
func (m *Manager) ControllerRebindActive() bool {
	return m != nil && m.controllerRebindActive != nil && m.controllerRebindActive()
}

func (m *Manager) PointerBlocked(x, y int) bool {
	if m == nil || m.root == nil {
		return false
	}
	if m.pointerTransform != nil {
		x, y = m.pointerTransform(x, y)
	}
	return m.root.PointerBlocked(geometry.Pt(float32(x), float32(y)))
}

// TextInputActive reports whether a text field is currently accepting input,
// either through the on-screen keyboard, a focused text field in the widget
// tree, or a registered external predicate.
func (m *Manager) TextInputActive() bool {
	if m == nil {
		return false
	}
	if m.controllerKeyboard != nil {
		return true
	}
	if m.textInputActive != nil && m.textInputActive() {
		return true
	}
	return m.root != nil && focusedTextField(m.root) != nil
}

// PointerOverUI is the single predicate deciding whether a screen point belongs
// to the UI rather than the world. Both the real mouse path and the controller
// pointer route through it so they cannot drift apart.
func (m *Manager) PointerOverUI(x, y int) bool {
	if m == nil {
		return false
	}
	return m.PointerBlocked(x, y) || m.TextInputActive()
}

// focusedTextField finds a focused text field anywhere in the tree.
func focusedTextField(root widget.Widget) *textfield.Widget {
	if root == nil {
		return nil
	}
	if field, ok := root.(*textfield.Widget); ok && field.IsFocused() {
		return field
	}
	for _, child := range controllerChildren(root) {
		if found := focusedTextField(child); found != nil {
			return found
		}
	}
	return nil
}

func (m *Manager) ViewportChanged(oldWidth, oldHeight, width, height int) {
	if m == nil || width <= 0 || height <= 0 || (oldWidth == width && oldHeight == height) {
		return
	}
	overlays := append([]widget.Widget(nil), m.overlays...)
	for _, overlay := range overlays {
		if responsive, ok := overlay.(interface {
			viewportChanged(oldWidth, oldHeight, width, height int)
		}); ok {
			responsive.viewportChanged(oldWidth, oldHeight, width, height)
		}
	}
	if m.root != nil {
		widget.MarkRedrawInTree(m.root)
		m.root.SetNeedsRedraw(true)
	}
	if m.app != nil {
		m.app.Invalidate()
	}
}

// OverlayAt returns the same topmost hit target used for pointer dispatch.
// Drag-and-drop destinations use it to avoid accepting items through a window.
func (m *Manager) OverlayAt(x, y int) widget.Widget {
	if m == nil || m.root == nil {
		return nil
	}
	position := geometry.Pt(float32(x), float32(y))
	for i := len(m.root.children) - 1; i >= 0; i-- {
		if child := m.root.children[i]; widgetCoversPoint(child, position) {
			return child
		}
	}
	return nil
}

func (m *Manager) RaiseOverlay(root widget.Widget) {
	m.raiseOverlay(root)
}

func (m *Manager) apply() {
	m.root = newOverlayRoot(m.overlays)
	m.root.onActivate = m.raiseOverlay
	if m.app != nil && m.root != nil {
		m.app.SetUIRoot(m.root)
		disableRootRepaintBoundary(m.root)
		m.root.SetNeedsRedraw(true)
	}
}

func (m *Manager) raiseOverlay(overlay widget.Widget) {
	if m == nil || overlay == nil || len(m.overlays) < 2 {
		return
	}
	top := len(m.overlays) - len(m.foreground) - 1
	if top < 0 {
		return
	}
	for i, child := range m.overlays {
		if child != overlay || i >= top {
			continue
		}
		copy(m.overlays[i:], m.overlays[i+1:top+1])
		m.overlays[top] = overlay
		if m.root != nil {
			// Keep the active root so pointer capture and keyboard focus survive.
			m.root.children = append(m.root.children[:0], m.overlays...)
			m.root.SetNeedsRedraw(true)
		}
		return
	}
}

func disableRootRepaintBoundary(root widget.Widget) {
	type boundarySetter interface{ SetRepaintBoundary(bool) }
	if rb, ok := root.(boundarySetter); ok {
		// The renderer already caches the complete UI image between dirty frames.
		// Drawing the root directly keeps rounded clipping on the normal canvas;
		// gogpu/ui's scene recorder currently degrades rounded clips to rectangles.
		rb.SetRepaintBoundary(false)
	}
}

type overlayRoot struct {
	widget.WidgetBase
	children   []widget.Widget
	onActivate func(widget.Widget)
}

func newOverlayRoot(children []widget.Widget) *overlayRoot {
	root := &overlayRoot{children: append([]widget.Widget(nil), children...)}
	root.SetVisible(true)
	root.SetEnabled(true)
	root.SetNeedsRedraw(true)
	return root
}

func (r *overlayRoot) Layout(ctx widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.Biggest()
	if size.Width <= 0 || size.Height <= 0 {
		size = constraints.Constrain(geometry.Sz(1, 1))
	}
	r.SetBounds(geometry.FromPointSize(r.Position(), size))
	for _, child := range r.children {
		child.Layout(ctx, geometry.Loose(size))
	}
	return size
}

func (r *overlayRoot) Draw(ctx widget.Context, canvas widget.Canvas) {
	if !r.IsVisible() {
		return
	}
	clip := canvas.ClipBounds()
	for _, child := range r.children {
		if !widgetIntersectsRect(child, clip) {
			continue
		}
		widget.StampScreenOrigin(child, canvas)
		widget.DrawChild(child, ctx, canvas)
	}
}

func (r *overlayRoot) Event(ctx widget.Context, e event.Event) bool {
	if !r.IsVisible() || !r.IsEnabled() {
		return false
	}
	if mouse, ok := e.(*event.MouseEvent); ok {
		return r.dispatchPositionedEvent(ctx, e, mouse.Position)
	}
	if wheel, ok := e.(*event.WheelEvent); ok {
		return r.dispatchPositionedEvent(ctx, e, wheel.Position)
	}
	for i := len(r.children) - 1; i >= 0; i-- {
		if r.children[i].Event(ctx, e) {
			return true
		}
	}
	return false
}

func (r *overlayRoot) dispatchPositionedEvent(ctx widget.Context, e event.Event, position geometry.Point) bool {
	for i := len(r.children) - 1; i >= 0; i-- {
		child := r.children[i]
		if !widgetCoversPoint(child, position) {
			continue
		}
		if mouse, ok := e.(*event.MouseEvent); ok && mouse.IsPress() && overlayRaisesOnPress(child) && r.onActivate != nil {
			r.onActivate(child)
		}
		child.Event(ctx, e)
		return true
	}
	return false
}

func overlayRaisesOnPress(overlay widget.Widget) bool {
	positioned, ok := overlay.(*positionedOverlay)
	return ok && positioned.raiseOnPress
}

func (r *overlayRoot) PointerBlocked(position geometry.Point) bool {
	if r == nil || !r.IsVisible() || !r.IsEnabled() {
		return false
	}
	for i := len(r.children) - 1; i >= 0; i-- {
		if widgetCoversPoint(r.children[i], position) {
			return true
		}
	}
	return false
}

// IsUIRootEmpty lets the render bridge discard a previously published UI
// image immediately when the last overlay is removed. This avoids retaining a
// stale asynchronous frame while the empty root is rasterized.
func (r *overlayRoot) IsUIRootEmpty() bool {
	return r == nil || len(r.children) == 0
}

func widgetCoversPoint(child widget.Widget, position geometry.Point) bool {
	if child == nil {
		return false
	}
	if visible, ok := child.(interface{ IsVisible() bool }); ok && !visible.IsVisible() {
		return false
	}
	if enabled, ok := child.(interface{ IsEnabled() bool }); ok && !enabled.IsEnabled() {
		return false
	}
	if bounds, ok := child.(interface{ Bounds() geometry.Rect }); ok {
		return bounds.Bounds().Contains(position)
	}
	if box, ok := child.(*primitives.BoxWidget); ok {
		return box.Bounds().Contains(position)
	}
	return false
}

func widgetIntersectsRect(child widget.Widget, rect geometry.Rect) bool {
	if child == nil {
		return false
	}
	if rect.IsEmpty() {
		return false
	}
	if visible, ok := child.(interface{ IsVisible() bool }); ok && !visible.IsVisible() {
		return false
	}
	if bounds, ok := child.(interface{ Bounds() geometry.Rect }); ok {
		return bounds.Bounds().Intersects(rect)
	}
	if box, ok := child.(*primitives.BoxWidget); ok {
		return box.Bounds().Intersects(rect)
	}
	return true
}

func (r *overlayRoot) Children() []widget.Widget {
	if len(r.children) == 0 {
		return nil
	}
	children := make([]widget.Widget, len(r.children))
	copy(children, r.children)
	return children
}
