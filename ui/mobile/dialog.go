package mobile

import (
	"github.com/kivutar/goro/mobileui"
)

// DialogTree builds the NPC conversation window: the speaker's turns, an
// optional notice, and the choices the script offers.
func (k Kit) DialogTree(model mobileui.MobileDialogModel, layout mobileui.DialogLayout) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if !model.Open || layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	if layout.Header.W > 0 {
		c.Place(k.Text(dialogTitle(model), RoleTitle), layout.Header)
	}
	if layout.Close.W > 0 {
		c.Place(k.Button("Close", ButtonNormal), layout.Close)
	}
	k.placeDialogMessages(c, model, layout)
	if layout.Notice.W > 0 && model.Notice != "" {
		c.Place(k.Wrapped(model.Notice, RoleMuted, 2), layout.Notice)
	}
	k.placeDialogOptions(c, model, layout)
	return c
}

func dialogTitle(model mobileui.MobileDialogModel) string {
	if model.Title != "" {
		return model.Title
	}
	return "Dialog"
}

// placeDialogMessages renders each speaker turn. The layout resolves speaker
// labels and body rectangles per turn, including collapsing the label when the
// same speaker talks twice in a row.
func (k Kit) placeDialogMessages(c *Canvas, model mobileui.MobileDialogModel, layout mobileui.DialogLayout) {
	if len(layout.MessageBlocks) == 0 {
		// Older single-message dialogs still populate Message rather than blocks.
		if layout.Message.W > 0 && model.Message != "" {
			c.Place(k.Wrapped(mobileui.StripROText(model.Message), RoleBody, 0), layout.Message)
		}
		return
	}
	for _, block := range layout.MessageBlocks {
		if block.SpeakerRect.W > 0 && block.Speaker != "" {
			c.Place(k.Text(block.Speaker, RoleTitle), block.SpeakerRect)
		}
		if block.TextRect.W > 0 && block.Text != "" {
			// RO scripts embed ^RRGGBB colour codes; they must not reach the screen.
			c.Place(k.Wrapped(mobileui.StripROText(block.Text), RoleBody, 0), block.TextRect)
		}
	}
}

func (k Kit) placeDialogOptions(c *Canvas, model mobileui.MobileDialogModel, layout mobileui.DialogLayout) {
	for i, rect := range layout.Options {
		if i >= len(model.Options) || rect.W <= 0 || rect.H <= 0 {
			break
		}
		option := model.Options[i]
		c.Place(k.ContentButton(mobileui.StripROText(option.Label), buttonStateFor(option.Enabled)), rect)
	}
}
