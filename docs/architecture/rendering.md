# Rendering

Package `render` wraps GoGPU/wgpu. It draws what it is given and knows nothing
about Ragnarok maps, actors, input, or UI behavior.

## Entry point

`render.Run(game, cfg.Window, cfg.Render)` creates the window, the gogpu/ui
`App`, and the frame loop. The loop calls the `render.Game` interface:

```go
type Game interface {
	Update() error
	Draw(*Frame)
	Resize(width, height int)
	InputState() *input.State
}
```

`app.Game` implements it. `render` injects back into the app through optional
interfaces (`SetQuitFunc`, `SetUIApp`), so `app`/`game` see only
`client.UIApp` rather than gogpu types.

Backend name: `gogpu-wgpu`. Graphics API is selectable (`--graphics-api vulkan|gles`).

## Files

| File | Role |
|---|---|
| `backend.go` | Window/app creation, frame loop, gogpu/ui bridge, screenshots, profiling |
| `frame.go`, `frame_commands.go` | `Frame`: the per-frame command buffer |
| `gpu_context.go`, `gpu_frame.go`, `gpu_renderer.go`, `gpu_shaders.go` | GPU pipelines, buffers, shaders |
| `cpu_raster.go` | CPU rasterization fallback path |
| `image.go`, `image_draw.go` | `Image` = CPU texture data plus draw helpers |
| `world_mesh.go` | Retained GPU meshes for static world geometry |
| `primitives.go`, `types.go` | Vertices, draw options, camera types |
| `cursor.go` | Hardware/software cursor handling |
| `ui_async.go`, `ui_image_canvas.go` | Async UI publishing, gogpu/ui canvas backed by `Image` |

## The Frame

`Frame` is a command buffer, not an immediate-mode surface. Per frame:
`BeginFrame()`, then commands, then the renderer consumes them.

- 2D: `Fill`, `DrawImage`, `DrawTriangles(Owned)`
- 3D: `SetCamera3D`, `DrawTriangles3D(Owned)`
- World: `DrawWorldMesh(*WorldMesh)` for retained geometry,
  `DrawWorldBillboard(WorldBillboardCommand)` for sprites

The `*Owned` variants hand ownership of the slices to the renderer, avoiding a
copy; do not reuse those slices afterwards.

## Performance model

Static GND surfaces, lightmapped subdivisions, and RSM placements are built once
per loaded map into retained `WorldMesh` objects in persistent GPU buffers.
Sprites (actors, NPCs, mobs, items, effects) go through a shared-quad billboard
pipeline with per-instance data instead of per-frame quad vertex slices. Camera
projection and fog are computed shader-side.

Render stats (`--stats`, world debug stats) expose `world_mesh_commands`,
`retained_world_meshes`, `world_billboards`, and remaining dynamic draws. Use
these numbers, not intuition, when judging a rendering change.

`--bench-seconds`, `--cpu-profile`, and the UI profile flag support measurement
runs. See `docs/render-gogpu-migration.md` for migration history.

## Where drawing logic lives

Actual RO drawing (sprites, ground, models, effects) lives in `game`:
`sprite_render.go`, `rsm_render.go`, `static_world_mesh.go`, `gnd.go`,
`effects_*.go`, `scene_projection.go`. If a change is about *what* is drawn it
belongs there; if it is about *how* pixels reach the GPU it belongs in `render`.
