package input

import (
	"fmt"
	"math"
	"time"
)

// MobileMovementMode controls how a single finger addresses the world.
// MovementHoldToMove makes a drag follow walkable ground; MovementTapToMove
// only acts when the touch is released.
type MobileMovementMode uint8

const (
	MovementHoldToMove MobileMovementMode = iota
	MovementTapToMove
)

func (m MobileMovementMode) String() string {
	if m == MovementTapToMove {
		return "Tap to move"
	}
	return "Hold to move"
}

// MobileControls is the persisted, device-independent mobile input policy.
// Screen coordinates and pointer IDs deliberately remain in the adapter.
type MobileControls struct {
	MovementMode      MobileMovementMode
	CameraSensitivity float64
	ZoomSensitivity   float64
	InvertCameraY     bool
	LongPressMS       int
	ShowTargetNames   bool
}

func DefaultMobileControls() MobileControls {
	return MobileControls{
		MovementMode:      MovementHoldToMove,
		CameraSensitivity: 1,
		ZoomSensitivity:   1,
		LongPressMS:       550,
		ShowTargetNames:   true,
	}
}

func (c MobileControls) Validate() error {
	if c.MovementMode > MovementTapToMove {
		return fmt.Errorf("invalid mobile movement mode %d", c.MovementMode)
	}
	if !finiteInRange(c.CameraSensitivity, 0.25, 3) {
		return fmt.Errorf("mobile camera sensitivity must be between 0.25 and 3")
	}
	if !finiteInRange(c.ZoomSensitivity, 0.25, 3) {
		return fmt.Errorf("mobile zoom sensitivity must be between 0.25 and 3")
	}
	if c.LongPressMS < 300 || c.LongPressMS > 1500 {
		return fmt.Errorf("mobile long press must be between 300 and 1500 milliseconds")
	}
	return nil
}

func (c MobileControls) Normalized() MobileControls {
	defaults := DefaultMobileControls()
	if c.Validate() != nil {
		return defaults
	}
	return c
}

func (c MobileControls) GestureConfig() GestureConfig {
	c = c.Normalized()
	gesture := DefaultGestureConfig()
	gesture.LongPressDuration = time.Duration(c.LongPressMS) * time.Millisecond
	return gesture
}

func finiteInRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}
