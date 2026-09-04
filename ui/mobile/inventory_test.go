package mobile

import (
	"testing"

	"github.com/gogpu/ui/uitest"
	"github.com/kivutar/goro/mobileui"
)

func inventoryFixture(t testing.TB, vp mobileui.Viewport) (
	mobileui.MobileInventoryModel,
	mobileui.MobileInventoryLayout,
	mobileui.InventoryInteractionState,
) {
	if t != nil {
		t.Helper()
	}
	model := mobileui.FixtureInventory("default")
	state := mobileui.InventoryInteractionState{Screen: mobileui.ScreenInventory}
	layout := mobileui.LayoutInventory(vp, mobileui.DefaultInventoryTokens(), model, state)
	return model, layout, state
}

func qualViewports() []struct {
	Name string
	VP   mobileui.Viewport
} {
	return []struct {
		Name string
		VP   mobileui.Viewport
	}{
		{"portrait", mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}},
		{"landscape", mobileui.Viewport{Width: 2340, Height: 1080, SafeLeft: 48, SafeRight: 48}},
		{"fold-outer", mobileui.FoldOuterViewport()},
	}
}

func TestInventoryTreeBuildsAtEveryViewport(t *testing.T) {
	k := testKit()
	for _, vp := range qualViewports() {
		t.Run(vp.Name, func(t *testing.T) {
			model, layout, state := inventoryFixture(t, vp.VP)
			c := k.InventoryTree(model, layout, state)
			canvas := layoutAndDraw(t, c)

			if len(c.Children()) == 0 {
				t.Fatal("inventory tree is empty")
			}
			if len(canvas.RoundRects) == 0 {
				t.Error("inventory drew no surfaces")
			}
			if len(canvas.StyledTexts) == 0 && len(canvas.Texts) == 0 {
				t.Error("inventory drew no text")
			}
		})
	}
}

func TestInventoryStaysInsideSafeArea(t *testing.T) {
	k := testKit()
	for _, vp := range qualViewports() {
		t.Run(vp.Name, func(t *testing.T) {
			model, layout, state := inventoryFixture(t, vp.VP)
			safe := layout.Safe
			c := k.InventoryTree(model, layout, state)
			for i, child := range c.children {
				r := child.rect
				if r.Min.X < safe.X-0.5 || r.Min.Y < safe.Y-0.5 ||
					r.Max.X > safe.X+safe.W+0.5 || r.Max.Y > safe.Y+safe.H+0.5 {
					t.Errorf("child %d at %v escapes the safe area %+v", i, r, safe)
				}
			}
		})
	}
}

func TestInventoryOnlyDrawsVisibleCells(t *testing.T) {
	// The grid scrolls; cells scrolled out of the viewport must not be placed,
	// otherwise every frame pays for the whole inventory.
	k := testKit()
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model, layout, state := inventoryFixture(t, vp)
	if len(layout.Cells) == 0 {
		t.Skip("fixture produced no cells")
	}
	// Push the grid far out of view; nothing from it should be placed then.
	// Cells must be copied, not aliased — the layout struct shares the slice.
	scrolled := layout
	scrolled.Cells = append([]mobileui.InventoryCellRect(nil), layout.Cells...)
	for i := range scrolled.Cells {
		scrolled.Cells[i].Rect.Y += layout.GridViewport.H * 10
	}
	c := k.InventoryTree(model, scrolled, state)
	for _, child := range c.children {
		if child.rect.Min.Y > layout.GridViewport.Bottom()+0.5 {
			t.Fatalf("a scrolled-away cell was still placed at %v", child.rect)
		}
	}
	if got := len(IconRects(model, scrolled, state)); got != 0 {
		t.Errorf("scrolled-away cells produced %d sprite placements, want 0", got)
	}

	// And with the real layout, the visible ones are placed.
	if got := len(IconRects(model, layout, state)); got == 0 {
		t.Error("no sprites placed for a fixture with items in view")
	}
}

func TestInventorySelectionIsVisiblyDistinct(t *testing.T) {
	k := testKit()
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model := mobileui.FixtureInventory("default")
	if len(model.Items) == 0 {
		t.Skip("fixture has no items")
	}

	plainState := mobileui.InventoryInteractionState{Screen: mobileui.ScreenInventory}
	plainLayout := mobileui.LayoutInventory(vp, mobileui.DefaultInventoryTokens(), model, plainState)
	plain := layoutAndDraw(t, k.InventoryTree(model, plainLayout, plainState))

	selState := mobileui.InventoryInteractionState{Screen: mobileui.ScreenInventory}
	selState.Select(model, model.Items[0].Index)
	selLayout := mobileui.LayoutInventory(vp, mobileui.DefaultInventoryTokens(), model, selState)
	selected := layoutAndDraw(t, k.InventoryTree(model, selLayout, selState))

	if len(plain.RoundRects) == len(selected.RoundRects) && sameColors(plain, selected) {
		t.Error("selecting an item changed nothing on screen")
	}
}

func sameColors(a, b *uitest.MockCanvas) bool {
	if len(a.RoundRects) != len(b.RoundRects) {
		return false
	}
	for i := range a.RoundRects {
		if a.RoundRects[i].Color != b.RoundRects[i].Color {
			return false
		}
	}
	return true
}

func TestIconRectsMatchOccupiedCells(t *testing.T) {
	// The sprite pass and the raster must agree about cell geometry; they are
	// two passes over the same layout, so this pins them together.
	vp := mobileui.Viewport{Width: 1080, Height: 2340, SafeTop: 48, SafeBottom: 36}
	model, layout, state := inventoryFixture(t, vp)
	icons := IconRects(model, layout, state)
	items := state.FilteredItems(model)

	for _, icon := range icons {
		if !icon.Rect.Intersects(layout.GridViewport) {
			t.Errorf("icon for item %d placed outside the grid viewport", icon.Item.ItemID)
		}
		found := false
		for _, item := range items {
			if item.Index == icon.Item.Index {
				found = true
			}
		}
		if !found {
			t.Errorf("icon for index %d is not in the filtered item set", icon.Item.Index)
		}
	}
}

func TestGroupDigitsFormatsZeny(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{2147483647, "2,147,483,647"},
		{-4500, "-4,500"},
	} {
		if got := groupDigits(tc.in); got != tc.want {
			t.Errorf("groupDigits(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
