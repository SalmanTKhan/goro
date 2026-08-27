package mobileui

import (
	"image/color"
	"strings"
	"unicode/utf8"
)

// MobileTextRun is the renderer-neutral representation of the RO text
// formatting understood by the mobile surfaces. Text that is not a known RO
// control sequence remains visible instead of being silently discarded.
type MobileTextRun struct {
	Text  string
	Color color.RGBA
}

// NormalizeROText converts the line-break forms emitted by RO scripts and
// servers into newlines. This intentionally does not treat arbitrary HTML as
// markup: unknown tags are content and must remain visible for parity/debugging.
func NormalizeROText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '<' {
			end := strings.IndexByte(value[i:], '>')
			if end >= 0 {
				end += i + 1
				tag := value[i:end]
				if isROBreakTag(tag) {
					out.WriteByte('\n')
					i = end
					continue
				}
			}
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		out.WriteRune(r)
		i += size
	}
	return out.String()
}

func isROBreakTag(tag string) bool {
	switch {
	case strings.EqualFold(tag, "<br>"), strings.EqualFold(tag, "<br/>"), strings.EqualFold(tag, "<br />"):
		return true
	default:
		return false
	}
}

// ParseROText implements the color control used by the desktop RO surfaces:
// ^RRGGBB changes the current color and ^000000 resets it to base. Breaks and
// ordinary newlines are preserved in the returned runs.
func ParseROText(value string, base color.RGBA) []MobileTextRun {
	value = NormalizeROText(value)
	current := base
	var runs []MobileTextRun
	var text strings.Builder
	flush := func() {
		if text.Len() == 0 {
			return
		}
		runs = append(runs, MobileTextRun{Text: text.String(), Color: current})
		text.Reset()
	}
	for i := 0; i < len(value); {
		if value[i] == '^' && i+7 <= len(value) && isROHex(value[i+1:i+7]) {
			flush()
			if strings.EqualFold(value[i+1:i+7], "000000") {
				current = base
			} else {
				current = color.RGBA{R: roHexByte(value[i+1], value[i+2]), G: roHexByte(value[i+3], value[i+4]), B: roHexByte(value[i+5], value[i+6]), A: 255}
			}
			i += 7
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		text.WriteRune(r)
		i += size
	}
	flush()
	return runs
}

// StripROText removes only recognized RO color controls while preserving
// visible content and normalized line breaks.
func StripROText(value string) string {
	var out strings.Builder
	for _, run := range ParseROText(value, color.RGBA{}) {
		out.WriteString(run.Text)
	}
	return out.String()
}

func isROHex(value string) bool {
	if len(value) != 6 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

func roHexByte(hi, lo byte) uint8 {
	return roHexNibble(hi)<<4 | roHexNibble(lo)
}

func roHexNibble(value byte) uint8 {
	switch {
	case value >= '0' && value <= '9':
		return value - '0'
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10
	default:
		return 0
	}
}
