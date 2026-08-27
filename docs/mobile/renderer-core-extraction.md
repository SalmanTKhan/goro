# Raw WGPU renderer interface extraction

## Boundary

The production renderer remains in `render/gpu_renderer.go`, but its draw core
now accepts host-owned raw WGPU resources through:

- `render.GPUDeviceContext`: supplied device, queue, and configured surface format;
- `render.FrameTarget`: supplied target view, dimensions, and format;
- `render.NewGPURenderer`: constructs Goro pipelines from the supplied device;
- `(*render.GPURenderer).DrawTarget`: records and submits a frame without
  acquiring or presenting a surface texture.

The desktop `Draw` method remains as a compatibility wrapper. It obtains the
desktop surface view and framebuffer size from `gogpu.Context`, then delegates
to `DrawTarget`. Android owns `ANativeWindow`, surface configuration, acquire,
and present in `android/host/go/main.go`.

## Context and texture audit

Renderer-owned GPU work uses the supplied raw device and queue for buffers,
shaders, bind groups, pipelines, encoders, render passes, submissions, and
depth resources. The only remaining `gogpu.Context` dependency is the desktop
compatibility wrapper around surface acquisition and presentation.

Image uploads no longer use `gogpu.Texture`. `ensureTexture` creates a raw
`wgpu.Texture`, uploads CPU RGBA pixels with `Queue.WriteTexture`, and creates a
raw `wgpu.TextureView` for bind groups. Upload rows are padded to WebGPU's
256-byte alignment requirement without changing the CPU image representation.
Texture replacement releases both the view and texture.

## Depth and target ownership

The renderer owns its depth texture and view, recreating them when target
dimensions change. The host owns the transient surface texture and view. A
surface view is never retained by the renderer after `DrawTarget` returns.

## Android proof path

The Android host constructs `RawGPUContext` from its WGPU device and queue,
creates the production `GPURenderer`, builds a deterministic 2x2 texture
fixture, and submits it to the acquired surface view. The fixture intentionally
uses a non-256-byte row so the shared padding path is exercised.

This phase does not claim full map rendering. GRF/GND probing remains a
resource-access checkpoint; terrain upload and camera integration are the next
renderer phase.

## Dependency and platform rule

The renderer-core extraction contains no Android, JNI, `ANativeWindow`, WGPU
surface, or desktop UI dependency. Android uses the nested host module's
aligned WGPU version and passes only the raw handles required by the renderer.
