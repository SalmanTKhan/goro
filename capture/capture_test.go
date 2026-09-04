package capture

import (
	"bytes"
	"image"
	"image/color"
	"sync"
	"testing"
	"time"
)

func TestImageConvertsPaddedBGRA(t *testing.T) {
	frame := Frame{
		Width: 2, Height: 2, Stride: 12, PixelFormat: PixelFormatBGRA8,
		Pixels: []byte{
			1, 2, 3, 4, 5, 6, 7, 8, 99, 99, 99, 99,
			9, 10, 11, 12, 13, 14, 15, 16, 88, 88, 88, 88,
		},
	}
	img, err := Image(frame)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{3, 2, 1, 4, 7, 6, 5, 8, 11, 10, 9, 12, 15, 14, 13, 16}
	if !bytes.Equal(img.Pix, want) {
		t.Fatalf("pixels = %v, want %v", img.Pix, want)
	}
}

func TestOptionsNormalizeDefaultsAndRejectMismatches(t *testing.T) {
	still, err := (ScreenshotOptions{Format: StillWebP}).Normalized()
	if err != nil {
		t.Fatal(err)
	}
	if still.Quality != 90 || still.Lossless {
		t.Fatalf("normalized still options = %#v", still)
	}

	recording, err := (RecordingOptions{Path: "capture.mp4"}).Normalized()
	if err != nil {
		t.Fatal(err)
	}
	if recording.FPS != 30 || recording.Container != RecordingMP4 || recording.Codec != RecordingH264 {
		t.Fatalf("normalized recording options = %#v", recording)
	}
	if _, err := (RecordingOptions{FPS: 30, Container: RecordingWebM, Codec: RecordingH264, Path: "capture.webm"}).Normalized(); err == nil {
		t.Fatal("expected WebM/H.264 mismatch to be rejected")
	}
}

func TestWritePackedBGRAConvertsRowsAndPadding(t *testing.T) {
	frame := Frame{
		Width: 2, Height: 2, Stride: 12, PixelFormat: PixelFormatRGBA8,
		Pixels: []byte{
			1, 2, 3, 4, 5, 6, 7, 8, 99, 99, 99, 99,
			9, 10, 11, 12, 13, 14, 15, 16, 88, 88, 88, 88,
		},
	}
	var packed bytes.Buffer
	if err := writePackedBGRA(&packed, frame); err != nil {
		t.Fatal(err)
	}
	want := []byte{3, 2, 1, 4, 7, 6, 5, 8, 11, 10, 9, 12, 15, 14, 13, 16}
	if !bytes.Equal(packed.Bytes(), want) {
		t.Fatalf("packed pixels = %v, want %v", packed.Bytes(), want)
	}
}

func TestStillRoundTripsPNGAndWebP(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 70), B: 120, A: 255})
		}
	}
	frame := Frame{Width: 3, Height: 2, Stride: 12, PixelFormat: PixelFormatRGBA8, Pixels: img.Pix}
	for _, test := range []struct {
		name    string
		options ScreenshotOptions
	}{
		{name: "png", options: ScreenshotOptions{Format: StillPNG}},
		{name: "webp", options: ScreenshotOptions{Format: StillWebP, Quality: 90}},
		{name: "webp-lossless", options: ScreenshotOptions{Format: StillWebP, Lossless: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var encoded bytes.Buffer
			if err := EncodeStillTo(&encoded, frame, test.options); err != nil {
				t.Fatal(err)
			}
			if len(encoded.Bytes()) < 12 {
				t.Fatalf("encoded output is too short: %d", encoded.Len())
			}
			decoded, _, err := image.Decode(bytes.NewReader(encoded.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if got := decoded.Bounds().Size(); got != image.Pt(3, 2) {
				t.Fatalf("decoded dimensions = %v, want 3x2", got)
			}
		})
	}
}

func TestFrameQueueDropsWithoutBlocking(t *testing.T) {
	q := NewFrameQueue(1)
	frame := Frame{Width: 1, Height: 1, Stride: 4, PixelFormat: PixelFormatRGBA8, Pixels: []byte{1, 2, 3, 4}}
	if !q.Submit(frame) {
		t.Fatal("first frame was not accepted")
	}
	if q.Submit(frame) {
		t.Fatal("second frame unexpectedly accepted")
	}
	if got := q.Dropped(); got != 1 {
		t.Fatalf("dropped = %d, want 1", got)
	}
	q.Close()
	if q.Submit(frame) {
		t.Fatal("frame accepted after close")
	}
}

type testEncoder struct {
	mu      sync.Mutex
	started bool
	writes  int
	closed  bool
}

func (e *testEncoder) Start(RecordingOptions) error { e.started = true; return nil }
func (e *testEncoder) Write(Frame) error            { e.mu.Lock(); e.writes++; e.mu.Unlock(); return nil }
func (e *testEncoder) Close() error                 { e.closed = true; return nil }

func TestAsyncRecorderDrainsBeforeClose(t *testing.T) {
	encoder := &testEncoder{}
	recorder := NewAsyncRecorder(encoder, 2)
	options := RecordingOptions{FPS: 30, Container: RecordingMP4, Codec: RecordingH264, Path: "capture.mp4"}
	if err := recorder.Start(options); err != nil {
		t.Fatal(err)
	}
	frame := Frame{Width: 1, Height: 1, Stride: 4, PixelFormat: PixelFormatRGBA8, Pixels: []byte{1, 2, 3, 4}, PTS: time.Second}
	if !recorder.Submit(frame) {
		t.Fatal("frame was not accepted")
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	encoder.mu.Lock()
	writes, closed := encoder.writes, encoder.closed
	encoder.mu.Unlock()
	if writes != 1 || !closed {
		t.Fatalf("writes=%d closed=%t, want one write and close", writes, closed)
	}
}
