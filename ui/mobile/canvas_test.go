package mobile

import (
	"testing"

	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/uitest"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/ui/rotheme"
)

func testKit() Kit { return NewKit(rotheme.Mobile) }

func layoutAndDraw(t *testing.T, c *Canvas) *uitest.MockCanvas {
	t.Helper()
	ctx := uitest.NewMockContext()
	w, h := c.Size()
	c.Layout(ctx, geometry.Tight(geometry.Sz(w, h)))
	canvas := &uitest.MockCanvas{}
	c.Draw(ctx, canvas)
	return canvas
}

func TestCanvasPlacesChildrenAtAbsoluteRects(t *testing.T) {
	k := testKit()
	c := NewCanvas(1080, 2340)
	c.Place(k.Panel(), mobileui.Rect{X: 10, Y: 20, W: 300, H: 400})
	c.Place(k.Panel(), mobileui.Rect{X: 400, Y: 500, W: 200, H: 100})

	canvas := layoutAndDraw(t, c)

	// Each placed child is drawn under a transform at its absolute origin, and
	// paints itself in local coordinates. That is what makes mobileui's layout
	// rectangles the single source of geometry. Children push transforms of
	// their own, so look for ours as an ordered subsequence.
	want := []geometry.Point{{X: 10, Y: 20}, {X: 400, Y: 500}}
	next := 0
	for _, got := range canvas.Transforms {
		if next < len(want) && got == want[next] {
			next++
		}
	}
	if next != len(want) {
		t.Errorf("absolute origins %v not all pushed in order; got transforms %v", want, canvas.Transforms)
	}

	// Local painting: sizes come from the placed rect, origins are zero.
	wantSizes := []geometry.Size{{Width: 300, Height: 400}, {Width: 200, Height: 100}}
	if len(canvas.RoundRects) != len(wantSizes) {
		t.Fatalf("got %d panel fills, want %d", len(canvas.RoundRects), len(wantSizes))
	}
	for i, size := range wantSizes {
		b := canvas.RoundRects[i].Bounds
		if b.Min != (geometry.Point{}) {
			t.Errorf("panel %d painted at %v, want local origin", i, b.Min)
		}
		if b.Width() != size.Width || b.Height() != size.Height {
			t.Errorf("panel %d is %vx%v, want %vx%v", i, b.Width(), b.Height(), size.Width, size.Height)
		}
	}
}

func TestCanvasDropsZeroAreaRects(t *testing.T) {
	k := testKit()
	c := NewCanvas(1080, 2340)
	c.Place(k.Panel(), mobileui.Rect{X: 0, Y: 0, W: 0, H: 40})
	c.Place(k.Panel(), mobileui.Rect{X: 0, Y: 0, W: 40, H: 0})
	c.Place(k.Panel(), mobileui.Rect{X: 5, Y: 5, W: 10, H: 10})

	if got := len(c.Children()); got != 1 {
		t.Fatalf("canvas kept %d children, want only the non-empty one", got)
	}
}

func TestCanvasDeclinesEvents(t *testing.T) {
	// mobileui owns hit-testing for this presentation. If the widget tree ever
	// starts consuming events there would be two hit-test authorities and they
	// would drift; this test is the guard against that.
	c := NewCanvas(1080, 2340)
	c.Place(testKit().Button("Equip", ButtonNormal), mobileui.Rect{X: 0, Y: 0, W: 200, H: 60})
	if c.Event(uitest.NewMockContext(), uitest.Click(10, 10)) {
		t.Error("Canvas consumed a click; mobileui must remain the only hit-test authority")
	}
}

func TestBarFillIsProportionalAndClamped(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fraction float32
		wantFill float32
	}{
		{"empty", 0, 0},
		{"half", 0.5, 100},
		{"full", 1, 200},
		{"negative clamps to empty", -3, 0},
		{"overfull clamps to full", 5, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := testKit()
			c := NewCanvas(400, 100)
			c.Place(k.Bar(BarHP, tc.fraction), mobileui.Rect{X: 0, Y: 0, W: 200, H: 20})
			canvas := layoutAndDraw(t, c)

			if len(canvas.RoundRects) == 0 {
				t.Fatal("bar drew nothing")
			}
			// The track is always drawn; a fill is drawn on top only when non-zero.
			if tc.wantFill == 0 {
				if len(canvas.RoundRects) != 1 {
					t.Fatalf("empty bar drew %d rects, want just the track", len(canvas.RoundRects))
				}
				return
			}
			if len(canvas.RoundRects) != 2 {
				t.Fatalf("bar drew %d rects, want track + fill", len(canvas.RoundRects))
			}
			if got := canvas.RoundRects[1].Bounds.Width(); got != tc.wantFill {
				t.Errorf("fill width = %v, want %v", got, tc.wantFill)
			}
		})
	}
}

func TestBarUsesDesktopColors(t *testing.T) {
	k := testKit()
	for _, tc := range []struct {
		kind BarKind
		want widget.Color
	}{
		{BarHP, k.barColor(BarHP)},
		{BarSP, k.barColor(BarSP)},
		{BarBaseEXP, k.barColor(BarBaseEXP)},
	} {
		c := NewCanvas(400, 100)
		c.Place(k.Bar(tc.kind, 0.5), mobileui.Rect{X: 0, Y: 0, W: 200, H: 20})
		canvas := layoutAndDraw(t, c)
		if len(canvas.RoundRects) < 2 {
			t.Fatalf("bar %d drew no fill", tc.kind)
		}
		uitest.AssertColorEqual(t, canvas.RoundRects[1].Color, tc.want)
	}
}

func TestSelectedCardUsesDesktopSelectionTreatment(t *testing.T) {
	k := testKit()
	plain := NewCanvas(400, 400)
	plain.Place(k.Card(false), mobileui.Rect{X: 0, Y: 0, W: 100, H: 100})
	selected := NewCanvas(400, 400)
	selected.Place(k.Card(true), mobileui.Rect{X: 0, Y: 0, W: 100, H: 100})

	a := layoutAndDraw(t, plain)
	b := layoutAndDraw(t, selected)

	if len(a.RoundRects) == 0 || len(b.RoundRects) == 0 {
		t.Fatal("cards drew no background")
	}
	if a.RoundRects[0].Color == b.RoundRects[0].Color {
		t.Error("selected and unselected cards painted the same fill")
	}
}

func TestWindowDrawsTitleBarAboveBody(t *testing.T) {
	k := testKit()
	c := NewCanvas(1080, 2340)
	c.Place(k.Window("Inventory", k.Text("12 items", RoleBody)), mobileui.Rect{X: 0, Y: 0, W: 600, H: 800})
	canvas := layoutAndDraw(t, c)

	if len(canvas.StyledTexts) == 0 && len(canvas.Texts) == 0 {
		t.Fatal("window drew no text at all")
	}
	found := false
	for _, s := range canvas.StyledTexts {
		if s.Text == "Inventory" {
			found = true
		}
	}
	for _, s := range canvas.Texts {
		if s.Text == "Inventory" {
			found = true
		}
	}
	if !found {
		t.Error("window title was not drawn")
	}
}

func TestMobileTextIsLargerThanDesktop(t *testing.T) {
	// The whole point of rotheme.Mobile: readable text on a phone.
	mobile := NewKit(rotheme.Mobile)
	desktop := NewKit(rotheme.Default)
	mSize, _, _ := mobile.roleStyle(RoleBody)
	dSize, _, _ := desktop.roleStyle(RoleBody)
	if mSize <= dSize {
		t.Errorf("mobile body text %v is not larger than desktop %v", mSize, dSize)
	}
}
