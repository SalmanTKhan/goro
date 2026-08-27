package mobileui

import "fmt"

func FixtureMap(name string) MobileMapModel {
	model := MobileMapModel{MapName: "prontera", PlayerX: 78, PlayerY: 98, Raster: fixtureMapRaster()}
	switch name {
	case "map-empty":
		return model
	case "map-long-names":
		model.Warps = []MobileMapWarpModel{
			{ID: 7001, Name: "South gate to the Prontera field entrance", X: 14, Y: 12, DestinationMap: "prt_fild00", DestinationX: 100, DestinationY: 100},
			{ID: 7002, Name: "East district road to the Payon approach", X: 48, Y: 16, DestinationMap: "payon_forest_very_long_destination_name", DestinationX: 208, DestinationY: 122},
		}
	default:
		model.Warps = []MobileMapWarpModel{
			{ID: 7001, Name: "South Gate", X: 14, Y: 12, DestinationMap: "prt_fild05", DestinationX: 367, DestinationY: 205},
			{ID: 7002, Name: "East Gate", X: 48, Y: 16, DestinationMap: "payon", DestinationX: 208, DestinationY: 122},
		}
	}
	if name == "map-many" {
		model.Warps = model.Warps[:0]
		for i := 0; i < 18; i++ {
			model.Warps = append(model.Warps, MobileMapWarpModel{ID: uint32(7001 + i), Name: fmt.Sprintf("Exit %02d", i+1), X: 8 + i%8*7, Y: 8 + i/8*12, DestinationMap: fmt.Sprintf("map_%02d", i+1), DestinationX: 40 + i, DestinationY: 60 + i})
		}
	}
	return model
}

func fixtureMapRaster() MinimapRaster {
	const width, height = 64, 40
	cells := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cells[x+y*width] = 1
			if x == 31 || y == 25 {
				cells[x+y*width] = 2
			}
		}
	}
	return MinimapRaster{Width: width, Height: height, Cells: cells}
}
