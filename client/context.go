package client

import (
	"time"

	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/audio"
	"github.com/kivutar/goro/capture"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/network"
	"github.com/kivutar/goro/res"
	"github.com/kivutar/goro/session"
	"github.com/kivutar/goro/world"
)

type Context struct {
	Config                   config.Config
	Input                    *input.State
	Resources                *res.Manager
	Assets                   AssetAvailability
	Session                  *session.Session
	World                    *world.World
	Network                  *network.Client
	Offline                  *session.OfflineSession
	Audio                    *audio.BGM
	Started                  time.Time
	ScreenW                  int
	ScreenH                  int
	UIWidth                  int
	UIHeight                 int
	Runtime                  RuntimeSettings
	RequestQuit              func()
	RequestScreenshot        func() (string, error)
	RequestScreenshotOptions func(capture.ScreenshotOptions) (string, error)
	StartRecording           func(capture.RecordingOptions) (string, error)
	StopRecording            func() error
	UIApp                    UIApp
	UIManager                UIManager
	MobileSettingsHost       MobileSettingsHost
	UISettingsHost           UISettingsHost
	ControllerSettingsHost   ControllerSettingsHost
}

// ControllerSettings resolves the live controller policy, preferring the
// runtime host so a settings change applies without a restart and falling back
// to the loaded configuration for headless and test contexts.
func (c Context) ControllerSettings() input.ControllerSettings {
	if c.ControllerSettingsHost != nil {
		return c.ControllerSettingsHost.ControllerSettings().Normalized()
	}
	return c.Config.Controller.Normalized()
}

type PackState string

const (
	PackUnknown     PackState = "unknown"
	PackAvailable   PackState = "available"
	PackDownloading PackState = "downloading"
	PackFailed      PackState = "failed"
)

type AssetRequirement struct {
	MapName string
	Ready   bool
	Missing []string
}

type AssetEvent struct {
	Pack  string
	State PackState
	Error string
}

// AssetAvailability lets the game gate a transition without coupling game
// modes to Android's downloader. A completed overlay changes the resource
// view and the same requirement becomes ready on the next update.
type AssetAvailability interface {
	PackState(name string) PackState
	RequireMap(mapName string) AssetRequirement
	RequestPack(name string) error
	Subscribe(func(AssetEvent))
}

type UIApp interface {
	SetUIRoot(widget.Widget)
	Frame()
	Invalidate()
	Cursor() widget.CursorType
	HoveredWidget() widget.Widget
}

// UIController is an optional capability implemented by desktop render
// bridges. Keeping it separate preserves the lightweight UIApp contract used
// by headless and Android callers.
type UIController interface {
	SetControllerMode(bool)
	HandleControllerAction(input.UIAction) bool
}

type UIManager interface {
	AddOverlay(widget.Widget)
	RemoveOverlay(widget.Widget)
	Clear()
}

// UIPointer is the optional capability that answers whether a screen point
// belongs to the UI rather than the world. Both the real mouse path and the
// controller's virtual pointer resolve through it, so they always agree.
type UIPointer interface {
	PointerOverUI(x, y int) bool
}

// UIViewportManager is the optional responsive extension implemented by UI
// managers that own positioned overlays. Keeping it separate preserves small
// test and headless UIManager implementations.
type UIViewportManager interface {
	ViewportChanged(oldWidth, oldHeight, width, height int)
}

type MobileSettingsHost interface {
	MobileSettings() input.MobileSettings
	ApplyMobileSettings(input.MobileSettings) bool
}

type UISettingsHost interface {
	UISettings() input.UISettings
	ApplyUISettings(input.UISettings) bool
}

// ControllerSettingsHost lets the settings UI read and apply controller
// settings at runtime, mirroring UISettingsHost.
type ControllerSettingsHost interface {
	ControllerSettings() input.ControllerSettings
	ApplyControllerSettings(input.ControllerSettings) bool
}

type RuntimeSettings interface {
	Fullscreen() bool
	SetFullscreen(bool)
	VSync() bool
	SetVSync(bool)
	FPS() bool
	SetFPS(bool)
}

func (c Context) ScreenSize() (int, int) {
	width, height := c.ScreenW, c.ScreenH
	if width <= 0 {
		width = c.Config.Window.Width
	}
	if height <= 0 {
		height = c.Config.Window.Height
	}
	return width, height
}

func (c Context) UIScreenSize() (int, int) {
	width, height := c.UIWidth, c.UIHeight
	if width <= 0 || height <= 0 {
		return c.ScreenSize()
	}
	return width, height
}
