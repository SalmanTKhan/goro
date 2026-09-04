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
