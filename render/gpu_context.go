package render

import (
	"fmt"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	"github.com/kivutar/goro/config"
)

// GPUDeviceContext contains only device-level objects shared across frames.
// Hosts retain ownership of the instance, adapter, device, queue, and
// surface lifecycle; the renderer borrows the device and queue.
type GPUDeviceContext interface {
	Device() *wgpu.Device
	Queue() *wgpu.Queue
	SurfaceFormat() gputypes.TextureFormat
}

// RawGPUContext adapts a host-owned raw WGPU device/queue to GPUDeviceContext.
// It has no Android or desktop dependencies.
type RawGPUContext struct {
	device        *wgpu.Device
	queue         *wgpu.Queue
	surfaceFormat gputypes.TextureFormat
}

func NewRawGPUContext(device *wgpu.Device, queue *wgpu.Queue, format gputypes.TextureFormat) (*RawGPUContext, error) {
	if device == nil {
		return nil, fmt.Errorf("raw GPU device is nil")
	}
	if queue == nil {
		queue = device.Queue()
	}
	if queue == nil {
		return nil, fmt.Errorf("raw GPU queue is nil")
	}
	return &RawGPUContext{device: device, queue: queue, surfaceFormat: format}, nil
}

func (c *RawGPUContext) Device() *wgpu.Device                  { return c.device }
func (c *RawGPUContext) Queue() *wgpu.Queue                    { return c.queue }
func (c *RawGPUContext) SurfaceFormat() gputypes.TextureFormat { return c.surfaceFormat }

// FrameTarget is the transient color target supplied by a host for one frame.
// The renderer never owns or releases the view.
type FrameTarget struct {
	View          *wgpu.TextureView
	Width, Height int
	Format        gputypes.TextureFormat
}

func (t FrameTarget) Valid() bool {
	return t.View != nil && t.Width > 0 && t.Height > 0
}

// GPURenderer is the production renderer core. Its device-level resources are
// reusable across desktop and raw-WGPU hosts.
type GPURenderer = gpuRenderer

func NewGPURenderer(context GPUDeviceContext, cfg config.RenderConfig) (*GPURenderer, error) {
	return newGPURendererFromProvider(context, cfg)
}
