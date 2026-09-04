package mobile

import (
	"strings"
	"testing"
)

// TestNoLabelIsTruncated is the structural gate for a bug class that has shown
// up on nearly every screen: mobileui hands out rectangles whose fixed pixel
// sizes were tuned for the old scaled-bitmap text, which is far smaller than
// the real UI font the mobile presentation now renders with. The symptom is
// always a clipped label — "Equipm", a stat list stopping at three of six rows,
// a caption overlapping the field beside it.
//
// Rather than auditing those constants by hand, this walks every screen at
// every qualification viewport and fails on any label the renderer had to
// ellipsize. Wrapped prose is exempt: a description is *expected* to be cut off
// when it is longer than its panel, and the ellipsis is the correct outcome
// there. Single-line labels are not.
func TestNoLabelIsTruncated(t *testing.T) {
	k := testKit()
	for _, sc := range screenCases() {
		for _, vp := range qualViewports() {
			t.Run(sc.name+"/"+vp.Name, func(t *testing.T) {
				c := sc.build(k, vp.VP)
				// Measured with the real font, not the mock's flat estimate:
				// see measure_test.go.
				canvas := drawOn(t, c)

				for _, child := range c.children {
					// A button's label is the only thing naming its action, so an
					// ellipsis there is never acceptable.
					if button, ok := child.widget.(*buttonWidget); ok {
						if !button.fromWorld && !button.labelFits(canvas, child.rect.Width()) {
							t.Errorf("button label %q does not fit its %.0fx%.0f control",
								button.label, child.rect.Width(), child.rect.Height())
						}
						continue
					}
					text, ok := child.widget.(*textWidget)
					if !ok || text.content == "" {
						continue
					}
					// Only single-line labels are held to this standard, and only
					// those whose text is authored UI copy: a character or item
					// name from the world has no length bound the layout could
					// have honoured.
					if text.maxLines != 1 || text.fromWorld {
						continue
					}
					lines := wrapText(canvas, text.content, text.style,
						child.rect.Width(), text.lineLimit(child.rect.Height()))
					if len(lines) != 1 {
						t.Errorf("label %q wrapped to %d lines in a single-line box %v",
							text.content, len(lines), child.rect)
						continue
					}
					if strings.HasSuffix(lines[0], "…") && !strings.HasSuffix(text.content, "…") {
						t.Errorf("label %q does not fit its %.0fx%.0f box (drawn as %q)",
							text.content, child.rect.Width(), child.rect.Height(), lines[0])
					}
				}
			})
		}
	}
}
