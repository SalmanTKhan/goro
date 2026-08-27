# Mobile asset pipeline

`cmd/build-mobile-pack` creates the first-class mobile deployment pack from
the original RO data directory:

```powershell
go run ./cmd/build-mobile-pack `
  --data-dir <ro-data> `
  --map prontera `
  --out android\host\app\src\main\assets\goro-fixture\prontera.mobile.grf
```

The map command walks the closure used by the production renderer:

- `data/<map>.gnd`, `.rsw`, and `.gat`;
- GND textures;
- RSW models and their RSM textures;
- deterministic offline player, shadow, and non-PC actor resources.

When `--offline-content` is supplied, the map pack also preserves any
available NPC identity tables and includes the ACT/SPR resources referenced
by the content's NPC sprite IDs. The runtime's compiled legacy table remains
the fallback for older clients that do not ship those Lua tables. This keeps
offline NPC rendering data-driven without bundling the complete client sprite
library.

The same offline closure includes the authored damage-number and combat-message
sprites (`data/sprite/이팩트/숫자.*` and `data/sprite/이팩트/msg.*`) used by the
production combat feedback path.

It also closes the item and audio resources needed by the offline loop:

- item display/resource/description metadata and slot/card tables;
- ACT/SPR item sprites and available item icon BMPs for starter items, selected
  shop stock, and drops from monsters on the selected maps;
- `BGM/01.mp3` plus the map table fallback;
- Bash, level-up, starter knife attack/hit, and normal enemy-hit WAVs.

The item closure is intentionally content-driven. Items with no authored
resource name remain valid gameplay data and use the renderer's existing
fallback marker; they are reported in the pack manifest rather than being
given fabricated art.

It writes `<output>.json` unless `--manifest` is supplied. The manifest
contains the scope, selected maps, original GRF reference, included-file
count, source and archive byte counts, per-category counts/bytes, missing
dependencies, and a SHA-256 of the resulting pack. A missing GND remains
fatal; optional closure misses are explicit in the manifest and command
output.

Resource paths extracted from binary map/model data are canonicalized through
the same GRF lookup path before staging. This is important for EUC-KR texture
directories: raw byte paths can otherwise pack successfully while leaving RSM
faces unresolved at runtime.

To build the complete effective resource view, use `--complete`:

```powershell
go run ./cmd/build-mobile-pack `
  --data-dir <ro-data> `
  --complete `
  --out android\host\app\build\mobilepack\complete.mobile.grf
```

Complete mode includes loose resource files and all entries from the
top-level GRF/GPF archives, preserving the same first-match precedence as
`res.Manager`. It deliberately excludes client executables, libraries, patch
containers, and runtime-state directories. The staging directory is created
beside the output so large packs use the output volume rather than the system
temporary drive.

For a visual workflow, run the local browser editor:

```powershell
go run ./cmd/mobile-pack-editor `
  --data-dir <ro-data> `
  --out android\host\app\build\mobilepack\mobile.mobile.grf
```

Open the printed local URL. The editor inventories maps and source PAK/GRF/GPF containers,
supports selecting or clearing map closures, and can build either the selected
union or the complete pack. The inventory and pack manifest are deterministic,
so later category-level controls can be added without making the UI a second
resource authority.

This is a deployment/closure optimization, not a new runtime resource
authority: the legacy map command still produces an ordinary GRF, while the
project composer produces a chunked PAK plus GRF compatibility output. Both
load through `res.Manager`; the source GRF/data path remains the reference for
correctness. Pre-baked terrain meshes, sprite atlases, and ASTC/ETC2
transcoding remain separate measured format revisions.

## Reproducible asset projects

The project composer creates a lossless base pack plus incremental optional
packs, with exact stored-source, expanded-resource, PAK-size, and GRF-size
estimates:

```powershell
go run ./cmd/build-mobile-assets `
  --data-dir <ro-data> `
  --project build\mobile-assets.json `
  --offline-content offline\content.json `
  --init

go run ./cmd/build-mobile-assets `
  --project build\mobile-assets.json `
  --plan

go run ./cmd/build-mobile-assets `
  --project build\mobile-assets.json `
  --output-dir build\mobile-assets
```

The output contains `mobile-assets.json`, `validation.json`, and both numbered
PAK and GRF packs. PAK is the preferred mobile container: resources use
independent 256 KiB chunks, Zstandard where useful, and raw storage for
already-compressed media. The builder streams resources directly from loose
files and source archives; it does not create a complete expanded copy of the
client data. The Android host loads the profile named by `default_profile`,
verifies each selected pack hash, and extracts only that profile into
app-private storage.
For the current offline host, supply the generated `offline/content.json` at
initialization or in the editor; a missing file is reported as a base-pack
warning because the renderer closure itself does not contain gameplay content.

To compare output size and read behavior for a real project, run:

```powershell
go run ./cmd/benchmark-mobile-assets `
  --data-dir <ro-data> `
  --project build\mobile-assets.json `
  --output-dir build\mobile-assets-benchmark
```

The report includes build time, PAK/GRF archive size, sequential reads, and
deterministic random reads. Android startup, frame, and RSS measurements are
written by the host to `mobile-runtime-metrics.json`.
