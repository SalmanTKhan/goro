package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// MapTree builds the world map screen: the map raster with its warp list
// beside or below it, and the travel confirmation on top.
//
// The raster and its markers are drawn by the host from live map data; this is
// the frame around them and the list of destinations.
func (k Kit) MapTree(
	model mobileui.MobileMapModel,
	layout mobileui.MobileMapLayout,
	state mobileui.MapInteractionState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)

	k.placeMapHeader(c, model, layout)
	if layout.MapViewport.W > 0 && layout.MapViewport.H > 0 {
		c.Place(k.Panel(), layout.MapViewport)
	}
	k.placeWarpList(c, model, layout, state)
	k.placeWarpDetail(c, model, layout, state)
	k.placeTravelConfirm(c, model, layout, state)
	return c
}

func (k Kit) placeMapHeader(c *Canvas, model mobileui.MobileMapModel, layout mobileui.MobileMapLayout) {
	if layout.Header.W <= 0 {
		return
	}
	c.Place(k.Panel(), layout.Header)
	pad := k.Theme.Metrics.TableCellPadX
	if layout.BackButton.W > 0 {
		c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)
	}
	left := layout.Header.X + pad
	if layout.BackButton.W > 0 {
		left = layout.BackButton.Right() + pad
	}
	title := mobileui.Rect{X: left, Y: layout.Header.Y, W: layout.Header.Right() - pad - left, H: layout.Header.H}
	if title.W <= 0 {
		return
	}
	// Map name over the player's coordinates: the two facts a player checks
	// when opening the map.
	half := title.H / 2
	c.Place(k.Centered(model.MapName, RoleTitle), mobileui.Rect{X: title.X, Y: title.Y, W: title.W, H: half})
	c.Place(k.Centered(coordinates(model), RoleMuted), mobileui.Rect{X: title.X, Y: title.Y + half, W: title.W, H: half})
}

func coordinates(model mobileui.MobileMapModel) string {
	return strconv.Itoa(model.PlayerX) + ", " + strconv.Itoa(model.PlayerY)
}

func (k Kit) placeWarpList(
	c *Canvas,
	model mobileui.MobileMapModel,
	layout mobileui.MobileMapLayout,
	state mobileui.MapInteractionState,
) {
	if layout.WarpPanel.W <= 0 || layout.WarpPanel.H <= 0 {
		return
	}
	c.Place(k.Panel(), warpPanelSurface(layout))

	pad := k.Theme.Metrics.TableCellPadX
	if len(layout.WarpRows) == 0 {
		c.Place(k.Centered("No exits nearby", RoleMuted), layout.WarpPanel)
		return
	}
	for _, warp := range layout.WarpRows {
		if warp.Rect.W <= 0 || warp.Rect.H <= 0 {
			continue
		}
		if layout.WarpListViewport.H > 0 && !warp.Rect.Intersects(layout.WarpListViewport) {
			continue
		}
		model, ok := warpByID(model.Warps, warp.ID)
		if !ok {
			continue
		}
		c.Place(k.Card(warp.ID == state.SelectedWarpID), warp.Rect)

		inner := mobileui.Rect{X: warp.Rect.X + pad, Y: warp.Rect.Y, W: warp.Rect.W - 2*pad, H: warp.Rect.H}
		half := inner.H / 2
		c.Place(k.Content(warpName(model), RoleValue), mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: half})
		c.Place(k.Content(warpDestination(model), RoleMuted),
			mobileui.Rect{X: inner.X, Y: inner.Y + half, W: inner.W, H: half})
	}
}

func warpName(warp mobileui.MobileMapWarpModel) string {
	if warp.Name != "" {
		return warp.Name
	}
	return "Exit"
}

func warpDestination(warp mobileui.MobileMapWarpModel) string {
	if warp.DestinationMap == "" {
		return ""
	}
	return "to " + warp.DestinationMap + "  " +
		strconv.Itoa(warp.DestinationX) + ", " + strconv.Itoa(warp.DestinationY)
}

// placeWarpDetail shows the selected exit and the travel action.
func (k Kit) placeWarpDetail(
	c *Canvas,
	model mobileui.MobileMapModel,
	layout mobileui.MobileMapLayout,
	state mobileui.MapInteractionState,
) {
	if layout.DetailPanel.W <= 0 || layout.DetailPanel.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.DetailPanel)
	pad := k.Theme.Metrics.TableCellPadX
	inner := mobileui.Rect{
		X: layout.DetailPanel.X + pad, Y: layout.DetailPanel.Y + pad,
		W: layout.DetailPanel.W - 2*pad, H: layout.DetailPanel.H - 2*pad,
	}
	warp, ok := warpByID(model.Warps, state.SelectedWarpID)
	if !ok {
		c.Place(k.Centered("Select an exit to travel", RoleMuted), inner)
		return
	}
	rowH := k.Theme.Metrics.TableRowHeight
	c.Place(k.Wrapped(warpName(warp), RoleTitle, 2), mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: rowH})
	c.Place(k.Text(warpDestination(warp), RoleMuted),
		mobileui.Rect{X: inner.X, Y: inner.Y + rowH, W: inner.W, H: rowH})
	if layout.TravelButton.W > 0 {
		c.Place(k.Button("Travel", ButtonNormal), layout.TravelButton)
	}
}

// placeTravelConfirm is the modal shown before moving to an exit.
func (k Kit) placeTravelConfirm(
	c *Canvas,
	model mobileui.MobileMapModel,
	layout mobileui.MobileMapLayout,
	state mobileui.MapInteractionState,
) {
	if !state.ConfirmOpen || layout.ConfirmModal.W <= 0 {
		return
	}
	c.Place(k.Scrim(), layout.Safe)
	c.Place(k.Window("Travel", nil), layout.ConfirmModal)
	if layout.ConfirmButton.W > 0 {
		c.Place(k.Button("Travel", ButtonNormal), layout.ConfirmButton)
	}
	if layout.ConfirmCancel.W > 0 {
		c.Place(k.Button("Cancel", ButtonNormal), layout.ConfirmCancel)
	}

	bodyTop := layout.ConfirmModal.Y + k.Theme.Metrics.WindowTitleHeight
	bodyBottom := layout.ConfirmModal.Bottom()
	if layout.ConfirmButton.H > 0 {
		bodyBottom = layout.ConfirmButton.Y
	}
	body := mobileui.Rect{
		X: layout.ConfirmModal.X + k.Theme.Metrics.TableCellPadX, Y: bodyTop,
		W: layout.ConfirmModal.W - 2*k.Theme.Metrics.TableCellPadX, H: bodyBottom - bodyTop,
	}
	if body.H <= 0 {
		return
	}
	prompt := "Travel to this exit?"
	if warp, ok := warpByID(model.Warps, state.SelectedWarpID); ok {
		prompt = "Travel to " + warpName(warp) + "?"
	}
	c.Place(k.Wrapped(prompt, RoleBody, 2), body)
}

// warpPanelSurface bounds the warp list to the rows it holds.
func warpPanelSurface(layout mobileui.MobileMapLayout) mobileui.Rect {
	surface := layout.WarpPanel
	var bottom float32
	for _, warp := range layout.WarpRows {
		if warp.Rect.Bottom() > bottom {
			bottom = warp.Rect.Bottom()
		}
	}
	if bottom <= surface.Y {
		return surface
	}
	if h := bottom - surface.Y + 8; h < surface.H {
		surface.H = h
	}
	return surface
}

func warpByID(warps []mobileui.MobileMapWarpModel, id uint32) (mobileui.MobileMapWarpModel, bool) {
	for _, warp := range warps {
		if warp.ID == id {
			return warp, true
		}
	}
	return mobileui.MobileMapWarpModel{}, false
}
