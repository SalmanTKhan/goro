//go:build android && cgo

package main

import (
	"fmt"
	"math"

	"github.com/gogpu/gpucontext"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/app"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/render"
	"github.com/kivutar/goro/ui/rotheme"
)

const (
	// desktopUIWidth and desktopUIHeight are only a placeholder viewport for the
	// moment between constructing the app and the first Resize, which computes
	// the real logical size from the display.
	desktopUIWidth  = 1280
	desktopUIHeight = 720

	// desktopUIMinSpan is how many logical UI pixels the display's shorter edge
	// covers. It is the one number that decides how large the desktop UI appears
	// on a phone.
	//
	// Sizing the logical viewport to a fixed 1280-pixel width instead — treating
	// the handset as a small monitor — renders everything at roughly 0.84x on a
	// 1080-pixel phone, i.e. smaller than on the desktop it was designed for, on
	// a screen held at a fraction of the distance. A 324-pixel inventory window
	// came out about 270 physical pixels wide and was unreadable.
	//
	// Deriving the scale from the shorter edge makes the result independent of
	// panel resolution (a 1080p and a 1440p phone lay out identically, the
	// latter simply crisper) while still giving tablets and unfolded devices
	// more logical room, since their shorter edge is genuinely larger.
	//
	// 640 is set by the widest fixed-size window in the client: character
	// select at 576 (ui/character_select_window.go), which overflowed the
	// display at 560. Anything wider than this still needs the mobile window
	// placement policy to clamp it.
	desktopUIMinSpan = 640
)

type androidUIWindow struct{ width, height int }

func (w *androidUIWindow) Size() (int, int)     { return w.width, w.height }
func (w *androidUIWindow) ScaleFactor() float64 { return 1 }
func (w *androidUIWindow) RequestRedraw()       {}

var _ gpucontext.WindowProvider = (*androidUIWindow)(nil)

type desktopPresentation struct {
	game                    *app.Game
	ui                      *uiapp.App
	win                     *androidUIWindow
	image                   *render.Image
	width, height           int
	safeLeft, safeTop       int
	safeRight, safeBottom   int
	uiWidth, uiHeight       int
	offsetX, offsetY, scale float32
	uiScale                 float32
	dirty                   bool
	captured                bool
	debugLogged             bool

	// pointer queues touches for the polled input path, in logical UI
	// coordinates. Widget events reach gogpu immediately, but large parts of the
	// desktop UI never see those: window dragging (ui/window.go), the inventory,
	// storage and cart item grids, drag and drop, and the shortcut bar all poll
	// client.Context.Input once per frame instead. Without this the desktop UI
	// renders on Android but cannot be manipulated.
	//
	// Events are queued rather than collapsed to a latest-value because touches
	// arrive between render ticks: a tap whose press and release both land in the
	// same gap would otherwise cancel out and never be seen by the game.
	pointer     []pointerEvent
	pointerDown bool
}

// pointerEvent is one touch expressed in the client's mouse model.
type pointerEvent struct {
	x, y    int
	pressed bool
}

func newDesktopPresentation(game *app.Game, width, height int) *desktopPresentation {
	d := &desktopPresentation{game: game, win: &androidUIWindow{width: desktopUIWidth, height: desktopUIHeight}, uiWidth: desktopUIWidth, uiHeight: desktopUIHeight, uiScale: game.UISettings().Normalized().Scale, dirty: true}
	theme := rotheme.Default.AsTheme()
	theme.Colors.Background = widget.RGBA8(0, 0, 0, 0)
	// RasterizeUI creates a fresh canvas for every redraw. Framework-managed
	// rendering is incremental and assumes a persistent backing pixmap, so a
	// touch would redraw only its dirty widget into an otherwise empty canvas.
	// Host-managed mode redraws the complete widget tree into each fresh raster.
	d.ui = uiapp.New(uiapp.WithWindowProvider(d.win), uiapp.WithTheme(theme), uiapp.WithRenderMode(uiapp.RenderModeHostManaged))
	game.SetUIApp(d)
	d.Resize(width, height)
	return d
}

func (d *desktopPresentation) SetUIRoot(root widget.Widget) { d.ui.SetRoot(root); d.dirty = true }
func (d *desktopPresentation) Frame()                       { d.ui.Frame() }
func (d *desktopPresentation) Invalidate()                  { d.dirty = true }
func (d *desktopPresentation) SetUISettings(settings input.UISettings) {
	if d == nil {
		return
	}
	d.uiScale = settings.Normalized().Scale
	d.Resize(d.width, d.height)
}

func (d *desktopPresentation) SetSafeInsets(left, top, right, bottom int) {
	if d == nil {
		return
	}
	d.safeLeft = maxInt(0, left)
	d.safeTop = maxInt(0, top)
	d.safeRight = maxInt(0, right)
	d.safeBottom = maxInt(0, bottom)
	d.Resize(d.width, d.height)
}
func (d *desktopPresentation) Cursor() widget.CursorType    { return d.ui.Window().Context().Cursor() }
func (d *desktopPresentation) HoveredWidget() widget.Widget { return d.ui.Window().HoveredWidget() }

// ConsumeTouch reports ownership after Touch has dispatched the current
// event. The host calls Touch first, so this does not dispatch the UI event a
// second time when the shared mobile gesture adapter performs its filtering.
func (d *desktopPresentation) ConsumeTouch(input.TouchPoint) bool {
	return d != nil && d.captured
}

func (d *desktopPresentation) TextInputActive() bool {
	_, ok := d.ui.Window().Context().FocusedWidget().(*textfield.Widget)
	return ok
}

func (d *desktopPresentation) SetText(text string) {
	if field, ok := d.ui.Window().Context().FocusedWidget().(*textfield.Widget); ok {
		field.SetText(text)
	}
}

func (d *desktopPresentation) Resize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	d.width, d.height = width, height
	availableWidth := width - d.safeLeft - d.safeRight
	availableHeight := height - d.safeTop - d.safeBottom
	if availableWidth <= 0 || availableHeight <= 0 {
		return
	}
	userScale := d.uiScale
	if userScale <= 0 {
		userScale = 1
	}
	// Map the display's shorter edge onto desktopUIMinSpan logical pixels and
	// let the longer edge follow, so the viewport always covers the whole
	// display in either orientation without letterboxing. A larger user scale
	// means a larger scale factor, hence fewer logical pixels and bigger UI.
	shorter := availableWidth
	if availableHeight < shorter {
		shorter = availableHeight
	}
	scale := float32(shorter) / desktopUIMinSpan * userScale
	if scale <= 0 {
		scale = 1
	}
	uiWidth := int(math.Round(float64(availableWidth) / float64(scale)))
	uiHeight := int(math.Round(float64(availableHeight) / float64(scale)))
	if uiWidth < 1 {
		uiWidth = 1
	}
	if uiHeight < 1 {
		uiHeight = 1
	}
	d.uiWidth, d.uiHeight = uiWidth, uiHeight
	d.win.width, d.win.height = d.uiWidth, d.uiHeight
	d.game.SetUIViewport(d.uiWidth, d.uiHeight)
	d.scale = scale
	d.offsetX = float32(d.safeLeft) + (float32(availableWidth)-float32(d.uiWidth)*scale)/2
	d.offsetY = float32(d.safeTop) + (float32(availableHeight)-float32(d.uiHeight)*scale)/2
	// A surface change terminates the old physical gesture. Queue a release for
	// the polled desktop path so neither window dragging nor UI capture can
	// remain latched across rotation.
	if d.pointerDown {
		d.pointer = append(d.pointer, pointerEvent{x: d.uiWidth / 2, y: d.uiHeight / 2, pressed: false})
	}
	d.captured = false
	d.dirty = true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (d *desktopPresentation) Draw(frame *render.Frame) {
	if d == nil || frame == nil {
		return
	}
	if d.dirty || d.ui.Window().NeedsRedraw() || d.ui.Window().NeedsAnimationFrame() {
		firstDraw := !d.debugLogged
		if firstDraw {
			if d.ui.Window().Root() == nil {
				androidLog("stage=desktop-ui root=nil")
			} else {
				androidLog("stage=desktop-ui root=present")
			}
		}
		if image, _, err := render.RasterizeUI(d.ui, d.uiWidth, d.uiHeight, d.image); err == nil {
			d.image = image
			if d.image != nil && firstDraw {
				nonzero := 0
				for i := 3; i < len(d.image.RGBA().Pix); i += 4 {
					if d.image.RGBA().Pix[i] != 0 {
						nonzero++
					}
				}
				androidLog(fmt.Sprintf("stage=desktop-ui raster size=%dx%d alpha-pixels=%d", d.image.Bounds().Dx(), d.image.Bounds().Dy(), nonzero))
			}
		} else {
			androidLog(fmt.Sprintf("stage=desktop-ui raster-error=%v", err))
		}
		// The desktop backend clears this after each raster (render/backend.go);
		// without it the flag latches and every frame re-rasterizes the whole
		// widget tree through a freshly allocated canvas.
		d.ui.Window().ClearAnimationFrame()
		d.debugLogged = true
		d.dirty = false
	}
	if d.image != nil {
		frame.SetScreenTransform(d.scale, d.scale, d.offsetX, d.offsetY)
		frame.DrawImage(d.image, nil)
	}
}

func (d *desktopPresentation) logicalPoint(x, y int) (int, int, bool) {
	if d == nil || d.scale <= 0 {
		return 0, 0, false
	}
	lx := (float32(x) - d.offsetX) / d.scale
	ly := (float32(y) - d.offsetY) / d.scale
	return int(math.Round(float64(lx))), int(math.Round(float64(ly))), lx >= 0 && ly >= 0 && lx < float32(d.uiWidth) && ly < float32(d.uiHeight)
}

func (d *desktopPresentation) Touch(action, x, y int, pressed bool) bool {
	lx, ly, inside := d.logicalPoint(x, y)
	if !inside && action == 0 {
		return false
	}
	// Only a gesture that began on a UI widget belongs to the desktop UI.
	// Release events inside the fitted logical surface must otherwise fall
	// through to the world (for example, a tap-to-move release).
	if action != 0 && !d.captured {
		return false
	}
	if d.game != nil {
		if blocker, ok := d.game.ContextUIManager().(interface{ PointerBlocked(int, int) bool }); ok && action == 0 && !blocker.PointerBlocked(lx, ly) {
			return false
		}
	}
	if action == 0 {
		d.captured = true
	}
	typ := event.MouseMove
	button := event.ButtonNone
	buttons := event.ButtonState(0)
	if pressed {
		buttons = event.ButtonStateLeft
	}
	if action == 0 || action == 5 {
		typ, button = event.MousePress, event.ButtonLeft
	}
	if action == 1 || action == 3 || action == 6 {
		typ, button, buttons = event.MouseRelease, event.ButtonLeft, 0
	}
	d.ui.HandleEvent(event.NewMouseEvent(typ, button, buttons, geometry.Pt(float32(lx), float32(ly)), geometry.Pt(float32(lx), float32(ly)), 0))

	// Mirror the same touch into the polled input queue so the parts of the
	// desktop UI that read client.Context.Input each frame can respond too.
	down := pressed
	switch {
	case typ == event.MousePress:
		down = true
	case typ == event.MouseRelease:
		down = false
	}
	d.pointer = append(d.pointer, pointerEvent{x: lx, y: ly, pressed: down})

	d.dirty = true
	if action == 1 || action == 3 || action == 6 {
		d.captured = false
	}
	return true
}

// SyncInput applies queued touches to the game's input state. It must be called
// once per frame immediately before app.Game.Update, which ends the input frame
// itself, so just-pressed edges live for exactly that one update.
//
// Note this is the game's own state (app.Game.InputState), not the host's touch
// state: client.Context.Input is what the desktop windows poll.
//
// At most one button transition is applied per call: a press and the release
// that follows it have to be visible on separate frames, or a window would
// begin and end its drag within a single update and never move.
func (d *desktopPresentation) SyncInput(state *input.State) {
	if d == nil || state == nil || len(d.pointer) == 0 {
		return
	}
	consumed, transitions := 0, 0
	for _, ev := range d.pointer {
		if ev.pressed != d.pointerDown {
			if transitions > 0 {
				break // leave the rest of the queue for the next frame
			}
			transitions++
		}
		state.SetMousePosition(ev.x, ev.y)
		if ev.pressed != d.pointerDown {
			state.SetMouseButton(input.MouseButtonLeft, ev.pressed)
			d.pointerDown = ev.pressed
		}
		consumed++
	}
	d.pointer = d.pointer[consumed:]
}

var _ client.UIApp = (*desktopPresentation)(nil)
