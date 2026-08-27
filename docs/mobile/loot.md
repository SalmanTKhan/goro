# Mobile loot and pickup

The mobile combat-to-inventory loop: an offline drop is projected into a
nearby-loot rail, shown on the minimap, and exposed through a finger-sized
pickup action. The action emits the existing `CommandPickUpItem` command and the
offline authority remains responsible for range, weight, item identity, and
inventory mutation.

## Projection

`app.Game.MobileHUDModel` projects the current offline drops into
`mobileui.LootItemModel`. The model contains only the drop runtime identity,
stable item metadata, world position, quantity, and a deterministic Chebyshev
distance from the player. Drops are sorted by distance and then drop ID so the
HUD does not depend on map iteration order. Item markers use the same
renderer-neutral minimap projection as NPCs and monsters.

## Interaction

The HUD displays up to the rows that fit between the player panel and the
target panel. Each row is at least the shared 48px minimum touch target. A
nearby row emits `CommandPickUpItem` immediately. A farther row uses the same
command but the `WorldMode` pickup path first walks to the drop, then retries
the command when the player is in range. This keeps the mobile path aligned
with the existing desktop pickup approach behavior without moving pickup rules
into the UI.

Every loot row is consumed by the mobile HUD hit tester, so it cannot fall
through to movement or attack input. The world sprite, item label, inventory
projection, and pickup animation remain owned by the existing renderer/game
projection layers.

## Preview and tests

```text
go run ./cmd/mobile-ui-preview -screen loot -fixture loot-basic -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen loot -fixture loot-long-names -width 2400 -height 1080
```

The headless tests cover row ownership, semantic pickup command emission,
safe-area geometry, bounded rows, source-slice isolation, and offline
out-of-range auto-approach. The Fold outer 2268x832 target remains in the
layout matrix.

## Known limitations

- There is no separate loot window or drag-to-pick interaction; the nearby rail
  is intentionally the smallest useful mobile surface.
- Inventory-full and no-drop errors remain authority outcomes and are not
  duplicated as mobile validation rules.
- Exact ARM64 physical validation remains a device gate.
