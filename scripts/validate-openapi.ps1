Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

npx.cmd --yes @redocly/cli@1.34.5 lint --skip-rule operation-summary --skip-rule security-defined --skip-rule operation-4xx-response --skip-rule info-license --skip-rule no-server-example.com packages/contracts/openapi/openapi.yaml
if ($LASTEXITCODE -ne 0) {
    throw "OpenAPI validation failed."
}

Write-Host "[PASS] OpenAPI validation completed." -ForegroundColor Green

