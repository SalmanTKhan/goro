package ui

import (
	"fmt"
	"image"

	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/db"
	"github.com/kivutar/goro/session"
	"github.com/kivutar/goro/ui/rotheme"
)

type CharacterSelectWindowOptions struct {
	SelectedSlot  int
	MaxSlots      int
	Characters    []session.Character
	PreviewImages map[int]image.Image
}

type CharacterSelectWindowCallbacks struct {
	OnSelectSlot   func(int)
	OnActivateSlot func(int)
	OnPreviousSlot func()
	OnNextSlot     func()
	OnMake         func()
	OnOK           func()
	OnCancel       func()
	OnDelete       func()
}

type CharacterSelectWindow struct {
	Window
	opts      CharacterSelectWindowOptions
	callbacks CharacterSelectWindowCallbacks
}

const (
	characterSelectWindowW      = 576
	characterSelectInfoRowCount = 6
	characterSelectInfoRowH     = 16
	characterSelectInfoTableGap = 1
	characterSelectInfoPanelPad = 2
	characterSelectInfoPanelW   = 318
	characterSelectInfoTableH   = characterSelectInfoRowCount*characterSelectInfoRowH + (characterSelectInfoRowCount-1)*characterSelectInfoTableGap
	characterSelectInfoPanelH   = characterSelectInfoTableH + 2*characterSelectInfoPanelPad
	characterSelectWindowH      = 356 + characterSelectInfoRowH + characterSelectInfoTableGap
	characterSelectSlotW        = 139
	characterSelectSlotH        = 144
	characterSelectArrowGap     = 13
	characterSelectInfoColumnW  = (float32(characterSelectInfoPanelW-2*characterSelectInfoPanelPad) - rotheme.TableGap) / 2
	characterSelectInfoLabelW   = float32(44)
	characterSelectInfoValueW   = characterSelectInfoColumnW - characterSelectInfoLabelW - rotheme.TableGap
	characterSelectStatLabelW   = float32(32)
	characterSelectStatValueW   = characterSelectInfoColumnW - characterSelectStatLabelW - rotheme.TableGap
)

var characterSelectSelectedBG = widget.RGBA8(222, 237, 252, 255)

func NewCharacterSelectWindow(ctx client.Context, opts CharacterSelectWindowOptions, callbacks CharacterSelectWindowCallbacks) *CharacterSelectWindow {
	x, y, width, height := characterSelectWindowRect(ctx)
	w := &CharacterSelectWindow{
		opts:      opts,
		callbacks: callbacks,
	}
	w.Window = NewWindow(width, height)
	w.OpenAt(x, y, w.widgetTree())
	return w
}

func (w *CharacterSelectWindow) SetOptions(ctx client.Context, opts CharacterSelectWindowOptions) {
	if w == nil {
		return
	}
	sameTree := characterSelectWindowTreeEqual(w.opts, opts)
	x, y, width, height := characterSelectWindowRect(ctx)
	w.opts = opts
	w.SetAutoPosition(x, y)
	w.SetSize(width, height)
	if sameTree {
		return
	}
	w.SetContent(w.widgetTree())
}

func (w *CharacterSelectWindow) Update(ctx client.Context) bool {
	if w == nil {
		return false
	}
	return w.Window.Update(ctx)
}

func characterSelectWindowTreeEqual(a, b CharacterSelectWindowOptions) bool {
	if a.SelectedSlot != b.SelectedSlot ||
		a.MaxSlots != b.MaxSlots ||
		len(a.Characters) != len(b.Characters) {
		return false
	}
	for i := range a.Characters {
		if a.Characters[i] != b.Characters[i] {
			return false
		}
	}
	if len(a.PreviewImages) != len(b.PreviewImages) {
		return false
	}
	for slot, image := range a.PreviewImages {
		if b.PreviewImages[slot] != image {
			return false
		}
	}
	return true
}

func (w *CharacterSelectWindow) widgetTree() widget.Widget {
	page := CharacterSelectPage(w.opts.SelectedSlot)
	pageCount := maxInt(1, (w.opts.MaxSlots+2)/3)
	pageStart := page * 3
	selected, hasSelection := characterBySlot(w.opts.Characters, w.opts.SelectedSlot)
	deleteDisabled := characterSelectDeleteDisabled(w.opts)
	makeDisabled := characterSelectMakeDisabled(w.opts)
	return Win(
		Title("Select Character"),
		CloseButton(false),
		Size(characterSelectWindowW, characterSelectWindowH),
		Content(
			primitives.Box(
				primitives.HBox(
					characterSelectArrowButton(rotheme.IconButtonLeft, w.callbacks.OnPreviousSlot),
					primitives.HBox(
						w.slotWidget(pageStart),
						w.slotWidget(pageStart+1),
						w.slotWidget(pageStart+2),
					).
						Gap(25),
					characterSelectArrowButton(rotheme.IconButtonRight, w.callbacks.OnNextSlot),
				).
					CrossAlign(primitives.CrossAxisCenter).
					Gap(characterSelectArrowGap),

				rotheme.Text(fmt.Sprintf("%d / %d", page+1, pageCount)).
					Color(rotheme.Default.Colors.MutedText),

				w.infoPanel(selected, hasSelection),
			).
				PaddingTop(12).
				PaddingLeft(24).
				PaddingRight(24).
				Gap(9).
				CrossAlign(primitives.CrossAxisCenter),
		),
		Footer(
			rotheme.ButtonDisabled("Delete", deleteDisabled, func() {
				if w.callbacks.OnDelete != nil {
					w.callbacks.OnDelete()
				}
			}),
			primitives.Expanded(primitives.Box()),
			rotheme.ButtonDisabled("Make", makeDisabled, func() {
				if w.callbacks.OnMake != nil {
					w.callbacks.OnMake()
				}
			}),
			rotheme.Button("OK", func() {
				if w.callbacks.OnOK != nil {
					w.callbacks.OnOK()
				}
			}),
			rotheme.Button("Cancel", func() {
				if w.callbacks.OnCancel != nil {
					w.callbacks.OnCancel()
				}
			}),
		),
	)
}

func characterSelectArrowButton(kind rotheme.IconButtonKind, onClick func()) widget.Widget {
	return primitives.Box(
		primitives.Expanded(primitives.Box()),
		rotheme.IconButton(kind, func() {
			if onClick != nil {
				onClick()
			}
		}),
		primitives.Expanded(primitives.Box()),
	).
		Height(characterSelectSlotH).
		CrossAlign(primitives.CrossAxisCenter)
}

func (w *CharacterSelectWindow) slotWidget(slot int) widget.Widget {
	_, hasCharacter := characterBySlot(w.opts.Characters, slot)
	var preview image.Image
	if w.opts.PreviewImages != nil {
		preview = w.opts.PreviewImages[slot]
	}
	slotButton := button.New(
		button.TextOpt(""),
		button.PainterOpt(characterSelectSlotPainter{
			selected:     slot == w.opts.SelectedSlot,
			hasCharacter: hasCharacter,
			preview:      preview,
		}),
		button.OnClick(func() {
			if slot == w.opts.SelectedSlot {
				if w.callbacks.OnActivateSlot != nil {
					w.callbacks.OnActivateSlot(slot)
				}
				return
			}
			if w.callbacks.OnSelectSlot != nil {
				w.callbacks.OnSelectSlot(slot)
			}
		}),
	)
	// The current character/slot is the initial controller target, matching
	// the desktop selection state instead of starting on the tiny page arrow.
	if slot == w.opts.SelectedSlot {
		slotButton.SetFocused(true)
	}
	return primitives.Box(
		primitives.Expanded(slotButton),
	).
		Width(characterSelectSlotW).
		Height(characterSelectSlotH).
		CrossAlign(primitives.CrossAxisStretch)
}

type characterSelectSlotPainter struct {
	selected     bool
	hasCharacter bool
	preview      image.Image
}

func (p characterSelectSlotPainter) PaintButton(canvas widget.Canvas, state button.PaintState) {
	bounds := state.Bounds
	if bounds.IsEmpty() {
		return
	}
	bg := rotheme.Default.Colors.PanelBody
	border := rotheme.Default.Colors.WindowBorder
	if p.selected {
		bg = characterSelectSelectedBG
		border = rotheme.Default.Colors.ButtonBorder
	}
	if state.Hovered || state.Pressed {
		border = rotheme.Default.Colors.ButtonBorder
	}
	if state.Focused && !state.Disabled {
		border = rotheme.Default.Colors.InputFocus
	}
	canvas.DrawRect(bounds, bg)
	canvas.StrokeRect(bounds, border, 1)
	if state.Focused && !state.Disabled {
		canvas.StrokeRect(bounds.Expand(2), border, 2)
	}
	if p.preview != nil {
		imgBounds := p.preview.Bounds()
		x := bounds.Min.X + (bounds.Width()-float32(imgBounds.Dx()))/2
		y := bounds.Min.Y + (bounds.Height()-float32(imgBounds.Dy()))/2
		canvas.DrawImage(p.preview, geometry.Pt(x, y))
		return
	}
	if !p.hasCharacter {
		rotheme.DrawText(canvas, "Create", bounds, rotheme.Default.Typography.TextSize, rotheme.Default.Colors.MutedText, false, widget.TextAlignCenter)
	}
}

func (w *CharacterSelectWindow) infoPanel(character session.Character, hasCharacter bool) widget.Widget {
	if !hasCharacter {
		return primitives.Box(
			rotheme.Text("Empty Slot"),
			rotheme.Text("Use Make to create a character later."),
		).
			Width(characterSelectInfoPanelW).
			Height(characterSelectInfoPanelH).
			Padding(12).
			Gap(8).
			Background(rotheme.Default.Colors.PanelBody).
			BorderStyle(1, rotheme.Default.Colors.WindowBorder)
	}
	return primitives.Box(
		characterSelectStatsTable(character),
	).
		Width(characterSelectInfoPanelW).
		Height(characterSelectInfoPanelH).
		Padding(characterSelectInfoPanelPad).
		Background(rotheme.Default.Colors.PanelBody).
		BorderStyle(1, rotheme.Default.Colors.WindowBorder)
}

func characterSelectStatsTable(character session.Character) widget.Widget {
	return rotheme.Table(
		characterSelectStatsRows(character),
		rotheme.TableRowHeightOpt(characterSelectInfoRowH),
		rotheme.TableColors(characterSelectSelectedBG, rotheme.Default.Colors.WindowFooter),
	)
}

func characterSelectStatsRows(character session.Character) []rotheme.TableRow {
	leftRows := [characterSelectInfoRowCount]struct {
		label string
		value string
	}{
		{"Name", trimRunes(character.Name, 24)},
		{"Job", trimRunes(db.JobDisplayName(int(character.Job)), 18)},
		{"Level", fmt.Sprintf("%d / %d", character.Level, character.JobLevel)},
		{"Exp", fmt.Sprintf("%d", character.Exp)},
		{"HP", fmt.Sprintf("%d / %d", character.HP, character.MaxHP)},
		{"SP", fmt.Sprintf("%d / %d", character.SP, character.MaxSP)},
	}
	rightRows := [characterSelectInfoRowCount]struct {
		label string
		value string
	}{
		{"STR", fmt.Sprintf("%d", character.Str)},
		{"AGI", fmt.Sprintf("%d", character.Agi)},
		{"VIT", fmt.Sprintf("%d", character.Vit)},
		{"INT", fmt.Sprintf("%d", character.Int)},
		{"DEX", fmt.Sprintf("%d", character.Dex)},
		{"LUK", fmt.Sprintf("%d", character.Luk)},
	}
	rows := make([]rotheme.TableRow, 0, len(leftRows))
	for i, left := range leftRows {
		right := rightRows[i]
		rows = append(rows, rotheme.TableRow{
			{Text: left.label, Width: characterSelectInfoLabelW, Align: widget.TextAlignLeft, Head: true},
			{Text: left.value, Width: characterSelectInfoValueW, Align: widget.TextAlignLeft},
			{Text: right.label, Width: characterSelectStatLabelW, Align: widget.TextAlignLeft, Head: true},
			{Text: right.value, Width: characterSelectStatValueW, Align: widget.TextAlignRight},
		})
	}
	return rows
}

func CharacterSelectPage(slot int) int {
	if slot < 0 {
		return 0
	}
	return slot / 3
}

func characterSelectDeleteDisabled(opts CharacterSelectWindowOptions) bool {
	_, hasSelection := characterBySlot(opts.Characters, opts.SelectedSlot)
	return !hasSelection
}

func characterSelectMakeDisabled(opts CharacterSelectWindowOptions) bool {
	_, hasSelection := characterBySlot(opts.Characters, opts.SelectedSlot)
	return hasSelection
}

func characterSelectWindowRect(ctx client.Context) (int, int, int, int) {
	return centeredWindowRect(ctx, characterSelectWindowW, characterSelectWindowH)
}

func characterBySlot(characters []session.Character, slot int) (session.Character, bool) {
	for _, character := range characters {
		if int(character.Slot) == slot {
			return character, true
		}
	}
	return session.Character{}, false
}
