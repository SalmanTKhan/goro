# Mobile offline profile and character creation

A local profile surface for the offline mobile client. The profile owns the
character identity and appearance needed to start an offline adventure: name,
sex, hair style, hair color, and the six novice starter attributes.

The mobile screen projects `session.OfflineProfile` into
`mobileui.MobileProfileModel`. The controller edits a draft only and emits
`input.CommandSaveOfflineProfile`; `game.WorldMode` validates and applies the
command. The offline save file remains the persistence authority. Online
character creation continues to use the existing login-mode protocol path and
is not changed by this slice.

The editor supports a touch-only name keyboard, sex switching, hair style and
color cycling, and paired starter-stat changes matching the existing online
novice creation semantics. Saving an existing profile updates appearance and
name without resetting progression. Saving a new character resets the local
starter state, inventory, novice skill, and current offline map.

The preview uses the production humanoid resource lookup, but the
renderer-neutral model contains only stable identity and appearance values.
No Android, JNI, WGPU, packet, or desktop widget types cross into `mobileui`.

Known limitations:

- The current slice stores one active offline profile rather than an online
  account character list.
- Native Android IME integration is not required; the in-surface keyboard is
  used for the current host.
- Profile deletion, account sync, and online character creation UI remain in
  the online-session bootstrap slice.
