package res

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"math"
)

type GND struct {
	VersionMajor byte
	VersionMinor byte
	Width        int
	Height       int
	Zoom         float32
	Water        GNDWater
	Textures     []string
	Lightmaps    []GNDLightmap
	Surfaces     []GNDSurface
	Cells        []GNDCell
}

type GNDWater struct {
	Present     bool
	Level       float32
	Type        int32
	WaveHeight  float32
	WaveSpeed   float32
	WavePitch   float32
	AnimSpeed   int32
	SplitWidth  int32
	SplitHeight int32
}

type GNDLightmap struct {
	Alpha [8][8]uint8
	Color [8][8]color.RGBA
}

type GNDSurface struct {
	U          [4]float32
	V          [4]float32
	TextureID  int
	LightmapID int
	Color      color.RGBA
}

type GNDCell struct {
	Heights [4]float32
	Top     int
	Front   int
	Right   int
}

func ParseGND(data []byte) (*GND, error) {
	reader := gndReader{data: data}
	if string(reader.bytes(4)) != "GRGN" {
		return nil, fmt.Errorf("invalid gnd signature")
	}

	major := reader.u8()
	minor := reader.u8()
	width := int(reader.i32())
	height := int(reader.i32())
	zoom := reader.f32()
	if reader.err != nil {
		return nil, reader.err
	}
	if major != 1 || minor <= 6 {
		return nil, fmt.Errorf("unsupported gnd version %d.%d", major, minor)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid gnd size %dx%d", width, height)
	}

	textureCount := int(reader.i32())
	textureNameLength := int(reader.i32())
	if textureCount < 0 || textureNameLength <= 0 {
		return nil, fmt.Errorf("invalid gnd texture table count=%d length=%d", textureCount, textureNameLength)
	}
	textures := make([]string, textureCount)
	for i := range textures {
		textures[i] = fixedResourceString(reader.bytes(textureNameLength))
	}

	lightmapCount := int(reader.i32())
	lightmapWidth := int(reader.i32())
	lightmapHeight := int(reader.i32())
	lightmapFormat := int(reader.i32())
	if lightmapCount < 0 {
		return nil, fmt.Errorf("invalid gnd lightmap count=%d", lightmapCount)
	}
	if lightmapWidth <= 0 || lightmapHeight <= 0 || lightmapFormat <= 0 {
		return nil, fmt.Errorf("invalid gnd lightmap header count=%d size=%dx%d format=%d", lightmapCount, lightmapWidth, lightmapHeight, lightmapFormat)
	}
	lightmaps := make([]GNDLightmap, lightmapCount)
	if minor == 7 {
		for i := range lightmaps {
			lightmaps[i] = decodeGNDLightmapRaw(reader.bytes(256))
		}
	} else {
		indexes := make([][4]int, lightmapCount)
		for i := range indexes {
			indexes[i] = [4]int{int(reader.i32()), int(reader.i32()), int(reader.i32()), int(reader.i32())}
		}
		colorChannelCount := int(reader.i32())
		if colorChannelCount < 0 {
			return nil, fmt.Errorf("invalid gnd color channel count=%d", colorChannelCount)
		}
		colorChannels := make([][40]byte, colorChannelCount)
		for i := range colorChannels {
			copy(colorChannels[i][:], reader.bytes(40))
		}
		for i, index := range indexes {
			lightmaps[i] = decodeGNDLightmapIndexed(index, colorChannels)
		}
	}

	surfaceCount := int(reader.i32())
	if surfaceCount < 0 {
		return nil, fmt.Errorf("invalid gnd surface count=%d", surfaceCount)
	}
	surfaces := make([]GNDSurface, surfaceCount)
	for i := range surfaces {
		var surface GNDSurface
		for j := range surface.U {
			surface.U[j] = reader.f32()
		}
		for j := range surface.V {
			surface.V[j] = reader.f32()
		}
		surface.TextureID = int(reader.u16())
		surface.LightmapID = int(reader.u16())
		b := reader.u8()
		g := reader.u8()
		r := reader.u8()
		a := reader.u8()
		surface.Color = color.RGBA{R: r, G: g, B: b, A: a}
		surfaces[i] = surface
	}

	cellCount := width * height
	cells := make([]GNDCell, cellCount)
	for i := range cells {
		var cell GNDCell
		for j := range cell.Heights {
			cell.Heights[j] = -reader.f32() * 0.2
		}
		cell.Top = int(reader.i32())
		cell.Front = int(reader.i32())
		cell.Right = int(reader.i32())
		cells[i] = cell
	}
	if reader.err != nil {
		return nil, reader.err
	}
	var water GNDWater
	if minor >= 8 && reader.remaining() >= 32 {
		water = GNDWater{
			Present:     true,
			Level:       -reader.f32() / 5,
			Type:        reader.i32(),
			WaveHeight:  reader.f32() / 5,
			WaveSpeed:   reader.f32(),
			WavePitch:   reader.f32(),
			AnimSpeed:   reader.i32(),
			SplitWidth:  reader.i32(),
			SplitHeight: reader.i32(),
		}
		if minor >= 9 && water.SplitWidth > 0 && water.SplitHeight > 0 {
			reader.skip(int(water.SplitWidth * water.SplitHeight * 24))
		}
	}

	return &GND{
		VersionMajor: major,
		VersionMinor: minor,
		Width:        width,
		Height:       height,
		Zoom:         zoom,
		Water:        water,
		Textures:     textures,
		Lightmaps:    lightmaps,
		Surfaces:     surfaces,
		Cells:        cells,
	}, nil
}

func (g *GND) InBounds(x, y int) bool {
	return g != nil && x >= 0 && y >= 0 && x < g.Width && y < g.Height
}

func (g *GND) Cell(x, y int) (GNDCell, bool) {
	if !g.InBounds(x, y) {
		return GNDCell{}, false
	}
	return g.Cells[y*g.Width+x], true
}

func (g *GND) Surface(index int) (GNDSurface, bool) {
	if g == nil || index < 0 || index >= len(g.Surfaces) {
		return GNDSurface{}, false
	}
	return g.Surfaces[index], true
}

func (g *GND) Lightmap(index int) (GNDLightmap, bool) {
	if g == nil || index < 0 || index >= len(g.Lightmaps) {
		return GNDLightmap{}, false
	}
	return g.Lightmaps[index], true
}

func decodeGNDLightmapRaw(data []byte) GNDLightmap {
	var lightmap GNDLightmap
	if len(data) < 256 {
		return lightmap
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pixel := y*8 + x
			lightmap.Alpha[y][x] = data[pixel]
			base := 64 + pixel*3
			lightmap.Color[y][x] = color.RGBA{R: data[base], G: data[base+1], B: data[base+2], A: 255}
		}
	}
	return lightmap
}

func decodeGNDLightmapIndexed(index [4]int, channels [][40]byte) GNDLightmap {
	var lightmap GNDLightmap
	if len(channels) == 0 {
		return lightmap
	}
	for _, channelID := range index {
		if channelID < 0 || channelID >= len(channels) {
			return lightmap
		}
	}
	a := unpackGNDLightmap5BitChannel(channels[index[0]])
	r := unpackGNDLightmap5BitChannel(channels[index[1]])
	g := unpackGNDLightmap5BitChannel(channels[index[2]])
	b := unpackGNDLightmap5BitChannel(channels[index[3]])
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			src := x*8 + y
			lightmap.Alpha[y][x] = a[src]
			lightmap.Color[y][x] = color.RGBA{R: r[src], G: g[src], B: b[src], A: 255}
		}
	}
	return lightmap
}

func unpackGNDLightmap5BitChannel(channel [40]byte) [64]uint8 {
	var out [64]uint8
	for i := range out {
		bitOffset := i * 5
		byteOffset := bitOffset / 8
		bitShift := bitOffset % 8
		if bitShift >= 4 {
			out[i] = ((channel[byteOffset] & byte(0xf8>>bitShift)) << bitShift) |
				((channel[byteOffset+1] & byte(0xf8<<(8-bitShift))) >> (8 - bitShift))
		} else {
			out[i] = (channel[byteOffset] & byte(0xf8>>bitShift)) << bitShift
		}
	}
	return out
}

func fixedBinaryString(data []byte) string {
	if index := bytes.IndexByte(data, 0); index >= 0 {
		data = data[:index]
	}
	for len(data) > 0 && isASCIIWhitespace(data[0]) {
		data = data[1:]
	}
	for len(data) > 0 && isASCIIWhitespace(data[len(data)-1]) {
		data = data[:len(data)-1]
	}
	return string(data)
}

func isASCIIWhitespace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

type gndReader struct {
	data   []byte
	offset int
	err    error
}

func (r *gndReader) bytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.offset+n > len(r.data) {
		r.err = fmt.Errorf("gnd truncated at offset %d reading %d bytes", r.offset, n)
		return nil
	}
	out := r.data[r.offset : r.offset+n]
	r.offset += n
	return out
}

func (r *gndReader) skip(n int) {
	_ = r.bytes(n)
}

func (r *gndReader) remaining() int {
	if r.err != nil || r.offset >= len(r.data) {
		return 0
	}
	return len(r.data) - r.offset
}

func (r *gndReader) u8() byte {
	data := r.bytes(1)
	if len(data) == 0 {
		return 0
	}
	return data[0]
}

func (r *gndReader) u16() uint16 {
	data := r.bytes(2)
	if len(data) < 2 {
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}

func (r *gndReader) i32() int32 {
	data := r.bytes(4)
	if len(data) < 4 {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(data))
}

func (r *gndReader) f32() float32 {
	data := r.bytes(4)
	if len(data) < 4 {
		return 0
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(data))
}
