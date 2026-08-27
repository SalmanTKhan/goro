package input

import (
	"math"
	"sort"
	"time"
)

type GestureKind uint8

const (
	GestureTap GestureKind = iota
	GestureLongPress
	GestureDragStart
	GestureDrag
	GestureTwoFingerDrag
	GesturePinch
	GestureTouchCancel
)

type GestureConfig struct {
	TapMaxMovement    float64
	TapMaxDuration    time.Duration
	LongPressDuration time.Duration
	DragActivation    float64
	PinchActivation   float64
}

func DefaultGestureConfig() GestureConfig {
	return GestureConfig{
		TapMaxMovement:    18,
		TapMaxDuration:    350 * time.Millisecond,
		LongPressDuration: 550 * time.Millisecond,
		DragActivation:    12,
		PinchActivation:   2,
	}
}

type TouchFrame struct {
	Points []TouchPoint
	At     time.Time
	Cancel bool
}

type GestureEvent struct {
	Kind     GestureKind
	ID       TouchID
	Position WorldPosition
	DeltaX   float64
	DeltaY   float64
	Delta    float64
}

type touchTrack struct {
	point     TouchPoint
	start     TouchPoint
	startedAt time.Time
	dragging  bool
	longPress bool
}

type GestureRecognizer struct {
	config       GestureConfig
	tracks       map[TouchID]touchTrack
	previous     []TouchPoint
	pinching     bool
	lastDistance float64
	multiTouch   bool
	lastMidpoint WorldPosition
}

func NewGestureRecognizer(config GestureConfig) *GestureRecognizer {
	if config.TapMaxMovement <= 0 || config.TapMaxDuration <= 0 || config.LongPressDuration <= 0 || config.DragActivation <= 0 || config.PinchActivation <= 0 {
		config = DefaultGestureConfig()
	}
	return &GestureRecognizer{config: config, tracks: make(map[TouchID]touchTrack)}
}

func (r *GestureRecognizer) Reset() {
	if r == nil {
		return
	}
	r.tracks = make(map[TouchID]touchTrack)
	r.previous = nil
	r.pinching = false
	r.lastDistance = 0
	r.multiTouch = false
	r.lastMidpoint = WorldPosition{}
}

func (r *GestureRecognizer) Update(frame TouchFrame) []GestureEvent {
	if r == nil {
		return nil
	}
	if frame.At.IsZero() {
		frame.At = time.Unix(0, 0)
	}
	points := append([]TouchPoint(nil), frame.Points...)
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })
	if frame.Cancel {
		r.Reset()
		return []GestureEvent{{Kind: GestureTouchCancel}}
	}

	current := make(map[TouchID]TouchPoint, len(points))
	for _, point := range points {
		current[point.ID] = point
		if _, ok := r.tracks[point.ID]; !ok {
			r.tracks[point.ID] = touchTrack{point: point, start: point, startedAt: frame.At}
		}
	}

	var events []GestureEvent
	if len(points) >= 2 {
		distance := distanceBetween(points[0], points[1])
		center := midpoint(points[0], points[1])
		if !r.pinching {
			r.pinching = true
			r.lastDistance = distance
			r.lastMidpoint = center
		} else if delta := distance - r.lastDistance; math.Abs(delta) >= r.config.PinchActivation {
			events = append(events, GestureEvent{Kind: GesturePinch, Position: midpoint(points[0], points[1]), Delta: delta})
			r.lastDistance = distance
		}
		if r.multiTouch {
			deltaX := center.X - r.lastMidpoint.X
			deltaY := center.Y - r.lastMidpoint.Y
			if deltaX != 0 || deltaY != 0 {
				events = append(events, GestureEvent{Kind: GestureTwoFingerDrag, Position: center, DeltaX: deltaX, DeltaY: deltaY})
			}
		}
		r.multiTouch = true
		r.lastMidpoint = center
	} else {
		r.pinching = false
		r.lastDistance = 0
		r.lastMidpoint = WorldPosition{}
	}

	for _, point := range points {
		track := r.tracks[point.ID]
		previousPoint := track.point
		movementX := float64(point.X - track.start.X)
		movementY := float64(point.Y - track.start.Y)
		movement := math.Hypot(movementX, movementY)
		startedDrag := false
		if len(points) == 1 && !r.multiTouch && !track.dragging && movement >= r.config.DragActivation {
			track.dragging = true
			startedDrag = true
			events = append(events, GestureEvent{Kind: GestureDragStart, ID: point.ID, Position: pointPosition(point)})
		}
		if len(points) == 1 && !r.multiTouch && track.dragging && !startedDrag {
			deltaX := float64(point.X - previousPoint.X)
			deltaY := float64(point.Y - previousPoint.Y)
			if deltaX != 0 || deltaY != 0 {
				events = append(events, GestureEvent{Kind: GestureDrag, ID: point.ID, Position: pointPosition(point), DeltaX: deltaX, DeltaY: deltaY})
			}
		} else if !track.longPress && len(points) == 1 && !r.multiTouch && frame.At.Sub(track.startedAt) >= r.config.LongPressDuration && movement <= r.config.TapMaxMovement {
			track.longPress = true
			events = append(events, GestureEvent{Kind: GestureLongPress, ID: point.ID, Position: pointPosition(point)})
		}
		track.point = point
		r.tracks[point.ID] = track
	}

	for id, track := range r.tracks {
		if _, ok := current[id]; ok {
			continue
		}
		if !r.multiTouch && !r.pinching && !track.dragging && !track.longPress && frame.At.Sub(track.startedAt) <= r.config.TapMaxDuration && distanceBetween(track.start, track.point) <= r.config.TapMaxMovement {
			events = append(events, GestureEvent{Kind: GestureTap, ID: id, Position: pointPosition(track.point)})
		}
		delete(r.tracks, id)
	}
	if len(points) == 0 {
		r.multiTouch = false
	}
	r.previous = points
	return events
}

func pointPosition(point TouchPoint) WorldPosition {
	return WorldPosition{X: float64(point.X), Y: float64(point.Y)}
}

func midpoint(a, b TouchPoint) WorldPosition {
	return WorldPosition{X: float64(a.X+b.X) / 2, Y: float64(a.Y+b.Y) / 2}
}

func distanceBetween(a, b TouchPoint) float64 {
	return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))
}
