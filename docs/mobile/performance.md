# Mobile performance and usability plan

This document defines the optimization loop for the Android client. Targets are
budgets to validate on hardware, not claims about current device performance.

## Budgets and evidence

The primary target is a stable 60 FPS presentation with a 30 FPS battery mode
on thermally constrained devices. At 60 FPS, CPU frame work should remain below
12 ms at p95 so acquire/present and OS scheduling retain headroom. The initial
memory target is a steady process RSS below 1 GiB on the current two-map mobile
pack, with no sustained growth after repeated map and screen transitions.

Every device run should preserve `mobile-runtime-metrics.json` and record:

- device, Android version, resolution, refresh rate, and thermal state;
- map load and time to first map frame;
- steady FPS and average CPU frame time;
- peak and steady RSS;
- terrain build, texture decode, upload, and estimated GPU texture bytes;
- frame-buffer allocations after the first presented frame.

The existing metrics are directional. Add p50/p95/p99 CPU and GPU timings before
using them as a release gate.

## Phase 1: frame-loop and touch foundations

Implemented in the first optimization pass:

- Reuse `render.Frame` and its retained command slices until the surface size
  changes. The runtime metrics now report frame-buffer allocations; a steady
  session should remain at one allocation per surface size.
- Cache the configured WGPU surface format instead of asking the driver for
  surface capabilities on every frame.
- Apply a 12-logical-pixel touch slop. Small finger motion remains a tap, while
  crossing the threshold transfers the complete displacement to scrolling.
- Compile the Android ARM64 shared library and run the platform-neutral mobile
  input/layout suites in CI.

## Next measurements and changes

1. Capture a 60-second device trace in Prontera while idle, walking, fighting,
   opening inventory, and scrolling settings. Add frame-time percentiles and
   separate update, projection/UI, draw-build, GPU-submit, and present timing.
2. Stop rebuilding every mobile screen projection every frame. Refresh the HUD
   every frame, update the active full-screen model on revision changes, and
   keep unsolicited modal/session events on explicit dirty signals.
3. Split static HUD chrome from dynamic text, cooldowns, and minimap markers so
   movement does not rerasterize the complete widget surface.
4. Add explicit 30/60 FPS modes with fixed-step gameplay updates and independent
   render pacing. Rendering at 30 FPS must not reduce network pumping or command
   responsiveness to 30 Hz.
5. Measure fill-rate before adding render scale. If GPU-bound, render the world
   to a scaled intermediate target while keeping UI and touch coordinates at
   native resolution; use 1.0, 0.85, and 0.70 quality steps.
6. Add texture residency budgets, eviction telemetry, and map-transition soak
   tests before lowering asset quality globally.

## UI usability checks

- Verify every actionable control is at least 48 logical pixels and has no
  overlap with system gesture insets.
- Test taps and scroll starts with a real finger, not only emulator mouse input.
- Check the Fold outer `2268x832` target plus a narrow portrait phone and a
  conventional 20:9 landscape phone.
- Keep gameplay visible behind HUD chrome, while full-screen lists preserve
  readable type, deterministic back behavior, and scroll ownership.
