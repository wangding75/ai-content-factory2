param(
    [ValidateSet('Load','Verify','Clean','Exercise')][string]$Action = 'Verify',
    [string]$ApiBaseUrl = 'http://127.0.0.1:18080/api/v1'
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$load = Join-Path $PSScriptRoot '09-f6e-unified-p0-load.sql'
$clean = Join-Path $PSScriptRoot '09-f6e-unified-p0-clean.sql'
$verify = Join-Path $PSScriptRoot '09-f6e-unified-p0-verify.sql'
$projectId = 'f6e00000-0000-4000-8000-000000000003'
$ordinaryId = 'f6e00000-0000-4000-8009-000000000001'

function Assert-LocalTarget {
    Push-Location $root
    try {
        $config = (& docker compose config --format json | ConvertFrom-Json)
        if ($config.name -ne 'ai-content-factory2' -or -not $config.services.postgres) { throw "Refusing write: compose target is not the local ai-content-factory2 postgres service (name=$($config.name))." }
        $identity = (& docker compose exec -T postgres psql -U postgres -d ai_content_factory -Atqc "SELECT current_database() || '|' || coalesce(inet_server_addr()::text, 'local-socket')" 2>&1)
        if ($LASTEXITCODE -ne 0 -or $identity -notmatch '^ai_content_factory\|') { throw "Refusing write: expected local database ai_content_factory; actual connection result: $identity" }
        Write-Host "Target database confirmed: $identity"
    } finally { Pop-Location }
}
function Invoke-SqlFile([string]$Path) {
    Push-Location $root
    $containerPath = '/tmp/acf-f6e-test-data.sql'
    try {
        & docker compose cp $Path "postgres:$containerPath"; if ($LASTEXITCODE -ne 0) { throw "docker compose cp failed for $Path (exit $LASTEXITCODE)." }
        & docker compose exec -T postgres psql -U postgres -d ai_content_factory -v ON_ERROR_STOP=1 -f $containerPath
        if ($LASTEXITCODE -ne 0) { throw "psql failed for $Path (exit $LASTEXITCODE)." }
    } finally { & docker compose exec -T postgres rm -f $containerPath *> $null; Pop-Location }
}
function Invoke-Scalar([string]$Sql) {
    Push-Location $root
    try { $value = (& docker compose exec -T postgres psql -U postgres -d ai_content_factory -Atqc $Sql); if ($LASTEXITCODE -ne 0) { throw "psql scalar query failed (exit $LASTEXITCODE): $Sql" }; return ($value | Out-String).Trim() } finally { Pop-Location }
}
function Assert-Api {
    $paths = @(
        "/projects?limit=20&offset=0&q=F6E&status=producing",
        "/projects/$projectId/workspace",
        '/materials?scope=global&limit=20&offset=0',
        "/projects/$projectId/materials",
        "/projects/$projectId/storylines",
        "/projects/$projectId/chapter-plans?limit=20&offset=0",
        "/projects/$projectId/works?limit=20&offset=0",
        '/workflow-runs?limit=20&offset=0'
    )
    foreach ($path in $paths) {
        $result = Invoke-RestMethod -Uri ($ApiBaseUrl + $path) -Method Get -TimeoutSec 15
        if ($null -eq $result.data) { throw "API did not return an envelope data value for $path" }
    }
    $projects = Invoke-RestMethod -Uri "$ApiBaseUrl/projects?limit=20&offset=0&q=F6E%20%E6%98%9F%E6%B8%AF&status=producing"
    if ($projects.data.total -ne 1 -or $projects.data.items[0].id -ne $projectId) { throw 'API project name/status AND filtering did not return the fixed producing project.' }
    Write-Host 'API verification passed: projects, workspace, materials, storylines, chapter plans, works, and workflow runs.'
}

Assert-LocalTarget
switch ($Action) {
    'Load' { Invoke-SqlFile $load; Invoke-SqlFile $verify }
    'Verify' { Invoke-SqlFile $verify; Assert-Api }
    'Clean' { Invoke-SqlFile $clean; if ((Invoke-Scalar "SELECT count(*) FROM projects WHERE created_by='acf-test-data-f6e'") -ne '0') { throw 'Fixture cleanup left project rows.' } }
    'Exercise' {
        Invoke-SqlFile $clean
        $before = Invoke-Scalar 'SELECT count(*) FROM projects'
        Invoke-SqlFile $load; Invoke-SqlFile $verify; Assert-Api
        Invoke-SqlFile $load; Invoke-SqlFile $verify
        Invoke-SqlFile $clean
        Push-Location $root
        try { & docker compose exec -T postgres psql -U postgres -d ai_content_factory -v ON_ERROR_STOP=1 -c "INSERT INTO projects(id,name,type,status,description,current_stage,created_by) VALUES('$ordinaryId','ordinary protection probe','novel','planning','','project_setup','ordinary-user')"; if ($LASTEXITCODE -ne 0) { throw 'ordinary data insertion failed.' } } finally { Pop-Location }
        Invoke-SqlFile $load; Invoke-SqlFile $clean
        if ((Invoke-Scalar "SELECT count(*) FROM projects WHERE id='$ordinaryId'") -ne '1') { throw 'Fixture cleanup affected the ordinary protection probe.' }
        Push-Location $root
        try { & docker compose exec -T postgres psql -U postgres -d ai_content_factory -v ON_ERROR_STOP=1 -c "DELETE FROM projects WHERE id='$ordinaryId'"; if ($LASTEXITCODE -ne 0) { throw 'ordinary protection probe cleanup failed.' } } finally { Pop-Location }
        Invoke-SqlFile $load; Invoke-SqlFile $verify; Assert-Api
        $after = Invoke-Scalar 'SELECT count(*) FROM projects'
        Write-Host "Exercise passed. Baseline projects=$before; final projects=$after; ordinary data protection verified."
    }
}
