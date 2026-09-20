package mobile

import (
	"strings"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
	"github.com/kivutar/goro/mobileui"
)

// DialogTree builds the NPC conversation window: the speaker's turns, an
// optional notice, and the choices the script offers.
func (k Kit) DialogTree(model mobileui.MobileDialogModel, layout mobileui.DialogLayout) *Canvas {
	return k.DialogTreeScrolled(model, layout, 0)
}

// DialogTreeScrolled renders the message body as a clipped scroll viewport.
// Header, notice and actions remain fixed while only prose moves.
func (k Kit) DialogTreeScrolled(model mobileui.MobileDialogModel, layout mobileui.DialogLayout, offset float32) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if !model.Open || layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	if layout.Header.W > 0 {
		c.Place(k.Text(dialogTitle(model), RoleTitle), layout.Header)
	}
	if layout.Message.W > 0 && layout.Message.H > 0 {
		c.Place(k.dialogBody(model, offset), layout.Message)
	}
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

func (k Kit) placeDialogOptions(c *Canvas, model mobileui.MobileDialogModel, layout mobileui.DialogLayout) {
	for i, rect := range layout.Options {
		if i >= len(model.Options) || rect.W <= 0 || rect.H <= 0 {
			break
		}
		option := model.Options[i]
		c.Place(k.ContentButton(mobileui.StripROText(option.Label), buttonStateFor(option.Enabled)), rect)
	}
}

type dialogBodyWidget struct {
	widget.WidgetBase
	model       mobileui.MobileDialogModel
	offset      float32
	bodyStyle   widget.TextStyle
	speakerStyle widget.TextStyle
}

func (k Kit) dialogBody(model mobileui.MobileDialogModel, offset float32) *dialogBodyWidget {
	bodySize, bodyColor, bodyBold := k.roleStyle(RoleBody)
	speakerSize, speakerColor, speakerBold := k.roleStyle(RoleTitle)
	family := k.Theme.Typography.FontFamily
	boldFamily := k.Theme.Typography.BoldFontFamily
	bodyFamily := family
	if bodyBold && boldFamily != "" {
		bodyFamily, bodyBold = boldFamily, false
	}
	speakerFamily := family
	if speakerBold && boldFamily != "" {
		speakerFamily, speakerBold = boldFamily, false
	}
	w := &dialogBodyWidget{
		model: model,
		offset: offset,
		bodyStyle: widget.TextStyle{FontFamily: bodyFamily, FontSize: bodySize, Bold: bodyBold, Color: bodyColor, Align: widget.TextAlignLeft},
		speakerStyle: widget.TextStyle{FontFamily: speakerFamily, FontSize: speakerSize, Bold: speakerBold, Color: speakerColor, Align: widget.TextAlignLeft},
	}
	w.SetVisible(true)
	w.SetEnabled(true)
	return w
}

func (w *dialogBodyWidget) Layout(_ widget.Context, constraints geometry.Constraints) geometry.Size {
	size := constraints.Constrain(geometry.Sz(constraints.MaxWidth, constraints.MaxHeight))
	w.SetBounds(geometry.FromPointSize(w.Position(), size))
	return size
}

func (w *dialogBodyWidget) Draw(_ widget.Context, canvas widget.Canvas) {
	if !w.IsVisible() || w.Bounds().IsEmpty() {
		return
	}
	bounds := w.Bounds()
	bodyH := w.bodyStyle.FontSize * 1.3
	speakerH := w.speakerStyle.FontSize * 1.3
	type row struct {
		text string
		style widget.TextStyle
		h float32
	}
	var rows []row
	messages := w.model.Messages
	if len(messages) == 0 {
		if speaker, body, ok := mobileui.ParseDialogSpeaker(w.model.Message); ok {
			messages = []mobileui.DialogMessage{{Speaker: speaker, Text: body}}
		} else {
			messages = []mobileui.DialogMessage{{Text: w.model.Message}}
		}
	}
	previousSpeaker := strings.TrimSpace(w.model.Title)
	for i, message := range messages {
		speaker := strings.TrimSpace(message.Speaker)
		if speaker != "" && speaker != previousSpeaker {
			if len(rows) > 0 {
				rows = append(rows, row{h: 6})
			}
			rows = append(rows, row{text: speaker, style: w.speakerStyle, h: speakerH})
		}
		if i > 0 {
			rows = append(rows, row{h: 4})
		}
		for _, line := range wrapText(canvas, mobileui.StripROText(message.Text), w.bodyStyle, bounds.Width()-10, 10000) {
			rows = append(rows, row{text: line, style: w.bodyStyle, h: bodyH})
		}
		if speaker != "" {
			previousSpeaker = speaker
		}
	}
	contentH := float32(0)
	for _, r := range rows {
		contentH += r.h
	}
	maxOffset := contentH - bounds.Height()
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := w.offset
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	canvas.PushClip(bounds)
	y := bounds.Min.Y - offset
	for _, r := range rows {
		if r.text != "" && y+r.h >= bounds.Min.Y && y <= bounds.Max.Y {
			drawStyled(canvas, r.text, geometry.NewRect(bounds.Min.X, y, bounds.Width()-10, r.h), r.style)
		}
		y += r.h
	}
	canvas.PopClip()

	if maxOffset > 0 {
		railX := bounds.Max.X - 4
		canvas.DrawRoundRect(geometry.NewRect(railX, bounds.Min.Y, 3, bounds.Height()), widget.RGBA(0, 0, 0, 0.12), 1.5)
		thumbH := bounds.Height() * bounds.Height() / contentH
		if thumbH < 28 {
			thumbH = 28
		}
		travel := bounds.Height() - thumbH
		thumbY := bounds.Min.Y
		if maxOffset > 0 {
			thumbY += travel * (offset / maxOffset)
		}
		canvas.DrawRoundRect(geometry.NewRect(railX, thumbY, 3, thumbH), widget.RGBA(0.20, 0.28, 0.36, 0.55), 1.5)
	}
}

func (w *dialogBodyWidget) Event(widget.Context, event.Event) bool { return false }
func (w *dialogBodyWidget) Children() []widget.Widget { return nil }
