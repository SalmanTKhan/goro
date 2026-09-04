package mobile

import (
	"strings"
	"testing"

	"github.com/gogpu/ui/uitest"
	"github.com/kivutar/goro/mobileui"
)

// drawnLines returns the text of every line the canvas was asked to draw.
func drawnLines(canvas *uitest.MockCanvas) []string {
	var out []string
	for _, s := range canvas.StyledTexts {
		out = append(out, s.Text)
	}
	for _, s := range canvas.Texts {
		out = append(out, s.Text)
	}
	return out
}

func drawText(t *testing.T, content string, role TextRole, maxLines int, r mobileui.Rect) []string {
	t.Helper()
	c := NewCanvas(2000, 2000)
	c.Place(testKit().Wrapped(content, role, maxLines), r)
	return drawnLines(layoutAndDraw(t, c))
}

func TestLongTextWrapsAcrossLines(t *testing.T) {
	content := "A potion made from grinded Red Herbs that restores a small amount of HP when consumed."
	lines := drawText(t, content, RoleBody, 0, mobileui.Rect{X: 0, Y: 0, W: 400, H: 300})
	if len(lines) < 2 {
		t.Fatalf("expected the text to wrap, got %d line(s): %q", len(lines), lines)
	}
	// Wrapping must not lose or duplicate words.
	joined := strings.Join(lines, " ")
	for _, word := range strings.Fields(content) {
		if !strings.Contains(joined, word) {
			t.Errorf("word %q was dropped by wrapping: %q", word, joined)
		}
	}
}

func TestSingleLineTextIsTruncatedNotOverflowed(t *testing.T) {
	// A label that cannot fit must be shortened with an ellipsis, because the
	// canvas does not clip: an overflowing line would paint over its neighbours.
	lines := drawText(t, "Equipment and Accessories", RoleBody, 1, mobileui.Rect{X: 0, Y: 0, W: 90, H: 60})
	if len(lines) != 1 {
		t.Fatalf("expected exactly one line, got %q", lines)
	}
	if !strings.HasSuffix(lines[0], "…") {
		t.Errorf("truncated label %q should end with an ellipsis", lines[0])
	}
	if lines[0] == "Equipment and Accessories" {
		t.Error("label was not truncated at all")
	}
}

func TestMaxLinesIsHonoured(t *testing.T) {
	content := strings.Repeat("word ", 200)
	lines := drawText(t, content, RoleBody, 2, mobileui.Rect{X: 0, Y: 0, W: 300, H: 900})
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.HasSuffix(lines[1], "…") {
		t.Errorf("clipped paragraph should end with an ellipsis, got %q", lines[1])
	}
}

func TestTextNeverExceedsItsBoxHeight(t *testing.T) {
	// With maxLines unset the paragraph may only use as many rows as fit.
	box := mobileui.Rect{X: 0, Y: 0, W: 300, H: 100}
	lines := drawText(t, strings.Repeat("word ", 200), RoleBody, 0, box)
	size, _, _ := testKit().roleStyle(RoleBody)
	rowHeight := size * 1.3
	if float32(len(lines))*rowHeight > box.H+0.5 {
		t.Errorf("%d lines at %v each overflow a box of height %v", len(lines), rowHeight, box.H)
	}
}

func TestShortTextIsNotEllipsized(t *testing.T) {
	lines := drawText(t, "Use", RoleBody, 1, mobileui.Rect{X: 0, Y: 0, W: 400, H: 60})
	if len(lines) != 1 || lines[0] != "Use" {
		t.Errorf("short label was altered: %q", lines)
	}
}

func TestDescriptionStripsROColorCodes(t *testing.T) {
	// RO item descriptions carry ^RRGGBB control codes. They must never reach
	// the screen as literal text.
	got := joinLines([]string{"A potion that restores ^000088about 45 HP^000000."})
	if strings.Contains(got, "^") {
		t.Errorf("colour codes leaked into the description: %q", got)
	}
	if !strings.Contains(got, "about 45 HP") {
		t.Errorf("stripping removed the content too: %q", got)
	}
}

func TestDescriptionLinesJoinWithSpaces(t *testing.T) {
	// The client supplies pre-broken lines; joining them without a separator
	// produced runs like "made fromgrinded".
	got := joinLines([]string{"A potion made from", "grinded Red Herbs that", "restores HP."})
	if strings.Contains(got, "fromgrinded") || strings.Contains(got, "thatrestores") {
		t.Errorf("description lines were joined without spaces: %q", got)
	}
	if got != "A potion made from grinded Red Herbs that restores HP." {
		t.Errorf("unexpected join result: %q", got)
	}
}
