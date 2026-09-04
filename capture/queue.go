package capture

import (
	"sync"
	"sync/atomic"
)

// FrameQueue is a bounded, non-blocking handoff. Submit never waits for an
// encoder; when full, the frame is dropped and Dropped increases.
type FrameQueue struct {
	frames   chan Frame
	closed   chan struct{}
	mu       sync.Mutex
	isClosed bool
	dropped  atomic.Uint64
}

func NewFrameQueue(capacity int) *FrameQueue {
	if capacity < 1 {
		capacity = 1
	}
	return &FrameQueue{
		frames: make(chan Frame, capacity),
		closed: make(chan struct{}),
	}
}

func (q *FrameQueue) Submit(frame Frame) bool {
	if q == nil {
		return false
	}
	clone, err := frame.Clone()
	if err != nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.isClosed {
		return false
	}
	select {
	case q.frames <- clone:
		return true
	default:
		q.dropped.Add(1)
		return false
	}
}

func (q *FrameQueue) Frames() <-chan Frame {
	if q == nil {
		return nil
	}
	return q.frames
}

func (q *FrameQueue) Close() {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.isClosed {
		return
	}
	q.isClosed = true
	close(q.closed)
	close(q.frames)
}

func (q *FrameQueue) Dropped() uint64 {
	if q == nil {
		return 0
	}
	return q.dropped.Load()
}

// AsyncRecorder drains the bounded queue on a worker and serializes encoder
// calls. Close first stops new submissions, drains queued frames, then calls
// the backend's Close so container finalization is deterministic.
type AsyncRecorder struct {
	encoder  Encoder
	queue    *FrameQueue
	done     chan struct{}
	errMu    sync.Mutex
	err      error
	start    sync.Once
	startErr error
	close    sync.Once
	closeErr error
}

func NewAsyncRecorder(encoder Encoder, capacity int) *AsyncRecorder {
	return &AsyncRecorder{encoder: encoder, queue: NewFrameQueue(capacity)}
}

func (r *AsyncRecorder) Start(options RecordingOptions) error {
	if r == nil || r.encoder == nil {
		return ErrClosed
	}
	r.start.Do(func() {
		r.startErr = r.encoder.Start(options)
		if r.startErr != nil {
			return
		}
		r.done = make(chan struct{})
		go r.run()
	})
	return r.startErr
}

func (r *AsyncRecorder) run() {
	defer close(r.done)
	for frame := range r.queue.frames {
		if err := r.encoder.Write(frame); err != nil {
			r.setError(err)
			return
		}
	}
}

func (r *AsyncRecorder) Submit(frame Frame) bool {
	if r == nil || r.done == nil || r.hasError() {
		return false
	}
	return r.queue.Submit(frame)
}

func (r *AsyncRecorder) Close() error {
	if r == nil || r.done == nil {
		if r != nil {
			return r.startErr
		}
		return nil
	}
	r.close.Do(func() {
		r.queue.Close()
		<-r.done
		r.closeErr = r.encoder.Close()
		if err := r.Error(); err != nil {
			r.closeErr = err
		}
	})
	return r.closeErr
}

func (r *AsyncRecorder) Dropped() uint64 {
	if r == nil {
		return 0
	}
	return r.queue.Dropped()
}

func (r *AsyncRecorder) Error() error {
	if r == nil {
		return nil
	}
	r.errMu.Lock()
	defer r.errMu.Unlock()
	return r.err
}

func (r *AsyncRecorder) hasError() bool { return r.Error() != nil }

func (r *AsyncRecorder) setError(err error) {
	if err == nil {
		return
	}
	r.errMu.Lock()
	if r.err == nil {
		r.err = err
	}
	r.errMu.Unlock()
}
