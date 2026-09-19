# Controller integration boundary

The desktop controller slice is layered on top of `main`; it is not a fork of
the login, world, or renderer architecture. When synchronizing `dev` with
`main`, keep the merge focused on these boundaries:

- `input/controller.go` and `input/gamepad/`: normalized device snapshots,
  bindings, SDL3 polling, hotplug, rumble, and diagnostics.
- `render/backend.go` and `render/controller.go`: controller polling, semantic
  UI actions, virtual pointer routing, and desktop touch pointer adaptation.
- `game/controller_input.go` and `ui/`: semantic gameplay/UI consumers. Do not
  add SDL or vendor-specific checks to these layers.
- `scripts/deploy-steamdeck.ps1` and `scripts/run-steamdeck.sh`: Linux SDL3
  runtime and live-log deployment path.

After a `main` merge, reconcile conflicts in those boundaries before touching
unrelated gameplay or UI changes. Validate the merge with:

```powershell
go build ./...
go test ./input/... ./render/... ./game/... ./ui/...
staticcheck ./input/... ./render/... ./game/... ./ui/...
```

Physical acceptance remains separate from repository validation: deploy the
Linux amd64 build, confirm SDL3 and the device name in the live log, then test
PS5/Deck buttons, sticks, triggers, UI confirmation, world movement, camera,
attack, loot, and touch input.

## Steam Deck development launch

Run the deployed launcher through Steam as a Non-Steam Game and select a
standard Gamepad template for its per-game Controller Layout. The profile must
output ordinary gamepad A/B/X/Y and sticks to SDL; it should not translate
those controls into keyboard keys. Goro's positional mapping is then stable:
South/A/Cross is Confirm, East/B/Circle is Cancel, West/X/Square is Attack,
and North/Y/Triangle is Loot.

The right trackpad is an optional pointer override. Touch moves the pointer;
only a physical trackpad click or explicit Confirm produces a click. Launching
`run-steamdeck.sh` directly from Desktop Mode is useful for diagnostics, but
it may use Steam's desktop layout rather than the per-game layout.

Controller activation only changes controller glyph/focus mode. It does not
open the on-screen keyboard. Text entry opens the controller keyboard only
after an explicit Confirm on a focused text field.

For a repeatable post-merge report, run:

```powershell
./scripts/verify-controller.ps1
./scripts/verify-controller.ps1 -Staticcheck
```

The verifier is deliberately not wired into merge prevention. It reports the
controller slice's build, focused tests, and optional static analysis so a
conflict resolution can be reviewed without changing the synchronization path.
