[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Step {
    param([Parameter(Mandatory = $true)][string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

$repoRoot = (& git rev-parse --show-toplevel 2>$null)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($repoRoot)) {
    throw 'Current directory is not inside a Git repository.'
}
$repoRoot = [System.IO.Path]::GetFullPath($repoRoot.Trim())

$databaseRoot = Join-Path $repoRoot 'database'
$requiredFiles = @(
    (Join-Path $databaseRoot 'Initialize-Database.ps1'),
    (Join-Path $databaseRoot 'Consolidate-Database-Scripts.ps1'),
    (Join-Path $databaseRoot 'Test-Database-Management-Package.ps1'),
    (Join-Path $databaseRoot 'testdata\complete-test-data.sql'),
    (Join-Path $databaseRoot '.gitignore')
)

Write-Step 'Check required files'
foreach ($file in $requiredFiles) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
        throw "Required file is missing: $file"
    }
}

Write-Step 'Parse PowerShell scripts'
$powerShellFiles = @(
    (Join-Path $databaseRoot 'Initialize-Database.ps1'),
    (Join-Path $databaseRoot 'Consolidate-Database-Scripts.ps1'),
    (Join-Path $databaseRoot 'Test-Database-Management-Package.ps1')
)

foreach ($file in $powerShellFiles) {
    $tokens = $null
    $errors = $null

    [void][System.Management.Automation.Language.Parser]::ParseFile(
        $file,
        [ref]$tokens,
        [ref]$errors
    )

    if ($errors.Count -gt 0) {
        $errors | Format-List
        throw "PowerShell syntax validation failed: $file"
    }
}

Write-Step 'Validate initializer migration tracking'
$initializerText = [System.IO.File]::ReadAllText(
    (Join-Path $databaseRoot 'Initialize-Database.ps1')
)

if ($initializerText -match '(?i)\bdirty\b') {
    throw 'Initializer incorrectly assumes schema_migrations.dirty.'
}

if ($initializerText -notmatch 'string_agg\(version::text') {
    throw 'Initializer does not compare the applied Migration version set.'
}

Write-Step 'Validate consolidation safety policy'
$consolidationText = [System.IO.File]::ReadAllText(
    (Join-Path $databaseRoot 'Consolidate-Database-Scripts.ps1')
)

$requiredMarkers = @(
    'Update path references after moves',
    'database/tools/Validate-Migration-History.ps1',
    'database/tools/Validate-Database-Consistency.ps1',
    'database/Consolidate-Database-Scripts.ps1',
    'database/Test-Database-Management-Package.ps1',
    'database/README.md',
    'canonical Migration history changed unexpectedly',
    'stale script path references remain'
)

foreach ($marker in $requiredMarkers) {
    if (-not $consolidationText.Contains($marker)) {
        throw "Required consolidation marker is missing: $marker"
    }
}

$forbiddenMarkers = @(
    'DeleteUnknownMaintenance',
    "Action = 'delete'",
    'Test-DatabaseScript'
)

foreach ($marker in $forbiddenMarkers) {
    if ($consolidationText.Contains($marker)) {
        throw "Unsafe consolidation marker is present: $marker"
    }
}

Write-Step 'Validate fixture SQL'
$seedSql = [System.IO.File]::ReadAllText(
    (Join-Path $databaseRoot 'testdata\complete-test-data.sql')
)

$beginCount = [regex]::Matches(
    $seedSql,
    '(?im)^\s*BEGIN;\s*$'
).Count
$commitCount = [regex]::Matches(
    $seedSql,
    '(?im)^\s*COMMIT;\s*$'
).Count

if ($beginCount -ne 1 -or $commitCount -ne 1) {
    throw 'Fixture SQL transaction boundary is invalid.'
}

Write-Step 'Check required commands'
foreach ($command in @('git', 'psql', 'pg_dump')) {
    if ($null -eq (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "Required command is unavailable: $command"
    }
}

Write-Host ""
Write-Host 'PASS'
