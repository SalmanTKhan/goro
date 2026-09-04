package mobile

import (
	"image/color"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/ui"
	"github.com/kivutar/goro/ui/rotheme"
)

// BarKind selects the fill colour of a meter. The values come from the desktop
// client so a player reads the same colours in either presentation.
type BarKind int

const (
	// BarHP is the health meter.
	BarHP BarKind = iota
	// BarSP is the spirit meter.
	BarSP
	// BarBaseEXP is the base experience meter.
	BarBaseEXP
	// BarJobEXP is the job experience meter.
	BarJobEXP
)

// barTrackColor mirrors characterWindowBarBack in ui/character_window.go, which
// is unexported; the rest of the meter colours are taken from the desktop
// package by reference so they cannot drift.
var barTrackColor = color.RGBA{R: 224, G: 232, B: 242, A: 255}

func (k Kit) barColor(kind BarKind) widget.Color {
	switch kind {
	case BarSP:
		return toWidgetColor(ui.PlayerSPBarColor)
	case BarBaseEXP, BarJobEXP:
		// Both experience meters use the window border blue on the desktop
		// (characterWindowEXPColor / characterWindowJobEXPColor).
		return toWidgetColor(ui.WindowBorderColor)
	default:
		return toWidgetColor(ui.PlayerHPBarColor)
	}
}

// Bar returns a meter filled to fraction (clamped to 0..1). The fill is painted
// rather than laid out because the widget's width is not known until layout.
func (k Kit) Bar(kind BarKind, fraction float32) widget.Widget {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	b := &barWidget{
		fraction: fraction,
		track:    toWidgetColor(barTrackColor),
		fill:     k.barColor(kind),
		border:   k.Theme.Colors.WindowBorder,
		radius:   k.Theme.Metrics.ButtonRadius / 2,
	}
	b.SetVisible(true)
	b.SetEnabled(true)
	return b
}

type barWidget struct {
	widget.WidgetBase
	fraction    float32
	track, fill widget.Color
	border      widget.Color
	radius      float32
}

func (b *barWidget) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.Constrain(geometry.Sz(constraints.MaxWidth, constraints.MaxHeight))
	b.SetBounds(geometry.FromPointSize(b.Position(), size))
	return size
}

func (b *barWidget) Draw(_ widget.Context, canvas widget.Canvas) {
	if !b.IsVisible() {
		return
	}
	bounds := b.Bounds()
	if bounds.IsEmpty() {
		return
	}
	canvas.DrawRoundRect(bounds, b.track, b.radius)
	if b.fraction > 0 {
		filled := bounds
		filled.Max.X = bounds.Min.X + bounds.Width()*b.fraction
		// A sliver of fill still has to read as a fill, so never round it away.
		canvas.DrawRoundRect(filled, b.fill, min32(b.radius, filled.Width()/2))
	}
	canvas.StrokeRoundRect(bounds, b.border, b.radius, 1)
}

func (b *barWidget) Event(widget.Context, event.Event) bool { return false }
func (b *barWidget) Children() []widget.Widget              { return nil }

// TitleBar returns the Ragnarok window title bar: a vertical blue gradient with
// a bottom border and the title inset from the left edge.
func (k Kit) TitleBar(title string) widget.Widget {
	label := k.Text(title, RoleTitle)
	bar := &titleBarWidget{
		child:  label,
		top:    k.Theme.Colors.WindowTitleTop,
		bottom: k.Theme.Colors.WindowTitle,
		border: k.Theme.Colors.WindowBorder,
		padX:   k.Theme.Metrics.TableCellPadX,
	}
	bar.SetVisible(true)
	bar.SetEnabled(true)
	if setter, ok := widget.Widget(label).(interface{ SetParent(widget.Widget) }); ok {
		setter.SetParent(bar)
	}
	return bar
}

type titleBarWidget struct {
	widget.WidgetBase
	child               widget.Widget
	top, bottom, border widget.Color
	padX                float32
}

func (w *titleBarWidget) Layout(ctx widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.Constrain(geometry.Sz(constraints.MaxWidth, constraints.MaxHeight))
	w.SetBounds(geometry.FromPointSize(w.Position(), size))
	inner := geometry.Sz(max32(size.Width-2*w.padX, 0), size.Height)
	w.child.Layout(ctx, geometry.Tight(inner))
	if setter, ok := w.child.(interface{ SetBounds(geometry.Rect) }); ok {
		setter.SetBounds(geometry.NewRect(w.padX, 0, inner.Width, inner.Height))
	}
	return size
}

func (w *titleBarWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	if !w.IsVisible() {
		return
	}
	bounds := w.Bounds()
	rotheme.DrawVerticalGradientWithBottomBorder(canvas, bounds, w.top, w.bottom, w.border, 1)
	canvas.PushTransform(bounds.Min)
	widget.StampScreenOrigin(w.child, canvas)
	widget.DrawChild(w.child, ctx, canvas)
	canvas.PopTransform()
}

func (w *titleBarWidget) Event(widget.Context, event.Event) bool { return false }
func (w *titleBarWidget) Children() []widget.Widget              { return []widget.Widget{w.child} }

// Window returns a complete Ragnarok window: gradient title bar over a bordered
// body holding content. This is the mobile counterpart of ui.Win.
func (k Kit) Window(title string, content widget.Widget) widget.Widget {
	titleHeight := k.Theme.Metrics.WindowTitleHeight
	body := primitives.Box()
	if content != nil {
		body = primitives.Box(content)
	}
	return primitives.VBox(
		primitives.Box(k.TitleBar(title)).Height(titleHeight),
		primitives.Box(body).Background(k.Theme.Colors.WindowBody),
	).
		CrossAlign(primitives.CrossAxisStretch).
		Background(k.Theme.Colors.WindowBody).
		BorderStyle(1, k.Theme.Colors.WindowBorder).
		Rounded(k.Theme.Metrics.ButtonRadius)
}

func toWidgetColor(c color.RGBA) widget.Color {
	return widget.RGBA8(c.R, c.G, c.B, c.A)
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
