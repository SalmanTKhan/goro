# Mobile Friends and Party

The desktop Friends window's Friends and Party tabs projected into a bounded,
safe-area-aware mobile screen.

## Presentation

`mobileui.ProjectSocial` copies friend names/IDs/state and party member
names/IDs/map/role/HP into `MobileSocialModel`. Selection, tab state, scroll
offset, and whisper target preparation belong to
`MobileSocialController`; no packet struct or desktop widget crosses the
boundary.

The landscape layout is bounded to 1600 logical pixels. The list and detail
panes share the available safe area, so extra Fold outer-display width adds
breathing room instead of stretching social controls. Rows and actions retain
at least 48 logical pixels where the viewport permits them. Portrait uses a
stacked list/detail arrangement.

## Interaction and authority

Friend and party rows are UI-owned. Tapping a row selects it; tapping Back
first clears selection, then closes the screen. Scrolling changes only the
deterministic mobile scroll state and is clamped to the list extent. Party
creation and invitations open a native text prompt; the Go controller keeps
the draft and emits a command only after the user confirms.

The following commands are semantic and accepted only when the online network
client is present:

- `CommandDeleteFriend(TargetAccountID, TargetCharID)`
- `CommandCreateParty(Text)`
- `CommandInviteParty(Text)`
- `CommandLeaveParty`
- `CommandExpelPartyMember(ActorID, Text)`
- `CommandRespondFriendRequest(TargetAccountID, TargetCharID, Accepted)`
- `CommandRespondPartyInvite(RequestID, Accepted)`
- `CommandSetPartySettings(ExpShare, RefuseInvites)`
- `CommandSendWhisper(TargetName, Text)`

`WorldMode.ApplyPlayerCommand` forwards these to the existing network helpers.
Offline mode still projects deterministic empty social state with an explicit
notice, but cannot fabricate friend/party mutations. Whisper opens the
existing chat controller with the selected recipient and uses the same native
text editor as world chat.

Incoming friend requests and party invitations are projected from transient
session state. They take precedence over social list interaction and expose
large Accept/Decline targets. Party leaders also get a bounded settings sheet
with the same two EXP-share choices and invite-refusal toggle as the desktop
party settings window. The command consumer sends both existing party setting
packets and updates the local projection only after both sends succeed.

## Preview and tests

```powershell
go run ./cmd/mobile-ui-preview `
  -screen social `
  -fixture social-party `
  -viewport fold-outer

go run ./cmd/mobile-ui-preview `
  -screen social `
  -fixture social-long `
  -width 2268 `
  -height 832
```

Fixtures cover a normal Friends tab, a full Party tab, empty state, long
rosters, incoming friend/party requests, party settings, and offline
action-disabled state. Tests cover tab switching, selection/detail behavior,
delete/expel/leave/response/settings command emission, whisper target
selection, targeting cancellation precedence, scroll clamping, safe bounds,
48px rows, and mobile-only touch ownership.

The Android host exposes Social from the HUD menu and renders it through the
same production host path as the other mobile surfaces. No Android, JNI,
WGPU, renderer, or desktop UI import exists in `mobileui`.

## Limitations

- Party-name, invite-name, and whisper composition use the native Android
  keyboard bridge. The renderer-neutral social controller still owns draft,
  confirmation, and semantic command emission.
- The live 2008 profile accepts the existing 28-byte `CZ_MAKE_GROUP2`
  party-create packet and projects the resulting party back to the mobile Party
  tab.
- Guild, crafting, refinement, cards, vending setup, and storage/cart remain
  separate parity slices.
- Adding a new friend is not yet supported.
- The physical device gate remains open.
