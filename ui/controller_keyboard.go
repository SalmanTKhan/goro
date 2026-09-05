package ui

import (
	"strings"

	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/ui/rotheme"
)

const (
	controllerKeyboardWidth  = 520
	controllerKeyboardHeight = 300
	controllerKeyboardKeyH   = 30
)

type controllerKeyboardTarget interface {
	Text() string
	SetText(string)
}

// ControllerKeyboard is a reusable on-screen keyboard for any focused
// gogpu/ui text field. Physical keyboard and IME input continue to use the
// text field's native event path; this widget only adds controller entry.
type ControllerKeyboard struct {
	Window
	target controllerKeyboardTarget
	field  *textfield.Widget
	owner  *Manager
	shift  bool
	caps   bool
}

func newControllerKeyboard(owner *Manager, field *textfield.Widget) *ControllerKeyboard {
	k := &ControllerKeyboard{owner: owner, field: field, target: field}
	k.Window = NewWindow(controllerKeyboardWidth, controllerKeyboardHeight)
	k.SetControllerActionHandler(func(action input.UIAction) bool {
		if action != input.UIActionCancel {
			return false
		}
		k.close()
		return true
	})
	k.OpenAt(140, 72, k.widgetTree())
	return k
}

func (k *ControllerKeyboard) widgetTree() widget.Widget {
	row := func(labels string) widget.Widget {
		keys := make([]widget.Widget, 0, len(labels))
		for _, label := range labels {
			key := string(label)
			keys = append(keys, controllerKeyboardButton(key, 42, func() { k.insert(key) }))
		}
		return primitives.HBox(keys...).Gap(2).CrossAlign(primitives.CrossAxisCenter)
	}
	control := func(label string, width float32, callback func()) widget.Widget {
		return controllerKeyboardButton(label, width, callback)
	}
	return Win(
		Title("Controller Keyboard"),
		CloseButton(false),
		Size(controllerKeyboardWidth, controllerKeyboardHeight),
		Content(
			primitives.Box(
				row("1234567890"),
				row("QWERTYUIOP"),
				row("ASDFGHJKL"),
				row("ZXCVBNM"),
				primitives.HBox(
					control("Shift", 66, func() {
						k.shift = !k.shift
						k.invalidate()
					}),
					control("Caps", 66, func() {
						k.caps = !k.caps
						k.invalidate()
					}),
					control("Backspace", 104, k.backspace),
					control("Space", 104, func() { k.insert(" ") }),
				).Gap(2).CrossAlign(primitives.CrossAxisCenter),
			).
				Padding(10).
				Gap(5),
		),
		Footer(
			primitives.Expanded(primitives.Box()),
			control("Done", 72, k.submit),
			control("Cancel", 72, k.cancel),
		),
	)
}

func controllerKeyboardButton(label string, width float32, callback func()) widget.Widget {
	return primitives.Box(rotheme.Button(label, callback)).Width(width).Height(controllerKeyboardKeyH)
}

func (k *ControllerKeyboard) insert(value string) {
	if k == nil || k.target == nil {
		return
	}
	if value != " " && len([]rune(value)) == 1 {
		runeValue := []rune(value)[0]
		if k.caps != k.shift {
			runeValue = []rune(strings.ToUpper(string(runeValue)))[0]
		} else {
			runeValue = []rune(strings.ToLower(string(runeValue)))[0]
		}
		value = string(runeValue)
		if k.shift {
			k.shift = false
		}
	}
	if !k.sendFieldEvent(value, 0) {
		k.target.SetText(k.target.Text() + value)
	}
	k.invalidate()
}

func (k *ControllerKeyboard) backspace() {
	if k == nil || k.target == nil {
		return
	}
	runes := []rune(k.target.Text())
	if len(runes) == 0 {
		return
	}
	if !k.sendFieldEvent("", event.KeyBackspace) {
		k.target.SetText(string(runes[:len(runes)-1]))
	}
	k.invalidate()
}

// sendFieldEvent keeps controller text entry on the text field's native event
// path when the desktop bridge can provide a widget context. This preserves
// cursor/selection handling, change callbacks, validation, and redraw
// invalidation. Small headless tests and non-desktop hosts use the target
// fallback above instead.
func (k *ControllerKeyboard) sendFieldEvent(text string, key event.Key) bool {
	if k == nil || k.field == nil || k.owner == nil || k.owner.app == nil {
		return false
	}
	provider, ok := k.owner.app.(interface{ WidgetContext() widget.Context })
	if !ok {
		return false
	}
	ctx := provider.WidgetContext()
	if ctx == nil {
		return false
	}
	wasFocused := k.field.IsFocused()
	k.field.SetFocused(true)
	defer k.field.SetFocused(wasFocused)
	if key != 0 {
		return k.field.Event(ctx, event.NewKeyEvent(event.KeyPress, key, 0, event.ModNone))
	}
	for _, r := range text {
		if !k.field.Event(ctx, event.NewKeyEvent(event.KeyPress, 0, r, event.ModNone)) {
			return false
		}
	}
	return true
}

func (k *ControllerKeyboard) invalidate() {
	if k == nil || k.owner == nil || k.owner.app == nil {
		return
	}
	k.owner.app.Invalidate()
}

func (k *ControllerKeyboard) submit() {
	if k == nil {
		return
	}
	// Done dismisses the modal keyboard and returns focus to the field. It is
	// intentionally not an Enter/submit action: pressing Done while editing a
	// login or character field must not submit the surrounding form with a
	// partially entered value.
	k.close()
}

func (k *ControllerKeyboard) cancel() {
	if k != nil {
		k.close()
	}
}

func (k *ControllerKeyboard) close() {
	if k == nil {
		return
	}
	if k.owner != nil {
		k.owner.closeControllerKeyboard(k)
		return
	}
	k.Window.Close()
}

func (k *ControllerKeyboard) firstFocusable() widget.Widget {
	if k == nil {
		return nil
	}
	return firstControllerFocusable(k.Widget())
}

func firstControllerFocusable(root widget.Widget) widget.Widget {
	if root == nil {
		return nil
	}
	if focus, ok := root.(widget.Focusable); ok && focus.IsFocusable() {
		if child, ok := root.(widget.Widget); ok {
			return child
		}
	}
	for _, child := range controllerChildren(root) {
		if focus := firstControllerFocusable(child); focus != nil {
			return focus
		}
	}
	return nil
}

// firstControllerContentFocusable prefers controls in the body/footer of a
// window over the title-bar close button. The close button remains the
// fallback for display-only windows and is still reachable through traversal.
func firstControllerContentFocusable(root widget.Widget) widget.Widget {
	if root == nil {
		return nil
	}
	if _, titleBar := root.(*roTitleBarWidget); titleBar {
		return nil
	}
	if focus, ok := root.(widget.Focusable); ok && focus.IsFocusable() {
		return root
	}
	for _, child := range controllerChildren(root) {
		if focus := firstControllerContentFocusable(child); focus != nil {
			return focus
		}
	}
	return nil
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
