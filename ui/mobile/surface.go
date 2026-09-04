package mobile

import (
	"github.com/kivutar/goro/mobileui"
)

// SurfaceTree builds the generic list screen. Settings is its main user, but
// the same projection backs any label/value screen, so this one builder covers
// several destinations rather than each growing its own layout.
func (k Kit) SurfaceTree(
	model mobileui.SurfaceModel,
	layout mobileui.SurfaceLayout,
	state mobileui.SurfaceInteractionState,
) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	// Like the other list screens the backing sheet follows its content, so a
	// short settings list does not paint a full-height slab over the world.
	panel := layout.Panel
	if bottom := surfaceContentBottom(layout); bottom > panel.Y && bottom < panel.Bottom() {
		panel.H = bottom - panel.Y
	}
	c.Place(k.Panel(), panel)

	if layout.Back.W > 0 {
		c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.Back)
	}
	if layout.Header.W > 0 {
		title := mobileui.Rect{X: layout.Header.X, Y: layout.Header.Y, W: layout.Header.W, H: layout.Header.H}
		if layout.Back.W > 0 {
			pad := k.Theme.Metrics.TableCellPadX
			title.X = layout.Back.Right() + pad
			title.W = max32(0, layout.Header.Right()-title.X)
		}
		if title.W > 0 {
			c.Place(k.Centered(model.Title, RoleTitle), title)
		}
	}
	k.placeSurfaceRows(c, model, layout, state)

	// The layout anchors the notice to the bottom of the safe area. With the
	// panel hugging its rows that would leave the text stranded over the world,
	// so it follows the last row instead and stays on the sheet.
	if model.Notice != "" && layout.Notice.W > 0 {
		notice := layout.Notice
		notice.Y = min32(notice.Y, panel.Bottom()-notice.H-12)
		if notice.Y > layout.Header.Bottom() {
			c.Place(k.Wrapped(model.Notice, RoleMuted, 2), notice)
		}
	}
	return c
}

func (k Kit) placeSurfaceRows(
	c *Canvas,
	model mobileui.SurfaceModel,
	layout mobileui.SurfaceLayout,
	state mobileui.SurfaceInteractionState,
) {
	pad := k.Theme.Metrics.TableCellPadX
	for i, row := range layout.Rows {
		if i >= len(model.Items) || row.W <= 0 || row.H <= 0 {
			break
		}
		if !row.Intersects(layout.ListViewport) {
			continue
		}
		item := model.Items[i]

		// A section header is a caption, not a control: no card, no value. Its
		// blurb, when the model supplies one, sits under the heading.
		if item.Kind == mobileui.SurfaceItemSection {
			labelH := mobileui.SurfaceRowLabelHeight()
			c.Place(k.Text(item.Label, RoleTitle),
				mobileui.Rect{X: row.X + pad, Y: row.Y, W: row.W - 2*pad, H: min32(labelH, row.H)})
			if item.Detail != "" && row.H > labelH {
				c.Place(k.Wrapped(item.Detail, RoleMuted, 1),
					mobileui.Rect{X: row.X + pad, Y: row.Y + labelH, W: row.W - 2*pad, H: row.H - labelH})
			}
			continue
		}

		selected := state.SelectedID != "" && state.SelectedID == item.ID
		c.Place(k.Card(selected), row)

		inner := mobileui.Rect{X: row.X + pad, Y: row.Y, W: row.W - 2*pad, H: row.H}
		label, value := k.surfaceRowRects(inner, item)

		labelRole := RoleBody
		if !item.Enabled {
			labelRole = RoleMuted
		}
		// A row with help text stacks label over detail, splitting at the same
		// place the layout reserved. The detail is prose, so it wraps across the
		// remaining height rather than being forced onto one line.
		if item.Detail != "" {
			labelH := mobileui.SurfaceRowLabelHeight()
			c.Place(k.Text(item.Label, labelRole), mobileui.Rect{X: label.X, Y: label.Y, W: label.W, H: labelH})
			detail := mobileui.Rect{X: inner.X, Y: label.Y + labelH, W: inner.W, H: inner.H - labelH}
			if detail.H > 0 {
				c.Place(k.Wrapped(item.Detail, RoleMuted, 0), detail)
			}
			if value.W > 0 && item.Value != "" {
				c.Place(k.RightAligned(item.Value, RoleValue),
					mobileui.Rect{X: value.X, Y: value.Y, W: value.W, H: labelH})
			}
			continue
		}
		c.Place(k.Text(item.Label, labelRole), label)
		if value.W > 0 && item.Value != "" {
			c.Place(k.RightAligned(item.Value, RoleValue), value)
		}
	}
}

// surfaceRowRects splits a row into its label and value columns. A row with no
// value gives the whole width to the label.
func (k Kit) surfaceRowRects(inner mobileui.Rect, item mobileui.SurfaceItem) (label, value mobileui.Rect) {
	if item.Value == "" {
		return inner, mobileui.Rect{}
	}
	valueW := inner.W * 0.38
	label = mobileui.Rect{X: inner.X, Y: inner.Y, W: inner.W - valueW, H: inner.H}
	value = mobileui.Rect{X: inner.X + inner.W - valueW, Y: inner.Y, W: valueW, H: inner.H}
	return label, value
}

// surfaceContentBottom reports where the last visible row ends.
func surfaceContentBottom(layout mobileui.SurfaceLayout) float32 {
	bottom := layout.Header.Bottom()
	for _, row := range layout.Rows {
		if row.Intersects(layout.ListViewport) && row.Bottom() > bottom {
			bottom = row.Bottom()
		}
	}
	if layout.DetailSheet.H > 0 && layout.DetailSheet.Bottom() > bottom {
		bottom = layout.DetailSheet.Bottom()
	}
	// Reserve room for the notice so it lands inside the sheet.
	if layout.Notice.H > 0 {
		bottom += layout.Notice.H + 12
	}
	return bottom + 12
}
