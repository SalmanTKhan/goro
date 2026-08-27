package res

import (
	"image"
	"strings"
)

// GNDTextureDiagnostic describes one texture referenced by a GND texture
// table. It is intentionally a read-only diagnostic projection; it is not
// part of the renderer or gameplay state.
type GNDTextureDiagnostic struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	UsedByTopCells    int    `json:"used_by_top_cells"`
	UsedByWallCells   int    `json:"used_by_wall_cells"`
	Resolved          bool   `json:"resolved"`
	Source            string `json:"source,omitempty"`
	EncodedBytes      int64  `json:"encoded_bytes,omitempty"`
	Width             int    `json:"width,omitempty"`
	Height            int    `json:"height,omitempty"`
	DecodedRGBABytes  int64  `json:"decoded_rgba_bytes,omitempty"`
	TransparentPixels int64  `json:"transparent_pixels,omitempty"`
	DecodeError       string `json:"decode_error,omitempty"`
}

// GNDDiagnostics records the source-level reasons a terrain surface can fail
// to appear. Counts are deterministic for a given GND and resource root.
type GNDDiagnostics struct {
	Width                     int                    `json:"width"`
	Height                    int                    `json:"height"`
	CellCount                 int                    `json:"cell_count"`
	TextureCount              int                    `json:"texture_count"`
	LightmapCount             int                    `json:"lightmap_count"`
	SurfaceCount              int                    `json:"surface_count"`
	TopCells                  int                    `json:"top_cells"`
	EmptyTopCells             int                    `json:"empty_top_cells"`
	FrontCells                int                    `json:"front_cells"`
	RightCells                int                    `json:"right_cells"`
	InvalidTopReferences      int                    `json:"invalid_top_references"`
	InvalidFrontReferences    int                    `json:"invalid_front_references"`
	InvalidRightReferences    int                    `json:"invalid_right_references"`
	InvalidTextureReferences  int                    `json:"invalid_texture_references"`
	InvalidLightmapReferences int                    `json:"invalid_lightmap_references"`
	LightmapPixels            int                    `json:"lightmap_pixels"`
	LightmapWhiteColorPixels  int                    `json:"lightmap_white_color_pixels"`
	LightmapZeroColorPixels   int                    `json:"lightmap_zero_color_pixels"`
	LightmapZeroAlphaPixels   int                    `json:"lightmap_zero_alpha_pixels"`
	UVOutsideUnit             int                    `json:"uv_outside_unit"`
	ZeroAlphaSurfaceColors    int                    `json:"zero_alpha_surface_colors"`
	UsedSurfaceCount          int                    `json:"used_surface_count"`
	TexturesResolved          int                    `json:"textures_resolved"`
	TexturesMissing           int                    `json:"textures_missing"`
	Textures                  []GNDTextureDiagnostic `json:"textures"`
}

// AnalyzeGND inspects map references and, when manager is non-nil, resolves
// and decodes every texture in the GND table. It does not substitute fallback
// colors and does not mutate the parsed map.
func AnalyzeGND(gnd *GND, manager *Manager) GNDDiagnostics {
	if gnd == nil {
		return GNDDiagnostics{}
	}
	diagnostics := GNDDiagnostics{
		Width:         gnd.Width,
		Height:        gnd.Height,
		CellCount:     len(gnd.Cells),
		TextureCount:  len(gnd.Textures),
		LightmapCount: len(gnd.Lightmaps),
		SurfaceCount:  len(gnd.Surfaces),
		Textures:      make([]GNDTextureDiagnostic, len(gnd.Textures)),
	}
	for id, name := range gnd.Textures {
		diagnostics.Textures[id] = GNDTextureDiagnostic{ID: id, Name: name}
	}
	for _, lightmap := range gnd.Lightmaps {
		for y := range lightmap.Alpha {
			for x := range lightmap.Alpha[y] {
				diagnostics.LightmapPixels++
				if lightmap.Alpha[y][x] == 0 {
					diagnostics.LightmapZeroAlphaPixels++
				}
				pixel := lightmap.Color[y][x]
				if pixel.R == 255 && pixel.G == 255 && pixel.B == 255 {
					diagnostics.LightmapWhiteColorPixels++
				}
				if pixel.R == 0 && pixel.G == 0 && pixel.B == 0 {
					diagnostics.LightmapZeroColorPixels++
				}
			}
		}
	}
	usedSurfaces := make(map[int]struct{})
	for _, cell := range gnd.Cells {
		if cell.Top < 0 {
			diagnostics.EmptyTopCells++
		} else {
			diagnostics.TopCells++
			diagnostics.inspectSurface(gnd, cell.Top, usedSurfaces, true)
		}
		if cell.Front >= 0 {
			diagnostics.FrontCells++
			if cell.Front >= len(gnd.Surfaces) {
				diagnostics.InvalidFrontReferences++
			} else {
				diagnostics.inspectSurface(gnd, cell.Front, usedSurfaces, false)
			}
		}
		if cell.Right >= 0 {
			diagnostics.RightCells++
			if cell.Right >= len(gnd.Surfaces) {
				diagnostics.InvalidRightReferences++
			} else {
				diagnostics.inspectSurface(gnd, cell.Right, usedSurfaces, false)
			}
		}
	}
	diagnostics.UsedSurfaceCount = len(usedSurfaces)
	for id := range gnd.Textures {
		if manager == nil || strings.TrimSpace(gnd.Textures[id]) == "" {
			if strings.TrimSpace(gnd.Textures[id]) == "" {
				diagnostics.TexturesMissing++
			}
			continue
		}
		candidates := GroundTextureCandidates(gnd.Textures[id])
		var encoded []byte
		var source string
		var err error
		for _, candidate := range candidates {
			encoded, err = manager.ReadFile(candidate)
			if err == nil {
				source = candidate
				break
			}
		}
		if err != nil || source == "" {
			diagnostics.TexturesMissing++
			diagnostics.Textures[id].DecodeError = "resource not found"
			continue
		}
		diagnostics.Textures[id].EncodedBytes = int64(len(encoded))
		diagnostics.Textures[id].Source = source
		decoded, decodeErr := DecodeImageData(encoded)
		if decodeErr != nil {
			diagnostics.Textures[id].DecodeError = decodeErr.Error()
			diagnostics.TexturesMissing++
			continue
		}
		diagnostics.Textures[id].Resolved = true
		diagnostics.TexturesResolved++
		bounds := decoded.Bounds()
		diagnostics.Textures[id].Width = bounds.Dx()
		diagnostics.Textures[id].Height = bounds.Dy()
		diagnostics.Textures[id].DecodedRGBABytes = int64(bounds.Dx()) * int64(bounds.Dy()) * 4
		diagnostics.Textures[id].TransparentPixels = transparentPixelCount(decoded)
	}
	return diagnostics
}

func (d *GNDDiagnostics) inspectSurface(gnd *GND, surfaceID int, used map[int]struct{}, top bool) {
	if surfaceID < 0 || surfaceID >= len(gnd.Surfaces) {
		if top {
			d.InvalidTopReferences++
		}
		return
	}
	used[surfaceID] = struct{}{}
	surface := gnd.Surfaces[surfaceID]
	if surface.TextureID < 0 || surface.TextureID >= len(gnd.Textures) {
		d.InvalidTextureReferences++
	} else if top {
		d.Textures[surface.TextureID].UsedByTopCells++
	} else {
		d.Textures[surface.TextureID].UsedByWallCells++
	}
	if surface.LightmapID < 0 || surface.LightmapID >= len(gnd.Lightmaps) {
		d.InvalidLightmapReferences++
	}
	if surface.Color.A == 0 {
		d.ZeroAlphaSurfaceColors++
	}
	for i := range surface.U {
		if surface.U[i] < 0 || surface.U[i] > 1 || surface.V[i] < 0 || surface.V[i] > 1 {
			d.UVOutsideUnit++
		}
	}
}

func transparentPixelCount(img image.Image) int64 {
	if img == nil {
		return 0
	}
	var count int64
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < 65535 {
				count++
			}
		}
	}
	return count
}
