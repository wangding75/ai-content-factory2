[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [switch]$Apply
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Step {
    param([Parameter(Mandatory = $true)][string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

function Get-RepositoryRoot {
    $root = (& git rev-parse --show-toplevel 2>$null)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($root)) {
        throw 'BLOCKED: current directory is not inside a Git repository.'
    }

    return [System.IO.Path]::GetFullPath($root.Trim())
}

function Normalize-GitPath {
    param([Parameter(Mandatory = $true)][string]$Path)
    return $Path.Replace('\', '/').TrimStart('/')
}

function Get-TrackedFiles {
    $files = @(& git ls-files)
    if ($LASTEXITCODE -ne 0) {
        throw 'BLOCKED: git ls-files failed.'
    }

    return @(
        $files |
            ForEach-Object { Normalize-GitPath $_ }
    )
}

function Test-TextFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    $extensions = @(
        '.md', '.txt', '.ps1', '.psm1', '.sh', '.sql',
        '.go', '.py', '.json', '.yaml', '.yml', '.toml',
        '.env', '.example', '.xml'
    )

    return $extensions -contains (
        [System.IO.Path]::GetExtension($Path).ToLowerInvariant()
    )
}

$repoRoot = Get-RepositoryRoot
$databaseRoot = Join-Path $repoRoot 'database'
$reportsRoot = Join-Path $databaseRoot 'reports'
New-Item -ItemType Directory -Force -Path $reportsRoot | Out-Null

$approvedMoves = [ordered]@{
    'scripts/validate-migration-history.ps1' =
        'database/tools/Validate-Migration-History.ps1'

    'scripts/validate-database-consistency.ps1' =
        'database/tools/Validate-Database-Consistency.ps1'

    'scripts/test-data/09-f6e-unified-p0.ps1' =
        'database/legacy/test-data/09-f6e-unified-p0.ps1'

    'scripts/test-data/09-f6e-unified-p0-clean.sql' =
        'database/legacy/test-data/09-f6e-unified-p0-clean.sql'

    'scripts/test-data/09-f6e-unified-p0-load.sql' =
        'database/legacy/test-data/09-f6e-unified-p0-load.sql'

    'scripts/test-data/09-f6e-unified-p0-verify.sql' =
        'database/legacy/test-data/09-f6e-unified-p0-verify.sql'

    'scripts/validate-iteration16-data-foundation.ps1' =
        'database/legacy/iteration-validations/Validate-Iteration16-Data-Foundation.ps1'

    'scripts/verify-projects-migration.ps1' =
        'database/legacy/migration-validations/Verify-Projects-Migration.ps1'
}

$protectedPaths = @(
    'export-iteration-review-bundle.ps1',
    'init-project-scaffold-windows.ps1',
    'scripts/agent/verify-production.ps1',
    'scripts/browser-acceptance/build-report.py',
    'scripts/qa/iteration06/assert-db.ps1',
    'scripts/qa/iteration06/reset-fixtures.ps1',
    'scripts/qa/iteration06/start.ps1',
    'scripts/qa/iteration06/status.ps1',
    'scripts/qa/iteration06/stop.ps1',
    'scripts/start-infrastructure.ps1',
    'scripts/validate-iteration17-contract.ps1',
    'scripts/validate-iteration18-contract.ps1'
)

$referenceScanExclusions = @(
    'database/Consolidate-Database-Scripts.ps1',
    'database/Test-Database-Management-Package.ps1',
    'database/README.md'
)

Write-Step 'Validate consolidation state'

$trackedBefore = Get-TrackedFiles
$sourceSet = @($approvedMoves.Keys)
$destinationSet = @($approvedMoves.Values)

if ($sourceSet.Count -ne 8) {
    throw "BLOCKED: expected 8 approved sources, found $($sourceSet.Count)."
}

if (($sourceSet | Select-Object -Unique).Count -ne $sourceSet.Count) {
    throw 'BLOCKED: duplicate approved source paths exist.'
}

if (($destinationSet | Select-Object -Unique).Count -ne $destinationSet.Count) {
    throw 'BLOCKED: duplicate approved destination paths exist.'
}

foreach ($protectedPath in $protectedPaths) {
    if ($trackedBefore -notcontains $protectedPath) {
        throw "BLOCKED: protected tracked file is missing: $protectedPath"
    }
}

$migrationFilesBefore = @(
    $trackedBefore |
        Where-Object {
            $_.StartsWith(
                'apps/api/migrations/',
                [System.StringComparison]::OrdinalIgnoreCase
            )
        } |
        Sort-Object
)

if ($migrationFilesBefore.Count -eq 0) {
    throw 'BLOCKED: canonical Migration history was not found.'
}

$pendingMoves = @()
$alreadyMoved = @()

foreach ($source in $sourceSet) {
    $destination = $approvedMoves[$source]
    $sourceTracked = $trackedBefore -contains $source
    $destinationTracked = $trackedBefore -contains $destination

    if ($sourceTracked -and -not $destinationTracked) {
        $pendingMoves += [pscustomobject]@{
            Source = $source
            Destination = $destination
        }
        continue
    }

    if (-not $sourceTracked -and $destinationTracked) {
        $alreadyMoved += [pscustomobject]@{
            Source = $source
            Destination = $destination
        }
        continue
    }

    if ($sourceTracked -and $destinationTracked) {
        throw "BLOCKED: source and destination are both tracked: $source"
    }

    throw "BLOCKED: neither source nor destination is tracked: $source"
}

Write-Host 'Safety gate: PASS'
Write-Host "Pending moves: $($pendingMoves.Count)"
Write-Host "Already moved: $($alreadyMoved.Count)"
Write-Host 'Deletes: 0'

if (-not $Apply) {
    Write-Host ""
    Write-Host 'PASS'
    return
}

Write-Step 'Apply pending moves'

foreach ($move in $pendingMoves) {
    $destinationFullPath = Join-Path $repoRoot (
        $move.Destination.Replace(
            '/',
            [System.IO.Path]::DirectorySeparatorChar
        )
    )
    $destinationDirectory = Split-Path -Parent $destinationFullPath
    New-Item -ItemType Directory -Force -Path $destinationDirectory | Out-Null

    if ($PSCmdlet.ShouldProcess(
        $move.Source,
        "Move to $($move.Destination)"
    )) {
        & git mv -- $move.Source $move.Destination
        if ($LASTEXITCODE -ne 0) {
            throw "BLOCKED: git mv failed: $($move.Source)"
        }
    }
}

Write-Step 'Update path references after moves'

$replacementMap = @{}
foreach ($source in $sourceSet) {
    $destination = $approvedMoves[$source]
    $replacementMap[$source] = $destination
    $replacementMap[$source.Replace('/', '\')] = $destination.Replace('/', '\')
}

$trackedAfterMove = Get-TrackedFiles
$updatedReferenceFiles = @()

foreach ($relativePath in $trackedAfterMove) {
    if ($referenceScanExclusions -contains $relativePath) {
        continue
    }

    if ($relativePath.StartsWith(
        'apps/api/migrations/',
        [System.StringComparison]::OrdinalIgnoreCase
    )) {
        continue
    }

    $fullPath = Join-Path $repoRoot (
        $relativePath.Replace(
            '/',
            [System.IO.Path]::DirectorySeparatorChar
        )
    )

    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        continue
    }

    if (-not (Test-TextFile -Path $fullPath)) {
        continue
    }

    $content = [System.IO.File]::ReadAllText($fullPath)
    $updated = $content

    foreach ($oldPath in $replacementMap.Keys) {
        $updated = $updated.Replace(
            $oldPath,
            $replacementMap[$oldPath]
        )
    }

    if ($updated -ne $content) {
        [System.IO.File]::WriteAllText(
            $fullPath,
            $updated,
            (New-Object System.Text.UTF8Encoding($false))
        )
        $updatedReferenceFiles += $relativePath
    }
}

Write-Step 'Verify consolidation result'

$trackedAfter = Get-TrackedFiles

foreach ($source in $sourceSet) {
    $destination = $approvedMoves[$source]

    if ($trackedAfter -contains $source) {
        throw "BLOCKED: source still tracked after consolidation: $source"
    }

    if ($trackedAfter -notcontains $destination) {
        throw "BLOCKED: destination is not tracked after consolidation: $destination"
    }
}

foreach ($protectedPath in $protectedPaths) {
    if ($trackedAfter -notcontains $protectedPath) {
        throw "BLOCKED: protected file was moved or deleted: $protectedPath"
    }
}

$migrationFilesAfter = @(
    $trackedAfter |
        Where-Object {
            $_.StartsWith(
                'apps/api/migrations/',
                [System.StringComparison]::OrdinalIgnoreCase
            )
        } |
        Sort-Object
)

if (($migrationFilesBefore -join "`n") -ne ($migrationFilesAfter -join "`n")) {
    throw 'BLOCKED: canonical Migration history changed unexpectedly.'
}

$staleReferences = @()

foreach ($relativePath in $trackedAfter) {
    if ($referenceScanExclusions -contains $relativePath) {
        continue
    }

    if ($relativePath.StartsWith(
        'apps/api/migrations/',
        [System.StringComparison]::OrdinalIgnoreCase
    )) {
        continue
    }

    $fullPath = Join-Path $repoRoot (
        $relativePath.Replace(
            '/',
            [System.IO.Path]::DirectorySeparatorChar
        )
    )

    if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
        continue
    }

    if (-not (Test-TextFile -Path $fullPath)) {
        continue
    }

    $content = [System.IO.File]::ReadAllText($fullPath)

    foreach ($source in $sourceSet) {
        if (
            $content.Contains($source) -or
            $content.Contains($source.Replace('/', '\'))
        ) {
            $staleReferences += "$relativePath -> $source"
        }
    }
}

if ($staleReferences.Count -gt 0) {
    throw (
        "BLOCKED: stale script path references remain:`n" +
        ($staleReferences -join "`n")
    )
}

& git diff --check
if ($LASTEXITCODE -ne 0) {
    throw 'BLOCKED: git diff --check failed.'
}

Write-Host ""
Write-Host 'PASS'
Write-Host "Moved this run: $($pendingMoves.Count)"
Write-Host "Already moved: $($alreadyMoved.Count)"
Write-Host "Updated reference files: $($updatedReferenceFiles.Count)"
Write-Host 'Deleted: 0'
Write-Host 'Protected files: PASS'
Write-Host 'Migration history: PASS'
Write-Host 'Stale references: 0'
