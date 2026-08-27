package mobileui

type CharacterLayout struct {
	Safe, Panel, Header, BackButton, ContentViewport Rect
	Vitals, Progress, Stats, Combat                  Rect
	SkillsButton                                     Rect
	ContentExtent, ScrollOffset                      float32
	Stacked                                          bool
}

func LayoutCharacter(viewport Viewport, model MobileCharacterModel) CharacterLayout {
	return LayoutCharacterScrolled(viewport, model, 0)
}

func LayoutCharacterScrolled(viewport Viewport, model MobileCharacterModel, offset float32) CharacterLayout {
	safe := viewport.SafeRect()
	layout := CharacterLayout{Safe: safe, ScrollOffset: maxf(0, offset)}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	profile := viewport.Profile()
	panel := centeredMobileRail(safe, 1200, 16)
	stacked := viewport.UsesStackedCards()
	if stacked {
		panel = centeredMobileRail(safe, 720, 16)
	}
	// A phone in landscape has more horizontal pixels but the same short
	// interaction height. Give its cards a little more width while retaining
	// the narrow portrait rail; the Fold profile keeps the bounded wide rail.
	if profile == LayoutLandscape {
		panel = centeredMobileRail(safe, 0, 16)
	}
	layout.Panel, layout.Stacked = panel, stacked
	headerPad := float32(12)
	if !stacked {
		headerPad = 20
	}
	layout.Header = Rect{X: layout.Panel.X + headerPad, Y: layout.Panel.Y + headerPad, W: layout.Panel.W - 2*headerPad, H: 56}
	layout.BackButton = Rect{X: layout.Header.X, Y: layout.Header.Y, W: 112, H: 52}
	contentX, contentY := layout.Panel.X+headerPad, layout.Header.Bottom()+12
	contentW := maxf(0, layout.Panel.W-2*headerPad)
	contentBottom := layout.Panel.Bottom() - 12
	layout.ContentViewport = Rect{X: contentX, Y: contentY, W: contentW, H: maxf(0, contentBottom-contentY)}
	if stacked {
		gap := float32(12)
		cursor := contentY - layout.ScrollOffset
		layout.Vitals = Rect{X: contentX, Y: cursor, W: contentW, H: 112}
		cursor = layout.Vitals.Bottom() + gap
		layout.Progress = Rect{X: contentX, Y: cursor, W: contentW, H: 100}
		cursor = layout.Progress.Bottom() + gap
		layout.Stats = Rect{X: contentX, Y: cursor, W: contentW, H: 252}
		cursor = layout.Stats.Bottom() + gap
		layout.Combat = Rect{X: contentX, Y: cursor, W: contentW, H: 340}
		cursor = layout.Combat.Bottom() + gap
		layout.SkillsButton = Rect{X: contentX, Y: cursor, W: contentW, H: 52}
		layout.ContentExtent = cursor + layout.SkillsButton.H - (contentY - layout.ScrollOffset)
		// Let short profiles hug their content instead of painting a full-height
		// white rail. Long profiles still retain the safe viewport and scroll.
		layout.ContentViewport = Rect{X: contentX, Y: contentY, W: contentW, H: maxf(0, layout.Panel.Bottom()-contentY-12)}
		return layout
	}
	layout.Vitals = Rect{X: contentX, Y: contentY, W: contentW, H: 112}
	layout.Progress = Rect{X: contentX, Y: layout.Vitals.Bottom() + 12, W: contentW, H: 100}
	columnGap := float32(16)
	columnW := maxf(0, (contentW-columnGap)/2)
	columnY := layout.Progress.Bottom() + 12
	columnH := maxf(160, contentBottom-columnY-72)
	layout.Stats = Rect{X: contentX, Y: columnY, W: columnW, H: columnH}
	layout.Combat = Rect{X: contentX + columnW + columnGap, Y: columnY, W: columnW, H: columnH}
	layout.SkillsButton = Rect{X: layout.Panel.Right() - 196, Y: layout.Panel.Bottom() - 68, W: 176, H: 52}
	layout.ContentExtent = layout.ContentViewport.H
	return layout
}

type SkillsLayout struct {
	Safe, Panel, Header, BackButton, CharacterButton Rect
	Points, ListViewport, Detail                     Rect
	Rows                                             []Rect
	Stacked                                          bool
}

func LayoutSkills(viewport Viewport, model MobileSkillsModel, offset float32) SkillsLayout {
	safe := viewport.SafeRect()
	layout := SkillsLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	profile := viewport.Profile()
	panel := centeredMobileRail(safe, 1200, 16)
	stacked := viewport.UsesStackedCards()
	if stacked {
		panel = centeredMobileRail(safe, 720, 16)
	}
	if profile == LayoutLandscape {
		panel = centeredMobileRail(safe, 0, 16)
	}
	layout.Panel, layout.Stacked = panel, stacked
	pad := float32(12)
	if !stacked {
		pad = 20
	}
	layout.Header = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + pad, W: layout.Panel.W - 2*pad, H: 56}
	layout.BackButton = Rect{X: layout.Header.X, Y: layout.Header.Y, W: 112, H: 52}
	layout.CharacterButton = Rect{X: layout.Header.Right() - 156, Y: layout.Header.Y, W: 144, H: 52}
	contentY := layout.Header.Bottom() + 16
	contentH := maxf(0, layout.Panel.Bottom()-contentY-20)
	if stacked {
		contentW := maxf(0, layout.Panel.W-2*pad)
		layout.Points = Rect{X: layout.Panel.X + pad, Y: contentY, W: contentW, H: 48}
		listY := layout.Points.Bottom() + 8
		listH := maxf(140, contentH-48-8)
		detailH := float32(0)
		if model.Selection.HasSelection {
			detailH = minf(300, maxf(210, contentH*0.38))
			listH = minf(listH, maxf(180, contentH-48-8-detailH-12))
		}
		availableH := maxf(0, contentH-48-8-detailH)
		listH = minf(listH, availableH)
		layout.ListViewport = Rect{X: layout.Panel.X + pad, Y: listY, W: contentW, H: listH}
		if detailH > 0 {
			layout.Detail = Rect{X: layout.Panel.X + pad, Y: listY + listH + 12, W: contentW, H: detailH}
		}
		for i := range model.Skills {
			layout.Rows = append(layout.Rows, Rect{X: layout.ListViewport.X + 8, Y: layout.ListViewport.Y + 8 + float32(i)*64 - offset, W: maxf(0, layout.ListViewport.W-16), H: 56})
		}
		return layout
	}
	listW := minf(620, maxf(360, layout.Panel.W*0.54))
	if listW > layout.Panel.W-40 {
		listW = maxf(0, layout.Panel.W-40)
	}
	layout.ListViewport = Rect{X: layout.Panel.X + 20, Y: contentY + 56, W: listW, H: maxf(0, contentH-56)}
	layout.Points = Rect{X: layout.ListViewport.X, Y: contentY, W: listW, H: 48}
	detailX := layout.ListViewport.Right() + 16
	layout.Detail = Rect{X: detailX, Y: contentY, W: maxf(0, layout.Panel.Right()-20-detailX), H: contentH}
	for i := range model.Skills {
		layout.Rows = append(layout.Rows, Rect{X: layout.ListViewport.X + 8, Y: layout.ListViewport.Y + 8 + float32(i)*64 - offset, W: maxf(0, layout.ListViewport.W-16), H: 56})
	}
	return layout
}
