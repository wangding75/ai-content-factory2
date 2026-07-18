param(
    [Parameter(Mandatory)][string]$TaskId,
    [Parameter(Mandatory)][string]$CommitMessage,
    [string]$ExpectedBranch = "main",
    [string[]]$IncludePath = @(),
    [switch]$StageAll,
    [switch]$Push
)

. "$PSScriptRoot\common.ps1"

$started = Get-Date
$root = Get-RepositoryRoot
Set-Location $root

$branch = (& git branch --show-current).Trim()
if ($branch -ne $ExpectedBranch) {
    throw "Unexpected branch. Expected '$ExpectedBranch', actual '$branch'."
}

Invoke-Native -FilePath "git" -Arguments @("diff", "--check") -Label "git diff --check"
Invoke-Native -FilePath "git" -Arguments @("diff", "--name-status") -Label "Changed files"
Invoke-Native -FilePath "git" -Arguments @("diff", "--stat") -Label "Diff stat"
Invoke-Native -FilePath "git" -Arguments @("status", "--short", "--untracked-files=all") -Label "Working tree"

if ($StageAll) {
    Invoke-Native -FilePath "git" -Arguments @("add", "--all") -Label "Stage all"
}
elseif ($IncludePath.Count -gt 0) {
    Invoke-Native -FilePath "git" -Arguments (@("add", "--") + $IncludePath) -Label "Stage approved paths"
}
else {
    throw "Refusing to stage implicitly. Supply -IncludePath or -StageAll."
}

Invoke-Native -FilePath "git" -Arguments @("diff", "--cached", "--name-status") -Label "Staged files"
Invoke-Native -FilePath "git" -Arguments @("diff", "--cached", "--stat") -Label "Staged stat"

& git diff --cached --quiet
if ($LASTEXITCODE -eq 0) {
    throw "No staged changes to commit."
}

Invoke-Native -FilePath "git" -Arguments @("commit", "-m", $CommitMessage) -Label "Commit"

$commit = (& git rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $commit.Length -ne 40) {
    throw "Unable to resolve full commit hash."
}

if ($Push) {
    Invoke-Native -FilePath "git" -Arguments @("push", "origin", $ExpectedBranch) -Label "Push"
}

$status = @(& git status --short --untracked-files=all)
if ($LASTEXITCODE -ne 0) {
    throw "Unable to read final working-tree status."
}
if ($status.Count -gt 0) {
    throw "Commit completed but working tree is not clean: $($status -join '; ')"
}

$finished = Get-Date
$report = [ordered]@{
    task = $TaskId
    stage = "finalize"
    result = "passed"
    branch = $branch
    commit = $commit
    commit_message = $CommitMessage
    pushed = [bool]$Push
    workspace_clean = $true
    started_at = $started.ToString("o")
    finished_at = $finished.ToString("o")
    duration_seconds = [math]::Round(($finished - $started).TotalSeconds, 2)
}

$reportPath = Join-Path $root ".ai-dev\reports\$TaskId-final.json"
Write-JsonUtf8NoBom -Path $reportPath -Value $report

Write-Host ""
Write-Host "[PASS] Commit finalized." -ForegroundColor Green
Write-Host "Commit: $commit"
Write-Host "Push:   $([bool]$Push)"
Write-Host "Report: $reportPath"
