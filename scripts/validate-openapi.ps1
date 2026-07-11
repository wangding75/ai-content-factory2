Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

npx.cmd --yes @redocly/cli@1.34.5 lint --skip-rule operation-summary --skip-rule security-defined --skip-rule operation-4xx-response --skip-rule info-license --skip-rule no-server-example.com packages/contracts/openapi/openapi.yaml
if ($LASTEXITCODE -ne 0) {
    throw "OpenAPI validation failed."
}

$schemaPaths = @(
    "packages/contracts/content-packs/novel/project-planning.schema.json",
    "packages/contracts/content-packs/novel/material.schema.json"
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

