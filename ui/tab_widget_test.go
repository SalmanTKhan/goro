package ui

import (
	"image"
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/uitest"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/ui/rotheme"
)

func TestDisabledTabDoesNotRunClickHandler(t *testing.T) {
	clicked := false
	tab := newTabWidget(tabWidgetConfig{
		disabled: true,
		onClick:  func() { clicked = true },
	})

	press := event.NewMouseEvent(event.MousePress, event.ButtonLeft, event.ButtonStateLeft, geometry.Pt(1, 1), geometry.Pt(1, 1), 0)
	if !tab.Event(widget.NewContext(), press) {
		t.Fatal("disabled tab did not consume the click")
	}
	if clicked {
		t.Fatal("disabled tab ran its click handler")
	}
}

func TestTabWidgetKeepsHorizontalLabelsByDefault(t *testing.T) {
	tab := newTabWidget(tabWidgetConfig{label: "Friends", width: 72, height: 24})
	tab.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(72, 24)))
	canvas := &uitest.MockCanvas{}

	tab.Draw(widget.NewContext(), canvas)

	if len(canvas.StyledTexts) != 1 || canvas.StyledTexts[0].Text != "Friends" {
		t.Fatalf("horizontal label draws = %+v, want one styled text draw", canvas.StyledTexts)
	}
	if len(canvas.Images) != 0 {
		t.Fatalf("horizontal label image draws = %d, want 0", len(canvas.Images))
	}
}

func TestTabWidgetDrawsCounterClockwiseLabelAsCenteredImage(t *testing.T) {
	tab := newTabWidget(tabWidgetConfig{
		label:         "Equip",
		labelRotation: rotheme.TextRotationCounterClockwise,
		width:         inventoryBagTabW,
		height:        inventoryBagTabH,
	})
	tab.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(inventoryBagTabW, inventoryBagTabH)))
	canvas := &uitest.MockCanvas{}

	tab.Draw(widget.NewContext(), canvas)

	if len(canvas.StyledTexts) != 0 || len(canvas.Texts) != 0 {
		t.Fatal("rotated label should not use the horizontal text path")
	}
	if len(canvas.Images) != 1 {
		t.Fatalf("rotated label image draws = %d, want 1", len(canvas.Images))
	}
	call := canvas.Images[0]
	imageBounds := call.Image.Bounds()
	if imageBounds.Dy() <= imageBounds.Dx() {
		t.Fatalf("rotated label image size = %v, want portrait orientation", imageBounds.Size())
	}
	scalable, ok := call.Image.(interface {
		RasterizeForScale(scale float32, width, height int) image.Image
	})
	if !ok {
		t.Fatal("rotated label image does not support native-resolution rasterization")
	}
	scaled := scalable.RasterizeForScale(2, imageBounds.Dx()*2, imageBounds.Dy()*2)
	if scaled.Bounds().Dx() != imageBounds.Dx()*2 || scaled.Bounds().Dy() != imageBounds.Dy()*2 {
		t.Fatalf("2x rotated label size = %v, want %dx%d", scaled.Bounds().Size(), imageBounds.Dx()*2, imageBounds.Dy()*2)
	}
	center := geometry.Pt(
		call.At.X+float32(imageBounds.Dx())/2,
		call.At.Y+float32(imageBounds.Dy())/2,
	)
	if distance := center.Sub(tab.Bounds().Center()); distance.X < -0.5 || distance.X > 0.5 || distance.Y < -0.5 || distance.Y > 0.5 {
		t.Fatalf("rotated label center offset = %v, want at most half a pixel", distance)
	}
}

func TestInventoryBagUsesOnlyVerticalCategoryTabs(t *testing.T) {
	window := InventoryBagWindow{}
	column := window.tabColumn(Context{})
	column.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(inventoryBagTabRail, inventoryBagViewH)))

	children := column.Children()
	if len(children) != len(inventoryBagTabs) {
		t.Fatalf("tab column children = %d, want only %d category tabs", len(children), len(inventoryBagTabs))
	}
	for i := range inventoryBagTabs {
		tab, ok := children[i].(*tabWidget)
		if !ok {
			t.Fatalf("tab column child %d = %T, want *tabWidget", i, children[i])
		}
		if tab.cfg.labelRotation != rotheme.TextRotationCounterClockwise {
			t.Fatalf("tab %q rotation = %v, want counter-clockwise", tab.cfg.label, tab.cfg.labelRotation)
		}
	}
}

func TestStorageVerticalTabsFitTableHeight(t *testing.T) {
	column := (&StorageWindow{}).tabColumn(Context{})
	column.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(storageTabRailW, storageTableViewH)))

	children := column.Children()
	if len(children) != len(storageCategoryTabs) {
		t.Fatalf("storage tab column children = %d, want %d", len(children), len(storageCategoryTabs))
	}
	last, ok := children[len(children)-1].(*tabWidget)
	if !ok {
		t.Fatalf("last storage tab = %T, want *tabWidget", children[len(children)-1])
	}
	if last.Bounds().Max.Y > storageTableViewH {
		t.Fatalf("last storage tab bottom = %.1f, table height = %d", last.Bounds().Max.Y, storageTableViewH)
	}
}

func TestVerticalTabFrameSeparatesRailFromContent(t *testing.T) {
	const (
		railW    = 34
		contentW = 80
		height   = 100
	)
	frame := verticalTabFrame(
		primitives.Box().Width(railW),
		primitives.Box().Width(contentW),
	)
	frame.Layout(widget.NewContext(), geometry.Tight(geometry.Sz(railW+verticalTabDividerW+contentW, height)))

	children := frame.Children()
	if len(children) != 3 {
		t.Fatalf("vertical tab frame children = %d, want rail, divider, and content", len(children))
	}
	dividerBounds := children[1].(interface{ Bounds() geometry.Rect }).Bounds()
	if dividerBounds.Min.X != railW || dividerBounds.Width() != verticalTabDividerW || dividerBounds.Height() != height {
		t.Fatalf("vertical tab divider bounds = %v, want x=%d width=%d height=%d", dividerBounds, railW, verticalTabDividerW, height)
	}
	canvas := &uitest.MockCanvas{}
	children[1].Draw(widget.NewContext(), canvas)
	if len(canvas.Rects) != 1 {
		t.Fatalf("vertical tab divider draws = %d, want one visible rectangle", len(canvas.Rects))
	}
	uitest.AssertColorEqual(t, canvas.Rects[0].Color, rotheme.Default.Colors.WindowBorder)
	contentBounds := children[2].(interface{ Bounds() geometry.Rect }).Bounds()
	if contentBounds.Min.X != railW+verticalTabDividerW {
		t.Fatalf("vertical tab content x = %.1f, want %d", contentBounds.Min.X, railW+verticalTabDividerW)
	}
}
