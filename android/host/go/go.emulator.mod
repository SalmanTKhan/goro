module github.com/kivutar/goro/android-host

go 1.26.4

require (
	github.com/gogpu/gputypes v0.5.2
	github.com/gogpu/wgpu v0.31.6
	github.com/kivutar/goro v0.0.0
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/log v1.0.0 // indirect
	github.com/charmbracelet/x/ansi v0.8.0 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13-0.20250311204145-2c3ea96c31dd // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/coregx/signals v0.1.0 // indirect
	github.com/ebitengine/oto/v3 v3.5.0-alpha.8 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/go-fonts/dejavu v0.3.4 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-text/typesetting v0.3.4 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/godexture/codec-mp3 v0.0.0 // indirect
	github.com/godexture/core v0.0.0 // indirect
	github.com/godexture/format-mp3 v0.0.0 // indirect
	github.com/godexture/metadata-id3 v0.0.0 // indirect
	github.com/godexture/sdk v0.0.0 // indirect
	github.com/gogpu/gg v0.48.16 // indirect
	github.com/gogpu/gogpu v0.44.6 // indirect
	github.com/gogpu/gpucontext v0.28.0 // indirect
	github.com/gogpu/naga v0.18.0 // indirect
	github.com/gogpu/ui v0.1.36 // indirect
	github.com/jfreymuth/pulse v0.1.1 // indirect
	github.com/klauspost/compress v1.19.2 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yuin/gopher-lua v1.1.2 // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d // indirect
	golang.org/x/image v0.43.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.38.0 // indirect
)

replace github.com/kivutar/goro => ../../..

// Phase 0I compatibility probe: compile the root renderer against the
// Android host's WGPU API before wiring concrete device handles.
replace github.com/gogpu/wgpu v0.30.19 => github.com/gogpu/wgpu v0.31.6

replace github.com/gogpu/wgpu => .\.emulator-wgpu

replace github.com/gogpu/gputypes v0.5.1 => github.com/gogpu/gputypes v0.5.2

// Keep gogpu and its platform-provider interface on the root module's
// compatible version while the Android host uses the newer raw WGPU handles.
replace github.com/gogpu/gpucontext v0.28.0 => github.com/gogpu/gpucontext v0.21.1

replace github.com/go-webgpu/goffi => .\.emulator-goffi

// Keep the Android host on the same audio decoder API as the root module.
// Module replacements are not inherited through the local goro replacement.
replace github.com/godexture/codec-mp3 => github.com/godexture/godec/plugins/codec-mp3 v0.0.0-20260621142744-bd77e78cfab1

replace github.com/godexture/format-mp3 => github.com/godexture/godec/plugins/format-mp3 v0.0.0-20260621142744-bd77e78cfab1

replace github.com/godexture/core => github.com/godexture/godec/core v0.0.0-20260621142744-bd77e78cfab1

replace github.com/godexture/sdk => github.com/godexture/godec/pkg v0.0.0-20260621142744-bd77e78cfab1

replace github.com/godexture/metadata-id3 => github.com/godexture/godec/plugins/metadata-id3 v0.0.0-20260621142744-bd77e78cfab1
