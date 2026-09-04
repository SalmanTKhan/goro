package mobile

import (
	"strings"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// textWidget draws a string inside its bounds, wrapping and truncating to fit.
//
// gogpu's own TextWidget cannot do this here: the canvas implementation of
// DrawStyledText renders a single unclipped line, so MaxLines and Ellipsis have
// no effect and long labels simply spill over their box and over whatever is
// next to them. Since the mobile screens position everything absolutely and
// cannot rely on a parent to clip, the text has to lay itself out.
//
// Wrapping happens at draw time because that is the first point at which a
// canvas is available to measure with.
type textWidget struct {
	widget.WidgetBase
	content    string
	style      widget.TextStyle
	maxLines   int // 0 means "as many as fit in the bounds"
	lineHeight float32
	// content text comes from the game world — a character's name, an item, a
	// map — and may legitimately be longer than any box reserved for it, so an
	// ellipsis is the correct outcome. Fixed UI strings are held to a stricter
	// standard by the truncation gate.
	fromWorld bool
}

func (k Kit) newText(content string, role TextRole, maxLines int) *textWidget {
	size, color, bold := k.roleStyle(role)
	family := k.Theme.Typography.FontFamily
	if bold && k.Theme.Typography.BoldFontFamily != "" {
		// rotheme renders bold by swapping to the bold family; match it so the
		// mobile tree measures text exactly as the desktop tree does.
		family = k.Theme.Typography.BoldFontFamily
		bold = false
	}
	t := &textWidget{
		content: content,
		style: widget.TextStyle{
			FontFamily: family,
			FontSize:   size,
			Bold:       bold,
			Color:      color,
			Align:      widget.TextAlignLeft,
		},
		maxLines:   maxLines,
		lineHeight: 1.3,
	}
	t.SetVisible(true)
	t.SetEnabled(true)
	return t
}

// Align sets the horizontal alignment.
func (t *textWidget) Align(align widget.TextAlign) *textWidget {
	t.style.Align = align
	return t
}

func (t *textWidget) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.Constrain(geometry.Sz(constraints.MaxWidth, constraints.MaxHeight))
	t.SetBounds(geometry.FromPointSize(t.Position(), size))
	return size
}

func (t *textWidget) Draw(_ widget.Context, canvas widget.Canvas) {
	if !t.IsVisible() || t.content == "" {
		return
	}
	bounds := t.Bounds()
	if bounds.IsEmpty() {
		return
	}
	rowHeight := t.style.FontSize * t.lineHeight
	lines := wrapText(canvas, t.content, t.style, bounds.Width(), t.lineLimit(bounds.Height()))
	if len(lines) == 0 {
		return
	}

	// Vertically centre the block within the bounds, matching how a single line
	// is centred by the canvas.
	blockHeight := float32(len(lines)) * rowHeight
	top := bounds.Min.Y + (bounds.Height()-blockHeight)/2
	if top < bounds.Min.Y {
		top = bounds.Min.Y
	}
	for i, line := range lines {
		row := geometry.NewRect(bounds.Min.X, top+float32(i)*rowHeight, bounds.Width(), rowHeight)
		drawStyled(canvas, line, row, t.style)
	}
}

// lineLimit reports how many lines may be drawn in a box of the given height:
// as many as fit, capped by the caller's maxLines. Shared with the truncation
// gate in the tests so both agree on what "fits" means.
func (t *textWidget) lineLimit(height float32) int {
	rowHeight := t.style.FontSize * t.lineHeight
	if rowHeight <= 0 {
		return 1
	}
	fits := int(height / rowHeight)
	if fits < 1 {
		fits = 1
	}
	if t.maxLines > 0 && t.maxLines < fits {
		return t.maxLines
	}
	return fits
}

func (t *textWidget) Event(widget.Context, event.Event) bool { return false }
func (t *textWidget) Children() []widget.Widget              { return nil }

// wrapText greedily breaks content into at most limit lines that fit width,
// honouring explicit newlines and ellipsizing the final line when content
// remains. Returns nil when nothing can be shown.
func wrapText(canvas widget.Canvas, content string, style widget.TextStyle, width float32, limit int) []string {
	if width <= 0 || limit <= 0 {
		return nil
	}
	var lines []string
	for _, paragraph := range strings.Split(content, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}
		current := ""
		for _, word := range words {
			candidate := word
			if current != "" {
				candidate = current + " " + word
			}
			if measureStyled(canvas, candidate, style) <= width || current == "" {
				current = candidate
				continue
			}
			lines = append(lines, current)
			if len(lines) == limit {
				return truncateLast(canvas, lines, paragraphRemainder(words, word), style, width)
			}
			current = word
		}
		if current != "" {
			lines = append(lines, current)
			if len(lines) == limit {
				return lines
			}
		}
	}
	return lines
}

// paragraphRemainder reports whether any content is left unshown, which is what
// decides if the last line needs an ellipsis.
func paragraphRemainder(words []string, current string) bool {
	for i, word := range words {
		if word == current {
			return i < len(words)
		}
	}
	return true
}

func truncateLast(canvas widget.Canvas, lines []string, more bool, style widget.TextStyle, width float32) []string {
	if !more || len(lines) == 0 {
		return lines
	}
	last := lines[len(lines)-1]
	const ellipsis = "…"
	if measureStyled(canvas, last+ellipsis, style) <= width {
		lines[len(lines)-1] = last + ellipsis
		return lines
	}
	runes := []rune(last)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		candidate := strings.TrimRight(string(runes), " ") + ellipsis
		if measureStyled(canvas, candidate, style) <= width {
			lines[len(lines)-1] = candidate
			return lines
		}
	}
	lines[len(lines)-1] = ellipsis
	return lines
}

func measureStyled(canvas widget.Canvas, s string, style widget.TextStyle) float32 {
	if s == "" {
		return 0
	}
	if styled, ok := canvas.(widget.StyledTextDrawer); ok {
		if w := styled.MeasureStyledText(s, style); w > 0 {
			return w
		}
	}
	if w := canvas.MeasureText(s, style.FontSize, style.Bold); w > 0 {
		return w
	}
	// Last resort so wrapping still makes progress on a canvas that cannot
	// measure; a rough average glyph width is better than an infinite line.
	return float32(len([]rune(s))) * style.FontSize * 0.5
}

func drawStyled(canvas widget.Canvas, s string, bounds geometry.Rect, style widget.TextStyle) {
	if styled, ok := canvas.(widget.StyledTextDrawer); ok {
		styled.DrawStyledText(s, bounds, style)
		return
	}
	canvas.DrawText(s, bounds, style.FontSize, style.Color, style.Bold, style.Align)
}
