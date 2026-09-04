package mobile

import (
	"github.com/kivutar/goro/mobileui"
)

// ChatTree builds the chat screen: the message log with a composer beneath it.
func (k Kit) ChatTree(model mobileui.MobileChatModel, layout mobileui.ChatLayout, draft string) *Canvas {
	c := NewCanvas(layout.Safe.W+2*layout.Safe.X, layout.Safe.H+2*layout.Safe.Y)
	if layout.Panel.W <= 0 || layout.Panel.H <= 0 {
		return c
	}
	c.Place(k.Panel(), layout.Panel)

	k.placeChatHeader(c, model, layout)
	k.placeChatMessages(c, model, layout)
	k.placeChatComposer(c, model, layout, draft)
	return c
}

func (k Kit) placeChatHeader(c *Canvas, model mobileui.MobileChatModel, layout mobileui.ChatLayout) {
	if layout.Back.W > 0 {
		c.Place(k.Button(mobileui.BackLabel, ButtonNormal), layout.Back)
	}
	if layout.Header.W <= 0 {
		return
	}
	pad := k.Theme.Metrics.TableCellPadX
	left := layout.Header.X + pad
	if layout.Back.W > 0 {
		left = layout.Back.Right() + pad
	}
	title := mobileui.Rect{X: left, Y: layout.Header.Y, W: layout.Header.Right() - pad - left, H: layout.Header.H}
	if title.W > 0 {
		c.Place(k.Centered(chatTitle(model), RoleTitle), title)
	}
}

// chatTitle names the conversation: a whisper is with someone, otherwise the
// channel is what identifies it.
func chatTitle(model mobileui.MobileChatModel) string {
	if model.Recipient != "" {
		return "To " + model.Recipient
	}
	if model.Channel != "" {
		return model.Channel
	}
	return "Chat"
}

func (k Kit) placeChatMessages(c *Canvas, model mobileui.MobileChatModel, layout mobileui.ChatLayout) {
	if layout.MessageViewport.W <= 0 || layout.MessageViewport.H <= 0 {
		return
	}
	c.Place(k.Panel(), layout.MessageViewport)
	if len(model.Messages) == 0 {
		message := "No messages yet"
		if model.Notice != "" {
			message = model.Notice
		}
		c.Place(k.Centered(message, RoleMuted), layout.MessageViewport)
		return
	}

	pad := k.Theme.Metrics.TableCellPadX
	for i, row := range layout.Rows {
		index := chatRowIndex(layout, i)
		if index < 0 || index >= len(model.Messages) || row.W <= 0 || row.H <= 0 {
			continue
		}
		message := model.Messages[index]
		inner := mobileui.Rect{X: row.X + pad, Y: row.Y, W: row.W - 2*pad, H: row.H}
		if message.Sender == "" {
			// A system line has no speaker, so it takes the whole row.
			c.Place(k.Wrapped(mobileui.StripROText(message.Text), RoleMuted, 0), inner)
			continue
		}
		senderW := inner.W * 0.28
		c.Place(k.Text(message.Sender, RoleLabel),
			mobileui.Rect{X: inner.X, Y: inner.Y, W: senderW, H: inner.H})
		c.Place(k.Wrapped(mobileui.StripROText(message.Text), RoleBody, 0),
			mobileui.Rect{X: inner.X + senderW, Y: inner.Y, W: inner.W - senderW, H: inner.H})
	}
}

func (k Kit) placeChatComposer(
	c *Canvas,
	model mobileui.MobileChatModel,
	layout mobileui.ChatLayout,
	draft string,
) {
	if layout.Composer.W > 0 && layout.Composer.H > 0 {
		c.Place(k.Panel(), layout.Composer)
		pad := k.Theme.Metrics.TableCellPadX
		inner := mobileui.Rect{X: layout.Composer.X + pad, Y: layout.Composer.Y,
			W: layout.Composer.W - 2*pad, H: layout.Composer.H}
		if inner.W > 0 {
			// An empty composer shows a prompt rather than nothing, so it reads
			// as a place to type.
			if draft == "" {
				c.Place(k.Text("Say something…", RoleMuted), inner)
			} else {
				c.Place(k.Text(draft, RoleBody), inner)
			}
		}
	}
	if layout.Send.W > 0 && layout.Send.H > 0 {
		c.Place(k.Button("Send", buttonStateFor(model.CanSend)), layout.Send)
	}
}

func chatRowIndex(layout mobileui.ChatLayout, row int) int {
	if row < len(layout.RowIndices) {
		return layout.RowIndices[row]
	}
	return row
}
