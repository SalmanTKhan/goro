package mobilepack

import (
	"encoding/json"
	"fmt"
	"os"
)

// RuntimeMetrics is the stable, renderer-facing subset emitted by the
// Android host. Keeping it in the pack package lets a device result be merged
// into the same manifest that records source closure and GND diagnostics.
type RuntimeMetrics struct {
	Format                     string   `json:"format"`
	Version                    int      `json:"version"`
	Map                        string   `json:"map"`
	MapLoadMS                  float64  `json:"map_load_ms"`
	TimeToFirstMapFrameMS      float64  `json:"time_to_first_map_frame_ms"`
	SteadyFPS                  float64  `json:"steady_fps"`
	AverageCPUFrameMS          float64  `json:"average_cpu_frame_ms"`
	PeakProcessRSSBytes        int64    `json:"peak_process_rss_bytes"`
	SteadyProcessRSSBytes      int64    `json:"steady_process_rss_bytes"`
	TerrainBuildMS             float64  `json:"terrain_build_ms"`
	TerrainChunkBuilds         int      `json:"terrain_chunk_builds"`
	TerrainTextureFallbacks    int      `json:"terrain_texture_fallbacks"`
	RSMTextureFallbacks        int      `json:"rsm_texture_fallbacks"`
	RSMEmptyTextureFallbacks   int      `json:"rsm_empty_texture_fallbacks"`
	RSMTextureFallbackExamples []string `json:"rsm_texture_fallback_examples,omitempty"`
	TextureDecodeMS            float64  `json:"texture_decode_ms"`
	TextureDecodeCount         int      `json:"texture_decode_count"`
	TextureEncodedBytes        int64    `json:"texture_encoded_bytes"`
	TextureDecodedRGBABytes    int64    `json:"texture_decoded_rgba_bytes"`
	TextureUploadMS            float64  `json:"texture_upload_ms"`
	TextureUploadCount         int      `json:"texture_upload_count"`
	TextureUploadedBytes       int64    `json:"texture_uploaded_bytes"`
	EstimatedTextureGPUBytes   int64    `json:"estimated_texture_gpu_bytes"`
	LastFrameAt                string   `json:"last_frame_at"`
}

// MergeRuntimeMetrics attaches a device run to an existing pack manifest.
// It overwrites only the runtime_metrics field and leaves the deterministic
// source/asset measurements intact.
func MergeRuntimeMetrics(manifestPath, runtimePath string) error {
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("read mobile pack manifest: %w", err)
	}
	runtimeData, err := os.ReadFile(runtimePath)
	if err != nil {
		return err
	}
	var metrics RuntimeMetrics
	if err := json.Unmarshal(runtimeData, &metrics); err != nil {
		return fmt.Errorf("read runtime metrics: %w", err)
	}
	manifest.RuntimeMetrics = &metrics
	merged, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, append(merged, '\n'), 0o644)
}
