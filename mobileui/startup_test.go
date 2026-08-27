package mobileui

import "testing"

func TestStartupFlow(t *testing.T) {
	v := Viewport{Width: 2268, Height: 832, SafeLeft: 24, SafeRight: 24}
	c := NewStartupController(v)
	if c.Phase != StartupTitle || !c.Layout.Action.Contains(c.Layout.Action.X+1, c.Layout.Action.Y+1) {
		t.Fatal("startup title/action not initialized")
	}
	c.Tap(c.Layout.Action.X+1, c.Layout.Action.Y+1)
	if c.Phase != StartupProfile {
		t.Fatalf("phase=%v, want profile", c.Phase)
	}
	c.EnterWorld()
	if c.Active() {
		t.Fatal("startup gate remained active after entering world")
	}
}

func TestStartupExposesOnlineSwitch(t *testing.T) {
	c := NewStartupController(FoldOuterViewport())
	if c.ActionAt(c.Layout.AlternateAction.X+1, c.Layout.AlternateAction.Y+1) != StartupSwitchOnline {
		t.Fatal("startup did not expose the online mode action")
	}
	if c.Layout.AlternateAction.W < 48 || c.Layout.AlternateAction.H < 48 {
		t.Fatalf("online mode action is not touch safe: %+v", c.Layout.AlternateAction)
	}
	if c.Phase != StartupTitle {
		t.Fatalf("online switch changed startup phase: %v", c.Phase)
	}
}
