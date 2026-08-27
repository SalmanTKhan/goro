# Android native-surface host

This host owns the Android Activity, `SurfaceView`, JNI/NDK `ANativeWindow`
reference, and locked Go render thread. It connects those platform pieces to
the production Goro renderer and the shared Goro online/offline session path.
Offline mode uses local authority; Online mode uses the existing network,
login, session, and world packet paths. The host also uses the production
audio package through Oto's Android Oboe backend when built with the
`nofakecgo` tag.

The physical-device package target is `arm64-v8a`. The debug APK also includes
an `x86_64` library for the local emulator validation path; the emulator ABI is
not a substitute for the ARM64 physical-device gate.

## Contract

- arm64-v8a production target;
- Android API 29 minimum, API 36 compile/target;
- orientation-responsive Activity; portrait is canonical and landscape/Fold
  layouts are supported when the safe width permits;
- Vulkan only;
- Java owns the Activity and `SurfaceView`;
- C owns the acquired `ANativeWindow` reference;
- the Go render thread owns WGPU surface/device operations;
- the C reference is released only after Go has released the WGPU surface;
- touch events are forwarded to `goro/input.State` without mouse emulation;
- Java `WindowInsets` are forwarded to the mobile safe-area layout.
- one-finger world input uses the persisted Hold-to-move/Tap-to-move setting;
- two-finger drag rotates the camera and pinch remains zoom;
- the active ground touch shows the shared classic green walk-cell highlight;
- settings changes apply on the render thread and are saved in the user INI.

## Dependencies

The host uses a nested module so the Phase 0 experiment can consume the
released WGPU Android preview (`github.com/gogpu/wgpu@v0.31.6`) without
changing the root Goro dependency graph. The root module is used only for the
existing `input.State` package.

Required local tools:

- Go 1.26+;
- Android SDK platform 36 and build tools;
- Android NDK r29 (`29.0.14206865`) with the arm64 API-29 clang toolchain;
- CMake 3.22.1;
- JDK 17 (the verified workstation build uses Microsoft OpenJDK 17.0.12);
- the repository-local Gradle 8.10.2 wrapper;
- an API-29+ arm64 Vulkan device for the hard gate.

## Build

From `D:\Projects\goro\android\host`:

```powershell
$env:ANDROID_HOME = 'C:\Users\salma\AppData\Local\Android\Sdk'
$env:ANDROID_NDK_HOME = "$env:ANDROID_HOME\ndk\29.0.14206865"
$env:CC = "$env:ANDROID_NDK_HOME\toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android29-clang.cmd"
$env:CXX = "$env:ANDROID_NDK_HOME\toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android29-clang++.cmd"
$env:GOOS = 'android'
$env:GOARCH = 'arm64'
$env:CGO_ENABLED = '1'
Push-Location go
go build -tags nofakecgo -buildmode=c-shared -trimpath -o ..\app\src\main\jniLibs\arm64-v8a\libgoro_android.so .
Pop-Location
.\gradlew.bat :app:assembleDebug
```

For the x86_64 emulator, rebuild the same Go package with the emulator
toolchain and output directory before running Gradle:

```powershell
$env:CC = "$env:ANDROID_NDK_HOME\toolchains\llvm\prebuilt\windows-x86_64\bin\x86_64-linux-android29-clang.cmd"
$env:CXX = "$env:ANDROID_NDK_HOME\toolchains\llvm\prebuilt\windows-x86_64\bin\x86_64-linux-android29-clang++.cmd"
$env:GOARCH = 'amd64'
Push-Location go
go build -tags nofakecgo -buildmode=c-shared -trimpath -o ..\app\src\main\jniLibs\x86_64\libgoro_android.so .
Pop-Location
.\gradlew.bat :app:assembleDebug
```

Install and run:

```powershell
adb install -r app\build\outputs\apk\debug\app-debug.apk
adb shell am force-stop com.kivutar.goro.host
adb shell monkey -p com.kivutar.goro.host 1
adb logcat -s GoroAndroidHost GoroAndroidGo
```

The host prefers an embedded `goro-fixture/data.pak` asset. At startup it
extracts that archive to the app-private files directory, logs its size, and
loads the Prontera map at surface creation. The legacy
`goro-fixture/renderer-fixture.grf` remains as a fallback when the PAK is not
available. A developer override remains available: if
`/storage/emulated/0/Android/data/com.kivutar.goro.host/files/goro-data/data.grf`
exists, that external root is selected instead. Push user-supplied data with:

```powershell
adb shell mkdir -p /sdcard/Android/data/com.kivutar.goro.host/files/goro-data
adb push .\data.grf /sdcard/Android/data/com.kivutar.goro.host/files/goro-data/data.grf
```

Build the local gameplay content first from the configured rAthena checkout:

```powershell
go run ./cmd/build-offline-content `
  --rathena-root D:\path\to\rathena `
  --maps prontera,prt_fild05 `
  --starter-poring `
  --out offline\content.json
```

The fixture builder is then run from the repository root with both maps and
the generated content:

```powershell
go run ./cmd/build-render-fixture `
  --data-dir D:\path\to\ro-data `
  --maps prontera,prt_fild05 `
  --offline-content offline\content.json `
  --out android\host\app\src\main\assets\goro-fixture\renderer-fixture.grf
```

The builder walks GND textures, RSW models/model textures, and the sprite
resources referenced by the supplied offline NPC, monster, inventory, shop,
and drop content. It also includes item metadata/icons, Prontera BGM, and the
authored combat/level-up WAV closure used by the offline loop. It reports
missing dependencies and a SHA-256, and uses the existing `res.Manager` and
GRF writer.
The generated asset is local test data and should remain untracked unless its
distribution rights are clear.

For a pack plus machine-readable closure metrics, use the v1 mobile-pack
command from the repository root:

```powershell
go run ./cmd/build-mobile-pack `
  --data-dir D:\path\to\ro-data `
  --maps prontera,prt_fild05 `
  --offline-content offline\content.json `
  --out android\host\app\src\main\assets\goro-fixture\renderer-fixture.grf
```

This writes `prontera.mobile.grf.json` by default. It is still a normal GRF
for the existing loader; mesh baking and GPU texture transcoding are not
silently implied by the v1 manifest.
for the existing loader; mesh baking and GPU texture transcoding are not
silently implied by the v1 manifest.

To build the complete resource view instead of one map, add `--complete`:

```powershell
go run ./cmd/build-mobile-pack `
  --data-dir D:\Spel\OldRO `
  --complete `
  --offline-content offline\content.json `
  --out android\host\app\build\mobilepack\complete.mobile.grf
```

The complete pack includes all loose resource files and all entries from the
top-level GRF/GPF archives. It can be very large after extraction; use the
local map-selection editor when the APK should receive only a tested subset:

```powershell
go run ./cmd/mobile-pack-editor `
  --data-dir D:\Spel\OldRO `
  --out android\host\app\build\mobilepack\mobile.mobile.grf
```

The editor opens a local browser page, inventories source maps, and builds a
selected-map union or complete pack. It writes the normal JSON pack manifest
next to the output.

For the reproducible modular workflow, initialize and build a project:

```powershell
go run ./cmd/build-mobile-assets `
  --data-dir D:\path\to\ro-data `
  --project build\mobile-assets.json `
  --offline-content offline\content.json `
  --init
go run ./cmd/build-mobile-assets `
  --project build\mobile-assets.json `
  --output-dir build\mobile-assets
```

The local editor can open the same project with `--project`. It supports a
lossless base pack, incremental optional packs, path include/exclude rules,
profiles, PAK/GRF size estimates, and PAK chunk/Zstandard settings. The Gradle
host stages only the selected profile and format into APK assets without
copying generated files into the source tree:

```powershell
.\gradlew.bat :app:assembleDebug `
  -PmobileAssetDir=D:\Projects\goro\build\mobile-assets `
  -PmobileAssetFormat=pak `
  -PmobileAssetProfile=default
```

`mobileAssetFormat` accepts `pak`, `grf`, or `auto`; `pak` is the default.
`mobileAssetProfile` defaults to the manifest's default profile. The Java host
selects the staged profile, extracts only its packs, checks SHA-256 values, and
falls back to GRF only when the preferred artifact is unavailable. The shared
Go resource manager can load either container directly. Pack/profile boundaries
are also the handoff point for a future split-APK or Play Asset Delivery
integration.

Optional CDN patching is enabled only when `mobileAssetPatchManifestUrl` is
provided. The manifest is Ed25519-signed and may offer `pak`, `grf`, `gpf`,
`thor`, or `rgz` artifacts per logical pack. The format property accepts
`auto` or one explicit format; `mobileAssetPatchNetworkPolicy` accepts
`unmetered` (default) or `connected`, and
`mobileAssetPatchPublicKey` supplies the base64-encoded Ed25519 public key:

```powershell
.\gradlew.bat :app:assembleDebug `
  -PmobileAssetDir=D:\Projects\goro\build\mobile-assets `
  -PmobileAssetFormat=pak `
  -PmobileAssetProfile=default `
  -PmobileAssetPatchManifestUrl=https://cdn.example/r42/mobile-delivery.json `
  -PmobileAssetPatchFormat=auto `
  -PmobileAssetPatchNetworkPolicy=unmetered `
  -PmobileAssetPatchPublicKey=<base64-ed25519-public-key>
```

The base remains playable before patch work completes. Downloads resume through
`.part` files, are hash-checked, and are activated as release-local overlays;
THOR/RGZ are consumed natively and never execute a patcher process. An empty
manifest URL performs no asset-network requests.

The resource probe and production map renderer are integrated. The generated
APK is a local debug artifact; rebuild it after changing Go, JNI, or fixture
inputs.

## Online session configuration

The Android host uses the shared goro user configuration at
`%APPDATA%\goro\goro.ini`. Set `mobile.mode = online` and configure the
selected server under `[server]`; omit the setting to keep the default
offline mode.

```ini
[mobile]
mode = online

[server]
name = Local rAthena
host = 192.168.1.20
auth_port = 6900
char_port = 6121
zone_port = 5121
```

Credentials continue to use the shared `[login]` section. The mobile runtime
uses the same packet/session/login implementation as desktop and does not
silently fall back to offline authority after a failed connection.

For the Android Emulator connecting to a server on the same Windows host, use
`10.0.2.2` rather than `127.0.0.1`. The verified Sabine 2008 profile is:

```ini
[mobile]
mode = online

[server]
name = Local Sabine
host = 10.0.2.2
auth_port = 6900
char_port = 6121
zone_port = 5121
client_date = 20080910
profile = 23
```

The mobile host reads this through the normal user-config path after the
Android fixture root is selected. For a local test install, the config can be
staged with `adb shell run-as` into
`files/goro-fixture/goro/goro.ini`; keep credentials in that local file rather
than committing them.
