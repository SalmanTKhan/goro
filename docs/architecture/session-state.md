# Session state (`session`)

`session.Session` is the durable per-player state shared across login,
character selection, map transitions, and every UI window. It holds values, not
behavior, and knows nothing about transport.

## Contents

- Identity/auth: `AccountID`, `CharID`, `AuthCode`, `UserLevel`, `Sex`,
  `AdminList`, `Playing`, `Dead`.
- Server lists: `CharServers`, `Characters`, `Selected`, `Zone`.
- Server clock: `ServerTick` + `ServerTickAt` (used to convert server ticks to
  local time for movement and cooldowns).
- Player: `PlayerX/Y/Dir`, `Vitals`, `Progress`, `Stats`, `Skills`, `Hotkeys`,
  `Statuses`, `Movement`, `AttackRange`.
- Contents: `Inventory`, `Storage`, `Cart`.
- Social: `Friends`, `PendingFriendRequest`, `Whisper`, `Party`,
  `PendingPartyInvite`, `GuildID`/`Guild`/`GuildName`/`EmblemVersion`.
- Companions: `Homunculus`, `Mercenary` (both `Companion`).
- Client options mirrored from config: `NoShift`, `NoCtrl`, `LessEffects`,
  `SnapTargets`, `SnapItems`, `ShowEquip`, `HomunculusCustomAI`,
  `HomunculusAggressive`, `MercenaryCustomAI`, `MercenaryAggressive`.

`SelectCharacter(character)` is the reset boundary: it seeds vitals, progress,
stats, and inventory from the character list entry and clears everything
map- or guild-scoped. Add new per-character fields there or they will leak
across character switches.

## The authority boundary

```go
type GameSession interface {
	State() *Session
	HandleCommand(input.PlayerCommand) bool
	Update(time.Duration)
}
```

`authority.go` defines this contract so presentation and input do not care
whether the authority is a server or the local offline simulation. `offline.go`
implements it for offline play; the online path can adopt the same interface.

## Files

| File | Role |
|---|---|
| `session.go` | The `Session` struct and its sub-structs |
| `authority.go` | `GameSession`, offline monster state machine, combat authority types |
| `offline.go`, `offline_profile.go` | Offline session lifecycle, pause/resume, save/load |
| `offline_content.go`, `offline_content_runtime.go` | Local content pack model and runtime view |
| `starter_loadout.go` | Starting equipment/inventory for offline characters |

See [offline-mode.md](offline-mode.md) for how these fit together.
