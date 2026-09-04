package mobile

import (
	"strconv"

	"github.com/kivutar/goro/mobileui"
)

// CharacterTree builds the status screen: identity and vitals, level progress,
// the primary stats with their raise buttons, and derived combat figures.
func (k Kit) CharacterTree(model mobileui.MobileCharacterModel, layout mobileui.CharacterLayout) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	// The backing panel hugs its content rather than filling the rail: a
	// full-height white sheet below the last card reads as a broken screen and
	// hides the world the UI floats over.
	panel := layout.Panel
	if bottom := contentBottom(layout); bottom > panel.Y && bottom < panel.Bottom() {
		panel.H = bottom - panel.Y
	}
	c.Place(k.Panel(), panel)

	k.placeCharacterHeader(c, model, layout)
	k.placeVitals(c, model, layout.Vitals)
	k.placeProgress(c, model, layout.Progress)
	k.placeStats(c, model, layout.Stats)
	k.placeCombat(c, model, layout.Combat)
	return c
}

// contentBottom reports where the status screen's last placed card ends.
func contentBottom(layout mobileui.CharacterLayout) float32 {
	bottom := layout.Header.Bottom()
	for _, r := range []mobileui.Rect{layout.Vitals, layout.Progress, layout.Stats, layout.Combat, layout.SkillsButton} {
		if r.H > 0 && r.Bottom() > bottom {
			bottom = r.Bottom()
		}
	}
	return bottom + 12
}

func (k Kit) placeCharacterHeader(c *Canvas, model mobileui.MobileCharacterModel, layout mobileui.CharacterLayout) {
	if layout.Header.W <= 0 {
		return
	}
	pad := k.Theme.Metrics.TableCellPadX
	c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.BackButton)
	if layout.SkillsButton.W > 0 {
		c.Place(k.Button("Skills", ButtonNormal), layout.SkillsButton)
	}

	// Name over job/level, filling the header between the back button and
	// whatever else shares that row. In the stacked layout the Skills button
	// sits at the bottom of the screen instead, so it must not bound the title.
	left := layout.BackButton.Right() + pad
	right := layout.Header.Right() - pad
	if layout.SkillsButton.W > 0 && layout.SkillsButton.Y < layout.Header.Bottom() {
		right = layout.SkillsButton.X - pad
	}
	if right <= left {
		return
	}
	title := mobileui.Rect{X: left, Y: layout.Header.Y, W: right - left, H: layout.Header.H}
	half := title.H / 2
	c.Place(k.Centered(model.Name, RoleTitle), mobileui.Rect{X: title.X, Y: title.Y, W: title.W, H: half})
	c.Place(k.Centered(jobLine(model), RoleMuted), mobileui.Rect{X: title.X, Y: title.Y + half, W: title.W, H: half})
}

func jobLine(model mobileui.MobileCharacterModel) string {
	return model.JobName + "  Lv " + strconv.Itoa(model.BaseLevel) + " / Job " + strconv.Itoa(model.JobLevel)
}

// placeVitals draws the HP and SP meters with their values.
func (k Kit) placeVitals(c *Canvas, model mobileui.MobileCharacterModel, area mobileui.Rect) {
	if area.W <= 0 || area.H <= 0 {
		return
	}
	c.Place(k.Panel(), area)
	rows := k.meterRows(area, 2)
	k.placeMeter(c, rows[0], "HP", model.HP, model.MaxHP, BarHP)
	k.placeMeter(c, rows[1], "SP", model.SP, model.MaxSP, BarSP)
}

// placeProgress draws base and job experience.
func (k Kit) placeProgress(c *Canvas, model mobileui.MobileCharacterModel, area mobileui.Rect) {
	if area.W <= 0 || area.H <= 0 {
		return
	}
	c.Place(k.Panel(), area)
	rows := k.meterRows(area, 3)
	k.placeMeter64(c, rows[0], "Base", model.BaseExp, model.NextBaseExp, BarBaseEXP)
	k.placeMeter64(c, rows[1], "Job", model.JobExp, model.NextJobExp, BarJobEXP)

	// Weight and zeny share the last row: both are capacity figures a player
	// checks alongside progress.
	pad := k.Theme.Metrics.TableCellPadX
	half := (rows[2].W - pad) / 2
	c.Place(k.Text("Weight "+strconv.Itoa(model.Weight)+" / "+strconv.Itoa(model.MaxWeight), RoleMuted),
		mobileui.Rect{X: rows[2].X, Y: rows[2].Y, W: half, H: rows[2].H})
	c.Place(k.Text(zenyLabel(model.Zeny), RoleMuted),
		mobileui.Rect{X: rows[2].X + half + pad, Y: rows[2].Y, W: half, H: rows[2].H})
}

// meterRows splits a panel into n evenly spaced inset rows.
func (k Kit) meterRows(area mobileui.Rect, n int) []mobileui.Rect {
	pad := k.Theme.Metrics.TableCellPadX
	inner := mobileui.Rect{X: area.X + pad, Y: area.Y + pad, W: area.W - 2*pad, H: area.H - 2*pad}
	rows := make([]mobileui.Rect, 0, n)
	h := inner.H / float32(n)
	for i := 0; i < n; i++ {
		rows = append(rows, mobileui.Rect{X: inner.X, Y: inner.Y + float32(i)*h, W: inner.W, H: h})
	}
	return rows
}

func (k Kit) placeMeter(c *Canvas, row mobileui.Rect, label string, current, max int, kind BarKind) {
	k.placeMeter64(c, row, label, int64(current), int64(max), kind)
}

func (k Kit) placeMeter64(c *Canvas, row mobileui.Rect, label string, current, max int64, kind BarKind) {
	pad := k.Theme.Metrics.TableCellPadX
	labelW := row.W * 0.16
	valueW := row.W * 0.30
	barW := row.W - labelW - valueW - 2*pad
	if barW <= 0 {
		return
	}
	// The bar is thinner than the row so the label and value read on the same
	// baseline rather than being squashed by a full-height meter.
	barH := row.H * 0.45
	barY := row.Y + (row.H-barH)/2

	c.Place(k.Text(label, RoleLabel), mobileui.Rect{X: row.X, Y: row.Y, W: labelW, H: row.H})
	c.Place(k.Bar(kind, fraction(current, max)),
		mobileui.Rect{X: row.X + labelW + pad, Y: barY, W: barW, H: barH})
	c.Place(k.Text(meterValue(current, max), RoleValue),
		mobileui.Rect{X: row.X + labelW + pad + barW + pad, Y: row.Y, W: valueW, H: row.H})
}

// meterValue formats a current/maximum pair for the narrow column beside a
// meter. Experience runs to seven digits or more, which does not fit there
// grouped, so large numbers are abbreviated instead of being truncated.
func meterValue(current, max int64) string {
	if max >= 100000 || current >= 100000 {
		return compactNumber(current) + " / " + compactNumber(max)
	}
	return groupDigits(current) + " / " + groupDigits(max)
}

// compactNumber renders a magnitude in at most five characters: 1.2M, 345k, 87.
func compactNumber(v int64) string {
	negative := v < 0
	if negative {
		v = -v
	}
	var out string
	switch {
	case v >= 1_000_000_000:
		out = oneDecimal(v, 1_000_000_000) + "B"
	case v >= 1_000_000:
		out = oneDecimal(v, 1_000_000) + "M"
	case v >= 1_000:
		out = oneDecimal(v, 1_000) + "k"
	default:
		out = strconv.FormatInt(v, 10)
	}
	if negative {
		return "-" + out
	}
	return out
}

// oneDecimal renders v/unit with a single decimal place, dropping a trailing
// ".0" so round values stay short.
func oneDecimal(v, unit int64) string {
	whole := v / unit
	tenths := (v % unit) * 10 / unit
	if tenths == 0 || whole >= 100 {
		return strconv.FormatInt(whole, 10)
	}
	return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(tenths, 10)
}

func fraction(current, max int64) float32 {
	if max <= 0 {
		return 0
	}
	return float32(current) / float32(max)
}

// placeStats lists the primary stats. Each row shows the value, any equipment
// bonus, and a raise button when points are available.
func (k Kit) placeStats(c *Canvas, model mobileui.MobileCharacterModel, area mobileui.Rect) {
	if area.W <= 0 || area.H <= 0 || len(model.Stats) == 0 {
		return
	}
	c.Place(k.Panel(), area)
	pad := k.Theme.Metrics.TableCellPadX
	rowH := k.Theme.Metrics.TableRowHeight
	y := area.Y + pad

	header := "Stats"
	if model.StatPoints > 0 {
		header += "  (" + strconv.Itoa(model.StatPoints) + " points)"
	}
	c.Place(k.Text(header, RoleTitle), mobileui.Rect{X: area.X + pad, Y: y, W: area.W - 2*pad, H: rowH})
	y += rowH

	touch := k.Theme.Metrics.MinTouchTarget
	for _, stat := range model.Stats {
		if y+rowH > area.Bottom()-pad {
			return
		}
		inner := area.W - 2*pad
		labelW := inner * 0.30
		valueW := inner * 0.30
		c.Place(k.Text(stat.Label, RoleLabel), mobileui.Rect{X: area.X + pad, Y: y, W: labelW, H: rowH})
		c.Place(k.Text(statValue(stat), RoleValue), mobileui.Rect{X: area.X + pad + labelW, Y: y, W: valueW, H: rowH})

		// The raise button sits beside the value rather than at the far edge, so
		// it stays reachable and visibly belongs to its row.
		if stat.CanAdd {
			btnX := area.X + pad + labelW + valueW + pad
			if btnX+touch <= area.Right()-pad {
				c.Place(k.Button("+", ButtonNormal), mobileui.Rect{X: btnX, Y: y, W: touch, H: rowH})
				c.Place(k.Text(strconv.Itoa(stat.Cost), RoleMuted),
					mobileui.Rect{X: btnX + touch + pad, Y: y, W: area.Right() - pad - btnX - touch - pad, H: rowH})
			}
		}
		y += rowH
	}
}

func statValue(stat mobileui.CharacterStatModel) string {
	out := strconv.Itoa(stat.Value)
	if stat.Bonus != 0 {
		out += " + " + strconv.Itoa(stat.Bonus)
	}
	return out
}

// placeCombat lists the derived combat figures as label/value pairs.
func (k Kit) placeCombat(c *Canvas, model mobileui.MobileCharacterModel, area mobileui.Rect) {
	if area.W <= 0 || area.H <= 0 {
		return
	}
	c.Place(k.Panel(), area)
	pad := k.Theme.Metrics.TableCellPadX
	rowH := k.Theme.Metrics.TableRowHeight

	rows := [][2]string{
		{"ATK", withBonus(model.Attack, model.AttackBonus)},
		{"DEF", withBonus(model.Defense, model.DefenseBonus)},
		{"MDEF", withBonus(model.MDefense, model.MDefenseBonus)},
		{"HIT", strconv.Itoa(model.Hit)},
		{"FLEE", withBonus(model.Flee, model.FleeBonus)},
		{"CRIT", strconv.Itoa(model.Critical)},
		{"ASPD", withBonus(model.ASPD, model.ASPDBonus)},
	}

	y := area.Y + pad
	c.Place(k.Text("Combat", RoleTitle), mobileui.Rect{X: area.X + pad, Y: y, W: area.W - 2*pad, H: rowH})
	y += rowH

	// Two columns keep the list compact instead of stretching one value per row
	// across the whole panel.
	inner := area.W - 2*pad
	colW := inner / 2
	for i, row := range rows {
		col := i % 2
		rowY := y + float32(i/2)*rowH
		if rowY+rowH > area.Bottom()-pad {
			break
		}
		x := area.X + pad + float32(col)*colW
		c.Place(k.Text(row[0], RoleLabel), mobileui.Rect{X: x, Y: rowY, W: colW * 0.45, H: rowH})
		c.Place(k.Text(row[1], RoleValue), mobileui.Rect{X: x + colW*0.45, Y: rowY, W: colW * 0.55, H: rowH})
	}
}

func withBonus(value, bonus int) string {
	if bonus == 0 {
		return strconv.Itoa(value)
	}
	return strconv.Itoa(value) + " + " + strconv.Itoa(bonus)
}
