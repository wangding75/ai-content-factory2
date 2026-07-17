Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

npx.cmd --yes @redocly/cli@1.34.5 lint --skip-rule operation-summary --skip-rule security-defined --skip-rule operation-4xx-response --skip-rule info-license --skip-rule no-server-example.com packages/contracts/openapi/openapi.yaml
if ($LASTEXITCODE -ne 0) {
    throw "OpenAPI validation failed."
}
$openApiText = Get-Content -Raw "packages/contracts/openapi/openapi.yaml"
$iteration04Operations = @(
    "listProjectStorylines",
    "createProjectStoryline",
    "createStorylineChild",
    "updateStoryline",
    "listProjectForeshadowings",
    "createProjectForeshadowing",
    "updateForeshadowing"
)
foreach ($operationId in $iteration04Operations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) {
        throw "Iteration 04 OpenAPI operation is missing: $operationId"
    }
}
$schemaPaths = @(
    "packages/contracts/content-packs/novel/project-planning.schema.json",
    "packages/contracts/content-packs/novel/material.schema.json",
    "packages/contracts/content-packs/novel/plot-line.schema.json",
    "packages/contracts/content-packs/novel/foreshadowing.schema.json",
    "packages/contracts/content-packs/novel/chapter-plan.schema.json",
    "packages/contracts/content-packs/novel/content-item.schema.json",
    "packages/contracts/content-packs/novel/content-version.schema.json",
    "packages/contracts/content-packs/novel/mock-generation-parameters.schema.json",
    "packages/contracts/content-packs/novel/review-report.schema.json",
    "packages/contracts/content-packs/novel/review-finding.schema.json",
    "packages/contracts/content-packs/novel/review-recommendation.schema.json",
    "packages/contracts/content-packs/novel/workflow-run-summary.schema.json",
    "packages/contracts/content-packs/novel/mock-rewrite.schema.json",
    "packages/contracts/content-packs/novel/content-version-query.schema.json",
    "packages/contracts/content-packs/novel/project-work.schema.json"
    ,"packages/contracts/content-packs/novel/global-lite.schema.json"
)

foreach ($schemaPath in $schemaPaths) {
    try {
        $schema = Get-Content -Raw $schemaPath | ConvertFrom-Json
    }
    catch {
        throw "JSON Schema validation failed for ${schemaPath}: $($_.Exception.Message)"
    }

    if ($schema.'$schema' -ne "https://json-schema.org/draft/2020-12/schema" -or
        [string]::IsNullOrWhiteSpace($schema.'$id') -or
        [string]::IsNullOrWhiteSpace($schema.title)) {
        throw "JSON Schema metadata validation failed for $schemaPath."
    }
}

$iteration05Operations = @(
    "listProjectChapterPlans",
    "mockGenerateProjectChapterPlans",
    "getChapterPlan",
    "updateChapterPlan",
    "deleteChapterPlan",
    "confirmProjectChapterPlans"
)
foreach ($operationId in $iteration05Operations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) {
        throw "Iteration 05 OpenAPI operation is missing: $operationId"
    }
}

$iteration05Paths = @(
    "/api/v1/projects/{projectId}/chapter-plans",
    "/api/v1/projects/{projectId}/chapter-plans/mock-generate",
    "/api/v1/chapter-plans/{chapterPlanId}",
    "/api/v1/projects/{projectId}/chapter-plans/confirm"
)
foreach ($path in $iteration05Paths) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) {
        throw "Iteration 05 OpenAPI path is missing: $path"
    }
}
foreach ($methodExpectation in @(
    @{ Path = "/api/v1/projects/{projectId}/chapter-plans"; Method = "get" },
    @{ Path = "/api/v1/projects/{projectId}/chapter-plans/mock-generate"; Method = "post" },
    @{ Path = "/api/v1/chapter-plans/{chapterPlanId}"; Method = "get" },
    @{ Path = "/api/v1/chapter-plans/{chapterPlanId}"; Method = "patch" },
    @{ Path = "/api/v1/chapter-plans/{chapterPlanId}"; Method = "delete" },
    @{ Path = "/api/v1/projects/{projectId}/chapter-plans/confirm"; Method = "post" }
)) {
    $pathPattern = "(?ms)^  " + [regex]::Escape($methodExpectation.Path) + ":\s*$.*?(?=^  /|^components:)"
    $pathBlock = [regex]::Match($openApiText, $pathPattern).Value
    if ($pathBlock -notmatch ("(?m)^    " + $methodExpectation.Method + ":\s*$")) {
        throw "Iteration 05 API method is missing: $($methodExpectation.Method.ToUpperInvariant()) $($methodExpectation.Path)"
    }
}
foreach ($errorExpectation in @(
    @{ OperationId = "listProjectChapterPlans"; Errors = @("400", "404") },
    @{ OperationId = "mockGenerateProjectChapterPlans"; Errors = @("400", "404", "409") },
    @{ OperationId = "getChapterPlan"; Errors = @("400", "404") },
    @{ OperationId = "updateChapterPlan"; Errors = @("400", "404", "409") },
    @{ OperationId = "deleteChapterPlan"; Errors = @("400", "404", "409") },
    @{ OperationId = "confirmProjectChapterPlans"; Errors = @("400", "404", "409") }
)) {
    $operationPattern = "(?ms)^\s*operationId:\s*" + [regex]::Escape($errorExpectation.OperationId) + "\s*$.*?(?=^\s*operationId:|^  /|^components:)"
    $operationBlock = [regex]::Match($openApiText, $operationPattern).Value
    foreach ($errorCode in $errorExpectation.Errors) {
        if ($operationBlock -notmatch ('(?m)^        "' + $errorCode + '": \{\$ref: "#/components/responses/')) {
            throw "Iteration 05 API error response is missing: $($errorExpectation.OperationId) $errorCode"
        }
    }
}
if ([regex]::Matches($openApiText, "(?m)^\s*operationId:\s*(listProjectChapterPlans|mockGenerateProjectChapterPlans|getChapterPlan|updateChapterPlan|deleteChapterPlan|confirmProjectChapterPlans)\s*$").Count -ne 6) {
    throw "Iteration 05 OpenAPI contains an unexpected or duplicate operationId."
}
foreach ($requiredFragment in @(
    'operationId: updateChapterPlan',
    'operationId: deleteChapterPlan',
    'operationId: confirmProjectChapterPlans',
    'expected_version',
    '"400": {$ref: "#/components/responses/BadRequest"}',
    '"404": {$ref: "#/components/responses/NotFound"}',
    '"409": {$ref: "#/components/responses/Conflict"}'
)) {
    if ($openApiText -notmatch [regex]::Escape($requiredFragment)) {
        throw "Iteration 05 required contract fragment is missing: $requiredFragment"
    }
}

$chapterPlanSchema = Get-Content -Raw "packages/contracts/content-packs/novel/chapter-plan.schema.json" | ConvertFrom-Json
$chapterPlanFields = @("chapter_no", "title", "summary", "storyline_refs_json", "material_refs_json", "foreshadowing_refs_json", "chapter_goal", "creation_notes")
if (($chapterPlanSchema.required -join ',') -ne ($chapterPlanFields -join ',')) {
    throw "Chapter-plan JSON Schema required fields do not match the frozen editable model."
}
if ($openApiText -notmatch [regex]::Escape('required: [id, project_id, chapter_no, title, summary, status, source, storyline_refs_json, material_refs_json, foreshadowing_refs_json, chapter_goal, creation_notes, confirmed_at, version, created_at, updated_at]')) {
    throw "Chapter-plan OpenAPI response required fields do not match the frozen model."
}
foreach ($field in $chapterPlanFields) {
    if ($chapterPlanSchema.properties.PSObject.Properties.Name -notcontains $field -or
        $chapterPlanSchema.required -notcontains $field) {
        throw "Chapter-plan JSON Schema field/required mismatch: $field"
    }
    if ($openApiText -notmatch ("(?m)^        " + [regex]::Escape($field) + ":")) {
        throw "Chapter-plan OpenAPI field mismatch: $field"
    }
}
foreach ($nullableField in @("chapter_goal", "creation_notes")) {
    if ($chapterPlanSchema.properties.$nullableField.type -notcontains "null" -or
        $openApiText -notmatch ('(?m)^        ' + [regex]::Escape($nullableField) + ': \{type: \[string, "null"\]')) {
        throw "Chapter-plan nullable field mismatch: $nullableField"
    }
}
if ($chapterPlanSchema.properties.storyline_refs_json.items.properties.relation.enum -join ',' -ne 'primary,secondary') {
    throw "Chapter-plan JSON Schema relation enum mismatch."
}
if ($openApiText -notmatch 'enum: \[primary, secondary\]') {
    throw "Chapter-plan OpenAPI relation enum mismatch."
}
if ($openApiText -notmatch 'enum: \[pending_confirmation, confirmed\]' -or
    $openApiText -notmatch 'enum: \[mock_generated\]') {
    throw "Chapter-plan OpenAPI status/source enum mismatch."
}

$iteration06Operations = @("createContentItemForChapterPlan", "getContentItem", "saveContentItemDraft", "mockGenerateContentItem", "mockReviewContentItem", "listContentItemReviews", "getReview")
foreach ($operationId in $iteration06Operations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 06 OpenAPI operation is missing: $operationId" }
}
foreach ($path in @("/api/v1/chapter-plans/{chapterPlanId}/content", "/api/v1/content-items/{contentItemId}", "/api/v1/content-items/{contentItemId}/draft", "/api/v1/content-items/{contentItemId}/mock-generate", "/api/v1/content-items/{contentItemId}/reviews/mock", "/api/v1/content-items/{contentItemId}/reviews", "/api/v1/reviews/{reviewId}")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 06 OpenAPI path is missing: $path" }
}
foreach ($fragment in @("ContentItem", "ContentVersion", "ReviewReport", "WorkflowRunSummary", "expected_version", "Idempotency-Key", "content_version_already_reviewed", "created_at DESC, id DESC")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 06 required contract fragment is missing: $fragment" }
}
foreach ($schemaPath in $schemaPaths | Where-Object { $_ -match "(content-item|content-version|mock-generation|review-|workflow-run)" }) {
    $schema = Get-Content -Raw $schemaPath | ConvertFrom-Json
    if ($schema.additionalProperties -ne $false) { throw "Iteration 06 schema must set additionalProperties=false: $schemaPath" }
}

$iteration071AOperations = @("mockRewriteContentItem", "getWorkflowRun")
foreach ($operationId in $iteration071AOperations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 07.1A OpenAPI operation is missing: $operationId" }
}
foreach ($path in @("/api/v1/content-items/{contentItemId}/rewrites/mock", "/api/v1/workflow-runs/{workflowRunId}")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 07.1A OpenAPI path is missing: $path" }
}
foreach ($fragment in @("MockRewriteRequest", "MockRewriteParameters", "MockRewriteResult", "WorkflowRunDetail", "content_mock_rewrite", "mock_rewrite", "idempotency_key_reused_with_different_payload")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 07.1A required contract fragment is missing: $fragment" }
}

$iteration071BOperations = @("listContentItemVersions", "getContentVersion")
foreach ($operationId in $iteration071BOperations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 07.1B OpenAPI operation is missing: $operationId" }
}
foreach ($path in @("/api/v1/content-items/{contentItemId}/versions", "/api/v1/content-versions/{versionId}")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 07.1B OpenAPI path is missing: $path" }
}
foreach ($fragment in @("ContentVersionListEnvelope", "ContentVersionDetailEnvelope", "version_no DESC, id DESC", "ContentItem.current_version_id", "ContentVersionSourceSummary", "ContentVersionReviewSummary", "ContentVersionWorkflowRunSummary")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 07.1B required contract fragment is missing: $fragment" }
}

$iteration071COperations = @("listProjectWorks", "getProjectWork")
foreach ($operationId in $iteration071COperations) { if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 07.1C OpenAPI operation is missing: $operationId" } }
foreach ($path in @("/api/v1/projects/{projectId}/works", "/api/v1/works/{workId}")) { if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 07.1C OpenAPI path is missing: $path" } }
foreach ($fragment in @("ProjectWorkReadModel", "ProjectWorkListEnvelope", "ProjectWorkDetailEnvelope", "Stable read-only alias of content_item.id", "chapter_plan.chapter_no ASC, content_item.id ASC")) { if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 07.1C required contract fragment is missing: $fragment" } }

$iteration08Operations = @("listMaterials", "listGlobalWorks", "listBuiltinWorkflows", "listGlobalWorkflowRuns", "listCapabilities", "listIntegrations")
foreach ($operationId in $iteration08Operations) {
    if ([regex]::Matches($openApiText, "(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$").Count -ne 1) { throw "Iteration 08 OpenAPI operation must exist exactly once: $operationId" }
}
foreach ($path in @("/api/v1/materials", "/api/v1/works", "/api/v1/workflows/builtin", "/api/v1/workflow-runs", "/api/v1/capabilities", "/api/v1/integrations")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 08 OpenAPI path is missing: $path" }
}
foreach ($fragment in @("GlobalWorkListEnvelope", "BuiltinWorkflowListEnvelope", "GlobalWorkflowRunListEnvelope", "CapabilityListEnvelope", "IntegrationListEnvelope", "GlobalScopeQuery", "current_version_id", "started_at DESC, id DESC", "not_available")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 08 required contract fragment is missing: $fragment" }
}
$globalLiteSchema = Get-Content -Raw "packages/contracts/content-packs/novel/global-lite.schema.json" | ConvertFrom-Json
if ($globalLiteSchema.additionalProperties -ne $false -or $globalLiteSchema.'$defs'.builtin_workflow.properties.provider_key.const -ne "mock") { throw "Iteration 08 Global Lite JSON Schema drift." }

Write-Host "[PASS] OpenAPI and Novel JSON Schema validation completed." -ForegroundColor Green

