$ErrorActionPreference="Stop"
$repoRoot=(Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$runtime=Join-Path $env:TEMP "acf-i06-qa-runtime"; $statePath=Join-Path $runtime "state.json"; $helper=Join-Path $runtime "db-helper.go"
if (-not (Test-Path -LiteralPath $statePath) -or -not (Test-Path -LiteralPath $helper)) { throw "QA runtime is not initialized. Run start.ps1." }
if ([string]::IsNullOrWhiteSpace($env:TEST_DATABASE_URL)) { throw "TEST_DATABASE_URL is required." }
function Listener([int]$port) { Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1 }
function Http200([string]$url) { try { (Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec 3).StatusCode -eq 200 } catch { $false } }
$api=Listener 18083; $web=Listener 13005
if (-not $api -or -not (Http200 "http://127.0.0.1:18083/api/v1/meta")) { throw "API is not healthy." }
if (-not $web -or -not (Http200 "http://127.0.0.1:13005/")) { throw "Web is not healthy." }
$homeHtml=(Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:13005/" -TimeoutSec 3).Content
$static=[regex]::Match($homeHtml,'(?:src|href)="([^"]*/_next/static/[^"]+)"').Groups[1].Value
if ([string]::IsNullOrWhiteSpace($static) -or -not (Http200 ("http://127.0.0.1:13005"+$static))) { throw "A _next/static asset is not healthy." }
Push-Location (Join-Path $repoRoot "apps/web"); try { & node -e "const { chromium }=require('@playwright/test');(async()=>{const b=await chromium.launch({headless:true});await b.close()})().catch(e=>{console.error(e);process.exit(1)})";if ($LASTEXITCODE -ne 0) {throw "Chromium launch failed"}} finally {Pop-Location}
Push-Location (Join-Path $repoRoot "apps/api"); try { $info=& go run $helper info $env:TEST_DATABASE_URL; if ($LASTEXITCODE -ne 0) {throw "database info failed"} } finally {Pop-Location}
$db=$info|ConvertFrom-Json;if($db.database-ne "ai_content_factory_i06_qa" -or -not $db.exists -or $db.version-ne 6){throw "QA database or migration version is invalid."}
@{api_pid=$api.OwningProcess;web_pid=$web.OwningProcess;api_meta=200;web_home=200;static=200;chromium="PASS";database=$db.database;migration_version=$db.version}|ConvertTo-Json -Compress
