package mobileui

import (
	"image/color"
	"testing"
)

func TestParseROTextSupportsBreaksColorsAndReset(t *testing.T) {
	base := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	runs := ParseROText("one<BR />two ^FF8040warm^000000 base\r\nlast", base)
	if len(runs) != 3 {
		t.Fatalf("unexpected runs: %+v", runs)
	}
	if runs[0].Text != "one\ntwo " || runs[0].Color != base {
		t.Fatalf("unexpected base run: %+v", runs[0])
	}
	if runs[1].Text != "warm" || runs[1].Color != (color.RGBA{R: 255, G: 128, B: 64, A: 255}) {
		t.Fatalf("unexpected colored run: %+v", runs[1])
	}
	if runs[2].Text != " base\nlast" || runs[2].Color != base {
		t.Fatalf("unexpected reset run: %+v", runs[2])
	}
	if got := StripROText("A^00FF00B<br/>C"); got != "AB\nC" {
		t.Fatalf("unexpected stripped text %q", got)
	}
}

func TestNormalizeROTextLeavesUnknownTagsVisible(t *testing.T) {
	if got := NormalizeROText("<item 501>^BADBAD"); got != "<item 501>^BADBAD" {
		t.Fatalf("unknown markup was changed: %q", got)
	}
}
