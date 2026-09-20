package mobileui

import (
	"testing"

	"github.com/kivutar/goro/input"
)

func TestTouchSessionKeepsFingerJitterAsTap(t *testing.T) {
	var session TouchSession
	if !session.Begin(input.TouchPoint{ID: 7, X: 100, Y: 100}, TouchHUD, false) {
		t.Fatal("begin touch")
	}
	if dx, dy, owned := session.Move(input.TouchPoint{ID: 7, X: 106, Y: 105}); !owned || dx != 0 || dy != 0 {
		t.Fatalf("jitter move = (%v, %v, %t), want (0, 0, true)", dx, dy, owned)
	}
	owner, moved := session.End(7)
	if owner != TouchHUD || moved {
		t.Fatalf("end = (%v, %t), want (%v, false)", owner, moved, TouchHUD)
	}
}

func TestTouchSessionStartsDragAfterSlop(t *testing.T) {
	var session TouchSession
	session.Begin(input.TouchPoint{ID: 3, X: 20, Y: 30}, TouchInventory, false)
	if dx, dy, owned := session.Move(input.TouchPoint{ID: 3, X: 32, Y: 39}); !owned || dx != 12 || dy != 9 {
		t.Fatalf("first drag = (%v, %v, %t), want (12, 9, true)", dx, dy, owned)
	}
	if dx, dy, owned := session.Move(input.TouchPoint{ID: 3, X: 35, Y: 45}); !owned || dx != 3 || dy != 6 {
		t.Fatalf("continued drag = (%v, %v, %t), want (3, 6, true)", dx, dy, owned)
	}
	owner, moved := session.End(3)
	if owner != TouchInventory || !moved {
		t.Fatalf("end = (%v, %t), want (%v, true)", owner, moved, TouchInventory)
	}
}

func TestTouchSessionIgnoresAnotherPointer(t *testing.T) {
	var session TouchSession
	session.Begin(input.TouchPoint{ID: 1, X: 0, Y: 0}, TouchHUD, false)
	if dx, dy, owned := session.Move(input.TouchPoint{ID: 2, X: 100, Y: 100}); owned || dx != 0 || dy != 0 {
		t.Fatalf("foreign pointer move = (%v, %v, %t), want (0, 0, false)", dx, dy, owned)
	}
}
