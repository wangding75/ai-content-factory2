$ErrorActionPreference="Stop"
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$runtime=Join-Path $env:TEMP "acf-i06-qa-runtime"
$helper=Join-Path $runtime "db-helper.go"
if (-not (Test-Path -LiteralPath $helper)) { throw "QA runtime helper is missing. Run scripts/qa/iteration06/start.ps1 first." }
if ([string]::IsNullOrWhiteSpace($env:TEST_DATABASE_URL)) { throw "TEST_DATABASE_URL is required." }
Push-Location (Join-Path $repoRoot "apps/api")
try { $json=& go run $helper fixture $env:TEST_DATABASE_URL; if ($LASTEXITCODE -ne 0) { throw "fixture reset failed" } } finally { Pop-Location }
$fixturePath=Join-Path $runtime "fixtures.json"
[IO.File]::WriteAllText($fixturePath, $json, [Text.UTF8Encoding]::new($false))
$json
