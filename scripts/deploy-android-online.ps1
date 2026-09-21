[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ClientRoot,
    [int]$ServerIndex = 0,
    [string]$DeviceSerial = '',
    [ValidateSet('mobileui','desktop')][string]$UiMode = 'mobileui',
    [switch]$NoBuild,
    [switch]$NoLaunch
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$client = (Resolve-Path $ClientRoot).Path
$apk = Join-Path $repo 'android\host\app\build\outputs\apk\debug\app-debug.apk'
$package = 'com.kivutar.goro.host'
$remoteRoot = "/sdcard/Android/data/$package/files/goro-data"
$adb = if ($env:ANDROID_HOME) { Join-Path $env:ANDROID_HOME 'platform-tools\adb.exe' } else { 'adb.exe' }

if (!(Test-Path -LiteralPath $client -PathType Container)) { throw "ClientRoot is not a directory: $client" }
if (!(Get-Command $adb -ErrorAction SilentlyContinue) -and !(Test-Path -LiteralPath $adb)) { throw "adb was not found. Set ANDROID_HOME or add platform-tools to PATH." }

$clientInfoCandidates = @(
    (Join-Path $client 'data\clientinfo.xml'),
    (Join-Path $client 'data\sclientinfo.xml'),
    (Join-Path $client 'clientinfo.xml'),
    (Join-Path $client 'sclientinfo.xml'),
    (Join-Path $client 'System\clientinfo.xml'),
    (Join-Path $client 'System\sclientinfo.xml')
)
$clientInfoPath = $clientInfoCandidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
if (!$clientInfoPath) { throw "No clientinfo.xml found below $client" }

[xml]$clientInfo = Get-Content -LiteralPath $clientInfoPath -Raw
$connections = @($clientInfo.clientinfo.connection)
if ($connections.Count -eq 0 -or $ServerIndex -lt 0 -or $ServerIndex -ge $connections.Count) { throw "ServerIndex $ServerIndex is outside the clientinfo connection list (count=$($connections.Count))" }
$connection = $connections[$ServerIndex]
$hostName = ([string]$connection.address).Trim()
$authPort = [int]([string]$connection.port).Trim()
$serverName = ([string]$connection.display).Trim()
if (!$hostName -or !$authPort) { throw "Selected clientinfo connection is missing address or port" }
if (!$serverName) { $serverName = $hostName }

$archives = @('data.grf', 'rdata.grf', 'fdata.grf', 'event.grf') | ForEach-Object { Join-Path $client $_ } | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf }
if ($archives.Count -eq 0) { throw "No GRF archives found in $client" }

if (!$NoBuild) {
    & powershell -ExecutionPolicy Bypass -File (Join-Path $repo 'scripts\build-android-test.ps1') -NoLaunch
    if ($LASTEXITCODE -ne 0) { throw "Android APK build failed with exit code $LASTEXITCODE" }
}
if (!(Test-Path -LiteralPath $apk -PathType Leaf)) { throw "APK not found: $apk" }

$adbArgs = @()
if ($DeviceSerial) { $adbArgs += @('-s', $DeviceSerial) }
function Invoke-Adb([string[]]$Arguments) {
    & $adb @adbArgs @Arguments
    if ($LASTEXITCODE -ne 0) { throw "adb command failed with exit code $LASTEXITCODE" }
}

Invoke-Adb @('install', '-r', $apk)
Invoke-Adb @('shell', 'mkdir', '-p', "$remoteRoot/data", "$remoteRoot/goro")
foreach ($archive in $archives) {
    Write-Host "Pushing archive: $archive"
    Invoke-Adb @('push', $archive, $remoteRoot)
}
Write-Host "Pushing clientinfo: $clientInfoPath"
Invoke-Adb @('push', $clientInfoPath, "$remoteRoot/data/clientinfo.xml")

$config = @"
[mobile]
mode = online
presentation = $UiMode
"@
$configFile = Join-Path ([System.IO.Path]::GetTempPath()) 'goro-android-online.ini'
[System.IO.File]::WriteAllText($configFile, $config, [System.Text.UTF8Encoding]::new($false))
try {
    Write-Host "Pushing online config: $serverName ($hostName`:$authPort)"
    Invoke-Adb @('push', $configFile, "$remoteRoot/goro/goro.ini")
} finally {
    Remove-Item -LiteralPath $configFile -Force -ErrorAction SilentlyContinue
}

Write-Host "Android online resources deployed to $remoteRoot"
if (!$NoLaunch) {
    Invoke-Adb @('shell', 'am', 'force-stop', $package)
    Invoke-Adb @('shell', 'monkey', '-p', $package, '1')
    Write-Host "Logs: adb $($adbArgs -join ' ') logcat -s GoroAndroidHost GoroAndroidGo"
}
