package input

import (
	"testing"
	"time"
)

type testPicker struct{ target PickedTarget }

func (p testPicker) Pick(WorldPosition) (PickedTarget, bool) { return p.target, true }

type testUI struct{ consumed bool }

func (u testUI) ConsumeTouch(TouchPoint) bool { return u.consumed }

func testTime(ms int) time.Time { return time.Unix(0, int64(ms)*int64(time.Millisecond)) }

func TestDesktopAdapterEmitsSemanticCommands(t *testing.T) {
	var sink CommandBuffer
	adapter := DesktopInputAdapter{
		Picker: testPicker{target: PickedTarget{Kind: TargetGround, Position: WorldPosition{X: 4, Y: 7}}},
		Sink:   &sink,
	}
	adapter.Update(DesktopFrame{Position: WorldPosition{X: 10, Y: 20}, LeftJustPressed: true})
	adapter.Update(DesktopFrame{RightDown: true, MouseDX: 3, MouseDY: -2, WheelY: 1})
	commands := sink.Commands()
	if len(commands) != 3 || commands[0].Kind != CommandMoveTo || commands[1].Kind != CommandRotateCamera || commands[2].Kind != CommandZoomCamera {
		t.Fatalf("commands = %#v", commands)
	}
}

func TestUIConsumedTouchDoesNotEmitWorldCommand(t *testing.T) {
	var sink CommandBuffer
	adapter := NewMobileInputAdapter(DefaultGestureConfig(), testPicker{target: PickedTarget{Kind: TargetGround}}, testUI{consumed: true}, &sink)
	adapter.Update(TouchFrame{At: testTime(0), Points: []TouchPoint{{ID: 1, X: 10, Y: 20}}})
	adapter.Update(TouchFrame{At: testTime(100)})
	if got := sink.Commands(); len(got) != 0 {
		t.Fatalf("consumed touch emitted %#v", got)
	}
}

func TestMobileHoldToMoveNeverRotatesCamera(t *testing.T) {
	var sink CommandBuffer
	adapter := NewMobileInputAdapterWithControls(DefaultMobileControls(), testPicker{target: PickedTarget{Kind: TargetGround, Position: WorldPosition{X: 4, Y: 7}}}, nil, &sink)
	adapter.Update(TouchFrame{At: testTime(0), Points: []TouchPoint{{ID: 1, X: 10, Y: 20}}})
	adapter.Update(TouchFrame{At: testTime(20), Points: []TouchPoint{{ID: 1, X: 30, Y: 20}}})
	adapter.Update(TouchFrame{At: testTime(30), Points: []TouchPoint{{ID: 1, X: 35, Y: 20}}})
	commands := sink.Commands()
	if len(commands) != 2 || commands[0].Kind != CommandMoveTo || commands[1].Kind != CommandMoveTo {
		t.Fatalf("hold-to-move commands = %#v", commands)
	}
}

func TestMobileTapToMoveDragDoesNotEmitMovementOrRotation(t *testing.T) {
	controls := DefaultMobileControls()
	controls.MovementMode = MovementTapToMove
	var sink CommandBuffer
	adapter := NewMobileInputAdapterWithControls(controls, testPicker{target: PickedTarget{Kind: TargetGround}}, nil, &sink)
	adapter.Update(TouchFrame{At: testTime(0), Points: []TouchPoint{{ID: 1, X: 10, Y: 20}}})
	adapter.Update(TouchFrame{At: testTime(20), Points: []TouchPoint{{ID: 1, X: 30, Y: 20}}})
	adapter.Update(TouchFrame{At: testTime(30), Points: []TouchPoint{{ID: 1, X: 35, Y: 20}}})
	adapter.Update(TouchFrame{At: testTime(40)})
	if got := sink.Commands(); len(got) != 0 {
		t.Fatalf("tap-to-move drag commands = %#v", got)
	}
}

func TestMobileTwoFingerDragRotatesAndPinchZooms(t *testing.T) {
	controls := DefaultMobileControls()
	controls.CameraSensitivity = 2
	controls.ZoomSensitivity = 0.5
	var sink CommandBuffer
	adapter := NewMobileInputAdapterWithControls(controls, nil, nil, &sink)
	adapter.Update(TouchFrame{At: testTime(0), Points: []TouchPoint{{ID: 1, X: 10, Y: 10}, {ID: 2, X: 30, Y: 10}}})
	adapter.Update(TouchFrame{At: testTime(10), Points: []TouchPoint{{ID: 1, X: 15, Y: 10}, {ID: 2, X: 35, Y: 10}}})
	adapter.Update(TouchFrame{At: testTime(20), Points: []TouchPoint{{ID: 1, X: 15, Y: 10}, {ID: 2, X: 45, Y: 10}}})
	commands := sink.Commands()
	if len(commands) != 3 || commands[0].Kind != CommandRotateCamera || commands[0].DeltaX != 10 || commands[1].Kind != CommandZoomCamera || commands[1].DeltaY != 5 || commands[2].Kind != CommandRotateCamera || commands[2].DeltaX != 10 {
		t.Fatalf("two-finger commands = %#v", commands)
	}
}

func TestMobileLongPressInspectsActor(t *testing.T) {
	var sink CommandBuffer
	adapter := NewMobileInputAdapterWithControls(DefaultMobileControls(), testPicker{target: PickedTarget{Kind: TargetActor, ActorID: 42}}, nil, &sink)
	adapter.Update(TouchFrame{At: testTime(0), Points: []TouchPoint{{ID: 1, X: 10, Y: 20}}})
	adapter.Update(TouchFrame{At: testTime(600), Points: []TouchPoint{{ID: 1, X: 10, Y: 20}}})
	commands := sink.Commands()
	if len(commands) != 1 || commands[0].Kind != CommandInspectActor || commands[0].ActorID != 42 {
		t.Fatalf("long-press commands = %#v", commands)
	}
}

func TestMobileAdapterMapsTargets(t *testing.T) {
	cases := []struct {
		name   string
		target PickedTarget
		want   CommandKind
	}{
		{"ground", PickedTarget{Kind: TargetGround}, CommandMoveTo},
		{"hostile", PickedTarget{Kind: TargetActor, ActorID: 7, Hostile: true}, CommandAttackActor},
		{"npc", PickedTarget{Kind: TargetNPC, ActorID: 8}, CommandInteractActor},
		{"vending", PickedTarget{Kind: TargetVending, ActorID: 10}, CommandOpenVending},
		{"item", PickedTarget{Kind: TargetItem, ItemID: 9}, CommandPickUpItem},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sink CommandBuffer
			adapter := NewMobileInputAdapter(DefaultGestureConfig(), testPicker{target: tc.target}, nil, &sink)
			adapter.Update(TouchFrame{At: testTime(0), Points: []TouchPoint{{ID: 1, X: 10, Y: 20}}})
			adapter.Update(TouchFrame{At: testTime(100)})
			got := sink.Commands()
			if len(got) != 1 || got[0].Kind != tc.want {
				t.Fatalf("commands = %#v, want %v", got, tc.want)
			}
		})
	}
}

func TestSkillTargetStateEmitsAndClearsSemanticCommand(t *testing.T) {
	var state SkillTargetState
	state.BeginActor(42, 3)
	command, ok := state.Select(PickedTarget{Kind: TargetActor, ActorID: 9})
	if !ok || command.Kind != CommandUseSkillOnActor || command.SkillID != 42 || command.Level != 3 || command.ActorID != 9 {
		t.Fatalf("command = %#v, ok=%t", command, ok)
	}
	if state.Mode != SkillTargetIdle {
		t.Fatalf("state after select = %#v, want idle", state)
	}
}

func TestGestureTapDragLongPressPinchAndCancel(t *testing.T) {
	r := NewGestureRecognizer(DefaultGestureConfig())
	r.Update(TouchFrame{At: testTime(0), Points: []TouchPoint{{ID: 1, X: 10, Y: 10}}})
	events := r.Update(TouchFrame{At: testTime(100)})
	if len(events) != 1 || events[0].Kind != GestureTap {
		t.Fatalf("tap events = %#v", events)
	}

	r.Update(TouchFrame{At: testTime(200), Points: []TouchPoint{{ID: 1, X: 10, Y: 10}}})
	events = r.Update(TouchFrame{At: testTime(210), Points: []TouchPoint{{ID: 1, X: 30, Y: 10}}})
	if len(events) != 1 || events[0].Kind != GestureDragStart {
		t.Fatalf("drag start events = %#v", events)
	}
	events = r.Update(TouchFrame{At: testTime(220), Points: []TouchPoint{{ID: 1, X: 35, Y: 10}}})
	if len(events) != 1 || events[0].Kind != GestureDrag || events[0].DeltaX != 5 {
		t.Fatalf("drag events = %#v", events)
	}

	r.Reset()
	r.Update(TouchFrame{At: testTime(300), Points: []TouchPoint{{ID: 1, X: 10, Y: 10}}})
	events = r.Update(TouchFrame{At: testTime(900), Points: []TouchPoint{{ID: 1, X: 10, Y: 10}}})
	if len(events) != 1 || events[0].Kind != GestureLongPress {
		t.Fatalf("long press events = %#v", events)
	}

	r.Reset()
	r.Update(TouchFrame{At: testTime(1000), Points: []TouchPoint{{ID: 1, X: 0, Y: 0}, {ID: 2, X: 10, Y: 0}}})
	events = r.Update(TouchFrame{At: testTime(1010), Points: []TouchPoint{{ID: 1, X: 0, Y: 0}, {ID: 2, X: 30, Y: 0}}})
	if len(events) != 2 || events[0].Kind != GesturePinch || events[0].Delta != 20 || events[1].Kind != GestureTwoFingerDrag || events[1].DeltaX != 10 {
		t.Fatalf("pinch events = %#v", events)
	}

	events = r.Update(TouchFrame{At: testTime(1020), Cancel: true})
	if len(events) != 1 || events[0].Kind != GestureTouchCancel {
		t.Fatalf("cancel events = %#v", events)
	}
}
