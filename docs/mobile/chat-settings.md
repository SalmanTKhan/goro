# Mobile chat and settings

Two renderer-neutral surfaces on top of the mobile HUD:

- a transcript/composer surface for the existing world chat path;
- a bounded settings surface for mobile-only navigation into client settings.

They do not add offline chat authority, account mutation, or new network
packets.

## Ownership

Online chat messages are owned by `game.WorldMode` and its existing desktop
`ui.ChatConsole`. `WorldMode.MobileChatModel` copies the console's text into
`mobileui.MobileChatModel`; sender parsing is presentation-only. The Android
host draws that model and `ChatController` emits the existing
`input.CommandSendGlobalChat` command. `game.WorldMode.ApplyPlayerCommand`
continues to validate the session and sends through the existing network
client.

Offline mode intentionally projects an explicit system notice and disables
the composer. `OfflineSession` has no chat authority, so the mobile layer does
not pretend that a local message was sent or persist a fake transcript.

Settings are represented by `SurfaceModel` and `SurfaceController`. The first
surface exposes Audio, Controls, Display, and an explicitly disabled
online-only Account entry. Selecting a row opens a local detail sheet; actual
settings mutation remains on the existing desktop/client settings paths until
an authoritative mobile settings contract is defined.

## Layout and interaction

Both surfaces use the safe `Viewport` rather than Android coordinates. Chat
uses a centered panel bounded to 900 logical pixels. Settings uses a centered
panel bounded to 1200 logical pixels on landscape/Fold displays. Chat rows
retain their source message index while scrolling, so the visible transcript
does not drift from the scroll offset.

All open chat touches are modal-owned. Settings touches are owned while the
settings screen is active. The HUD menu exposes Settings in landscape as well
as portrait; no touch on either surface reaches world movement, attack, or
targeting.

The minimum mobile touch target remains 48 logical pixels. The Fold outer
viewport is covered by the same matrix as the other mobile surfaces:
`2268x832`, safe-area variants, and the portrait/standard landscape fixtures.

## Headless preview

```powershell
go run ./cmd/mobile-ui-preview `
  -screen chat `
  -fixture chat-long `
  -viewport fold-outer

go run ./cmd/mobile-ui-preview `
  -screen settings `
  -viewport fold-outer
```

The output is deterministic JSON and is a layout/model snapshot, not Android
rendering evidence. The chat fixture also supports `chat-empty`.

## Limitations

- The controller owns the draft; physical text entry goes through the Android
  host's native text-input bridge (see [mobile-online.md](mobile-online.md)).
- Offline chat is read-only by design.
- Settings rows explain or open a local detail state; audio, controls, and
  display mutation remain on the existing desktop/client settings paths.
- Guild, crafting, refinement, and cards remain separate online or future
  surfaces.
