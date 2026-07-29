param([string]$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path)

$ErrorActionPreference = 'Stop'
$iteration = Join-Path $Root 'docs/development-inputs/p1/iterations/iteration-17-real-content-review'
$openApiPath = Join-Path $Root 'packages/contracts/openapi/openapi.yaml'
$migrationUpPath = Join-Path $Root 'apps/api/migrations/000017_real_content_review_foundation.up.sql'
$migrationDownPath = Join-Path $Root 'apps/api/migrations/000017_real_content_review_foundation.down.sql'

& (Join-Path $PSScriptRoot 'validate-openapi.ps1')
if (-not $?) { throw 'Base OpenAPI validation failed.' }

$openApi = Get-Content -Raw -Encoding utf8 $openApiPath
$migration = Get-Content -Raw -Encoding utf8 $migrationUpPath
$manifest = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'ui-manifest.json') | ConvertFrom-Json
$master = Get-Content -Raw -Encoding utf8 (Join-Path $Root 'docs/development-inputs/p1/ui-master-manifest.json') | ConvertFrom-Json
$forbiddenStage = 'content_' + 'review'

function Assert-Contract([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

function Get-OpenApiPathBlock([string]$Path) {
    $pattern = '(?ms)^  ' + [regex]::Escape($Path) + ':\r?\n(.*?)(?=^  /api/v1/|\z)'
    $match = [regex]::Match($openApi, $pattern)
    Assert-Contract $match.Success "Missing OpenAPI path: $Path"
    return $match.Value
}

$requiredPaths = @(
    '/api/v1/content-versions/{contentVersionId}/review-runs/preflight',
    '/api/v1/content-versions/{contentVersionId}/review-runs',
    '/api/v1/content-items/{contentItemId}/review-summary',
    '/api/v1/content-items/{contentItemId}/reviews',
    '/api/v1/reviews/{reviewId}',
    '/api/v1/reviews/{reviewId}/issues/{issueId}',
    '/api/v1/workflow-runs/{workflowRunId}/review-result-consumption-retries'
)
foreach ($path in $requiredPaths) { [void](Get-OpenApiPathBlock $path) }

foreach ($path in @(
    '/api/v1/content-versions/{contentVersionId}/review-runs',
    '/api/v1/reviews/{reviewId}/issues/{issueId}',
    '/api/v1/workflow-runs/{workflowRunId}/review-result-consumption-retries'
)) {
    Assert-Contract ((Get-OpenApiPathBlock $path).Contains('#/components/parameters/IdempotencyKey')) "Command path is missing Idempotency-Key: $path"
}

Assert-Contract ($openApi -match '(?m)^    ApplicableStage:\r?\n      type: string\r?\n      enum: \[chapter_planning, content_generation, review, rewrite\]$') 'ApplicableStage does not contain the frozen review Stage.'
Assert-Contract (-not $openApi.Contains($forbiddenStage)) 'A parallel Review Stage remains in OpenAPI.'
Assert-Contract ($openApi.Contains('subjectType=content_version') -and $migration.Contains("subject_type = 'content_version'")) 'subjectType=content_version is not frozen in OpenAPI and Migration 17.'
Assert-Contract ($openApi.Contains('const: review.input.v1') -and $openApi.Contains('ReviewRuntimeInputV1:')) 'review.input.v1 is not frozen.'
Assert-Contract ($openApi.Contains('const: review.output.v1') -and $openApi.Contains('ReviewRuntimeOutputV1:')) 'review.output.v1 is not frozen.'
$inputRequired = 'required: [schemaVersion, projectId, contentItemId, sourceContentVersionId, sourceContentVersionVersion, sourceContentHash, sourceTitle, sourceContent, optionalInstructions, reviewDimensions, workflowRunId, correlationId]'
$outputRequired = 'required: [schemaVersion, conclusion, summary, passedRuleCount, issues, recommendations]'
$issueRequired = 'required: [issueKey, position, categoryKey, categoryLabel, severity, title, description, evidence, location, suggestion]'
Assert-Contract ($openApi.Contains($inputRequired)) 'review.input.v1 required fields or canonical order changed.'
Assert-Contract ($openApi.Contains($outputRequired)) 'review.output.v1 required fields changed.'
Assert-Contract ($openApi.Contains($issueRequired)) 'review.output.v1 Issue required fields changed.'
Assert-Contract ($openApi.Contains('issues: {type: array, maxItems: 200, uniqueItems: true') -and $openApi.Contains('recommendations: {type: array, maxItems: 100, uniqueItems: true')) 'review.output.v1 collection limits changed.'
Assert-Contract ($openApi.Contains('issueKey and position are each unique; positions are contiguous from 1.') -and $openApi.Contains('single JSON object with no trailing JSON')) 'review.output.v1 semantic uniqueness or trailing-JSON rejection is missing.'
foreach ($secretTerm in @('credentials', 'Authorization', 'Cookie', 'Preflight Token')) {
    Assert-Contract ($openApi.Contains($secretTerm)) "Runtime contract safety rule is missing: $secretTerm"
}

$stateMatch = [regex]::Match($openApi, 'ContentReviewResultState: \{type: string, enum: \[([^\]]+)\]\}')
Assert-Contract $stateMatch.Success 'ContentReviewResultState is missing.'
$states = @($stateMatch.Groups[1].Value.Split(',') | ForEach-Object { $_.Trim() })
$expectedStates = @('idle','not_configured','queued','running','review_ready','runtime_failed','output_validation_failed','result_consumption_failed')
Assert-Contract ($states.Count -eq $expectedStates.Count -and (Compare-Object $states $expectedStates).Count -eq 0) "Unexpected ContentReviewResultState values: $($states -join ', ')"
$historyBlock = [regex]::Match($openApi, '(?ms)^    ContentReviewHistoryItem:\r?\n(.*?)(?=^    [A-Za-z][A-Za-z0-9]+:\r?\n)').Value
Assert-Contract ($historyBlock.Contains('required: [workflowRun, sourceContentVersionSummary, reportSummary, state, latestError]') -and $historyBlock.Contains('state: {$ref: "#/components/schemas/ContentReviewResultState"}') -and $historyBlock.Contains('latestError: {anyOf: [{$ref: "#/components/schemas/ContentReviewSafeError"}, {type: "null"}]}')) 'ContentReviewHistoryItem state/latestError contract is incomplete.'
$reviewErrorCodes = @('content_version_not_found','review_not_found','review_issue_not_found','workflow_run_not_found','review_issue_version_conflict','workflow_run_version_conflict')
foreach ($reviewErrorCode in $reviewErrorCodes) {
    Assert-Contract ($openApi.Contains($reviewErrorCode)) "Missing precise review error code: $reviewErrorCode"
}

$reviewReportBlock = [regex]::Match($openApi, '(?ms)^    ReviewReport:\r?\n(.*?)(?=^    [A-Za-z][A-Za-z0-9]+:\r?\n)').Value
Assert-Contract ($reviewReportBlock -match 'workflowRunId: \{type: \[string, "null"\]') 'ReviewReport.workflowRunId is not nullable in OpenAPI.'
Assert-Contract ($migration.Contains('ALTER COLUMN workflow_run_id DROP NOT NULL')) 'Migration 17 does not make P0 ReviewReport.workflow_run_id nullable.'
Assert-Contract ($migration.Contains('review_reports_workflow_run_unique') -or (Get-Content -Raw -Encoding utf8 (Join-Path $Root 'apps/api/migrations/000006_editor_review.up.sql')).Contains('review_reports_workflow_run_unique')) 'Non-null ReviewReport.workflow_run_id uniqueness is not preserved.'
Assert-Contract ($openApi.Contains('/api/v1/content-items/{contentItemId}/reviews/mock:')) 'P0 Mock Review API is missing.'
Assert-Contract ($openApi -match 'ReviewIssueDisposition: \{type: string, enum: \[open, ignored\]\}') 'ReviewIssue disposition is not exactly open/ignored.'
Assert-Contract (-not $openApi.Contains('/rewrite-runs')) 'Iteration 18 rewrite-run API was opened early.'

Assert-Contract ($manifest.status -eq 'frozen_cf_17_01') 'Iteration 17 manifest is not frozen_cf_17_01.'
Assert-Contract ($manifest.frameCount -eq 8 -and $manifest.frames.Count -eq 8) 'Iteration 17 manifest must contain exactly 8 Frames.'
$frameIds = @($manifest.frames.frameId)
Assert-Contract (($frameIds | Select-Object -Unique).Count -eq 8) 'Iteration 17 Frame IDs are not unique.'
foreach ($frame in $manifest.frames) {
    Assert-Contract (Test-Path -LiteralPath (Join-Path $iteration $frame.screenPath) -PathType Leaf) "Missing screen.png for $($frame.frameId)."
    Assert-Contract (Test-Path -LiteralPath (Join-Path $iteration $frame.htmlPath) -PathType Leaf) "Missing code.html for $($frame.frameId)."
}
$masterFrames = @($master.frames | Where-Object { $_.iteration -eq 'iteration-17-real-content-review' })
Assert-Contract ($masterFrames.Count -eq 8 -and (@($masterFrames.frameId | Select-Object -Unique).Count -eq 8)) 'UI Master Manifest does not map exactly 8 Iteration 17 Frames.'
foreach ($frame in $manifest.frames) {
    $mapped = @($masterFrames | Where-Object { $_.frameId -eq $frame.frameId })
    Assert-Contract ($mapped.Count -eq 1 -and $mapped[0].screen -eq $frame.screenPath -and $mapped[0].html -eq $frame.htmlPath) "UI Master Manifest mismatch: $($frame.frameId)"
}

$contractDocuments = Get-ChildItem $iteration -File | Where-Object { $_.Extension -in '.md', '.yaml', '.json' }
foreach ($document in $contractDocuments) {
    $text = Get-Content -Raw -Encoding utf8 $document.FullName
    Assert-Contract (-not $text.Contains('rebuild_candidate')) "Unfrozen status remains in $($document.Name)."
    Assert-Contract (-not $text.Contains($forbiddenStage)) "Parallel Review Stage remains in $($document.Name)."
}

$developmentPlan = Get-Content -Raw -Encoding utf8 (Join-Path $iteration 'development-plan.md')
$taskRows = [regex]::Matches($developmentPlan, '(?m)^\| [1-4] \| (CF-17-0[1-4]) \|')
Assert-Contract ($taskRows.Count -eq 4) 'development-plan.md must contain exactly four top-level task rows.'
Assert-Contract (($taskRows | ForEach-Object { $_.Groups[1].Value } | Select-Object -Unique).Count -eq 4) 'development-plan.md task IDs are not the four unique frozen IDs.'
Assert-Contract (-not ($developmentPlan -match 'CF-17-\d{2}[A-Z]')) 'development-plan.md still contains split task IDs.'

Assert-Contract (Test-Path -LiteralPath $migrationUpPath -PathType Leaf) 'Migration 17 up file is missing.'
Assert-Contract (Test-Path -LiteralPath $migrationDownPath -PathType Leaf) 'Migration 17 forward-only downgrade guard is missing.'
$migrationChanges = @(& git -C $Root diff --name-only -- apps/api/migrations)
Assert-Contract ($LASTEXITCODE -eq 0) 'Unable to inspect Migration changes.'
foreach ($changedMigration in $migrationChanges) {
    Assert-Contract ($changedMigration -match '^apps/api/migrations/000017_') "Historical Migration was modified: $changedMigration"
}
foreach ($term in @(
    'workflow_run_records_active_review_subject_idx',
    'review_reports_workflow_run_scope_trigger',
    'schema_version',
    'source_content_version_version',
    'source_content_hash',
    'issue_key',
    'category_label',
    'evidence_json',
    'disposition',
    'review_findings_review_issue_key_unique',
    'review_findings_review_severity_disposition_position_idx'
)) {
    Assert-Contract ($migration.Contains($term)) "Migration 17 is missing: $term"
}
Assert-Contract (-not $migration.Contains('CREATE TABLE review_issues')) 'Migration 17 creates a forbidden parallel review_issues table.'

Write-Host '[PASS] Iteration 17 OpenAPI, Runtime contracts, Migration, manifest, traceability and four-task plan validation completed.' -ForegroundColor Green
