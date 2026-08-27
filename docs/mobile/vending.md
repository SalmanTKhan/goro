# Mobile vending browse and buy

A mobile surface for another player's vending shop: a bounded, renderer-neutral
projection of the existing online protocol and desktop board interaction.

## Presentation

`mobileui.MobileVendingModel` contains shop identity, the player's current
zeny, loading/notice state, and item rows. `MobileVendingController` owns
selection, deterministic scrolling, quantity state, and local screen close
behavior. Item rows carry the server vending index separately from the item ID;
the index is what the existing purchase packet requires.

## Layout

Landscape uses a centered, bounded panel with a scrollable item list and a
detail pane. The content rail is capped at 1280 logical pixels, including the
Fold outer `2268x832` target, so additional width remains world visibility and
breathing room rather than proportionally enlarged UI. Portrait stacks the
list and details. Rows and actions remain at least 48px.

## Commands and authority

- `TargetVending` is a world-picking result, not a gameplay rule.
- `CommandOpenVending` requests the existing vending list helper.
- `CommandBuyVendingItem` sends one selected vending entry and quantity through
  `network.Client.SendVendingPurchase`.

The server remains authoritative for distance, shop existence, stock, price,
weight, inventory, and zeny. The mobile layer does not mutate inventory or
duplicate vending rules. The desktop vending setup/cart/own-shop flow remains
unchanged.

## Preview

```text
go run ./cmd/mobile-ui-preview -screen vending -fixture vending-basic -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen vending -fixture vending-long -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen vending -fixture vending-disabled -viewport fold-outer
```

Fixtures include empty, long-list, long-name, and disabled-item states.

## Limitations

- Live shop qualification against a populated server/device is a separate
  matrix item.
- Own-shop setup, price entry, and vending cart/bulk checkout are separate
  slices.
- Item detail uses fields available in the vending list; it does not invent
  descriptions or stats absent from the existing resource projection.

