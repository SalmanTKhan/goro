package render

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/kivutar/goro/capture"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/glog"
)

const captureReadbackSlots = 3

type captureFrameMeta struct {
	still       *stillCaptureJob
	record      bool
	pts         time.Duration
	pixelFormat capture.PixelFormat
}

type readbackSlot struct {
	buffer      *wgpu.Buffer
	pending     *wgpu.MapPending
	copyQueued  bool
	busy        bool
	meta        captureFrameMeta
	width       int
	height      int
	bytesPerRow int
}

// captureReadbackRing copies the completed production surface into one of
// three GPU buffers. Copy encoding is part of the production command buffer;
// map polling happens on later frames and never waits for the GPU.
type captureReadbackRing struct {
	device  *wgpu.Device
	slots   [captureReadbackSlots]readbackSlot
	width   int
	height  int
	stride  int
	dropped atomic.Uint64
}

func newCaptureReadbackRing(device *wgpu.Device) *captureReadbackRing {
	return &captureReadbackRing{device: device}
}

func (r *captureReadbackRing) prepare(width, height int) error {
	if r == nil || r.device == nil {
		return fmt.Errorf("capture GPU device is unavailable")
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid capture dimensions %dx%d", width, height)
	}
	if r.width == width && r.height == height && r.stride > 0 {
		return nil
	}
	if !r.idle() {
		return nil
	}
	for i := range r.slots {
		if r.slots[i].buffer != nil {
			r.slots[i].buffer.Release()
		}
		r.slots[i] = readbackSlot{}
	}
	r.width, r.height = width, height
	r.stride = align256(width * 4)
	size := uint64(r.stride * height)
	for i := range r.slots {
		buffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{
			Label: "goro-capture-readback",
			Size:  size,
			Usage: wgpu.BufferUsageMapRead | wgpu.BufferUsageCopyDst,
		})
		if err != nil {
			r.releaseBuffers()
			return fmt.Errorf("create capture readback buffer: %w", err)
		}
		r.slots[i] = readbackSlot{buffer: buffer, width: width, height: height, bytesPerRow: r.stride}
	}
	return nil
}

func (r *captureReadbackRing) encode(encoder *wgpu.CommandEncoder, source *wgpu.Texture, width, height int, meta captureFrameMeta) bool {
	if r == nil || encoder == nil || source == nil || r.width != width || r.height != height {
		if r != nil {
			r.dropped.Add(1)
		}
		return false
	}
	for i := range r.slots {
		slot := &r.slots[i]
		if slot.busy {
			continue
		}
		encoder.CopyTextureToBuffer(source, slot.buffer, []wgpu.BufferTextureCopy{{
			BufferLayout: wgpu.ImageDataLayout{
				Offset:       0,
				BytesPerRow:  uint32(r.stride), //nolint:gosec // stride derives from positive dimensions
				RowsPerImage: uint32(height),   //nolint:gosec // height is validated by prepare
			},
			TextureBase: wgpu.ImageCopyTexture{Texture: source},
			Size:        wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}, //nolint:gosec // dimensions are positive
		}})
		slot.busy = true
		slot.copyQueued = true
		slot.meta = meta
		return true
	}
	r.dropped.Add(1)
	return false
}

func (r *captureReadbackRing) afterSubmit() {
	if r == nil {
		return
	}
	for i := range r.slots {
		slot := &r.slots[i]
		if !slot.copyQueued || slot.pending != nil {
			continue
		}
		pending, err := slot.buffer.MapAsync(wgpu.MapModeRead, 0, uint64(slot.bytesPerRow*slot.height))
		if err != nil {
			slot.busy, slot.copyQueued = false, false
			r.dropped.Add(1)
			glog.Warnf("capture readback map failed: %v", err)
			continue
		}
		slot.copyQueued = false
		slot.pending = pending
	}
}

func (r *captureReadbackRing) poll(consume func(capture.Frame, captureFrameMeta) bool) {
	if r == nil || r.device == nil {
		return
	}
	r.device.Poll(wgpu.PollPoll)
	for i := range r.slots {
		slot := &r.slots[i]
		if !slot.busy || slot.pending == nil {
			continue
		}
		ready, err := slot.pending.Status()
		if !ready {
			continue
		}
		meta := slot.meta
		slot.pending = nil
		if err != nil {
			slot.busy = false
			r.dropped.Add(1)
			glog.Warnf("capture readback failed: %v", err)
			continue
		}
		mapped, err := slot.buffer.MappedRange(0, uint64(slot.bytesPerRow*slot.height))
		if err != nil {
			slot.busy = false
			r.dropped.Add(1)
			glog.Warnf("capture readback map range failed: %v", err)
			continue
		}
		pixels := make([]byte, slot.bytesPerRow*slot.height)
		copy(pixels, mapped.Bytes())
		_ = slot.buffer.Unmap()
		slot.busy = false
		slot.meta = captureFrameMeta{}
		if consume != nil {
			consume(capture.Frame{
				Width: slot.width, Height: slot.height, Stride: slot.bytesPerRow,
				PixelFormat: meta.pixelFormat, PTS: meta.pts, Pixels: pixels,
			}, meta)
		}
	}
}

func (r *captureReadbackRing) idle() bool {
	if r == nil {
		return true
	}
	for i := range r.slots {
		if r.slots[i].busy || r.slots[i].copyQueued || r.slots[i].pending != nil {
			return false
		}
	}
	return true
}

func (r *captureReadbackRing) droppedFrames() uint64 {
	if r == nil {
		return 0
	}
	return r.dropped.Load()
}

func (r *captureReadbackRing) release() {
	if r == nil || !r.idle() {
		return
	}
	r.releaseBuffers()
	r.width, r.height, r.stride = 0, 0, 0
}

func (r *captureReadbackRing) releaseBuffers() {
	for i := range r.slots {
		if r.slots[i].buffer != nil {
			r.slots[i].buffer.Release()
		}
		r.slots[i] = readbackSlot{}
	}
}

func align256(value int) int {
	return (value + 255) &^ 255
}

type stillCaptureJob struct {
	path    string
	options capture.ScreenshotOptions
	finish  func(string, error)
}

type captureRuntime struct {
	device        *wgpu.Device
	config        config.CaptureConfig
	ring          *captureReadbackRing
	stillQueue    chan stillCaptureResult
	stillDone     chan struct{}
	still         *stillCaptureJob
	recorder      *capture.AsyncRecorder
	recordPath    string
	recordStarted time.Time
	recordFPS     int
	recordFrames  uint64
	recordNextPTS time.Duration
	stopping      bool
	resizeStop    bool
}

type stillCaptureResult struct {
	job   *stillCaptureJob
	frame capture.Frame
}

func newCaptureRuntime(device *wgpu.Device, cfg config.CaptureConfig) *captureRuntime {
	r := &captureRuntime{
		device: device, config: cfg, ring: newCaptureReadbackRing(device),
		stillQueue: make(chan stillCaptureResult, 2), stillDone: make(chan struct{}),
	}
	go r.runStillEncoder()
	return r
}

func (r *captureRuntime) runStillEncoder() {
	defer close(r.stillDone)
	for result := range r.stillQueue {
		err := capture.EncodeStill(result.job.path, result.frame, result.job.options)
		if result.job.finish != nil {
			result.job.finish(result.job.path, err)
		}
	}
}

func (r *captureRuntime) poll() {
	if r == nil {
		return
	}
	r.ring.poll(func(frame capture.Frame, meta captureFrameMeta) bool {
		if meta.still != nil {
			select {
			case r.stillQueue <- stillCaptureResult{job: meta.still, frame: frame}:
			default:
				if meta.still.finish != nil {
					meta.still.finish(meta.still.path, fmt.Errorf("still capture queue is full"))
				}
			}
		}
		if meta.record && r.recorder != nil {
			r.recorder.Submit(frame)
		}
		return true
	})
}

func (r *captureRuntime) startRecording(options capture.RecordingOptions) error {
	if r == nil {
		return fmt.Errorf("capture runtime is unavailable")
	}
	if r.recorder != nil {
		return fmt.Errorf("recording is already active")
	}
	encoder, err := capture.NewEncoder(options, r.config.FFmpegPath)
	if err != nil {
		return err
	}
	recorder := capture.NewAsyncRecorder(encoder, captureReadbackSlots*2)
	if err := recorder.Start(options); err != nil {
		return err
	}
	r.recorder = recorder
	r.recordPath = options.Path
	r.recordStarted = time.Now()
	r.recordFPS = options.FPS
	r.recordFrames = 0
	r.recordNextPTS = 0
	r.stopping = false
	r.resizeStop = false
	return nil
}

func (r *captureRuntime) requestStill(job *stillCaptureJob) error {
	if r == nil {
		return fmt.Errorf("capture runtime is unavailable")
	}
	if r.still != nil {
		return fmt.Errorf("a screenshot is already pending")
	}
	r.still = job
	return nil
}

func (r *captureRuntime) requestStop(resize bool) {
	if r == nil || r.recorder == nil {
		return
	}
	r.stopping = true
	r.resizeStop = r.resizeStop || resize
}

func (r *captureRuntime) encode(encoder *wgpu.CommandEncoder, source *wgpu.Texture, width, height int, format gputypes.TextureFormat, pts time.Duration) bool {
	if r == nil {
		return false
	}
	record := false
	if r.recorder != nil && !r.stopping {
		record, pts = r.nextRecordingFrame(pts)
	}
	if r.still == nil && !record {
		return false
	}
	if err := r.ring.prepare(width, height); err != nil {
		glog.Warnf("capture readback unavailable: %v", err)
		return false
	}
	pixelFormat := capture.PixelFormatBGRA8
	if format == gputypes.TextureFormatRGBA8Unorm || format == gputypes.TextureFormatRGBA8UnormSrgb {
		pixelFormat = capture.PixelFormatRGBA8
	}
	meta := captureFrameMeta{still: r.still, record: record, pts: pts, pixelFormat: pixelFormat}
	if r.ring.encode(encoder, source, width, height, meta) {
		if r.still != nil {
			r.still = nil
		}
		return true
	}
	return false
}

// nextRecordingFrame converts render-loop time into a fixed-FPS output clock.
// It also prevents a 30 FPS recording from duplicating every 60 Hz display
// frame. A late render frame advances the clock by one frame and is counted as
// a drop if the GPU readback ring cannot accept it.
func (r *captureRuntime) nextRecordingFrame(elapsed time.Duration) (bool, time.Duration) {
	if r == nil || r.recordFPS <= 0 || elapsed < r.recordNextPTS {
		return false, 0
	}
	interval := time.Second / time.Duration(r.recordFPS)
	pts := time.Duration(r.recordFrames) * interval
	r.recordFrames++
	r.recordNextPTS = time.Duration(r.recordFrames) * interval
	return true, pts
}

func (r *captureRuntime) afterSubmit() {
	if r != nil {
		r.ring.afterSubmit()
	}
}

func (r *captureRuntime) finishIfReady(finishRecording func(string, error), resize func(string)) {
	if r == nil || r.recorder == nil || !r.stopping || !r.ring.idle() {
		return
	}
	path := r.recordPath
	err := r.recorder.Close()
	dropped := r.recorder.Dropped() + r.ring.droppedFrames()
	if dropped > 0 {
		glog.Warnf("capture dropped frames=%d path=%s", dropped, path)
	}
	r.recorder = nil
	r.recordPath = ""
	r.recordStarted = time.Time{}
	r.recordFPS = 0
	r.recordFrames = 0
	r.recordNextPTS = 0
	if finishRecording != nil {
		finishRecording(path, err)
	}
	if r.resizeStop && resize != nil {
		resize(path)
	}
	r.resizeStop = false
}

func (r *captureRuntime) close(finishRecording func(string, error)) {
	if r == nil {
		return
	}
	// Shutdown is the one lifecycle boundary where waiting for the device is
	// appropriate: it lets the last submitted readback enter the normal poll
	// path before the encoder is finalized. Gameplay frames never use PollWait.
	r.poll()
	if r.device != nil {
		r.device.Poll(wgpu.PollWait)
		r.poll()
	}
	if r.recorder != nil {
		path := r.recordPath
		err := r.recorder.Close()
		dropped := r.recorder.Dropped() + r.ring.droppedFrames()
		if dropped > 0 {
			glog.Warnf("capture dropped frames=%d path=%s", dropped, path)
		}
		if finishRecording != nil {
			finishRecording(path, err)
		}
		r.recorder = nil
		r.recordStarted = time.Time{}
		r.recordFPS = 0
		r.recordFrames = 0
		r.recordNextPTS = 0
	}
	if r.still != nil {
		job := r.still
		r.still = nil
		if job.finish != nil {
			job.finish(job.path, capture.ErrClosed)
		}
	}
	close(r.stillQueue)
	<-r.stillDone
	r.ring.release()
}

func (r *runner) prepareCapture() {
	if r == nil || r.gpu == nil {
		return
	}
	if r.capture == nil {
		r.capture = newCaptureRuntime(r.gpu.dev, r.captureCfg)
	}
	r.capture.poll()

	if requester, ok := r.game.(captureOptionsRequester); ok {
		if options, path, pending := requester.ConsumeCaptureRequest(); pending {
			if completer, ok := r.game.(screenshotRequester); ok {
				job := &stillCaptureJob{path: path, options: options, finish: completer.CompleteScreenshot}
				if err := r.capture.requestStill(job); err != nil {
					completer.CompleteScreenshot(path, err)
				}
			}
		}
	} else if requester, ok := r.game.(screenshotRequester); ok {
		if path, pending := requester.ConsumeScreenshotRequest(); pending {
			job := &stillCaptureJob{path: path, options: capture.ScreenshotOptions{Format: capture.StillPNG}, finish: requester.CompleteScreenshot}
			if err := r.capture.requestStill(job); err != nil {
				requester.CompleteScreenshot(path, err)
			}
		}
	}

	if requester, ok := r.game.(recordingRequester); ok {
		if options, pending := requester.ConsumeRecordingStart(); pending {
			if err := r.capture.startRecording(options); err != nil {
				requester.CompleteRecording(options.Path, err)
			}
		}
		if requester.ConsumeRecordingStop() {
			r.capture.requestStop(false)
		}
	}
	r.finishCapture()
}

func (r *runner) finishCapture() {
	if r == nil || r.capture == nil {
		return
	}
	var finish func(string, error)
	var resize func(string)
	if requester, ok := r.game.(recordingRequester); ok {
		finish = requester.CompleteRecording
		resize = requester.RecordingResize
	}
	r.capture.finishIfReady(finish, resize)
}

func (r *runner) closeCapture() {
	if r == nil || r.capture == nil {
		return
	}
	var finish func(string, error)
	if requester, ok := r.game.(recordingRequester); ok {
		finish = requester.CompleteRecording
	}
	r.capture.close(finish)
	r.capture = nil
}
