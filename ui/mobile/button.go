package mobile

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// buttonWidget paints a button surface and its label together.
//
// Composing a primitives.Box around a primitives.Text does not work here: that
// text draws a single unclipped line, so any label wider than the button spills
// outside the filled rectangle — which is exactly how "‹ Back" ended up printed
// beyond its own background. Owning both halves lets the label be measured
// against the surface it sits on and shortened if it genuinely cannot fit.
//
// Padding is proportional rather than fixed for the same reason: the layout
// hands out button rectangles sized for the old bitmap font, and a fixed 20px
// inset on each side leaves a 104px control with too little room for its word.
type buttonWidget struct {
	widget.WidgetBase
	label      string
	style      widget.TextStyle
	background widget.Color
	border     widget.Color
	radius     float32
	padX       float32
	// fromWorld marks a label that comes from game data — an NPC script's menu
	// choice, for instance — which has no length the layout could have
	// reserved for. Such a label may ellipsize; authored UI copy may not.
	fromWorld bool
}

func (k Kit) newButton(label string, state ButtonState) *buttonWidget {
	background := k.Theme.Colors.Button
	role := RoleBody
	switch state {
	case ButtonPressed:
		background = k.Theme.Colors.ButtonDown
	case ButtonDisabled:
		background = k.Theme.Colors.Disabled
		role = RoleMuted
	}
	size, color, bold := k.roleStyle(role)
	family := k.Theme.Typography.FontFamily
	if bold && k.Theme.Typography.BoldFontFamily != "" {
		family = k.Theme.Typography.BoldFontFamily
		bold = false
	}
	b := &buttonWidget{
		label: label,
		style: widget.TextStyle{
			FontFamily: family,
			FontSize:   size,
			Bold:       bold,
			Color:      color,
			Align:      widget.TextAlignCenter,
		},
		background: background,
		border:     k.Theme.Colors.ButtonBorder,
		radius:     k.Theme.Metrics.ButtonRadius,
		padX:       k.Theme.Metrics.ButtonPaddingX,
	}
	b.SetVisible(true)
	b.SetEnabled(true)
	return b
}

func (b *buttonWidget) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.Constrain(geometry.Sz(constraints.MaxWidth, constraints.MaxHeight))
	b.SetBounds(geometry.FromPointSize(b.Position(), size))
	return size
}

// inset is the horizontal breathing room around the label. It never takes more
// than a quarter of the control, so a narrow button spends its width on the
// word rather than on padding.
func (b *buttonWidget) inset(width float32) float32 {
	return min32(b.padX, width*0.12)
}

func (b *buttonWidget) Draw(_ widget.Context, canvas widget.Canvas) {
	if !b.IsVisible() {
		return
	}
	bounds := b.Bounds()
	if bounds.IsEmpty() {
		return
	}
	canvas.DrawRoundRect(bounds, b.background, b.radius)
	canvas.StrokeRoundRect(bounds, b.border, b.radius, 1)
	if b.label == "" {
		return
	}
	inset := b.inset(bounds.Width())
	text := geometry.NewRect(bounds.Min.X+inset, bounds.Min.Y, bounds.Width()-2*inset, bounds.Height())
	if text.Width() <= 0 {
		return
	}
	lines := wrapText(canvas, b.label, b.style, text.Width(), 1)
	if len(lines) == 0 {
		return
	}
	drawStyled(canvas, lines[0], text, b.style)
}

// labelFits reports whether the label renders in full at the given width. The
// truncation gate uses this: a button is the one place where an ellipsis is
// never acceptable, because the label is the only thing naming the action.
func (b *buttonWidget) labelFits(canvas widget.Canvas, width float32) bool {
	if b.label == "" {
		return true
	}
	inset := b.inset(width)
	available := width - 2*inset
	if available <= 0 {
		return false
	}
	return measureStyled(canvas, b.label, b.style) <= available
}

func (b *buttonWidget) Event(widget.Context, event.Event) bool { return false }
func (b *buttonWidget) Children() []widget.Widget              { return nil }
