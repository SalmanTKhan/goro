package ui

import (
	"fmt"
	"image"
	"strings"

	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/ui/inputprompt"
	"github.com/kivutar/goro/ui/rotheme"
	"golang.org/x/image/draw"
)

func controllerPromptSignature(ctx client.Context) string {
	if ctx.Input == nil {
		return "none"
	}
	s := ctx.Input.Controller()
	settings := ctx.ControllerSettings()
	return fmt.Sprintf("%d/%t/%d/%d/%d/%d", ctx.Input.InputSource(), s.Connected, s.Kind, settings.Bindings.Confirm, settings.Bindings.Cancel, settings.Bindings.Attack)
}

// controllerPromptFooter returns a compact, context-aware fallback for a
// window footer. The resolver still owns family and binding selection; keeping
// this helper in ui makes every surface use the same visibility rule.
func controllerPromptFooter(ctx client.Context, actions ...input.Action) widget.Widget {
	if ctx.Input == nil || ctx.Input.InputSource() != input.InputSourceController || !ctx.Input.Controller().Connected {
		return primitives.Box()
	}
	return rotheme.Text(inputprompt.NewResolver().FooterText(ctx.ControllerSettings(), ctx.Input.Controller().Kind, actions...))
}

func appendControllerPromptFooter(footer []widget.Widget, ctx client.Context, actions ...input.Action) []widget.Widget {
	if ctx.Input == nil || ctx.Input.InputSource() != input.InputSourceController || !ctx.Input.Controller().Connected {
		return footer
	}
	return append(footer, controllerPromptFooter(ctx, actions...))
}

// controllerButtonLabel keeps prompts inside the action control instead of
// reserving a second footer row that can displace the actual buttons.
func contextualControllerButtonLabel(ctx client.Context, base string, action input.Action) string {
	if ctx.Input == nil || ctx.Input.InputSource() != input.InputSourceController || !ctx.Input.Controller().Connected {
		return base
	}
	prompt := inputprompt.NewResolver().ForAction(ctx.ControllerSettings(), ctx.Input.Controller().Kind, action)
	if prompt.Text == "" || len(prompt.Text) > 16 || strings.HasPrefix(prompt.Text, "Button") {
		return base
	}
	return base + " [" + prompt.Text + "]"
}

// contextualControllerButton composes the real cached glyph with the action
// control. The text label remains visible for keyboard users and as a fallback
// when an asset is unavailable.
func contextualControllerButton(
	ctx client.Context,
	base string,
	action input.Action,
	onClick func(),
) *primitives.BoxWidget {
	if ctx.Input == nil ||
		ctx.Input.InputSource() != input.InputSourceController ||
		!ctx.Input.Controller().Connected {
		return rotheme.Button(base, onClick)
	}

	prompt := inputprompt.NewResolver().ForAction(
		ctx.ControllerSettings(),
		ctx.Input.Controller().Kind,
		action,
	)

	if prompt.Image == nil {
		return rotheme.Button(
			contextualControllerButtonLabel(ctx, base, action),
			onClick,
		)
	}

	glyphImage := centeredPromptImage(prompt.Image, 24, 20)

	glyph := primitives.Box(
		newStaticImageWidget(glyphImage, 24, 24),
	).
		Width(24).
		Height(24).
		Background(widget.RGBA8(38, 45, 62, 235)).
		Rounded(4)

	return primitives.HBox(
		rotheme.Button(base, onClick),
		glyph,
	).
		Gap(3).
		CrossAlign(primitives.CrossAxisCenter)
}

func contextualControllerButtonDisabled(
	ctx client.Context,
	base string,
	action input.Action,
	disabled bool,
	onClick func(),
) *primitives.BoxWidget {
	if ctx.Input == nil ||
		ctx.Input.InputSource() != input.InputSourceController ||
		!ctx.Input.Controller().Connected {
		return rotheme.ButtonDisabled(base, disabled, onClick)
	}

	prompt := inputprompt.NewResolver().ForAction(
		ctx.ControllerSettings(),
		ctx.Input.Controller().Kind,
		action,
	)

	if prompt.Image == nil {
		return rotheme.ButtonDisabled(
			contextualControllerButtonLabel(ctx, base, action),
			disabled,
			onClick,
		)
	}

	glyphImage := centeredPromptImage(prompt.Image, 22, 16)

	glyph := primitives.Box(
		newStaticImageWidget(glyphImage, 22, 22),
	).
		Width(22).
		Height(22).
		Background(widget.RGBA8(38, 45, 62, 235)).
		Rounded(4)

	return primitives.HBox(
		rotheme.ButtonDisabled(base, disabled, onClick),
		glyph,
	).
		Gap(3).
		CrossAlign(primitives.CrossAxisCenter)
}

// centeredPromptImage trims transparent source padding, preserves aspect ratio,
// and centers the visible artwork inside a fixed-size transparent canvas.
//
// With two arguments:
//
//	centeredPromptImage(src, 20)
//
// the glyph fills up to the full 20x20 canvas.
//
// With three arguments:
//
//	centeredPromptImage(src, 24, 20)
//
// the output is 24x24 while the visible glyph occupies at most 20x20.
func centeredPromptImage(src image.Image, canvasSize int, maxGlyphSize ...int) image.Image {
	if src == nil || canvasSize <= 0 {
		return nil
	}

	glyphSize := canvasSize
	if len(maxGlyphSize) > 0 {
		glyphSize = maxGlyphSize[0]
	}

	if glyphSize <= 0 {
		return nil
	}
	if glyphSize > canvasSize {
		glyphSize = canvasSize
	}

	srcBounds := src.Bounds()

	minX, minY := srcBounds.Max.X, srcBounds.Max.Y
	maxX, maxY := srcBounds.Min.X-1, srcBounds.Min.Y-1

	const alphaThreshold uint32 = 0x0800

	for y := srcBounds.Min.Y; y < srcBounds.Max.Y; y++ {
		for x := srcBounds.Min.X; x < srcBounds.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if a <= alphaThreshold {
				continue
			}

			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	canvas := image.NewRGBA(
		image.Rect(0, 0, canvasSize, canvasSize),
	)

	if maxX < minX || maxY < minY {
		return canvas
	}

	visible := image.Rect(
		minX,
		minY,
		maxX+1,
		maxY+1,
	)

	srcW := visible.Dx()
	srcH := visible.Dy()

	if srcW <= 0 || srcH <= 0 {
		return canvas
	}

	scale := min(
		float64(glyphSize)/float64(srcW),
		float64(glyphSize)/float64(srcH),
	)

	dstW := max(
		1,
		int(float64(srcW)*scale+0.5),
	)
	dstH := max(
		1,
		int(float64(srcH)*scale+0.5),
	)

	dstX := (canvasSize - dstW) / 2
	dstY := (canvasSize - dstH) / 2

	dst := image.Rect(
		dstX,
		dstY,
		dstX+dstW,
		dstY+dstH,
	)

	draw.ApproxBiLinear.Scale(
		canvas,
		dst,
		src,
		visible,
		draw.Over,
		nil,
	)

	return canvas
}