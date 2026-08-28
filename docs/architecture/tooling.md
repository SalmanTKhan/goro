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
amd64 and arm64 with `-ldflags "-s -w"`. Windows binaries get an icon compiled
in via `rsrc` from `packaging/windows/goro.ico`.

## Icon and packaging

`internal/appicon/icon.png` is the canonical 32x32 pixel-art source, embedded in
the binary and handed to GoGPU. Regenerate platform assets with
`go generate ./internal/appicon`, which writes `packaging/windows/goro.ico` and
`packaging/linux/goro.png`. `packaging/linux/goro.desktop` is a distribution
template. macOS releases are bare binaries with no app bundle. Details:
`docs/application-icon.md`.

## Website

`website/` is deployed to GitHub Pages by `.github/workflows/pages.yml` on
pushes to `main` that touch it.
