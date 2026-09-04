package render

import (
	"testing"

	"github.com/gogpu/gpucontext"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
)

func TestRasterizeUIProducesPixels(t *testing.T) {
	app := uiapp.New(uiapp.WithWindowProvider(gpucontext.NullWindowProvider{W: 1280, H: 720}))
	app.SetRoot(primitives.Box().Width(100).Height(100).Background(widget.RGBA8(255, 0, 0, 255)))
	app.Frame()
	image, drawn, err := RasterizeUI(app, 1280, 720, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !drawn || image == nil {
		t.Fatalf("drawn=%t image=%v", drawn, image)
	}
	for i := 3; i < len(image.RGBA().Pix); i += 4 {
		if image.RGBA().Pix[i] != 0 {
			return
		}
	}
	t.Fatal("raster image contains no nontransparent pixels")
}

func TestRasterizeUIRepaintsCleanSiblingsIntoFreshFrame(t *testing.T) {
	app := uiapp.New(uiapp.WithWindowProvider(gpucontext.NullWindowProvider{W: 200, H: 100}), uiapp.WithRenderMode(uiapp.RenderModeHostManaged))
	left := primitives.Box().Width(100).Height(100).Background(widget.RGBA8(255, 0, 0, 255))
	right := primitives.Box().Width(100).Height(100).Background(widget.RGBA8(0, 255, 0, 255))
	root := primitives.HBox(left, right)
	app.SetRoot(root)
	app.Frame()
	first, drawn, err := RasterizeUI(app, 200, 100, nil)
	if err != nil || !drawn || first == nil {
		t.Fatalf("initial raster drawn=%t image=%v err=%v", drawn, first, err)
	}

	left.SetNeedsRedraw(true)
	app.Window().Context().Invalidate()
	second, drawn, err := RasterizeUI(app, 200, 100, first)
	if err != nil || !drawn || second == nil {
		t.Fatalf("incremental raster drawn=%t image=%v err=%v", drawn, second, err)
	}
	for _, x := range []int{25, 175} {
		if alpha := second.RGBA().RGBAAt(x, 50).A; alpha == 0 {
			t.Fatalf("alpha at x=%d is zero; clean sibling disappeared", x)
		}
	}
}
