# Mobile inventory and equipment

Inventory and equipment presentation in `mobileui`, without changing desktop
windows or Android rendering.

## Projection

`ProjectInventory` copies `session.Inventory` rows into
`InventoryItemModel`. It preserves item identity/index, quantity, type,
equipped state, refinement, cards, equipment location, and only metadata
available through the small `ItemMetadataSource` interface. `ProjectEquipment`
maps the existing equipment masks from `db` into the supported head, weapon,
shield, armor, garment, shoes, accessory, and ammo slots.

## Categories and selection

Categories follow the existing `db` item types: All, Equipment, Usable, Etc,
and Cards. Filtering is presentation state. Tapping a visible item updates
selection and an item detail model; it never emits a gameplay command.

## Layout and scrolling

`LayoutInventory` and `LayoutEquipment` use `Viewport.SafeRect` and mobile
tokens rather than desktop coordinates. The inventory has tabs, a grid
viewport, detail surface, actions, and an equipment navigation button.
At 2268x832 the inventory and equipment surfaces use a centered bounded
content rail rather than scaling panels across the ultrawide display; extra
width remains breathing room/world visibility.
`InventoryScrollState` stores viewport extent, content extent, row extent,
offset, clamping, and page calculations. It is independent of mouse-wheel or
touch-drag semantics.

## Commands and quantity state

Use, equip, unequip, and drop emit `input.PlayerCommand` values carrying the
inventory index and, for drop, quantity. `game.WorldMode.ApplyPlayerCommand`
validates use eligibility and calls the existing network methods; mobile UI
does not mutate session state or implement item rules. `QuantityState` handles
min/max, increment/decrement, explicit values, confirm, and cancel.

## Navigation and ownership

Inventory opens equipment and returns to inventory; back closes quantity first,
then detail, then the screen. If a shared skill targeting state is active,
back emits `CommandCancelAction` first. Every touch inside an open inventory
or equipment safe area is UI-consumed, including blank areas, tabs, grid,
details, scrolling, modal, and buttons. World touches are not consumed after
returning to `ScreenWorldHUD`.

## Fixture preview

```text
go run ./cmd/mobile-ui-preview -screen inventory -fixture inventory-full -width 2400 -height 1080
go run ./cmd/mobile-ui-preview -screen equipment -fixture equipment-full -width 2400 -height 1080
go run ./cmd/mobile-ui-preview -screen inventory -fixture inventory-long-names -viewport fold-outer
```

Fixtures cover basic/full inventories, long names, large quantities, empty
states, and partial/full equipment. The output is deterministic JSON layout
data, not Android or renderer evidence.
