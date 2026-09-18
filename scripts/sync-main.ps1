[CmdletBinding()]
param(
    [string]$Remote = "origin",
    [string]$Branch = "main"
)

$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot ".."))

Push-Location $repo
try {
    $dirty = @(git status --porcelain)
    if ($dirty.Count -gt 0) {
        throw "Working tree is dirty. Commit or stash the controller slice before syncing $Remote/$Branch.`n$($dirty -join "`n")"
    }

    $current = (git branch --show-current).Trim()
    if ([string]::IsNullOrWhiteSpace($current)) {
        throw "Detached HEAD cannot be synchronized with $Remote/$Branch."
    }

    Write-Host "Fetching $Remote/$Branch..."
    git fetch $Remote $Branch
    if ($LASTEXITCODE -ne 0) { throw "git fetch failed with exit code $LASTEXITCODE" }

    Write-Host "Merging $Remote/$Branch into $current..."
    git merge --no-edit "$Remote/$Branch"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Merge requires manual reconciliation. Conflicting paths:" -ForegroundColor Yellow
        git diff --name-only --diff-filter=U
        exit 1
    }

    Write-Host "Synchronized $current with $Remote/$Branch."
}
finally {
    Pop-Location
}
