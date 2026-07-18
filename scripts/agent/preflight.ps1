param(
    [Parameter(Mandatory)][string]$TaskId,
    [string]$ExpectedBranch = "main",
    [string]$ExpectedHead = "",
    [switch]$AllowDirty,
    [string[]]$AllowedDirtyPath = @()
)

. "$PSScriptRoot\common.ps1"

$started = Get-Date
$root = Get-RepositoryRoot
Set-Location $root

Assert-Command -Name "git"

$requiredDocs = @(
    "docs\agent\00-project.md",
    "docs\agent\01-development-standard.md",
    "docs\agent\02-review-standard.md"
)

# The toolkit may be landing for the first time, so its own standards can be
# untracked but must still be present and explicitly allowed as dirty paths.
$missing = @($requiredDocs | Where-Object {
    -not (Test-Path -LiteralPath (Join-Path $root $_) -PathType Leaf)
})
if ($missing.Count -gt 0) {
    throw "Missing required Agent standards: $($missing -join ', ')"
}

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0) { throw "Unable to read current branch." }
if ($ExpectedBranch -and $branch -ne $ExpectedBranch) {
    throw "Unexpected branch. Expected '$ExpectedBranch', actual '$branch'."
}

$head = (& git rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw "Unable to read HEAD." }
if ($ExpectedHead -and $head -ne $ExpectedHead) {
    throw "Unexpected HEAD. Expected '$ExpectedHead', actual '$head'."
}

$statusPaths = @(Get-GitStatusPaths)
$unexpected = @()

if ($statusPaths.Count -gt 0) {
    if (-not $AllowDirty) {
        $unexpected = $statusPaths
    }
    elseif ($AllowedDirtyPath.Count -gt 0) {
        $unexpected = @($statusPaths | Where-Object {
            -not (Test-PathAllowed -Path $_ -AllowedPatterns $AllowedDirtyPath)
        })
    }
}

if ($unexpected.Count -gt 0) {
    throw "Unexpected working-tree changes: $($unexpected -join ', ')"
}

$finished = Get-Date
$report = [ordered]@{
    task = $TaskId
    stage = "preflight"
    result = "passed"
    branch = $branch
    head = $head
    dirty_paths = $statusPaths
    started_at = $started.ToString("o")
    finished_at = $finished.ToString("o")
    duration_seconds = [math]::Round(($finished - $started).TotalSeconds, 2)
}

$reportPath = Join-Path $root ".ai-dev\reports\$TaskId-preflight.json"
Write-JsonUtf8NoBom -Path $reportPath -Value $report

Write-Host ""
Write-Host "[PASS] Preflight completed." -ForegroundColor Green
Write-Host "Branch: $branch"
Write-Host "HEAD:   $head"
Write-Host "Report: $reportPath"
