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

Desktop controller input is provided by `input/gamepad` on Windows, Linux, and
macOS through SDL3's standardized Gamepad API. Android builds use the no-op
backend and do not import SDL. `input/controller.go` resolves physical W/A/S/D
and DualSense-compatible positional controls into the same action state: a
0.15 radial inner deadzone, 0.95 outer deadzone, normalized axes, and
deterministic eight-way movement. Controller movement wins when both devices
are providing movement; keyboard or mouse activity returns the UI to desktop
input mode.

Two persisted, orthogonal modes govern behaviour: `MoveMode` (character walking
versus a stick-driven pointer) and `UINavMode` (pointer versus focus traversal in
menus). `input/virtual_cursor.go` holds the pointer itself — sub-pixel position,
a direction-preserving quadratic response curve, and a `Move` that reports only
genuine integer-pixel changes so no redundant event is injected. The renderer
injects synthetic pointer events at `render.fanoutEventSource`, the single fork
that feeds both `input.State` (world picking) and the gogpu widget tree (windows,
drag, sliders, menus), so one emit reaches everything a real mouse would.
`input.State.SetPointerSource` keeps those injected events attributed to the
controller rather than the mouse. Camera rotation is deliberately *not*
synthesized as a right-button drag: it stays `CommandRotateCamera`, because a
held synthetic button plus injected motion would also feed `MouseDX/MouseDY` and
rotate the camera twice per frame.

The production world consumer preserves the existing walk horizon of 8 cells,
refills at 3 cells, cooldowns, and server-approved stop behavior. A configured
Lua input profile remains authoritative for that frame, preventing duplicate
walk requests. Lua profiles can inspect `goro.actions()` and
`goro.controller()` for normalized state.

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

Controller-facing world intents are appended to the command vocabulary as
`CommandMoveDirection`, target cycling, focused attack/interact/loot, and
shortcut activation. `game.WorldMode` continues to apply them through the
existing authoritative movement, combat, item, and camera systems.

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
