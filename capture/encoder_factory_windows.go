//go:build windows

package capture

import "fmt"

// NewEncoder selects the platform-native encoder for the default container
// and the explicitly configured FFmpeg fallback for WebM.
func NewEncoder(options RecordingOptions, ffmpegPath string) (Encoder, error) {
	options, err := options.Normalized()
	if err != nil {
		return nil, err
	}
	switch options.Container {
	case RecordingMP4:
		return NewMediaFoundationEncoder(), nil
	case RecordingWebM:
		if ffmpegPath == "" {
			return nil, fmt.Errorf("WebM recording requires capture.ffmpeg_path or --ffmpeg-path")
		}
		return NewFFmpegEncoder(ffmpegPath), nil
	default:
		return nil, fmt.Errorf("unsupported recording container %q", options.Container)
	}
}
