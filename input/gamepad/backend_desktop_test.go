//go:build (windows || linux || darwin) && !android

package gamepad

import "testing"

func TestAxisNormalizesSDLInt16Range(t *testing.T) {
	if got := axis(-32768); got != -1 {
		t.Fatalf("axis(-32768) = %f, want -1", got)
	}
	if got := axis(32767); got != 1 {
		t.Fatalf("axis(32767) = %f, want 1", got)
	}
}

func TestTriggerClampsSDLNegativeValues(t *testing.T) {
	if got := trigger(-32768); got != 0 {
		t.Fatalf("trigger(-32768) = %f, want 0", got)
	}
	if got := trigger(32767); got != 1 {
		t.Fatalf("trigger(32767) = %f, want 1", got)
	}
}
