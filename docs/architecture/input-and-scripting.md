# Input and scripting

## `input` package

Backend-neutral input. It knows nothing about RO gameplay, rendering, or
packets — the backend fills it, `game` interprets it.

`input.State` accumulates a frame of input and is advanced with `Update()` /
`EndFrame()`. Setters are called by the backend: `SetKey`, `SetKeyCode`,
`SetMouseButton`, `SetMousePosition`, `AddWheel`, `AddTextInput`, `SetTouch`.
Queries: `Pressed`, `JustPressed`, `KeyCodeDown`, `KeyCodeJustPressed`,
`KeyCodeJustReleased`, `ConsumeKeyCodePress`, `MousePressed`,
`MouseJustPressed`, `MouseJustReleased`, `TextInput`.

Key codes are physical (`gpucontext.Key`, e.g. `KeyW`, `ShiftLeft`), layout
independent — WASD positions are ZQSD on AZERTY. `keycode.go` maps names.
`ConsumeKeyCodePress` is the way to claim an edge without disturbing held state.

Files: `input.go` (state), `keycode.go`, `adapters.go` (backend adapters),
`gestures.go` (touch gestures), `mobile_controls.go` / `mobile_settings.go`
(on-screen control layout and persisted mobile settings), `commands.go`.

## PlayerCommand

`commands.go` defines the presentation-independent command vocabulary that
crosses from any front end (desktop input, mobile HUD, script) to the session
authority: `CommandKind` covers movement and targeting (`CommandMoveTo`,
`CommandAttackActor`, `CommandUseSkill*`), inventory and equipment, shops,
storage, trade, vending, chat and whisper, party/friends/guild, NPC dialog
stepping, camera, window toggles, mobile settings, and online session control.

Supporting types: `WorldPosition` (world-space target — screen picking stays in
the input adapter and never crosses this boundary), `PickedTarget` /
`TargetKind`, `WorldPicker`, `UIHitTester`, `CommandSink` / `CommandBuffer`
(`Emit`, `Commands`, `Reset`), and `SkillTargetState` (`BeginActor`,
`BeginGround`, `Cancel`, `Select`) for the two-step skill targeting flow.

Commands are consumed by `session.GameSession.HandleCommand` and by the mobile
command handlers in `game/mobile_*.go`.

## Lua scripting

Optional in-game character scripting: `--script <path>`. The script defines a
global `tick()` (called about every 150 ms while world mode is active) and
optionally `input()` (called once per frame for keyboard edges).

The API lives in the global `goro` table: `goro.keyboard` (`available`,
`is_down`, `was_pressed`, `was_released`, `consume_press`, `text`),
`goro.player()`, `goro.hp()`, `goro.sp()`, plus enemy, nearby-player, companion,
floor-item, and inventory queries and actions for walking, stopping, attacking,
looting, item use, chat, and targeted skills. The API only reports and acts;
policy is Lua's.

Implementation: `game/bot.go`, `bot_keyboard.go`, `bot_movement.go`,
`bot_targeting.go`. Examples: `scripts/loot-and-attack.lua`, `scripts/wasd.lua`.
Full reference: `docs/bot-scripting.md`.
