package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// profileStatLabels names the six starter stats in the order the model stores
// them.
var profileStatLabels = [6]string{"STR", "AGI", "VIT", "INT", "DEX", "LUK"}

// ProfileTree builds the offline character screen: a preview, the identity and
// appearance controls, the starter stat allocation, and the on-screen keyboard
// while a name is being typed.
func (k Kit) ProfileTree(model mobileui.MobileProfileModel, layout mobileui.MobileProfileLayout) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	// The sheet follows its content rather than filling the rail.
	panel := layout.Panel
	if bottom := profileContentBottom(layout); bottom > panel.Y && bottom < panel.Bottom() {
		panel.H = bottom - panel.Y
	}
	c.Place(k.Panel(), panel)

	k.placeProfileHeader(c, layout)
	if layout.Preview.W > 0 && layout.Preview.H > 0 {
		// The character sprite itself is drawn by the host from the resource
		// path over this frame; ProfilePreviewRect reports where.
		c.Place(k.Panel(), layout.Preview)
	}
	k.placeProfileIdentity(c, model, layout)
	k.placeProfileAppearance(c, model, layout)
	k.placeProfileStats(c, model, layout)
	k.placeProfileActions(c, model, layout)

	if layout.Notice.W > 0 && model.Notice != "" {
		c.Place(k.Wrapped(model.Notice, RoleMuted, 2), layout.Notice)
	}
	k.placeProfileKeyboard(c, layout)
	return c
}

// profileContentBottom reports where the screen's last element ends.
func profileContentBottom(layout mobileui.MobileProfileLayout) float32 {
	bottom := layout.Header.Bottom()
	rects := []mobileui.Rect{layout.Preview, layout.Identity, layout.Appearance, statsCardSurface(layout),
		layout.EditButton, layout.NewButton, layout.ContinueButton,
		layout.SaveButton, layout.CancelButton, layout.Keyboard, layout.Notice}
	for _, row := range layout.StatRows {
		rects = append(rects, row)
	}
	for _, r := range rects {
		if r.H > 0 && r.Bottom() > bottom {
			bottom = r.Bottom()
		}
	}
	return bottom + 12
}

func (k Kit) placeProfileHeader(c *Canvas, layout mobileui.MobileProfileLayout) {
	if layout.BackButton.W > 0 {
		c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)
	}
	if layout.HeaderTitle.W > 0 {
		c.Place(k.Centered("Character", RoleTitle), layout.HeaderTitle)
	}
}

func (k Kit) placeProfileIdentity(
	c *Canvas,
	model mobileui.MobileProfileModel,
	layout mobileui.MobileProfileLayout,
) {
	if layout.Identity.W > 0 && layout.Identity.H > 0 {
		c.Place(k.Panel(), layout.Identity)
	}
	if layout.NameLabel.W > 0 {
		c.Place(k.Text("Name", RoleLabel), layout.NameLabel)
	}
	if layout.NameField.W > 0 {
		c.Place(k.Card(layout.EditingName), layout.NameField)
		pad := k.Theme.Metrics.TableCellPadX
		inner := mobileui.Rect{X: layout.NameField.X + pad, Y: layout.NameField.Y,
			W: layout.NameField.W - 2*pad, H: layout.NameField.H}
		name := model.Name
		if name == "" {
			name = "Unnamed"
		}
		c.Place(k.Text(name, RoleValue), inner)
	}
	if layout.SexLabel.W > 0 {
		// Outside the editor the value reads as a row, not a control: the
		// editor's SexButton sits below the panel it belongs to and would be
		// clipped away here.
		if layout.Editor {
			c.Place(k.Text("Sex", RoleLabel), layout.SexLabel)
		} else {
			k.placeValueRow(c, layout.SexLabel, "Sex", sexLabel(model))
		}
	}
	if layout.Editor && layout.SexButton.W > 0 {
		c.Place(k.Button(sexLabel(model), ButtonNormal), layout.SexButton)
	}
}

// placeValueRow renders a label with its value pushed to the trailing edge,
// the shape the desktop client uses for read-only character facts.
func (k Kit) placeValueRow(c *Canvas, row mobileui.Rect, label, value string) {
	if row.W <= 0 || row.H <= 0 {
		return
	}
	valueW := row.W * 0.55
	c.Place(k.Text(label, RoleLabel), mobileui.Rect{X: row.X, Y: row.Y, W: row.W - valueW, H: row.H})
	c.Place(k.RightAligned(value, RoleValue),
		mobileui.Rect{X: row.X + row.W - valueW, Y: row.Y, W: valueW, H: row.H})
}

func sexLabel(model mobileui.MobileProfileModel) string {
	if model.SexLabel != "" {
		return model.SexLabel
	}
	if model.Sex == 0 {
		return "Female"
	}
	return "Male"
}

func (k Kit) placeProfileAppearance(
	c *Canvas,
	model mobileui.MobileProfileModel,
	layout mobileui.MobileProfileLayout,
) {
	if layout.Appearance.W > 0 && layout.Appearance.H > 0 {
		c.Place(k.Panel(), layout.Appearance)
	}
	if !layout.Editor {
		// A read-only appearance is two value rows, which fit the short panel
		// the layout reserves when the editor is closed.
		row := layout.AppearanceLabel
		if row.W <= 0 {
			return
		}
		k.placeValueRow(c, row, "Hair style", strconv.Itoa(model.HairStyle))
		k.placeValueRow(c, mobileui.Rect{X: row.X, Y: row.Y + row.H + 8, W: row.W, H: row.H},
			"Hair colour", strconv.Itoa(model.HairColor))
		return
	}
	if layout.AppearanceLabel.W > 0 {
		c.Place(k.Text("Hair", RoleLabel), layout.AppearanceLabel)
	}
	if layout.HairPrev.W > 0 {
		c.Place(k.Button("‹", ButtonNormal), layout.HairPrev)
	}
	if layout.HairNext.W > 0 {
		c.Place(k.Button("›", ButtonNormal), layout.HairNext)
	}
	if layout.HairColor.W > 0 {
		c.Place(k.Button("Colour "+strconv.Itoa(model.HairColor), ButtonNormal), layout.HairColor)
	}
}

// placeProfileStats draws the six starter stats with their +/- controls.
func (k Kit) placeProfileStats(
	c *Canvas,
	model mobileui.MobileProfileModel,
	layout mobileui.MobileProfileLayout,
) {
	if layout.Stats.W > 0 && layout.Stats.H > 0 {
		// The card follows its rows; the layout hands it every remaining pixel
		// so the editor's controls have somewhere to go.
		c.Place(k.Panel(), statsCardSurface(layout))
	}
	pad := k.Theme.Metrics.TableCellPadX
	// A heading, so the block reads as the character's stats rather than six
	// unexplained numbers — but only when the layout left a whole row of space
	// above the first stat. Squeezing it in prints it through that row.
	rowH := k.Theme.Metrics.TableRowHeight
	if layout.Stats.W > 0 && len(layout.StatRows) > 0 &&
		layout.StatRows[0].Y-layout.Stats.Y >= rowH+pad {
		c.Place(k.Text("Stats", RoleTitle), mobileui.Rect{
			X: layout.Stats.X + pad, Y: layout.Stats.Y + pad,
			W: layout.Stats.W - 2*pad, H: rowH,
		})
	}
	for i, row := range layout.StatRows {
		if row.W <= 0 || row.H <= 0 {
			continue
		}
		minus, plus := layout.StatMinus[i], layout.StatPlus[i]
		// The label and value share the space the buttons leave.
		right := row.Right()
		if plus.W > 0 {
			right = plus.X - pad
		}
		inner := mobileui.Rect{X: row.X + pad, Y: row.Y, W: right - (row.X + pad), H: row.H}
		if inner.W > 0 {
			valueW := inner.W * 0.4
			c.Place(k.Text(profileStatLabels[i], RoleLabel),
				mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W - valueW, H: inner.H})
			c.Place(k.RightAligned(strconv.Itoa(int(model.Stats[i])), RoleValue),
				mobileui.Rect{X: inner.X + inner.W - valueW, Y: inner.Y, W: valueW, H: inner.H})
		}
		if minus.W > 0 {
			c.Place(k.Button("−", ButtonNormal), minus)
		}
		if plus.W > 0 {
			c.Place(k.Button("+", ButtonNormal), plus)
		}
	}
	if layout.StarterTotal.W > 0 {
		c.Place(k.Centered(starterTotal(model), RoleMuted), layout.StarterTotal)
	}
}

func starterTotal(model mobileui.MobileProfileModel) string {
	total := 0
	for _, stat := range model.Stats {
		total += int(stat)
	}
	return "Total " + strconv.Itoa(total)
}

func (k Kit) placeProfileActions(
	c *Canvas,
	model mobileui.MobileProfileModel,
	layout mobileui.MobileProfileLayout,
) {
	for _, action := range []struct {
		rect    mobileui.Rect
		label   string
		enabled bool
	}{
		{layout.EditButton, "Edit", model.Editable},
		{layout.NewButton, "New", true},
		{layout.ContinueButton, "Continue", model.Available},
		{layout.SaveButton, "Save", true},
		{layout.CancelButton, "Cancel", true},
	} {
		if action.rect.W > 0 && action.rect.H > 0 {
			c.Place(k.Button(action.label, buttonStateFor(action.enabled)), action.rect)
		}
	}
}

// placeProfileKeyboard draws the in-app keyboard used for the offline name.
// The mobile profile does not use the Android IME, so this is the only way to
// type here.
func (k Kit) placeProfileKeyboard(c *Canvas, layout mobileui.MobileProfileLayout) {
	if !layout.EditingName || layout.Keyboard.W <= 0 || layout.Keyboard.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.Keyboard)
	for _, key := range layout.Keys {
		if key.Rect.W <= 0 || key.Rect.H <= 0 {
			continue
		}
		c.Place(k.Button(key.Key, ButtonNormal), key.Rect)
	}
	if layout.KeyboardDone.W > 0 {
		c.Place(k.Button("Done", ButtonNormal), layout.KeyboardDone)
	}
}

// statsCardSurface bounds the stats card to the rows and total it contains.
func statsCardSurface(layout mobileui.MobileProfileLayout) mobileui.Rect {
	card := layout.Stats
	bottom := layout.StarterTotal.Bottom()
	for i, row := range layout.StatRows {
		if row.H > 0 && row.Bottom() > bottom {
			bottom = row.Bottom()
		}
		if plus := layout.StatPlus[i]; plus.H > 0 && plus.Bottom() > bottom {
			bottom = plus.Bottom()
		}
	}
	if bottom <= card.Y {
		return card
	}
	if h := bottom - card.Y + 12; h < card.H {
		card.H = h
	}
	return card
}

// ProfilePreviewRect reports where the host must draw the character sprite.
func ProfilePreviewRect(layout mobileui.MobileProfileLayout) (mobileui.Rect, bool) {
	if layout.Preview.W <= 0 || layout.Preview.H <= 0 {
		return mobileui.Rect{}, false
	}
	// Inset so the sprite sits inside the frame rather than on its border.
	pad := float32(8)
	return mobileui.Rect{
		X: layout.Preview.X + pad, Y: layout.Preview.Y + pad,
		W: layout.Preview.W - 2*pad, H: layout.Preview.H - 2*pad,
	}, true
}
