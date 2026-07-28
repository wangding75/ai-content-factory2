[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$apiBaseUrl = 'http://127.0.0.1:18080'
$n8nBaseUrl = 'http://127.0.0.1:15678'
$n8nInternalBaseUrl = 'http://n8n:5678'
$n8nWorkflowId = 'f4e32de0-15a0-4100-8000-000000000001'
$webhookPath = 'acf-iteration-15-local-chapter-planning'
$projectId = '13a13e7e-656e-4174-bc96-c301692ebced'
$connectionName = 'Local N8N'
$workflowName = 'Local N8N Iteration 15 Chapter Planning'
$contractVersion = 'v1'
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))

function ConvertTo-CompactJson {
  param([Parameter(Mandatory)]$Value)

  return ($Value | ConvertTo-Json -Depth 30 -Compress)
}

function Get-SafeErrorSummary {
  param([AllowEmptyString()][string]$RawBody)

  if ([string]::IsNullOrWhiteSpace($RawBody)) {
    return [pscustomobject]@{ Code = 'unknown'; Message = 'No safe response body was returned.' }
  }

  try {
    $parsed = $RawBody | ConvertFrom-Json
    $code = [string]$parsed.error.code
    $message = [string]$parsed.error.message
    if ([string]::IsNullOrWhiteSpace($code)) { $code = 'unknown' }
    if ([string]::IsNullOrWhiteSpace($message)) { $message = 'No safe error message was returned.' }
    if ($message.Length -gt 300) { $message = $message.Substring(0, 300) }
    return [pscustomobject]@{ Code = $code; Message = $message }
  } catch {
    return [pscustomobject]@{ Code = 'unparseable_error'; Message = 'The response body was not a valid ErrorEnvelope.' }
  }
}

function Assert-SingleEnvelope {
  param(
    [Parameter(Mandatory)]$Envelope,
    [Parameter(Mandatory)][string]$Operation
  )

  $propertyNames = @($Envelope.PSObject.Properties.Name)
  if (
    $propertyNames.Count -ne 2 -or
    $propertyNames -notcontains 'data' -or
    $propertyNames -notcontains 'request_id' -or
    [string]::IsNullOrWhiteSpace([string]$Envelope.request_id) -or
    $null -eq $Envelope.data
  ) {
    throw "$Operation did not return the required single {data, request_id} envelope."
  }
}

function Invoke-AcfRequest {
  param(
    [Parameter(Mandatory)][ValidateSet('GET', 'POST', 'PUT', 'PATCH', 'DELETE')][string]$Method,
    [Parameter(Mandatory)][string]$Path,
    $Body,
    [AllowEmptyString()][string]$IdempotencyKey = '',
    [int[]]$AcceptedStatus = @(200)
  )

  $headers = @{}
  if (-not [string]::IsNullOrWhiteSpace($IdempotencyKey)) {
    $headers['Idempotency-Key'] = $IdempotencyKey
  }

  $arguments = @{
    Uri = $apiBaseUrl + $Path
    Method = $Method
    Headers = $headers
    TimeoutSec = 30
    UseBasicParsing = $true
  }
  if ($null -ne $Body) {
    $arguments['ContentType'] = 'application/json'
    $arguments['Body'] = ConvertTo-CompactJson $Body
  }

  try {
    $response = Invoke-WebRequest @arguments
  } catch {
    $statusCode = 'unavailable'
    if ($null -ne $_.Exception.Response) {
      try { $statusCode = [int]$_.Exception.Response.StatusCode } catch {}
    }
    $summary = Get-SafeErrorSummary ([string]$_.ErrorDetails.Message)
    throw "ACF request failed: $Method $Path; HTTP $statusCode; code=$($summary.Code); summary=$($summary.Message)"
  }

  if ($AcceptedStatus -notcontains [int]$response.StatusCode) {
    throw "ACF request returned unexpected status: $Method $Path; HTTP $($response.StatusCode)."
  }

  try {
    $envelope = $response.Content | ConvertFrom-Json
  } catch {
    throw "ACF request did not return JSON: $Method $Path; HTTP $($response.StatusCode)."
  }
  Assert-SingleEnvelope -Envelope $envelope -Operation "$Method $Path"

  return [pscustomobject]@{
    StatusCode = [int]$response.StatusCode
    Envelope = $envelope
    Data = $envelope.data
  }
}

function Invoke-N8nWebhookHealth {
  $requestId = 'acf-145-03c-production-webhook-health'
  $body = [ordered]@{
    probeType = 'acf_workflow_verification'
    stage = 'chapter_planning'
    contractVersion = $contractVersion
    requestId = $requestId
  }
  try {
    $response = Invoke-WebRequest `
      -UseBasicParsing `
      -Uri "$n8nBaseUrl/webhook/$webhookPath" `
      -Method Post `
      -ContentType 'application/json' `
      -Body (ConvertTo-CompactJson $body) `
      -TimeoutSec 30
  } catch {
    $statusCode = 'unavailable'
    if ($null -ne $_.Exception.Response) {
      try { $statusCode = [int]$_.Exception.Response.StatusCode } catch {}
    }
    throw "n8n Production Webhook health failed: HTTP $statusCode."
  }

  if ($response.StatusCode -ne 200) {
    throw "n8n Production Webhook health returned HTTP $($response.StatusCode)."
  }
  try {
    $result = $response.Content | ConvertFrom-Json
  } catch {
    throw 'n8n Production Webhook health did not return JSON.'
  }
  if (
    $result.verified -ne $true -or
    $result.stage -ne 'chapter_planning' -or
    $result.contractVersion -ne $contractVersion -or
    $result.requestId -ne $requestId
  ) {
    throw 'n8n Production Webhook health returned an invalid verification response.'
  }
}

function Invoke-N8nNode {
  param(
    [Parameter(Mandatory)][string]$JavaScript,
    [Parameter(Mandatory)][hashtable]$Environment
  )

  $arguments = @('compose', '-f', 'compose.yml', '-f', 'compose.n8n.yml', 'exec', '-T')
  foreach ($entry in $Environment.GetEnumerator()) {
    $arguments += @('-e', "$($entry.Key)=$($entry.Value)")
  }
  $arguments += @('n8n', 'node', '-e', $JavaScript)

  $previousErrorActionPreference = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try {
    $raw = & docker @arguments 2>&1
    $exitCode = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $previousErrorActionPreference
  }
  if ($exitCode -ne 0) {
    throw "Read-only n8n execution audit failed: $(([string]($raw | Out-String)).Trim())"
  }
  return ([string]($raw | Out-String)).Trim()
}

function Get-N8nExecutionSnapshot {
  $javaScript = @'
const sqlite3 = require('/usr/local/lib/node_modules/n8n/node_modules/.pnpm/sqlite3@5.1.7/node_modules/sqlite3');
const db = new sqlite3.Database('/home/node/.n8n/database.sqlite', sqlite3.OPEN_READONLY);
db.get(
  'SELECT COALESCE(MAX(id), 0) AS latestId, COUNT(*) AS count FROM execution_entity WHERE workflowId = ?',
  [process.env.ACF_WORKFLOW_ID],
  (error, row) => {
    if (error) {
      console.error(error.message);
      process.exitCode = 1;
    } else {
      console.log(JSON.stringify(row));
    }
    db.close();
  },
);
'@
  $raw = Invoke-N8nNode -JavaScript $javaScript -Environment @{ ACF_WORKFLOW_ID = $n8nWorkflowId }
  try {
    return $raw | ConvertFrom-Json
  } catch {
    throw 'Read-only n8n execution snapshot did not return JSON.'
  }
}

function Get-N8nVerificationExecution {
  param([Parameter(Mandatory)][int]$ExecutionId)

  $javaScript = @'
const sqlite3 = require('/usr/local/lib/node_modules/n8n/node_modules/.pnpm/sqlite3@5.1.7/node_modules/sqlite3');
const { parse } = require('/usr/local/lib/node_modules/n8n/node_modules/.pnpm/flatted@3.2.7/node_modules/flatted');
const db = new sqlite3.Database('/home/node/.n8n/database.sqlite', sqlite3.OPEN_READONLY);
db.get(
  `SELECT e.id, e.workflowId, e.finished, e.mode, e.status, e.startedAt, e.stoppedAt, d.data
     FROM execution_entity e
     JOIN execution_data d ON d.executionId = e.id
    WHERE e.id = ? AND e.workflowId = ?`,
  [Number(process.env.ACF_EXECUTION_ID), process.env.ACF_WORKFLOW_ID],
  (error, row) => {
    if (error || !row) {
      console.error(error ? error.message : 'execution_not_found');
      process.exitCode = 1;
      db.close();
      return;
    }
    try {
      const parsed = parse(row.data);
      const runData = parsed.resultData.runData;
      const request = runData['Production Webhook'][0].data.main[0][0].json.body;
      const response = runData['Return Runtime Output or Contract Error'][0].data.main[0][0].json;
      console.log(JSON.stringify({
        id: row.id,
        workflowId: row.workflowId,
        finished: Boolean(row.finished),
        mode: row.mode,
        status: row.status,
        startedAt: row.startedAt,
        stoppedAt: row.stoppedAt,
        request: {
          probeType: request.probeType,
          stage: request.stage,
          contractVersion: request.contractVersion,
          requestId: request.requestId,
        },
        response: {
          verified: response.verified,
          stage: response.stage,
          contractVersion: response.contractVersion,
          requestId: response.requestId,
        },
      }));
    } catch (parseError) {
      console.error(parseError.message);
      process.exitCode = 1;
    }
    db.close();
  },
);
'@
  $raw = Invoke-N8nNode -JavaScript $javaScript -Environment @{
    ACF_EXECUTION_ID = $ExecutionId
    ACF_WORKFLOW_ID = $n8nWorkflowId
  }
  try {
    return $raw | ConvertFrom-Json
  } catch {
    throw "Read-only n8n execution $ExecutionId audit did not return JSON."
  }
}

function Get-ExactNamedResource {
  param(
    [Parameter(Mandatory)][string]$Path,
    [Parameter(Mandatory)][string]$Name,
    [Parameter(Mandatory)][string]$ResourceLabel
  )

  $encodedName = [System.Uri]::EscapeDataString($Name)
  $separator = if ($Path.Contains('?')) { '&' } else { '?' }
  $response = Invoke-AcfRequest -Method GET -Path "$Path$separator`q=$encodedName&limit=100&offset=0"
  $matches = @($response.Data.items | Where-Object { $_.name -ceq $Name })
  if ($matches.Count -gt 1) {
    throw "Duplicate $ResourceLabel resources named '$Name' were found."
  }
  if ($matches.Count -eq 1) {
    return $matches[0]
  }
  return $null
}

function Assert-ConnectionTarget {
  param([Parameter(Mandatory)]$Connection)

  if (
    $Connection.name -ne $connectionName -or
    $Connection.connectionType -ne 'n8n' -or
    $Connection.baseUrl -ne $n8nInternalBaseUrl -or
    $Connection.authType -ne 'api_key' -or
    [int]$Connection.timeoutSeconds -ne 30 -or
    $Connection.typeConfig.referenceType -ne 'workflow_id' -or
    $Connection.typeConfig.referenceValue -ne $n8nWorkflowId
  ) {
    throw "The reusable Connection '$connectionName' does not match the fixed local n8n target."
  }
}

function Assert-WorkflowTarget {
  param(
    [Parameter(Mandatory)]$Workflow,
    [Parameter(Mandatory)][string]$ConnectionId
  )

  $stages = @($Workflow.applicableStages)
  if (
    $Workflow.name -ne $workflowName -or
    $Workflow.connectionId -ne $ConnectionId -or
    $Workflow.connectionType -ne 'n8n' -or
    $Workflow.workflowType -ne 'n8n' -or
    $stages.Count -ne 1 -or
    $stages[0] -ne 'chapter_planning' -or
    $Workflow.typeConfig.referenceType -ne 'webhook_path' -or
    $Workflow.typeConfig.referenceValue -ne $webhookPath -or
    $Workflow.inputContractVersion -ne $contractVersion -or
    $Workflow.outputContractVersion -ne $contractVersion
  ) {
    throw "The reusable Workflow Configuration '$workflowName' does not match the fixed local n8n target."
  }
}

function Get-ChapterPlanningBinding {
  $response = Invoke-AcfRequest -Method GET -Path "/api/v1/projects/$projectId/workflow-bindings"
  $items = @($response.Data.items)
  $matches = @($items | Where-Object { $_.stage -eq 'chapter_planning' })
  if ($items.Count -ne 4 -or $matches.Count -ne 1) {
    throw 'The project workflow binding query did not return the four fixed stages.'
  }
  return $matches[0]
}

function Get-RuntimeCounts {
  $runs = Invoke-AcfRequest -Method GET -Path "/api/v1/workflow-runs?projectId=$projectId&stage=chapter_planning&limit=1&offset=0"
  $batches = Invoke-AcfRequest -Method GET -Path "/api/v1/projects/$projectId/chapter-plan-candidate-batches?limit=1&offset=0"
  return [pscustomobject]@{
    Runs = [int]$runs.Data.total
    Batches = [int]$batches.Data.total
  }
}

Push-Location $repoRoot
try {
  & docker info *> $null
  if ($LASTEXITCODE -ne 0) {
    throw 'Docker is not available.'
  }

  $n8n = & docker compose -f compose.yml -f compose.n8n.yml ps --format json n8n | ConvertFrom-Json
  if ($n8n.State -ne 'running' -or $n8n.Health -ne 'healthy') {
    throw 'The n8n container must be running and healthy.'
  }

  $apiHealth = Invoke-AcfRequest -Method GET -Path '/healthz'
  $apiReady = Invoke-AcfRequest -Method GET -Path '/readyz'
  if ($apiHealth.Data.status -ne 'ok' -or $apiReady.Data.status -ne 'ready') {
    throw 'The ACF API health or readiness response is not healthy.'
  }

  try {
    $n8nHealth = Invoke-WebRequest -UseBasicParsing -Uri "$n8nBaseUrl/healthz" -TimeoutSec 15
  } catch {
    throw 'The n8n health endpoint is unavailable.'
  }
  if ($n8nHealth.StatusCode -ne 200) {
    throw "The n8n health endpoint returned HTTP $($n8nHealth.StatusCode)."
  }

  Invoke-N8nWebhookHealth
  $runtimeCountsBefore = Get-RuntimeCounts

  $connection = Get-ExactNamedResource `
    -Path '/api/v1/workflow-connections' `
    -Name $connectionName `
    -ResourceLabel 'Connection'
  $connectionAction = 'reused'
  if ($null -eq $connection) {
    $connectionCreateBody = [ordered]@{
      name = $connectionName
      connectionType = 'n8n'
      baseUrl = $n8nInternalBaseUrl
      authType = 'api_key'
      timeoutSeconds = 30
      typeConfig = [ordered]@{
        referenceType = 'workflow_id'
        referenceValue = $n8nWorkflowId
      }
    }
    $connectionCreate = Invoke-AcfRequest `
      -Method POST `
      -Path '/api/v1/workflow-connections' `
      -Body $connectionCreateBody `
      -IdempotencyKey 'acf-145-03c-connection-create-local-n8n' `
      -AcceptedStatus @(201)
    $connection = $connectionCreate.Data
    $connectionAction = 'created'
  }
  Assert-ConnectionTarget -Connection $connection
  $connectionId = [string]$connection.id

  $connectionBeforeVerify = (Invoke-AcfRequest -Method GET -Path "/api/v1/workflow-connections/$connectionId").Data
  Assert-ConnectionTarget -Connection $connectionBeforeVerify
  $connectionVerifyMode = 'existing state confirmed; Verify not repeated'
  $connectionVerifyFirst = [pscustomobject]@{ StatusCode = 200; Data = $connectionBeforeVerify }
  $connectionVerifyReplay = [pscustomobject]@{ StatusCode = 200; Data = $connectionBeforeVerify }
  if ($connectionBeforeVerify.integrationStatus -ne 'connected' -or $connectionBeforeVerify.enabled -ne $true) {
    $connectionVerifyMode = 'first response and same-key replay passed'
    $connectionExpectedVersion = [int]$connectionBeforeVerify.version
    $connectionVerifyKey = "acf-145-03c-connection-verify-$connectionId-v$connectionExpectedVersion"
    $connectionExecutionBefore = Get-N8nExecutionSnapshot
    $connectionVerifyBody = [ordered]@{ expectedVersion = $connectionExpectedVersion }
    $connectionVerifyFirst = Invoke-AcfRequest `
      -Method POST `
      -Path "/api/v1/workflow-connections/$connectionId/verify" `
      -Body $connectionVerifyBody `
      -IdempotencyKey $connectionVerifyKey
    $connectionVerifyReplay = Invoke-AcfRequest `
      -Method POST `
      -Path "/api/v1/workflow-connections/$connectionId/verify" `
      -Body $connectionVerifyBody `
      -IdempotencyKey $connectionVerifyKey
    $connectionExecutionAfter = Get-N8nExecutionSnapshot
    if (
      $connectionVerifyFirst.Data.integrationStatus -ne 'connected' -or
      $connectionVerifyFirst.Data.enabled -ne $true -or
      [int]$connectionVerifyFirst.Data.version -ne ($connectionExpectedVersion + 1) -or
      (ConvertTo-CompactJson $connectionVerifyFirst.Data) -ne (ConvertTo-CompactJson $connectionVerifyReplay.Data) -or
      [int]$connectionExecutionAfter.count -ne [int]$connectionExecutionBefore.count -or
      [int]$connectionExecutionAfter.latestId -ne [int]$connectionExecutionBefore.latestId
    ) {
      throw 'Connection Verify or its idempotent replay did not satisfy the lifecycle contract.'
    }
  }
  $connection = (Invoke-AcfRequest -Method GET -Path "/api/v1/workflow-connections/$connectionId").Data
  if ($connection.integrationStatus -ne 'connected' -or $connection.enabled -ne $true) {
    throw 'The queried Connection is not connected and enabled after Verify.'
  }

  $workflow = Get-ExactNamedResource `
    -Path '/api/v1/workflow-configurations' `
    -Name $workflowName `
    -ResourceLabel 'Workflow Configuration'
  $workflowAction = 'reused'
  if ($null -eq $workflow) {
    $workflowCreateBody = [ordered]@{
      name = $workflowName
      connectionId = $connectionId
      applicableStages = @('chapter_planning')
      typeConfig = [ordered]@{
        referenceType = 'webhook_path'
        referenceValue = $webhookPath
      }
      inputContractVersion = $contractVersion
      outputContractVersion = $contractVersion
      defaultParameters = [ordered]@{}
      note = 'Local deterministic n8n workflow for Iteration 15 chapter planning.'
    }
    $workflowCreate = Invoke-AcfRequest `
      -Method POST `
      -Path '/api/v1/workflow-configurations' `
      -Body $workflowCreateBody `
      -IdempotencyKey 'acf-145-03c-workflow-create-local-chapter-planning' `
      -AcceptedStatus @(201)
    $workflow = $workflowCreate.Data
    $workflowAction = 'created'
  }
  Assert-WorkflowTarget -Workflow $workflow -ConnectionId $connectionId
  $workflowId = [string]$workflow.id

  $workflowBeforeVerify = (Invoke-AcfRequest -Method GET -Path "/api/v1/workflow-configurations/$workflowId").Data
  Assert-WorkflowTarget -Workflow $workflowBeforeVerify -ConnectionId $connectionId
  $workflowVerifyMode = 'existing state confirmed; Verify not repeated'
  $workflowVerifyFirst = [pscustomobject]@{ StatusCode = 200; Data = $workflowBeforeVerify }
  $workflowVerifyReplay = [pscustomobject]@{ StatusCode = 200; Data = $workflowBeforeVerify }
  $verifyExecutionId = 'not-created-this-run'
  if ($workflowBeforeVerify.integrationStatus -ne 'connected' -or $workflowBeforeVerify.enabled -ne $true) {
    $workflowVerifyMode = 'first response and same-key replay passed'
    $workflowExpectedVersion = [int]$workflowBeforeVerify.version
    $workflowVerifyKey = "acf-145-03c-workflow-verify-$workflowId-v$workflowExpectedVersion"
    $workflowExecutionBefore = Get-N8nExecutionSnapshot
    $workflowVerifyBody = [ordered]@{ expectedVersion = $workflowExpectedVersion }
    $workflowVerifyFirst = Invoke-AcfRequest `
      -Method POST `
      -Path "/api/v1/workflow-configurations/$workflowId/verify" `
      -Body $workflowVerifyBody `
      -IdempotencyKey $workflowVerifyKey

    $workflowExecutionAfterFirst = Get-N8nExecutionSnapshot
    if (
      [int]$workflowExecutionAfterFirst.count -ne ([int]$workflowExecutionBefore.count + 1) -or
      [int]$workflowExecutionAfterFirst.latestId -le [int]$workflowExecutionBefore.latestId
    ) {
      throw 'Workflow Verify did not create exactly one new n8n execution.'
    }
    $verifyExecutionId = [int]$workflowExecutionAfterFirst.latestId
    $verifyExecution = Get-N8nVerificationExecution -ExecutionId $verifyExecutionId
    if (
      $verifyExecution.finished -ne $true -or
      $verifyExecution.mode -ne 'webhook' -or
      $verifyExecution.status -ne 'success' -or
      $verifyExecution.request.probeType -ne 'acf_workflow_verification' -or
      $verifyExecution.request.stage -ne 'chapter_planning' -or
      $verifyExecution.request.contractVersion -ne $contractVersion -or
      [string]::IsNullOrWhiteSpace([string]$verifyExecution.request.requestId) -or
      $verifyExecution.response.verified -ne $true -or
      $verifyExecution.response.stage -ne 'chapter_planning' -or
      $verifyExecution.response.contractVersion -ne $contractVersion -or
      $verifyExecution.response.requestId -ne $verifyExecution.request.requestId
    ) {
      throw "n8n execution $verifyExecutionId did not contain the required verification probe result."
    }

    $workflowVerifyReplay = Invoke-AcfRequest `
      -Method POST `
      -Path "/api/v1/workflow-configurations/$workflowId/verify" `
      -Body $workflowVerifyBody `
      -IdempotencyKey $workflowVerifyKey
    $workflowExecutionAfterReplay = Get-N8nExecutionSnapshot
    if (
      $workflowVerifyFirst.Data.integrationStatus -ne 'connected' -or
      $workflowVerifyFirst.Data.enabled -ne $true -or
      [int]$workflowVerifyFirst.Data.version -ne ($workflowExpectedVersion + 1) -or
      (ConvertTo-CompactJson $workflowVerifyFirst.Data) -ne (ConvertTo-CompactJson $workflowVerifyReplay.Data) -or
      [int]$workflowExecutionAfterReplay.count -ne [int]$workflowExecutionAfterFirst.count -or
      [int]$workflowExecutionAfterReplay.latestId -ne [int]$verifyExecutionId
    ) {
      throw 'Workflow Configuration Verify or its idempotent replay did not satisfy the lifecycle contract.'
    }
  }
  $workflow = (Invoke-AcfRequest -Method GET -Path "/api/v1/workflow-configurations/$workflowId").Data
  if ($workflow.integrationStatus -ne 'connected' -or $workflow.enabled -ne $true) {
    throw 'The queried Workflow Configuration is not connected and enabled after Verify.'
  }

  $originalBinding = Get-ChapterPlanningBinding
  $bindingBody = [ordered]@{ workflowConfigurationId = $workflowId }
  if ($originalBinding.bound -eq $true) {
    $bindingVersion = [int]$originalBinding.binding.version
    $bindingBody['expectedVersion'] = $bindingVersion
    $bindingKey = "acf-145-03c-binding-$projectId-v$bindingVersion-$workflowId"
  } else {
    $bindingKey = "acf-145-03c-binding-$projectId-create-$workflowId"
  }
  $bindingWrite = Invoke-AcfRequest `
    -Method PUT `
    -Path "/api/v1/projects/$projectId/workflow-bindings/chapter_planning" `
    -Body $bindingBody `
    -IdempotencyKey $bindingKey `
    -AcceptedStatus @(200, 201)
  if (
    $bindingWrite.Data.stage -ne 'chapter_planning' -or
    $bindingWrite.Data.bound -ne $true -or
    $bindingWrite.Data.binding.workflowConfigurationId -ne $workflowId
  ) {
    throw 'The project binding write did not return the local Workflow Configuration.'
  }

  $finalBinding = Get-ChapterPlanningBinding
  if (
    $finalBinding.bound -ne $true -or
    $finalBinding.binding.workflowConfigurationId -ne $workflowId -or
    $finalBinding.workflowConfigurationSummary.id -ne $workflowId -or
    $finalBinding.workflowConfigurationSummary.connectionId -ne $connectionId -or
    $finalBinding.workflowConfigurationSummary.integrationStatus -ne 'connected' -or
    $finalBinding.workflowConfigurationSummary.enabled -ne $true -or
    @($finalBinding.workflowConfigurationSummary.applicableStages) -notcontains 'chapter_planning'
  ) {
    throw 'The queried project binding is not the connected local chapter-planning Workflow Configuration.'
  }

  $preflightBody = [ordered]@{
    generationMode = 'range'
    target = [ordered]@{
      startChapterNo = 1
      endChapterNo = 1
    }
    storylineSelection = [ordered]@{ mode = 'auto_balanced' }
    contextOptions = [ordered]@{
      includeProjectMaterials = $true
      includeUnpaidForeshadowings = $true
      includePriorChapterSummaries = $true
      coreSettingsOnly = $false
    }
    additionalInstructions = $null
  }
  $preflightFirst = Invoke-AcfRequest `
    -Method POST `
    -Path "/api/v1/projects/$projectId/chapter-plan-runs/preflight" `
    -Body $preflightBody
  $preflightReplay = Invoke-AcfRequest `
    -Method POST `
    -Path "/api/v1/projects/$projectId/chapter-plan-runs/preflight" `
    -Body $preflightBody

  $preflightTokenPresent = -not [string]::IsNullOrWhiteSpace([string]$preflightFirst.Data.preflightToken)
  $inputDigest = [string]$preflightFirst.Data.inputDigest
  $digestStable = $inputDigest -ceq [string]$preflightReplay.Data.inputDigest
  if (
    $preflightFirst.Data.result -ne 'passed' -or
    $preflightFirst.Data.status -ne 'passed' -or
    @($preflightFirst.Data.blockers).Count -ne 0 -or
    $null -eq $preflightFirst.Data.inputSummary -or
    $null -eq $preflightFirst.Data.executionConfigurationSummary -or
    $preflightFirst.Data.executionConfigurationSummary.stage -ne 'chapter_planning' -or
    $preflightFirst.Data.executionConfigurationSummary.workflowBindingId -ne $finalBinding.binding.id -or
    [int]$preflightFirst.Data.executionConfigurationSummary.workflowBindingVersion -ne [int]$finalBinding.binding.version -or
    -not $preflightTokenPresent -or
    $inputDigest -cnotmatch '^[a-f0-9]{64}$' -or
    -not $digestStable
  ) {
    throw 'Chapter Planning Preflight did not return the required stable passed result.'
  }

  $runtimeCountsAfter = Get-RuntimeCounts
  if (
    $runtimeCountsAfter.Runs -ne $runtimeCountsBefore.Runs -or
    $runtimeCountsAfter.Batches -ne $runtimeCountsBefore.Batches
  ) {
    throw 'Runtime Run or Candidate Batch data changed; this task must not execute Create Run.'
  }

  $originalBindingSummary = if ($originalBinding.bound -eq $true) {
    "bound:$($originalBinding.binding.id):workflow:$($originalBinding.binding.workflowConfigurationId):version:$($originalBinding.binding.version)"
  } else {
    'unbound'
  }
  $finalBindingSummary = "bound:$($finalBinding.binding.id):workflow:$($finalBinding.binding.workflowConfigurationId):version:$($finalBinding.binding.version)"

  Write-Output 'STATUS: PASS'
  Write-Output 'API health: healthz=200; readyz=200'
  Write-Output 'n8n health: healthz=200; Production Webhook verification=passed'
  Write-Output "Connection action: $connectionAction"
  Write-Output "Connection ID: $connectionId"
  Write-Output "Connection Verify first: HTTP $($connectionVerifyFirst.StatusCode); connected=true; enabled=true; version=$($connectionVerifyFirst.Data.version); mode=$connectionVerifyMode"
  Write-Output "Connection Verify replay: HTTP $($connectionVerifyReplay.StatusCode); data_identical=true; version=$($connectionVerifyReplay.Data.version); workflow_execution_delta=0; mode=$connectionVerifyMode"
  Write-Output "Workflow Configuration action: $workflowAction"
  Write-Output "Workflow Configuration ID: $workflowId"
  Write-Output "Workflow Verify first: HTTP $($workflowVerifyFirst.StatusCode); connected=true; enabled=true; version=$($workflowVerifyFirst.Data.version); mode=$workflowVerifyMode"
  Write-Output "Workflow Verify replay: HTTP $($workflowVerifyReplay.StatusCode); data_identical=true; version=$($workflowVerifyReplay.Data.version); n8n_execution_delta=0; mode=$workflowVerifyMode"
  Write-Output "Verify n8n execution ID: $verifyExecutionId"
  Write-Output "Verify n8n probe: verified=true; stage=chapter_planning; contractVersion=$contractVersion; requestId_round_trip=true"
  Write-Output "Project original binding: $originalBindingSummary"
  Write-Output "Project final binding: $finalBindingSummary"
  Write-Output 'Preflight status: passed'
  Write-Output "Preflight token present: $($preflightTokenPresent.ToString().ToLowerInvariant())"
  Write-Output "Input digest: $inputDigest"
  Write-Output "Input digest stable: $($digestStable.ToString().ToLowerInvariant())"
  Write-Output "Runtime data unchanged: runs=$($runtimeCountsBefore.Runs); batches=$($runtimeCountsBefore.Batches)"
} finally {
  Pop-Location
}
