# Mobile online slice

## Scope

The Android host supports an explicit Online/Offline selector. Online mode uses
the existing `network.Client`, `game.LoginMode`, `session.Session`, and world
packet projections. Offline mode creates a separate local authority and does not
fall back to it when an online connection fails.

The initial server entry is local configuration. `config.ServerCatalogProvider`
is the discovery seam for a future registry; remote asset-diff and item-metadata
providers must remain read-only and versioned. Shared character storage and
cross-server imports are out of scope for this phase.

## Server configuration

Online mode is selected through the mobile session config; the server endpoint
is supplied explicitly and credentials stay in the shared `[login]` section, not
the APK:

```ini
[mobile]
mode = online

[server]
name = Local Server
host = 10.0.2.2
auth_port = 6900
char_port = 6121
zone_port = 5121
client_date = 20080910
profile = 23
```

`10.0.2.2` is the Android Emulator route to the host machine; a physical device
uses the real server address.

## Bootstrap flow

Online session bootstrap is implemented in the mobile host: connect,
authenticate, handle the account-to-character-server handoff, list and
select/create a character, take the zone handoff, and enter the map through the
existing online login protocol. The mobile semantic command layer then forwards
the shared network paths for movement, attacks, skills, item use, NPC
interaction, chat (through the native Android text-input bridge), inventory,
storage, trade, vending, friend/party actions, and explicit disconnect. Disconnect
returns to character selection without automatic re-entry.

Party creation uses the existing `CZ_MAKE_GROUP2` (`0x01E8`) packet path.
Packet builders, parsers, and mobile projections are covered by the repository's
Go tests.

## Build and validation

Build both ABIs from `android/host/go` with the Android API-29 clang toolchains,
then package the debug APK from `android/host`:

```powershell
$env:GOOS = 'android'
$env:GOARCH = 'amd64' # repeat with arm64
$env:CGO_ENABLED = '1'
go build -tags nofakecgo -buildmode=c-shared -trimpath `
  -o ..\app\src\main\jniLibs\x86_64\libgoro_android.so .

.\gradlew.bat :app:assembleDebug
```

The debug APK includes `x86_64` for the local emulator and `arm64-v8a` for the
production device target. Repository validation for this slice: `go test ./...`,
`go test -race ./...`, `go vet ./...`, and the Gradle APK build all pass.

## Qualification boundary

Live evidence to date is emulator evidence: x86_64 runtime on the local Vulkan
emulator against a local server. ARM64 is build-validated; physical Android GPU,
thermal, lifecycle, and touch behavior remain a device gate. The future server
registry, remote diff downloads, merged item API, and shared character storage
are intentionally not implemented here.
