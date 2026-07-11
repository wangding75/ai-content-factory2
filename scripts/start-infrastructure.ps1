Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

docker compose up -d postgres redis
if ($LASTEXITCODE -ne 0) {
    throw "Unable to start PostgreSQL and Redis."
}

docker compose ps