package mobile

import (
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/ui"
	"github.com/kivutar/goro/ui/rotheme"
)

// Kit is the mobile visual vocabulary. Every screen builder composes these
// pieces, so a change to the mobile look happens once, here, instead of across
// the per-screen drawing code the Android host used to carry.
//
// A Kit is a value; copy it freely.
type Kit struct {
	Theme rotheme.Theme
}

// NewKit returns a kit painting with the given theme. Pass rotheme.Mobile for
// the mobile presentation.
func NewKit(theme rotheme.Theme) Kit { return Kit{Theme: theme} }

// TextRole names the typographic slots a mobile screen uses. Sizes derive from
// the theme's base text size so a theme swap rescales everything coherently.
type TextRole int

const (
	// RoleBody is ordinary content text.
	RoleBody TextRole = iota
	// RoleTitle is a window or screen title.
	RoleTitle
	// RoleMuted is secondary text: hints, units, empty states.
	RoleMuted
	// RoleValue is a number the player reads at a glance (HP, zeny, a stat).
	RoleValue
	// RoleLabel is the caption beside a value.
	RoleLabel
)

func (k Kit) roleStyle(role TextRole) (size float32, color widget.Color, bold bool) {
	base := k.Theme.Typography.TextSize
	switch role {
	case RoleTitle:
		return base * 1.15, k.Theme.Colors.TitleText, true
	case RoleMuted:
		return base * 0.85, k.Theme.Colors.MutedText, false
	case RoleValue:
		return base, k.Theme.Colors.Text, true
	case RoleLabel:
		return base * 0.85, k.Theme.Colors.MutedText, false
	default:
		return base, k.Theme.Colors.Text, false
	}
}

// Text returns a single-line label in the given role, ellipsized if it does not
// fit its box.
func (k Kit) Text(content string, role TextRole) *textWidget {
	return k.newText(content, role, 1)
}

// Wrapped returns text that wraps to at most maxLines and ellipsizes beyond it.
// Pass 0 to use as many lines as the box allows. This replaces the hand-rolled
// word-wrapping the Android host used to do.
func (k Kit) Wrapped(content string, role TextRole, maxLines int) *textWidget {
	return k.newText(content, role, maxLines)
}

// Centered returns a single-line label centred within its box.
func (k Kit) Centered(content string, role TextRole) *textWidget {
	return k.newText(content, role, 1).Align(widget.TextAlignCenter)
}

// Content returns a single-line label holding world data — a character name, an
// item, a map. Unlike a fixed UI string it may be arbitrarily long, so it is
// allowed to ellipsize.
func (k Kit) Content(content string, role TextRole) *textWidget {
	t := k.newText(content, role, 1)
	t.fromWorld = true
	return t
}

// RightAligned returns a single-line label pushed to the trailing edge, for
// values that read better against a panel's right margin.
func (k Kit) RightAligned(content string, role TextRole) *textWidget {
	return k.newText(content, role, 1).Align(widget.TextAlignRight)
}

// Workspace is the full-screen ground a mobile screen sits on.
func (k Kit) Workspace() *primitives.BoxWidget {
	return primitives.Box().Background(k.Theme.Colors.PanelBody)
}

// Panel is a plain bordered surface: the mobile equivalent of a desktop window
// body without its title bar.
func (k Kit) Panel() *primitives.BoxWidget {
	return primitives.Box().
		Background(k.Theme.Colors.WindowBody).
		BorderStyle(1, k.Theme.Colors.WindowBorder).
		Rounded(k.Theme.Metrics.ButtonRadius)
}

// Card is a tappable surface in a list or grid. Selected cards reuse the
// desktop client's selection fill and border (ui.SelectionColor /
// ui.SelectionBorder) rather than a Material-style highlight.
func (k Kit) Card(selected bool) *primitives.BoxWidget {
	box := k.Panel()
	if selected {
		return box.
			Background(toWidgetColor(ui.SelectionColor)).
			BorderStyle(2, toWidgetColor(ui.SelectionBorder))
	}
	return box
}

// Slot is an inventory or equipment cell. Empty slots stay quiet so real item
// sprites dominate the grid.
func (k Kit) Slot(occupied, selected bool) *primitives.BoxWidget {
	if selected {
		return k.Card(true)
	}
	if !occupied {
		return primitives.Box().
			Background(k.Theme.Colors.Disabled).
			BorderStyle(1, k.Theme.Colors.FooterLine).
			Rounded(k.Theme.Metrics.ButtonRadius)
	}
	return k.Card(false)
}

// ButtonState distinguishes the states a mobile button can be drawn in. There
// is no hover state: a finger does not hover.
type ButtonState int

const (
	// ButtonNormal is the resting state.
	ButtonNormal ButtonState = iota
	// ButtonPressed is drawn while a touch is down on the control.
	ButtonPressed
	// ButtonDisabled is drawn when the action is unavailable.
	ButtonDisabled
)

// Button returns a labelled button sized by the caller's rectangle. It is
// non-interactive by design — mobileui routes the tap. The label is measured
// against the surface so it can never print outside it.
func (k Kit) Button(label string, state ButtonState) *buttonWidget {
	return k.newButton(label, state)
}

// ContentButton returns a button whose label comes from game data, such as an
// NPC script's menu choice. Unlike authored UI copy it may be arbitrarily long,
// so it is allowed to ellipsize.
func (k Kit) ContentButton(label string, state ButtonState) *buttonWidget {
	b := k.newButton(label, state)
	b.fromWorld = true
	return b
}

// RowLines splits a list row into its title and subtitle lines, inset on all
// four sides. Without the vertical inset the two lines sit flush against the
// row's border and against each other, which is what made the friends list look
// cramped.
//
// leading is the space already consumed at the row's start, such as an icon
// square; pass zero when there is none.
func (k Kit) RowLines(row mobileui.Rect, leading float32) (title, subtitle mobileui.Rect) {
	padX := k.Theme.Metrics.TableCellPadX
	padY := k.Theme.Metrics.TableGap * 3
	inner := mobileui.Rect{
		X: row.X + leading + padX,
		Y: row.Y + padY,
		W: row.W - leading - 2*padX,
		H: row.H - 2*padY,
	}
	if inner.W <= 0 || inner.H <= 0 {
		return mobileui.Rect{}, mobileui.Rect{}
	}
	half := inner.H / 2
	title = mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: half}
	subtitle = mobileui.Rect{X: inner.X, Y: inner.Y + half, W: inner.W, H: half}
	return title, subtitle
}

// Divider is a hairline separator.
func (k Kit) Divider() *primitives.BoxWidget {
	return primitives.Box().Background(k.Theme.Colors.FooterLine)
}

// Scrim dims the world behind a modal sheet. It stays light enough that the
// game reads through it, matching the desktop client's restraint about
// covering the play area.
func (k Kit) Scrim() *primitives.BoxWidget {
	c := k.Theme.Colors.TitleText
	c.A = 0.28
	return primitives.Box().Background(c)
}
