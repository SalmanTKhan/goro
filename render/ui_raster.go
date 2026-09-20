package render

import (
	"github.com/gogpu/gg"
	uiapp "github.com/gogpu/ui/app"
	uirender "github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// RasterizeUI draws the shared Gogpu widget tree into a retained Goro image.
// The returned image may be reused by passing it as dst; its version is
// advanced so the GPU renderer updates the existing texture.
func RasterizeUI(app *uiapp.App, width, height int, dst *Image) (*Image, bool, error) {
	if app == nil || width <= 0 || height <= 0 {
		return dst, false, nil
	}
	// Android's host-managed presentation creates a cleared canvas for every
	// raster. A normal incremental draw would therefore publish only the dirty
	// widgets and turn every unchanged part of the retained UI texture
	// transparent. Mark the complete tree before drawing so each successful
	// raster is an atomic, self-contained frame.
	win := app.Window()
	if win == nil || win.Root() == nil {
		return dst, false, nil
	}
	widget.MarkRedrawInTree(win.Root())
	if ctx := win.Context(); ctx != nil {
		ctx.Invalidate()
	}
	dc := gg.NewContext(width, height)
	dc.SetRGBA(0, 0, 0, 0)
	dc.Clear()
	drawn := win.DrawTo(uirender.NewCanvas(dc, width, height))
	if !drawn {
		return dst, false, nil
	}
	if err := dc.FlushGPU(); err != nil {
		return dst, false, err
	}
	return imageFromGGContext(dc, dst), true, nil
}


// RetainedUIRasterizer keeps a CPU raster surface alive across host-managed UI
// redraws. Unlike RasterizeUI, it does not force the complete widget tree dirty
// for every update. gogpu/ui can therefore repaint only its accumulated dirty
// regions while unchanged pixels remain on the retained surface.
//
// ForceFull should be used when the UI root, viewport, or presentation scale
// changes. It clears the retained pixels first so removed widgets cannot leave
// stale content behind.
type RetainedUIRasterizer struct {
	dc            *gg.Context
	width, height int
}

func (r *RetainedUIRasterizer) Close() {
	if r == nil || r.dc == nil {
		return
	}
	_ = r.dc.Close()
	r.dc = nil
	r.width = 0
	r.height = 0
}

func (r *RetainedUIRasterizer) ensure(width, height int) bool {
	if r.dc != nil && r.width == width && r.height == height {
		return false
	}
	r.Close()
	r.dc = gg.NewContext(width, height)
	r.width = width
	r.height = height
	r.clear()
	return true
}

func (r *RetainedUIRasterizer) clear() {
	if r == nil || r.dc == nil {
		return
	}
	r.dc.SetRGBA(0, 0, 0, 0)
	r.dc.Clear()
}

// Rasterize updates the retained surface and copies the resulting pixels into
// dst. The Image identity is retained when dimensions match, so the GPU side
// performs an in-place texture update instead of allocating a replacement.
func (r *RetainedUIRasterizer) Rasterize(app *uiapp.App, width, height int, dst *Image, forceFull bool) (*Image, bool, error) {
	if r == nil || app == nil || width <= 0 || height <= 0 {
		return dst, false, nil
	}
	win := app.Window()
	if win == nil || win.Root() == nil {
		return dst, false, nil
	}
	if r.ensure(width, height) {
		forceFull = true
	}
	if forceFull {
		r.clear()
		widget.MarkRedrawInTree(win.Root())
		if ctx := win.Context(); ctx != nil {
			ctx.Invalidate()
		}
	}

	drawn := win.DrawTo(uirender.NewCanvas(r.dc, width, height))
	if !drawn {
		return dst, false, nil
	}
	if err := r.dc.FlushGPU(); err != nil {
		return dst, false, err
	}
	return imageFromGGContext(r.dc, dst), true, nil
}
