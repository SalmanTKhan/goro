package input

type DesktopFrame struct {
	Position        WorldPosition
	LeftJustPressed bool
	RightDown       bool
	MouseDX         float64
	MouseDY         float64
	WheelY          float64
}

type DesktopInputAdapter struct {
	Picker WorldPicker
	UI     UIHitTester
	Sink   CommandSink
}

func (a DesktopInputAdapter) Update(frame DesktopFrame) {
	if a.Sink == nil {
		return
	}
	if frame.RightDown && (frame.MouseDX != 0 || frame.MouseDY != 0) {
		a.Sink.Emit(PlayerCommand{Kind: CommandRotateCamera, DeltaX: frame.MouseDX, DeltaY: frame.MouseDY})
	}
	if frame.WheelY != 0 {
		a.Sink.Emit(PlayerCommand{Kind: CommandZoomCamera, DeltaY: frame.WheelY})
	}
	if !frame.LeftJustPressed || a.UI != nil && a.UI.ConsumeTouch(TouchPoint{X: int(frame.Position.X), Y: int(frame.Position.Y)}) {
		return
	}
	a.emitPick(frame.Position)
}

type MobileInputAdapter struct {
	Recognizer *GestureRecognizer
	Picker     WorldPicker
	UI         UIHitTester
	Sink       CommandSink
	controls   MobileControls
	uiTouches  map[TouchID]bool
}

func NewMobileInputAdapter(config GestureConfig, picker WorldPicker, ui UIHitTester, sink CommandSink) *MobileInputAdapter {
	return &MobileInputAdapter{Recognizer: NewGestureRecognizer(config), Picker: picker, UI: ui, Sink: sink, controls: DefaultMobileControls(), uiTouches: make(map[TouchID]bool)}
}

func NewMobileInputAdapterWithControls(controls MobileControls, picker WorldPicker, ui UIHitTester, sink CommandSink) *MobileInputAdapter {
	controls = controls.Normalized()
	return &MobileInputAdapter{Recognizer: NewGestureRecognizer(controls.GestureConfig()), Picker: picker, UI: ui, Sink: sink, controls: controls, uiTouches: make(map[TouchID]bool)}
}

func (a *MobileInputAdapter) Controls() MobileControls {
	if a == nil {
		return DefaultMobileControls()
	}
	return a.controls
}

// SetControls applies the mobile policy at the next input update without
// replacing the adapter or losing an existing picker/UI connection.
func (a *MobileInputAdapter) SetControls(controls MobileControls) {
	if a == nil {
		return
	}
	a.controls = controls.Normalized()
	if a.Recognizer == nil {
		a.Recognizer = NewGestureRecognizer(a.controls.GestureConfig())
		return
	}
	a.Recognizer.config = a.controls.GestureConfig()
	a.Recognizer.Reset()
}

func (a *MobileInputAdapter) Update(frame TouchFrame) {
	if a == nil || a.Recognizer == nil || a.Sink == nil {
		return
	}
	if frame.Cancel {
		a.uiTouches = make(map[TouchID]bool)
		a.Recognizer.Reset()
		a.Sink.Emit(PlayerCommand{Kind: CommandCancelAction})
		return
	}
	filtered := make([]TouchPoint, 0, len(frame.Points))
	active := make(map[TouchID]bool, len(frame.Points))
	for _, point := range frame.Points {
		active[point.ID] = true
		if _, known := a.uiTouches[point.ID]; !known && a.UI != nil && a.UI.ConsumeTouch(point) {
			a.uiTouches[point.ID] = true
		}
		if !a.uiTouches[point.ID] {
			filtered = append(filtered, point)
		}
	}
	for id := range a.uiTouches {
		if !active[id] {
			delete(a.uiTouches, id)
		}
	}
	if len(a.uiTouches) > 0 {
		a.Recognizer.Reset()
		return
	}
	for _, event := range a.Recognizer.Update(TouchFrame{Points: filtered, At: frame.At}) {
		a.emitGesture(event)
	}
}

func (a *MobileInputAdapter) emitGesture(event GestureEvent) {
	switch event.Kind {
	case GestureTap:
		a.emitPick(event.Position)
	case GestureLongPress:
		a.emitLongPress(event.Position)
	case GestureDragStart:
		if a.controls.MovementMode == MovementHoldToMove {
			a.emitGroundMove(event.Position)
		}
	case GestureDrag:
		if a.controls.MovementMode == MovementHoldToMove {
			a.emitGroundMove(event.Position)
		}
	case GestureTwoFingerDrag:
		deltaY := event.DeltaY * a.controls.CameraSensitivity
		if a.controls.InvertCameraY {
			deltaY = -deltaY
		}
		a.Sink.Emit(PlayerCommand{Kind: CommandRotateCamera, DeltaX: event.DeltaX * a.controls.CameraSensitivity, DeltaY: deltaY})
	case GesturePinch:
		a.Sink.Emit(PlayerCommand{Kind: CommandZoomCamera, DeltaY: event.Delta * a.controls.ZoomSensitivity})
	case GestureTouchCancel:
		a.Sink.Emit(PlayerCommand{Kind: CommandCancelAction})
	}
}

func (a *MobileInputAdapter) emitLongPress(position WorldPosition) {
	if a == nil || a.Picker == nil {
		return
	}
	target, ok := a.Picker.Pick(position)
	if !ok {
		return
	}
	switch target.Kind {
	case TargetActor, TargetNPC:
		a.Sink.Emit(PlayerCommand{Kind: CommandInspectActor, ActorID: target.ActorID, Position: position})
	case TargetGround:
		if a.controls.MovementMode == MovementHoldToMove {
			a.emitPickedTarget(target)
		}
	}
}

func (a *MobileInputAdapter) emitGroundMove(position WorldPosition) {
	if a == nil || a.Picker == nil {
		return
	}
	if target, ok := a.Picker.Pick(position); ok && target.Kind == TargetGround {
		a.Sink.Emit(PlayerCommand{Kind: CommandMoveTo, Position: target.Position})
	}
}

func (a DesktopInputAdapter) emitPick(position WorldPosition) {
	if a.Picker == nil {
		return
	}
	if target, ok := a.Picker.Pick(position); ok {
		emitPickedTarget(a.Sink, target)
	}
}

func (a *MobileInputAdapter) emitPick(position WorldPosition) {
	if a.Picker == nil {
		return
	}
	if target, ok := a.Picker.Pick(position); ok {
		emitPickedTarget(a.Sink, target)
	}
}

func (a *MobileInputAdapter) emitPickedTarget(target PickedTarget) {
	emitPickedTarget(a.Sink, target)
}

func emitPickedTarget(sink CommandSink, target PickedTarget) {
	if sink == nil {
		return
	}
	switch target.Kind {
	case TargetGround:
		sink.Emit(PlayerCommand{Kind: CommandMoveTo, Position: target.Position})
	case TargetActor:
		kind := CommandSelectActor
		if target.Hostile {
			kind = CommandAttackActor
		}
		sink.Emit(PlayerCommand{Kind: kind, ActorID: target.ActorID})
	case TargetNPC:
		sink.Emit(PlayerCommand{Kind: CommandInteractActor, ActorID: target.ActorID})
	case TargetVending:
		sink.Emit(PlayerCommand{Kind: CommandOpenVending, ActorID: target.ActorID})
	case TargetItem:
		sink.Emit(PlayerCommand{Kind: CommandPickUpItem, ItemID: target.ItemID})
	}
}
