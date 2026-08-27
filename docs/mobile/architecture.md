# Mobile architecture

The playable slice keeps one gameplay projection and one command boundary for
online and offline operation:

```text
embedded/external GRF + GND       rAthena checkout
            ↓                             ↓
     production renderer            build-offline-content
            ↓                             ↓
       app.NewOffline ─────────→ offline/content.json
            ↓                             ↓
 session.Session / world.World ← local authority
            ↓
       mobileui projection
            ↓
 layout + touch ownership
            ↓
      input.PlayerCommand
            ↓
   session.GameSession
      ┌─────┴─────┐
 OfflineSession OnlineSession
      │
 local authority
```

`app.NewOffline` creates the normal app graph, loads `offline/content.json`
through `res.Manager`, selects `game.WorldMode`, and omits the network client.
The renderer still uses the production GND/GRF map path; offline mode is not a
second renderer or a desktop-window export. rAthena YAML and scripts are
build-time inputs only; arbitrary script execution is not part of the client.

`session.Session` owns player position, vitals, inventory, equipment, skills,
and character data. `world.World` owns the active map projection and actors.
`mobileui` receives read-only presentation models. `session.GameSession` is the
authority seam for local gameplay commands. The offline runtime owns combat,
skills, drops, pickup, shops, storage, and the corresponding player-state
mutations. `game.WorldMode` validates world movement/picking and projects local
actors/items into the renderer-facing `world.World`. The online path continues
to use existing network helpers.

The Android host owns only platform concerns: Activity lifecycle, native window
references, WGPU frame acquisition/presentation, WindowInsets, and forwarding
touch events. It does not implement item rules or mutate session fields
directly.

## Startup

The Android host resolves resources in this order:

1. app-private extracted embedded fixture;
2. external-files override at `goro-data/data.grf` when present.

It creates `app.NewOffline`, which reads `offline/content.json` through
`res.Manager`, initializes the local level-one starter profile, and starts the
normal `game.WorldMode`. The first frame is a real map frame whose NPCs, shops,
monsters, drops, and warps come from the generated content pack.

The embedded offline pack also contains the item metadata and authored item
ACT/SPR/icon resources reachable from the starter inventory, selected shops, and
selected monster drop tables. Map BGM and the compact combat feedback sound set
are included in the same pack. The Android host enables the existing audio
package; ARM64 builds must use the `nofakecgo` build tag so Oto selects its
Android Oboe backend.

Build the offline content pack from the configured rAthena checkout:

```powershell
go run ./cmd/build-offline-content `
  --rathena-root <rathena-checkout> `
  --maps prontera,prt_fild05 `
  --starter-poring `
  --out offline\content.json
```

The importer resolves pre-renewal YAML imports and `db/import` overrides, reads
direct shop/monster/warp declarations from the active NPC include graph, and
records a source commit and content fingerprint. It only projects recognized
`itemheal` and `percentheal` effects; unsupported item scripts are warned and
disabled. Warps to maps outside the selected pack are warned and omitted so the
pack remains self-contained. Generated JSON is local/ignored and does not
replace proprietary client GRF/resource data. `--starter-poring` is an explicit
validation-fixture overlay; it places the imported monster definition at
Prontera `(82,98)` without changing the normal authority or map rules.

The current validation pair retains the source-backed Prontera `prt002` exit to
`prt_fild05` and the `prtf002` return exit. A map is only traversable when both
ends are included in the pack.

## Local authority and session contract

Offline commands are semantic and remain inside the existing gameplay path:

- ground tap → GAT-validated local movement;
- actor tap → local NPC/monster target selection;
- attack → fixed-step target validation using imported stats, damage, death,
  EXP, respawn, and deterministic imported-rate drops;
- skill target → semantic self/actor/ground command with SP and cooldown checks;
- floor item tap → range-validated pickup, offline auto-approach, and inventory
  projection;
- shop/storage commands → local zeny, quantity, stack, and persistence rules;
- NPC interaction → local deterministic dialog and source-backed shop option;
- use item → local source-backed `itemheal`/`percentheal` action;
- equip/unequip → existing inventory/equipment state shape;
- drop → quantity-validated local inventory removal;
- camera drag/pinch → existing camera command handling.

`session.GameSession` defines `State`, `HandleCommand`, and fixed-step `Update`.
`OfflineSession` implements it without importing Android, JNI, WGPU, or renderer
packages. `OnlineSession` implements the same contract on the existing network
authority path.

Imported content is source-backed, while the current Goro local-authority
formulas remain explicitly separate from a claim of full server parity.

## Persistence and lifecycle

`OfflineSession` coordinates pause/resume and save/load but does not duplicate
authoritative gameplay data. On shutdown, the host writes `offline-save.json` in
app-private files. A later launch restores the session projection and player
position. Surface destruction pauses the local session; surface recreation
resumes it.

The save format is versioned (`Version: 4`) and includes the content fingerprint
plus the shared player projection, storage, skills, hotkeys, statuses,
cooldowns, monsters, and projected floor drops. Map state is recreated from the
matching content pack. If the fingerprint changes, the world is re-seeded while
player progress and inventory are preserved. Fixed-step accumulator time, queued
events, and wall-clock scheduling are transient and reset on load. Older JSON
without a version and Version 2/3 JSON remain readable through migration
defaults. It is still local persistence, not an online character database.

## Scope

The current slice includes:

- generated source-backed `prontera` + `prt_fild05` content with NPC shops,
  field spawns, drops, and map warps;
- external `data.grf` override for local resource testing;
- landscape HUD with safe-area layout, target panel, minimap, menu, and skills;
- inventory/equipment screens with filtering, selection, details, scrolling,
  quantity modal, and semantic item actions;
- simple local NPC dialog and target selection;
- JSON save/load of local player position, vitals, progress, inventory,
  equipment, skills, hotkeys, statuses, storage, and stats.

It is intentionally not a replacement for the online session: no local server
protocol, database, cart, crafting, quests, or multiplayer protocol is
introduced. Goro local-authority formulas remain explicitly separate from a
claim of full rAthena gameplay parity.
