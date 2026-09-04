package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// StartupTree builds the title screen: the game's name, a line of context and
// the two ways in.
func (k Kit) StartupTree(model mobileui.StartupModel, layout mobileui.StartupLayout) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	if layout.Logo.W > 0 {
		c.Place(k.Centered(model.Title, RoleTitle), layout.Logo)
	}
	if layout.Subtitle.W > 0 && model.Subtitle != "" {
		c.Place(k.Wrapped(model.Subtitle, RoleMuted, 2), layout.Subtitle)
	}
	if layout.Action.W > 0 {
		c.Place(k.Button(startupActionLabel(model), ButtonNormal), layout.Action)
	}
	if layout.AlternateAction.W > 0 {
		c.Place(k.Button("Play online", ButtonNormal), layout.AlternateAction)
	}
	return c
}

func startupActionLabel(model mobileui.StartupModel) string {
	if model.Action != "" {
		return model.Action
	}
	return "Start offline"
}

// OnlineLoginTree builds the character-select grid for an online session.
func (k Kit) OnlineLoginTree(model mobileui.MobileOnlineLoginModel, layout mobileui.OnlineLoginLayout) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	if layout.Title.W > 0 {
		c.Place(k.Centered(onlineTitle(model), RoleTitle), layout.Title)
	}
	if layout.Status.W > 0 && model.Status != "" {
		c.Place(k.Centered(model.Status, RoleMuted), layout.Status)
	}
	if layout.Network.W > 0 && model.Network != "" {
		c.Place(k.Centered(model.Network, RoleMuted), layout.Network)
	}
	k.placeCharacterSlots(c, model, layout)
	if layout.Notice.W > 0 && model.Notice != "" {
		c.Place(k.Wrapped(model.Notice, RoleMuted, 2), layout.Notice)
	}

	for _, action := range []struct {
		rect    mobileui.Rect
		label   string
		enabled bool
	}{
		{layout.Reconnect, "Reconnect", model.CanReconnect},
		{layout.Disconnect, "Disconnect", model.CanDisconnect},
		{layout.Create, "Create", model.CanCreate},
		{layout.Mode, "Offline", model.CanSwitchMode},
	} {
		if action.rect.W > 0 && action.rect.H > 0 {
			c.Place(k.Button(action.label, buttonStateFor(action.enabled)), action.rect)
		}
	}
	return c
}

func onlineTitle(model mobileui.MobileOnlineLoginModel) string {
	if model.Server != "" {
		return model.Server
	}
	return "Select character"
}

// placeCharacterSlots draws the grid of save slots. An empty slot invites
// creation rather than showing nothing.
func (k Kit) placeCharacterSlots(
	c *Canvas,
	model mobileui.MobileOnlineLoginModel,
	layout mobileui.OnlineLoginLayout,
) {
	pad := k.Theme.Metrics.TableCellPadX
	for i, slot := range layout.Slots {
		if slot.W <= 0 || slot.H <= 0 {
			continue
		}
		if i >= len(model.Characters) {
			c.Place(k.Slot(false, false), slot)
			c.Place(k.Centered("Empty", RoleMuted), slot)
			continue
		}
		character := model.Characters[i]
		c.Place(k.Card(model.SelectedSlot == character.Slot), slot)
		if !character.Occupied {
			c.Place(k.Centered("Create", RoleMuted), slot)
			continue
		}
		inner := mobileui.Rect{X: slot.X + pad, Y: slot.Y + pad, W: slot.W - 2*pad, H: slot.H - 2*pad}
		if inner.H <= 0 {
			continue
		}
		third := inner.H / 3
		c.Place(k.Centered(character.Name, RoleValue),
			mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: third})
		c.Place(k.Centered(character.JobName, RoleMuted),
			mobileui.Rect{X: inner.X, Y: inner.Y + third, W: inner.W, H: third})
		c.Place(k.Centered("Lv "+strconv.Itoa(character.Level), RoleMuted),
			mobileui.Rect{X: inner.X, Y: inner.Y + 2*third, W: inner.W, H: third})
	}
}
