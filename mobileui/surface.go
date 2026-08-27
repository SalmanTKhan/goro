package mobileui

import (
	"fmt"

	"github.com/kivutar/goro/input"
)

type SurfaceItemKind uint8

const (
	SurfaceItemRow SurfaceItemKind = iota
	SurfaceItemSection
)

// SurfaceItem is the common projection for settings, social, companion, and
// advanced windows. IDs are stable semantic IDs; the renderer never needs a
// widget pointer or a screen coordinate to describe an action.
type SurfaceItem struct {
	ID      string
	Label   string
	Value   string
	Detail  string
	Enabled bool
	Kind    SurfaceItemKind
}

type SurfaceModel struct {
	Title  string
	Notice string
	Items  []SurfaceItem
}

type SurfaceInteractionState struct {
	SelectedID string
	DetailOpen bool
}

type SurfaceLayout struct {
	Safe, Panel, Header, Back, ListViewport, DetailSheet, Notice Rect
	Rows                                                         []Rect
	RowIDs                                                       []string
	Portrait                                                     bool
}

func LayoutSurface(viewport Viewport, model SurfaceModel, state SurfaceInteractionState, offset float32) SurfaceLayout {
	safe := viewport.SafeRect()
	layout := SurfaceLayout{Safe: safe, Portrait: viewport.IsPortrait()}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	panelW := minf(1200, maxf(0, safe.W-32))
	if layout.Portrait {
		panelW = maxf(0, safe.W-32)
	}
	layout.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: safe.Y + 8, W: panelW, H: maxf(0, safe.H-16)}
	pad := float32(20)
	if layout.Portrait {
		pad = 16
	}
	layout.Header = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + 12, W: layout.Panel.W - 2*pad, H: 56}
	layout.Back = Rect{X: layout.Header.X, Y: layout.Header.Y, W: 104, H: 52}
	listY := layout.Header.Bottom() + 12
	listBottom := layout.Panel.Bottom() - 12
	if model.Notice != "" {
		layout.Notice = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Bottom() - 34, W: layout.Panel.W - 2*pad, H: 22}
		listBottom = layout.Notice.Y - 8
	}
	if state.DetailOpen {
		detailH := minf(260, maxf(196, safe.H*0.30))
		layout.DetailSheet = Rect{X: layout.Panel.X + pad, Y: listBottom - detailH, W: layout.Panel.W - 2*pad, H: detailH}
		listBottom = layout.DetailSheet.Y - 12
	}
	layout.ListViewport = Rect{X: layout.Panel.X + pad, Y: listY, W: layout.Panel.W - 2*pad, H: maxf(0, listBottom-listY)}
	rowExtent := float32(64)
	for i, item := range model.Items {
		rect := Rect{X: layout.ListViewport.X, Y: layout.ListViewport.Y + float32(i)*rowExtent - offset, W: layout.ListViewport.W, H: 56}
		if rect.Y < layout.ListViewport.Y || rect.Bottom() > layout.ListViewport.Bottom() {
			continue
		}
		layout.Rows = append(layout.Rows, rect)
		layout.RowIDs = append(layout.RowIDs, item.ID)
	}
	return layout
}

func SurfaceScrollExtent(layout SurfaceLayout, itemCount int, offset float32) ScrollState {
	rowExtent := float32(64)
	content := float32(itemCount) * rowExtent
	if content > 0 {
		content -= 8
	}
	return ScrollState{ViewportExtent: layout.ListViewport.H, ContentExtent: content, Offset: offset, RowExtent: rowExtent}
}

type SurfaceController struct {
	Model     SurfaceModel
	State     SurfaceInteractionState
	Layout    SurfaceLayout
	Viewport  Viewport
	Scroll    ScrollState
	Sink      input.CommandSink
	OnItemTap func(SurfaceItem) bool
}

func NewSurfaceController(model SurfaceModel, viewport Viewport, sink input.CommandSink) *SurfaceController {
	c := &SurfaceController{Model: model, Viewport: viewport, Sink: sink}
	c.relayout()
	return c
}

func (c *SurfaceController) SetModel(model SurfaceModel) {
	if c == nil {
		return
	}
	c.Model = model
	c.relayout()
}

func (c *SurfaceController) Resize(viewport Viewport) {
	if c == nil {
		return
	}
	c.Viewport = viewport
	c.relayout()
}

func (c *SurfaceController) ConsumeTouch(point input.TouchPoint) bool {
	return c != nil && c.Layout.Safe.Contains(float32(point.X), float32(point.Y))
}

func (c *SurfaceController) Tap(x, y float32) bool {
	if c == nil || !c.Layout.Safe.Contains(x, y) {
		return false
	}
	if c.Layout.Back.Contains(x, y) {
		return c.Back()
	}
	if c.State.DetailOpen && !c.Layout.DetailSheet.Contains(x, y) && c.Layout.DetailSheet.W > 0 {
		c.State.DetailOpen = false
		c.State.SelectedID = ""
		c.relayout()
		return true
	}
	for i, row := range c.Layout.Rows {
		if !row.Contains(x, y) || i >= len(c.Layout.RowIDs) {
			continue
		}
		item, ok := surfaceItemByID(c.Model, c.Layout.RowIDs[i])
		if !ok {
			return true
		}
		if item.Kind == SurfaceItemSection {
			c.State.SelectedID = ""
			return true
		}
		c.State.SelectedID = item.ID
		if item.Enabled && c.OnItemTap != nil && c.OnItemTap(item) {
			c.relayout()
			return true
		}
		c.State.DetailOpen = true
		c.relayout()
		return true
	}
	return true
}

func surfaceItemByID(model SurfaceModel, id string) (SurfaceItem, bool) {
	for _, item := range model.Items {
		if item.ID == id {
			return item, true
		}
	}
	return SurfaceItem{}, false
}

func (c *SurfaceController) ScrollBy(delta float32) bool {
	if c == nil || c.State.DetailOpen {
		return false
	}
	c.Scroll.ScrollBy(delta)
	c.relayout()
	return true
}

func (c *SurfaceController) Back() bool {
	if c == nil {
		return false
	}
	if c.State.DetailOpen {
		c.State.DetailOpen = false
		c.State.SelectedID = ""
		c.relayout()
		return true
	}
	return true
}

func (c *SurfaceController) Selected() (SurfaceItem, bool) {
	if c == nil || c.State.SelectedID == "" {
		return SurfaceItem{}, false
	}
	for _, item := range c.Model.Items {
		if item.ID == c.State.SelectedID {
			return item, true
		}
	}
	return SurfaceItem{}, false
}

func (c *SurfaceController) relayout() {
	if c == nil {
		return
	}
	base := LayoutSurface(c.Viewport, c.Model, c.State, 0)
	c.Scroll = SurfaceScrollExtent(base, len(c.Model.Items), c.Scroll.Offset)
	c.Scroll.SetOffset(c.Scroll.Offset)
	c.Layout = LayoutSurface(c.Viewport, c.Model, c.State, c.Scroll.Offset)
}

func SettingsSurface() SurfaceModel {
	return SettingsSurfaceForSettings(input.DefaultMobileSettings())
}

func SettingsSurfaceForControls(controls input.MobileControls) SurfaceModel {
	settings := input.DefaultMobileSettings()
	settings.Controls = controls
	return SettingsSurfaceForSettings(settings)
}

func SettingsSurfaceForSettings(settings input.MobileSettings) SurfaceModel {
	settings = settings.Normalized()
	controls := settings.Controls
	controls = controls.Normalized()
	return SurfaceModel{
		Title:  "SETTINGS",
		Notice: "Tap a row to change it. Changes apply now and save automatically.",
		Items: []SurfaceItem{
			{ID: "section-controls", Label: "CONTROLS", Value: "", Detail: "Touch and camera behavior.", Enabled: false, Kind: SurfaceItemSection},
			{ID: "movement", Label: "Movement", Value: controls.MovementMode.String(), Detail: "Hold to move follows walkable ground while your finger is down. Tap to move acts when the touch is released.", Enabled: true},
			{ID: "camera-rotation", Label: "Camera rotation", Value: "2-finger drag", Detail: "Drag with two fingers to rotate the camera. One-finger world input never rotates the camera.", Enabled: false},
			{ID: "camera-sensitivity", Label: "Camera sensitivity", Value: fmt.Sprintf("%.2fx", controls.CameraSensitivity), Enabled: true},
			{ID: "zoom-sensitivity", Label: "Pinch sensitivity", Value: fmt.Sprintf("%.2fx", controls.ZoomSensitivity), Enabled: true},
			{ID: "invert-camera-y", Label: "Invert camera vertical", Value: onOff(controls.InvertCameraY), Enabled: true},
			{ID: "long-press", Label: "Inspect hold duration", Value: fmt.Sprintf("%d ms", controls.LongPressMS), Enabled: true},
			{ID: "show-target-names", Label: "Show target names", Value: onOff(controls.ShowTargetNames), Enabled: true},
			{ID: "reset-controls", Label: "Reset controls", Value: "Defaults", Detail: "Restore hold-to-move, two-finger camera rotation, standard sensitivity, and touch inspection defaults.", Enabled: true},
			{ID: "section-audio", Label: "AUDIO", Value: "", Detail: "Sound preferences.", Enabled: false, Kind: SurfaceItemSection},
			{ID: "bgm-enabled", Label: "Background music", Value: onOff(settings.Audio.BGMEnabled), Enabled: true},
			{ID: "bgm-volume", Label: "BGM volume", Value: fmt.Sprintf("%d%%", int(settings.Audio.BGMVolume*100+0.5)), Enabled: true},
			{ID: "sfx-volume", Label: "SFX volume", Value: fmt.Sprintf("%d%%", int(settings.Audio.SFXVolume*100+0.5)), Enabled: true},
			{ID: "section-display", Label: "DISPLAY", Value: "", Detail: "Mobile HUD visibility.", Enabled: false, Kind: SurfaceItemSection},
			{ID: "show-minimap", Label: "Show minimap", Value: onOff(settings.Display.ShowMinimap), Enabled: true},
			{ID: "section-gameplay", Label: "GAMEPLAY", Value: "", Detail: "Shared client gameplay behavior.", Enabled: false, Kind: SurfaceItemSection},
			{ID: "no-shift", Label: "No Shift targeting", Value: onOff(settings.Gameplay.NoShift), Enabled: true},
			{ID: "no-ctrl", Label: "No Ctrl attacking", Value: onOff(settings.Gameplay.NoCtrl), Enabled: true},
			{ID: "less-effects", Label: "Less effects", Value: onOff(settings.Gameplay.LessEffects), Enabled: true},
			{ID: "snap-targets", Label: "Snap to targets", Value: onOff(settings.Gameplay.SnapTargets), Enabled: true},
			{ID: "snap-items", Label: "Snap to items", Value: onOff(settings.Gameplay.SnapItems), Enabled: true},
			{ID: "reset-all", Label: "Reset all mobile settings", Value: "Defaults", Detail: "Restore controls, audio, display, and gameplay settings to their validated defaults.", Enabled: true},
		},
	}
}

// SettingsSurfaceForSession adds the connection controls that are only
// meaningful to the active mobile session. Keeping them in a separate
// projection preserves the stable settings surface used by previews and
// offline mode while giving an online world an explicit disconnect action.
func SettingsSurfaceForSession(settings input.MobileSettings, online bool, server, status string) SurfaceModel {
	model := SettingsSurfaceForSettings(settings)
	value, detail := "Offline mode", "Offline owns all gameplay state; no network connection is active."
	if online {
		value = status
		if value == "" {
			value = "Online"
		}
		detail = "Online owns gameplay authority for this session."
	}
	if server == "" {
		server = "Configured server"
	}
	sessionItems := []SurfaceItem{
		{ID: "section-session", Label: "SESSION", Detail: "Connection and authority.", Enabled: false, Kind: SurfaceItemSection},
		{ID: "session-status", Label: "Connection", Value: value, Detail: detail, Enabled: false},
		{ID: "session-server", Label: "Server", Value: server, Enabled: false},
		{ID: "disconnect", Label: "Disconnect", Value: "Return to character selection", Detail: "Close the online session and return to the shared character screen.", Enabled: online},
	}
	model.Items = append(sessionItems, model.Items...)
	return model
}

func onOff(value bool) string {
	if value {
		return "On"
	}
	return "Off"
}
