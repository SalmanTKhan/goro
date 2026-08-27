# Mobile map and warp slice

The map screen is a renderer-neutral projection of the current map raster,
player position, and warp declarations. `app.Game.MobileMapModel` reads those
values from the existing world/offline session; it does not own map state or
invent destination rules.

## Interaction

- Open Map from the HUD menu.
- Tap an exit to select it and expose its destination map and coordinates.
- Tap `Confirm Travel`, then confirm the destination.
- The controller emits the existing `input.CommandMoveTo` at the authoritative
  warp cell. `WorldMode.requestWalk` and `OfflineSession.TryWarpAt` continue to
  own movement and map-change behavior.
- Back closes the confirmation state first, then returns to the world HUD.

All touches inside the map surface, destination list, and confirmation modal
are UI-consumed. They cannot fall through to world movement, attack, or skill
targeting.

## Layout

Complex content is bounded to 1800 pixels on wide displays. The map consumes
the larger left region and exits/details use a compact right rail. The Fold 3
outer landscape viewport, 2268x832, is covered by the deterministic snapshot
matrix. Safe-area insets are applied through the shared `Viewport.SafeRect`.

The map raster reuses the existing minimap projection and player marker. Nearby
offline monsters, NPCs, and warps are projected as semantic markers from the
same world coordinates; hostile markers are distinct from NPC markers and the
selected target is emphasized. Warp markers are derived from the same raster
coordinates. Long exit and destination names are clipped before rendering; no
live GPU texture or Android type enters `mobileui`.

## Offline traversal fixture

The current Android validation pack is generated from the selected rAthena
maps `prontera,prt_fild05`. Its retained source-backed exits are:

- Prontera `prt002` at `(22,203)` → `prt_fild05` `(367,205)`;
- `prt_fild05` `prtf002` at `(373,205)` → Prontera `(26,203)`.

Starting at `(78,98)`, walk to the south gate or use the map screen to select
the exit. After the existing fade/handoff, the field map is active and its
imported spawns are available to target and fight. The field can return
through the west-side gate. A deterministic starter Poring remains at
Prontera `(82,98)` for the initial skill check.

Build the content and fixture with:

```powershell
go run ./cmd/build-offline-content `
  --rathena-root <rathena-checkout> `
  --maps prontera,prt_fild05 `
  --starter-poring `
  --out offline\content.json

go run ./cmd/build-render-fixture `
  --data-dir <ro-data> `
  --maps prontera,prt_fild05 `
  --offline-content offline\content.json `
  --out android\host\app\src\main\assets\goro-fixture\renderer-fixture.grf
```

The fixture builder closes every ACT/SPR pair referenced by the selected
offline monster spawns and reports missing dependencies before producing the
GRF.

## Fixtures and limitations

`cmd/mobile-ui-preview -screen map` supports `map-basic`, `map-many`,
`map-long-names`, and `map-empty`. Headless fixtures remain renderer-neutral;
the two-map source-backed traversal is validated through the offline content
and session tests rather than by simulating renderer map loads in `mobileui`.

The screen does not add a new warp packet or local teleport rule. Online warp
selection and server map changes remain outside this offline-first slice.
