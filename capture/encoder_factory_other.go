//go:build !windows

package capture

import "fmt"

func NewEncoder(options RecordingOptions, ffmpegPath string) (Encoder, error) {
	return nil, fmt.Errorf("video recording is currently supported on Windows only")
}
