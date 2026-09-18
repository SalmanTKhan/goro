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
