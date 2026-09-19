[CmdletBinding()]
param(
    [switch]$Staticcheck
)

# Post-merge verification for the controller slice. This is intentionally a
# standalone report, not a CI merge gate: callers can run it after resolving a
# merge and decide whether to continue with unrelated work.
$ErrorActionPreference = "Continue"
$repo = (Resolve-Path (Join-Path $PSScriptRoot ".."))
$failed = $false

Push-Location $repo
try {
    $checks = @(
        @{ Name = "build"; Command = { go build ./... } },
        @{ Name = "controller tests"; Command = { go test ./input/... ./input/gamepad/... -count=1 } },
        @{ Name = "render and UI input tests"; Command = { go test ./render ./ui -run 'Controller|Fanout|WireInput|LoginWindow|CharacterDragKeepsAttachedBasicMenu' -count=1 } }
    )
    if ($Staticcheck) {
        $checks += @{ Name = "staticcheck"; Command = { staticcheck ./input/... ./render/... ./game/... ./ui/... } }
    }

    foreach ($check in $checks) {
        Write-Host "`n== $($check.Name) =="
        & $check.Command
        if ($LASTEXITCODE -ne 0) {
            Write-Host "$($check.Name) failed (report only)." -ForegroundColor Yellow
            $failed = $true
        }
    }
}
finally {
    Pop-Location
}

if ($failed) {
    Write-Host "`nController verification reported failures; merge synchronization was not modified." -ForegroundColor Yellow
    exit 1
}
Write-Host "`nController verification passed."
