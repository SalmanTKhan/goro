package input

import (
	"math"
	"time"
)

// cursorMaxFrameDelta bounds the timestep the virtual cursor integrates over so
// a stall — a debugger break, a map load — cannot fling the pointer across the
// screen on the frame that follows it.
const cursorMaxFrameDelta = 100 * time.Millisecond

// VirtualCursor is the analog-stick-driven pointer. It holds sub-pixel position
// so slow stick deflections still accumulate into motion, and reports movement
// only when the integer pixel position actually changes, which is what keeps
// the renderer from injecting a redundant pointer event every frame.
type VirtualCursor struct {
	x, y          float32
	width, height int
	ready         bool
}

// Reset centers the cursor in a viewport of the given size.
func (c *VirtualCursor) Reset(width, height int) {
	if c == nil {
		return
	}
	c.width, c.height = width, height
	c.x, c.y = float32(width)/2, float32(height)/2
	c.ready = width > 0 && height > 0
	c.clamp()
}

// Resize keeps the cursor at the same relative position in a resized viewport.
func (c *VirtualCursor) Resize(width, height int) {
	if c == nil || width <= 0 || height <= 0 {
		return
	}
	if !c.ready || c.width <= 0 || c.height <= 0 {
		c.Reset(width, height)
		return
	}
	c.x *= float32(width) / float32(c.width)
	c.y *= float32(height) / float32(c.height)
	c.width, c.height = width, height
	c.clamp()
}

// SyncTo adopts an engine-initiated pointer position so the virtual cursor
// stays coherent with a window that moved the pointer itself.
func (c *VirtualCursor) SyncTo(x, y int) {
	if c == nil {
		return
	}
	c.x, c.y = float32(x), float32(y)
	c.ready = true
	c.clamp()
}

// Ready reports whether the cursor has been given a viewport. A cursor that is
// legitimately parked at the origin is still ready — the flag is explicit
// rather than inferred from the position.
func (c *VirtualCursor) Ready() bool {
	return c != nil && c.ready
}

func (c *VirtualCursor) Position() (int, int) {
	if c == nil {
		return 0, 0
	}
	return int(c.x), int(c.y)
}

// Move integrates a deadzone-corrected stick vector and reports whether the
// integer pixel position changed. speed is the travel in pixels per second at
// full deflection, before the response curve.
func (c *VirtualCursor) Move(dx, dy float32, dt time.Duration, speed float32) bool {
	if c == nil || !c.ready || speed <= 0 || dt <= 0 {
		return false
	}
	dx, dy = CursorResponse(dx, dy)
	if dx == 0 && dy == 0 {
		return false
	}
	seconds := float32(dt.Seconds())
	beforeX, beforeY := int(c.x), int(c.y)
	c.x += dx * speed * seconds
	c.y += dy * speed * seconds
	c.clamp()
	return int(c.x) != beforeX || int(c.y) != beforeY
}

func (c *VirtualCursor) clamp() {
	maxX, maxY := float32(0), float32(0)
	if c.width > 0 {
		maxX = float32(c.width - 1)
	}
	if c.height > 0 {
		maxY = float32(c.height - 1)
	}
	c.x = clampFloat32(c.x, 0, maxX)
	c.y = clampFloat32(c.y, 0, maxY)
}

// CursorResponse applies a quadratic ramp to the magnitude of a stick vector so
// small deflections give fine control without amplifying diagonals. The curve
// is applied to the magnitude rather than per axis, which is what keeps the
// direction of travel identical to the direction of the stick.
func CursorResponse(x, y float32) (float32, float32) {
	magnitude := float32(math.Hypot(float64(x), float64(y)))
	if magnitude <= 0 {
		return 0, 0
	}
	unitX, unitY := x/magnitude, y/magnitude
	if magnitude > 1 {
		magnitude = 1
	}
	scaled := magnitude * magnitude
	return unitX * scaled, unitY * scaled
}

// ClampFrameDelta bounds a frame timestep for cursor integration.
func ClampFrameDelta(dt time.Duration) time.Duration {
	if dt < 0 {
		return 0
	}
	if dt > cursorMaxFrameDelta {
		return cursorMaxFrameDelta
	}
	return dt
}

func clampFloat32(value, low, high float32) float32 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
