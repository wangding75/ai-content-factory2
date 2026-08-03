[CmdletBinding()]
param(
    [string]$DatabaseUrl = $env:DATABASE_URL
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ([string]::IsNullOrWhiteSpace($DatabaseUrl)) {
    throw 'DATABASE_URL is required.'
}

$repoRoot = [System.IO.Path]::GetFullPath((& git rev-parse --show-toplevel).Trim())
if ($LASTEXITCODE -ne 0) { throw 'Current directory is not inside the repository.' }
$apiRoot = Join-Path $repoRoot 'apps\api'
$migrationRoot = Join-Path $apiRoot 'migrations'
$psqlCommand = Get-Command psql -ErrorAction SilentlyContinue
if ($null -ne $psqlCommand) {
    $psql = $psqlCommand.Source
}
else {
    $psql = Get-ChildItem -LiteralPath (Join-Path $env:ProgramFiles 'PostgreSQL') -Recurse -Filter 'psql.exe' -ErrorAction SilentlyContinue |
        Sort-Object -Property FullName -Descending |
        Select-Object -First 1 -ExpandProperty FullName
    if ([string]::IsNullOrWhiteSpace($psql)) { throw 'psql was not found on PATH or under Program Files\PostgreSQL.' }
}
$suffix = [guid]::NewGuid().ToString('N').Substring(0, 12)
$databaseNames = @(
    "acf_i19_migration_${suffix}_baseline",
    "acf_i19_migration_${suffix}_direct",
    "acf_i19_migration_${suffix}_cycle"
)
$temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) "acf-i19-migration-$suffix"
$migration18Root = Join-Path $temporaryRoot 'migrations-18'
$migration22Root = Join-Path $temporaryRoot 'migrations-22'

function Assert-TemporaryDatabaseName {
    param([Parameter(Mandatory = $true)][string]$Name)
    if ($Name -notmatch '^acf_i19_migration_[a-f0-9]{12}_(baseline|direct|cycle)$') {
        throw "Unsafe temporary database name: $Name"
    }
}

function Get-DatabaseUrl {
    param([Parameter(Mandatory = $true)][string]$DatabaseName)
    Assert-TemporaryDatabaseName $DatabaseName
    $builder = [System.UriBuilder]$DatabaseUrl
    $builder.Path = "/$DatabaseName"
    return $builder.Uri.AbsoluteUri
}

function Get-AdminUrl {
    $builder = [System.UriBuilder]$DatabaseUrl
    $builder.Path = '/postgres'
    return $builder.Uri.AbsoluteUri
}

function Invoke-Psql {
    param(
        [Parameter(Mandatory = $true)][string]$TargetUrl,
        [Parameter(Mandatory = $true)][string]$Sql,
        [switch]$Scalar
    )
    $arguments = @('-X', '--no-psqlrc', '--set=ON_ERROR_STOP=1', "--dbname=$TargetUrl", "--command=$Sql")
    if ($Scalar) { $arguments = @('-X', '--no-psqlrc', '--tuples-only', '--no-align', '--set=ON_ERROR_STOP=1', "--dbname=$TargetUrl", "--command=$Sql") }
    $output = & $psql @arguments
    if ($LASTEXITCODE -ne 0) { throw 'psql command failed.' }
    if ($Scalar) { return (($output | ForEach-Object { $_.Trim() }) -join "`n").Trim() }
}

function Invoke-Migrate {
    param(
        [Parameter(Mandatory = $true)][string]$TargetUrl,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [string]$MigrationsDirectory = $migrationRoot
    )
    Push-Location $apiRoot
    try {
        $previousDatabaseUrl = $env:DATABASE_URL
        $previousMigrationsDirectory = $env:MIGRATIONS_DIR
        try {
            $env:DATABASE_URL = $TargetUrl
            $env:MIGRATIONS_DIR = $MigrationsDirectory
            & go run ./cmd/migrate @Arguments
            if ($LASTEXITCODE -ne 0) { throw "migrate $($Arguments -join ' ') failed." }
        }
        finally {
            $env:DATABASE_URL = $previousDatabaseUrl
            $env:MIGRATIONS_DIR = $previousMigrationsDirectory
        }
    }
    finally {
        Pop-Location
    }
}

function Invoke-MigrateDownToVersion {
    param(
        [Parameter(Mandatory = $true)][string]$TargetUrl,
        [Parameter(Mandatory = $true)][int]$TargetVersion
    )

    while ($true) {
        $current = [int](Invoke-Psql -TargetUrl $TargetUrl -Sql 'SELECT COALESCE(MAX(version), 0) FROM schema_migrations;' -Scalar)
        if ($current -le $TargetVersion) {
            return
        }
        Invoke-Migrate -TargetUrl $TargetUrl -Arguments @('down', '1')
    }
}

function Get-SchemaFingerprint {
    param([Parameter(Mandatory = $true)][string]$TargetUrl)
    $sql = @'
WITH schema_objects AS (
    SELECT 'column' AS kind,
           table_name || '.' || column_name || ':' || data_type || ':' || udt_name || ':' || is_nullable || ':' || COALESCE(column_default, '') AS definition
    FROM information_schema.columns
    WHERE table_schema = 'public'
    UNION ALL
    SELECT 'constraint', c.relname || '.' || con.conname || ':' || pg_get_constraintdef(con.oid, true)
    FROM pg_constraint con
    JOIN pg_class c ON c.oid = con.conrelid
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public'
    UNION ALL
    SELECT 'index', tablename || '.' || indexname || ':' || indexdef
    FROM pg_indexes
    WHERE schemaname = 'public'
)
SELECT md5(string_agg(kind || ':' || definition, E'\n' ORDER BY kind, definition))
FROM schema_objects;
'@
    return Invoke-Psql -TargetUrl $TargetUrl -Sql $sql -Scalar
}

New-Item -ItemType Directory -Path $migration18Root -Force | Out-Null
New-Item -ItemType Directory -Path $migration22Root -Force | Out-Null
Get-ChildItem -LiteralPath $migrationRoot -File -Filter '*.sql' |
    Where-Object { $_.Name -match '^0000(0[1-9]|1[0-8])_' } |
    ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $migration18Root $_.Name) }
Get-ChildItem -LiteralPath $migrationRoot -File -Filter '*.sql' |
    Where-Object { $_.Name -match '^0000(0[1-9]|1[0-9]|2[0-2])_' } |
    ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $migration22Root $_.Name) }

$adminUrl = Get-AdminUrl
try {
    foreach ($databaseName in $databaseNames) {
        Assert-TemporaryDatabaseName $databaseName
        Invoke-Psql -TargetUrl $adminUrl -Sql "CREATE DATABASE `"$databaseName`";"
    }

    $baselineUrl = Get-DatabaseUrl $databaseNames[0]
    $directUrl = Get-DatabaseUrl $databaseNames[1]
    $cycleUrl = Get-DatabaseUrl $databaseNames[2]

    Invoke-Migrate -TargetUrl $baselineUrl -Arguments @('up') -MigrationsDirectory $migration18Root
    Invoke-Migrate -TargetUrl $directUrl -Arguments @('up')
    Invoke-Migrate -TargetUrl $cycleUrl -Arguments @('up')
    Invoke-MigrateDownToVersion -TargetUrl $cycleUrl -TargetVersion 18

    $downVersion = Invoke-Psql -TargetUrl $cycleUrl -Sql 'SELECT COALESCE(MAX(version), 0) FROM schema_migrations;' -Scalar
    if ($downVersion -ne '18') { throw "Migration down ended at version $downVersion instead of 18." }
    $baselineFingerprint = Get-SchemaFingerprint $baselineUrl
    $downFingerprint = Get-SchemaFingerprint $cycleUrl
    if ($baselineFingerprint -ne $downFingerprint) {
        throw "Migration 19 down schema differs from the direct version 18 schema: baseline=$baselineFingerprint down=$downFingerprint"
    }

    Invoke-Migrate -TargetUrl $cycleUrl -Arguments @('up')
    $directFingerprint = Get-SchemaFingerprint $directUrl
    $cycleFingerprint = Get-SchemaFingerprint $cycleUrl
    if ($directFingerprint -ne $cycleFingerprint) {
        throw "Migration 19 repeated up schema differs from direct up: direct=$directFingerprint cycle=$cycleFingerprint"
    }
    $finalVersion = Invoke-Psql -TargetUrl $cycleUrl -Sql 'SELECT COALESCE(MAX(version), 0) FROM schema_migrations;' -Scalar
    if ($finalVersion -ne '22') { throw "Migration cycle ended at version $finalVersion instead of 22." }

    Write-Output "[PASS] Down schema matches direct version 18: $baselineFingerprint"
    Write-Output "[PASS] Direct up schema matches up/down/up: $directFingerprint"
    Write-Output '[PASS] Migration cycle ended at version 22.'
}
finally {
    foreach ($databaseName in $databaseNames) {
        Assert-TemporaryDatabaseName $databaseName
        try { Invoke-Psql -TargetUrl $adminUrl -Sql "DROP DATABASE IF EXISTS `"$databaseName`" WITH (FORCE);" } catch { Write-Warning $_ }
    }
    if (Test-Path -LiteralPath $temporaryRoot -PathType Container) {
        [System.IO.Directory]::Delete($temporaryRoot, $true)
    }
}
