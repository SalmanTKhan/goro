package mobile

import (
	"testing"

	"github.com/kivutar/goro/mobileui"
)

// fullBag builds an inventory large enough to overflow any viewport, which the
// 8-item fixture never does.
func fullBag(count int) mobileui.MobileInventoryModel {
	model := mobileui.MobileInventoryModel{Weight: 900, MaxWeight: 1000, Zeny: 1284500}
	for i := 0; i < count; i++ {
		model.Items = append(model.Items, mobileui.InventoryItemModel{
			Index:         uint16(i),
			ItemID:        uint16(501 + i),
			Identified:    true,
			DisplayName:   "Item",
			Quantity:      i + 1,
			Category:      mobileui.InventoryCategoryEtc,
			NameAvailable: true,
		})
	}
	return model
}

func scrollScene(t *testing.T, offset float32) (
	mobileui.MobileInventoryModel,
	mobileui.MobileInventoryLayout,
	mobileui.InventoryInteractionState,
) {
	t.Helper()
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model := fullBag(60)
	state := mobileui.InventoryInteractionState{Screen: mobileui.ScreenInventory}
	state.Scroll.Offset = offset
	return model, mobileui.LayoutInventory(vp, mobileui.DefaultInventoryTokens(), model, state), state
}

func TestFullBagOverflowsAndIsScrollable(t *testing.T) {
	model, layout, state := scrollScene(t, 0)
	extent := mobileui.ScrollExtentForLayout(model, state, mobileui.DefaultInventoryTokens(), layout)
	if extent.ContentExtent <= extent.ViewportExtent {
		t.Fatalf("a 60-item bag should overflow: content=%v viewport=%v", extent.ContentExtent, extent.ViewportExtent)
	}
	if extent.MaxOffset() <= 0 {
		t.Fatalf("no scroll range available: %+v", extent)
	}
}

func TestScrollingChangesWhichItemsAreDrawn(t *testing.T) {
	k := testKit()

	_, topLayout, topState := scrollScene(t, 0)
	topModel, _, _ := scrollScene(t, 0)
	top := visibleIndices(IconRects(topModel, topLayout, topState))

	scrolledModel, scrolledLayout, scrolledState := scrollScene(t, 400)
	scrolled := visibleIndices(IconRects(scrolledModel, scrolledLayout, scrolledState))

	if len(top) == 0 || len(scrolled) == 0 {
		t.Fatalf("expected visible cells before and after scrolling (%d / %d)", len(top), len(scrolled))
	}
	if sameIndexSet(top, scrolled) {
		t.Error("scrolling did not change which items are visible")
	}

	// And the rendered tree must follow the same cull, so the raster and the
	// sprite pass stay in agreement while scrolled.
	c := k.InventoryTree(scrolledModel, scrolledLayout, scrolledState)
	for _, child := range c.children {
		if child.rect.Min.Y > scrolledLayout.GridViewport.Bottom()+0.5 &&
			child.rect.Max.Y < scrolledLayout.Safe.Y+scrolledLayout.Safe.H {
			// A cell below the grid viewport but inside the safe area would be a
			// leak from the scrolled-away region.
			if child.rect.Height() <= scrolledLayout.GridCellHeight+1 {
				t.Errorf("scrolled-away cell drawn at %v", child.rect)
			}
		}
	}
}

func TestScrolledCellsStayInsideTheGridViewport(t *testing.T) {
	model, layout, state := scrollScene(t, 250)
	for _, placement := range IconRects(model, layout, state) {
		if !placement.Rect.Intersects(layout.GridViewport) {
			t.Errorf("sprite at %+v is outside the grid viewport %+v", placement.Rect, layout.GridViewport)
		}
	}
}

func TestScrollIsClampedAtBothEnds(t *testing.T) {
	model, layout, state := scrollScene(t, 0)
	extent := mobileui.ScrollExtentForLayout(model, state, mobileui.DefaultInventoryTokens(), layout)

	extent.SetOffset(-500)
	if extent.Offset != 0 {
		t.Errorf("scrolling above the top was not clamped: %v", extent.Offset)
	}
	extent.SetOffset(extent.MaxOffset() + 5000)
	if extent.Offset != extent.MaxOffset() {
		t.Errorf("scrolling past the end was not clamped: %v want %v", extent.Offset, extent.MaxOffset())
	}
}

func visibleIndices(placements []IconPlacement) []uint16 {
	out := make([]uint16, 0, len(placements))
	for _, p := range placements {
		out = append(out, p.Item.Index)
	}
	return out
}

func sameIndexSet(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
