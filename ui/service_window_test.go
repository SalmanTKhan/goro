package ui

import (
	"testing"

	"github.com/kivutar/goro/client"
	"github.com/kivutar/goro/input"
)

func TestServiceWindowClampsInitialSelection(t *testing.T) {
	window := NewServiceWindow(client.Context{ScreenW: 1280, ScreenH: 720}, []string{"Local", "Internet"}, ServiceWindowOptions{Selected: 8}, ServiceWindowCallbacks{})

	if got := window.SelectedIndex(); got != 0 {
		t.Fatalf("selected service = %d, want 0", got)
	}
}

func TestServiceWindowUsesStageTitle(t *testing.T) {
	window := NewServiceWindow(client.Context{ScreenW: 1280, ScreenH: 720}, []string{"Local"}, ServiceWindowOptions{Title: "Server"}, ServiceWindowCallbacks{})

	if window.title != "Server" {
		t.Fatalf("service window title = %q, want Server", window.title)
	}
}

func TestServiceWindowConfirmUsesCurrentSelection(t *testing.T) {
	selected := -1
	window := NewServiceWindow(client.Context{ScreenW: 1280, ScreenH: 720}, []string{"Local", "Internet"}, ServiceWindowOptions{Selected: 1}, ServiceWindowCallbacks{
		OnSelect: func(index int) {
			selected = index
		},
	})

	window.confirm()

	if selected != 1 {
		t.Fatalf("confirmed service = %d, want 1", selected)
	}
}

func TestServiceWindowCannotConfirmEmptyList(t *testing.T) {
	called := false
	window := NewServiceWindow(client.Context{ScreenW: 1280, ScreenH: 720}, nil, ServiceWindowOptions{}, ServiceWindowCallbacks{
		OnSelect: func(int) {
			called = true
		},
	})

	window.confirm()

	if called {
		t.Fatal("empty service list was confirmed")
	}
}

func TestServiceWindowEnterConfirmsCurrentSelection(t *testing.T) {
	selected := -1
	inputState := input.NewState()
	ctx := client.Context{Input: inputState, ScreenW: 1280, ScreenH: 720}
	window := NewServiceWindow(ctx, []string{"Local", "Internet"}, ServiceWindowOptions{Selected: 1}, ServiceWindowCallbacks{
		OnSelect: func(index int) {
			selected = index
		},
	})
	inputState.SetKey(input.KeyEnter, true)

	if !window.Update(ctx) {
		t.Fatal("Enter was not consumed")
	}
	if selected != 1 {
		t.Fatalf("confirmed service = %d, want 1", selected)
	}
}
