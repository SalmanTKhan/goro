# Goro Mobile

The mobile client runs the production Goro renderer inside an Android
native-surface host and drives it through a platform-independent presentation
and command boundary. It supports an explicit **Offline** mode (local authority,
generated content pack) and an **Online** mode (the existing `network.Client`,
login, session, and world packet paths). Offline mode never silently falls back
to network authority.

The desktop `ui` package and its behavior are unchanged. `mobileui` is a
separate top-level package of read-only presentation models, deterministic
safe-area layout, and semantic `input.PlayerCommand` emission; it has no
Android, JNI, WGPU, renderer, or packet dependency.

## Foundations

- [architecture.md](architecture.md) — offline/online projection, session
  contract, startup, content pack build, save format.
- [renderer-core-extraction.md](renderer-core-extraction.md) — raw WGPU
  device/frame interface that lets the Android host drive the production
  renderer.
- [input.md](input.md) — semantic input boundary, gesture recognition, and
  UI/world touch ownership.
- [ui.md](ui.md) — HUD architecture, deterministic layout, the
  Fold outer `2268x832` target, and the mobile visual contract.
- [startup.md](startup.md) — title/loading gate and profile
  entry point.
- [online.md](online.md) — online session bootstrap and local
  server compatibility.
- [assets.md](assets.md) — deterministic closure-pack tooling
  (chunked PAK + compatibility GRF + metrics manifest).

## Screens

Each screen is a read-only projection plus a controller that owns only
screen-local selection/scroll/navigation state and emits existing semantic
commands. The headless `cmd/ui-preview` harness lays out every screen
without assets or a renderer.

- [profile.md](profile.md) — offline profile and character
  creation.
- [inventory.md](inventory.md) — inventory and equipment.
- [npc-shop.md](npc-shop.md) — NPC dialog, shop, storage,
  quantity, and scrolling.
- [character-skills.md](character-skills.md) — Character and
  Skills read-only screens.
- [combat.md](combat.md) — attack/skill targeting boundary and
  offline combat presentation.
- [loot.md](loot.md) — nearby-loot rail and pickup.
- [map.md](map.md) — minimap screen and warp navigation.
- [chat-settings.md](chat-settings.md) — world chat transcript,
  composer, and bounded settings surface.
- [social.md](social.md) — Friends and Party tabs, requests, and
  party settings.
- [trade.md](trade.md) — player-to-player trade.
- [vending.md](vending.md) — vending browse and buy.

## Android host

[android/host](../../android/host/README.md) owns the Java Activity and
`SurfaceView`, the JNI/NDK `ANativeWindow` reference, the locked Go render
thread, WGPU Android surface creation, safe-area insets, native text input, and
raw touch forwarding into `input.State`. It uses a nested module for the aligned
WGPU Android preview and changes no root dependency or gameplay code.

Generated content, GRFs, packs, the native library, and the APK are local build
artifacts. The real-data and device-acceptance route is a validation gate
whenever source or device inputs change.
