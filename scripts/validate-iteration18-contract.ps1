param([string]$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path)

$ErrorActionPreference = 'Stop'
$iteration = Join-Path $Root 'docs/development-inputs/p1/iterations/iteration-18-real-content-rewrite'
$openApiPath = Join-Path $Root 'packages/contracts/openapi/openapi.yaml'
$apiScopePath = Join-Path $iteration 'api-scope.yaml'
$tracePath = Join-Path $iteration 'ui-contract-traceability.md'
$manifestPath = Join-Path $iteration 'ui-manifest.json'
$dataModelPath = Join-Path $iteration 'data-model.md'
$transactionPath = Join-Path $iteration 'transaction-and-migration-design.md'
$migrationUpPath = Join-Path $Root 'apps/api/migrations/000018_real_content_rewrite_foundation.up.sql'
$migrationDownPath = Join-Path $Root 'apps/api/migrations/000018_real_content_rewrite_foundation.down.sql'
$migration15Path = Join-Path $Root 'apps/api/migrations/000015_content_generation_data_foundation.up.sql'

function Assert-Contract([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

function Get-OpenApiPathBlock([string]$Path) {
    $pattern = '(?ms)^  ' + [regex]::Escape($Path) + ':\r?\n(.*?)(?=^  /api/v1/|\z)'
    $match = [regex]::Match($script:openApi, $pattern)
    Assert-Contract $match.Success "Missing OpenAPI path: $Path"
    return $match.Value
}

function Get-OpenApiSchemaBlock([string]$Name) {
    $pattern = '(?ms)^    ' + [regex]::Escape($Name) + ':\r?\n(.*?)(?=^    [A-Za-z][A-Za-z0-9]+:\r?\n|\z)'
    $match = [regex]::Match($script:openApi, $pattern)
    Assert-Contract $match.Success "Missing OpenAPI Schema: $Name"
    return $match.Value
}

& (Join-Path $PSScriptRoot 'validate-openapi.ps1')
if (-not $?) { throw 'Base OpenAPI validation failed.' }

$openApi = Get-Content -Raw -Encoding utf8 $openApiPath
$apiScope = Get-Content -Raw -Encoding utf8 $apiScopePath
$trace = Get-Content -Raw -Encoding utf8 $tracePath
$business = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'business-rules.md')
$closedLoop = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'closed-loop.md')
$acceptance = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'acceptance.md')
$iterationPlan = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'iteration-plan.md')
$developmentPlan = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'development-plan.md')
$dataModel = Get-Content -Raw -Encoding utf8 $dataModelPath
$transaction = Get-Content -Raw -Encoding utf8 $transactionPath
$migrationUp = Get-Content -Raw -Encoding utf8 $migrationUpPath
$migrationDown = Get-Content -Raw -Encoding utf8 $migrationDownPath
$migration15 = Get-Content -Raw -Encoding utf8 $migration15Path
$manifest = Get-Content -Raw -Encoding utf8 $manifestPath | ConvertFrom-Json

$operations = @(
    @{method='get'; operationId='getContentRewriteAvailability'; path='/api/v1/reviews/{reviewId}/rewrite-availability'},
    @{method='post'; operationId='preflightContentRewrite'; path='/api/v1/reviews/{reviewId}/rewrites/preflight'},
    @{method='post'; operationId='createContentRewriteRun'; path='/api/v1/reviews/{reviewId}/rewrites'},
    @{method='get'; operationId='getContentRewriteSummary'; path='/api/v1/reviews/{reviewId}/rewrite-summary'},
    @{method='get'; operationId='listContentRewriteHistory'; path='/api/v1/content-items/{contentItemId}/rewrite-history'},
    @{method='get'; operationId='getContentRewriteResult'; path='/api/v1/workflow-runs/{workflowRunId}/rewrite-result'},
    @{method='post'; operationId='retryContentRewriteResultConsumption'; path='/api/v1/workflow-runs/{workflowRunId}/rewrite-result-consumption-retries'}
)

foreach ($operation in $operations) {
    $block = Get-OpenApiPathBlock $operation.path
    Assert-Contract ($block -match "(?m)^    $($operation.method):\r?$") "Wrong or missing method for $($operation.path)"
    Assert-Contract ($block.Contains("operationId: $($operation.operationId)")) "Missing operationId $($operation.operationId)"
    Assert-Contract ($apiScope.Contains("operationId: $($operation.operationId)")) "api-scope.yaml is missing $($operation.operationId)"
    Assert-Contract ($apiScope.Contains("path: $($operation.path)")) "api-scope.yaml path mismatch: $($operation.path)"
}

foreach ($commandPath in @(
    '/api/v1/reviews/{reviewId}/rewrites',
    '/api/v1/workflow-runs/{workflowRunId}/rewrite-result-consumption-retries'
)) {
    Assert-Contract ((Get-OpenApiPathBlock $commandPath).Contains('#/components/parameters/IdempotencyKey')) "Command path is missing Idempotency-Key: $commandPath"
}

$createBlock = Get-OpenApiPathBlock '/api/v1/reviews/{reviewId}/rewrites'
Assert-Contract ($createBlock.Contains('"201"') -and $createBlock.Contains('"200"')) 'Create Rewrite Run must return 201 first and 200 on replay.'
Assert-Contract ($createBlock.Contains('stage=rewrite') -and $createBlock.Contains('subjectType=review_report')) 'Create Rewrite Run server-fixed Stage/Subject is missing.'
Assert-Contract ($openApi -match '(?m)^    ApplicableStage:\r?\n      type: string\r?\n      enum: \[chapter_planning, content_generation, review, rewrite\]$') 'ApplicableStage does not contain the unique rewrite Stage.'

$requiredSchemas = @(
    'ContentRewriteAvailability',
    'ContentRewritePreflightRequest',
    'ContentRewritePreflightReport',
    'CreateContentRewriteRunRequest',
    'RewriteRuntimeInputV1',
    'RewriteRuntimeOutputV1',
    'ContentRewriteSummary',
    'ContentRewriteHistoryItem',
    'ContentRewriteResult',
    'RetryContentRewriteResultConsumptionRequest',
    'ContentRewriteErrorCode'
)
foreach ($schema in $requiredSchemas) { [void](Get-OpenApiSchemaBlock $schema) }

$input = Get-OpenApiSchemaBlock 'RewriteRuntimeInputV1'
$inputRequired = 'required: [schemaVersion, workflowRunId, correlationId, projectId, contentItemId, sourceContentVersionId, sourceContentVersionVersion, sourceContentHash, sourceTitle, sourceContent, reviewReportId, reportSnapshot, selectedIssues, optionalInstructions, rewriteOptions]'
Assert-Contract ($input.Contains($inputRequired)) 'rewrite.input.v1 required fields or canonical order changed.'
Assert-Contract ($input.Contains('const: rewrite.input.v1')) 'rewrite.input.v1 schemaVersion is not frozen.'
Assert-Contract ($input.Contains('minItems: 1, maxItems: 50') -or ($input.Contains('minItems: 1') -and $input.Contains('maxItems: 50'))) 'rewrite.input.v1 selected Issue bounds are missing.'

$output = Get-OpenApiSchemaBlock 'RewriteRuntimeOutputV1'
$outputRequired = 'required: [schemaVersion, title, content, summary, addressedIssues, unresolvedIssues, warnings, metadata]'
Assert-Contract ($output.Contains($outputRequired)) 'rewrite.output.v1 required fields changed.'
Assert-Contract ($output.Contains('const: rewrite.output.v1')) 'rewrite.output.v1 schemaVersion is not frozen.'
foreach ($term in @('single strict JSON object with no trailing JSON', 'Unknown fields', 'partition all selected Issue IDs exactly once', 'Candidate-ID')) {
    Assert-Contract ($output.Contains($term)) "rewrite.output.v1 semantic validation is missing: $term"
}
foreach ($field in @('addressedIssues', 'unresolvedIssues', 'warnings', 'metadata')) {
    Assert-Contract ($output.Contains("$field`:")) "rewrite.output.v1 is missing $field"
}

$stateMatch = [regex]::Match($openApi, 'ContentRewriteResultState:\r?\n      type: string\r?\n      enum: \[([^\]]+)\]')
Assert-Contract $stateMatch.Success 'ContentRewriteResultState is missing.'
$states = @($stateMatch.Groups[1].Value.Split(',') | ForEach-Object { $_.Trim() })
$expectedStates = @('idle','not_configured','queued','running','candidate_ready','runtime_failed','output_validation_failed','result_consumption_failed')
Assert-Contract ($states.Count -eq 8 -and (Compare-Object $states $expectedStates).Count -eq 0) "Unexpected Rewrite Summary states: $($states -join ', ')"
Assert-Contract ($business.Contains('active Run') -and $business.Contains('not_configured/idle')) 'Summary state priority is not frozen.'
Assert-Contract ($business.Contains('candidateIsCurrent=true') -and $business.Contains('canSetCurrent=false')) 'Post-Set-Current Summary semantics are missing.'

$retryBlock = Get-OpenApiPathBlock '/api/v1/workflow-runs/{runId}/retries'
foreach ($term in @('failed, cancelled or output_validation_failed', 'inputOverride is forbidden', 'result_consumption_failed', 'retryOfRunId')) {
    Assert-Contract ($retryBlock.Contains($term)) "Runtime Retry rule is missing: $term"
}
Assert-Contract ($retryBlock.Contains('"201"') -and $retryBlock.Contains('"200"')) 'Runtime Retry must return 201 first and 200 on replay.'
$consumptionRetryBlock = Get-OpenApiPathBlock '/api/v1/workflow-runs/{workflowRunId}/rewrite-result-consumption-retries'
foreach ($term in @('never calls Runtime or n8n', 'creates no new Run')) {
    Assert-Contract ($consumptionRetryBlock.Contains($term)) "Result Consumption Retry rule is missing: $term"
}
$consumptionRetryRequest = Get-OpenApiSchemaBlock 'RetryContentRewriteResultConsumptionRequest'
Assert-Contract ($consumptionRetryRequest.Contains('required: [expectedRunVersion]')) 'Result Consumption Retry expectedRunVersion is missing.'

$setCurrentBlock = Get-OpenApiPathBlock '/api/v1/content-items/{contentItemId}/current-version'
foreach ($term in @('workflow_rewrite', 'expectedCurrentVersionId', 'expectedCurrentVersion', 'Candidate already current returns 200', 'idempotency_conflict')) {
    Assert-Contract ($setCurrentBlock.Contains($term)) "Set Current rule is missing: $term"
}
$setCurrentRequest = Get-OpenApiSchemaBlock 'SetCurrentContentVersionRequest'
Assert-Contract ($setCurrentRequest.Contains('required: [candidateVersionId, expectedCurrentVersionId, expectedCurrentVersion]')) 'Set Current CAS request changed.'

$errorBlock = Get-OpenApiSchemaBlock 'ContentRewriteErrorCode'
$errorMatch = [regex]::Match($errorBlock, 'enum: \[([^\]]+)\]')
Assert-Contract $errorMatch.Success 'ContentRewriteErrorCode enum is missing.'
$actualErrors = @($errorMatch.Groups[1].Value.Split(',') | ForEach-Object { $_.Trim() })
$expectedErrors = @(
    'review_not_found',
    'review_issue_not_found',
    'rewrite_not_available',
    'rewrite_not_configured',
    'rewrite_preflight_expired',
    'rewrite_preflight_stale',
    'rewrite_preflight_consumed',
    'active_rewrite_run_conflict',
    'rewrite_candidate_not_found',
    'rewrite_candidate_not_ready',
    'rewrite_output_validation_failed',
    'rewrite_result_consumption_failed',
    'content_version_conflict',
    'idempotency_conflict',
    'workflow_run_not_found',
    'workflow_run_version_conflict',
    'internal_error'
)
Assert-Contract ($actualErrors.Count -eq $expectedErrors.Count -and (Compare-Object $actualErrors $expectedErrors).Count -eq 0) "Unexpected Rewrite error codes: $($actualErrors -join ', ')"

Assert-Contract ($apiScope.Contains('runtime_stage: rewrite')) 'api-scope.yaml does not freeze stage=rewrite.'
Assert-Contract ($apiScope.Contains('input_contract: rewrite.input.v1') -and $apiScope.Contains('output_contract: rewrite.output.v1')) 'api-scope.yaml Runtime contracts are missing.'
Assert-Contract ($apiScope.Contains('minimum: 1') -and $apiScope.Contains('maximum: 50') -and $apiScope.Contains('empty_selection: forbidden')) 'api-scope.yaml Issue selection is incomplete.'
foreach ($state in $expectedStates) {
    Assert-Contract ($apiScope.Contains("  - $state")) "api-scope.yaml is missing Summary state: $state"
}
Assert-Contract (-not ($apiScope + $business + $closedLoop + $acceptance + $iterationPlan + $developmentPlan + $trace).Contains('content_rewrite')) 'Deprecated content_rewrite Stage remains in CF-18-01A contract documents.'
Assert-Contract ($developmentPlan.Contains('CF-18-01B') -and $developmentPlan.Contains('CF-18-01A')) 'CF-18-01B boundary is not explicit.'

$expectedFrames = @(
    'I18_D2_REVIEW_REWRITE_ENTRY',
    'I18_D4_REWRITE_AVAILABILITY',
    'I18_D4_CREATE_REWRITE',
    'I18_D4_REWRITE_CONFIG_DRAWER',
    'I18_D4_REWRITE_RUNNING',
    'I18_D4_REWRITE_FAILED',
    'I18_D5_RESULT_CONSUMPTION_FAILED',
    'I18_D5_REWRITE_RESULT',
    'I18_D5_SET_CURRENT_CONFIRM'
)
Assert-Contract ($manifest.frameCount -eq 9 -and $manifest.frames.Count -eq 9) 'Iteration 18 prototype manifest must contain exactly 9 Frames.'
foreach ($frameId in $expectedFrames) {
    Assert-Contract ($trace.Contains("``$frameId``")) "UI traceability is missing Frame: $frameId"
    $frame = @($manifest.frames | Where-Object { $_.frameId -eq $frameId })
    Assert-Contract ($frame.Count -eq 1) "Prototype manifest mismatch for Frame: $frameId"
    $screen = Join-Path $iteration $frame[0].screenPath
    $html = Join-Path $iteration $frame[0].htmlPath
    Assert-Contract (Test-Path -LiteralPath $screen -PathType Leaf) "Missing screen.png: $frameId"
    Assert-Contract (Test-Path -LiteralPath $html -PathType Leaf) "Missing code.html: $frameId"
    Assert-Contract ((Get-Item -LiteralPath $html).Length -gt 0) "Empty code.html: $frameId"
}
Assert-Contract ($trace.Contains('| Frame |') -and ([regex]::Matches($trace, '(?m)^\| `I18_').Count -eq 9)) 'UI traceability must contain the 9-Frame contract table.'

Assert-Contract ($dataModel.Contains('frozen_cf_18_01b') -and $transaction.Contains('frozen_cf_18_01b')) 'CF-18-01B data and transaction documents are not frozen.'
foreach ($table in @(
    'content_items',
    'content_versions',
    'workflow_run_records',
    'workflow_run_events',
    'idempotency_records',
    'review_reports',
    'review_findings'
)) {
    Assert-Contract ($dataModel.Contains($table)) "CF-18-01B data model does not reuse $table."
}
Assert-Contract (-not (($dataModel + $transaction + $migrationUp) -match '(?i)stage\s*=?\s*[''`"]?content_rewrite')) 'Deprecated content_rewrite Stage remains in CF-18-01B.'
Assert-Contract (-not ($migrationUp -match '(?im)^\s*CREATE\s+TABLE\b')) 'Migration 18 must not create a parallel table.'
Assert-Contract (-not ($migrationUp -match '(?im)^\s*ALTER\s+TABLE[\s\S]*?\bADD\s+COLUMN\b')) 'Migration 18 must not add an unproven column.'
Assert-Contract (-not ($migrationUp -match '(?i)CREATE\s+TABLE\s+rewrite_source_issue_links')) 'Forbidden Rewrite Issue link table remains in Migration 18.'
foreach ($term in @(
    "'workflow_rewrite'",
    'content_versions_workflow_rewrite_shape',
    'workflow_run_records_rewrite_subject_shape_check',
    'workflow_run_records_active_rewrite_subject_idx',
    'workflow_run_records_rewrite_scope_trigger',
    'content_versions_workflow_rewrite_scope_trigger',
    "'rewrite.input.v1'",
    "'rewrite.output.v1'"
)) {
    Assert-Contract ($migrationUp.Contains($term)) "Migration 18 is missing: $term"
}
Assert-Contract ($migration15.Contains('content_versions_source_workflow_run_unique_idx')) 'Existing same-Run Candidate unique index is missing from Migration 15.'
Assert-Contract ($dataModel.Contains('input_payload.selectedIssues') -and $dataModel.Contains('content_versions_source_workflow_run_unique_idx')) 'Selected Issue persistence or Candidate uniqueness is not frozen.'
foreach ($section in @(
    '## 3. Preflight',
    'Create Rewrite Run',
    '## 5. Rewrite',
    '### 6.1 Runtime Retry',
    '### 6.2 Result Consumption Retry',
    'Set Current / CAS',
    '## 2.',
    'Event'
)) {
    Assert-Contract ($transaction.Contains($section)) "CF-18-01B transaction boundary is missing: $section"
}
$runtimeRetryMatch = [regex]::Match($transaction, '(?ms)^### 6\.1 Runtime Retry\r?\n(.*?)(?=^### 6\.2 Result Consumption Retry)')
Assert-Contract $runtimeRetryMatch.Success 'Runtime Retry transaction section cannot be located.'
$runtimeRetry = $runtimeRetryMatch.Groups[1].Value
$runtimeRetryDiscoveryRead = $runtimeRetry.IndexOf('normal consistent read')
$runtimeRetryContentItemAdvisory = $runtimeRetry.IndexOf('ContentItem advisory lock')
$runtimeRetryActiveSubjectAdvisory = $runtimeRetry.IndexOf('active subject advisory lock')
$runtimeRetryRunRowLock = $runtimeRetry.IndexOf('`FOR UPDATE`')
Assert-Contract ($runtimeRetryDiscoveryRead -ge 0) 'Runtime Retry must retain its normal consistent discovery read.'
Assert-Contract ($runtimeRetryContentItemAdvisory -ge 0 -and $runtimeRetryActiveSubjectAdvisory -ge 0 -and $runtimeRetryRunRowLock -ge 0) 'Runtime Retry lock-order terms are missing.'
Assert-Contract ($runtimeRetryDiscoveryRead -lt $runtimeRetryContentItemAdvisory -and $runtimeRetryContentItemAdvisory -lt $runtimeRetryActiveSubjectAdvisory -and $runtimeRetryActiveSubjectAdvisory -lt $runtimeRetryRunRowLock) 'Runtime Retry must acquire ContentItem and active subject advisory locks before WorkflowRun FOR UPDATE.'
Assert-Contract (-not ($runtimeRetry -match '(?s)`FOR UPDATE`.*(?:ContentItem advisory lock|active subject advisory lock)')) 'Runtime Retry retains the obsolete WorkflowRun-before-advisory-lock order.'
foreach ($term in @('SERIALIZABLE', 'FOR UPDATE', 'advisory lock', 'Runtime/n8n', 'candidateIsCurrent=true')) {
    Assert-Contract (($dataModel + $transaction).Contains($term)) "CF-18-01B atomicity or lock rule is missing: $term"
}
foreach ($term in @(
    'content_versions_workflow_rewrite_scope_trigger',
    'workflow_run_records_rewrite_scope_trigger',
    'workflow_run_records_active_rewrite_subject_idx',
    'content_versions_workflow_rewrite_shape',
    'workflow_run_records_rewrite_subject_shape_check'
)) {
    Assert-Contract ($migrationDown.Contains($term)) "Migration 18 down file does not remove its object: $term"
}
Assert-Contract (-not ($migrationDown -match '(?im)^\s*DROP\s+TABLE\b')) 'Migration 18 down must not drop an existing table.'

$allowedChanges = @(
    'apps/api/migrations/000018_real_content_rewrite_foundation.up.sql',
    'apps/api/migrations/000018_real_content_rewrite_foundation.down.sql',
    'docs/development-inputs/p1/iterations/iteration-18-real-content-rewrite/data-model.md',
    'docs/development-inputs/p1/iterations/iteration-18-real-content-rewrite/transaction-and-migration-design.md',
    'docs/development-inputs/p1/iterations/iteration-18-real-content-rewrite/development-plan.md',
    'docs/development-inputs/p1/iterations/iteration-18-real-content-rewrite/iteration-plan.md',
    'scripts/validate-iteration18-contract.ps1'
)
$trackedChanges = @(& git -C $Root diff --name-only)
Assert-Contract ($LASTEXITCODE -eq 0) 'Unable to inspect tracked file changes.'
$untrackedChanges = @(& git -C $Root ls-files --others --exclude-standard)
Assert-Contract ($LASTEXITCODE -eq 0) 'Unable to inspect untracked file changes.'
$allChanges = @($trackedChanges + $untrackedChanges | ForEach-Object { $_.Replace('\', '/') } | Sort-Object -Unique)
foreach ($changedPath in $allChanges) {
    Assert-Contract ($allowedChanges -contains $changedPath) "Protected or out-of-scope file changed: $changedPath"
}
$historicalMigrationChanges = @($allChanges | Where-Object {
    $_ -match '^apps/api/migrations/0000(0[1-9]|1[0-7])_'
})
Assert-Contract ($historicalMigrationChanges.Count -eq 0) "Migration 1-17 changed: $($historicalMigrationChanges -join ', ')"

Write-Host '[PASS] Iteration 18 business/API and CF-18-01B data model, transaction, lock, idempotency, CAS, Migration 18 and protected-scope validation completed.' -ForegroundColor Green
