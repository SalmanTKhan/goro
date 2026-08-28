# Mobile and Android

Detailed per-feature mobile documentation already lives in `docs/mobile/`
(`architecture.md`, `startup.md`, `input.md`, `ui.md`, `map.md`, `combat.md`,
`inventory.md`, `loot.md`, `character-skills.md`, `npc-shop.md`, `trade.md`,
`vending.md`, `social.md`, `chat-settings.md`, `profile.md`, `assets.md`,
`online.md`, `renderer-core-extraction.md`). This page is the orientation map.

## `mobileui`

Backend-neutral mobile presentation: models, layout, controllers, state. It does
no rendering and no transport — it turns session/world data into read-only view
models and turns touch interaction into `input.PlayerCommand` values.

Typical triple per feature: `<feature>_model.go` (view model),
`<feature>_layout.go` (touch-target geometry), `<feature>_controller.go`
(interaction → commands), plus `<feature>_fixtures.go` for deterministic tests.

Covered: HUD (`model.go`: `PlayerHUDModel`, `TargetHUDModel`, `SkillSlotModel`,
`StatusEffectModel`, `LootItemModel`), `map`, `combat`, `inventory`, `economy`
(shop/storage), `trade`, `vending`, `social`, `chat`, `dialog`,
`character_skills`, `grid`, `interaction`, `layout`, `layout_options`.

`app.Game` exposes the projections (`MobileHUDModel`, `MobileMapModel`,
`MobileInventoryModel`, `MobileSkillsModel`, `MobileShopModel`,
`MobileStorageModel`, `MobileSocialModel`, `MobileTradeModel`,
`MobileVendingModel`, `MobileProfileModel`, `MobileChatModel`,
`MobileCharacterModel`) and the icon/preview draw helpers. Interaction returns
through `app.Game.ApplyPlayerCommand`.

Preview the layouts on the desktop with:

```bash
go run ./cmd/mobile-ui-preview
```

## Mobile input

`input.MobileControls` and `input.MobileDisplaySettings` (config sections
`[mobile]`, `[mobile_display]`) define the on-screen control layout and display
tuning; `input/gestures.go` interprets touches. `CommandSetMobileControls` /
`CommandResetMobileControls` / `CommandSetMobileSettings` /
`CommandResetMobileSettings` persist changes through `config.UserSettings`.

## Asset packs

Mobile builds ship modular deterministic packs instead of the full client data:

| Command | Purpose |
|---|---|
| `cmd/build-mobile-assets` | Build modular packs from a versioned mobile asset project |
| `cmd/build-mobile-pack` | Build one deterministic pack plus a manifest with closure and compression metrics |
| `cmd/mobile-pack-editor` | Local browser UI for composing packs from a client data directory |
| `cmd/benchmark-mobile-assets` | Compare generated PAK and GRF artifacts |
| `cmd/merge-mobile-metrics` | Attach measured Android runtime metrics to a manifest |

Implementation: `internal/mobilepack` (`project.go`, `builder.go`,
`delivery.go`, `inventory.go`, `terrain.go`, `textures.go`,
`runtime_metrics.go`).

At runtime, packs mount as `res.AssetOverlay`s
(`app.MountAssetOverlay` / `ReplaceAssetOverlays`). Gating is through
`client.AssetAvailability`: `RequireMap(mapName)` returns an
`AssetRequirement`, `RequestPack(name)` triggers a download,
`Subscribe(func(AssetEvent))` reports `PackState` transitions. Game modes never
see the Android downloader — a completed overlay simply changes the resource
view and the requirement becomes ready on a later update.

## Android host

`android/host/` is a Gradle project with a Go module under `android/host/go`
(`main.go`, `mobile_presentation.go`, `resource_probe.go`, `goro_android.h`,
plus emulator-specific module files and a goffi overlay). See
`android/host/README.md`. Android work is in progress — treat it as the least
settled part of the tree.
