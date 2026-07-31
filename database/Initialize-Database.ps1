[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [Parameter(Mandatory = $true)]
    [switch]$ConfirmClearData,

    [switch]$SkipBackup,

    [string]$SeedFile,

    [string]$BackupDirectory
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Step {
    param([Parameter(Mandatory = $true)][string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code ${LASTEXITCODE}: $FilePath"
    }
}

function Get-RepositoryRoot {
    $root = (& git rev-parse --show-toplevel 2>$null)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($root)) {
        throw 'Current directory is not inside a Git repository.'
    }
    return [System.IO.Path]::GetFullPath($root.Trim())
}

function Get-DatabaseNameFromUrl {
    param([Parameter(Mandatory = $true)][string]$DatabaseUrl)

    try {
        $uri = [System.Uri]$DatabaseUrl
    }
    catch {
        throw 'DATABASE_URL is not a valid URI.'
    }

    $databaseName = $uri.AbsolutePath.Trim('/')
    if ([string]::IsNullOrWhiteSpace($databaseName)) {
        throw 'DATABASE_URL does not contain a database name.'
    }

    return [System.Uri]::UnescapeDataString($databaseName)
}

function Invoke-PsqlScalar {
    param(
        [Parameter(Mandatory = $true)][string]$PsqlPath,
        [Parameter(Mandatory = $true)][string]$DatabaseUrl,
        [Parameter(Mandatory = $true)][string]$Sql
    )

    $output = & $PsqlPath `
        '-X' `
        '--no-psqlrc' `
        '--tuples-only' `
        '--no-align' `
        '--set=ON_ERROR_STOP=1' `
        "--dbname=$DatabaseUrl" `
        "--command=$Sql"

    if ($LASTEXITCODE -ne 0) {
        throw 'psql query failed.'
    }

    return (($output | ForEach-Object { $_.Trim() }) -join "`n").Trim()
}

if (-not $ConfirmClearData) {
    throw 'Refusing to clear data. Re-run with -ConfirmClearData.'
}

if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL)) {
    throw 'DATABASE_URL is not set.'
}

$databaseName = Get-DatabaseNameFromUrl -DatabaseUrl $env:DATABASE_URL
if ($databaseName -ne 'ai_content_factory') {
    throw "Refusing to operate on database '$databaseName'. Expected ai_content_factory."
}

$repoRoot = Get-RepositoryRoot
$databaseRoot = Join-Path $repoRoot 'database'
$generatedRoot = Join-Path $databaseRoot 'generated'
$migrationsRoot = Join-Path $repoRoot 'apps\api\migrations'

if (-not (Test-Path -LiteralPath $migrationsRoot -PathType Container)) {
    throw "Migration directory not found: $migrationsRoot"
}

$migrationVersions = @(
    Get-ChildItem -LiteralPath $migrationsRoot -File -Filter '*.up.sql' |
        ForEach-Object {
            if ($_.Name -notmatch '^(?<version>\d+)_.*\.up\.sql$') {
                throw "Invalid up migration filename: $($_.Name)"
            }
            [int]$Matches['version']
        } |
        Sort-Object -Unique
)

if ($migrationVersions.Count -eq 0) {
    throw 'No up migrations were found.'
}

$expectedMigrationState = ($migrationVersions -join ',')
$expectedMigrationMax = $migrationVersions[-1]

if ([string]::IsNullOrWhiteSpace($SeedFile)) {
    $SeedFile = Join-Path $databaseRoot 'testdata\complete-test-data.sql'
}
elseif (-not [System.IO.Path]::IsPathRooted($SeedFile)) {
    $SeedFile = Join-Path $repoRoot $SeedFile
}
$SeedFile = [System.IO.Path]::GetFullPath($SeedFile)

if (-not (Test-Path -LiteralPath $SeedFile -PathType Leaf)) {
    throw "Seed file not found: $SeedFile"
}

New-Item -ItemType Directory -Force -Path $generatedRoot | Out-Null

$psql = (Get-Command psql -ErrorAction Stop).Source
$pgDump = $null
if (-not $SkipBackup) {
    $pgDump = (Get-Command pg_dump -ErrorAction Stop).Source
}

Write-Step 'Validate migration state'
$migrationState = Invoke-PsqlScalar `
    -PsqlPath $psql `
    -DatabaseUrl $env:DATABASE_URL `
    -Sql 'SELECT COALESCE(string_agg(version::text, '','' ORDER BY version), '''') FROM schema_migrations;'

if ($migrationState -ne $expectedMigrationState) {
    throw "Unexpected migration versions '$migrationState'. Expected '$expectedMigrationState'."
}

Write-Host "Migration versions: $migrationState"
Write-Host "Migration max version: $expectedMigrationMax"

if (-not $SkipBackup) {
    Write-Step 'Create full custom-format backup'

    if ([string]::IsNullOrWhiteSpace($BackupDirectory)) {
        $BackupDirectory = Join-Path ([System.IO.Path]::GetTempPath()) 'acf-database-backups'
    }
    elseif (-not [System.IO.Path]::IsPathRooted($BackupDirectory)) {
        $BackupDirectory = Join-Path $repoRoot $BackupDirectory
    }

    $BackupDirectory = [System.IO.Path]::GetFullPath($BackupDirectory)
    New-Item -ItemType Directory -Force -Path $BackupDirectory | Out-Null

    $timestamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')
    $backupFile = Join-Path $BackupDirectory "ai_content_factory_before_reset_$timestamp.dump"

    Invoke-Native -FilePath $pgDump -Arguments @(
        '--format=custom',
        '--no-owner',
        '--no-privileges',
        "--file=$backupFile",
        $env:DATABASE_URL
    )

    $backupInfo = Get-Item -LiteralPath $backupFile
    if ($backupInfo.Length -le 0) {
        throw "Backup file is empty: $backupFile"
    }

    Write-Host "Backup: $backupFile"
}

Write-Step 'Read current public table inventory'
$tableRows = & $psql `
    '-X' `
    '--no-psqlrc' `
    '--tuples-only' `
    '--no-align' `
    '--field-separator=|' `
    '--set=ON_ERROR_STOP=1' `
    "--dbname=$env:DATABASE_URL" `
    '--command=SELECT schemaname, tablename FROM pg_tables WHERE schemaname = ''public'' ORDER BY tablename;'

if ($LASTEXITCODE -ne 0) {
    throw 'Unable to read public table inventory.'
}

$tables = @()
foreach ($row in $tableRows) {
    $parts = $row.Trim() -split '\|', 2
    if ($parts.Count -ne 2) {
        continue
    }

    $tables += [pscustomobject]@{
        Schema = $parts[0]
        Table = $parts[1]
    }
}

$businessTables = @($tables | Where-Object { $_.Table -ne 'schema_migrations' })
if ($businessTables.Count -eq 0) {
    throw 'No business tables were found in public schema.'
}

Write-Host "Public tables: $($tables.Count)"
Write-Host "Business tables to clear: $($businessTables.Count)"

$generatedSql = Join-Path $generatedRoot 'initialize-current-database.sql'

$seedSql = [System.IO.File]::ReadAllText($SeedFile)
$seedSql = $seedSql -replace "`r`n", "`n"

$beginMatches = [regex]::Matches($seedSql, '(?im)^\s*BEGIN;\s*$')
$commitMatches = [regex]::Matches($seedSql, '(?im)^\s*COMMIT;\s*$')
if ($beginMatches.Count -ne 1 -or $commitMatches.Count -ne 1) {
    throw 'Seed SQL must contain exactly one BEGIN and one COMMIT statement.'
}

$seedBody = [regex]::Replace(
    $seedSql,
    '(?im)^\s*BEGIN;\s*$|^\s*COMMIT;\s*$',
    ''
)
$seedBody = [regex]::Replace(
    $seedBody,
    '(?im)^\s*\\set\s+ON_ERROR_STOP\s+on\s*$',
    ''
).Trim()

$initializationSql = @"
\set ON_ERROR_STOP on

BEGIN;
SET CONSTRAINTS ALL DEFERRED;

DO `$acf_reset`$
DECLARE
    table_list TEXT;
BEGIN
    SELECT string_agg(format('%I.%I', schemaname, tablename), ', ' ORDER BY tablename)
    INTO table_list
    FROM pg_tables
    WHERE schemaname = 'public'
      AND tablename <> 'schema_migrations';

    IF table_list IS NULL THEN
        RAISE EXCEPTION 'No business tables found in public schema';
    END IF;

    EXECUTE 'TRUNCATE TABLE ' || table_list || ' RESTART IDENTITY CASCADE';
END
`$acf_reset`$;

$seedBody

DO `$acf_coverage`$
DECLARE
    target_table RECORD;
    row_count BIGINT;
BEGIN
    FOR target_table IN
        SELECT schemaname, tablename
        FROM pg_tables
        WHERE schemaname = 'public'
          AND tablename <> 'schema_migrations'
        ORDER BY tablename
    LOOP
        EXECUTE format(
            'SELECT COUNT(*) FROM %I.%I',
            target_table.schemaname,
            target_table.tablename
        )
        INTO row_count;

        IF row_count = 0 THEN
            RAISE EXCEPTION
                'Fixture coverage failed: %.% is empty',
                target_table.schemaname,
                target_table.tablename;
        END IF;
    END LOOP;
END
`$acf_coverage`$;

COMMIT;
"@

[System.IO.File]::WriteAllText(
    $generatedSql,
    $initializationSql,
    (New-Object System.Text.UTF8Encoding($false))
)

Write-Step 'Initialize database from generated script'
if ($PSCmdlet.ShouldProcess(
    'ai_content_factory',
    "Clear $($businessTables.Count) business tables and load deterministic fixtures"
)) {
    Invoke-Native -FilePath $psql -Arguments @(
        '-X',
        '--no-psqlrc',
        '--set=ON_ERROR_STOP=1',
        "--dbname=$env:DATABASE_URL",
        "--file=$generatedSql"
    )
}
else {
    throw 'Operation cancelled.'
}

Write-Step 'Verify migration state was preserved'
$finalMigrationState = Invoke-PsqlScalar `
    -PsqlPath $psql `
    -DatabaseUrl $env:DATABASE_URL `
    -Sql 'SELECT COALESCE(string_agg(version::text, '','' ORDER BY version), '''') FROM schema_migrations;'

if ($finalMigrationState -ne $expectedMigrationState) {
    throw "Migration versions changed unexpectedly: $finalMigrationState"
}

Write-Step 'Report row counts'
$countRows = & $psql `
    '-X' `
    '--no-psqlrc' `
    '--tuples-only' `
    '--no-align' `
    '--field-separator=|' `
    '--set=ON_ERROR_STOP=1' `
    "--dbname=$env:DATABASE_URL" `
    '--command=SELECT format(''%I.%I'', schemaname, tablename), (xpath(''/row/c/text()'', query_to_xml(format(''SELECT count(*) AS c FROM %I.%I'', schemaname, tablename), false, true, '''')))[1]::text::bigint FROM pg_tables WHERE schemaname = ''public'' AND tablename <> ''schema_migrations'' ORDER BY tablename;'

if ($LASTEXITCODE -ne 0) {
    throw 'Unable to read final row counts.'
}

$totalRows = 0L
$nonEmptyTables = 0
foreach ($row in $countRows) {
    $parts = $row.Trim() -split '\|', 2
    if ($parts.Count -ne 2) {
        continue
    }

    $count = [int64]$parts[1]
    $totalRows += $count
    if ($count -gt 0) {
        $nonEmptyTables++
    }

    Write-Host ("{0,-50} {1,8}" -f $parts[0], $count)
}

if ($nonEmptyTables -ne $businessTables.Count) {
    throw "Fixture coverage is incomplete: $nonEmptyTables of $($businessTables.Count) business tables contain data."
}

Write-Host ""
Write-Host "Database: ai_content_factory"
Write-Host "Migration versions: $finalMigrationState"
Write-Host "Migration max version: $expectedMigrationMax"
Write-Host "Business tables: $($businessTables.Count)"
Write-Host "Non-empty tables: $nonEmptyTables"
Write-Host "Fixture rows: $totalRows"
Write-Host "Generated initialization SQL: $generatedSql"
Write-Host "PASS"
