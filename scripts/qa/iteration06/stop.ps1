$ErrorActionPreference="Stop"
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$runtime=Join-Path $env:TEMP "acf-i06-qa-runtime"; $statePath=Join-Path $runtime "state.json"; $helper=Join-Path $runtime "db-helper.go"
if (-not (Test-Path -LiteralPath $statePath)) { throw "QA runtime is not initialized." }
if ([string]::IsNullOrWhiteSpace($env:TEST_DATABASE_URL)) { throw "TEST_DATABASE_URL is required." }
$state=Get-Content -LiteralPath $statePath -Raw|ConvertFrom-Json
foreach($processId in @($state.api_pid,$state.web_pid)){if($processId -and (Get-Process -Id $processId -ErrorAction SilentlyContinue)){Stop-Process -Id $processId -Force}}
if(Test-Path -LiteralPath $helper){Push-Location (Join-Path $repoRoot "apps/api");try{& go run $helper drop $env:TEST_DATABASE_URL;if ($LASTEXITCODE -ne 0) {throw "QA database drop failed"}}finally{Pop-Location}}
Remove-Item -LiteralPath $runtime -Recurse -Force
Write-Output "Iteration 06 QA stopped and isolated database removed."
