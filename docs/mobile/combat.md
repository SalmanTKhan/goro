# Mobile combat and skill targeting

The existing offline combat authority is exposed through a small,
renderer-independent mobile interaction boundary. It does not add combat rules,
packet fields, or a second simulation.

## Ownership

| Concern | Authoritative owner | Mobile presentation / adapter |
| --- | --- | --- |
| Actor picking and attack intent | `game/WorldMode` and `game/player_commands.go` | `input.PlayerCommand` from the mobile input adapter |
| Attack and skill rules | `session.OfflineRuntime` / existing network helpers | no duplicated rules |
| Target identity and HP | `session.OfflineSession` plus world projection | `TargetHUDModel` through `app.Game.MobileHUDModel` |
| Skill targeting mode | `session.Skill.Type` and `input.SkillTargetState` | HUD `SkillSlotModel` and `Navigation.Targeting` |
| Cooldown/SP availability | offline authority and `session.Session` | read-only `SkillSlotModel.Usable` / cooldown metadata |
| Target cancellation | mobile navigation state | `CombatCancel` hit target and `CommandCancelAction` |
| Drops and pickup | `session.OfflineRuntime` and `game.WorldMode` | existing world picker and semantic pickup command |
| Android drawing | `android/host/go/mobile_presentation.go` | draws the prompt and cancel affordance only |

## Interaction contract

```text
tap skill
  ├── self skill      -> UseSkill
  ├── actor skill     -> pending actor target
  └── ground skill    -> pending ground target

pending target
  ├── tap valid world target -> UseSkillOnActor / UseSkillAtPosition
  ├── tap CANCEL              -> CancelAction
  └── Back                    -> CancelAction
```

While a target is pending, world touches remain UI-owned by the mobile
presentation boundary. The explicit cancel target is laid out inside the safe
area and has the same minimum 48px touch contract as other controls.

The mobile layer does not decide range, damage, cooldown, SP cost, target
validity, drops, or pickup distance. Those checks remain in existing gameplay
consumers and `session.OfflineRuntime`.

## Offline combat presentation

Offline damage and monster-death events are routed through the existing
`network.ActorActionNotify` presentation path inside
`game.syncOfflineProjection`, so the offline authority reuses the production
renderer/gameplay presentation for:

- normal attack action frames;
- skill action frames and skill-name bubbles;
- hit reactions;
- damage floaters;
- monster death animations and fade/removal timing.

Offline mobile packs include the authored combat feedback resources
(`숫자.act/.spr` and `msg.act/.spr`). World-space damage feedback uses a shared
1.5x display multiplier, with a scaled, outlined bitmap-text fallback when a
source pack does not provide those sprites.

The offline authority remains responsible for range, cooldown, SP, damage,
death, EXP, drops, and respawn; no combat values or network packet behavior are
duplicated by the bridge.

## Preview fixtures

```text
go run ./cmd/mobile-ui-preview -screen combat -fixture combat-targeting -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen combat -fixture combat-ground -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen combat -fixture combat-cooldown -viewport fold-outer
```

The JSON preview includes the projected `MobileCombatModel`, HUD layout, and
target/skill fixture state. `game/offline_projection_test.go` verifies that
normal damage starts the local attack animation, creates a damage floater, and
preserves a dead monster until its death animation can render, and that the
offline Bash event starts a skill action and publishes the skill-name bubble.
`mobileui` tests cover actor and ground targeting, cancel ownership,
cancellation command emission, safe-area placement, and touch-target minimums
across the representative viewport matrix.

## Limitations

- Combat feedback is intentionally compact; a full combat log/history is a
  renderer/gameplay concern for a later measured pass.
- Physical touch validation of attack, skill targeting, and pickup on the
  Fold outer `2268x832` target remains a device gate.
