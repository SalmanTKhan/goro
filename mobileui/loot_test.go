package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestLootControllerConsumesRowsAndEmitsPickup(t *testing.T) {
	model := Fixture("loot-basic")
	var commands input.CommandBuffer
	c := NewController(model, FoldOuterViewport(), &commands)
	if len(c.Layout.LootRows) != 1 {
		t.Fatalf("loot rows = %d, want one", len(c.Layout.LootRows))
	}
	row := c.Layout.LootRows[0]
	if !c.ConsumeTouch(input.TouchPoint{X: int(row.X + 8), Y: int(row.Y + 8)}) {
		t.Fatal("loot row touch was not owned by HUD")
	}
	if !c.Tap(row.X+8, row.Y+8) {
		t.Fatal("loot row tap was not handled")
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandPickUpItem || got[0].ItemID != 50001 {
		t.Fatalf("pickup command = %+v", got)
	}
}

func TestLootLayoutIsSafeAndDoesNotCoverCombatControls(t *testing.T) {
	l := LayoutHUD(FoldOuterViewport(), DefaultTokens(), Fixture("loot-many"), Navigation{})
	if l.LootPanel.W == 0 || len(l.LootRows) != 6 {
		t.Fatalf("loot layout = panel:%+v rows:%d, want six visible rows", l.LootPanel, len(l.LootRows))
	}
	if l.LootPanel.X < l.Safe.X || l.LootPanel.Y < l.Safe.Y || l.LootPanel.Right() > l.Safe.Right() || l.LootPanel.Bottom() > l.Safe.Bottom() {
		t.Fatalf("loot panel escapes safe area: %+v in %+v", l.LootPanel, l.Safe)
	}
	if l.LootPanel.Intersects(l.TargetPanel) || l.LootPanel.Intersects(l.SkillBar) || l.LootPanel.Intersects(l.ChatBar) {
		t.Fatalf("loot panel overlaps another control: loot=%+v target=%+v skills=%+v chat=%+v", l.LootPanel, l.TargetPanel, l.SkillBar, l.ChatBar)
	}
	for i, row := range l.LootRows {
		if row.W < DefaultTokens().MinTouchTarget || row.H < DefaultTokens().MinTouchTarget {
			t.Fatalf("loot row %d below touch target: %+v", i, row)
		}
		if row.X < l.Safe.X || row.Y < l.Safe.Y || row.Right() > l.Safe.Right() || row.Bottom() > l.Safe.Bottom() {
			t.Fatalf("loot row %d escapes safe area: %+v in %+v", i, row, l.Safe)
		}
	}
}

func TestProjectCopiesLootSlice(t *testing.T) {
	source := HUDSource{Loot: []LootItemModel{{DropID: 50001, Name: "Jellopy", Quantity: 1}}}
	model := Project(source)
	source.Loot[0].Name = "changed"
	if model.Loot[0].Name != "Jellopy" {
		t.Fatalf("projected loot shared source backing: %+v", model.Loot)
	}
}
