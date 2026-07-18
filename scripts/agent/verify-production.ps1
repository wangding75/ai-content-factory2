param(
    [Parameter(Mandatory)][string]$TaskId,
    [string]$WebUrl = "http://127.0.0.1:13001",
    [string]$ApiHealthUrl = "http://127.0.0.1:18080/api/v1/meta",
    [int]$MaxBuildAttempts = 2,
    [bool]$ReuseHealthy = $true
)

. "$PSScriptRoot\common.ps1"

$started = Get-Date
$root = Get-RepositoryRoot
Set-Location $root

Assert-Command -Name "docker"

if (-not (Wait-DockerStable -TimeoutSec 90)) {
    throw "Docker Engine did not become stable within 90 seconds."
}

$reused = $false
$attempts = 0
$result = "failed"

function Test-ProductionReady {
    $services = @(& docker compose ps -a --format json 2>$null | ConvertFrom-Json)
    if ($LASTEXITCODE -ne 0 -or $services.Count -eq 0) {
        return $false
    }

    $requiredServices = @("postgres", "migrate", "api", "web")
    foreach ($serviceName in $requiredServices) {
        $service = @($services | Where-Object { $_.Service -eq $serviceName }) | Select-Object -First 1
        if (-not $service) {
            return $false
        }

        if ($serviceName -eq "migrate") {
            if ($service.State -ne "exited" -or $service.ExitCode -ne "0") {
                return $false
            }
        }
        elseif ($service.State -ne "running") {
            return $false
        }
        elseif ($serviceName -in @("postgres", "api") -and $service.Health -ne "healthy") {
            return $false
        }
    }

    if (-not (Test-HttpOk -Url $WebUrl -TimeoutSec 5)) {
        return $false
    }
    if ($ApiHealthUrl -and -not (Test-HttpOk -Url $ApiHealthUrl -TimeoutSec 5)) {
        return $false
    }
    return $true
}

try {
    if ($ReuseHealthy -and (Test-ProductionReady)) {
        Write-Host "[INFO] Existing production services are reachable; rebuild skipped." -ForegroundColor Yellow
        $reused = $true
    }
    else {
        for ($attempt = 1; $attempt -le $MaxBuildAttempts; $attempt++) {
            $attempts = $attempt
            Write-Host ""
            Write-Host "== Docker production attempt $attempt/$MaxBuildAttempts ==" -ForegroundColor Cyan

            & docker compose up -d --build --remove-orphans
            if ($LASTEXITCODE -eq 0) {
                break
            }

            if ($attempt -ge $MaxBuildAttempts) {
                throw "Docker Compose failed after $MaxBuildAttempts attempts."
            }

            Write-Host "[WARN] Compose failed. Waiting for Docker Engine stability before one retry." -ForegroundColor Yellow
            if (-not (Wait-DockerStable -TimeoutSec 90)) {
                throw "Docker Engine did not recover after Compose failure."
            }
        }
    }

    Invoke-Native -FilePath "docker" `
        -Arguments @("compose", "ps", "-a") `
        -Label "Docker Compose status"

    $deadline = (Get-Date).AddSeconds(120)
    while ((Get-Date) -lt $deadline -and -not (Test-ProductionReady)) {
        Start-Sleep -Seconds 3
    }

    if (-not (Test-ProductionReady)) {
        & docker compose logs --since=15m migrate api web
        throw "Production services did not become reachable."
    }

    Invoke-Native -FilePath "docker" `
        -Arguments @("compose", "logs", "--since=10m", "migrate", "api", "web") `
        -Label "Recent service logs"

    $result = "passed"
}
finally {
    $finished = Get-Date
    $report = [ordered]@{
        task = $TaskId
        stage = "verify-production"
        result = $result
        reused_existing_services = $reused
        build_attempts = $attempts
        web_url = $WebUrl
        api_health_url = $ApiHealthUrl
        started_at = $started.ToString("o")
        finished_at = $finished.ToString("o")
        duration_seconds = [math]::Round(($finished - $started).TotalSeconds, 2)
    }
    $reportPath = Join-Path $root ".ai-dev\reports\$TaskId-production.json"
    Write-JsonUtf8NoBom -Path $reportPath -Value $report
}

Write-Host ""
Write-Host "[PASS] Production verification completed." -ForegroundColor Green
Write-Host "Report: $reportPath"
