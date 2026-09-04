package render

import (
	"time"

	"github.com/kivutar/goro/input"
)

// navRepeater implements the hold-to-repeat behavior shared by the directional
// and analog controller navigation paths. It tracks whether an action is held
// explicitly rather than relying on a zero-value sentinel, because UIActionUp
// is the zero value of input.UIAction and was previously indistinguishable
// from "nothing held".
type navRepeater struct {
	action   input.UIAction
	active   bool
	at       time.Time
	repeated bool
}

// fire reports whether the action should be dispatched this frame. A newly
// held action fires immediately, waits delay before the first repeat, and then
// repeats every rate for as long as it stays held.
func (n *navRepeater) fire(action input.UIAction, now time.Time, delay, rate time.Duration) bool {
	if n == nil {
		return false
	}
	if !n.active || n.action != action {
		n.action = action
		n.active = true
		n.repeated = false
		n.at = now
		return true
	}
	interval := delay
	if n.repeated {
		interval = rate
	}
	if now.Sub(n.at) < interval {
		return false
	}
	n.at = now
	n.repeated = true
	return true
}

func (n *navRepeater) reset() {
	if n == nil {
		return
	}
	n.action = 0
	n.active = false
	n.repeated = false
	n.at = time.Time{}
}
