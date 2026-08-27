package mobilepack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kivutar/goro/res"
)

type BuildOptions struct {
	OptimizeTextures bool
	BakeTerrain      bool
	OfflineContent   string
}

type TextureOptimizationMetric struct {
	SourceResource        string `json:"source_resource"`
	SourceSHA256          string `json:"source_sha256"`
	OriginalEncodedBytes  int64  `json:"original_encoded_bytes"`
	DecodedRGBABytes      int64  `json:"decoded_rgba_bytes"`
	OptimizedResource     string `json:"optimized_resource"`
	OptimizedEncodedBytes int64  `json:"optimized_encoded_bytes"`
	EstimatedGPUBytes     int64  `json:"estimated_gpu_bytes"`
	Format                string `json:"format"`
	Width                 int    `json:"width"`
	Height                int    `json:"height"`
	MipLevels             int    `json:"mip_levels"`
}

type TextureOptimizationManifest struct {
	Format   string                      `json:"format"`
	Version  int                         `json:"version"`
	Fallback string                      `json:"fallback"`
	Textures []TextureOptimizationMetric `json:"textures"`
}

func (b *builder) optimizeTextureFiles() error {
	names := make([]string, 0, len(b.files))
	for name := range b.files {
		ext := strings.ToLower(filepath.Ext(name))
		if !strings.HasPrefix(strings.ToLower(name), "data/texture/") {
			continue
		}
		if ext != ".bmp" && ext != ".tga" && ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	metrics := make([]TextureOptimizationMetric, 0, len(names))
	for _, name := range names {
		data := b.files[name]
		decoded, err := res.DecodeImageData(data)
		if err != nil {
			return fmt.Errorf("optimize texture %s: %w", name, err)
		}
		var optimized bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&optimized, decoded); err != nil {
			return fmt.Errorf("encode optimized texture %s: %w", name, err)
		}
		hash := sha256.Sum256(data)
		bounds := decoded.Bounds()
		optimizedName := "mobile/optimized/" + name + ".png"
		optimizedData := optimized.Bytes()
		b.files[optimizedName] = append([]byte(nil), optimizedData...)
		b.included[optimizedName] = struct{}{}
		b.recordCategory(optimizedName, int64(len(optimizedData)))
		metrics = append(metrics, TextureOptimizationMetric{
			SourceResource: name, SourceSHA256: hex.EncodeToString(hash[:]),
			OriginalEncodedBytes: int64(len(data)), DecodedRGBABytes: int64(bounds.Dx()) * int64(bounds.Dy()) * 4,
			OptimizedResource: optimizedName, OptimizedEncodedBytes: int64(len(optimizedData)),
			EstimatedGPUBytes: int64(bounds.Dx()) * int64(bounds.Dy()) * 4,
			Format:            "png-zlib", Width: bounds.Dx(), Height: bounds.Dy(), MipLevels: 1,
		})
	}
	manifest := TextureOptimizationManifest{Format: "goro-mobile-texture-optimization", Version: 1, Fallback: "source-decoded-rgba8", Textures: metrics}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestName := "mobile/optimized/manifest.json"
	b.files[manifestName] = append(manifestData, '\n')
	b.included[manifestName] = struct{}{}
	b.recordCategory(manifestName, int64(len(b.files[manifestName])))
	b.textureMetrics = metrics
	return nil
}
