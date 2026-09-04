package mobile

import (
	"testing"

	"github.com/gogpu/gg"
	"github.com/gogpu/ui/geometry"
	uirender "github.com/gogpu/ui/render"
	"github.com/gogpu/ui/uitest"
	"github.com/gogpu/ui/widget"
)

// realCanvas returns a canvas backed by the same rasterizer the client draws
// with, so measurements match what is actually rendered.
//
// uitest.MockCanvas estimates text width as 0.5 * fontSize per rune, which
// under-measures real DejaVu. A gate built on that estimate passes while the
// shipped frame truncates — exactly what happened to the trade screen's back
// button. Measuring with the real font closes that gap.
func realCanvas(width, height int) widget.Canvas {
	dc := gg.NewContext(width, height)
	return uirender.NewCanvas(dc, width, height)
}

// drawOn lays out and draws a canvas through a real-font surface, returning the
// measuring canvas so callers can re-measure exactly what was drawn.
func drawOn(t *testing.T, c *Canvas) widget.Canvas {
	t.Helper()
	w, h := c.Size()
	canvas := realCanvas(int(w), int(h))
	ctx := uitest.NewMockContext()
	c.Layout(ctx, geometry.Tight(geometry.Sz(w, h)))
	c.Draw(ctx, canvas)
	return canvas
}

// TestRealFontIsWiderThanTheMockEstimate documents why the gates use a real
// canvas. If this ever stops holding, the gates could be relaxed back to the
// mock.
func TestRealFontIsWiderThanTheMockEstimate(t *testing.T) {
	k := testKit()
	style := k.newText("‹ Back", RoleBody, 1).style

	real := measureStyled(realCanvas(256, 64), "‹ Back", style)
	mock := measureStyled(&uitest.MockCanvas{}, "‹ Back", style)
	if real <= 0 {
		t.Fatal("real canvas measured nothing; font not registered?")
	}
	if real <= mock {
		t.Errorf("real font width %v is not greater than the mock estimate %v; "+
			"the truncation gate may be able to use the mock again", real, mock)
	}
}
