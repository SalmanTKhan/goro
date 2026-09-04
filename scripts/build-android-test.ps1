[CmdletBinding()]
param(
    [switch]$NoLaunch,
    [ValidateSet('mobileui','desktop')][string]$UiMode = 'mobileui',
    [string]$Avd = 'Medium_Phone_API_36.1',
    [string]$AssetRoot = '',
    [switch]$Landscape,
    [switch]$FPS,
    [switch]$NoVSync
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$hostRoot = Join-Path $repo 'android\host'
$goRoot = Join-Path $hostRoot 'go'
$sdk = if ($env:ANDROID_HOME) { $env:ANDROID_HOME } else { 'C:\Users\salma\AppData\Local\Android\Sdk' }
$ndk = Join-Path $sdk 'ndk\29.0.14206865'
$jdk = 'C:\Program Files\Microsoft\jdk-17.0.12.7-hotspot'
$adb = Join-Path $sdk 'platform-tools\adb.exe'
$emulator = Join-Path $sdk 'emulator\emulator.exe'
$gradle = Join-Path $hostRoot 'gradlew.bat'
$armOut = Join-Path $hostRoot 'app\src\main\jniLibs\arm64-v8a'
$x86Out = Join-Path $hostRoot 'app\src\main\jniLibs\x86_64'

foreach ($path in @($sdk,$ndk,$adb,$emulator,$gradle)) { if (!(Test-Path $path)) { throw "Android prerequisite missing: $path" } }
$env:JAVA_HOME = $jdk
$env:ANDROID_HOME = $sdk
$env:ANDROID_NDK_HOME = $ndk
$env:CGO_ENABLED = '1'
$env:PATH = "$(Join-Path $jdk 'bin');$(Join-Path $sdk 'platform-tools');$env:PATH"

function Write-TextUtf8NoBom([string]$path, [string]$text) {
    [System.IO.File]::WriteAllText($path, $text, (New-Object System.Text.UTF8Encoding($false)))
}

function Prepare-ModuleOverlay([string]$name, [string]$source, [string]$destination) {
    if (!(Test-Path $source)) { throw "Pinned module source missing: $source" }
    if (Test-Path $destination) { Remove-Item -LiteralPath $destination -Recurse -Force }
    Copy-Item -LiteralPath $source -Destination $destination -Recurse
}

$moduleCache = Join-Path (go env GOPATH) 'pkg\mod'
Prepare-ModuleOverlay 'wgpu' (Join-Path $moduleCache 'github.com\gogpu\wgpu@v0.31.6') (Join-Path $goRoot '.emulator-wgpu')
Get-ChildItem -LiteralPath (Join-Path $goRoot '.emulator-wgpu') -Recurse -Force -File | ForEach-Object { $_.IsReadOnly = $false }
Get-ChildItem -LiteralPath (Join-Path $goRoot '.emulator-wgpu') -Recurse -Force -File | ForEach-Object {
    $text = Get-Content -LiteralPath $_.FullName -Raw
    if ($text) { Write-TextUtf8NoBom $_.FullName $text.Replace('//go:build android && arm64', '//go:build android') }
}
Prepare-ModuleOverlay 'goffi' (Join-Path $moduleCache 'github.com\go-webgpu\goffi@v0.6.3') (Join-Path $goRoot '.emulator-goffi')
Get-ChildItem -LiteralPath (Join-Path $goRoot '.emulator-goffi') -Recurse -Force -File | ForEach-Object { $_.IsReadOnly = $false }
Get-ChildItem -LiteralPath (Join-Path $goRoot '.emulator-goffi\internal\dl') -Filter '*.s' -File | ForEach-Object {
    $text = Get-Content -LiteralPath $_.FullName -Raw
    $text = $text.Replace('(linux || darwin || freebsd)', '((linux && !android) || darwin || freebsd)')
    $text = $text.Replace('(linux || darwin || freebsd) &&', '((linux && !android) || darwin || freebsd) &&')
    Write-TextUtf8NoBom $_.FullName $text
}
Copy-Item -LiteralPath (Join-Path $goRoot 'compat\errno_android_amd64.go') -Destination (Join-Path $goRoot '.emulator-goffi\internal\syscall\errno_android_amd64.go') -Force
$dlSource = Join-Path $goRoot '.emulator-goffi\internal\dl\dl_android_cgo.go'
$dlText = Get-Content -LiteralPath $dlSource -Raw
$dlText = $dlText.Replace('//go:build android && cgo && arm64', '//go:build android && cgo')
Write-TextUtf8NoBom $dlSource $dlText
$dlConstants = Join-Path $goRoot '.emulator-goffi\internal\dl\dl_android.go'
$constantText = Get-Content -LiteralPath $dlConstants -Raw
$constantText = $constantText.Replace('//go:build android && arm64', '//go:build android')
Write-TextUtf8NoBom $dlConstants $constantText
$ffiSource = Join-Path $goRoot '.emulator-goffi\ffi\dl_android.go'
$ffiText = Get-Content -LiteralPath $ffiSource -Raw
$ffiText = $ffiText.Replace('//go:build android && arm64', '//go:build android && (arm64 || amd64)')
Write-TextUtf8NoBom (Join-Path $goRoot '.emulator-goffi\ffi\dl_android_amd64.go') $ffiText

$armCc = Join-Path $ndk 'toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android29-clang.cmd'
$armCxx = Join-Path $ndk 'toolchains\llvm\prebuilt\windows-x86_64\bin\aarch64-linux-android29-clang++.cmd'
$x86Cc = Join-Path $ndk 'toolchains\llvm\prebuilt\windows-x86_64\bin\x86_64-linux-android29-clang.cmd'
$x86Cxx = Join-Path $ndk 'toolchains\llvm\prebuilt\windows-x86_64\bin\x86_64-linux-android29-clang++.cmd'
foreach ($path in @($armCc,$armCxx,$x86Cc,$x86Cxx)) { if (!(Test-Path $path)) { throw "NDK compiler missing: $path" } }

New-Item -ItemType Directory -Force -Path $armOut,$x86Out | Out-Null
Push-Location $goRoot
try {
    $env:GOOS = 'android'; $env:GOARCH = 'arm64'; $env:CC = $armCc; $env:CXX = $armCxx
    go build -tags nofakecgo -buildmode=c-shared -trimpath -o (Join-Path $armOut 'libgoro_android.so') .
    if ($LASTEXITCODE -ne 0) { throw "arm64 Go build failed with exit code $LASTEXITCODE" }
    $env:GOARCH = 'amd64'; $env:CC = $x86Cc; $env:CXX = $x86Cxx
    go build -modfile .\go.emulator.mod -tags nofakecgo -buildmode=c-shared -trimpath -o (Join-Path $x86Out 'libgoro_android.so') .
    if ($LASTEXITCODE -ne 0) { throw "x86_64 Go build failed with exit code $LASTEXITCODE" }
} finally { Pop-Location }

$configPath = Join-Path $hostRoot 'app\src\main\assets\goro-fixture\goro\goro.ini'
New-Item -ItemType Directory -Force -Path (Split-Path $configPath) | Out-Null
$assetsRoot = Join-Path $hostRoot 'app\src\main\assets'

# Resolve the fixture's parts by role rather than assuming a directory shape.
# Fixtures exist in two layouts: the staged one the APK consumes
# (offline\content.json, goro-fixture\renderer-fixture.grf) and an older flat
# one (offline-content.json, <map>.grf). Accept either.
function Resolve-FixturePart([string]$root, [string[]]$candidates, [string]$glob) {
    foreach ($candidate in $candidates) {
        $path = Join-Path $root $candidate
        if (Test-Path -LiteralPath $path -PathType Leaf) { return (Resolve-Path -LiteralPath $path).Path }
    }
    if ($glob) {
        $match = Get-ChildItem -LiteralPath $root -Filter $glob -File -ErrorAction SilentlyContinue |
            Sort-Object Length -Descending | Select-Object -First 1
        if ($match) { return $match.FullName }
    }
    return $null
}

$offlineContent = $null
$mapArchive = $null
$mapArchiveIsPak = $false
if ($AssetRoot) {
    if (!(Test-Path -LiteralPath $AssetRoot)) { throw "Asset root missing: $AssetRoot" }
    if ((Resolve-Path -LiteralPath $AssetRoot).Path -eq (Resolve-Path -LiteralPath $assetsRoot).Path) {
        throw "AssetRoot must be a separate source fixture, not the staged assets directory: $assetsRoot"
    }
    $offlineContent = Resolve-FixturePart $AssetRoot @('offline\content.json','offline-content.json') $null
    if (!$offlineContent) {
        throw "AssetRoot has no offline content (looked for offline\content.json and offline-content.json): $AssetRoot"
    }
    $pak = Resolve-FixturePart $AssetRoot @('goro-fixture\data.pak','data.pak') '*.pak'
    if ($pak) {
        $mapArchive = $pak
        $mapArchiveIsPak = $true
    } else {
        $mapArchive = Resolve-FixturePart $AssetRoot @('goro-fixture\renderer-fixture.grf','renderer-fixture.grf') '*.grf'
        if (!$mapArchive) { throw "AssetRoot has no map archive (looked for a .pak or .grf): $AssetRoot" }
    }
} else {
    # No fixture supplied: reuse whatever is already staged, so a rebuild does
    # not require re-specifying the source every time.
    $stagedContent = Join-Path $assetsRoot 'offline\content.json'
    if (!(Test-Path -LiteralPath $stagedContent)) {
        throw 'AssetRoot is required: no fixture is staged yet. Provide a directory containing the offline content and a map archive.'
    }
    Write-Host "asset-stage source=already-staged path=$assetsRoot"
}
$vsyncValue = if ($NoVSync) { 'false' } else { 'true' }
$fpsValue = if ($FPS) { 'true' } else { 'false' }
Write-TextUtf8NoBom $configPath "[render]`nvsync=$vsyncValue`nfps=$fpsValue`n[mobile]`npresentation=$UiMode`nmode=offline`n"
if ($AssetRoot) {
    # Stage each part into the layout MainActivity extracts from, rather than
    # mirroring the source tree. That is what lets a flat fixture be used
    # directly, and it keeps the staged directory free of stray files.
    $stageContent = Join-Path $assetsRoot 'offline\content.json'
    New-Item -ItemType Directory -Force -Path (Split-Path $stageContent) | Out-Null
    Copy-Item -LiteralPath $offlineContent -Destination $stageContent -Force

    $archiveName = if ($mapArchiveIsPak) { 'data.pak' } else { 'renderer-fixture.grf' }
    $stageArchive = Join-Path $assetsRoot (Join-Path 'goro-fixture' $archiveName)
    New-Item -ItemType Directory -Force -Path (Split-Path $stageArchive) | Out-Null
    Copy-Item -LiteralPath $mapArchive -Destination $stageArchive -Force

    # A GRF and a PAK are alternatives; leaving both staged would let the app
    # silently keep using the stale one.
    $staleName = if ($mapArchiveIsPak) { 'renderer-fixture.grf' } else { 'data.pak' }
    $stale = Join-Path $assetsRoot (Join-Path 'goro-fixture' $staleName)
    if (Test-Path -LiteralPath $stale) { Remove-Item -LiteralPath $stale -Force }

    Write-Host "asset-stage content=$offlineContent archive=$mapArchive staged-as=$archiveName"
}
Push-Location $hostRoot
try { & $gradle ':app:assembleDebug'; if ($LASTEXITCODE -ne 0) { throw "Gradle build failed with exit code $LASTEXITCODE" } } finally { Pop-Location }

$apk = Join-Path $hostRoot 'app\build\outputs\apk\debug\app-debug.apk'
if (!$NoLaunch) {
    & $adb 'devices' | Out-Null
    $emulatorDevice = (& $adb devices) | Select-String '^emulator-.*\s+device$'
    if (!$emulatorDevice) {
        Start-Process -WindowStyle Hidden -FilePath $emulator -ArgumentList '-avd', $Avd, '-no-snapshot-load', '-no-boot-anim'
        $deadline = (Get-Date).AddMinutes(3)
        do { Start-Sleep -Seconds 2; $boot = (& $adb shell getprop sys.boot_completed 2>$null) } while ($boot -notmatch '1' -and (Get-Date) -lt $deadline)
        if ($boot -notmatch '1') { throw "Emulator did not finish booting: $Avd" }
    }
    & $adb install -r $apk
    $rotation = if ($Landscape) { 1 } else { 0 }
    & $adb shell cmd window user-rotation lock $rotation
    & $adb logcat -c
    & $adb shell am force-stop com.kivutar.goro.host
    & $adb shell monkey -p com.kivutar.goro.host 1 | Out-Null

    # The desktop presentation exposes the deterministic Offline checkbox and
    # Login button. Select it before capturing so screenshots show the world,
    # rather than the startup/login surface.
    Start-Sleep -Seconds 3
    if ($Landscape) {
        & $adb shell input tap 870 610
        Start-Sleep -Milliseconds 250
        & $adb shell input tap 1170 710
    } else {
        & $adb shell input tap 440 1900
        Start-Sleep -Milliseconds 250
        & $adb shell input tap 650 2070
    }
    $worldDeadline = (Get-Date).AddSeconds(30)
    do {
        Start-Sleep -Seconds 1
        $worldReady = (& $adb logcat -d -s GoroAndroidGo:I GoroAndroidHost:I) -match 'stage=mobile-metrics frames=([6-9][0-9]|[1-9][0-9][0-9])'
    } while (!$worldReady -and (Get-Date) -lt $worldDeadline)
    if (!$worldReady) { throw 'World did not reach the measured-frame gate before screenshot capture' }
    Start-Sleep -Seconds 2
    $remoteScreenshot = '/sdcard/goro-android-test.png'
    & $adb shell screencap -p $remoteScreenshot
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to capture emulator screenshot"
    }
    & $adb pull $remoteScreenshot (Join-Path $hostRoot 'app\build\android-test.png') | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to copy emulator screenshot"
    }
    & $adb shell rm -f $remoteScreenshot
    & $adb logcat -d -s GoroAndroidHost GoroAndroidGo > (Join-Path $hostRoot 'app\build\android-test.log')
}
Write-Host "APK: $apk"
