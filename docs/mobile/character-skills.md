# Mobile character and skills

Read-only Character and Skills screens:

- Character: progression, vitals, zeny, weight, attributes, and combat values.
- Skills: skill points, skill rows, level/cost/range/target details, selection,
  and deterministic scrolling.

## Ownership

| Concern | Authoritative owner | Mobile presentation |
| --- | --- | --- |
| Character identity/job | `session.Session.SelectedCharacter()` | `ProjectCharacter` |
| Vitals/progression | `session.Session.Vitals` and `Progress` | `MobileCharacterModel` |
| Attributes/combat values | `session.Session.Stats` | `CharacterStatModel` and combat fields |
| Skill list/points | `session.Session.Skills` | `ProjectSkills` |
| Skill selection/scroll | UI-local state | `CharacterSkillsController` |
| Android drawing | `android/host/go/mobile_presentation.go` | consumes shared models/layouts |

The projection copies plain values and does not expose session structs, desktop
widgets, packets, renderer handles, JNI, or Android types. Existing desktop
character windows are unchanged.

## Interaction contract

Character and Skills screens consume all touches while open. Navigation is
presentation-local:

```text
World HUD -> Character -> Skills
World HUD -> Skills -> Character
Back     -> World HUD
```

Selecting a skill updates only `SkillSelectionModel`. It does not emit a
gameplay command. The repository has no existing authoritative mobile stat
allocation or skill-upgrade command, so the slice does not invent one.

The skills list uses a bounded viewport with clamped offset and 64px row
extent. Touch drag and future mouse/controller adapters can map to the same
`ScrollBy` method.

## Preview

```text
go run ./cmd/mobile-ui-preview -screen character -fixture character-rich -viewport fold-outer
go run ./cmd/mobile-ui-preview -screen skills -fixture skills-many -viewport fold-outer
```

Fixtures include an attribute-rich character, an empty/new character, a basic
skill list, and a long skill list. Layout tests cover 1920x1080, 2400x1080,
2560x1440, Fold outer 2268x832, and safe-area insets.

## Known limitations

- Stat allocation and skill upgrades are intentionally read-only until an
  existing gameplay authority path is available.
- Skill descriptions and live icon textures are not synthesized; only metadata
  already present in `session.Skill` is shown.
- Exact Fold outer physical touch validation remains a device gate.
