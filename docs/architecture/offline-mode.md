# Offline mode (local authority)

Offline play runs the same client graph as online — same rendering, same
projections, same mobile commands — but replaces the server with a local
authority. This is what makes the Android build usable without a rAthena
server, and it doubles as a deterministic test bed.

## Entry points

`app.NewOffline(cfg)` (starts in Prontera) and `app.NewOfflineAtMap(cfg, map)`
build the normal `app.Game` via `app.New` and then enter `WorldMode` directly
with local authority instead of `LoginMode`. The content pack is read from
`offline/content.json` through the resource manager, so it can ship inside a
GRF/PAK like any other resource.

## Authority

`session.GameSession` (`session/authority.go`) is the boundary:

```go
State() *Session
HandleCommand(input.PlayerCommand) bool
Update(time.Duration)
```

`session.OfflineSession` implements it. `authority.go` also holds the monster
simulation: `OfflineMonster` with an `OfflineMonsterState` machine
(`MonsterIdle`, `MonsterWander`, `MonsterAcquireTarget`, `MonsterChase`,
`MonsterAttack`, `MonsterDead`, `MonsterRespawn`), spawn anchors, and the local
combat resolution.

`Pause()` / `Resume()` stop and restart the simulation (used when the app is
backgrounded on Android). `Save(path, state)` / `Load(path, state)` persist an
`OfflineSave` / `OfflineRuntimeSave`, and `starter_loadout.go` seeds a new
character.

## Content pack

`session/offline_content.go` defines the data model decoded from
`offline/content.json`:

`OfflineContent` (with `OfflineContentSource`), `OfflineMap`, `OfflineItem` and
`OfflineUseEffect`, `OfflineMonsterDef`, `OfflineSkillDef`, `OfflineNPC`,
`OfflineShopDef` / `OfflineShopItemDef`, `OfflineSpawn`, `OfflineWarp`.

`DecodeOfflineContent` parses it, `NormalizeOfflineContent` canonicalizes it,
and `ContentFingerprint()` gives a stable hash used to detect pack changes.
`offline_content_runtime.go` is the runtime view built from the decoded pack.

Regenerate the pack deterministically with:

```bash
go run ./cmd/build-offline-content
```

The importer that reads upstream server data lives in
`internal/offlinecontent/importer.go`.

## Projection back into the world

`game/offline_projection.go` turns offline authority state into the same
`world.World` actors and floor items the network path produces, so drawing and
UI never branch on online/offline. `game/offline_profile.go` and
`session/offline_profile.go` cover the offline character profile.

Selection between modes is `[mobile] session mode` in config
(`config.SessionModeOnline` vs offline); `app.Game.Online()` reports which is
active.
