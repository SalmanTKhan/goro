package render

import (
	"time"

	"github.com/gogpu/gg"
	uiapp "github.com/gogpu/ui/app"
	uirender "github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// UIRasterMetrics splits host-managed UI raster cost into the phases that can
// be optimized independently. Total includes context allocation/clear and any
// small bookkeeping not represented by the named sub-phases.
type UIRasterMetrics struct {
	MarkDirty time.Duration
	Draw      time.Duration
	Flush     time.Duration
	ImageCopy time.Duration
	Total     time.Duration
}

// RasterizeUI draws the shared Gogpu widget tree into a retained Goro image.
// The returned image may be reused by passing it as dst; its version is
// advanced so the GPU renderer updates the existing texture.
func RasterizeUI(app *uiapp.App, width, height int, dst *Image) (*Image, bool, error) {
	image, drawn, _, err := RasterizeUIProfiled(app, width, height, dst)
	return image, drawn, err
}

// RasterizeUIProfiled is RasterizeUI with phase timings for Android/mobile
// profiling. It deliberately preserves the exact synchronous host-managed
// drawing semantics used by RasterizeUI.
func RasterizeUIProfiled(app *uiapp.App, width, height int, dst *Image) (*Image, bool, UIRasterMetrics, error) {
	var metrics UIRasterMetrics
	started := time.Now()
	if app == nil || width <= 0 || height <= 0 {
		return dst, false, metrics, nil
	}
	// Android's host-managed presentation creates a cleared canvas for every
	// raster. A normal incremental draw would therefore publish only the dirty
	// widgets and turn every unchanged part of the retained UI texture
	// transparent. Mark the complete tree before drawing so each successful
	// raster is an atomic, self-contained frame.
	win := app.Window()
	if win == nil || win.Root() == nil {
		return dst, false, metrics, nil
	}

	phase := time.Now()
	widget.MarkRedrawInTree(win.Root())
	if ctx := win.Context(); ctx != nil {
		ctx.Invalidate()
	}
	metrics.MarkDirty = time.Since(phase)

	dc := gg.NewContext(width, height)
	defer dc.Close()
	dc.SetRGBA(0, 0, 0, 0)
	dc.Clear()

	phase = time.Now()
	drawn := win.DrawTo(uirender.NewCanvas(dc, width, height))
	metrics.Draw = time.Since(phase)
	if !drawn {
		metrics.Total = time.Since(started)
		return dst, false, metrics, nil
	}

	phase = time.Now()
	if err := dc.FlushGPU(); err != nil {
		metrics.Flush = time.Since(phase)
		metrics.Total = time.Since(started)
		return dst, false, metrics, err
	}
	metrics.Flush = time.Since(phase)

	phase = time.Now()
	image := imageFromGGContext(dc, dst)
	metrics.ImageCopy = time.Since(phase)
	metrics.Total = time.Since(started)
	return image, true, metrics, nil
}
