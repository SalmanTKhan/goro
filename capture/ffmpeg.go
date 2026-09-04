package capture

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// FFmpegEncoder is the optional WebM/VP9 fallback. It starts lazily on the
// first frame because the raw-video dimensions are part of the FFmpeg input
// contract while RecordingOptions intentionally stays renderer-neutral.
type FFmpegEncoder struct {
	binary  string
	options RecordingOptions
	cmd     *exec.Cmd
	pipe    io.WriteCloser
	started bool
	mu      sync.Mutex
}

func NewFFmpegEncoder(binary string) *FFmpegEncoder {
	return &FFmpegEncoder{binary: binary}
}

func (e *FFmpegEncoder) Start(options RecordingOptions) error {
	options, err := options.Normalized()
	if err != nil {
		return err
	}
	if options.Container != RecordingWebM || options.Codec != RecordingVP9 {
		return fmt.Errorf("FFmpeg encoder only supports WebM/VP9")
	}
	if e.binary == "" {
		return fmt.Errorf("FFmpeg path is empty")
	}
	if strings.TrimSpace(options.Path) == "" {
		return fmt.Errorf("recording path is required")
	}
	if _, err := exec.LookPath(e.binary); err != nil {
		return fmt.Errorf("FFmpeg unavailable: %w", err)
	}
	if err := ensureOutputDirectory(options.Path); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return ErrClosed
	}
	e.options = options
	e.started = true
	return nil
}

func (e *FFmpegEncoder) Write(frame Frame) error {
	if e == nil {
		return ErrClosed
	}
	if err := frame.Validate(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started || e.options.Path == "" {
		return fmt.Errorf("FFmpeg encoder was not started")
	}
	if e.pipe == nil {
		return e.startProcess(frame)
	}
	return writePackedBGRA(e.pipe, frame)
}

func (e *FFmpegEncoder) startProcess(frame Frame) error {
	e.cmd = exec.Command(e.binary,
		"-f", "rawvideo",
		"-pixel_format", "bgra",
		"-video_size", fmt.Sprintf("%dx%d", frame.Width, frame.Height),
		"-framerate", fmt.Sprintf("%d", e.options.FPS),
		"-i", "pipe:0",
		"-an",
		"-c:v", "libvpx-vp9",
		"-deadline", "realtime",
		"-cpu-used", "6",
		"-row-mt", "1",
		"-crf", "30",
		"-b:v", "0",
		"-f", "webm",
		e.options.Path,
	)
	e.cmd.Stderr = io.Discard
	pipe, err := e.cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := e.cmd.Start(); err != nil {
		_ = pipe.Close()
		e.cmd = nil
		return err
	}
	e.pipe = pipe
	return writePackedBGRA(e.pipe, frame)
}

func (e *FFmpegEncoder) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd == nil {
		e.started = false
		return nil
	}
	closeErr := e.pipe.Close()
	err := e.cmd.Wait()
	e.cmd, e.pipe, e.started = nil, nil, false
	if closeErr != nil {
		return closeErr
	}
	return err
}

func ensureOutputDirectory(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func writePackedBGRA(w io.Writer, frame Frame) error {
	if frame.PixelFormat == PixelFormatBGRA8 && frame.Stride == frame.Width*4 {
		_, err := w.Write(frame.Pixels[:frame.Stride*frame.Height])
		return err
	}
	row := frame.Width * 4
	packed := make([]byte, row)
	for y := 0; y < frame.Height; y++ {
		src := frame.Pixels[y*frame.Stride : y*frame.Stride+row]
		if frame.PixelFormat == PixelFormatRGBA8 {
			for i := 0; i < row; i += 4 {
				packed[i], packed[i+1], packed[i+2], packed[i+3] = src[i+2], src[i+1], src[i], src[i+3]
			}
		} else {
			copy(packed, src)
		}
		if _, err := w.Write(packed); err != nil {
			return err
		}
	}
	return nil
}
