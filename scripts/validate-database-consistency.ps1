# Validate Database Consistency
# Validates migration history, database state, and schema structure
# PowerShell 5.1 compatible

$ErrorActionPreference = 'Stop'

# Locate repo root
$repoRoot = git rev-parse --show-toplevel
if ($LASTEXITCODE -ne 0) {
    Write-Error "FAIL: not a Git repository"
    exit 1
}

# Check DATABASE_URL
if (-not $env:DATABASE_URL) {
    Write-Error "FAIL: DATABASE_URL is not set"
    exit 1
}

Write-Output "Repository: $repoRoot"
Write-Output ""

# Step 1: Run migration history validation
Write-Output "=== Migration History Integrity ==="
$historyScript = Join-Path $repoRoot 'scripts\validate-migration-history.ps1'
$historyResult = & powershell -ExecutionPolicy Bypass -File $historyScript 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Output $historyResult
    Write-Error "FAIL: migration history integrity check failed"
    exit 1
}
Write-Output $historyResult
Write-Output ""

# Step 2: Run dbcheck
Write-Output "=== Database Schema Consistency ==="
$apiDir = Join-Path $repoRoot 'apps\api'
Push-Location $apiDir
try {
    $dbcheckResult = & go run ./cmd/dbcheck 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Output $dbcheckResult
        Write-Error "FAIL: database schema consistency check failed"
        Pop-Location
        exit 1
    }
    Write-Output $dbcheckResult
} finally {
    Pop-Location
}

Write-Output ""
Write-Output "PASS"