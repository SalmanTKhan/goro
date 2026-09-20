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


// IncrementalUIRasterizer keeps the framework-managed backing pixmap alive
// across Android desktop-UI updates. In RenderModeFrameworkManaged, DrawTo can
// then skip idle frames and redraw only dirty regions instead of traversing and
// painting the complete widget tree for every published UI texture.
type IncrementalUIRasterizer struct {
	dc            *gg.Context
	width, height int
}

func (r *IncrementalUIRasterizer) Close() {
	if r == nil || r.dc == nil {
		return
	}
	_ = r.dc.Close()
	r.dc = nil
	r.width = 0
	r.height = 0
}

func (r *IncrementalUIRasterizer) ensure(width, height int) bool {
	if r.dc != nil && r.width == width && r.height == height {
		return false
	}
	r.Close()
	r.dc = gg.NewContext(width, height)
	r.width = width
	r.height = height
	return true
}

// Rasterize updates a persistent framework-managed UI surface. forceFull is
// reserved for structural host changes (first frame, SetRoot, resize, UI scale
// changes) where Android needs an atomic complete surface before publication.
func (r *IncrementalUIRasterizer) Rasterize(app *uiapp.App, width, height int, dst *Image, forceFull bool) (*Image, bool, UIRasterMetrics, error) {
	var metrics UIRasterMetrics
	started := time.Now()
	if r == nil || app == nil || width <= 0 || height <= 0 {
		return dst, false, metrics, nil
	}
	win := app.Window()
	if win == nil || win.Root() == nil {
		return dst, false, metrics, nil
	}
	if r.ensure(width, height) {
		forceFull = true
	}
	if forceFull {
		phase := time.Now()
		// FrameworkManaged normally forces a full repaint after SetRoot/resize,
		// but the Android host also has presentation-only invalidations (safe
		// area and UI scale). Explicitly dirty the tree for those cases while
		// keeping the framework's persistent-pixmap ownership intact.
		widget.MarkRedrawInTree(win.Root())
		if ctx := win.Context(); ctx != nil {
			ctx.Invalidate()
		}
		metrics.MarkDirty = time.Since(phase)
	}

	phase := time.Now()
	drawn := win.DrawTo(uirender.NewCanvas(r.dc, width, height))
	metrics.Draw = time.Since(phase)
	if !drawn {
		metrics.Total = time.Since(started)
		return dst, false, metrics, nil
	}

	phase = time.Now()
	if err := r.dc.FlushGPU(); err != nil {
		metrics.Flush = time.Since(phase)
		metrics.Total = time.Since(started)
		return dst, false, metrics, err
	}
	metrics.Flush = time.Since(phase)

	phase = time.Now()
	image := imageFromGGContext(r.dc, dst)
	metrics.ImageCopy = time.Since(phase)
	metrics.Total = time.Since(started)
	return image, true, metrics, nil
}
