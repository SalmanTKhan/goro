// Package capture contains renderer-independent image and video capture
// contracts. Rendering code owns GPU readback; this package owns the bounded
// handoff to still-image and video encoders.
package capture

import (
	"errors"
	"fmt"
	"time"
)

type StillFormat string

const (
	StillPNG  StillFormat = "png"
	StillWebP StillFormat = "webp"
)

type PixelFormat string

const (
	PixelFormatRGBA8 PixelFormat = "rgba8"
	PixelFormatBGRA8 PixelFormat = "bgra8"
)

type RecordingContainer string

const (
	RecordingMP4  RecordingContainer = "mp4"
	RecordingWebM RecordingContainer = "webm"
)

type RecordingCodec string

const (
	RecordingH264 RecordingCodec = "h264"
	RecordingVP9  RecordingCodec = "vp9"
)

type ScreenshotOptions struct {
	Format   StillFormat
	Quality  int
	Lossless bool
}

type RecordingOptions struct {
	FPS       int
	Container RecordingContainer
	Codec     RecordingCodec
	Path      string
}

type Frame struct {
	Width, Height int
	Stride        int
	PixelFormat   PixelFormat
	PTS           time.Duration
	Pixels        []byte
}

type Encoder interface {
	Start(RecordingOptions) error
	Write(Frame) error
	Close() error
}

var (
	ErrClosed       = errors.New("capture: encoder is closed")
	ErrQueueClosed  = errors.New("capture: frame queue is closed")
	ErrInvalidFrame = errors.New("capture: invalid frame")
)

func (f StillFormat) Valid() bool {
	return f == StillPNG || f == StillWebP
}

func (c RecordingContainer) Valid() bool {
	return c == RecordingMP4 || c == RecordingWebM
}

func (c RecordingCodec) Valid() bool {
	return c == RecordingH264 || c == RecordingVP9
}

func (o ScreenshotOptions) Normalized() (ScreenshotOptions, error) {
	if o.Format == "" {
		o.Format = StillPNG
	}
	if !o.Format.Valid() {
		return ScreenshotOptions{}, fmt.Errorf("unsupported still format %q", o.Format)
	}
	if o.Quality == 0 {
		o.Quality = 90
	}
	if o.Format == StillPNG && o.Lossless {
		return ScreenshotOptions{}, fmt.Errorf("PNG does not accept a WebP lossless option")
	}
	if o.Format == StillWebP && (o.Quality < 0 || o.Quality > 100) {
		return ScreenshotOptions{}, fmt.Errorf("WebP quality must be between 0 and 100")
	}
	return o, nil
}

func (o RecordingOptions) Normalized() (RecordingOptions, error) {
	if o.FPS == 0 {
		o.FPS = 30
	}
	if o.Container == "" {
		o.Container = RecordingMP4
	}
	if o.Codec == "" {
		if o.Container == RecordingWebM {
			o.Codec = RecordingVP9
		} else {
			o.Codec = RecordingH264
		}
	}
	if o.FPS != 30 && o.FPS != 60 {
		return RecordingOptions{}, fmt.Errorf("recording FPS must be 30 or 60")
	}
	if !o.Container.Valid() {
		return RecordingOptions{}, fmt.Errorf("unsupported recording container %q", o.Container)
	}
	if !o.Codec.Valid() {
		return RecordingOptions{}, fmt.Errorf("unsupported recording codec %q", o.Codec)
	}
	if o.Container == RecordingMP4 && o.Codec != RecordingH264 {
		return RecordingOptions{}, fmt.Errorf("MP4 recording requires H.264")
	}
	if o.Container == RecordingWebM && o.Codec != RecordingVP9 {
		return RecordingOptions{}, fmt.Errorf("WebM recording requires VP9")
	}
	return o, nil
}

func (f Frame) Validate() error {
	if f.Width <= 0 || f.Height <= 0 || f.Stride < f.Width*4 {
		return fmt.Errorf("%w: dimensions=%dx%d stride=%d", ErrInvalidFrame, f.Width, f.Height, f.Stride)
	}
	if f.PixelFormat != PixelFormatRGBA8 && f.PixelFormat != PixelFormatBGRA8 {
		return fmt.Errorf("%w: unsupported pixel format %q", ErrInvalidFrame, f.PixelFormat)
	}
	if len(f.Pixels) < f.Stride*f.Height {
		return fmt.Errorf("%w: pixel data is %d bytes, need %d", ErrInvalidFrame, len(f.Pixels), f.Stride*f.Height)
	}
	return nil
}

func (f Frame) Clone() (Frame, error) {
	if err := f.Validate(); err != nil {
		return Frame{}, err
	}
	clone := f
	clone.Pixels = make([]byte, f.Stride*f.Height)
	copy(clone.Pixels, f.Pixels[:f.Stride*f.Height])
	return clone, nil
}
