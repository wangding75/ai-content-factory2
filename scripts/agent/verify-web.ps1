param(
    [Parameter(Mandatory)][string]$TaskId,
    [string[]]$TargetTest = @(),
    [string[]]$LintPath = @(),
    [switch]$SkipFullTest,
    [switch]$SkipBuild
)

. "$PSScriptRoot\common.ps1"

$started = Get-Date
$root = Get-RepositoryRoot
Set-Location $root

Assert-Command -Name "pnpm.cmd"

$steps = [ordered]@{}
$result = "failed"

try {
    foreach ($test in $TargetTest) {
        Invoke-Native -FilePath "pnpm.cmd" `
            -Arguments @("--dir", "apps/web", "exec", "node", "--test", $test) `
            -Label "Targeted test: $test"
    }
    $steps.targeted_tests = if ($TargetTest.Count -gt 0) { "passed" } else { "not_requested" }

    if (-not $SkipFullTest) {
        Invoke-Native -FilePath "pnpm.cmd" `
            -Arguments @("test:web") `
            -Label "Web tests"
        $steps.web_tests = "passed"
    }
    else {
        $steps.web_tests = "skipped"
    }

    Invoke-Native -FilePath "pnpm.cmd" `
        -Arguments @("typecheck:web") `
        -Label "Typecheck"
    $steps.typecheck = "passed"

    if ($LintPath.Count -gt 0) {
        $args = @("--dir", "apps/web", "exec", "eslint") + $LintPath
        Invoke-Native -FilePath "pnpm.cmd" `
            -Arguments $args `
            -Label "Targeted lint"
        $steps.targeted_lint = "passed"
    }
    else {
        $steps.targeted_lint = "not_requested"
    }

    if (-not $SkipBuild) {
        Invoke-Native -FilePath "pnpm.cmd" `
            -Arguments @("build:web") `
            -Label "Production build"
        $steps.production_build = "passed"
    }
    else {
        $steps.production_build = "skipped"
    }

    Invoke-Native -FilePath "git" `
        -Arguments @("diff", "--check") `
        -Label "git diff --check"
    $steps.git_diff_check = "passed"

    $result = "passed"
}
finally {
    $finished = Get-Date
    $report = [ordered]@{
        task = $TaskId
        stage = "verify-web"
        result = $result
        steps = $steps
        targeted_tests = $TargetTest
        lint_paths = $LintPath
        started_at = $started.ToString("o")
        finished_at = $finished.ToString("o")
        duration_seconds = [math]::Round(($finished - $started).TotalSeconds, 2)
    }
    $reportPath = Join-Path $root ".ai-dev\reports\$TaskId-web.json"
    Write-JsonUtf8NoBom -Path $reportPath -Value $report
}

Write-Host ""
Write-Host "[PASS] Web verification completed." -ForegroundColor Green
Write-Host "Report: $reportPath"
