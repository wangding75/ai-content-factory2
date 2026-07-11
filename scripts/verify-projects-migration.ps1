Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if (-not $env:DATABASE_URL) {
    $env:DATABASE_URL = "postgres://acf:acf@localhost:5432/acf?sslmode=disable"
}

function Invoke-NativeChecked([scriptblock]$Command, [string]$Message) {
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        & $Command *> $null
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }
    if ($LASTEXITCODE -ne 0) { throw $Message }
}


function Invoke-SqlAssertion([string]$Name, [string]$Sql) {
    docker compose exec -T postgres psql -U acf -d acf -v ON_ERROR_STOP=1 -c $Sql *> $null
    if ($LASTEXITCODE -ne 0) { throw "Database assertion failed: $Name" }
    Write-Host "[PASS] $Name" -ForegroundColor Green
}

Invoke-NativeChecked { docker compose up -d postgres } "Unable to start PostgreSQL."
$containerId = (docker compose ps -q postgres 2>$null | Select-Object -Last 1).Trim()
if ([string]::IsNullOrWhiteSpace($containerId)) { throw "PostgreSQL container was not found." }

$healthy = $false
for ($attempt = 1; $attempt -le 30; $attempt++) {
    $health = (docker inspect --format "{{.State.Health.Status}}" $containerId 2>$null | Select-Object -Last 1).Trim()
    if ($health -eq "healthy") { $healthy = $true; break }
    Start-Sleep -Seconds 2
}
if (-not $healthy) { throw "PostgreSQL did not become healthy." }
Write-Host "[PASS] PostgreSQL health=healthy" -ForegroundColor Green

$projectId = [guid]::NewGuid().ToString()
$invalidTypeId = [guid]::NewGuid().ToString()
$cleanupSql = "DELETE FROM projects WHERE id IN ('$projectId', '$invalidTypeId');"
$projectsUp = $false

try {
    Push-Location .\apps\api
    try {
        Invoke-NativeChecked { go run ./cmd/migrate up } "Migration up failed."
        Invoke-NativeChecked { go run ./cmd/migrate down 1 } "Migration down 1 failed."
    } finally { Pop-Location }

    Invoke-SqlAssertion "audit_logs survives down 1" @'
DO $$
BEGIN
    IF to_regclass('public.audit_logs') IS NULL THEN RAISE EXCEPTION 'audit_logs must exist'; END IF;
END
$$;
'@

    Invoke-SqlAssertion "projects is absent after down 1" @'
DO $$
BEGIN
    IF to_regclass('public.projects') IS NOT NULL THEN RAISE EXCEPTION 'projects must be absent'; END IF;
END
$$;
'@

    Invoke-SqlAssertion "schema_migrations is version 1 after down 1" @'
DO $$
DECLARE current_version BIGINT;
BEGIN
    SELECT COALESCE(MAX(version), 0) INTO current_version FROM schema_migrations;
    IF current_version <> 1 THEN RAISE EXCEPTION 'expected version 1, got %', current_version; END IF;
END
$$;
'@

    Push-Location .\apps\api
    try { Invoke-NativeChecked { go run ./cmd/migrate up } "Second migration up failed." }
    finally { Pop-Location }
    $projectsUp = $true

    Invoke-SqlAssertion "projects exists after second up" @'
DO $$
BEGIN
    IF to_regclass('public.projects') IS NULL THEN RAISE EXCEPTION 'projects must exist'; END IF;
END
$$;
'@

    Invoke-SqlAssertion "schema_migrations is version 2 after second up" @'
DO $$
DECLARE current_version BIGINT;
BEGIN
    SELECT COALESCE(MAX(version), 0) INTO current_version FROM schema_migrations;
    IF current_version <> 2 THEN RAISE EXCEPTION 'expected version 2, got %', current_version; END IF;
END
$$;
'@

    $defaultSql = @'
INSERT INTO projects (id, name, type, created_by) VALUES ('__PROJECT_ID__', 'Migration validation', 'novel', 'system');
DO $$
DECLARE project_status TEXT; project_stage TEXT;
BEGIN
    SELECT status, current_stage INTO project_status, project_stage FROM projects WHERE id = '__PROJECT_ID__';
    IF project_status <> 'planning' OR project_stage <> 'project_setup' THEN
        RAISE EXCEPTION 'unexpected defaults: %, %', project_status, project_stage;
    END IF;
END
$$;
'@.Replace('__PROJECT_ID__', $projectId)
    Invoke-SqlAssertion "default status and current_stage" $defaultSql

    $invalidTypeSql = @'
DO $$
DECLARE rejected BOOLEAN := false;
BEGIN
    BEGIN
        INSERT INTO projects (id, name, type, created_by) VALUES ('__INVALID_TYPE_ID__', 'Invalid type', 'series', 'system');
    EXCEPTION WHEN check_violation THEN rejected := true;
    END;
    IF NOT rejected THEN RAISE EXCEPTION 'non-novel type was accepted'; END IF;
END
$$;
'@.Replace('__INVALID_TYPE_ID__', $invalidTypeId)
    Invoke-SqlAssertion "non-novel type is rejected" $invalidTypeSql

    Write-Host "[PASS] All migration database assertions completed." -ForegroundColor Green
}
finally {
    if ($projectsUp) {
        docker compose exec -T postgres psql -U acf -d acf -v ON_ERROR_STOP=1 -c $cleanupSql *> $null
        if ($LASTEXITCODE -ne 0) { throw "Migration validation cleanup failed." }
        Write-Host "[PASS] Migration validation data cleaned up." -ForegroundColor Green
    }
}


