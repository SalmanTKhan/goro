# Mobile NPC dialog, shop, and storage

The platform-independent NPC interaction surface. It is offline-first and keeps
the existing session authority responsible for all state changes.

## Ownership

| Concern | Current owner | Mobile projection / adapter |
| --- | --- | --- |
| NPC actor and talk request | `game/WorldMode` and `game/player_commands.go` | `input.CommandInteractActor` from the mobile host |
| Dialog presentation | `mobileui.MobileDialogModel` | `mobileui.DialogController` |
| Shop definitions | `session.OfflineRuntime` / `session.OfflineShop` | `mobileui.ProjectShopWithMetadata` |
| Inventory and zeny | `session.Session.Inventory` | read-only `MobileShopModel` and inventory models |
| Buy/sell mutation | `session/authority.go` through `Game.ApplyPlayerCommand` | `CommandBuyItem` / `CommandSellItem` |
| Storage state and mutation | `session.Session.Storage` and offline authority | `MobileStorageModel` / `CommandWithdrawItem` |
| Android drawing | `android/host/go/mobile_presentation.go` | draws controller layouts; owns no shop rules |

The desktop windows are not reused or resized. The mobile models contain plain
values and stable item/resource identifiers only; packet structs, renderer
handles, JNI, and Android types do not cross into `mobileui`.

## Dialog

`ProjectDialog` creates a deterministic NPC title, message, and enabled option
list. Close is UI-local. Open Shop emits `input.CommandOpenShop` with the NPC
identity. `DialogController` owns safe-area hit testing and consumes every touch
inside the dialog surface before world input is considered.

## Shop and storage

`MobileShopModel` exposes buy rows from the existing offline shop definition and
sell rows projected from the current inventory. The mobile layer does not
calculate or enforce gameplay rules: the offline authority continues to validate
stock, zeny, item quantity, and mutations. The current offline fixture uses the
repository's existing sell value of five zeny per item.

`MobileEconomyController` supports bounded Buy and Sell tabs, semantic buy,
sell, and withdraw commands, deterministic safe-area layouts with 52px tabs and
56px rows, and close/back behavior with complete screen-local touch ownership.

Storage remains a separate screen model. Depositing from inventory uses the
existing `CommandDepositItem` path; opening storage uses `CommandOpenStorage`.

## Quantity and scrolling

Selecting a shop or storage row opens `EconomyQuantityState`. It carries the
semantic operation, NPC identity when needed, item index, minimum, maximum, and
current value. Increment, decrement, confirm, and cancel are UI-local. Confirm
emits the existing `CommandBuyItem`, `CommandSellItem`, or `CommandWithdrawItem`;
the authority remains responsible for validation and mutation.

Shop and storage lists use a bounded `InventoryScrollState` with a 64px row
extent, a measured list viewport, clamped offset, and explicit row indices. The
row-index mapping prevents a scrolled visual row from targeting the wrong item.
Touch drag maps to `ScrollBy`, and the entire open economy surface consumes
world touches.

## Preview fixtures

```text
go run ./cmd/mobile-ui-preview -screen dialog -fixture dialog-long -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen shop -fixture shop-sell -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen shop -fixture shop-long -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen storage -fixture storage-long -viewport fold-outer
```

Fixtures are deterministic and do not require game assets or a renderer.
`mobileui` tests cover the Fold outer viewport, safe-area insets, minimum touch
targets, dialog actions, buy/sell/withdraw commands, bounded economy width/list
height/visible rows, quantity-modal bounds, and the absence of world
fallthrough while a screen is open.

## Limitations

- Quantity entry is intentionally button-based; Android keyboard/text entry is
  not part of this platform-independent slice.
- Price/stock updates after a command are authority-driven and refresh on the
  next projection pass.
