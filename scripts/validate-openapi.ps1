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
foreach ($forbiddenName in @("/content-items", "ContentItem", "ContentDraft", "ContentVersion")) {
    if ($openApiText -match [regex]::Escape($forbiddenName)) {
        throw "OpenAPI contains out-of-scope name: $forbiddenName"
    }
}

$schemaPaths = @(
    "packages/contracts/content-packs/novel/project-planning.schema.json",
    "packages/contracts/content-packs/novel/material.schema.json",
    "packages/contracts/content-packs/novel/plot-line.schema.json",
    "packages/contracts/content-packs/novel/foreshadowing.schema.json",
    "packages/contracts/content-packs/novel/chapter-plan.schema.json"
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
if ($openApiText -match '(?i)/content-items|ContentItem|ContentDraft|ContentVersion') {
    throw "Iteration 05 contract contains a forbidden Iteration 06 surface."
}

Write-Host "[PASS] OpenAPI and Novel JSON Schema validation completed." -ForegroundColor Green

