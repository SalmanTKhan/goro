package mobileui

import (
	"fmt"
	"testing"

	"github.com/kivutar/goro/input"
)

func TestChatUsesSafeBoundedPanelAndTouchSafeControls(t *testing.T) {
	viewport := Viewport{Width: 2268, Height: 832, SafeLeft: 24, SafeRight: 24, SafeTop: 18, SafeBottom: 18}
	c := NewChatController(viewport, nil)
	c.Open(MobileChatModel{Channel: "WORLD", Messages: []MobileChatMessage{{Sender: "SYSTEM", Text: "ready"}}})
	l := c.Layout
	if l.Panel.X < l.Safe.X || l.Panel.Right() > l.Safe.Right() || l.Panel.Y < l.Safe.Y || l.Panel.Bottom() > l.Safe.Bottom() {
		t.Fatalf("chat panel escaped safe area: panel=%+v safe=%+v", l.Panel, l.Safe)
	}
	for name, rect := range map[string]Rect{"back": l.Back, "composer": l.Composer, "send": l.Send} {
		if rect.W < 48 || rect.H < 48 {
			t.Fatalf("%s missed minimum touch target: %+v", name, rect)
		}
	}
	if !c.ConsumeTouch(input.TouchPoint{X: 0, Y: 0}) {
		t.Fatal("open chat did not claim modal touch")
	}
}

func TestChatScrollKeepsVisibleMessageIndices(t *testing.T) {
	messages := make([]MobileChatMessage, 40)
	for i := range messages {
		messages[i] = MobileChatMessage{Sender: "SYSTEM", Text: fmt.Sprintf("message-%d", i)}
	}
	c := NewChatController(FoldOuterViewport(), nil)
	c.Open(MobileChatModel{Channel: "WORLD", Messages: messages})
	c.ScrollBy(100000)
	if c.Offset.Offset != c.Offset.MaxOffset() {
		t.Fatalf("scroll offset=%v max=%v", c.Offset.Offset, c.Offset.MaxOffset())
	}
	if len(c.Layout.Rows) == 0 || len(c.Layout.Rows) != len(c.Layout.RowIndices) {
		t.Fatalf("rows=%d indices=%d", len(c.Layout.Rows), len(c.Layout.RowIndices))
	}
	if c.Layout.RowIndices[0] == 0 || c.Layout.RowIndices[len(c.Layout.RowIndices)-1] >= len(messages) {
		t.Fatalf("visible message indices=%v", c.Layout.RowIndices)
	}
}

func TestChatSendIsSemanticAndOfflineReadOnly(t *testing.T) {
	var sink input.CommandBuffer
	c := NewChatController(Viewport{Width: 1920, Height: 1080}, &sink)
	c.Open(MobileChatModel{Channel: "WORLD", CanSend: true})
	c.SetDraft("  hello mobile  ")
	if !c.Send() {
		t.Fatal("online chat send was not accepted")
	}
	commands := sink.Commands()
	if len(commands) != 1 || commands[0].Kind != input.CommandSendGlobalChat || commands[0].Text != "hello mobile" {
		t.Fatalf("commands=%+v", commands)
	}
	if c.Draft != "" {
		t.Fatalf("draft=%q after accepted send", c.Draft)
	}

	c.SetModel(MobileChatModel{Channel: "WORLD", CanSend: false, Notice: "offline"})
	c.SetDraft("should stay local")
	if c.Send() {
		t.Fatal("offline chat send was accepted")
	}
	if c.Draft != "should stay local" || len(sink.Commands()) != 1 {
		t.Fatalf("offline draft/commands changed: draft=%q commands=%+v", c.Draft, sink.Commands())
	}
}

func TestChatWhisperUsesSelectedRecipient(t *testing.T) {
	var sink input.CommandBuffer
	c := NewChatController(Viewport{Width: 1920, Height: 1080}, &sink)
	c.Open(MobileChatModel{Channel: "WORLD", Recipient: "Alice", CanSend: true})
	c.SetDraft(" hello friend ")
	if !c.Send() {
		t.Fatal("whisper send was not accepted")
	}
	got := sink.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandSendWhisper || got[0].TargetName != "Alice" || got[0].Text != "hello friend" {
		t.Fatalf("whisper command=%+v", got)
	}
}
