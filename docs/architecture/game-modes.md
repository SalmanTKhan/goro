# Game modes (`game`)

`game` is the orchestration layer: it reacts to input and packets, drives world
state, triggers audio and effects, and composes UI. It is the largest package by
far and is deliberately split into one file per feature.

## The mode machine

```go
type Mode interface {
	Name() string
	Enter(client.Context)
	Update(client.Context) (Mode, error)
	Draw(client.Context, *render.Frame)
}
```

Optional interfaces a mode may also implement: `DrawOverlay`, `DrawUIOverlay`,
`FrameSubmitted`.

`game.NewManager(ctx, NewLoginMode())` holds the active mode. `Update` returns a
non-nil `Mode` to transition; the manager then calls `Enter` on it.
`Manager.UpdateContext` refreshes the shared `client.Context` (screen size,
runtime settings) each frame.

Two modes exist:

- `LoginMode` (`login.go`) — server connect, account login, character
  select/create/delete, background and fade phases.
- `WorldMode` (`world.go`) — in-game.

## LoginMode

Phase-driven (`loginPhase` with fades between phases). Handles server selection
from `clientinfo.xml`, `CA_LOGIN`, char-server ping keepalive, and the
character list. Windows come from `ui` and are published through
`publishPhaseWindow` / `clearLoginWindows`. `login_account_window.go`,
`login_character_select_window.go`, `login_character_create_window.go`, and
`login_mobile.go` hold the per-screen composition. On map-in it fades and
returns a `WorldMode`.

It already applies a subset of zone packets (parameter changes, actor bootstrap,
status effects, cart) so that state restored at login is not lost before
`WorldMode` takes over.

## WorldMode

`Enter` loads the map (RSW/GND/GAT, models, lightmaps, weather, BGM), prewarms
the first frame under a black cover, then fades in (`map_fade.go`).
`Update` drains packets, applies input, steps movement/effects/companions, and
returns a new mode on map change, character-select return, or disconnect.

Key collaborators inside `game`:

| Concern | Files |
|---|---|
| Packet reactions | `world_packets.go`, `session_state_updates.go`, `parameter_changes.go` |
| Drawing | `sprite_render.go`, `rsm_render.go`, `gnd.go`, `static_world_mesh.go`, `scene_projection.go`, `scene_clear.go` |
| Effects | `effects*.go`, `effects_str.go`, `skill_unit_models.go`, `level99_aura.go`, `less_effects.go` |
| Map presentation | `map_weather*.go`, `fog.go`, `map_fade.go`, `map_cell_update.go`, `rsw_effects.go` |
| Camera and picking | `camera.go`, `cursor.go`, `tile_cursor.go`, `npc_pick.go` |
| UI wiring | `ui_actions.go`, `ui_assets.go`, `manager.go` |
| Mobile command handling | `mobile_*.go` |

`worldUI` inside `world.go` centralizes whether keyboard input/shortcuts are
blocked by a focused UI control — check `KeyboardShortcutsBlocked` before adding
a new hotkey.

## Adding a feature

1. Parse the protocol in `network`.
2. Store durable state in `session`, map-scoped state in `world`.
3. React and orchestrate in a new `game/<feature>.go` (+ test).
4. Compose windows in `ui/<feature>_window.go` (+ test).
5. If mobile should reach it, add a `input.PlayerCommand` kind and a `mobileui`
   model/controller.
