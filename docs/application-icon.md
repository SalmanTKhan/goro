# Application Icon

`internal/appicon/icon.png` is the canonical 32x32 pixel-art source. It is
embedded in the Go binary and passed to GoGPU for the native window.

## Runtime window icon

`render.Run` calls `WithIcon(appicon.Image())`, which GoGPU applies on X11
(`_NET_WM_ICON`) but not on Windows or Wayland. On Windows the window therefore
falls back to the generic icon, so `render/window_icon_windows.go` pushes the
icon from the executable's own resource (`goro_windows_*.syso`, icon group
`32512`) onto the window with `WM_SETICON` on the first frame; the non-Windows
build is a no-op (`render/window_icon_other.go`). This means the Windows
`.syso` (below) must exist for the in-app window icon, not just the taskbar one.

After replacing the source icon, regenerate the platform assets with:

```sh
go generate ./internal/appicon
```

The generator creates:

- `packaging/windows/goro.ico`, containing 16, 32, 48, 64, and 256 pixel
  variants. This feeds the Windows resource step below.
- `packaging/linux/goro.png`, a 256x256 nearest-neighbor variant for desktop
  packaging.
- `android/host/app/src/main/res/mipmap-{mdpi,hdpi,xhdpi,xxhdpi,xxxhdpi}/ic_launcher.png`,
  the Android launcher icon (48/72/96/144/192 px). The manifest references it as
  `@mipmap/ic_launcher`.

## Windows executable resources

`go generate .` (directive in `main.go`) runs `packaging/windows/generate.go`,
which combines `packaging/windows/goro.ico` with version fields derived from
`git describe` and writes `goro_windows_amd64.syso` and `goro_windows_arm64.syso`
at the repository root using a pinned `goversioninfo`. Go links each `.syso` into
the matching `GOOS`/`GOARCH` build by its filename suffix, so a plain
`CGO_ENABLED=0 go build -tags nofakecgo .` produces `goro.exe` with the taskbar
icon and a populated **Details** property tab (File/Product version =
`git describe` string, numeric version = `major.minor.patch.<commits-since-tag>`,
and the commit hash in **Comments**).

The `.syso` files are committed. Regenerate and commit them when cutting a
release tag; the release workflow also regenerates them from the exact tag. The
intermediate `packaging/windows/versioninfo.json` is git-ignored. There is no
Authenticode signature — that would need a code-signing certificate.

`packaging/linux/goro.desktop` is a template for distribution packages. Install
the desktop file and PNG in the appropriate XDG application and icon directories.

GoGPU v0.44.6 uses a fixed `gogpu` application ID on Wayland, so some Wayland
compositors will not associate `goro.desktop` with a directly launched window.
X11 receives the embedded icon directly.

The macOS releases remain bare binaries. They do not include app-bundle or ICNS
icon packaging.
