package ui

import (
	"fmt"

	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/glog"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/ui/rotheme"
)

const (
	controllerWindowW = 360
	controllerWindowH = 460
)

// ControllerWindow is the controller setup page: live device diagnostics on top
// so hardware bring-up is visible without a debugger, and a press-to-rebind
// table below. Only the physical button of each action is configurable — what
// an action does stays in the input resolve and dispatch code.
type ControllerWindow struct {
	Window
	capturing    input.Action
	captureOpen  bool
	lastSnapshot input.ControllerSnapshot
}

func (w *ControllerWindow) Toggle(ctx client.Context) {
	w.ctx = ctx
	w.EnsureWindow(controllerWindowW, controllerWindowH)
	if w.IsOpen() {
		w.cancelCapture()
		w.Window.Close()
		w.Publish(ctx)
		return
	}
	w.CloseOnEsc = true
	w.configureControllerNavigation()
	w.Window.Open(ctx, w.widgetTree(ctx))
	w.Publish(ctx)
}

func (w *ControllerWindow) configureControllerNavigation() {
	w.SetControllerActionHandler(func(action input.UIAction) bool {
		if action != input.UIActionCancel || w == nil || !w.IsOpen() {
			return false
		}
		w.cancelCapture()
		w.Close()
		return true
	})
}

func (w *ControllerWindow) Update(ctx client.Context) bool {
	w.EnsureWindow(controllerWindowW, controllerWindowH)
	w.ctx = ctx
	if !w.IsOpen() {
		if w.captureOpen {
			w.cancelCapture()
		}
		return false
	}
	consumed := w.Window.Update(ctx)
	if w.captureOpen {
		w.pollCapture(ctx)
	}
	// The diagnostics are live, so redraw whenever the device state changes.
	if snapshot := controllerSnapshot(ctx); snapshot != w.lastSnapshot {
		w.lastSnapshot = snapshot
		w.refresh(ctx)
	}
	w.Publish(ctx)
	return consumed
}

// RebindActive reports whether the window is waiting for a button press. The
// renderer suppresses controller dispatch while it is, so the button being
// bound does not also fire the action it is being bound to.
func (w *ControllerWindow) RebindActive() bool {
	return w != nil && w.captureOpen
}

func (w *ControllerWindow) beginCapture(ctx client.Context, action input.Action) {
	w.capturing = action
	w.captureOpen = true
	w.refresh(ctx)
}

func (w *ControllerWindow) cancelCapture() {
	w.captureOpen = false
	w.capturing = 0
}

// pollCapture assigns the first button pressed while capture is open.
func (w *ControllerWindow) pollCapture(ctx client.Context) {
	if ctx.Input == nil {
		return
	}
	for button := input.ControllerButtonSouth; button <= input.ControllerButtonRightTrigger; button++ {
		if !ctx.Input.ControllerButtonJustPressed(button) {
			continue
		}
		settings := ctx.ControllerSettings()
		settings.Bindings.Set(w.capturing, button)
		w.cancelCapture()
		w.applySettings(ctx, settings)
		return
	}
}

func (w *ControllerWindow) applySettings(ctx client.Context, settings input.ControllerSettings) {
	if ctx.ControllerSettingsHost != nil {
		ctx.ControllerSettingsHost.ApplyControllerSettings(settings)
	}
	if _, err := config.SaveControllerSettings(settings); err != nil {
		glog.Warnf("controller settings save failed: %v", err)
	}
	w.refresh(ctx)
}

func (w *ControllerWindow) refresh(ctx client.Context) {
	w.EnsureWindow(controllerWindowW, controllerWindowH)
	w.ctx = ctx
	w.SetContent(w.widgetTree(ctx))
	w.Publish(ctx)
}

func (w *ControllerWindow) widgetTree(ctx client.Context) widget.Widget {
	return Win(
		Title("Controller"),
		CloseButton(true),
		OnClose(w.Close),
		Size(controllerWindowW, controllerWindowH),
		Content(w.contentTree(ctx)),
	)
}

func (w *ControllerWindow) contentTree(ctx client.Context) widget.Widget {
	settings := ctx.ControllerSettings()
	children := []widget.Widget{
		rotheme.SectionLabel("Device"),
		rotheme.Text(controllerDeviceSummary(ctx)),
		rotheme.Text(controllerAxisSummary(ctx)),
		rotheme.Text(controllerButtonSummary(ctx)),
		rotheme.SectionLabel("Bindings"),
	}
	for _, action := range input.BindableActions() {
		children = append(children, w.bindingRow(ctx, action, settings))
	}
	children = append(children,
		rotheme.Button("Restore defaults", func() {
			restored := ctx.ControllerSettings()
			restored.Bindings = input.DefaultControllerBindings()
			w.applySettings(ctx, restored)
		}),
	)
	return primitives.Box(children...).Padding(14).Gap(6)
}

func (w *ControllerWindow) bindingRow(ctx client.Context, action input.Action, settings input.ControllerSettings) widget.Widget {
	label := fmt.Sprintf("%s: %s", action.Name(), controllerButtonLabel(settings.Bindings.Get(action)))
	if w.captureOpen && w.capturing == action {
		label = action.Name() + ": press a button…"
	}
	captured := action
	return rotheme.Button(label, func() {
		if w.captureOpen && w.capturing == captured {
			w.cancelCapture()
			w.refresh(ctx)
			return
		}
		w.beginCapture(ctx, captured)
	})
}

func controllerSnapshot(ctx client.Context) input.ControllerSnapshot {
	if ctx.Input == nil {
		return input.ControllerSnapshot{}
	}
	return ctx.Input.Controller()
}

func controllerDeviceSummary(ctx client.Context) string {
	snapshot := controllerSnapshot(ctx)
	if !snapshot.Connected {
		return "No controller connected"
	}
	name := snapshot.Name
	if name == "" {
		name = "Unknown controller"
	}
	return fmt.Sprintf("%s (%s)", name, snapshot.Kind)
}

func controllerAxisSummary(ctx client.Context) string {
	snapshot := controllerSnapshot(ctx)
	return fmt.Sprintf("L %+.2f,%+.2f  R %+.2f,%+.2f  LT %.2f RT %.2f",
		snapshot.LeftX, snapshot.LeftY, snapshot.RightX, snapshot.RightY,
		snapshot.LeftTrigger, snapshot.RightTrigger)
}

func controllerButtonSummary(ctx client.Context) string {
	snapshot := controllerSnapshot(ctx)
	held := ""
	for button := input.ControllerButtonSouth; button <= input.ControllerButtonRightTrigger; button++ {
		if !snapshot.ButtonDown(button) {
			continue
		}
		if held != "" {
			held += " "
		}
		held += controllerButtonLabel(button)
	}
	if held == "" {
		return "Buttons: none"
	}
	return "Buttons: " + held
}

func controllerButtonLabel(button input.ControllerButton) string {
	switch button {
	case input.ControllerButtonSouth:
		return "South"
	case input.ControllerButtonEast:
		return "East"
	case input.ControllerButtonWest:
		return "West"
	case input.ControllerButtonNorth:
		return "North"
	case input.ControllerButtonBack:
		return "Back"
	case input.ControllerButtonStart:
		return "Start"
	case input.ControllerButtonLeftStick:
		return "L3"
	case input.ControllerButtonRightStick:
		return "R3"
	case input.ControllerButtonLeftShoulder:
		return "L1"
	case input.ControllerButtonRightShoulder:
		return "R1"
	case input.ControllerButtonDPadUp:
		return "D-Up"
	case input.ControllerButtonDPadDown:
		return "D-Down"
	case input.ControllerButtonDPadLeft:
		return "D-Left"
	case input.ControllerButtonDPadRight:
		return "D-Right"
	case input.ControllerButtonTouchpad:
		return "Touchpad"
	case input.ControllerButtonLeftTrigger:
		return "L2"
	case input.ControllerButtonRightTrigger:
		return "R2"
	default:
		return "?"
	}
}
