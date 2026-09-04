// Package mobile builds gogpu/ui widget trees for the mobileui presentation.
//
// The split of responsibility is deliberate and narrow:
//
//   - mobileui owns geometry, state and touch. Its layout functions produce
//     absolute rectangles and its controllers hit-test them. None of that
//     changes here.
//   - this package owns pixels. It turns those rectangles into widgets styled
//     by rotheme, so the mobile presentation renders through the same
//     rasterizer, palette and font as the desktop client instead of a private
//     set of DrawRect calls.
//
// Builders in this package are pure: models and layout rectangles in, a widget
// tree out. They never read session state and never emit commands. In
// particular the trees are non-interactive — Canvas.Event always declines —
// because taps continue to flow through mobileui's existing controllers rather
// than through gogpu event dispatch. Keeping one hit-testing authority is what
// prevents the visual and touch geometry from drifting apart.
package mobile

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/mobileui"
)

// Canvas places children at absolute positions. gogpu/ui offers flex boxes but
// no absolute layout, and mobileui has already solved the layout problem; this
// is the adapter between the two.
type Canvas struct {
	widget.WidgetBase
	width, height float32
	children      []placedChild
}

type placedChild struct {
	widget widget.Widget
	rect   geometry.Rect
}

// NewCanvas returns an empty canvas covering the given logical size.
func NewCanvas(width, height float32) *Canvas {
	c := &Canvas{width: width, height: height}
	c.SetVisible(true)
	c.SetEnabled(true)
	c.SetBounds(geometry.NewRect(0, 0, width, height))
	return c
}

// Place adds a child at an absolute mobileui rectangle. Zero-area rectangles
// are dropped: layout code routinely returns them for sections that are not
// visible at the current viewport, and placing them would cost a draw call for
// nothing.
func (c *Canvas) Place(child widget.Widget, r mobileui.Rect) *Canvas {
	if child == nil || r.W <= 0 || r.H <= 0 {
		return c
	}
	if setter, ok := child.(interface{ SetParent(widget.Widget) }); ok {
		setter.SetParent(c)
	}
	c.children = append(c.children, placedChild{widget: child, rect: toGeometry(r)})
	return c
}

// PlaceAll adds several children sharing one rectangle, bottom to top.
func (c *Canvas) PlaceAll(r mobileui.Rect, children ...widget.Widget) *Canvas {
	for _, child := range children {
		c.Place(child, r)
	}
	return c
}

// Size reports the logical size the canvas was built for.
func (c *Canvas) Size() (float32, float32) { return c.width, c.height }

func (c *Canvas) Layout(ctx widget.Context, _ geometry.Constraints) geometry.Size {
	size := geometry.Sz(c.width, c.height)
	c.SetBounds(geometry.NewRect(0, 0, size.Width, size.Height))
	for _, child := range c.children {
		childSize := geometry.Sz(child.rect.Width(), child.rect.Height())
		child.widget.Layout(ctx, geometry.Tight(childSize))
		// The child lays out in its own local space; the absolute offset is
		// applied as a transform at draw time, matching how ui/window.go
		// positions overlays.
		if setter, ok := child.widget.(interface{ SetBounds(geometry.Rect) }); ok {
			setter.SetBounds(geometry.FromPointSize(geometry.Point{}, childSize))
		}
	}
	return size
}

func (c *Canvas) Draw(ctx widget.Context, canvas widget.Canvas) {
	if !c.IsVisible() {
		return
	}
	for _, child := range c.children {
		canvas.PushTransform(child.rect.Min)
		widget.StampScreenOrigin(child.widget, canvas)
		widget.DrawChild(child.widget, ctx, canvas)
		canvas.PopTransform()
	}
}

// Event always declines. mobileui's controllers own hit-testing for this
// presentation; see the package comment.
func (c *Canvas) Event(widget.Context, event.Event) bool { return false }

func (c *Canvas) Children() []widget.Widget {
	out := make([]widget.Widget, 0, len(c.children))
	for _, child := range c.children {
		out = append(out, child.widget)
	}
	return out
}

func toGeometry(r mobileui.Rect) geometry.Rect {
	return geometry.NewRect(r.X, r.Y, r.W, r.H)
}
