# Tooling, configuration, build, and release

## Configuration (`config`)

Layered, later wins: built-in defaults → user config path → `goro.ini` (or
`--config <path>`) → environment → command-line flags. `config.LoadConfig(args)`
returns the whole `Config`; it never depends on runtime state.

Sections: `Window`, `Packet` (`client_date`, `profile`), `Login`, `Audio`,
`Render` (`graphics_api`, `vsync`, `fps`, `no_ui`, `async_ui`, profiling and
benchmark options), `Network` (`trace`), `MobileSession` (mode + server),
`Fog`, `Gameplay` (`no_shift`, `no_ctrl`, `less_effects`, `snap_targets`,
`snap_items`, `force_user_ai`), `Mobile` / `MobileDisplay`, `Script`, `Log`.

`config.UserSettings` is the persisted subset the in-game settings window can
change (fullscreen, vsync, fps, volumes, gameplay toggles, mobile controls and
display). `server.go` holds `ServerConfig` used by the mobile online path.

## Logging (`glog`)

`glog.Configure(cfg.Log)` returns a close function; level and file come from
`[log]`. Use `glog.Fatalf` only from `main`.

## `cmd/` tools

| Command | Purpose |
|---|---|
| `grf-pack`, `grf-extract` | Create and extract GRF archives |
| `grf-to-pak` | Convert a GRF to deterministic chunked GOROPAK v1 without expanding it |
| `build-render-fixture` | Build a small GRF with one static map's resources for render tests |
| `build-offline-content` | Generate the deterministic local-authority pack (`offline/content.json`) |
| `build-mobile-assets` | Build modular mobile packs from a versioned asset project |
| `build-mobile-pack` | Build one deterministic pack plus a metrics manifest |
| `mobile-pack-editor` | Local browser UI for composing mobile packs |
| `benchmark-mobile-assets` | Compare PAK vs GRF artifacts under identical selections |
| `merge-mobile-metrics` | Attach measured Android metrics to a pack manifest |
| `mobile-ui-preview` | Preview `mobileui` layouts on the desktop |

## Build

```bash
CGO_ENABLED=0 go build -tags nofakecgo .
```

`nofakecgo` avoids duplicate fake-cgo symbol providers in the pure-Go GoGPU
dependency stack; `TODO.md` tracks removing the requirement. Do not introduce
dependencies that need cgo.

## CI (`.github/workflows/ci.yml`)

`go vet ./...` then `staticcheck ./...` on ubuntu, windows, and macos with
`CGO_ENABLED=0` and `GOFLAGS=-tags=nofakecgo`. Go version comes from `go.mod`.

## Release (`.github/workflows/release.yml`)

Triggered on GitHub release creation. Cross-compiles Windows/macOS/Linux for
amd64 and arm64 with `-ldflags "-s -w"` (`-H=windowsgui` added for Windows). The
Windows job runs `go generate .` first to restamp the resource objects from the
exact tag (needs `fetch-depth: 0` for `git describe`).

## Icon and packaging

`internal/appicon/icon.png` is the canonical 32x32 pixel-art source, embedded in
the binary and handed to GoGPU. Regenerate platform assets with
`go generate ./internal/appicon`, which writes `packaging/windows/goro.ico`,
`packaging/linux/goro.png`, and the Android launcher icons under
`android/host/app/src/main/res/mipmap-*/ic_launcher.png`.
`packaging/linux/goro.desktop` is a distribution template. macOS releases are bare binaries with no app bundle.

Windows resources are separate. `go generate .` runs `packaging/windows/generate.go`,
which reads `packaging/windows/goro.ico` plus `git describe` and writes
`goro_windows_amd64.syso` / `goro_windows_arm64.syso` at the repo root (icon +
`VERSIONINFO`, via a pinned `goversioninfo`). Go links each `.syso` into the
matching `GOOS`/`GOARCH` build by filename, so plain `go build .` gets the icon
and the version fields. These `.syso` files are committed; regenerate and commit
them when tagging a release. Details: `docs/application-icon.md`.

## Build version

`internal/buildinfo.Version()` returns a build identifier (`git describe` form,
e.g. `v0.9.0-5-gdb40e9f`, or `devel`). `go generate ./internal/buildinfo` writes
the committed `internal/buildinfo/version_gen.go`; if that is stale the function
falls back to the Go toolchain's VCS stamp. `render.Run` appends it to the
default window title. Regenerate and commit `version_gen.go` when tagging;
the release workflow restamps it (needs `fetch-depth: 0`).

## Website

`website/` is deployed to GitHub Pages by `.github/workflows/pages.yml` on
pushes to `main` that touch it.
