# Architecture documentation

Per-system summaries meant to be loaded individually as context for a task. The
top-level orientation lives in `CLAUDE.md` at the repository root.

| Document | Read it when working on |
|---|---|
| [rendering.md](rendering.md) | GoGPU backend, the frame command buffer, meshes and billboards |
| [resources.md](resources.md) | GRF/PAK archives, map and sprite formats, client tables |
| [networking.md](networking.md) | Packet parsing/building, framing, adding a packet |
| [game-modes.md](game-modes.md) | The mode state machine, login flow, world mode |
| [world-and-movement.md](world-and-movement.md) | Actors, floor items, pathfinding, camera, interpolation |
| [session-state.md](session-state.md) | Durable player state and the authority boundary |
| [ui.md](ui.md) | Windows, overlays, the RO theme |
| [input-and-scripting.md](input-and-scripting.md) | Input state, player commands, the Lua bot API |
| [audio.md](audio.md) | BGM and SFX playback |
| [offline-mode.md](offline-mode.md) | Local authority, offline content pack |
| [mobile.md](mobile.md) | mobileui, mobile asset packs, the Android host |
| [gameplay-systems.md](gameplay-systems.md) | Combat, skills, items, social, pets, companions, map effects |
| [tooling.md](tooling.md) | Config, `cmd/` tools, build, CI, release, packaging |

Setup guides and design/coverage notes stay in the parent `docs/` directory;
mobile feature detail stays in `docs/mobile/`.
