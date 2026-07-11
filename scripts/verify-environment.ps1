Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

go version
git --version
node --version
pnpm.cmd --version
cmd /c "docker info >nul 2>&1"
if ($LASTEXITCODE -ne 0) {
    throw "Docker Engine is not running."
}
docker compose version

Write-Host "[PASS] Environment verification completed." -ForegroundColor Green