# Gameplay systems

Where each in-game system lives. Every entry follows the same shape: packets in
`network`, static tables in `db`, durable state in `session`, map state in
`world`, orchestration in `game`, windows in `ui`.

## Static tables (`db`)

Adapted from robr's DB directory; pure data plus lookup helpers.

`job.go` (job ids, names, baby/expanded variants), `skills.go`, `skill_info.go`,
`skill_tree.go`, `skill_units.go`, `status.go`, `status_icons.go`,
`item_tables.go`, `monster_tables.go`, `weapon.go`, `weapon_action.go`,
`effects.go`, `emotions.go`, `hit_sounds.go`, `mount.go`, `pet_actions.go`.

Import gaps are tracked in `docs/db-import-todo.md`.

## Combat

`game/battle.go` (damage application, attack timing), `damage_numbers.go`
(floating combat text), `game/pvp.go` and `ui/pvp_counter.go` (PvP flags and
ranking counter), `status_effects.go` (buffs/debuffs and their icons),
`game/sfx.go` (hit sounds from `db`).

Map-level combat permission comes from `world.MapProperty`.

## Skills

`game/skill_use.go` (cast, cast bar, cast cancellation, walk cancellation),
`skill_cursor_level.go` (level selection at the cursor), `skill_text.go` and
`skill_name_bubbles.go`, `skill_unit_models.go` (ground units and cast
markers), `teleport_handlers.go` (teleport/warp modals). Skill trees and levels
come from `db/skill_tree.go` and `res` skill metadata. UI: `ui/skill_window.go`,
`ui/teleport_modal.go`. Design notes: `docs/first-class-skills-v1.md`.

## Effects

`game/effects.go` dispatches; `effects_2d.go`, `effects_3d.go`,
`effects_sprite.go`, `effects_cylinder.go`, `effects_quadhorn.go`,
`effects_func.go`, and `effects_str.go` (STR animations from `res/str.go`)
implement the families. `less_effects.go` implements the reduced-effects option;
`level99_aura.go` the aura.

## Items and economy

`game/items.go`, `inventory_state.go`, `inventory_defs.go` (inventory and floor
drops), `trade.go`, `vending.go`, `vending_board.go`, `cart.go`. Crafting,
repair, refinement, identification, and card composition are driven from `game`
into the matching `ui` windows (`making_item_window.go`,
`making_arrow_window.go`, `card_composition_window.go`, `identify_window.go`).

## Character presentation

`game/sprite_assets.go`, `sprite_render.go`, `equipment_view.go` (item-specific
weapon sprites, wedding sprites), `actor_mount.go` (mounts),
`actor_effect_state.go`, `actor_state.go`, `emotions.go`, `speech_bubbles.go`,
`song_talk.go`. Walk/attack sprite work is tracked in
`docs/walk-combat-sprite-todo.md`.

## Social

`game/friends.go`, `party.go`, `guild.go`, `guild_emblem.go`, `chat_room.go`,
`whisper_window.go`, `console_messages.go`, plus the `ui` windows and context
menus of the same names.

## Pets and companions

`game/pet.go` and `pet_slot_machine.go` (capture, hatching, feeding, rename,
accessories, performances, familiarity-gated talk), `homunculus.go`,
`mercenary.go`, `companion.go`, `companion_ai.go` (default and custom `USER_AI`
behavior), `falcon.go`. Config `--force-user-ai` starts companions in custom AI
mode. Reference: `docs/companions-20080910.md`.

## Adoption

`game/adoption.go` with `network/adoption_packets.go` — baby-job conversion,
eligibility rules, and baby sprite scaling.

## NPCs

`game/npc_pick.go` (picking and talk requests), `special_npc.go`,
`ui/npc_dialog.go`. Granny 3D NPC models come through `res/gr2.go` and
`game/gr2_model.go` / `gr2_animation.go`.

## Map presentation

`game/map_weather*.go` (weather, Yuno clouds, pokjuk fireworks), `fog.go` (RSW
near/far fog matching the reference client), `map_fade.go`, `map_cell_update.go`,
`rsw_effects.go`. Outstanding work: `docs/map-effects-weather-todo.md`.

## Hotkeys and shortcuts

`game/hotkey_state.go` with `network/hotkey_packets.go` and
`ui/shortcut_bar.go` — the multi-row shortcut bar with classic key bindings.
`no_shift` / `no_ctrl` config options change modifier semantics.
