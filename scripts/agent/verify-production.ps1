param(
    [Parameter(Mandatory)][string]$TaskId,
    [string]$WebUrl = "http://127.0.0.1:13001",
    [string]$ApiHealthUrl = "http://127.0.0.1:18080/api/v1/meta",
    [ValidateRange(1, 2)][int]$MaxBuildAttempts = 2,
    [bool]$ReuseHealthy = $true,
    [switch]$PlanOnly,
    [switch]$ForceBuild,
    [string]$DockerCommand = "docker"
)

. "$PSScriptRoot\common.ps1"
Set-StrictMode -Version Latest
$started = Get-Date
$root = Get-RepositoryRoot
Set-Location $root
Assert-Command -Name $DockerCommand
$reportPath = Join-Path $root ".ai-dev\reports\$TaskId-production.json"
$statePath = Join-Path $root ".ai-dev\reports\production-fingerprint-state.json"
$commands = [Collections.Generic.List[object]]::new()
$buildAttempts = @()
$engineRecovered = $false
$result = 'failed'
$fingerprints = @{}
$state = $null
$changed = @()
$reused = @()
$recreateTargets = @()
$status = @()
$images = @{}

function Invoke-Docker {
    param([string[]]$Arguments, [string]$Label = '')
    $record = Invoke-CapturedNative -FilePath $DockerCommand -Arguments $Arguments -Label $Label
    $commands.Add($record)
    return $record
}
function Test-EngineDisconnect([object]$Record) {
    return (($Record.stderr + "`n" + $Record.stdout) -match '(?i)//\./pipe/docker_engine|file has already been closed|server: error reading preface|docker (daemon|engine).*(unavailable|not available)|error during connect')
}
function Wait-EngineRecovery {
    $delay = 1; $deadline = (Get-Date).AddSeconds(45)
    while ((Get-Date) -lt $deadline) {
        $probe = Invoke-Docker @('info') 'Docker Engine recovery probe'
        if ($probe.exit_code -eq 0) { return $true }
        Start-Sleep -Seconds $delay
        $delay = [Math]::Min($delay * 2, 8)
    }
    return $false
}
function Get-Json([object]$Record, [string]$What) {
    if ($Record.exit_code -ne 0) { throw "$What failed (exit $($Record.exit_code)): $($Record.command)`n$($Record.stderr)" }
    try { return $Record.stdout | ConvertFrom-Json } catch { throw "$What returned invalid JSON: $($_.Exception.Message)`n$($Record.stdout)" }
}
function Get-ComposeConfig { return Get-Json (Invoke-Docker @('compose','config','--format','json') 'Read Compose configuration') 'docker compose config' }
function Get-ServiceStatus {
    $record = Invoke-Docker @('compose','ps','-a','--format','json') 'Read Compose status'
    if ($record.exit_code -ne 0 -or [string]::IsNullOrWhiteSpace($record.stdout)) { return @() }
    try {
        return @($record.stdout -split "`r?`n" | Where-Object { $_.Trim() } | ForEach-Object {
            $item = $_ | ConvertFrom-Json
            # Keep only verification fields. Compose's display-only Mounts value
            # can contain a Unicode truncation marker on Windows Docker Desktop.
            [pscustomobject]@{ Service=$item.Service; State=$item.State; Health=$item.Health; ExitCode=$item.ExitCode; Image=$item.Image; Labels=$item.Labels }
        })
    } catch { return @() }
}
function Get-ServiceImageIds {
    $items = Get-ServiceStatus
    $map = @{}
    foreach ($item in $items) {
        if ($item.Service) {
            $imageId = if ($item.PSObject.Properties['ImageID']) { [string]$item.ImageID } else { '' }
            if (-not $imageId -and $item.Labels -match 'com\.docker\.compose\.image=(sha256:[0-9a-f]+)') { $imageId = $Matches[1] }
            $map[$item.Service] = $imageId
        }
    }
    return $map
}
function Test-ServiceHealthy([string]$Service, [object[]]$Status) {
    $item = @($Status | Where-Object Service -eq $Service | Select-Object -First 1)
    if (-not $item) { return $false }
    if ($Service -eq 'migrate') { return $item[0].State -eq 'exited' -and [string]$item[0].ExitCode -eq '0' }
    if ($item[0].State -ne 'running') { return $false }
    if ($Service -in @('postgres','api') -and $item[0].Health -ne 'healthy') { return $false }
    return $true
}
function Test-ProductionReady([string[]]$Required) {
    $status = Get-ServiceStatus
    foreach ($service in $Required) { if (-not (Test-ServiceHealthy -Service $service -Status $status)) { return $false } }
    return ((Test-HttpOk -Url $WebUrl -TimeoutSec 5) -and ((-not $ApiHealthUrl) -or (Test-HttpOk -Url $ApiHealthUrl -TimeoutSec 5)))
}
function Test-FingerprintExcluded([string]$Relative) {
    $p = $Relative.Replace('\','/')
    return $p -match '(^|/)(\.git|\.next|node_modules|coverage|playwright-report|test-results|reports|logs|tmp|temp|\.ai-dev|\.agents|docs|tasks)(/|$)' -or $p -match '\.(log|tmp|tsbuildinfo)$'
}
function Get-RelativeFilePath([string]$Base, [string]$Path) {
    $basePath = ([IO.Path]::GetFullPath($Base)).TrimEnd('\','/')
    $fullPath = [IO.Path]::GetFullPath($Path)
    if ($fullPath.StartsWith($basePath, [StringComparison]::OrdinalIgnoreCase)) {
        return $fullPath.Substring($basePath.Length).TrimStart('\','/')
    }
    return $fullPath
}
function Get-FileHashLines([string]$Context, [string]$Dockerfile, [string]$Service) {
    # Dockerfiles currently copy all source from their contexts.  Agent tooling is
    # excluded deliberately: it cannot affect either runtime image and prevents a
    # verification-script edit from invalidating Web's root build context.
    # Do not recursively enumerate ignored dependency trees (notably
    # node_modules). Git's file set includes tracked and relevant untracked
    # inputs while respecting the repository ignore contract.
    $files = @(& git ls-files --cached --others --exclude-standard) | ForEach-Object {
        $full = Join-Path $root $_
        if (Test-Path -LiteralPath $full -PathType Leaf) { Get-Item -LiteralPath $full }
    } | Where-Object {
        $relative = (Get-RelativeFilePath $Context $_.FullName).Replace('\','/')
        -not $relative.StartsWith('..') -and -not (Test-FingerprintExcluded $relative) -and $relative -notmatch '^(scripts/agent|docs/agent)/'
    }
    foreach ($file in $files | Sort-Object FullName) {
        $relative = (Get-RelativeFilePath $Context $file.FullName).Replace('\','/')
        "$relative`t$((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash)"
    }
    # Include resolved Dockerfile even when it is outside the context and include
    # the effective service build configuration (args, target, labels, etc.).
    "__dockerfile__`t$((Get-FileHash -LiteralPath $Dockerfile -Algorithm SHA256).Hash)"
}
function Get-Fingerprints($Config) {
    $values = @{}
    foreach ($property in $Config.services.psobject.Properties) {
        $service = $property.Name; $buildProperty = $property.Value.PSObject.Properties['build']; $build = if ($buildProperty) { $buildProperty.Value } else { $null }
        if ($null -eq $build) { continue }
        if ($build -is [string]) { $context = $build; $dockerfile = 'Dockerfile'; $buildJson = $build } else {
            $context = [string]$build.context; $dockerfile = [string]$build.dockerfile
            # Compose may reorder object properties between invocations. Hash a
            # canonical, sorted representation of all effective build options.
            $pairs = foreach ($item in $build.PSObject.Properties | Sort-Object Name) { "$($item.Name)=$($item.Value | ConvertTo-Json -Depth 20 -Compress)" }
            $buildJson = $pairs -join ';'
        }
        if (-not [IO.Path]::IsPathRooted($context)) { $context = Join-Path $root $context }
        if (-not [IO.Path]::IsPathRooted($dockerfile)) { $dockerfile = Join-Path $context $dockerfile }
        $lines = @("service=$service", "build=$buildJson") + @(Get-FileHashLines $context $dockerfile $service)
        $bytes = [Text.Encoding]::UTF8.GetBytes(($lines -join "`n"))
        $sha = [Security.Cryptography.SHA256]::Create()
        $values[$service] = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','').ToLowerInvariant()
        $sha.Dispose()
    }
    return $values
}
function Read-State {
    if (-not (Test-Path -LiteralPath $statePath)) { return $null }
    try { $state = Get-Content -Raw -LiteralPath $statePath | ConvertFrom-Json; if (-not $state.fingerprints) { return $null }; return $state } catch { Write-Host '[WARN] Fingerprint state is missing or corrupt; safely rebuilding affected services.' -ForegroundColor Yellow; return $null }
}
function Get-DependentClosure($Config, [string[]]$Seeds) {
    $result = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($seed in $Seeds) { [void]$result.Add($seed) }
    $changed = $true
    while ($changed) { $changed = $false; foreach ($property in $Config.services.psobject.Properties) {
        $dependsProperty = $property.Value.PSObject.Properties['depends_on']; $depends = if ($dependsProperty) { $dependsProperty.Value } else { $null }
        if ($depends) { foreach ($dependency in $depends.psobject.Properties.Name) { if ($result.Contains($dependency) -and $result.Add($property.Name)) { $changed = $true } }
        }
    } }
    return @($result | Sort-Object)
}
function Invoke-TargetBuild([string[]]$Targets, [int]$Number) {
    if ($Targets.Count -eq 0) { return $true }
    $record = Invoke-Docker (@('compose','build') + $Targets) "Build attempt ${Number}: $($Targets -join ', ')"
    $buildAttempts += [ordered]@{ number=$Number; services=$Targets; command=$record.command; exit_code=$record.exit_code; stderr=$record.stderr }
    return $record.exit_code -eq 0
}

try {
    if (-not (Wait-EngineRecovery)) { throw 'Docker Engine did not become available within 45 seconds.' }
    $config = Get-ComposeConfig
    $fingerprints = Get-Fingerprints $config
    $state = Read-State
    $status = Get-ServiceStatus
    $buildServices = @($fingerprints.Keys | Sort-Object)
    $changed = @()
    if ($ForceBuild -or -not $state) { $changed = $buildServices } else { foreach ($service in $buildServices) { if ($state.fingerprints.$service -ne $fingerprints[$service]) { $changed += $service } } }
    $required = @('postgres','migrate','api','web') | Where-Object { $null -ne $config.services.PSObject.Properties[$_] }
    $unhealthy = @($required | Where-Object { -not (Test-ServiceHealthy -Service $_ -Status $status) })
    # A changed image is built by its own service name. Runtime dependents are
    # started only when necessary; they are never rebuilt merely because API or
    # migrate changed.
    $buildTargets = @($changed | Where-Object { $_ -in $buildServices } | Sort-Object -Unique)
    $recreateTargets = @(Get-DependentClosure $config $unhealthy | Where-Object { $_ -notin $buildTargets })
    $reused = @($required | Where-Object { $_ -notin $buildTargets -and $_ -notin $recreateTargets })
    Write-Host "[DECISION] current fingerprints: $($fingerprints | ConvertTo-Json -Compress)"
    Write-Host "[DECISION] previous fingerprints: $($(if ($state) { $state.fingerprints | ConvertTo-Json -Compress } else { '<none>' }))"
    Write-Host "[DECISION] changed services: $($changed -join ', ')"
    Write-Host "[DECISION] reused services: $($reused -join ', ')"
    Write-Host "[DECISION] recreated services: $($recreateTargets -join ', ')"
    if ($PlanOnly) {
        $result = 'planned'
    }
    if (-not $PlanOnly -and $buildTargets.Count -gt 0) {
        $beforeIds = Get-ServiceImageIds
        $ok = Invoke-TargetBuild $buildTargets 1
        if (-not $ok) {
            $first = $commands[$commands.Count - 1]
            if (-not (Test-EngineDisconnect $first)) { throw "Targeted Docker build failed (exit $($first.exit_code)): $($first.command)`n$($first.stderr)" }
            $engineRecovered = Wait-EngineRecovery
            if (-not $engineRecovered) { throw "Docker Engine did not recover after: $($first.command)`n$($first.stderr)" }
            $afterIds = Get-ServiceImageIds
            $imageChanged = @($buildTargets | Where-Object { $beforeIds[$_] -ne $afterIds[$_] -and $afterIds[$_] }).Count -gt 0
            if (-not $imageChanged -and $MaxBuildAttempts -ge 2) {
                $ok = Invoke-TargetBuild $buildTargets 2
                if (-not $ok) { $second = $commands[$commands.Count - 1]; throw "Targeted Docker build failed twice. First: $($first.command) exit $($first.exit_code): $($first.stderr)`nSecond: $($second.command) exit $($second.exit_code): $($second.stderr)" }
            }
        }
        $up = Invoke-Docker (@('compose','up','-d','--no-build','--remove-orphans') + $buildTargets) 'Start rebuilt services'
        if ($up.exit_code -ne 0) { throw "Docker Compose start failed (exit $($up.exit_code)): $($up.command)`n$($up.stderr)" }
    }
    if (-not $PlanOnly -and $recreateTargets.Count -gt 0) {
        $up = Invoke-Docker (@('compose','up','-d','--no-build','--force-recreate','--remove-orphans') + $recreateTargets) 'Recreate unhealthy services'
        if ($up.exit_code -ne 0) { throw "Docker Compose recreate failed (exit $($up.exit_code)): $($up.command)`n$($up.stderr)" }
    }
    if (-not $PlanOnly) {
        $deadline = (Get-Date).AddSeconds(120)
        while ((Get-Date) -lt $deadline -and -not (Test-ProductionReady $required)) { Start-Sleep -Seconds 3 }
        if (-not (Test-ProductionReady $required)) {
            $logs = Invoke-Docker @('compose','logs','--since=15m','migrate','api','web') 'Failure logs'
            throw "Production services did not become reachable. $($logs.command) exit $($logs.exit_code)`n$($logs.stderr)" }
        $images = Get-ServiceImageIds
        Write-JsonUtf8NoBom -Path $statePath -Value ([ordered]@{ version=1; successful_at=(Get-Date).ToString('o'); fingerprints=$fingerprints; image_ids=$images })
        $result = 'passed'
    }
}
finally {
    $finished = Get-Date
    Write-JsonUtf8NoBom -Path $reportPath -Value ([ordered]@{ task=$TaskId; stage='verify-production'; result=$result; current_fingerprints=$fingerprints; previous_fingerprints=$(if ($state) { $state.fingerprints } else { $null }); changed_services=$changed; reused_services=$reused; recreated_services=$recreateTargets; build_attempts=$buildAttempts; engine_recovered=$engineRecovered; final_image_ids=$(if ($images) { $images } else { @{} }); final_health=$(if ($status) { $status } else { @() }); commands=@($commands | ForEach-Object { ConvertTo-CommandReport $_ }); started_at=$started.ToString('o'); finished_at=$finished.ToString('o') })
}
if ($result -eq 'planned') { Write-Host '[PLAN] Production decision completed.' -ForegroundColor Yellow } else { Write-Host '[PASS] Production verification completed.' -ForegroundColor Green }
