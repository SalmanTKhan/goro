package mobileui

// MobileMapWarpModel is a renderer-neutral projection of one authoritative
// offline warp. Selecting a warp is presentation state; the destination
// coordinates are retained only so the existing movement/warp authority can
// receive a normal CommandMoveTo after confirmation.
type MobileMapWarpModel struct {
	ID             uint32
	Name           string
	X              int
	Y              int
	Width          int
	Height         int
	DestinationMap string
	DestinationX   int
	DestinationY   int
}

type MobileMapModel struct {
	MapName string
	PlayerX int
	PlayerY int
	Raster  MinimapRaster
	Markers []MinimapMarkerModel
	Warps   []MobileMapWarpModel
}

type MapSource struct {
	MapName string
	PlayerX int
	PlayerY int
	Raster  MinimapRaster
	Markers []MinimapMarkerModel
	Warps   []MobileMapWarpModel
}

func ProjectMap(source MapSource) MobileMapModel {
	return MobileMapModel{
		MapName: source.MapName,
		PlayerX: source.PlayerX,
		PlayerY: source.PlayerY,
		Raster:  MinimapRaster{Width: source.Raster.Width, Height: source.Raster.Height, Cells: append([]uint8(nil), source.Raster.Cells...)},
		Markers: append([]MinimapMarkerModel(nil), source.Markers...),
		Warps:   append([]MobileMapWarpModel(nil), source.Warps...),
	}
}

func (m MobileMapModel) Warp(id uint32) (MobileMapWarpModel, bool) {
	for _, warp := range m.Warps {
		if warp.ID == id {
			return warp, true
		}
	}
	return MobileMapWarpModel{}, false
}
