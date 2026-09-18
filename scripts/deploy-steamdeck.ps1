param(
    [string]$DeckHost = $env:GORO_DECK_HOST,
    [string]$DataSource = $env:GORO_DATA_SOURCE,
    [string]$SteamGameId = "goro",
    [string]$Branch = "",
    [switch]$PushData,
    [switch]$ResetData,
    [switch]$NoBuild,
    [switch]$SkipSteamShortcut,
    [switch]$SkipSteamArtwork
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = if (Test-Path (Join-Path $ScriptDir "go.mod")) {
    $ScriptDir
} else {
    Split-Path -Parent $ScriptDir
}

if (-not (Test-Path (Join-Path $RepoRoot "go.mod"))) {
    throw "Could not locate repository root from: $ScriptDir"
}

if ([string]::IsNullOrWhiteSpace($DeckHost)) {
    throw @"
No Steam Deck host specified.

Use either:
  `$env:GORO_DECK_HOST = "<deck-host-or-ip>"
or:
  .\scripts\deploy-steamdeck.ps1 -DeckHost "<deck-host-or-ip>"
"@
}

if ($ResetData -and -not $PushData) {
    throw "-ResetData requires -PushData."
}

if (-not [string]::IsNullOrWhiteSpace($Branch) -and $NoBuild) {
    throw "-Branch cannot be combined with -NoBuild because selecting a branch requires building that branch."
}

# Branch deployments use a detached temporary git worktree. This keeps the
# developer's current checkout/branch and any uncommitted work untouched.
$BuildSourceRoot = $RepoRoot
$BranchWorktree = $null
$BuildBranchLabel = "current-worktree"
$BuildCommit = ""

if (-not [string]::IsNullOrWhiteSpace($Branch)) {
    & git -C $RepoRoot rev-parse --is-inside-work-tree *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "-Branch requires the repository to be a Git worktree."
    }

    Write-Host "Fetching branch '$Branch' from origin..."
    & git -C $RepoRoot fetch --prune origin $Branch
    if ($LASTEXITCODE -ne 0) {
        throw "git fetch failed for origin/$Branch with exit code $LASTEXITCODE"
    }

    & git -C $RepoRoot rev-parse --verify --quiet "refs/remotes/origin/$Branch" *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "Remote branch origin/$Branch was not found."
    }

    $safeBranch = $Branch -replace '[^A-Za-z0-9._-]', '_'
    $BranchWorktree = Join-Path $RepoRoot "dist\steamdeck-worktrees\$safeBranch"

    # Clean up a previous interrupted branch deployment.
    & git -C $RepoRoot worktree remove --force $BranchWorktree *> $null
    Remove-Item $BranchWorktree -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force (Split-Path -Parent $BranchWorktree) | Out-Null

    Write-Host "Creating detached build worktree for origin/$Branch..."
    & git -C $RepoRoot worktree add --detach $BranchWorktree "origin/$Branch"
    if ($LASTEXITCODE -ne 0) {
        throw "git worktree add failed for origin/$Branch with exit code $LASTEXITCODE"
    }

    $BuildSourceRoot = $BranchWorktree
    $BuildBranchLabel = "origin/$Branch"
}

Push-Location $BuildSourceRoot
try {
    $BuildCommit = (& git rev-parse --short=12 HEAD | Out-String).Trim()
}
finally {
    Pop-Location
}

$BuildDir = Join-Path $RepoRoot "dist\steamdeck"
$BuildExe = Join-Path $BuildDir "goro"
$SDLRuntime = Join-Path $BuildDir "libSDL3.so.0"
$IconSource = Join-Path $BuildSourceRoot "packaging\linux\goro.png"
if (-not (Test-Path $IconSource)) {
    $IconSource = Join-Path $RepoRoot "packaging\linux\goro.png"
}
$BuildIcon = Join-Path $BuildDir "goro.png"
$LauncherSource = Join-Path $BuildSourceRoot "scripts\run-steamdeck.sh"
if (-not (Test-Path $LauncherSource)) {
    # Keep the deployment helper available when building an older branch from
    # a temporary worktree that predates the launcher.
    $LauncherSource = Join-Path $RepoRoot "scripts\run-steamdeck.sh"
}
$LauncherBuild = Join-Path $BuildDir "run-steamdeck.sh"

$SteamArtworkSourceDir = Join-Path $BuildSourceRoot "packaging\steamdeck\artwork"
if (-not (Test-Path $SteamArtworkSourceDir)) {
    # Deployment infrastructure/artwork may exist only on the developer branch.
    # Reuse it while still building the selected client branch exactly.
    $SteamArtworkSourceDir = Join-Path $RepoRoot "packaging\steamdeck\artwork"
}
$SteamArtworkBuildDir = Join-Path $BuildDir "steam-artwork"
$ArtworkInstallerSource = Join-Path $RepoRoot "scripts\install-steam-artwork.py"
$ArtworkInstallerBuild = Join-Path $BuildDir "install-steam-artwork.py"
$ShortcutRegistrarSource = Join-Path $RepoRoot "scripts\register-steam-shortcut.py"
$ShortcutRegistrarBuild = Join-Path $BuildDir "register-steam-shortcut.py"
$ShortcutSettingsBuild = Join-Path $BuildDir "steam-shortcut.json"

$RemoteBuildDir = "/home/deck/devkit-game/goro"
$RemoteDataDir  = "/home/deck/goro-data/OldRO"

function Find-DevkitClient {
    $roots = @()

    if (${env:ProgramFiles(x86)}) {
        $roots += Join-Path ${env:ProgramFiles(x86)} "Steam\steamapps\common\SteamOSDevkitClient\windows-client"
    }
    if ($env:ProgramFiles) {
        $roots += Join-Path $env:ProgramFiles "Steam\steamapps\common\SteamOSDevkitClient\windows-client"
    }

    foreach ($candidate in $roots) {
        if (Test-Path (Join-Path $candidate "cygroot\bin\rsync.exe")) {
            return $candidate
        }
    }

    foreach ($regPath in @(
        "HKCU:\Software\Valve\Steam",
        "HKLM:\SOFTWARE\WOW6432Node\Valve\Steam",
        "HKLM:\SOFTWARE\Valve\Steam"
    )) {
        try {
            $item = Get-ItemProperty $regPath -ErrorAction Stop
            $steamPath = if ($item.SteamPath) { $item.SteamPath } else { $item.InstallPath }
            if ($steamPath) {
                $candidate = Join-Path $steamPath "steamapps\common\SteamOSDevkitClient\windows-client"
                if (Test-Path (Join-Path $candidate "cygroot\bin\rsync.exe")) {
                    return $candidate
                }
            }
        } catch {}
    }

    throw "SteamOS Devkit Client not found."
}

function Find-DevkitKey {
    $root = Join-Path $env:LOCALAPPDATA "steamos-devkit"
    if (-not (Test-Path $root)) {
        throw "SteamOS Devkit configuration not found. Pair the Deck first."
    }

    $key = Get-ChildItem $root -Recurse -File -Filter "devkit_rsa" -ErrorAction SilentlyContinue |
        Select-Object -First 1

    if (-not $key) {
        throw "SteamOS Devkit SSH key not found. Re-pair the Deck."
    }

    return $key.FullName
}

function Find-ConfiguredDataSource {
    $configPath = Join-Path $RepoRoot "goro.ini"
    if (-not (Test-Path $configPath)) {
        return $null
    }

    foreach ($line in Get-Content $configPath) {
        if ($line -match '^\s*data_dir\s*=\s*(.+?)\s*$') {
            $value = $Matches[1].Trim()
            if (($value.StartsWith('"') -and $value.EndsWith('"')) -or
                ($value.StartsWith("'") -and $value.EndsWith("'"))) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            if ($value) { return $value }
        }
    }

    return $null
}



function Install-SteamDeckRuntimeAssets {
    New-Item -ItemType Directory -Force $BuildDir | Out-Null

    # dist\steamdeck is intentionally reused between deployments. Remove any
    # SDL runtime left by a previous branch first so a branch that does not use
    # SDL3 cannot accidentally inherit dev's libSDL3.so.0.
    Remove-Item $SDLRuntime -Force -ErrorAction SilentlyContinue
    Remove-Item "$SDLRuntime.tmp" -Force -ErrorAction SilentlyContinue

    # SDL3 is branch-dependent. Newer controller-enabled branches use
    # github.com/Zyko0/go-sdl3, while older/vanilla branches may not depend on
    # SDL3 at all. Only bundle it when the selected branch's module graph has
    # that dependency.
    $moduleDir = ""
    $moduleExit = 1

    Push-Location $BuildSourceRoot
    try {
        $moduleDir = (& go list -m -f '{{.Dir}}' github.com/Zyko0/go-sdl3 2>$null | Out-String).Trim()
        $moduleExit = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }

    if ($moduleExit -eq 0 -and -not [string]::IsNullOrWhiteSpace($moduleDir)) {
        $compressedSDL = Join-Path $moduleDir "bin\binsdl\assets\sdl_amd64.so.gz"
        if (-not (Test-Path $compressedSDL)) {
            throw "Selected branch uses go-sdl3, but its compatible Linux amd64 runtime was not found: $compressedSDL"
        }

        $tempSDL = "$SDLRuntime.tmp"

        $inputStream = $null
        $outputStream = $null
        $gzipStream = $null
        try {
            $inputStream = [System.IO.File]::OpenRead($compressedSDL)
            $outputStream = [System.IO.File]::Create($tempSDL)
            $gzipStream = [System.IO.Compression.GZipStream]::new(
                $inputStream,
                [System.IO.Compression.CompressionMode]::Decompress
            )
            $gzipStream.CopyTo($outputStream)
        }
        finally {
            if ($gzipStream) { $gzipStream.Dispose() }
            if ($outputStream) { $outputStream.Dispose() }
            if ($inputStream) { $inputStream.Dispose() }
        }

        if (-not (Test-Path $tempSDL) -or (Get-Item $tempSDL).Length -le 0) {
            Remove-Item $tempSDL -Force -ErrorAction SilentlyContinue
            throw "SDL3 extraction produced an empty file."
        }

        Move-Item $tempSDL $SDLRuntime -Force
        Write-Host "Bundled SDL3:      $SDLRuntime"
    }
    else {
        Write-Host "Bundled SDL3:      not required by $BuildBranchLabel"
    }

    if (-not (Test-Path $IconSource)) {
        throw "Linux game icon not found: $IconSource"
    }

    Copy-Item $IconSource $BuildIcon -Force
    Write-Host "Bundled game icon: $BuildIcon"
}


function Install-SteamDeckArtworkAssets {
    if ($SkipSteamArtwork) {
        Write-Host "Steam artwork packaging skipped."
        return
    }

    if (-not (Test-Path $SteamArtworkSourceDir)) {
        throw "Steam Deck artwork directory not found: $SteamArtworkSourceDir"
    }

    if (-not (Test-Path $ArtworkInstallerSource)) {
        throw "Steam artwork installer not found: $ArtworkInstallerSource"
    }

    Remove-Item $SteamArtworkBuildDir -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force $SteamArtworkBuildDir | Out-Null

    foreach ($name in @("grid.png", "gridwide.png", "hero.png", "logo.png")) {
        $source = Join-Path $SteamArtworkSourceDir $name
        if (-not (Test-Path $source)) {
            throw "Missing Steam Deck artwork file: $source"
        }
        Copy-Item $source (Join-Path $SteamArtworkBuildDir $name) -Force
    }

    $artIcon = Join-Path $SteamArtworkSourceDir "icon.png"
    if (Test-Path $artIcon) {
        Copy-Item $artIcon (Join-Path $SteamArtworkBuildDir "icon.png") -Force
    }
    elseif (Test-Path $IconSource) {
        # Avoid requiring the same icon to be checked into the repository twice.
        Copy-Item $IconSource (Join-Path $SteamArtworkBuildDir "icon.png") -Force
    }
    else {
        throw "Missing Steam Deck icon.png and Linux fallback icon: $IconSource"
    }

    Copy-Item $ArtworkInstallerSource $ArtworkInstallerBuild -Force
    Write-Host "Bundled Steam art:  $SteamArtworkBuildDir"
    Write-Host "Bundled art helper: $ArtworkInstallerBuild"
}

function Install-SteamShortcutRegistrar {
    if ($SkipSteamShortcut) {
        return
    }

    if (-not (Test-Path $ShortcutRegistrarSource)) {
        throw "Steam shortcut registrar not found: $ShortcutRegistrarSource"
    }

    Copy-Item $ShortcutRegistrarSource $ShortcutRegistrarBuild -Force
    Write-Host "Bundled shortcut helper: $ShortcutRegistrarBuild"
}

function Write-SteamShortcutSettings {
    $startCommand = "goro --data-dir $RemoteDataDir --fullscreen --graphics-api vulkan"
    $settings = [ordered]@{
        gameid = $SteamGameId
        directory = $RemoteBuildDir
        argv = @($startCommand)
        env = @{}
        settings = [ordered]@{
            steam_play = "0"
        }
        force_appid = ""
    }

    $json = $settings | ConvertTo-Json -Depth 8 -Compress
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($ShortcutSettingsBuild, $json, $utf8NoBom)
}

if ($PushData -and [string]::IsNullOrWhiteSpace($DataSource)) {
    $DataSource = Find-ConfiguredDataSource
}

if ($PushData -and [string]::IsNullOrWhiteSpace($DataSource)) {
    throw @"
-PushData requires a source directory.

Use either:
  `$env:GORO_DATA_SOURCE = "<client-data-path>"
or:
  -DataSource "<client-data-path>"

A data_dir entry in goro.ini is also accepted.
"@
}

$DevkitClient = Find-DevkitClient
$CygBin = Join-Path $DevkitClient "cygroot\bin"
$Rsync = Join-Path $CygBin "rsync.exe"
$Ssh = Join-Path $CygBin "ssh.exe"
$Cygpath = Join-Path $CygBin "cygpath.exe"
$Key = Find-DevkitKey

$env:PATH = "$CygBin;$env:PATH"

function To-CygPath([string]$Path) {
    $resolved = (Resolve-Path $Path).Path
    $result = & $Cygpath -u $resolved
    if ($LASTEXITCODE -ne 0) {
        throw "cygpath failed for: $resolved"
    }
    return ($result | Out-String).Trim()
}

$KeyCyg = To-CygPath $Key

$SshArgs = @(
    "-o", "StrictHostKeyChecking=no",
    "-o", "UserKnownHostsFile=/dev/null",
    "-o", "GlobalKnownHostsFile=/dev/null",
    "-o", "IdentitiesOnly=yes",
    "-o", "ServerAliveInterval=15",
    "-o", "ServerAliveCountMax=12",
    "-o", "TCPKeepAlive=yes",
    "-o", "LogLevel=ERROR",
    "-i", $Key
)

$RsyncSsh = "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o GlobalKnownHostsFile=/dev/null -o IdentitiesOnly=yes -o ServerAliveInterval=15 -o ServerAliveCountMax=12 -o TCPKeepAlive=yes -o LogLevel=ERROR -i '$KeyCyg'"

function Invoke-DeckSsh([string]$Command) {
    & $Ssh @SshArgs "deck@$DeckHost" $Command
    if ($LASTEXITCODE -ne 0) {
        throw "SSH command failed with exit code $LASTEXITCODE"
    }
}

Write-Host "Steam Deck host: $DeckHost"
Write-Host "Repository:      $RepoRoot"
Write-Host "Build source:    $BuildBranchLabel"
if ($BuildCommit) {
    Write-Host "Build commit:    $BuildCommit"
}

Invoke-DeckSsh "printf 'Steam Deck SSH OK\n'"

if (-not $NoBuild) {
    New-Item -ItemType Directory -Force $BuildDir | Out-Null

    Write-Host "`nBuilding Linux amd64 Goro from $BuildBranchLabel..."
    Push-Location $BuildSourceRoot
    try {
        $saved = @{
            GOOS = $env:GOOS
            GOARCH = $env:GOARCH
            CGO_ENABLED = $env:CGO_ENABLED
            GOFLAGS = $env:GOFLAGS
        }

        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "0"
        $env:GOFLAGS = "-tags=nofakecgo"

        & go build -trimpath -o $BuildExe .
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        $env:GOOS = $saved.GOOS
        $env:GOARCH = $saved.GOARCH
        $env:CGO_ENABLED = $saved.CGO_ENABLED
        $env:GOFLAGS = $saved.GOFLAGS
        Pop-Location
    }
}

if (-not (Test-Path $BuildExe)) {
    throw "Linux build not found: $BuildExe"
}

Write-Host "`nPreparing Steam Deck runtime assets..."
Install-SteamDeckRuntimeAssets
Install-SteamDeckArtworkAssets
Install-SteamShortcutRegistrar
if (-not (Test-Path $LauncherSource)) {
    throw "SteamDeck launcher source not found: $LauncherSource"
}
Copy-Item $LauncherSource $LauncherBuild -Force
Write-Host "Bundled launcher:   $LauncherBuild"
Write-SteamShortcutSettings

# Leave a small provenance marker in the deployed package so it is easy to
# identify which branch/commit is currently installed on the Deck.
$buildInfo = @(
    "source=$BuildBranchLabel"
    "commit=$BuildCommit"
) -join "`n"
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $BuildDir "steamdeck-build.txt"), $buildInfo + "`n", $utf8NoBom)

if ($BranchWorktree) {
    Write-Host "Removing temporary branch worktree..."
    & git -C $RepoRoot worktree remove --force $BranchWorktree
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "Could not remove temporary worktree: $BranchWorktree"
    }
    $BranchWorktree = $null
}

Write-Host "`nUploading client build..."
Invoke-DeckSsh "mkdir -p '$RemoteBuildDir'"

$BuildDirCyg = (To-CygPath $BuildDir).TrimEnd("/") + "/"

& $Rsync -az --delete --chmod=Du=rwx,Dgo=rx,Fu=rwx,Fgo=rx `
    -e $RsyncSsh `
    $BuildDirCyg "deck@${DeckHost}:${RemoteBuildDir}/"

if ($LASTEXITCODE -ne 0) {
    throw "Client rsync failed with exit code $LASTEXITCODE"
}

Invoke-DeckSsh "test -x '$RemoteBuildDir/run-steamdeck.sh'"

Write-Host "Client uploaded."

if ($PushData) {
    if (-not (Test-Path $DataSource)) {
        throw "Client-data source does not exist: $DataSource"
    }

    if ($ResetData) {
        Write-Host "`nResetting remote client-data directory..."
        Invoke-DeckSsh "rm -rf '$RemoteDataDir' && mkdir -p '$RemoteDataDir'"
    } else {
        Invoke-DeckSsh "mkdir -p '$RemoteDataDir'"
    }

    Write-Host "`nUploading client data..."
    Write-Host "The first transfer may take a while; interrupted runs can be resumed."

    $DataSourceCyg = (To-CygPath $DataSource).TrimEnd("/") + "/"

    & $Rsync -a --partial --info=progress2 `
        -e $RsyncSsh `
        $DataSourceCyg "deck@${DeckHost}:${RemoteDataDir}/"

    if ($LASTEXITCODE -ne 0) {
        throw "Client-data rsync failed with exit code $LASTEXITCODE"
    }

    Write-Host "Client data uploaded."
}

if (-not $SkipSteamShortcut) {
    Write-Host "`nCreating/updating Steam devkit shortcut..."
    Invoke-DeckSsh "test -f ~/devkit-utils/steam-client-create-shortcut"
    Invoke-DeckSsh "test -f '$RemoteBuildDir/register-steam-shortcut.py'"
    Invoke-DeckSsh "test -f '$RemoteBuildDir/steam-shortcut.json'"

    # Do not pass the JSON through PowerShell -> SSH -> remote shell quoting.
    # The helper reads the uploaded JSON file and passes it to argparse as one
    # exact argv element, so spaces inside the game's launch command are safe.
    $registerCommand = "python3 '$RemoteBuildDir/register-steam-shortcut.py' --parms-file '$RemoteBuildDir/steam-shortcut.json'"
    Invoke-DeckSsh $registerCommand
    Write-Host "Steam shortcut updated: $SteamGameId"
}

if (-not $SkipSteamArtwork) {
    Write-Host "`nApplying Steam library artwork..."
    $artworkCommand = "python3 '$RemoteBuildDir/install-steam-artwork.py' --gameid '$SteamGameId' --exe '$RemoteBuildDir/goro' --art-dir '$RemoteBuildDir/steam-artwork'"
    Invoke-DeckSsh $artworkCommand
    Write-Host "Steam library artwork applied."
}

Write-Host "`nDone."
Write-Host "Remote client: $RemoteBuildDir/goro"
Write-Host "Remote launcher: $RemoteBuildDir/run-steamdeck.sh"
if (Test-Path $SDLRuntime) {
    Write-Host "Remote SDL3:   $RemoteBuildDir/libSDL3.so.0"
} else {
    Write-Host "Remote SDL3:   not required by selected branch"
}
Write-Host "Remote icon:   $RemoteBuildDir/goro.png"
if (-not $SkipSteamArtwork) {
    Write-Host "Steam artwork: $RemoteBuildDir/steam-artwork"
}
Write-Host "Remote data:   $RemoteDataDir"
Write-Host "Deployed from: $BuildBranchLabel @ $BuildCommit"
