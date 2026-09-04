package capture

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/deepteams/webp"
)

// EncodeStill writes one frame to path. The encoder accepts padded GPU rows
// and normalizes both RGBA8 and BGRA8 into image.RGBA before encoding.
func EncodeStill(path string, frame Frame, options ScreenshotOptions) error {
	options, err := options.Normalized()
	if err != nil {
		return err
	}
	img, err := Image(frame)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := encodeStill(file, img, options); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func EncodeStillTo(w io.Writer, frame Frame, options ScreenshotOptions) error {
	options, err := options.Normalized()
	if err != nil {
		return err
	}
	img, err := Image(frame)
	if err != nil {
		return err
	}
	return encodeStill(w, img, options)
}

func encodeStill(w io.Writer, img image.Image, options ScreenshotOptions) error {
	switch options.Format {
	case StillPNG:
		return png.Encode(w, img)
	case StillWebP:
		return webp.Encode(w, img, &webp.EncoderOptions{
			Quality:  float32(options.Quality),
			Method:   4,
			Lossless: options.Lossless,
		})
	default:
		return fmt.Errorf("unsupported still format %q", options.Format)
	}
}

// Image converts a capture frame into a tightly packed RGBA image. The
// conversion deliberately happens after GPU readback so the readback buffer
// can retain the API-native row padding.
func Image(frame Frame) (*image.RGBA, error) {
	if err := frame.Validate(); err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, frame.Width, frame.Height))
	for y := 0; y < frame.Height; y++ {
		src := frame.Pixels[y*frame.Stride:]
		dst := img.Pix[y*img.Stride:]
		for x := 0; x < frame.Width; x++ {
			si := x * 4
			di := x * 4
			switch frame.PixelFormat {
			case PixelFormatRGBA8:
				dst[di], dst[di+1], dst[di+2], dst[di+3] = src[si], src[si+1], src[si+2], src[si+3]
			case PixelFormatBGRA8:
				dst[di], dst[di+1], dst[di+2], dst[di+3] = src[si+2], src[si+1], src[si], src[si+3]
			}
		}
	}
	return img, nil
}
