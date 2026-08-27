package mobileui

import (
	"strings"
	"testing"

	"github.com/kivutar/goro/input"
)

func TestDialogLayoutAndShopCommand(t *testing.T) {
	model := FixtureDialog("dialog-long")
	viewport := Viewport{Width: 2268, Height: 832, SafeLeft: 24, SafeRight: 24, SafeTop: 18, SafeBottom: 18}
	var commands input.CommandBuffer
	c := NewDialogController(model, viewport, &commands)
	if !c.ConsumeTouch(input.TouchPoint{X: int(c.Layout.Panel.X + 1), Y: int(c.Layout.Panel.Y + 1)}) {
		t.Fatal("dialog touch was not consumed")
	}
	if c.Layout.Panel.X < c.Layout.Safe.X || c.Layout.Panel.Right() > c.Layout.Safe.Right() || c.Layout.Panel.Y < c.Layout.Safe.Y || c.Layout.Panel.Bottom() > c.Layout.Safe.Bottom() {
		t.Fatalf("dialog escaped safe area: %+v safe=%+v", c.Layout.Panel, c.Layout.Safe)
	}
	for _, option := range c.Layout.Options {
		if option.W < 48 || option.H < 48 {
			t.Fatalf("dialog option missed touch target: %+v", option)
		}
	}
	if len(c.Layout.Options) != 2 {
		t.Fatalf("unexpected dialog options: %+v", c.Model.Options)
	}
	shop := c.Layout.Options[1]
	if !c.Tap(shop.X+2, shop.Y+2) || c.Model.Open {
		t.Fatal("shop option did not close dialog")
	}
	got := commands.Commands()
	if len(got) != 1 || got[0].Kind != input.CommandOpenShop || got[0].NPCID != 9001 {
		t.Fatalf("unexpected dialog command: %+v", got)
	}
}

func TestDialogCloseIsLocal(t *testing.T) {
	var commands input.CommandBuffer
	c := NewDialogController(FixtureDialog("dialog-basic"), Viewport{Width: 1920, Height: 1080}, &commands)
	close := c.Layout.Close
	if !c.Tap(close.X+1, close.Y+1) || c.Model.Open {
		t.Fatal("close option did not close dialog")
	}
	if len(commands.Commands()) != 0 {
		t.Fatal("dialog close emitted a gameplay command")
	}
}

func TestDialogAndShopViewportMatrix(t *testing.T) {
	viewports := []Viewport{
		{Width: 1920, Height: 1080},
		{Width: 2400, Height: 1080},
		{Width: 2560, Height: 1440},
		FoldOuterViewport(),
		{Width: 2268, Height: 832, SafeLeft: 24, SafeRight: 24, SafeTop: 18, SafeBottom: 18},
	}
	for _, viewport := range viewports {
		dialog := LayoutDialog(viewport, FixtureDialog("dialog-long"))
		shop := LayoutShop(viewport, len(FixtureShop("shop-sell").Items), ShopBuyTab)
		for name, panel := range map[string]Rect{"dialog": dialog.Panel, "shop": shop.Panel} {
			if panel.X < panelSafe(viewport).X || panel.Right() > panelSafe(viewport).Right() || panel.Y < panelSafe(viewport).Y || panel.Bottom() > panelSafe(viewport).Bottom() {
				t.Errorf("%s panel escaped viewport=%+v panel=%+v", name, viewport, panel)
			}
		}
	}
}

func TestNPCDialogActionsEmitAuthoritativeCommands(t *testing.T) {
	var commands input.CommandBuffer
	model := MobileDialogModel{
		Open:  true,
		NPCID: 42,
		Options: []DialogOption{
			{Label: "Next", Action: DialogNext, Enabled: true},
			{Label: "Use Storage", Action: DialogMenuChoice, Value: 2, Enabled: true},
			{Label: "Close", Action: DialogNPCClose, Enabled: true},
		},
	}
	c := NewDialogController(model, FoldOuterViewport(), &commands)
	if !c.Tap(c.Layout.Options[0].X+1, c.Layout.Options[0].Y+1) {
		t.Fatal("next action was not consumed")
	}
	if !c.Tap(c.Layout.Options[1].X+1, c.Layout.Options[1].Y+1) {
		t.Fatal("menu action was not consumed")
	}
	if !c.Tap(c.Layout.Options[2].X+1, c.Layout.Options[2].Y+1) {
		t.Fatal("close action was not consumed")
	}
	got := commands.Commands()
	if len(got) != 3 || got[0].Kind != input.CommandNPCNext || got[1].Kind != input.CommandNPCMenuChoice || got[1].Choice != 2 || got[2].Kind != input.CommandNPCClose {
		t.Fatalf("unexpected NPC commands: %+v", got)
	}
}

func TestDialogContentAndActionsNeverOverlap(t *testing.T) {
	viewports := []Viewport{
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
		{Width: 1080, Height: 2400, SafeTop: 48, SafeBottom: 48},
		{Width: 1920, Height: 1080},
		{Width: 2400, Height: 1080},
		{Width: 2560, Height: 1440},
		{Width: 2268, Height: 832, SafeTop: 18, SafeBottom: 18, SafeLeft: 24, SafeRight: 24},
	}
	for _, viewport := range viewports {
		for _, optionCount := range []int{0, 1, 2, 3, 5} {
			for _, notice := range []string{"", "Only in Run-Midgard"} {
				model := MobileDialogModel{Open: true, Title: "NPC", Message: "The guide explains the next destination and what to do before leaving town.", Notice: notice}
				for i := 0; i < optionCount; i++ {
					model.Options = append(model.Options, DialogOption{Label: "Next", Action: DialogNext, Enabled: true})
				}
				layout := LayoutDialog(viewport, model)
				safe := viewport.SafeRect()
				assertInside(t, "dialog panel", layout.Panel, safe)
				assertInside(t, "dialog header", layout.Header, layout.Panel)
				assertInside(t, "dialog message", layout.Message, layout.Panel)
				assertInside(t, "dialog notice", layout.Notice, layout.Panel)
				assertInside(t, "dialog actions", layout.Actions, layout.Panel)
				if layout.Message.Intersects(layout.Notice) || layout.Message.Intersects(layout.Actions) || layout.Notice.Intersects(layout.Actions) {
					t.Fatalf("dialog content overlap viewport=%+v options=%d notice=%q message=%+v noticeRect=%+v actions=%+v", viewport, optionCount, notice, layout.Message, layout.Notice, layout.Actions)
				}
				for i, option := range layout.Options {
					assertInside(t, "dialog option", option, layout.Actions)
					assertTouchTarget(t, "dialog option", option)
					for j := 0; j < i; j++ {
						if option.Intersects(layout.Options[j]) {
							t.Fatalf("dialog options overlap viewport=%+v options=%d: %d=%+v %d=%+v", viewport, optionCount, i, option, j, layout.Options[j])
						}
					}
				}
			}
		}
	}
}

func panelSafe(viewport Viewport) Rect { return viewport.SafeRect() }

func TestDialogHTMLBreaksRemainVisibleInMobileMessageModel(t *testing.T) {
	message := "[Tine] Some married chocolate lovers almost double their experience at trainings!<br/>But everything isn't so simply..."
	normalized := normalizeDialogText(message)
	if !strings.Contains(normalized, "trainings!\nBut everything") {
		t.Fatalf("dialog break was not normalized: %q", normalized)
	}
	if got := dialogLineCount(message, 28); got < 4 {
		t.Fatalf("dialog message was under-counted: %d", got)
	}
	layout := LayoutDialog(Viewport{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24}, MobileDialogModel{
		Open:    true,
		Title:   "NPC",
		Message: message,
		Options: []DialogOption{
			{Label: "WOW! TELL ME MORE!", Action: DialogMenuChoice, Enabled: true},
			{Label: "MARRI... WHAT?", Action: DialogMenuChoice, Enabled: true},
			{Label: "CANCEL", Action: DialogNPCClose, Enabled: true},
		},
	})
	if layout.Message.H <= 0 || layout.Message.Intersects(layout.Actions) {
		t.Fatalf("dialog message is not separated from actions: message=%+v actions=%+v", layout.Message, layout.Actions)
	}
}

func TestParseDialogSpeakerUsesLeadingBracketName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		speaker string
		body    string
		ok      bool
	}{
		{name: "tine", input: "[Tine] Some married chocolate lovers", speaker: "Tine", body: "Some married chocolate lovers", ok: true},
		{name: "private mvp room", input: " [PrivateMvpRoom]\tPlease select a private MVP room.", speaker: "PrivateMvpRoom", body: "Please select a private MVP room.", ok: true},
		{name: "body brackets are not prefix", input: "Choose [a room] below.", body: "Choose [a room] below.", ok: false},
		{name: "empty bracket", input: "[] text", body: "[] text", ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			speaker, body, ok := ParseDialogSpeaker(test.input)
			if speaker != test.speaker || body != test.body || ok != test.ok {
				t.Fatalf("ParseDialogSpeaker(%q) = (%q, %q, %t), want (%q, %q, %t)", test.input, speaker, body, ok, test.speaker, test.body, test.ok)
			}
		})
	}
}

func TestParseDialogMessagesSplitsMultipleSpeakersInsideOnePacket(t *testing.T) {
	messages := ParseDialogMessages([]string{"[PrivateMvpRoom] Please select a private MVP room.<br/><br/>[Tine] Everything is not so simple..."})
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2: %#v", len(messages), messages)
	}
	if messages[0].Speaker != "PrivateMvpRoom" || messages[0].Text != "Please select a private MVP room." {
		t.Fatalf("first message = %#v", messages[0])
	}
	if messages[1].Speaker != "Tine" || messages[1].Text != "Everything is not so simple..." {
		t.Fatalf("second message = %#v", messages[1])
	}
}

func TestParseDialogMessagesPairsMarkerOnlyPacketsWithBody(t *testing.T) {
	messages := ParseDialogMessages([]string{
		"[PrivateMvpRoom]",
		"Please select a private MVP room.",
		"[PrivateMvpRoom]",
		"You can only use the room for 0 minutes.",
		"[PrivateMvpRoom]",
	})
	if len(messages) != 3 {
		t.Fatalf("message count = %d, want 3: %#v", len(messages), messages)
	}
	if messages[0].Speaker != "PrivateMvpRoom" || messages[0].Text != "Please select a private MVP room." || messages[1].Speaker != "PrivateMvpRoom" || messages[2].Speaker != "PrivateMvpRoom" {
		t.Fatalf("paired marker messages = %#v", messages)
	}
}

func TestDialogSpeakerBlocksCollapseRepeatedSpeakerLabels(t *testing.T) {
	model := MobileDialogModel{
		Open:  true,
		Title: "NPC",
		Messages: []DialogMessage{
			{Speaker: "PrivateMvpRoom", Text: "Please select a private MVP room."},
			{Speaker: "PrivateMvpRoom", Text: "You can only use the room for 0 minutes."},
			{Speaker: "Tine", Text: "But everything isn't so simply..."},
		},
		Options: []DialogOption{{Label: "Next", Action: DialogNext, Enabled: true}},
	}
	for _, viewport := range []Viewport{
		{Width: 390, Height: 844, SafeTop: 24, SafeBottom: 24},
		{Width: 2400, Height: 1080},
	} {
		layout := LayoutDialog(viewport, model)
		if len(layout.MessageBlocks) != len(model.Messages) {
			t.Fatalf("viewport=%+v message block count=%d, want %d", viewport, len(layout.MessageBlocks), len(model.Messages))
		}
		if layout.MessageBlocks[0].SpeakerRect.W <= 0 || layout.MessageBlocks[1].SpeakerRect.W != 0 || layout.MessageBlocks[2].SpeakerRect.W <= 0 {
			t.Fatalf("unexpected speaker label rects viewport=%+v blocks=%+v", viewport, layout.MessageBlocks)
		}
		for i, block := range layout.MessageBlocks {
			assertInside(t, "message block text", block.TextRect, layout.Message)
			if block.SpeakerRect.W > 0 {
				assertInside(t, "message block speaker", block.SpeakerRect, layout.Message)
			}
			if block.TextRect.Intersects(layout.Actions) || block.SpeakerRect.Intersects(layout.Actions) {
				t.Fatalf("message block %d overlaps actions viewport=%+v block=%+v actions=%+v", i, viewport, block, layout.Actions)
			}
		}
	}
}
