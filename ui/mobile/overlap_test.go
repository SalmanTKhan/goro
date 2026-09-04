package mobile

import (
	"testing"

	"github.com/gogpu/ui/geometry"
)

// TestTextDoesNotOverlapText closes the third variant of the bitmap-font sizing
// bug. The truncation gate catches text too wide for its box and the
// completeness gate catches content dropped entirely, but neither sees a label
// sitting in a rectangle too *short* for it: the text simply spills upward and
// downward onto its neighbours. That is what put "Your items" through the first
// trade row and "Name" through the name field's border.
//
// Two text boxes that overlap are always a layout error, since nothing in this
// presentation clips.
func TestTextDoesNotOverlapText(t *testing.T) {
	k := testKit()
	for _, sc := range screenCases() {
		for _, vp := range qualViewports() {
			t.Run(sc.name+"/"+vp.Name, func(t *testing.T) {
				c := sc.build(k, vp.VP)
				boxes := drawnTextBoxes(t, c)
				for i := 0; i < len(boxes); i++ {
					for j := i + 1; j < len(boxes); j++ {
						if !collides(boxes[i], boxes[j]) {
							continue
						}
						t.Errorf("text %q overlaps %q (%v vs %v)",
							boxes[i].text, boxes[j].text, boxes[i].rect, boxes[j].rect)
					}
				}
			})
		}
	}
}

type drawnText struct {
	text string
	rect geometry.Rect
}

// drawnTextBoxes returns the box each line of text was actually drawn into,
// which is what neighbours collide with — not the rectangle the layout
// reserved.
func drawnTextBoxes(t *testing.T, c *Canvas) []drawnText {
	t.Helper()
	canvas := drawOn(t, c)
	var out []drawnText
	for _, child := range c.children {
		text, isText := child.widget.(*textWidget)
		if !isText || text.content == "" {
			continue
		}
		rowHeight := text.style.FontSize * text.lineHeight
		lines := wrapText(canvas, text.content, text.style, child.rect.Width(), text.lineLimit(child.rect.Height()))
		if len(lines) == 0 {
			continue
		}
		// Mirror Draw's vertical centring so the boxes match what is painted.
		block := float32(len(lines)) * rowHeight
		top := child.rect.Min.Y + (child.rect.Height()-block)/2
		if top < child.rect.Min.Y {
			top = child.rect.Min.Y
		}
		for i, line := range lines {
			width := measureStyled(canvas, line, text.style)
			out = append(out, drawnText{
				text: line,
				rect: geometry.NewRect(child.rect.Min.X, top+float32(i)*rowHeight, width, rowHeight),
			})
		}
	}
	return out
}

// collides reports whether two drawn lines actually run into each other.
//
// The boxes measured here are line boxes, which include the leading above and
// below the glyphs, so consecutive lines always share a few pixels of empty
// space. Only an overlap deeper than that leading is a real collision — the
// defects this gate exists for (a heading printed through the row beneath it)
// overlap by half a row or more.
func collides(a, b drawnText) bool {
	const inkFraction = 0.62 // glyph ink as a share of the 1.3x line box

	horizontal := min32(a.rect.Max.X, b.rect.Max.X) - max32(a.rect.Min.X, b.rect.Min.X)
	if horizontal <= 1 {
		return false
	}
	vertical := min32(a.rect.Max.Y, b.rect.Max.Y) - max32(a.rect.Min.Y, b.rect.Min.Y)
	allowed := (1 - inkFraction) * min32(a.rect.Height(), b.rect.Height())
	return vertical > allowed
}
