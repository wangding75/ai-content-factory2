[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$FixtureJson,
  [Parameter(Mandatory = $true)][string]$ContentItemId,
  [Parameter(Mandatory = $true)][string]$ContentVersionId,
  [Parameter(Mandatory = $true)][string]$ReviewId,
  [Parameter(Mandatory = $true)][string]$GenerateIdempotencyKey,
  [Parameter(Mandatory = $true)][string]$ReviewIdempotencyKey
)

$ErrorActionPreference = "Stop"
$runtime = Join-Path $env:TEMP "acf-i06-qa-runtime"
if (-not (Test-Path -LiteralPath $runtime)) { throw "QA runtime is missing." }
if (-not (Test-Path -LiteralPath $FixtureJson)) { throw "FixtureJson does not exist: $FixtureJson" }
if ([string]::IsNullOrWhiteSpace($env:TEST_DATABASE_URL)) { throw "TEST_DATABASE_URL is required." }

$helperPath = Join-Path $runtime "assert-db.go"
$source = @'
package main

import (
  "context"
  "encoding/json"
  "fmt"
  "net/url"
  "os"
  "sort"

  "github.com/jackc/pgx/v5"
)

type fixtures struct { ProjectAID string `json:"project_a_id"`; ConfirmedA string `json:"confirmed_chapter_plan_id"`; PendingA string `json:"pending_chapter_plan_id"`; ProjectBID string `json:"project_b_id"`; ConfirmedB string `json:"confirmed_chapter_plan_b_id"` }
type item struct { ID, ChapterPlanID, Title, Status, CurrentVersionID string; ReviewedAt *string `json:"reviewed_at"` }
type version struct { ID string; ContentItemID string `json:"content_item_id"`; VersionNo int `json:"version_no"`; Version int; WordCount int `json:"word_count"`; Status, Source, Title, Content string; Summary *string; FrozenAt *string `json:"frozen_at"` }
type review struct { ID, ContentItemID, ContentVersionID, Status string }
type finding struct { ID string; ReviewID string `json:"review_id"` }
type recommendation struct { ID string; ReviewID string `json:"review_id"` }
type fixture struct {
  Fixtures fixtures `json:"fixtures"`
  ContentItemAID string `json:"content_item_a_id"`; ContentVersionAID string `json:"content_version_a_id"`; GenerateRunID string `json:"generate_workflow_run_id"`; ReviewRunID string `json:"review_workflow_run_id"`; ReviewID string `json:"review_id"`; ContentItemBID string `json:"content_item_b_id"`; ContentVersionBID string `json:"content_version_b_id"`
  GenerateKey string `json:"generate_idempotency_key"`; ReviewKey string `json:"review_idempotency_key"`
  APIFinal struct { ContentItem item `json:"content_item"`; CurrentVersion version `json:"current_version"`; Review review `json:"review"`; Findings []finding `json:"findings"`; Recommendations []recommendation `json:"recommendations"` } `json:"api_final"`
}
type failure struct { Name string `json:"name"`; Actual any `json:"actual"`; Expected any `json:"expected"` }
type output struct { Pass bool `json:"pass"`; Assertions int `json:"assertions"`; Failures []failure `json:"failures"`; Counts map[string]int `json:"counts"` }

func qaURL(raw string) string { u,e:=url.Parse(raw); if e!=nil { panic(e) }; u.Path="/ai_content_factory_i06_qa"; return u.String() }
func main() {
  if len(os.Args)!=7 { panic("usage: assert-db.go fixture.json content-item-id content-version-id review-id generate-key review-key") }
  raw,e:=os.ReadFile(os.Args[1]); if e!=nil { panic(e) }; var f fixture; if e=json.Unmarshal(raw,&f);e!=nil { panic(e) }
  out:=output{Pass:true, Counts:map[string]int{}}; check:=func(name string, actual, expected any) { out.Assertions++; a,_:=json.Marshal(actual); b,_:=json.Marshal(expected); if string(a)!=string(b) { out.Pass=false; out.Failures=append(out.Failures,failure{name,actual,expected}) } }
  check("argument.content_item_id", os.Args[2], f.ContentItemAID); check("argument.content_version_id", os.Args[3], f.ContentVersionAID); check("argument.review_id", os.Args[4], f.ReviewID); check("argument.generate_key", os.Args[5], f.GenerateKey); check("argument.review_key", os.Args[6], f.ReviewKey)
  ctx:=context.Background(); db,e:=pgx.Connect(ctx,qaURL(os.Getenv("TEST_DATABASE_URL"))); if e!=nil { panic(e) }; defer db.Close(ctx)
  count:=func(name,q string,args ...any) int { var n int; if e:=db.QueryRow(ctx,q,args...).Scan(&n);e!=nil { panic(e) }; out.Counts[name]=n; return n }
  one:=func(name,q string,args ...any) string { var s string; if e:=db.QueryRow(ctx,q,args...).Scan(&s);e!=nil { panic(e) }; return s }
  check("migration_version", one("migration_version","SELECT COALESCE(MAX(version),0)::text FROM schema_migrations"), "6")
  check("content_item_count_plan_a",count("content_item_count_plan_a","SELECT COUNT(*) FROM content_items WHERE chapter_plan_id=$1",f.Fixtures.ConfirmedA),1)
  check("content_item_count_plan_b",count("content_item_count_plan_b","SELECT COUNT(*) FROM content_items WHERE chapter_plan_id=$1",f.Fixtures.ConfirmedB),1)
  check("content_item_count_pending",count("content_item_count_pending","SELECT COUNT(*) FROM content_items WHERE chapter_plan_id=$1",f.Fixtures.PendingA),0)
  check("a_versions",count("a_versions","SELECT COUNT(*) FROM content_versions WHERE content_item_id=$1",f.ContentItemAID),1)
  check("b_versions",count("b_versions","SELECT COUNT(*) FROM content_versions WHERE content_item_id=$1",f.ContentItemBID),1)
  check("no_v2",count("no_v2","SELECT COUNT(*) FROM content_versions WHERE version_no<>1"),0)
  var project,plan,status,current,title,content,source,vstatus string; var versionNo,versionNoOpt,words int; var summary *string; var reviewedAt,frozenAt *string
  e=db.QueryRow(ctx,`SELECT ci.project_id::text,ci.chapter_plan_id::text,ci.status,ci.current_version_id::text,ci.title,cv.content,cv.summary,cv.source,cv.status,cv.version_no,cv.version,cv.word_count,ci.reviewed_at::text,cv.frozen_at::text FROM content_items ci JOIN content_versions cv ON cv.id=ci.current_version_id WHERE ci.id=$1`,f.ContentItemAID).Scan(&project,&plan,&status,&current,&title,&content,&summary,&source,&vstatus,&versionNo,&versionNoOpt,&words,&reviewedAt,&frozenAt); if e!=nil { panic(e) }
  check("a.project",project,f.Fixtures.ProjectAID);check("a.chapter_plan",plan,f.Fixtures.ConfirmedA);check("a.status",status,"reviewed");check("a.current_version",current,f.ContentVersionAID);check("a.version_no",versionNo,1);check("a.version",versionNoOpt,f.APIFinal.CurrentVersion.Version);check("a.source",source,"mock_generated");check("a.version_status",vstatus,"frozen");check("a.reviewed_at",reviewedAt!=nil,true);check("a.frozen_at",frozenAt!=nil,true);check("a.title",title,f.APIFinal.ContentItem.Title);check("a.content",content,f.APIFinal.CurrentVersion.Content);check("a.summary",summary,f.APIFinal.CurrentVersion.Summary);check("a.word_count",words,f.APIFinal.CurrentVersion.WordCount)
  var bProject,bPlan,bStatus,bVStatus,bSource string; var bVersionNo int; e=db.QueryRow(ctx,`SELECT ci.project_id::text,ci.chapter_plan_id::text,ci.status,cv.status,cv.source,cv.version_no FROM content_items ci JOIN content_versions cv ON cv.id=ci.current_version_id WHERE ci.id=$1`,f.ContentItemBID).Scan(&bProject,&bPlan,&bStatus,&bVStatus,&bSource,&bVersionNo);if e!=nil { panic(e) };check("b.project",bProject,f.Fixtures.ProjectBID);check("b.chapter_plan",bPlan,f.Fixtures.ConfirmedB);check("b.status",bStatus,"draft");check("b.version_status",bVStatus,"editable_draft");check("b.source",bSource,"manual_created");check("b.version_no",bVersionNo,1)
  check("a_generate_runs",count("a_generate_runs","SELECT COUNT(*) FROM workflow_runs WHERE content_item_id=$1 AND workflow_key='content_mock_generate'",f.ContentItemAID),1);check("a_review_runs",count("a_review_runs","SELECT COUNT(*) FROM workflow_runs WHERE content_item_id=$1 AND workflow_key='content_mock_review'",f.ContentItemAID),1);check("b_runs",count("b_runs","SELECT COUNT(*) FROM workflow_runs WHERE content_item_id=$1",f.ContentItemBID),0);check("running_runs",count("running_runs","SELECT COUNT(*) FROM workflow_runs WHERE status='running'"),0)
  check("generate_run_id",one("generate_run_id","SELECT id::text FROM workflow_runs WHERE content_item_id=$1 AND workflow_key='content_mock_generate'",f.ContentItemAID),f.GenerateRunID);check("review_run_id",one("review_run_id","SELECT id::text FROM workflow_runs WHERE content_item_id=$1 AND workflow_key='content_mock_review'",f.ContentItemAID),f.ReviewRunID)
  check("generate_idempotency",one("generate_idempotency","SELECT idempotency_key FROM workflow_runs WHERE id=$1",f.GenerateRunID),f.GenerateKey);check("review_idempotency",one("review_idempotency","SELECT idempotency_key FROM workflow_runs WHERE id=$1",f.ReviewRunID),f.ReviewKey);check("workflow_success_shape",count("workflow_success_shape","SELECT COUNT(*) FROM workflow_runs WHERE content_item_id=$1 AND (status<>'succeeded' OR request_fingerprint !~ '^[0-9a-f]{64}$')",f.ContentItemAID),0);check("duplicate_operation_key",count("duplicate_operation_key","SELECT COUNT(*) FROM (SELECT project_id,content_item_id,workflow_key,idempotency_key FROM workflow_runs GROUP BY 1,2,3,4 HAVING COUNT(*)>1) x"),0)
  check("a_review_reports",count("a_review_reports","SELECT COUNT(*) FROM review_reports WHERE content_item_id=$1",f.ContentItemAID),1);check("b_review_reports",count("b_review_reports","SELECT COUNT(*) FROM review_reports WHERE content_item_id=$1",f.ContentItemBID),0);check("review_id",one("review_id","SELECT id::text FROM review_reports WHERE content_item_id=$1",f.ContentItemAID),f.ReviewID);check("review_fixed_version",one("review_fixed_version","SELECT content_version_id::text FROM review_reports WHERE id=$1",f.ReviewID),f.ContentVersionAID);check("review_run_link",one("review_run_link","SELECT workflow_run_id::text FROM review_reports WHERE id=$1",f.ReviewID),f.ReviewRunID);check("review_completed",one("review_completed","SELECT status FROM review_reports WHERE id=$1",f.ReviewID),"completed")
  check("findings_count",count("findings_count","SELECT COUNT(*) FROM review_findings WHERE review_id=$1",f.ReviewID),len(f.APIFinal.Findings));check("recommendations_count",count("recommendations_count","SELECT COUNT(*) FROM review_recommendations WHERE review_id=$1",f.ReviewID),len(f.APIFinal.Recommendations))
  rows,e:=db.Query(ctx,"SELECT id::text,sort_order FROM review_findings WHERE review_id=$1 ORDER BY sort_order,id",f.ReviewID);if e!=nil {panic(e)}; var found []string; i:=0;for rows.Next(){var id string;var order int;rows.Scan(&id,&order);check(fmt.Sprintf("finding_sort_%d",i),order,i);found=append(found,id);i++};rows.Close();for i,x:=range f.APIFinal.Findings{check(fmt.Sprintf("finding_api_order_%d",i),found[i],x.ID)}
  rows,e=db.Query(ctx,"SELECT id::text,sort_order FROM review_recommendations WHERE review_id=$1 ORDER BY sort_order,id",f.ReviewID);if e!=nil {panic(e)}; var recs []string;i=0;for rows.Next(){var id string;var order int;rows.Scan(&id,&order);check(fmt.Sprintf("recommendation_sort_%d",i),order,i);recs=append(recs,id);i++};rows.Close();for i,x:=range f.APIFinal.Recommendations{check(fmt.Sprintf("recommendation_api_order_%d",i),recs[i],x.ID)}
  q:=map[string]string{"orphan_versions":"SELECT COUNT(*) FROM content_versions cv LEFT JOIN content_items ci ON ci.id=cv.content_item_id WHERE ci.id IS NULL","orphan_runs":"SELECT COUNT(*) FROM workflow_runs wr LEFT JOIN content_versions cv ON cv.id=wr.content_version_id AND cv.content_item_id=wr.content_item_id LEFT JOIN content_items ci ON ci.id=wr.content_item_id AND ci.project_id=wr.project_id WHERE cv.id IS NULL OR ci.id IS NULL","orphan_reports":"SELECT COUNT(*) FROM review_reports rr LEFT JOIN workflow_runs wr ON wr.id=rr.workflow_run_id AND wr.project_id=rr.project_id AND wr.content_item_id=rr.content_item_id AND wr.content_version_id=rr.content_version_id WHERE wr.id IS NULL","orphan_findings":"SELECT COUNT(*) FROM review_findings f LEFT JOIN review_reports r ON r.id=f.review_id WHERE r.id IS NULL","orphan_recommendations":"SELECT COUNT(*) FROM review_recommendations r LEFT JOIN review_reports rr ON rr.id=r.review_id WHERE rr.id IS NULL"}; keys:=make([]string,0,len(q));for k:=range q{keys=append(keys,k)};sort.Strings(keys);for _,k:=range keys{check(k,count(k,q[k]),0)}
  check("cross_project_content_item",count("cross_project_content_item","SELECT COUNT(*) FROM content_items ci JOIN chapter_plans cp ON cp.id=ci.chapter_plan_id WHERE ci.project_id<>cp.project_id"),0); check("review_tree_count",count("review_tree_count","SELECT COUNT(*) FROM review_reports WHERE content_item_id=$1 AND content_version_id=$2 AND workflow_run_id=$3",f.ContentItemAID,f.ContentVersionAID,f.ReviewRunID),1)
  b,_:=json.Marshal(out);fmt.Println(string(b));if !out.Pass {os.Exit(1)}
}
'@
[IO.File]::WriteAllText($helperPath, $source, [Text.UTF8Encoding]::new($false))

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
Push-Location (Join-Path $repoRoot "apps/api")
try {
  & go run $helperPath $FixtureJson $ContentItemId $ContentVersionId $ReviewId $GenerateIdempotencyKey $ReviewIdempotencyKey
  if ($LASTEXITCODE -ne 0) { throw "PostgreSQL terminal audit failed." }
} finally {
  Pop-Location
}
