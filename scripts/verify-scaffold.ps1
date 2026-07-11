Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Push-Location .\apps\api
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Go tests failed."
    }
}
finally {
    Pop-Location
}

pnpm.cmd --dir apps/web lint
if ($LASTEXITCODE -ne 0) {
    throw "Web lint failed."
}

pnpm.cmd --dir apps/web exec tsc --noEmit
if ($LASTEXITCODE -ne 0) {
    throw "Web typecheck failed."
}

docker compose config *> $null
if ($LASTEXITCODE -ne 0) {
    throw "Docker Compose configuration validation failed."
}

Write-Host "[PASS] Scaffold verification completed." -ForegroundColor Green