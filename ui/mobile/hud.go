package mobile

import (
	"strconv"

	"github.com/gogpu/ui/widget"

	"github.com/kivutar/goro/mobileui"
)

// HUDTree builds the world heads-up display: the player and target status
// panels, the loot rail, the minimap, the skill bar, the chat strip and the
// menu drawer.
//
// Unlike the full-screen surfaces this is drawn over live gameplay every frame,
// so it paints only what the layout actually reserved — no backing sheet, and
// nothing for a section the layout gave zero size.
//
// Skill and loot icons come from the resource path; HUDIconRects reports where.
func (k Kit) HUDTree(
	model mobileui.MobileHUDModel,
	layout mobileui.HUDLayout,
	nav mobileui.Navigation,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)

	k.placePlayerPanel(c, model.Player, layout.PlayerPanel)
	k.placeTargetPanel(c, model.Target, layout.TargetPanel)
	k.placeStatusEffects(c, model.Statuses, layout.StatusArea)
	k.placeMinimap(c, model.Minimap, layout.Minimap)
	k.placeLoot(c, model.Loot, layout)
	k.placeSkillBar(c, model.Skills, layout, nav)
	k.placeChatBar(c, layout)
	k.placeCombatBanner(c, layout, nav)
	k.placeMenu(c, layout, nav)
	return c
}

// placePlayerPanel shows name, level and the two vitals a player watches
// constantly.
func (k Kit) placePlayerPanel(c *Canvas, player mobileui.PlayerHUDModel, area mobileui.Rect) {
	if area.W <= 0 || area.H <= 0 {
		return
	}
	c.Place(k.Panel(), area)
	pad := k.Theme.Metrics.TableCellPadX
	inner := insetRect(area, pad)
	if inner.H <= 0 {
		return
	}

	// Name and level share the first of three rows; HP and SP take the rest.
	rowH := inner.H / 3
	levelW := inner.W * 0.34
	c.Place(k.Content(player.Name, RoleValue),
		mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W - levelW, H: rowH})
	c.Place(k.RightAligned(levelLine(player), RoleMuted),
		mobileui.Rect{X: inner.X + inner.W - levelW, Y: inner.Y, W: levelW, H: rowH})

	k.placeCompactMeter(c, mobileui.Rect{X: inner.X, Y: inner.Y + rowH, W: inner.W, H: rowH},
		BarHP, int64(player.HP), int64(player.MaxHP))
	k.placeCompactMeter(c, mobileui.Rect{X: inner.X, Y: inner.Y + 2*rowH, W: inner.W, H: rowH},
		BarSP, int64(player.SP), int64(player.MaxSP))
}

func levelLine(player mobileui.PlayerHUDModel) string {
	return "Lv " + strconv.Itoa(player.BaseLevel) + "/" + strconv.Itoa(player.JobLevel)
}

// placeCompactMeter is the HUD's meter: a bar with its numbers in a column
// beside it, so a vital costs one row rather than two.
//
// The numbers sit next to the bar rather than on top of it. Overlaying them
// puts dark text over the SP fill, where it is effectively unreadable, and the
// contrast changes as the bar drains.
func (k Kit) placeCompactMeter(c *Canvas, row mobileui.Rect, kind BarKind, current, max int64) {
	if row.W <= 0 || row.H <= 0 {
		return
	}
	pad := k.Theme.Metrics.TableGap * 2
	valueW := row.W * 0.34
	barW := row.W - valueW - pad
	if barW <= 0 {
		return
	}
	barH := min32(row.H*0.62, row.H)
	c.Place(k.Bar(kind, fraction(current, max)),
		mobileui.Rect{X: row.X, Y: row.Y + (row.H-barH)/2, W: barW, H: barH})
	c.Place(k.RightAligned(compactNumber(current)+"/"+compactNumber(max), RoleMuted),
		mobileui.Rect{X: row.X + barW + pad, Y: row.Y, W: valueW, H: row.H})
}

// placeTargetPanel is only drawn when something is targeted.
func (k Kit) placeTargetPanel(c *Canvas, target mobileui.TargetHUDModel, area mobileui.Rect) {
	if !target.Visible || area.W <= 0 || area.H <= 0 {
		return
	}
	c.Place(k.Panel(), area)
	inner := insetRect(area, k.Theme.Metrics.TableCellPadX)
	if inner.H <= 0 {
		return
	}
	half := inner.H / 2
	c.Place(k.Content(target.Name, RoleValue), mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: half})
	k.placeCompactMeter(c, mobileui.Rect{X: inner.X, Y: inner.Y + half, W: inner.W, H: half},
		BarHP, int64(target.HP), int64(target.MaxHP))
}

// placeStatusEffects lays buffs and debuffs out as a row of small squares.
func (k Kit) placeStatusEffects(c *Canvas, statuses []mobileui.StatusEffectModel, area mobileui.Rect) {
	if len(statuses) == 0 || area.W <= 0 || area.H <= 0 {
		return
	}
	side := min32(area.H, area.W)
	gap := k.Theme.Metrics.TableGap * 2
	x := area.X
	for range statuses {
		if x+side > area.Right() {
			return
		}
		c.Place(k.Slot(true, false), mobileui.Rect{X: x, Y: area.Y, W: side, H: side})
		x += side + gap
	}
}

func (k Kit) placeMinimap(c *Canvas, minimap mobileui.MinimapModel, area mobileui.Rect) {
	if !minimap.Visible || area.W <= 0 || area.H <= 0 {
		return
	}
	// The raster itself is drawn by the host from live map data; this is the
	// frame around it plus the map's name.
	c.Place(k.Panel(), area)
	labelH := min32(k.Theme.Metrics.TableRowHeight*0.6, area.H*0.24)
	c.Place(k.Content(minimap.MapName, RoleMuted).Align(widget.TextAlignCenter),
		mobileui.Rect{X: area.X, Y: area.Bottom() - labelH, W: area.W, H: labelH})
}

// placeLoot lists nearby drops as tappable rows.
func (k Kit) placeLoot(c *Canvas, loot []mobileui.LootItemModel, layout mobileui.HUDLayout) {
	if len(layout.LootRows) == 0 || len(loot) == 0 {
		return
	}
	if layout.LootPanel.W > 0 && layout.LootPanel.H > 0 {
		c.Place(k.Panel(), layout.LootPanel)
	}
	pad := k.Theme.Metrics.TableCellPadX
	for i, row := range layout.LootRows {
		if i >= len(loot) || row.W <= 0 || row.H <= 0 {
			break
		}
		item := loot[i]
		c.Place(k.Card(false), row)
		// A square at the leading edge holds the item sprite.
		textX := row.X + row.H + pad
		textW := row.Right() - pad - textX
		if textW <= 0 {
			continue
		}
		c.Place(k.Content(lootLabel(item), RoleBody), mobileui.Rect{X: textX, Y: row.Y, W: textW, H: row.H})
	}
}

func lootLabel(item mobileui.LootItemModel) string {
	if item.Quantity > 1 {
		return item.Name + " x" + strconv.Itoa(item.Quantity)
	}
	return item.Name
}

// placeSkillBar draws the hotbar plus its paging controls.
func (k Kit) placeSkillBar(
	c *Canvas,
	skills []mobileui.SkillSlotModel,
	layout mobileui.HUDLayout,
	nav mobileui.Navigation,
) {
	if len(layout.SkillSlots) == 0 {
		return
	}
	for i, slot := range layout.SkillSlots {
		if slot.W <= 0 || slot.H <= 0 {
			continue
		}
		index := layout.SkillStart + i
		if index >= len(skills) {
			// An empty hotbar position still reads as a slot.
			c.Place(k.Slot(false, false), slot)
			continue
		}
		skill := skills[index]
		targeting := nav.Targeting.Mode != 0 && nav.Targeting.SkillID == skill.SkillID
		c.Place(k.Slot(true, targeting), slot)

		// Level sits along the bottom of the slot, out of the sprite's way; a
		// skill that cannot be used right now is shown muted rather than hidden.
		role := RoleMuted
		if !skill.Usable {
			role = RoleLabel
		}
		c.Place(k.Centered("Lv"+strconv.Itoa(skill.Level), role),
			quantityBadgeRect(slot, k.Theme.Metrics.TableCellPadX))
	}
	if layout.SkillPagePrev.W > 0 {
		c.Place(k.Button("‹", ButtonNormal), layout.SkillPagePrev)
	}
	if layout.SkillPageNext.W > 0 {
		c.Place(k.Button("›", ButtonNormal), layout.SkillPageNext)
	}
}

func (k Kit) placeChatBar(c *Canvas, layout mobileui.HUDLayout) {
	if layout.ChatBar.W <= 0 || layout.ChatBar.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.ChatBar)
	if layout.ChatButton.W > 0 {
		c.Place(k.Button("Open", ButtonNormal), layout.ChatButton)
	}
	// The layout reserves a narrow "Chat" caption beside the prompt, sized for
	// the old bitmap text; at mobile text size the two overlap. The Open button
	// already names the action, so the prompt takes the whole strip instead.
	pad := k.Theme.Metrics.TableCellPadX
	prompt := mobileui.Rect{X: layout.ChatBar.X + pad, Y: layout.ChatBar.Y, H: layout.ChatBar.H}
	right := layout.ChatBar.Right() - pad
	if layout.ChatButton.W > 0 {
		right = layout.ChatButton.X - pad
	}
	prompt.W = max32(0, right-prompt.X)
	if prompt.W > 0 {
		c.Place(k.Text("Say something…", RoleMuted), prompt)
	}
}

// placeCombatBanner explains an active skill target prompt and offers a way out
// of it.
func (k Kit) placeCombatBanner(c *Canvas, layout mobileui.HUDLayout, nav mobileui.Navigation) {
	if layout.CombatBanner.W <= 0 || layout.CombatBanner.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.CombatBanner)
	prompt := mobileui.TargetingPrompt(nav.Targeting.Mode)
	if prompt == "" {
		return
	}
	inner := insetRect(layout.CombatBanner, k.Theme.Metrics.TableCellPadX)
	if layout.CombatCancel.W > 0 {
		inner.W = max32(0, layout.CombatCancel.X-inner.X-k.Theme.Metrics.TableCellPadX)
		c.Place(k.Button("Cancel", ButtonNormal), layout.CombatCancel)
	}
	c.Place(k.Wrapped(prompt, RoleBody, 2), inner)
}

// placeMenu draws the navigation drawer when it is open.
func (k Kit) placeMenu(c *Canvas, layout mobileui.HUDLayout, nav mobileui.Navigation) {
	if layout.Menu.W > 0 && layout.Menu.H > 0 {
		// A square 72-pixel control cannot hold the word "Menu" at mobile text
		// size, so it carries the conventional menu glyph instead.
		c.Place(k.Button("≡", buttonStateForOpen(nav.MenuOpen)), layout.Menu)
	}
	if !nav.MenuOpen || len(layout.MenuActions) == 0 {
		return
	}
	if layout.MenuPanel.W > 0 && layout.MenuPanel.H > 0 {
		c.Place(k.Panel(), layout.MenuPanel)
	}
	for _, action := range layout.MenuActions {
		if action.Rect.W <= 0 || action.Rect.H <= 0 {
			continue
		}
		c.Place(k.Button(action.Screen.String(), buttonStateForOpen(action.Screen == nav.Screen)), action.Rect)
	}
}

func buttonStateForOpen(active bool) ButtonState {
	if active {
		return ButtonPressed
	}
	return ButtonNormal
}

// HUDIconRects reports where the host must draw skill and loot sprites.
func HUDIconRects(
	model mobileui.MobileHUDModel,
	layout mobileui.HUDLayout,
) (skills []SkillIconPlacement, loot []IconPlacement) {
	for i, slot := range layout.SkillSlots {
		index := layout.SkillStart + i
		if index >= len(model.Skills) || slot.W <= 0 {
			continue
		}
		skills = append(skills, SkillIconPlacement{
			Skill: mobileui.MobileSkillModel{
				SkillID: model.Skills[index].SkillID,
				Name:    model.Skills[index].Name,
				Level:   model.Skills[index].Level,
				IconKey: model.Skills[index].IconKey,
			},
			Rect: slot,
		})
	}
	for i, row := range layout.LootRows {
		if i >= len(model.Loot) || row.W <= 0 {
			break
		}
		item := model.Loot[i]
		loot = append(loot, IconPlacement{
			Item: mobileui.InventoryItemModel{
				ItemID:      item.ItemID,
				Identified:  item.Identified,
				DisplayName: item.Name,
				Quantity:    item.Quantity,
			},
			Rect: mobileui.Rect{X: row.X, Y: row.Y, W: row.H, H: row.H},
		})
	}
	return skills, loot
}

func insetRect(r mobileui.Rect, pad float32) mobileui.Rect {
	return mobileui.Rect{X: r.X + pad, Y: r.Y + pad, W: r.W - 2*pad, H: r.H - 2*pad}
}
