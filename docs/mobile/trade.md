# Mobile trade

A renderer-neutral player-to-player trade surface for the existing online
protocol. It covers incoming requests, item and zeny offers, conclusion,
commit, cancellation, and server undo.

## Presentation

`mobileui.MobileTradeModel` contains the partner name, pending request, a
read-only inventory rail, both offer rails, zeny totals, conclusion state, and
action availability. `MobileTradeController` owns only selection, quantity,
scroll, and navigation state. It emits semantic commands; it does not expose
packet structs or desktop widgets.

The layout uses a bounded 1800px content rail. Landscape uses inventory on the
left and the two offer panels on the right; portrait stacks the same surfaces.
All rows and actions are at least 48px, and modal geometry is centered inside
the safe rectangle. The Fold outer target remains 2268x832.

## Commands

- `CommandRespondTradeRequest`
- `CommandTradeAddItem`
- `CommandTradeAddZeny`
- `CommandTradeConclude`
- `CommandTradeCommit`
- `CommandTradeCancel`

`WorldMode` forwards these to the existing `network.Client` trade methods.
The command path rejects offline use, invalid inventory indexes, equipped
items, quantities outside the current inventory, duplicate pending offers,
premature commits, and zeny above the current session balance.

## Preview

```text
go run ./cmd/mobile-ui-preview -screen trade -fixture trade-long-names -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen trade -fixture trade-request -viewport fold-outer
```

Fixtures include `trade-basic`, `trade-empty`, `trade-pending`,
`trade-concluded`, `trade-request`, and `trade-long-names`.

## Navigation and ownership

An active exchange or incoming request claims the entire mobile safe area.
Back closes quantity first, declines a pending request, or emits the existing
trade-cancel intent. Trade touches cannot fall through to world movement,
attack, pickup, or skill targeting.

## Limitations

A mobile actor-context sheet for starting a trade is separate host work.
Desktop trade behavior is unchanged; vending is covered by
[mobile-vending.md](mobile-vending.md). The live exchange matrix against a
populated server/device remains a separate qualification item.
