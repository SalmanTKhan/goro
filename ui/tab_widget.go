package ui

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/ui/rotheme"
)

const verticalTabDividerW = 1

type tabWidgetConfig struct {
	label         string
	labelRotation rotheme.TextRotation
	active        bool
	disabled      bool
	width         int
	height        int
	onClick       func()
}

type tabWidget struct {
	widget.WidgetBase
	cfg     tabWidgetConfig
	hovered bool
}

func newTabWidget(cfg tabWidgetConfig) *tabWidget {
	w := &tabWidget{cfg: cfg}
	w.SetVisible(true)
	w.SetEnabled(true)
	return w
}

func verticalTabFrame(tabs, content widget.Widget) widget.Widget {
	return primitives.HBox(
		tabs,
		primitives.Box().
			Width(verticalTabDividerW).
			Background(rotheme.Default.Colors.WindowBorder),
		content,
	).
		Gap(0).
		CrossAlign(primitives.CrossAxisStretch)
}

func (w *tabWidget) Layout(ctx widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.Constrain(geometry.Sz(float32(w.cfg.width), float32(w.cfg.height)))
	w.SetBounds(geometry.FromPointSize(w.Position(), size))
	return size
}

func (w *tabWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	bounds := w.Bounds()
	fill := rotheme.Default.Colors.Button
	textColor := rotheme.Default.Colors.Text
	if w.cfg.active {
		fill = rotheme.Default.Colors.WindowBody
	} else if w.cfg.disabled {
		textColor = rotheme.Default.Colors.MutedText
	} else if w.hovered {
		fill = rotheme.Default.Colors.ButtonHover
	}
	canvas.DrawRect(bounds, fill)
	canvas.StrokeRect(bounds, rotheme.Default.Colors.WindowBorder, 1)
	rotheme.DrawRotatedText(canvas, w.cfg.label, bounds, rotheme.Default.Typography.TextSize, textColor, false, w.cfg.labelRotation)
}

func (w *tabWidget) Event(ctx widget.Context, e event.Event) bool {
	mouse, ok := e.(*event.MouseEvent)
	if !ok {
		return false
	}
	if w.cfg.disabled {
		w.hovered = false
		ctx.SetCursor(widget.CursorDefault)
		return true
	}
	switch mouse.MouseType {
	case event.MouseEnter:
		w.hovered = true
		ctx.SetCursor(widget.CursorPointer)
		return true
	case event.MouseLeave:
		w.hovered = false
		ctx.SetCursor(widget.CursorDefault)
		return false
	case event.MousePress:
		if mouse.Button == event.ButtonLeft && w.cfg.onClick != nil {
			w.cfg.onClick()
			return true
		}
	}
	return true
}
