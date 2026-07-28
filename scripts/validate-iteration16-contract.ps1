param([string]$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path)

$ErrorActionPreference = 'Stop'
$iteration = Join-Path $Root 'docs/development-inputs/p1/iterations/iteration-16-real-content-generation'
$openapi = Join-Path $Root 'packages/contracts/openapi/openapi.yaml'

& (Join-Path $PSScriptRoot 'validate-openapi.ps1')
if (-not $?) { throw 'Base OpenAPI validation failed.' }

$yaml = Get-Content -Raw -Encoding utf8 $openapi
$requiredPaths = @(
  '/api/v1/content-items/{contentItemId}/content-generation-runs/preflight:',
  '/api/v1/content-items/{contentItemId}/content-generation-runs:',
  '/api/v1/content-items/{contentItemId}/content-generation-summary:',
  '/api/v1/content-generation-runs/{runId}/result-consumption-retries:',
  '/api/v1/content-items/{contentItemId}/current-version:'
)
foreach ($path in $requiredPaths) { if ($yaml.IndexOf($path) -lt 0) { throw "Missing OpenAPI path: $path" } }
foreach ($term in @('subjectType:', 'subjectId:', 'workflow_generated', 'source_content_version_id:', 'source_content_version_version:', 'source_workflow_run_id:', 'result_consumed', 'result_consumption_failed')) { if ($yaml.IndexOf($term) -lt 0) { throw "Missing OpenAPI contract term: $term" } }
$stateMatch = [regex]::Match($yaml, 'ContentGenerationResultState: \{type: string, enum: \[([^\]]+)\]\}')
if (-not $stateMatch.Success) { throw 'ContentGenerationResultState not found.' }
$states = $stateMatch.Groups[1].Value.Split(',').Trim()
$expectedStates = @('idle','not_configured','queued','running','candidate_ready','runtime_failed','output_validation_failed','result_consumption_failed')
if (@($states | Where-Object { $_ -notin $expectedStates }).Count -ne 0 -or $states.Count -ne $expectedStates.Count) { throw "Unexpected Summary states: $($states -join ', ')" }

$manifest = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'ui-manifest.json') | ConvertFrom-Json
$master = Get-Content -Raw -Encoding utf8 (Join-Path $Root 'docs/development-inputs/p1/ui-master-manifest.json') | ConvertFrom-Json
Add-Type -AssemblyName System.Drawing
if ($manifest.status -ne 'frozen_cf_16_01b' -or $manifest.frameCount -ne 11 -or $manifest.frames.Count -ne 11) { throw 'Iteration 16 manifest status or frame count is invalid.' }
$ids = @($manifest.frames.frameId)
if (($ids | Select-Object -Unique).Count -ne 11) { throw 'Iteration 16 frame IDs are not unique.' }
foreach ($frame in $manifest.frames) {
  foreach ($property in @('frameId','title','route','type','trigger','screenPath','htmlPath','api','summaryStates','developmentTask','acceptance')) {
    if ($null -eq $frame.$property -or ($frame.$property -is [string] -and [string]::IsNullOrWhiteSpace($frame.$property))) { throw "Missing $property on $($frame.frameId)" }
  }
  if ($frame.route -ne '/projects/{projectId}/works/{workId}') { throw "Non-canonical route on $($frame.frameId)" }
  $screenPath = Join-Path $iteration $frame.screenPath
  $htmlPath = Join-Path $iteration $frame.htmlPath
  if (-not (Test-Path $screenPath) -or -not (Test-Path $htmlPath)) { throw "Missing prototype asset on $($frame.frameId)" }
  if ((Get-Item $screenPath).Length -eq 0 -or (Get-Item $htmlPath).Length -eq 0) { throw "Empty prototype asset on $($frame.frameId)" }
  try { $image = [System.Drawing.Image]::FromFile($screenPath); if ($image.Width -le 0 -or $image.Height -le 0) { throw 'invalid dimensions' } } finally { if ($null -ne $image) { $image.Dispose() } }
}
$masterFrames = @($master.frames | Where-Object { $_.iteration -eq 'iteration-16-real-content-generation' })
if ($masterFrames.Count -ne 11 -or (@($masterFrames.frameId | Select-Object -Unique).Count -ne 11)) { throw 'Master manifest does not contain exactly 11 unique Iteration 16 frames.' }
foreach ($frame in $manifest.frames) {
  $masterFrame = $masterFrames | Where-Object frameId -eq $frame.frameId
  if ($masterFrame.route -ne $frame.route -or $masterFrame.screen -ne $frame.screenPath -or $masterFrame.html -ne $frame.htmlPath) { throw "Master manifest mismatch: $($frame.frameId)" }
}

$traceability = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'ui-contract-traceability.md')
foreach ($id in $ids) { if (([regex]::Matches($traceability, "(?m)^\| $id \|")).Count -ne 1) { throw "Traceability row count is not one: $id" } }
foreach ($state in $expectedStates) { if ($traceability.IndexOf("``$state``") -lt 0) { throw "Traceability missing state: $state" } }
foreach ($operation in @('preflightContentGenerationRun','createContentGenerationRun','getContentGenerationSummary','retryContentGenerationResultConsumption','setCurrentContentVersion')) { if ($traceability.IndexOf($operation) -lt 0) { throw "Traceability missing operation: $operation" } }
Write-Host '[PASS] Iteration 16 API, manifest, prototype, and traceability contract validation completed.' -ForegroundColor Green
