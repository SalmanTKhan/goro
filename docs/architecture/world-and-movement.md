# World and movement

Package `world` is the map model: what exists on the current map, where it is,
and how it moves. It has no knowledge of input, packets, UI, audio, or drawing.

## World

`world.New()` creates the model. Core contents:

- `Actor` map keyed by ID — players, NPCs, monsters, pets, homunculi,
  mercenaries. `UpsertActor` merges an incoming snapshot into an existing actor
  so partial packet updates do not clobber locally interpolated fields.
  `RemoveActor` removes it.
- `FloorItem` map — ground drops. `UpsertItem` / `RemoveItem`.
- Player position/direction — `SetPlayerPosition(x, y, dir)`.
- `Camera` — position and zoom.
- `MapProperty` — the map flags bitfield, with helpers such as
  `PlayerCombatEnabled()` and `PvPRankingEnabled()`. Reset per map with
  `ResetMapProperty`; PvP rankings clear with `ClearPvPRankings`.
- `WalkStep` — one step of a server-provided walk path.

Coordinates are RO tile coordinates. Screen projection is `game`'s job
(`scene_projection.go`).

## Movement

The server sends a start position, destination, and tick. `game/movement.go`
interpolates between tiles for smooth motion, plus walk cancellation and
direction updates. `game/pathfinding.go` computes local paths over the GAT
walkability grid for click-to-move and cursor snapping.

Gameplay policies related to movement live in `game`:

- `cursor.go`, `tile_cursor.go` — cursor state and snapping (`snap_targets`,
  `snap_items` config).
- `player_commands.go`, `player_state.go` — issuing move/attack/interact.
- `teleport_handlers.go` — teleport and warp modal flow.
- `bot_movement.go` — scripted movement for the Lua bot.

## Camera

`game/camera.go` implements smooth follow and zoom, bounded by the RSW
`CameraViewPoint` values loaded through `res` (some maps lock longitude).
