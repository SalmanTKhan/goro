package input

import (
	"math"
	"testing"
	"time"
)

func TestVirtualCursorResetCenters(t *testing.T) {
	var cursor VirtualCursor
	if cursor.Ready() {
		t.Fatal("zero cursor must not be ready")
	}
	cursor.Reset(800, 600)
	if !cursor.Ready() {
		t.Fatal("cursor must be ready after reset")
	}
	if x, y := cursor.Position(); x != 400 || y != 300 {
		t.Fatalf("center = %d,%d want 400,300", x, y)
	}
}

func TestVirtualCursorOriginStaysReady(t *testing.T) {
	// A cursor parked at the top-left corner is a legitimate position, not an
	// "uninitialized" sentinel, and must not be re-centered.
	var cursor VirtualCursor
	cursor.Reset(800, 600)
	cursor.SyncTo(0, 0)
	if !cursor.Ready() {
		t.Fatal("cursor at origin must stay ready")
	}
	if x, y := cursor.Position(); x != 0 || y != 0 {
		t.Fatalf("position = %d,%d want 0,0", x, y)
	}
}

func TestVirtualCursorMoveReportsPixelChanges(t *testing.T) {
	var cursor VirtualCursor
	cursor.Reset(800, 600)

	// Sub-pixel travel accumulates without reporting a change.
	if cursor.Move(1, 0, time.Millisecond, 100) {
		t.Fatal("sub-pixel motion must not report a pixel change")
	}
	moved := false
	for i := 0; i < 20; i++ {
		if cursor.Move(1, 0, time.Millisecond, 100) {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatal("accumulated sub-pixel motion must eventually change a pixel")
	}
}

func TestVirtualCursorMoveClampsToViewport(t *testing.T) {
	var cursor VirtualCursor
	cursor.Reset(320, 240)
	for i := 0; i < 100; i++ {
		cursor.Move(-1, -1, 16*time.Millisecond, 4000)
	}
	if x, y := cursor.Position(); x != 0 || y != 0 {
		t.Fatalf("min corner = %d,%d want 0,0", x, y)
	}
	for i := 0; i < 200; i++ {
		cursor.Move(1, 1, 16*time.Millisecond, 4000)
	}
	if x, y := cursor.Position(); x != 319 || y != 239 {
		t.Fatalf("max corner = %d,%d want 319,239", x, y)
	}
}

func TestVirtualCursorResizeKeepsRelativePosition(t *testing.T) {
	var cursor VirtualCursor
	cursor.Reset(800, 600)
	cursor.SyncTo(200, 150) // one quarter across, one quarter down
	cursor.Resize(1600, 1200)
	if x, y := cursor.Position(); x != 400 || y != 300 {
		t.Fatalf("resized position = %d,%d want 400,300", x, y)
	}
}

func TestCursorResponsePreservesDirection(t *testing.T) {
	x, y := CursorResponse(0.6, 0.6)
	if math.Abs(float64(x-y)) > 1e-6 {
		t.Fatalf("diagonal skewed: %v,%v", x, y)
	}
	// The quadratic ramp attenuates partial deflection but leaves full
	// deflection at unit magnitude.
	input := math.Hypot(0.6, 0.6)
	if magnitude := math.Hypot(float64(x), float64(y)); magnitude >= input {
		t.Fatalf("partial deflection not attenuated: %v >= %v", magnitude, input)
	}
	fx, fy := CursorResponse(1, 0)
	if fx != 1 || fy != 0 {
		t.Fatalf("full deflection = %v,%v want 1,0", fx, fy)
	}
	// Over-unit input is clamped rather than amplified.
	ox, _ := CursorResponse(2, 0)
	if ox != 1 {
		t.Fatalf("over-unit deflection = %v want 1", ox)
	}
}

func TestCursorResponseIsMonotonic(t *testing.T) {
	previous := float32(0)
	for step := 1; step <= 10; step++ {
		x, _ := CursorResponse(float32(step)/10, 0)
		if x < previous {
			t.Fatalf("response decreased at step %d: %v < %v", step, x, previous)
		}
		previous = x
	}
}

func TestClampFrameDelta(t *testing.T) {
	if got := ClampFrameDelta(16 * time.Millisecond); got != 16*time.Millisecond {
		t.Fatalf("normal delta = %v", got)
	}
	if got := ClampFrameDelta(5 * time.Second); got != cursorMaxFrameDelta {
		t.Fatalf("stall delta = %v want %v", got, cursorMaxFrameDelta)
	}
	if got := ClampFrameDelta(-time.Second); got != 0 {
		t.Fatalf("negative delta = %v want 0", got)
	}
}
