package ui

import (
	"image/color"
	"strings"
	"testing"

	uiapp "github.com/gogpu/ui/app"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/mobileui"
	"github.com/kivutar/goro/network"
)

func TestNPCDialogTextRunsParseColorCodes(t *testing.T) {
	base := color.RGBA{R: 246, G: 242, B: 232, A: 255}
	runs := npcDialogTextRuns("hello ^FF3300red^000000 base", base)

	if len(runs) != 3 {
		t.Fatalf("run count = %d, want 3: %#v", len(runs), runs)
	}
	if runs[0].text != "hello " || runs[0].color != base {
		t.Fatalf("first run = %#v", runs[0])
	}
	if runs[1].text != "red" || runs[1].color != (color.RGBA{R: 255, G: 51, B: 0, A: 255}) {
		t.Fatalf("colored run = %#v", runs[1])
	}
	if runs[2].text != " base" || runs[2].color != base {
		t.Fatalf("reset run = %#v", runs[2])
	}
}

func TestNPCDialogWrapIgnoresColorCodeWidth(t *testing.T) {
	base := color.RGBA{R: 246, G: 242, B: 232, A: 255}
	lines := wrapNPCDialogLines([]string{"one ^00AAFFtwo^000000 three"}, 10)

	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2: %#v", len(lines), lines)
	}
	if got := npcDialogPlainText(lines[0]); got != "one two" {
		t.Fatalf("first line = %q", got)
	}
	if got := npcDialogPlainText(lines[1]); got != "three" {
		t.Fatalf("second line = %q", got)
	}
	if lines[0][1].text != "two" || lines[0][1].color == base {
		t.Fatalf("wrapped colored run not preserved: %#v", lines[0])
	}
}

func TestNPCDialogTextSegmentsAdvanceByMeasuredRuns(t *testing.T) {
	base := color.RGBA{R: 246, G: 242, B: 232, A: 255}
	red := color.RGBA{R: 255, A: 255}
	runs := []npcDialogTextRun{
		{text: "hello ", color: base},
		{text: "red", color: red},
		{text: " again", color: base},
	}

	segments := npcDialogTextSegments(runs, func(text string) float32 {
		return float32(len([]rune(text)) * 10)
	})

	if len(segments) != 3 {
		t.Fatalf("segment count = %d, want 3: %#v", len(segments), segments)
	}
	if segments[0].x != 0 || segments[0].width != 60 {
		t.Fatalf("first segment = %#v, want x=0 width=60", segments[0])
	}
	if segments[1].x != 60 || segments[1].width != 30 || segments[1].color != red {
		t.Fatalf("colored segment = %#v, want x=60 width=30 red", segments[1])
	}
	if segments[2].x != 90 || segments[2].width != 60 {
		t.Fatalf("final segment = %#v, want x=90 width=60", segments[2])
	}
}

func TestNPCDialogChoiceWindowOpensBelowDialogImmediately(t *testing.T) {
	dialog := NPCDialog{}
	ctx := Context{
		Input:   input.NewState(),
		ScreenW: 1280,
		ScreenH: 720,
	}
	dialog.Apply(network.NPCDialog{
		Kind:    network.NPCDialogMenu,
		NPCID:   100,
		Options: []string{"Prontera", "Geffen", "Payon", "Alberta"},
	})

	if !dialog.Update(ctx) {
		t.Fatal("dialog update did not open choice window")
	}

	expectedX, expectedY, _, _ := dialog.menuBounds(ctx.ScreenW, ctx.ScreenH, dialog.dialogWindow.x, dialog.dialogWindow.y, dialog.dialogWindow.width, dialog.dialogWindow.height)
	if dialog.menuWindow.x != expectedX || dialog.menuWindow.y != expectedY {
		t.Fatalf("choice position = %d,%d, want %d,%d", dialog.menuWindow.x, dialog.menuWindow.y, expectedX, expectedY)
	}
	if dialog.menuWindow.x == 0 && dialog.menuWindow.y == 0 {
		t.Fatal("choice window opened at origin")
	}
}

func TestNPCDialogMenuStartsWithFirstSelection(t *testing.T) {
	dialog := NPCDialog{}
	dialog.menuRow = 2
	dialog.ensureMenuScrollSignal().Set(48)

	dialog.Apply(network.NPCDialog{
		Kind:    network.NPCDialogMenu,
		NPCID:   100,
		Options: []string{"Prontera", "Geffen"},
	})

	if dialog.menuRow != 0 {
		t.Fatalf("menu row = %d, want first selection", dialog.menuRow)
	}
	if scroll := dialog.ensureMenuScrollSignal().Get(); scroll != 0 {
		t.Fatalf("menu scroll = %.1f, want 0", scroll)
	}
}

func TestNPCDialogEmptyMenuStartsWithoutSelection(t *testing.T) {
	dialog := NPCDialog{}

	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogMenu, NPCID: 100})

	if dialog.menuRow != -1 {
		t.Fatalf("menu row = %d, want no selection for empty menu", dialog.menuRow)
	}
}

func TestNPCDialogMenuChoiceRequiresSelection(t *testing.T) {
	dialog := NPCDialog{
		open:    true,
		npcID:   100,
		action:  npcDialogActionMenu,
		options: []string{"Prontera", "Geffen"},
		menuRow: -1,
	}

	dialog.chooseSelected(Context{})

	if dialog.status != "" {
		t.Fatalf("status = %q, want no submit attempt without selection", dialog.status)
	}
	dialog.menuRow = 1
	dialog.chooseSelected(Context{})
	if dialog.status != "not connected" {
		t.Fatalf("status = %q, want submit attempt after selection", dialog.status)
	}
}

func TestNPCDialogMobileModelProjectsServerActions(t *testing.T) {
	dialog := NPCDialog{}
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogMenu, NPCID: 77, Message: "Choose", Options: []string{"^00AAFFUse Storage^000000", "Leave"}})
	model := dialog.MobileModel(Context{})
	if !model.Open || model.NPCID != 77 || len(model.Options) != 3 || model.Options[0].Action != mobileui.DialogMenuChoice || model.Options[0].Value != 1 {
		t.Fatalf("unexpected mobile NPC model: %+v", model)
	}
	if model.Options[0].Label != "^00AAFFUse Storage^000000" {
		t.Fatalf("mobile NPC model discarded RO color controls: %q", model.Options[0].Label)
	}
	if model.Options[2].Action != mobileui.DialogNPCClose {
		t.Fatalf("mobile menu close action = %+v", model.Options[2])
	}
}

func TestNPCDialogMobileModelProjectsBracketedSpeakers(t *testing.T) {
	dialog := NPCDialog{}
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 77, Message: "[PrivateMvpRoom] Please select a private MVP room."})
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 77, Message: "[PrivateMvpRoom] You can only use the room for 0 minutes."})
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 77, Message: "[Tine] Some married chocolate lovers almost double their experience at trainings!<br/>But everything isn't so simply..."})

	model := dialog.MobileModel(Context{})
	if model.Title != "PrivateMvpRoom" {
		t.Fatalf("mobile dialog title = %q, want first bracketed speaker", model.Title)
	}
	if len(model.Messages) != 3 {
		t.Fatalf("mobile dialog messages = %#v, want three speaker turns", model.Messages)
	}
	if model.Messages[0].Speaker != "PrivateMvpRoom" || model.Messages[0].Text != "Please select a private MVP room." {
		t.Fatalf("first mobile dialog message = %#v", model.Messages[0])
	}
	if model.Messages[1].Speaker != "PrivateMvpRoom" || model.Messages[2].Speaker != "Tine" {
		t.Fatalf("mobile dialog speakers = %#v", model.Messages)
	}
	if model.Messages[2].Text != "Some married chocolate lovers almost double their experience at trainings!\nBut everything isn't so simply..." {
		t.Fatalf("mobile dialog body changed RO break: %q", model.Messages[2].Text)
	}
	for _, message := range model.Messages {
		if strings.HasPrefix(strings.TrimSpace(message.Text), "[") {
			t.Fatalf("speaker prefix leaked into mobile dialog body: %#v", message)
		}
	}
}

func TestNPCDialogIgnoresInitialEmptySay(t *testing.T) {
	dialog := NPCDialog{}
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 100, Message: ""})

	if dialog.open {
		t.Fatalf("empty initial say opened dialog: %+v", dialog)
	}
}

func TestNPCDialogCloseHandlerRunsOnceForOpenDialog(t *testing.T) {
	dialog := NPCDialog{}
	closed := 0
	dialog.SetCloseHandler(func() { closed++ })
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 100, Message: "Hello"})
	dialog.Reset()
	if closed != 1 {
		t.Fatalf("close handler calls = %d, want 1", closed)
	}
	dialog.Reset()
	if closed != 1 {
		t.Fatalf("closed dialog invoked handler again: %d", closed)
	}
}

func TestNPCDialogEscapeDoesNotCloseDialogWaitingForNext(t *testing.T) {
	inputState := input.NewState()
	ctx := Context{
		Input:   inputState,
		ScreenW: 1280,
		ScreenH: 720,
	}
	dialog := NPCDialog{}
	closed := 0
	dialog.SetCloseHandler(func() { closed++ })
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 100, Message: "Hello"})
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogNext, NPCID: 100})
	dialog.Update(ctx) // Publish the dialog before processing keyboard input.

	inputState.SetKey(input.KeyEscape, true)
	if !dialog.Update(ctx) {
		t.Fatal("open NPC dialog did not consume Escape")
	}
	if !dialog.IsOpen() {
		t.Fatal("Escape closed a dialog waiting for Next")
	}
	if dialog.action != npcDialogActionNext {
		t.Fatalf("dialog action = %d, want Next", dialog.action)
	}
	if closed != 0 {
		t.Fatalf("close handler calls = %d, want 0", closed)
	}
}

func TestNPCDialogEscapeClosesDialogWaitingForClose(t *testing.T) {
	inputState := input.NewState()
	ctx := Context{
		Input:   inputState,
		ScreenW: 1280,
		ScreenH: 720,
	}
	dialog := NPCDialog{}
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 100, Message: "Goodbye"})
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogClose, NPCID: 100})
	dialog.Update(ctx)

	inputState.SetKey(input.KeyEscape, true)
	if !dialog.Update(ctx) {
		t.Fatal("dialog waiting for Close did not consume Escape")
	}
	if dialog.IsOpen() {
		t.Fatal("Escape left a dialog waiting for Close open")
	}
}

func npcDialogPlainText(runs []npcDialogTextRun) string {
	text := ""
	for _, run := range runs {
		text += run.text
	}
	return text
}

func TestNPCDialogConfirmRequiresReleaseBeforeAdvancing(t *testing.T) {
	// Regression: pressing confirm to talk to an NPC opened the dialog and then
	// advanced past its first page on that same held press.
	var dialog NPCDialog
	dialog.open = true
	dialog.action = npcDialogActionNext
	dialog.confirmArmed = false

	state := input.NewState()
	state.SetKey(input.KeyEnter, true)
	ctx := Context{Input: state}

	dialog.Update(ctx)
	if dialog.confirmArmed {
		t.Fatal("confirm armed while still held")
	}

	// Releasing arms it, but the release itself must not advance.
	state.EndFrame()
	state.SetKey(input.KeyEnter, false)
	dialog.Update(ctx)
	if !dialog.confirmArmed {
		t.Fatal("confirm not armed after release")
	}

	// The next genuine press is the one that advances.
	state.EndFrame()
	state.SetKey(input.KeyEnter, true)
	if !state.JustPressed(input.KeyEnter) {
		t.Fatal("test setup: expected a press edge")
	}
}

func TestNPCDialogControllerConfirmOwnsNextAction(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	ctx := Context{
		Input:     input.NewState(),
		UIManager: manager,
		ScreenW:   800,
		ScreenH:   600,
	}
	dialog := NPCDialog{}
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 100, Message: "Hello"})
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogNext, NPCID: 100})
	if !dialog.Update(ctx) {
		t.Fatal("NPC dialog did not publish")
	}
	app.Frame()

	if !manager.ControllerUIActive() {
		t.Fatal("NPC Next action did not activate controller UI")
	}
	focus := firstControllerFocusable(dialog.dialogWindow.published)
	if focus == nil {
		t.Fatal("NPC Next dialog has no controller-focusable action")
	}
	if controllerFocus, ok := focus.(interface{ IsFocused() bool }); !ok || !controllerFocus.IsFocused() {
		t.Fatalf("NPC Next action focus = %T, want initially focused", focus)
	}
	handler, ok := manager.overlays[len(manager.overlays)-1].(interface {
		HandleControllerAction(input.UIAction) bool
	})
	if !ok {
		t.Fatalf("top NPC overlay %T has no semantic controller handler", manager.overlays[len(manager.overlays)-1])
	}
	if !handler.HandleControllerAction(input.UIActionConfirm) {
		t.Fatal("NPC dialog did not consume controller Confirm")
	}
	if dialog.status != "not connected" {
		t.Fatalf("controller Next status = %q, want the local no-network result", dialog.status)
	}
}

func TestNPCDialogStaysAboveLaterHUDOverlays(t *testing.T) {
	app := uiapp.New(uiapp.WithRenderMode(uiapp.RenderModeFrameworkManaged))
	manager := NewManager()
	manager.SetUIApp(basicMenuTestApp{app: app})
	ctx := Context{
		Input:     input.NewState(),
		UIManager: manager,
		ScreenW:   800,
		ScreenH:   600,
	}
	dialog := NPCDialog{}
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogSay, NPCID: 100, Message: "Hello"})
	dialog.Apply(network.NPCDialog{Kind: network.NPCDialogNext, NPCID: 100})
	if !dialog.Update(ctx) {
		t.Fatal("NPC dialog did not publish")
	}

	// The world redraws HUD overlays after NPCDialog.Update. They must not
	// displace the modal dialog from the foreground controller scope.
	hud := positionedWidget(newInertOverlay(), 10, 10, 80, 40)
	manager.AddOverlay(hud)
	if len(manager.overlays) == 0 || manager.overlays[len(manager.overlays)-1] != dialog.dialogWindow.published {
		t.Fatal("later HUD overlay displaced NPC dialog from foreground")
	}
}
