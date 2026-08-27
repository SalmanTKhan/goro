package mobileui

import (
	"strings"

	"github.com/kivutar/goro/input"
)

type MobileChatMessage struct {
	Sender string
	Text   string
}

type MobileChatModel struct {
	Open      bool
	Channel   string
	Recipient string
	Messages  []MobileChatMessage
	CanSend   bool
	Notice    string
}

type ChatLayout struct {
	Safe, Panel, Header, Back, MessageViewport, Composer, Send Rect
	Rows                                                       []Rect
	RowIndices                                                 []int
}

type ChatController struct {
	Model    MobileChatModel
	Layout   ChatLayout
	Viewport Viewport
	Draft    string
	Offset   ScrollState
	Sink     input.CommandSink
}

func NewChatController(viewport Viewport, sink input.CommandSink) *ChatController {
	c := &ChatController{Viewport: viewport, Sink: sink}
	c.relayout()
	return c
}

func (c *ChatController) Open(model MobileChatModel) {
	if c == nil {
		return
	}
	model.Open = true
	if model.Channel == "" {
		model.Channel = "WORLD"
	}
	c.Model = model
	c.Offset = ScrollState{}
	c.relayout()
}

func (c *ChatController) Close() bool {
	if c == nil || !c.Model.Open {
		return false
	}
	c.Model.Open = false
	c.relayout()
	return true
}

// SetModel refreshes the app-owned projection while preserving local chat
// state. The mobile surface does not own transcript history or connection
// authority.
func (c *ChatController) SetModel(model MobileChatModel) {
	if c == nil {
		return
	}
	model.Open = c.Model.Open
	if model.Recipient == "" {
		model.Recipient = c.Model.Recipient
	}
	if model.Channel == "" {
		model.Channel = "WORLD"
	}
	c.Model = model
	c.relayout()
}

func (c *ChatController) Resize(viewport Viewport) {
	if c == nil {
		return
	}
	c.Viewport = viewport
	c.relayout()
}

func (c *ChatController) SetMessages(messages []MobileChatMessage) {
	if c == nil {
		return
	}
	c.Model.Messages = append([]MobileChatMessage(nil), messages...)
	c.relayout()
}

func (c *ChatController) SetDraft(draft string) {
	if c != nil {
		c.Draft = draft
	}
}

func (c *ChatController) ConsumeTouch(point input.TouchPoint) bool {
	return c != nil && c.Model.Open
}

func (c *ChatController) Tap(x, y float32) bool {
	if c == nil || !c.Model.Open {
		return false
	}
	if c.Layout.Back.Contains(x, y) {
		c.Close()
		return true
	}
	if c.Layout.Send.Contains(x, y) {
		return c.Send()
	}
	return true
}

func (c *ChatController) ScrollBy(delta float32) bool {
	if c == nil || !c.Model.Open {
		return false
	}
	c.Offset.ScrollBy(delta)
	c.relayout()
	return true
}

func (c *ChatController) Send() bool {
	if c == nil || !c.Model.Open || !c.Model.CanSend {
		return false
	}
	text := strings.TrimSpace(c.Draft)
	if text == "" {
		return false
	}
	command := input.PlayerCommand{Kind: input.CommandSendGlobalChat, Text: text}
	if strings.TrimSpace(c.Model.Recipient) != "" {
		command.Kind = input.CommandSendWhisper
		command.TargetName = strings.TrimSpace(c.Model.Recipient)
	}
	accepted := c.Sink != nil && c.Sink.Emit(command)
	if accepted {
		c.Draft = ""
	}
	return accepted
}

func (c *ChatController) relayout() {
	if c == nil {
		return
	}
	safe := c.Viewport.SafeRect()
	l := ChatLayout{Safe: safe}
	if !c.Model.Open || safe.W <= 0 || safe.H <= 0 {
		c.Layout = l
		return
	}
	panelW := minf(900, maxf(0, safe.W-32))
	panelH := maxf(0, safe.H-32)
	l.Panel = Rect{safe.X + (safe.W-panelW)/2, safe.Y + (safe.H-panelH)/2, panelW, panelH}
	pad := float32(16)
	l.Header = Rect{l.Panel.X + pad, l.Panel.Y + 12, l.Panel.W - 2*pad, 56}
	l.Back = Rect{l.Header.X, l.Header.Y, 104, 52}
	l.Composer = Rect{l.Panel.X + pad, l.Panel.Bottom() - pad - 56, l.Panel.W - 136 - pad, 56}
	l.Send = Rect{l.Composer.Right() + 8, l.Composer.Y, 112, 56}
	l.MessageViewport = Rect{l.Panel.X + pad, l.Header.Bottom() + 12, l.Panel.W - 2*pad, maxf(0, l.Composer.Y-l.Header.Bottom()-24)}
	rowExtent := float32(56)
	c.Offset.ViewportExtent = l.MessageViewport.H
	c.Offset.ContentExtent = maxf(0, float32(len(c.Model.Messages))*rowExtent-8)
	c.Offset.SetOffset(c.Offset.Offset)
	first := int(c.Offset.Offset / rowExtent)
	last := minInt(len(c.Model.Messages), first+int(l.MessageViewport.H/rowExtent)+2)
	for i := first; i < last; i++ {
		row := Rect{l.MessageViewport.X, l.MessageViewport.Y + float32(i)*rowExtent - c.Offset.Offset, l.MessageViewport.W, 48}
		if row.Y >= l.MessageViewport.Y && row.Bottom() <= l.MessageViewport.Bottom() {
			l.Rows = append(l.Rows, row)
			l.RowIndices = append(l.RowIndices, i)
		}
	}
	c.Layout = l
}
