package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestMapSelectionConfirmationEmitsExistingMovementCommand(t *testing.T) {
	var sink input.CommandBuffer
	c := NewMapController(FixtureMap("map-basic"), FoldOuterViewport(), &sink)
	if len(c.Layout.WarpRows) == 0 {
		t.Fatal("map has no visible warp rows")
	}
	row := c.Layout.WarpRows[0].Rect
	if !c.Tap(row.X+8, row.Y+8) {
		t.Fatal("warp selection was not consumed")
	}
	if c.State.SelectedWarpID == 0 {
		t.Fatal("warp selection was not recorded")
	}
	if !c.Tap(c.Layout.TravelButton.X+8, c.Layout.TravelButton.Y+8) || !c.State.ConfirmOpen {
		t.Fatal("travel confirmation did not open")
	}
	if !c.Tap(c.Layout.ConfirmButton.X+8, c.Layout.ConfirmButton.Y+8) {
		t.Fatal("confirmation tap was not consumed")
	}
	commands := sink.Commands()
	if len(commands) != 1 || commands[0].Kind != input.CommandMoveTo {
		t.Fatalf("commands = %+v, want one CommandMoveTo", commands)
	}
	if commands[0].Position.X != 14 || commands[0].Position.Y != 12 {
		t.Fatalf("movement target = %+v, want 14,12", commands[0].Position)
	}
}

func TestMapConfirmationCancelEmitsNoCommand(t *testing.T) {
	var sink input.CommandBuffer
	c := NewMapController(FixtureMap("map-basic"), FoldOuterViewport(), &sink)
	c.Tap(c.Layout.WarpRows[0].Rect.X+8, c.Layout.WarpRows[0].Rect.Y+8)
	c.Tap(c.Layout.TravelButton.X+8, c.Layout.TravelButton.Y+8)
	if !c.State.ConfirmOpen {
		t.Fatal("confirmation did not open")
	}
	c.Tap(c.Layout.ConfirmCancel.X+8, c.Layout.ConfirmCancel.Y+8)
	if c.State.ConfirmOpen || len(sink.Commands()) != 0 {
		t.Fatalf("state=%+v commands=%+v", c.State, sink.Commands())
	}
}

func TestMapScrollClampsAndConsumesScreenInput(t *testing.T) {
	c := NewMapController(FixtureMap("map-many"), FoldOuterViewport(), nil)
	if !c.ConsumeTouch(input.TouchPoint{X: int(c.Layout.WarpPanel.X + 4), Y: int(c.Layout.WarpPanel.Y + 4)}) {
		t.Fatal("map panel did not consume touch")
	}
	c.ScrollBy(10000)
	maxOffset := float32(len(c.Model.Warps))*56 - (c.Layout.DetailPanel.Y - c.Layout.WarpPanel.Y - 52)
	if c.State.ScrollOffset != maxOffset {
		t.Fatalf("scroll offset = %v, max = %v", c.State.ScrollOffset, maxOffset)
	}
	c.ScrollBy(-10000)
	if c.State.ScrollOffset != 0 {
		t.Fatalf("scroll offset after reverse = %v", c.State.ScrollOffset)
	}
}

func TestEmptyMapDoesNotExposeTravelButton(t *testing.T) {
	c := NewMapController(FixtureMap("map-empty"), FoldOuterViewport(), nil)
	if c.Layout.TravelButton.W != 0 || len(c.Layout.WarpRows) != 0 {
		t.Fatalf("empty map layout exposes travel controls: %+v", c.Layout)
	}
}

func TestMapProjectsWorldMarkersWithoutSharingBackingSlices(t *testing.T) {
	source := MapSource{
		MapName: "prontera",
		Raster:  MinimapRaster{Width: 2, Height: 2, Cells: []uint8{1, 0, 1, 1}},
		Markers: []MinimapMarkerModel{{ID: 9, Name: "Poring", X: 4, Y: 6, Kind: MinimapMarkerHostile}},
	}
	model := ProjectMap(source)
	source.Markers[0].Name = "changed"
	source.Raster.Cells[0] = 2
	if model.Markers[0].Name != "Poring" || model.Raster.Cells[0] != 1 {
		t.Fatalf("projected map shared source data: %+v", model)
	}
}
