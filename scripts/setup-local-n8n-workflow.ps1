[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$workflowName = 'ACF Iteration 15 Local Chapter Planning'
$workflowId = 'f4e32de0-15a0-4100-8000-000000000001'
$workflowPath = Join-Path $PSScriptRoot '..\infra\n8n\workflows\iteration-15-local-chapter-planning.json'
$workflowPath = [System.IO.Path]::GetFullPath($workflowPath)
$containerPath = '/tmp/iteration-15-local-chapter-planning.json'

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

$n8n = & docker compose -f compose.yml -f compose.n8n.yml ps --format json n8n | ConvertFrom-Json
if ($n8n.State -ne 'running' -or $n8n.Health -ne 'healthy') {
  throw 'The n8n container must be running and healthy.'
}

$health = Invoke-WebRequest -UseBasicParsing http://127.0.0.1:15678/healthz
if ($health.StatusCode -ne 200) {
  throw 'n8n healthz did not return HTTP 200.'
}

try {
  $workflow = Get-Content -Raw $workflowPath | ConvertFrom-Json
} catch {
  throw "Workflow JSON is invalid: $workflowPath"
}

if ($workflow.name -ne $workflowName -or $workflow.id -ne $workflowId) {
  throw 'Workflow JSON does not contain the expected fixed name and ID.'
}

& docker compose -f compose.yml -f compose.n8n.yml cp $workflowPath "n8n:$containerPath"
if ($LASTEXITCODE -ne 0) {
  throw 'Could not copy the workflow JSON into the n8n container.'
}

Invoke-N8nCli @('import:workflow', "--input=$containerPath")
Invoke-N8nCli @('update:workflow', "--id=$workflowId", '--active=true')

& docker compose -f compose.yml -f compose.n8n.yml restart n8n
if ($LASTEXITCODE -ne 0) {
  throw 'Could not restart n8n after workflow activation.'
}

for ($attempt = 1; $attempt -le 30; $attempt += 1) {
  try {
    if ((Invoke-WebRequest -UseBasicParsing http://127.0.0.1:15678/healthz).StatusCode -eq 200) {
      break
    }
  } catch {
    if ($attempt -eq 30) { throw 'n8n did not become healthy after workflow activation.' }
  }
  Start-Sleep -Seconds 1
}

$rawExport = (Invoke-N8nCli @('export:workflow', '--all') | Out-String).Trim()
$jsonStart = $rawExport.IndexOf('[')
if ($jsonStart -lt 0) {
  throw 'n8n workflow export did not return JSON.'
}
$exported = $rawExport.Substring($jsonStart) | ConvertFrom-Json
$sameName = @($exported | Where-Object { $_.name -eq $workflowName })
if ($sameName.Count -ne 1 -or $sameName[0].id -ne $workflowId -or -not $sameName[0].active) {
  throw 'The workflow import did not produce exactly one active workflow with the expected fixed ID.'
}

Write-Output "Workflow name: $workflowName"
Write-Output "Workflow ID: $workflowId"
Write-Output 'Production webhook (container): http://n8n:5678/webhook/acf-iteration-15-local-chapter-planning'
Write-Output 'Active: true'
