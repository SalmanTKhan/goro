# Mobile startup flow

The mobile client enters through a renderer-neutral startup gate:

`title/loading gate -> profile/character selection -> create/customize or continue -> world`

The title gate owns input while it is visible, and the world simulation is paused until the player selects **CONTINUE** from the profile screen. The offline host currently constructs its game graph synchronously before showing the title gate, so this is a presentation/loading gate rather than asynchronous asset loading.

The profile screen uses the existing offline profile projection and customization commands. The startup shell is independent of Android, JNI, WGPU, and desktop UI. Online mode can use the profile phase as the handoff point for its server-authoritative login/character-list flow.

The Fold outer-display target remains part of the layout matrix: `2268x832` landscape with safe-area insets.
