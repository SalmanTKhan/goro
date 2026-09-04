package client

import (
	"time"

	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/audio"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/network"
	"github.com/kivutar/goro/res"
	"github.com/kivutar/goro/session"
	"github.com/kivutar/goro/world"
)

type Context struct {
	Config             config.Config
	Input              *input.State
	Resources          *res.Manager
	Assets             AssetAvailability
	Session            *session.Session
	World              *world.World
	Network            *network.Client
	Offline            *session.OfflineSession
	Audio              *audio.BGM
	Started            time.Time
	ScreenW            int
	ScreenH            int
	UIWidth            int
	UIHeight           int
	Runtime            RuntimeSettings
	RequestQuit        func()
	RequestScreenshot  func() (string, error)
	UIApp              UIApp
	UIManager          UIManager
	MobileSettingsHost MobileSettingsHost
	UISettingsHost     UISettingsHost
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

type UIManager interface {
	AddOverlay(widget.Widget)
	RemoveOverlay(widget.Widget)
	Clear()
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
