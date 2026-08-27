package mobilepack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/kivutar/goro/res"
)

// TerrainBakeInfo describes the optional v1 structural terrain projection.
// The original GND remains in the pack and remains the default runtime path.
type TerrainBakeInfo struct {
	Format         string `json:"format"`
	Version        int    `json:"version"`
	SourceResource string `json:"source_resource"`
	BakedResource  string `json:"baked_resource"`
	SHA256         string `json:"sha256"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	CellCount      int    `json:"cell_count"`
	SurfaceCount   int    `json:"surface_count"`
	TextureCount   int    `json:"texture_count"`
	LightmapCount  int    `json:"lightmap_count"`
}

type terrainBake struct {
	Format    string           `json:"format"`
	Version   int              `json:"version"`
	Map       string           `json:"map"`
	SourceGND string           `json:"source_gnd"`
	Width     int              `json:"width"`
	Height    int              `json:"height"`
	Zoom      float32          `json:"zoom"`
	Water     res.GNDWater     `json:"water"`
	Textures  []string         `json:"textures"`
	Surfaces  []terrainSurface `json:"surfaces"`
	Cells     []terrainCell    `json:"cells"`
	Lightmaps int              `json:"lightmap_count"`
}

type terrainSurface struct {
	U          [4]float32 `json:"u"`
	V          [4]float32 `json:"v"`
	TextureID  int        `json:"texture_id"`
	LightmapID int        `json:"lightmap_id"`
	Color      [4]uint8   `json:"color"`
}

type terrainCell struct {
	Heights [4]float32 `json:"heights"`
	Top     int        `json:"top"`
	Front   int        `json:"front"`
	Right   int        `json:"right"`
}

func (b *builder) bakeTerrainFile() error {
	if b.parsedGND == nil {
		return fmt.Errorf("terrain bake requires %s.gnd", b.root)
	}
	gnd := b.parsedGND
	baked := terrainBake{
		Format: "goro-mobile-terrain-bake", Version: 1, Map: b.root,
		SourceGND: "data/" + b.root + ".gnd", Width: gnd.Width, Height: gnd.Height,
		Zoom: gnd.Zoom, Water: gnd.Water, Textures: append([]string(nil), gnd.Textures...),
		Surfaces: make([]terrainSurface, len(gnd.Surfaces)), Cells: make([]terrainCell, len(gnd.Cells)), Lightmaps: len(gnd.Lightmaps),
	}
	for index, surface := range gnd.Surfaces {
		baked.Surfaces[index] = terrainSurface{U: surface.U, V: surface.V, TextureID: surface.TextureID, LightmapID: surface.LightmapID, Color: [4]uint8{surface.Color.R, surface.Color.G, surface.Color.B, surface.Color.A}}
	}
	for index, cell := range gnd.Cells {
		baked.Cells[index] = terrainCell{Heights: cell.Heights, Top: cell.Top, Front: cell.Front, Right: cell.Right}
	}
	data, err := json.MarshalIndent(baked, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	resource := "mobile/terrain/" + b.root + ".json"
	b.files[resource] = data
	b.included[resource] = struct{}{}
	b.recordCategory(resource, int64(len(data)))
	hash := sha256.Sum256(data)
	b.terrainBake = &TerrainBakeInfo{
		Format: "goro-mobile-terrain-bake", Version: 1, SourceResource: baked.SourceGND, BakedResource: resource,
		SHA256: hex.EncodeToString(hash[:]), Width: gnd.Width, Height: gnd.Height, CellCount: len(gnd.Cells),
		SurfaceCount: len(gnd.Surfaces), TextureCount: len(gnd.Textures), LightmapCount: len(gnd.Lightmaps),
	}
	return nil
}
