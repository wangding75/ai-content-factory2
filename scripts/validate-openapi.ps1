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
    "packages/contracts/content-packs/novel/foreshadowing.schema.json"
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

Write-Host "[PASS] OpenAPI and Novel JSON Schema validation completed." -ForegroundColor Green

