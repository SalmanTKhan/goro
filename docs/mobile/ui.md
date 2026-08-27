# Mobile HUD architecture

The mobile HUD is implemented in the top-level `mobileui` package. It is
independent of Android, JNI, renderer handles, network calls, and the desktop
`ui` package.

`mobileui.HUDSource` and `mobileui.Project` copy authoritative values into plain
`MobileHUDModel` data. `ProjectSession` is a convenience adapter for
player/session fields; target selection, cooldowns, and world picking remain
owned by their existing systems. Models contain no widgets or mutable session
references.

`LayoutHUD` takes a logical `Viewport` with explicit safe-area insets and
returns deterministic rectangles for the player panel, target panel, status
area, minimap, menu, and skill slots. Skill and menu controls meet the 48-pixel
minimum touch target in the default token set. The Samsung Galaxy Z Fold 3 outer
landscape viewport is a first-class `FoldOuterViewport()` target at 2268x832;
HUD controls stay compact and edge anchored there.

`Controller` implements the UI ownership boundary consumed by the input layer.
It consumes touches in HUD hit regions, emits semantic `input.PlayerCommand`
values for self-targeted skills and menu navigation, enters
`input.SkillTargetState` for actor/ground skills, and gives back/cancel priority
to active targeting.

Every screen model follows the same rule: `Project*` functions copy
authoritative values into plain models, a controller owns only screen-local
selection/scroll/navigation state, and mutations are emitted as existing
semantic `input.PlayerCommand` values (`CommandOpenShop`, `CommandBuyItem`,
`CommandSellItem`, `CommandOpenStorage`, `CommandWithdrawItem`,
`CommandSendGlobalChat`, `CommandBuyVendingItem`, and so on) consumed by the
existing game/session authority. Packet structs and desktop widgets never cross
into `mobileui`. The per-screen contracts are in the sibling docs linked from
[README.md](README.md).

## Headless preview

The preview harness lays out deterministic fixtures without loading assets or
starting a renderer:

```text
go run ./cmd/mobile-ui-preview -fixture monster -width 2400 -height 1080 -safe-top 48
go run ./cmd/mobile-ui-preview -screen inventory -viewport fold-outer
```

`-viewport fold-outer` forces the primary physical target dimensions. The JSON
output is suitable for later renderer integration; it is not Android rendering
evidence.

## Android connection

`android/host/go/mobile_presentation.go` is the renderer-facing adapter. It uses
the platform-independent layouts and controllers, converts Android touch events
into `input.State`, and draws the resulting presentation with the existing
`render.Frame` commands. The host receives system-bar insets through
`WindowInsets` and updates the logical viewport before laying out controls.

The world renderer remains responsible for terrain, actors, camera, and map
resources. The mobile presentation draws only HUD, inventory/equipment, dialog,
and modal surfaces on top of the world. Screen-local touches are consumed before
world movement or attack commands are considered.

## Mobile visual contract

The Android presentation keeps the desktop `ui/rotheme` language rather than
switching to a separate dark skin: light panel bodies, blue title bars, blue
borders, dark text, and the same restrained status colors. Mobile-specific
changes are scale and composition, not a new visual identity:

- status and target cards reserve enough vertical space for readable HP/SP
  values and level text;
- skill slots are 76px controls with a visible key label, skill marker, level,
  and cooldown state;
- the HUD reserves a safe-area-aware chat strip and keeps the minimap/menu
  separated from the status card;
- dialog, inventory, equipment, shop, and storage surfaces use the same light
  window treatment and 48px-or-larger touch controls;
- mobile labels use the renderer's scaled bitmap path because the Android host
  submits `render.Frame` commands directly and does not use the desktop UI
  overlay rasterizer.

The Fold outer target remains 2268x832. The deterministic snapshot in
`mobileui/testdata/fold_outer_2268x832.json` is the geometry gate; the Android
APK/device screenshot is the readability gate.
