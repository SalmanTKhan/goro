package ui

import (
	"testing"

	"github.com/kivutar/goro/network"
)

func TestAutoSpellWindowConfirmsSelectedSkill(t *testing.T) {
	window := AutoSpellWindow{}
	ctx := Context{ScreenW: 800, ScreenH: 600}
	window.OpenList(ctx, network.AutoSpellList{SkillIDs: []uint16{11, 14, 19}}, nil)
	window.selectedRow = 1

	window.confirm(ctx)

	if window.IsOpen() {
		t.Fatal("auto spell window stayed open after selection")
	}
	if skillID, ok := window.PopSelection(); !ok || skillID != 14 {
		t.Fatalf("selection = %d, %t, want 14, true", skillID, ok)
	}
	if _, ok := window.PopSelection(); ok {
		t.Fatal("auto spell selection was returned more than once")
	}
}

func TestAutoSpellWindowCancelSelectsZero(t *testing.T) {
	window := AutoSpellWindow{}
	ctx := Context{ScreenW: 800, ScreenH: 600}
	window.OpenList(ctx, network.AutoSpellList{SkillIDs: []uint16{11}}, nil)

	window.cancel(ctx)

	if skillID, ok := window.PopSelection(); !ok || skillID != 0 {
		t.Fatalf("cancel selection = %d, %t, want 0, true", skillID, ok)
	}
}

func TestAutoSpellWindowResetDropsPendingSelection(t *testing.T) {
	window := AutoSpellWindow{}
	ctx := Context{ScreenW: 800, ScreenH: 600}
	window.OpenList(ctx, network.AutoSpellList{SkillIDs: []uint16{11}}, nil)
	window.cancel(ctx)

	window.Reset(ctx)

	if _, ok := window.PopSelection(); ok {
		t.Fatal("reset retained a stale auto spell selection")
	}
	if window.IsOpen() {
		t.Fatal("reset retained the auto spell window")
	}
}
