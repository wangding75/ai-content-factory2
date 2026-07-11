Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

docker compose down
if ($LASTEXITCODE -ne 0) {
    throw "Unable to stop Docker Compose services."
}