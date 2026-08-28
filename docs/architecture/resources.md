# Resources (`res`)

`res` loads and decodes original client data. No gameplay policy lives here.

## Manager

`res.NewManager(dataDir)` builds the resource view. Data directory resolution
order: `--data-dir`, `data_dir` in `goro.ini`, current working directory.
It discovers `clientinfo.xml` / `sclientinfo.xml` under `data/`, the root, or
`System/`.

`Manager` holds loose files, `Packs` (GOROPAK), `Archives` (GRF), and
`Overlays` (downloadable mobile packs mounted on top).

It also lazily caches parsed tables: accessory names, item metadata, non-PC
resource names, indoor RSW names, camera view points, `msgstringtable` entries,
fog parameters, skill resource names / max levels / display names /
descriptions, song talks, and pet talks. Each has a `...Loaded` flag — follow
that pattern when adding a table rather than parsing eagerly at startup.

`PreferOptimizedTextures` is set when a mobile pack ships pre-decoded textures;
source resources stay as the fallback.

## Formats

| File | Format |
|---|---|
| `grf.go`, `grf_decrypt.go`, `grf_writer.go` | GRF archives (legacy/alpha tables, LZSS, entry decryption), plus writing |
| `pak.go` | GOROPAK v1 — the project's deterministic chunked archive |
| `overlay.go`, `delivery.go` | Overlay mounting and pack delivery |
| `gnd.go`, `gnd_diagnostics.go` | Ground mesh, lightmaps |
| `gat.go` | Walkability/altitude tiles |
| `rsw.go` | Map world: models, lights, sounds, effects, water, fog |
| `rsm.go` | RSM 3D models |
| `gr2.go` with `gr2_geometry.go`, `gr2_bink.go`, `gr2_oodle.go`, `gr2_range.go` | Granny 3D models and their compression |
| `spr.go`, `act.go`, `pal.go`, `imf.go` | Sprites, animation actions, palettes, IMF |
| `str.go` | STR effect animations |
| `image.go`, `tga.go` | Image decoding |
| `lub.go` | RO Lua 5.1 `.lub` bytecode translated to GopherLua (see `TODO.md` for the reflect/unsafe caveat) |
| `msgstring.go` | `msgstringtable.txt` |
| `clientinfo.go` | Server connection list |
| `item_resource.go`, `player_resource.go`, `nonpc_resource.go`, `skill_resource.go` | Resource-name tables |
| `song_talk.go`, `pet_talk.go` | Client-side talk lines |

## Testing

`*_test.go` files are synthetic and always run. `*_real_test.go` files need a
real client data directory and skip when it is absent — never make the real-data
tests mandatory in CI.

## Related tooling

`cmd/grf-pack`, `cmd/grf-extract`, `cmd/grf-to-pak`, `cmd/build-render-fixture`
(a small GRF containing one static map's resources, used by render tests).
