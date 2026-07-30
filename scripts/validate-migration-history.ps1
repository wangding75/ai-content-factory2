# Validate Migration History Integrity
# Validates that all migration SQL files match their recorded SHA-256 checksums
# PowerShell 5.1 compatible

$ErrorActionPreference = 'Stop'

# Locate repo root
$repoRoot = git rev-parse --show-toplevel
if ($LASTEXITCODE -ne 0) {
    Write-Error "FAIL: not a Git repository or git command failed"
    exit 1
}

$migrationDir = Join-Path $repoRoot 'apps\api\migrations'
$manifestPath = Join-Path $migrationDir 'migration-checksums.sha256'
$manifestName = 'migration-checksums.sha256'

Write-Output "Repository: $repoRoot"
Write-Output "Migration directory: $migrationDir"
Write-Output "Manifest: $manifestPath"

# === 1. Basic checks ===

# 1.1 Migration directory exists
if (-not (Test-Path $migrationDir -PathType Container)) {
    Write-Error "FAIL: Migration directory not found: $migrationDir"
    exit 1
}

# 1.2 Manifest exists
if (-not (Test-Path $manifestPath -PathType Leaf)) {
    Write-Error "FAIL: Manifest not found: $manifestPath"
    exit 1
}

# 1.3 Manifest non-empty
$manifestContent = Get-Content $manifestPath -Raw
if ([string]::IsNullOrWhiteSpace($manifestContent)) {
    Write-Error "FAIL: Manifest is empty: $manifestPath"
    exit 1
}

# Collect actual migration SQL files
$actualFiles = Get-ChildItem -Path $migrationDir -Filter '*.sql' | Sort-Object Name

# 1.4 At least one up migration
$upFiles = $actualFiles | Where-Object { $_.Name -match '\.up\.sql$' }
if ($upFiles.Count -eq 0) {
    Write-Error "FAIL: No up migration files found"
    exit 1
}

# 1.5 At least one down migration
$downFiles = $actualFiles | Where-Object { $_.Name -match '\.down\.sql$' }
if ($downFiles.Count -eq 0) {
    Write-Error "FAIL: No down migration files found"
    exit 1
}

# === 2. Migration file name checks ===

$namePattern = '^(\d+)_(.+)\.(up|down)\.sql$'
$versionUpMap = @{}
$versionDownMap = @{}
$versionBaseNameMap = @{}

foreach ($file in $actualFiles) {
    $name = $file.Name

    if ($name -match $namePattern) {
        $version = $Matches[1]
        $baseName = $Matches[2]
        $direction = $Matches[3]

        if ($direction -eq 'up') {
            if ($versionUpMap.ContainsKey($version)) {
                Write-Error "FAIL: duplicate up migration for version $version"
                exit 1
            }
            $versionUpMap[$version] = $name
            $versionBaseNameMap[$version] = $baseName
        } else {
            if ($versionDownMap.ContainsKey($version)) {
                Write-Error "FAIL: duplicate down migration for version $version"
                exit 1
            }
            $versionDownMap[$version] = $name
            if ($versionBaseNameMap.ContainsKey($version)) {
                if ($versionBaseNameMap[$version] -ne $baseName) {
                    Write-Error "FAIL: up/down base name mismatch for version ${version}: up='$($versionBaseNameMap[$version])' down='$baseName'"
                    exit 1
                }
            } else {
                $versionBaseNameMap[$version] = $baseName
            }
        }
    } else {
        Write-Error "FAIL: invalid migration file name format: $name"
        exit 1
    }
}

# Check that every version with up has a matching down and vice versa
foreach ($version in $versionUpMap.Keys) {
    if (-not $versionDownMap.ContainsKey($version)) {
        Write-Error "FAIL: version $version has up but no down migration"
        exit 1
    }
}
foreach ($version in $versionDownMap.Keys) {
    if (-not $versionUpMap.ContainsKey($version)) {
        Write-Error "FAIL: version $version has down but no up migration"
        exit 1
    }
}

# === 3. Manifest format checks ===

$manifestLines = Get-Content $manifestPath
$manifestLinePattern = '^[0-9a-f]{64}  [^\\/]+\.sql$'
$seenNames = @{}
$seenLines = @{}
$lineNumber = 0

foreach ($line in $manifestLines) {
    $lineNumber++

    # No empty lines
    if ([string]::IsNullOrWhiteSpace($line)) {
        Write-Error "FAIL: Manifest contains empty line at line $lineNumber"
        exit 1
    }

    # Match format: <64 hex>  <filename>.sql
    if ($line -notmatch $manifestLinePattern) {
        Write-Error "FAIL: Manifest line $lineNumber has invalid format: $line"
        exit 1
    }

    # Extract filename
    $fileName = $line -replace '^[0-9a-f]{64}  ', ''

    # No backslash or forward slash in filename
    if ($fileName -match '[\\/]') {
        Write-Error "FAIL: Manifest line $lineNumber contains path separator in filename: $fileName"
        exit 1
    }

    # No absolute path (starts with drive letter or /)
    if ($fileName -match '^[A-Za-z]:') {
        Write-Error "FAIL: Manifest line $lineNumber contains absolute path: $fileName"
        exit 1
    }

    # Must not record manifest itself
    if ($fileName -eq $manifestName) {
        Write-Error "FAIL: Manifest records itself at line $lineNumber"
        exit 1
    }

    # No duplicate filename in manifest
    if ($seenNames.ContainsKey($fileName)) {
        Write-Error "FAIL: Manifest contains duplicate file name: $fileName"
        exit 1
    }
    $seenNames[$fileName] = $true

    # No duplicate line
    if ($seenLines.ContainsKey($line)) {
        Write-Error "FAIL: Manifest contains duplicate line: $line"
        exit 1
    }
    $seenLines[$line] = $true
}

# === 4. File set checks ===

# 4.1 Every actual SQL file must be in manifest
$manifestFileNames = $seenNames.Keys
foreach ($file in $actualFiles) {
    if ($manifestFileNames -notcontains $file.Name) {
        Write-Error "FAIL: Migration file not recorded in manifest: $($file.Name)"
        exit 1
    }
}

# 4.2 Every manifest entry must correspond to an actual file
$actualFileNames = $actualFiles | ForEach-Object { $_.Name }
foreach ($name in $manifestFileNames) {
    if ($actualFileNames -notcontains $name) {
        Write-Error "FAIL: Manifest records file that does not exist: $name"
        exit 1
    }
}

# 4.3 Counts must match
if ($actualFiles.Count -ne $manifestLines.Count) {
    Write-Error "FAIL: file count mismatch: $($actualFiles.Count) actual SQL files vs $($manifestLines.Count) manifest entries"
    exit 1
}

# === 5. Hash verification ===

foreach ($line in $manifestLines) {
    $expectedHash = $line.Substring(0, 64)
    $fileName = $line.Substring(66)

    $filePath = Join-Path $migrationDir $fileName
    if (-not (Test-Path $filePath -PathType Leaf)) {
        Write-Error "FAIL: Manifest file not found on disk: $fileName"
        exit 1
    }

    $actualHash = (Get-FileHash -Algorithm SHA256 -Path $filePath).Hash.ToLower()

    if ($actualHash -ne $expectedHash) {
        Write-Error "FAIL: Hash mismatch for $fileName"
        Write-Error "  Expected: $expectedHash"
        Write-Error "  Actual:   $actualHash"
        exit 1
    }
}

# === Summary ===

$versionCount = $versionUpMap.Count
$maxVersion = ($versionUpMap.Keys | ForEach-Object { [int]$_ } | Measure-Object -Maximum).Maximum

Write-Output "Up files: $($upFiles.Count)"
Write-Output "Down files: $($downFiles.Count)"
Write-Output "Migration versions: $versionCount"
Write-Output "Manifest entries: $($manifestLines.Count)"
Write-Output "Max version: $maxVersion"
Write-Output "PASS"