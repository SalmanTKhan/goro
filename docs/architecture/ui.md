# UI (`ui`)

`ui` owns window/modal/menu composition and reusable RO skinning. It may read
`client`, `session`, and `network` data, but it must not draw the map, render
sprites, or simulate gameplay.

The UI is built on `gogpu/ui` widgets. `render` exposes the widget app through
`client.UIApp`; `ui.Manager` implements `client.UIManager`.

## Manager

`ui.NewManager()` keeps an ordered overlay stack.

- `AddOverlay(widget)` / `RemoveOverlay(widget)` / `Clear()`
- `PointerOverUI(x, y)` — the single predicate for "this screen point belongs to
  the UI, not the world", so world clicks are not stolen. It combines
  `PointerBlocked` (a UI element covers the point) with `TextInputActive`. Both
  the real mouse path and the controller's virtual pointer resolve through it,
  via `client.UIPointer`, so they cannot drift apart. `game` checks it before
  world picking.
- `SetPointerTransform(fn)` — installs the physical-to-logical pointer
  conversion. Callers pass physical window coordinates while widget bounds are
  logical, and the two diverge whenever the UI scale is not 1; the renderer
  hands the manager the same transform it applies to real pointer events.
- `SetTextInputPredicate(fn)` / `SetControllerRebindPredicate(fn)` — injected
  sources of world state that lives outside the widget tree (the chat console,
  an open rebinding capture), keeping `ui` free of a dependency on `game`.
- `raiseOverlay` implements focus-to-front on press, which is what gives the
  classic overlapping-window behavior with stable dragging.
- `overlayRoot` is the synthetic root widget: it lays out, draws, and dispatches
  positioned events to overlays in stack order and reports `IsUIRootEmpty` so
  the backend can skip publishing an empty UI.

## Theme (`ui/rotheme`)

Reusable RO-styled primitives, independent of any particular window: `button`,
`icon_button`, `checkbox`, `radio`, `slider`, `dropdown`, `listview`,
`textfield`, `text`, `rotated_text`, `gradient`, `context_menu`, `table` /
`table_cell` / `table_view`, and `theme.go`.

Shared pieces at package level: `window.go` (draggable RO window frame),
`ro_titlebar.go`, `scrollbar.go`, `tab_widget.go`, `tooltip.go`,
`confirm_modal.go`, `context_menu.go`, `sprite_layer.go`, `surface.go`,
`selection_keyboard.go`, `util.go`.

## Windows

One file per window, each with a sibling `_test.go`. Grouped roughly:

- Login: `login_window.go`, `character_select_window.go`,
  `character_create_window.go`
- Character: `character_window.go`, `stats_window.go`, `equipment_window.go`,
  `equipment_choice_window.go`, `skill_window.go`, `status_icons.go`
- Items: `inventory_bag_window.go`, `inventory_actions.go`,
  `inventory_common.go`, `item_info_window.go`, `item_table_view.go`,
  `identify_window.go`, `identified_item_icon.go`,
  `item_pickup_notification.go`
- Storage and cart: `storage_window.go`, `storage_categories.go`,
  `cart_window.go`, `change_cart_window.go`
- Economy: `shop_window.go`, `shop_amount_prompt.go`, `trade_window.go`,
  `vending_window.go`, `card_composition_window.go`, `making_item_window.go`,
  `making_arrow_window.go`
- Social: `friends_window.go`, `friend_context_menu.go`,
  `friend_settings_window.go`, `party_*.go`, `guild_window.go`,
  `chat_room_window.go`, `chat_room_create_window.go`, `whisper_window.go`,
  `player_context_menu.go`
- Companions/pets: `homunculus_*.go`, `mercenary_*.go`, `pet_*.go`
- HUD/system: `basic_menu.go`, `shortcut_bar.go`, `minimap.go`, `console.go`,
  `emote_window.go`, `escape_menu.go`, `settings_window.go`,
  `teleport_modal.go`, `npc_dialog.go`, `text_prompt_window.go`,
  `pvp_counter.go`, `view_equipment_window.go`

## Conventions

- A window exposes state plus callbacks; `game` supplies the callbacks
  (`ui_actions.go`) and owns the consequences. Do not call `network` from `ui`
  window code.
- Text wrapping for chat lives in `chat_text_wrap.go` — reuse it rather than
  writing new wrapping.
- Every new window gets a test that at minimum builds it and exercises layout
  and its callbacks.

Migration notes and remaining gaps: `docs/ui-migration-todo.md`,
`docs/gogpu-ui-settings-example.md`.
