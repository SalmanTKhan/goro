package mobileui

const (
	// profileLabelRow is one line box of the real UI font. Label rows were 18
	// pixels, sized for the old bitmap text, and the real font spilled past the
	// panel borders.
	profileLabelRow float32 = 32
	// profileIdentityHeight keeps the panel's original budget: growing it
	// pushed the stats card off the bottom of short viewports.
	profileIdentityHeight float32 = 136
)

type ProfileKeyRect struct {
	Key  string
	Rect Rect
}

type MobileProfileLayout struct {
	Safe, Panel, Header, HeaderTitle, BackButton                    Rect
	Preview, Identity, Appearance, Stats, StarterTotal, Notice      Rect
	NameLabel, NameField, SexLabel, AppearanceLabel                 Rect
	SexButton, HairPrev, HairNext, HairColor                        Rect
	EditButton, NewButton, ContinueButton, SaveButton, CancelButton Rect
	Keyboard, KeyboardDone                                          Rect
	StatRows, StatMinus, StatPlus                                   [6]Rect
	Keys                                                            []ProfileKeyRect
	Editor, EditingName, Stacked                                    bool
}

func LayoutMobileProfile(viewport Viewport, model MobileProfileModel, editor, editingName bool) MobileProfileLayout {
	safe := viewport.SafeRect()
	layout := MobileProfileLayout{Safe: safe, Editor: editor, EditingName: editingName}
	if safe.W <= 0 || safe.H <= 0 {
		return layout
	}
	stacked := viewport.IsPortrait() || safe.W < 900
	layout.Stacked = stacked
	panelWidth := float32(1280)
	if stacked {
		panelWidth = minf(720, safe.W-32)
	}
	layout.Panel = centeredMobileRail(safe, panelWidth, 16)
	pad := float32(16)
	layout.Header = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + pad, W: layout.Panel.W - 2*pad, H: 56}
	layout.BackButton = Rect{X: layout.Header.X, Y: layout.Header.Y, W: BackButtonWidth(), H: 52}
	layout.HeaderTitle = Rect{X: layout.BackButton.Right() + 16, Y: layout.Header.Y, W: maxf(0, layout.Header.Right()-layout.BackButton.Right()-16), H: layout.Header.H}
	contentX := layout.Panel.X + pad
	contentY := layout.Header.Bottom() + 16
	contentW := maxf(0, layout.Panel.W-2*pad)
	var statsY float32
	statsX, statsW := contentX, contentW

	if stacked {
		previewH := minf(160, maxf(140, contentW*0.30))
		layout.Preview = Rect{X: contentX, Y: contentY, W: contentW, H: previewH}
		// Three identity rows need their own vertical space: section header,
		// name label/field, and the read-only sex label.
		layout.Identity = Rect{X: contentX, Y: layout.Preview.Bottom() + 12, W: contentW, H: profileIdentityHeight}
		appearanceH := float32(112)
		if editor {
			// Editor controls occupy two non-overlapping rows below the header:
			// sex, then hair style/color.
			appearanceH = 164
		}
		layout.Appearance = Rect{X: contentX, Y: layout.Identity.Bottom() + 12, W: contentW, H: appearanceH}
		statsY = layout.Appearance.Bottom() + 12
	} else {
		previewW := minf(300, maxf(230, contentW*0.25))
		layout.Preview = Rect{X: contentX, Y: contentY, W: previewW, H: layout.Panel.Bottom() - contentY - 16}
		rightX := layout.Preview.Right() + 16
		rightW := maxf(0, layout.Panel.Right()-pad-rightX)
		layout.Identity = Rect{X: rightX, Y: contentY, W: rightW, H: profileIdentityHeight}
		appearanceH := float32(116)
		if editor {
			appearanceH = 164
		}
		layout.Appearance = Rect{X: rightX, Y: layout.Identity.Bottom() + 12, W: rightW, H: appearanceH}
		statsY = layout.Appearance.Bottom() + 12
		statsX, statsW = rightX, rightW
	}

	// Label rows are a full line box tall. At 18 pixels they were sized for the
	// old bitmap text and the real font spilled past the panel borders.
	layout.NameLabel = Rect{X: layout.Identity.X + 12, Y: layout.Identity.Y + 6, W: maxf(0, layout.Identity.W-24), H: profileLabelRow}
	layout.NameField = Rect{X: layout.Identity.X + 12, Y: layout.NameLabel.Bottom() + 2, W: maxf(0, layout.Identity.W-24), H: 48}
	layout.SexLabel = Rect{X: layout.Identity.X + 12, Y: layout.NameField.Bottom() + 2, W: maxf(0, layout.Identity.W-24), H: profileLabelRow}
	layout.AppearanceLabel = Rect{X: layout.Appearance.X + 12, Y: layout.Appearance.Y + 8, W: maxf(0, layout.Appearance.W-24), H: profileLabelRow}
	layout.SexButton = Rect{X: layout.Appearance.X + 12, Y: layout.Appearance.Y + 58, W: maxf(110, minf(190, layout.Appearance.W*0.30)), H: 48}
	layout.HairPrev = Rect{X: layout.Appearance.X + 12, Y: layout.Appearance.Y + 112, W: 52, H: 48}
	layout.HairNext = Rect{X: layout.HairPrev.Right() + 8, Y: layout.HairPrev.Y, W: 52, H: 48}
	layout.HairColor = Rect{X: layout.HairNext.Right() + 12, Y: layout.HairPrev.Y, W: maxf(110, layout.Appearance.Right()-layout.HairNext.Right()-24), H: 48}

	// Action controls get their own footer reservation. The old layout placed
	// STARTER TOTAL in the same pixels as CONTINUE, and the keyboard editor
	// could leave stat content behind the modal.
	var buttonY float32
	if editor {
		buttonY = layout.Panel.Bottom() - 68
		if editingName {
			layout.Keyboard = Rect{X: contentX, Y: layout.Panel.Bottom() - 16 - 326, W: contentW, H: 326}
			buttonY = layout.Keyboard.Y - 60
		}
		statsBottom := maxf(statsY, buttonY-12)
		layout.Stats = Rect{X: statsX, Y: statsY, W: statsW, H: maxf(0, statsBottom-statsY)}
		layout.StarterTotal = Rect{X: layout.Stats.X + 16, Y: layout.Stats.Y + 8, W: maxf(0, layout.Stats.W-32), H: profileLabelRow}
		layout.SaveButton = Rect{X: layout.Panel.Right() - 252, Y: buttonY, W: 116, H: 52}
		layout.CancelButton = Rect{X: layout.SaveButton.Right() + 12, Y: buttonY, W: 116, H: 52}
	} else {
		statsBottom := maxf(statsY, layout.Panel.Bottom()-16)
		layout.Stats = Rect{X: statsX, Y: statsY, W: statsW, H: maxf(0, statsBottom-statsY)}
		buttonY = layout.Stats.Bottom() - 112
		editW := maxf(0, (layout.Stats.W-36)/2)
		layout.EditButton = Rect{X: layout.Stats.X + 12, Y: buttonY, W: editW, H: 52}
		layout.NewButton = Rect{X: layout.EditButton.Right() + 12, Y: buttonY, W: maxf(0, layout.Stats.Right()-layout.EditButton.Right()-24), H: 52}
		layout.ContinueButton = Rect{X: layout.Stats.X + 12, Y: layout.Stats.Bottom() - 56, W: maxf(0, layout.Stats.W-24), H: 52}
		layout.StarterTotal = Rect{X: layout.Stats.X + 16, Y: buttonY - 24, W: maxf(0, layout.Stats.W-32), H: 18}
		layout.Notice = Rect{X: layout.Stats.X + 16, Y: buttonY - 48, W: maxf(0, layout.Stats.W-32), H: 18}
	}
	layout.layoutStatRows()
	if editingName {
		layout.makeKeyboard()
	}
	return layout
}

func (l *MobileProfileLayout) layoutStatRows() {
	if l == nil || l.Stats.W <= 0 || l.Stats.H <= 0 {
		return
	}
	gap := float32(8)
	innerW := maxf(0, l.Stats.W-24)
	cellW := maxf(0, (innerW-gap)/2)
	rowGap := float32(6)
	rowH := float32(32)
	top := l.Stats.Y + 38
	if l.Editor {
		// Two columns keep six editable stats within a phone-height card while
		// each +/- control retains the 48px minimum touch target.
		rowGap = 4
		rowH = 40
		top = l.Stats.Y + 34
	}
	for i := 0; i < 6; i++ {
		row, col := i/2, i%2
		x := l.Stats.X + 12 + float32(col)*(cellW+gap)
		y := top + float32(row)*(rowH+rowGap)
		l.StatRows[i] = Rect{X: x, Y: y, W: cellW, H: rowH}
		if l.Editor {
			l.StatMinus[i] = Rect{X: x + cellW - 104, Y: y - 4, W: 48, H: 48}
			l.StatPlus[i] = Rect{X: x + cellW - 48, Y: y - 4, W: 48, H: 48}
		}
	}
	if !l.Editor {
		l.StatMinus, l.StatPlus = [6]Rect{}, [6]Rect{}
	}
}

func (l *MobileProfileLayout) makeKeyboard() {
	if l == nil || l.Keyboard.W <= 0 || l.Keyboard.H <= 0 {
		return
	}
	keys := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", "SPACE", "⌫", "CLEAR"}
	const cols = 5
	gap := float32(6)
	keyW := (l.Keyboard.W - gap*float32(cols-1)) / cols
	keyH := float32(48)
	for i, key := range keys {
		row, col := i/cols, i%cols
		l.Keys = append(l.Keys, ProfileKeyRect{Key: key, Rect: Rect{X: l.Keyboard.X + float32(col)*(keyW+gap), Y: l.Keyboard.Y + 8 + float32(row)*(keyH+gap), W: keyW, H: keyH}})
	}
	// The keyboard is a modal sheet. Keep an explicit, full-size dismissal
	// target in the otherwise unused fifth cell of the final row so the user
	// never has to discover that the editor header BACK button also dismisses
	// the keyboard.
	row, col := 5, 4
	l.KeyboardDone = Rect{X: l.Keyboard.X + float32(col)*(keyW+gap), Y: l.Keyboard.Y + 8 + float32(row)*(keyH+gap), W: keyW, H: keyH}
	l.Keys = append(l.Keys, ProfileKeyRect{Key: "DONE", Rect: l.KeyboardDone})
}
