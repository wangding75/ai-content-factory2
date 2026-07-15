$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$runtime = Join-Path $env:TEMP "acf-i06-qa-runtime"
$databaseName = "ai_content_factory_i06_qa"
$statePath = Join-Path $runtime "state.json"
$helperPath = Join-Path $runtime "db-helper.go"
New-Item -ItemType Directory -Force -Path $runtime | Out-Null

function Require-TargetDatabaseUrl {
  if ([string]::IsNullOrWhiteSpace($env:TEST_DATABASE_URL)) {
    throw "TEST_DATABASE_URL is required for the isolated Iteration 06 QA database."
  }
  return $env:TEST_DATABASE_URL
}

function Write-DatabaseHelper {
  if (Test-Path -LiteralPath $helperPath) { return }
  $source = @'
package main
import (
  "context"; "encoding/json"; "fmt"; "net/url"; "os"
  "github.com/google/uuid"; "github.com/jackc/pgx/v5"
)
const dbName = "ai_content_factory_i06_qa"
func target(raw string) string { u,e:=url.Parse(raw); if e!=nil { panic(e) }; u.Path="/"+dbName; return u.String() }
func admin(raw string) string { u,e:=url.Parse(raw); if e!=nil { panic(e) }; u.Path="/postgres"; return u.String() }
func connect(raw string) *pgx.Conn { c,e:=pgx.Connect(context.Background(),raw); if e!=nil { panic(e) }; return c }
func main(){ if len(os.Args)!=3 { panic("usage: ensure|version|fixture|drop|info TEST_DATABASE_URL") }; cmd,raw:=os.Args[1],os.Args[2]; ctx:=context.Background()
  switch cmd {
  case "ensure": c:=connect(admin(raw)); defer c.Close(ctx); var exists bool; if e:=c.QueryRow(ctx,"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)",dbName).Scan(&exists);e!=nil{panic(e)}; if !exists { if _,e:=c.Exec(ctx,"CREATE DATABASE "+dbName);e!=nil{panic(e)} }; fmt.Printf("exists_before=%t\n",exists)
  case "version": c:=connect(target(raw)); defer c.Close(ctx); var v int; if e:=c.QueryRow(ctx,"SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&v);e!=nil{panic(e)}; fmt.Printf("version=%d\n",v)
  case "info": c:=connect(admin(raw)); defer c.Close(ctx); var exists bool; if e:=c.QueryRow(ctx,"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)",dbName).Scan(&exists);e!=nil{panic(e)}; out:=map[string]any{"database":dbName,"exists":exists}; if exists { d:=connect(target(raw)); var v int; e:=d.QueryRow(ctx,"SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&v); d.Close(ctx); if e!=nil{panic(e)}; out["version"]=v }; b,_:=json.Marshal(out); fmt.Println(string(b))
  case "fixture": c:=connect(target(raw)); defer c.Close(ctx); tx,e:=c.Begin(ctx); if e!=nil{panic(e)}; defer tx.Rollback(ctx); if _,e=tx.Exec(ctx,"TRUNCATE TABLE projects CASCADE");e!=nil{panic(e)}; projectA,lineA,confirmedA,pendingA,projectB,lineB,confirmedB:=uuid.New(),uuid.New(),uuid.New(),uuid.New(),uuid.New(),uuid.New(),uuid.New(); for _,f:=range []struct{project,line uuid.UUID; name string}{{projectA,lineA,"A"},{projectB,lineB,"B"}}{if _,e=tx.Exec(ctx,"INSERT INTO projects(id,name,type,status,description,current_stage,created_by) VALUES($1,$2,'novel','planning','Reusable Iteration 06 QA fixture','content_production','i06-qa')",f.project,"I06 QA Project "+f.name);e!=nil{panic(e)};if _,e=tx.Exec(ctx,"INSERT INTO storylines(id,project_id,parent_id,type,relation,name,summary,status,sort_order,created_by) VALUES($1,$2,NULL,'main','root',$3,'Required upstream fixture','active',0,'i06-qa')",f.line,f.project,"I06 QA main storyline "+f.name);e!=nil{panic(e)}}; for _,f:=range []struct{project,line,plan uuid.UUID; title string}{{projectA,lineA,confirmedA,"I06 confirmed chapter A"},{projectB,lineB,confirmedB,"I06 confirmed chapter B"}}{if _,e=tx.Exec(ctx,"INSERT INTO chapter_plans(id,project_id,chapter_no,title,summary,chapter_goal,creation_notes,status,source,confirmed_at,created_by) VALUES($1,$2,1,$3,'Confirmed fixture','Exercise D1 UI states','QA fixture','confirmed','mock_generated',NOW(),'i06-qa')",f.plan,f.project,f.title);e!=nil{panic(e)};if _,e=tx.Exec(ctx,"INSERT INTO chapter_plan_storylines(chapter_plan_id,project_id,storyline_id,relation,position) VALUES($1,$2,$3,'primary',0)",f.plan,f.project,f.line);e!=nil{panic(e)}}; if _,e=tx.Exec(ctx,"INSERT INTO chapter_plans(id,project_id,chapter_no,title,summary,status,source,created_by) VALUES($1,$2,2,'I06 pending chapter A','Pending fixture','pending_confirmation','mock_generated','i06-qa')",pendingA,projectA);e!=nil{panic(e)}; if e=tx.Commit(ctx);e!=nil{panic(e)}; b,_:=json.Marshal(map[string]string{"project_a_id":projectA.String(),"confirmed_chapter_plan_id":confirmedA.String(),"pending_chapter_plan_id":pendingA.String(),"project_b_id":projectB.String(),"confirmed_chapter_plan_b_id":confirmedB.String()}); fmt.Println(string(b))
  case "drop": c:=connect(admin(raw)); defer c.Close(ctx); if _,e:=c.Exec(ctx,"DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)");e!=nil{panic(e)}; fmt.Println("dropped")
  default: panic("unknown command")
  }
}
'@
  [IO.File]::WriteAllText($helperPath, $source, [Text.UTF8Encoding]::new($false))
}

function Invoke-Helper([string]$command, [string]$databaseUrl) {
  Push-Location (Join-Path $repoRoot "apps/api")
  try { return (& go run $helperPath $command $databaseUrl) } finally { Pop-Location }
}

function Get-ContentHash([string[]]$paths) {
  $files = foreach ($path in $paths) { if (Test-Path -LiteralPath $path) { Get-ChildItem -LiteralPath $path -File -Recurse } }
  $manifest = ($files | Sort-Object FullName | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Path):$($_.Hash)" }) -join "`n"
  $sha = [Security.Cryptography.SHA256]::Create()
  try { return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($manifest)))).Replace("-", "") } finally { $sha.Dispose() }
}

function Test-Http200([string]$url) { try { return (Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec 3).StatusCode -eq 200 } catch { return $false } }
function Get-Listener([int]$port) { return Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1 }

$databaseUrl = Require-TargetDatabaseUrl
Write-DatabaseHelper
$ensure = Invoke-Helper "ensure" $databaseUrl
$databaseWasNew = $ensure -match "exists_before=false"
$migrationFiles = Get-ChildItem -LiteralPath (Join-Path $repoRoot "apps/api/migrations") -File | Sort-Object Name
$migrationHash = (($migrationFiles | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Name):$($_.Hash)" }) -join "`n")
$state = if (Test-Path -LiteralPath $statePath) { Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json } else { $null }
$needsMigration = $databaseWasNew -or -not $state -or $state.migration_hash -ne $migrationHash
if (-not $needsMigration) { try { $needsMigration = (Invoke-Helper "version" $databaseUrl) -notmatch "version=6" } catch { $needsMigration = $true } }
if ($needsMigration) {
  Push-Location (Join-Path $repoRoot "apps/api")
  try { $env:DATABASE_URL = $databaseUrl -replace '/[^/]+(\?.*)?$', "/$databaseName`$1"; & go run ./cmd/migrate up; if ($LASTEXITCODE -ne 0) { throw "migration failed" } } finally { Pop-Location }
}
if ((Invoke-Helper "version" $databaseUrl) -notmatch "version=6") { throw "QA database migration version is not 6." }

Push-Location (Join-Path $repoRoot "apps/web")
try { & node -e "const { chromium } = require('@playwright/test'); (async()=>{const b=await chromium.launch({headless:true}); await b.close(); console.log('Chromium launch PASS')})().catch(e=>{console.error(e);process.exit(1)})"; if ($LASTEXITCODE -ne 0) { throw "Playwright Chromium launch failed" } } finally { Pop-Location }

$apiListener = Get-Listener 18083
if (-not (Test-Http200 "http://127.0.0.1:18083/api/v1/meta")) {
  if ($apiListener) { throw "Port 18083 is occupied by a non-healthy process." }
  $apiLog = Join-Path $runtime "api.log"; $apiErr = Join-Path $runtime "api.err.log"
  $env:DATABASE_URL = $databaseUrl -replace '/[^/]+(\?.*)?$', "/$databaseName`$1"; $env:API_PORT = "18083"
  Start-Process -FilePath go -ArgumentList "run","./cmd/api" -WorkingDirectory (Join-Path $repoRoot "apps/api") -WindowStyle Hidden -RedirectStandardOutput $apiLog -RedirectStandardError $apiErr | Out-Null
  $deadline=(Get-Date).AddSeconds(40); while ((Get-Date) -lt $deadline -and -not (Test-Http200 "http://127.0.0.1:18083/api/v1/meta")) { Start-Sleep -Milliseconds 500 }
}
if (-not (Test-Http200 "http://127.0.0.1:18083/api/v1/meta")) { throw "API health check failed." }
$apiListener = Get-Listener 18083

$webInputs = @((Join-Path $repoRoot "apps/web/src"),(Join-Path $repoRoot "apps/web/next.config.ts"),(Join-Path $repoRoot "apps/web/public"))
$webHash = Get-ContentHash $webInputs
$nextDir = Join-Path $repoRoot "apps/web/.next"
$needsBuild = -not (Test-Path -LiteralPath $nextDir) -or -not $state -or $state.web_hash -ne $webHash
if ($needsBuild -and (Get-Listener 13005)) {
  if (-not $state -or -not $state.web_pid -or (Get-Listener 13005).OwningProcess -ne $state.web_pid) { throw "Port 13005 is occupied by a process not owned by this QA runtime." }
  Stop-Process -Id $state.web_pid -Force
  $deadline = (Get-Date).AddSeconds(10); while ((Get-Date) -lt $deadline -and (Get-Listener 13005)) { Start-Sleep -Milliseconds 200 }
}
if ($needsBuild) { Push-Location (Join-Path $repoRoot "apps/web"); try { $env:API_BASE_URL="http://127.0.0.1:18083/api/v1"; & .\node_modules\.bin\next.cmd build; if ($LASTEXITCODE -ne 0) { throw "Next build failed" } } finally { Pop-Location } }
$webListener = Get-Listener 13005
if (-not (Test-Http200 "http://127.0.0.1:13005/")) {
  if ($webListener) { throw "Port 13005 is occupied by a non-healthy process." }
  $webLog=Join-Path $runtime "web.log"; $webErr=Join-Path $runtime "web.err.log"; $cmd='set API_BASE_URL=http://127.0.0.1:18083/api/v1&& .\node_modules\.bin\next.cmd start -p 13005'
  Start-Process -FilePath cmd.exe -ArgumentList '/d','/c',$cmd -WorkingDirectory (Join-Path $repoRoot "apps/web") -WindowStyle Hidden -RedirectStandardOutput $webLog -RedirectStandardError $webErr | Out-Null
  $deadline=(Get-Date).AddSeconds(40); while ((Get-Date) -lt $deadline -and -not (Test-Http200 "http://127.0.0.1:13005/")) { Start-Sleep -Milliseconds 500 }
}
if (-not (Test-Http200 "http://127.0.0.1:13005/")) { throw "Web health check failed." }
$webListener=Get-Listener 13005
$stateJson = @{ database_name=$databaseName; database_url_source="TEST_DATABASE_URL"; migration_hash=$migrationHash; web_hash=$webHash; api_pid=$apiListener.OwningProcess; web_pid=$webListener.OwningProcess; api_log=(Join-Path $runtime "api.log"); web_log=(Join-Path $runtime "web.log") } | ConvertTo-Json
[IO.File]::WriteAllText($statePath, $stateJson, [Text.UTF8Encoding]::new($false))
Write-Output "Iteration 06 QA started: API PID $($apiListener.OwningProcess), Web PID $($webListener.OwningProcess), migration version 6."
