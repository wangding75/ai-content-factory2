param(
    [Parameter(Mandatory)][string]$TaskId,
    [Parameter(Mandatory)][string[]]$Route,
    [string]$BaseUrl = "http://127.0.0.1:13001",
    [string[]]$ForbiddenPattern = @(),
    [string[]]$AllowedFailurePattern = @(
        "favicon\.ico",
        "net::ERR_ABORTED"
    ),
    [int]$Width = 1440,
    [int]$Height = 1000
)

. "$PSScriptRoot\common.ps1"

$started = Get-Date
$root = Get-RepositoryRoot
Set-Location $root

Assert-Command -Name "node"

foreach ($item in $Route) {
    $url = [Uri]::new([Uri]$BaseUrl, $item).AbsoluteUri
    if (-not (Test-HttpOk -Url $url -TimeoutSec 10)) {
        throw "Route is not reachable: $url"
    }
}

$reportDir = Join-Path $root ".ai-dev\reports"
Ensure-Directory -Path $reportDir

$configPath = Join-Path $reportDir "$TaskId-browser-config.json"
$outputPath = Join-Path $reportDir "$TaskId-browser.json"

$config = [ordered]@{
    baseUrl = $BaseUrl
    routes = $Route
    forbiddenPatterns = $ForbiddenPattern
    allowedFailurePatterns = $AllowedFailurePattern
    viewport = [ordered]@{
        width = $Width
        height = $Height
    }
    timeoutMs = 45000
    settleMs = 500
    outputPath = $outputPath
    playwrightPackageJson = (Join-Path $root "apps\web\package.json")
}

Write-JsonUtf8NoBom -Path $configPath -Value $config

Invoke-Native -FilePath "node" `
    -Arguments @(
        "scripts/agent/browser-smoke.mjs",
        "--config",
        $configPath
    ) `
    -Label "Chromium route smoke"

$finished = Get-Date
Write-Host ""
Write-Host "[PASS] Browser route verification completed." -ForegroundColor Green
Write-Host "Report: $outputPath"
Write-Host "Duration: $([math]::Round(($finished - $started).TotalSeconds, 2))s"
