[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$workflows = @(
  @{ Name = 'ACF Iteration 15 Local Chapter Planning'; Id = 'f4e32de0-15a0-4100-8000-000000000001'; File = 'iteration-15-local-chapter-planning.json'; Webhook = 'acf-iteration-15-local-chapter-planning' },
  @{ Name = 'ACF Iteration 16 Local Content Generation'; Id = 'f4e32de0-16a0-4100-8000-000000000001'; File = 'iteration-16-local-content-generation.json'; Webhook = 'acf-iteration-16-local-content-generation' },
  @{ Name = 'ACF Iteration 17 Local Review'; Id = 'f4e32de0-17a0-4100-8000-000000000001'; File = 'iteration-17-local-review.json'; Webhook = 'acf-iteration-17-local-review' },
  @{ Name = 'ACF Iteration 18 Local Rewrite'; Id = 'f4e32de0-18a0-4100-8000-000000000001'; File = 'iteration-18-local-rewrite.json'; Webhook = 'acf-iteration-18-local-rewrite' }
)

function Invoke-N8nCli {
  param([Parameter(Mandatory)][string[]]$Arguments)

  & docker compose -f compose.yml -f compose.n8n.yml exec -T n8n n8n @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "n8n CLI failed: n8n $($Arguments -join ' ')"
  }
}

& docker info *> $null
if ($LASTEXITCODE -ne 0) {
  throw 'Docker is not available.'
}

$n8nContainerId = (& docker compose -f compose.yml -f compose.n8n.yml ps -q n8n).Trim()
if ([string]::IsNullOrWhiteSpace($n8nContainerId)) {
  throw 'The n8n container is not running.'
}
$n8nStatus = (& docker inspect -f "{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}" $n8nContainerId).Trim()
if ($n8nStatus -ne 'running healthy') {
  throw 'The n8n container must be running and healthy.'
}

$health = Invoke-WebRequest -UseBasicParsing http://127.0.0.1:15678/healthz
if ($health.StatusCode -ne 200) {
  throw 'n8n healthz did not return HTTP 200.'
}

$workflowRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\infra\n8n\workflows'))
foreach ($definition in $workflows) {
  $workflowPath = Join-Path $workflowRoot $definition.File
  $containerPath = "/tmp/$($definition.File)"
  try {
    $workflow = Get-Content -Raw $workflowPath | ConvertFrom-Json
  } catch {
    throw "Workflow JSON is invalid: $workflowPath"
  }

  if ($workflow.name -ne $definition.Name -or $workflow.id -ne $definition.Id) {
    throw "Workflow JSON does not contain the expected fixed name and ID: $($definition.File)"
  }

  & docker compose -f compose.yml -f compose.n8n.yml cp $workflowPath "n8n:$containerPath"
  if ($LASTEXITCODE -ne 0) {
    throw "Could not copy the workflow JSON into the n8n container: $($definition.File)"
  }

  Invoke-N8nCli @('import:workflow', "--input=$containerPath")
  Invoke-N8nCli @('update:workflow', "--id=$($definition.Id)", '--active=true')
}

& docker compose -f compose.yml -f compose.n8n.yml restart n8n
if ($LASTEXITCODE -ne 0) {
  throw 'Could not restart n8n after workflow activation.'
}

$healthy = $false
for ($attempt = 1; $attempt -le 30; $attempt += 1) {
  try {
    if ((Invoke-WebRequest -UseBasicParsing http://127.0.0.1:15678/healthz).StatusCode -eq 200) {
      $healthy = $true
      break
    }
  } catch {
    if ($attempt -eq 30) { throw 'n8n did not become healthy after workflow activation.' }
  }
  Start-Sleep -Seconds 1
}
if (-not $healthy) {
  throw 'n8n did not become healthy after workflow activation.'
}

$rawExport = (Invoke-N8nCli @('export:workflow', '--all') | Out-String).Trim()
$jsonStart = $rawExport.IndexOf('[')
if ($jsonStart -lt 0) {
  throw 'n8n workflow export did not return JSON.'
}
$exported = $rawExport.Substring($jsonStart) | ConvertFrom-Json
foreach ($definition in $workflows) {
  $sameWorkflow = @($exported | Where-Object { $_.name -eq $definition.Name })
  if ($sameWorkflow.Count -ne 1 -or $sameWorkflow[0].id -ne $definition.Id -or -not $sameWorkflow[0].active) {
    throw "The workflow import did not produce exactly one active workflow with the expected fixed ID: $($definition.Name)"
  }
  Write-Output "Workflow name: $($definition.Name)"
  Write-Output "Workflow ID: $($definition.Id)"
  Write-Output "Production webhook (container): http://n8n:5678/webhook/$($definition.Webhook)"
  Write-Output 'Active: true'
}
