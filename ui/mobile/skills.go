package mobile

import (
	"strconv"

	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
)

// SkillsTree builds the skills screen: a scrollable list of learned skills with
// a detail panel for the selected one.
//
// Skill icons are drawn by the host from the resource path; SkillIconRects
// reports where they belong.
func (k Kit) SkillsTree(model mobileui.MobileSkillsModel, layout mobileui.SkillsLayout) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	// Like the list, the backing panel follows its content rather than filling
	// the rail.
	panel := layout.Panel
	if bottom := skillsContentBottom(layout); bottom > panel.Y && bottom < panel.Bottom() {
		panel.H = bottom - panel.Y
	}
	c.Place(k.Panel(), panel)

	k.placeSkillsHeader(c, model, layout)
	k.placeSkillList(c, model, layout)
	k.placeSkillDetail(c, model, layout)
	return c
}

// skillsContentBottom reports where the last visible element of the screen ends.
func skillsContentBottom(layout mobileui.SkillsLayout) float32 {
	bottom := layout.Header.Bottom()
	for _, r := range []mobileui.Rect{layout.Points, listSurface(layout), layout.Detail} {
		if r.H > 0 && r.Bottom() > bottom {
			bottom = r.Bottom()
		}
	}
	return bottom + 12
}

func (k Kit) placeSkillsHeader(c *Canvas, model mobileui.MobileSkillsModel, layout mobileui.SkillsLayout) {
	if layout.Header.W <= 0 {
		return
	}
	c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)
	if layout.CharacterButton.W > 0 {
		c.Place(k.Button("Status", ButtonNormal), layout.CharacterButton)
	}
	if layout.Points.W > 0 {
		c.Place(k.Centered(skillPointsLabel(model.Points), RoleValue), layout.Points)
	}
}

func skillPointsLabel(points int) string {
	if points <= 0 {
		return "Skills"
	}
	return "Skills — " + strconv.Itoa(points) + " points"
}

func (k Kit) placeSkillList(c *Canvas, model mobileui.MobileSkillsModel, layout mobileui.SkillsLayout) {
	if layout.ListViewport.W <= 0 || layout.ListViewport.H <= 0 {
		return
	}
	// The list surface hugs its rows. The layout hands out the whole remaining
	// height so a long list can scroll, but painting all of it for six skills
	// leaves a tall blank sheet over the world.
	c.Place(k.Panel(), listSurface(layout))

	pad := k.Theme.Metrics.TableCellPadX
	touch := k.Theme.Metrics.MinTouchTarget
	for i, row := range layout.Rows {
		if i >= len(model.Skills) {
			break
		}
		// Rows scroll; anything outside the viewport is not drawn.
		if !row.Intersects(layout.ListViewport) {
			continue
		}
		skill := model.Skills[i]
		selected := model.Selection.HasSelection && model.Selection.SelectedIndex == i
		c.Place(k.Card(selected), row)

		// The icon occupies a square at the row's leading edge; the host draws
		// the real sprite there.
		iconW := row.H
		textX := row.X + iconW + pad
		textW := row.Right() - pad - textX
		if skill.Upgradable {
			textW -= touch + pad
		}
		if textW <= 0 {
			continue
		}
		half := row.H / 2
		c.Place(k.Content(skill.Name, RoleValue), mobileui.Rect{X: textX, Y: row.Y, W: textW, H: half})
		c.Place(k.Text(skillLevelLabel(skill), RoleMuted), mobileui.Rect{X: textX, Y: row.Y + half, W: textW, H: half})

		if skill.Upgradable {
			c.Place(k.Button("+", ButtonNormal), mobileui.Rect{
				X: row.Right() - pad - touch, Y: row.Y + (row.H-touch)/2, W: touch, H: touch,
			})
		}
	}
}

// listSurface bounds the list panel to the rows actually present, never
// exceeding the scrollable viewport it was given.
func listSurface(layout mobileui.SkillsLayout) mobileui.Rect {
	surface := layout.ListViewport
	var bottom float32
	for _, row := range layout.Rows {
		if row.Intersects(layout.ListViewport) && row.Bottom() > bottom {
			bottom = row.Bottom()
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

func skillLevelLabel(skill mobileui.MobileSkillModel) string {
	out := "Lv " + strconv.Itoa(skill.Level) + " / " + strconv.Itoa(skill.MaxLevel)
	if skill.SPCost > 0 {
		out += "   SP " + strconv.Itoa(skill.SPCost)
	}
	return out
}

func (k Kit) placeSkillDetail(c *Canvas, model mobileui.MobileSkillsModel, layout mobileui.SkillsLayout) {
	if layout.Detail.W <= 0 || layout.Detail.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.Detail)

	skill, ok := selectedSkill(model)
	pad := k.Theme.Metrics.TableCellPadX
	rowH := k.Theme.Metrics.TableRowHeight
	inner := mobileui.Rect{
		X: layout.Detail.X + pad, Y: layout.Detail.Y + pad,
		W: layout.Detail.W - 2*pad, H: layout.Detail.H - 2*pad,
	}
	if !ok {
		c.Place(k.Centered("Select a skill", RoleMuted), inner)
		return
	}

	y := inner.Y
	c.Place(k.Wrapped(skill.Name, RoleTitle, 2), mobileui.Rect{X: inner.X, Y: y, W: inner.W, H: rowH})
	y += rowH
	c.Place(k.Text(skillLevelLabel(skill), RoleMuted), mobileui.Rect{X: inner.X, Y: y, W: inner.W, H: rowH})
	y += rowH

	// Why a skill cannot be used outranks its statistics: the panel is short on
	// a phone, and dropping this line is what leaves a player stuck with a
	// control that silently does nothing.
	if skill.DisabledReason != "" && y+rowH <= inner.Bottom() {
		c.Place(k.Wrapped(skill.DisabledReason, RoleMuted, 2),
			mobileui.Rect{X: inner.X, Y: y, W: inner.W, H: rowH})
		y += rowH
	}

	for _, row := range skillDetailRows(skill) {
		if y+rowH > inner.Bottom() {
			break
		}
		c.Place(k.Text(row[0], RoleLabel), mobileui.Rect{X: inner.X, Y: y, W: inner.W * 0.4, H: rowH})
		c.Place(k.Text(row[1], RoleValue), mobileui.Rect{X: inner.X + inner.W*0.4, Y: y, W: inner.W * 0.6, H: rowH})
		y += rowH
	}
}

func skillDetailRows(skill mobileui.MobileSkillModel) [][2]string {
	rows := [][2]string{}
	if skill.SPCost > 0 {
		rows = append(rows, [2]string{"SP cost", strconv.Itoa(skill.SPCost)})
	}
	if skill.Range > 0 {
		rows = append(rows, [2]string{"Range", strconv.Itoa(skill.Range)})
	}
	rows = append(rows, [2]string{"Target", skillTargetLabel(skill)})
	return rows
}

func skillTargetLabel(skill mobileui.MobileSkillModel) string {
	switch skill.TargetMode {
	case input.SkillTargetActor:
		return "Actor"
	case input.SkillTargetGround:
		return "Ground"
	default:
		return "Self"
	}
}

func selectedSkill(model mobileui.MobileSkillsModel) (mobileui.MobileSkillModel, bool) {
	// HasSelection must be honoured: SelectedIndex defaults to 0, so ignoring it
	// would show the first skill as selected before the player picks anything.
	if !model.Selection.HasSelection {
		return mobileui.MobileSkillModel{}, false
	}
	index := model.Selection.SelectedIndex
	if index < 0 || index >= len(model.Skills) {
		return mobileui.MobileSkillModel{}, false
	}
	return model.Skills[index], true
}

// SkillIconRects reports where skill sprites belong, in list order.
func SkillIconRects(model mobileui.MobileSkillsModel, layout mobileui.SkillsLayout) []SkillIconPlacement {
	out := make([]SkillIconPlacement, 0, len(layout.Rows))
	for i, row := range layout.Rows {
		if i >= len(model.Skills) || !row.Intersects(layout.ListViewport) {
			continue
		}
		out = append(out, SkillIconPlacement{
			Skill: model.Skills[i],
			Rect:  mobileui.Rect{X: row.X, Y: row.Y, W: row.H, H: row.H},
		})
	}
	return out
}

// SkillIconPlacement pairs a skill with the square its sprite belongs in.
type SkillIconPlacement struct {
	Skill mobileui.MobileSkillModel
	Rect  mobileui.Rect
}
