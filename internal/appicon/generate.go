//go:build ignore

// Command generate derives platform icon files from icon.png.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	sourceFile, err := os.Open("icon.png")
	check(err)
	defer sourceFile.Close()

	source, err := png.Decode(sourceFile)
	check(err)
	if bounds := source.Bounds(); bounds.Dx() != bounds.Dy() {
		check(fmt.Errorf("source icon must be square, got %dx%d", bounds.Dx(), bounds.Dy()))
	}

	// Android launcher densities: mdpi 48, hdpi 72, xhdpi 96, xxhdpi 144, xxxhdpi 192.
	androidBuckets := map[string]int{
		"mdpi":    48,
		"hdpi":    72,
		"xhdpi":   96,
		"xxhdpi":  144,
		"xxxhdpi": 192,
	}

	frames := makeFrames(source, []int{16, 32, 48, 64, 72, 96, 144, 192, 256})
	projectRoot := filepath.Join("..", "..")
	writeFile(filepath.Join(projectRoot, "packaging", "windows", "goro.ico"), encodeICO(frames, []int{16, 32, 48, 64, 256}))
	writeFile(filepath.Join(projectRoot, "packaging", "linux", "goro.png"), frames[256])

	androidRes := filepath.Join(projectRoot, "android", "host", "app", "src", "main", "res")
	for density, size := range androidBuckets {
		writeFile(filepath.Join(androidRes, "mipmap-"+density, "ic_launcher.png"), frames[size])
	}
}

func makeFrames(source image.Image, sizes []int) map[int][]byte {
	frames := make(map[int][]byte, len(sizes))
	for _, size := range sizes {
		var output bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		check(encoder.Encode(&output, resizeNearest(source, size)))
		frames[size] = output.Bytes()
	}
	return frames
}

func resizeNearest(source image.Image, size int) *image.NRGBA {
	sourceBounds := source.Bounds()
	destination := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		sourceY := sourceBounds.Min.Y + y*sourceBounds.Dy()/size
		for x := range size {
			sourceX := sourceBounds.Min.X + x*sourceBounds.Dx()/size
			destination.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return destination
}

func encodeICO(frames map[int][]byte, sizes []int) []byte {
	var output bytes.Buffer
	writeLE(&output, uint16(0))
	writeLE(&output, uint16(1))
	writeLE(&output, uint16(len(sizes)))

	offset := uint32(6 + 16*len(sizes))
	for _, size := range sizes {
		data := frames[size]
		dimension := byte(size)
		if size == 256 {
			dimension = 0
		}
		output.WriteByte(dimension)
		output.WriteByte(dimension)
		output.WriteByte(0)
		output.WriteByte(0)
		writeLE(&output, uint16(1))
		writeLE(&output, uint16(32))
		writeLE(&output, uint32(len(data)))
		writeLE(&output, offset)
		offset += uint32(len(data))
	}
	for _, size := range sizes {
		output.Write(frames[size])
	}
	return output.Bytes()
}

func writeFile(path string, data []byte) {
	check(os.MkdirAll(filepath.Dir(path), 0o755))
	check(os.WriteFile(path, data, 0o644))
}

func writeLE(output *bytes.Buffer, value any) {
	check(binary.Write(output, binary.LittleEndian, value))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
