package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

const iteration06HTTPTestDatabase = "ai_content_factory_i06_http_test"

type i06Envelope struct { Data json.RawMessage `json:"data"`; Error struct { Code string `json:"code"` } `json:"error"`; RequestID string `json:"request_id"`; Raw []byte }

func i06Open(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper(); u := os.Getenv("TEST_DATABASE_URL"); if u == "" { t.Skip("TEST_DATABASE_URL is not set; Iteration 06 HTTP PostgreSQL integration test skipped outside its targeted run") }
	cfg, err := pgxpool.ParseConfig(u); if err != nil { t.Fatal(err) }; if cfg.ConnConfig.Database != iteration06HTTPTestDatabase { t.Fatalf("TEST_DATABASE_URL database=%q, want %q", cfg.ConnConfig.Database, iteration06HTTPTestDatabase) }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); t.Cleanup(cancel); p, err := pgxpool.NewWithConfig(ctx, cfg); if err != nil { t.Fatal(err) }; t.Cleanup(p.Close)
	var v int; if err := p.QueryRow(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&v); err != nil || v != 6 { t.Fatalf("migration version=%d err=%v, want 6", v, err) }; return p, ctx
}
func i06Server(p *pgxpool.Pool) *httptest.Server { return httptest.NewServer(New(":0", project.NewService(project.NewPostgresRepository(p)), contentitem.NewApplication(contentitem.NewPostgresRepository(p), nil)).httpServer.Handler) }
func i06Call(t *testing.T, c *http.Client, method, url string, body any, key string) (*http.Response, i06Envelope) {
	t.Helper(); var r io.Reader; if body != nil { var b []byte; var err error; if raw, ok := body.(json.RawMessage); ok { b = raw } else { b, err = json.Marshal(body); if err != nil { t.Fatal(err) } }; r = bytes.NewReader(b) }; req, err := http.NewRequest(method, url, r); if err != nil { t.Fatal(err) }; if body != nil { req.Header.Set("Content-Type", "application/json") }; if key != "" { req.Header.Set("Idempotency-Key", key) }; res, err := c.Do(req); if err != nil { t.Fatal(err) }; raw, err := io.ReadAll(res.Body); res.Body.Close(); if err != nil { t.Fatal(err) }; e := i06Envelope{Raw:raw}; if len(raw)>0 { if err=json.Unmarshal(raw,&e);err!=nil {t.Fatalf("invalid json: %s",raw)}; if e.RequestID=="" {t.Fatalf("missing request id: %s",raw)} }; return res,e
}
func i06Status(t *testing.T, r *http.Response, e i06Envelope, want int) { t.Helper(); if r.StatusCode != want { t.Fatalf("status=%d want=%d body=%s",r.StatusCode,want,e.Raw) } }
func i06Code(t *testing.T, e i06Envelope, code string) { t.Helper(); if e.Error.Code != code {t.Fatalf("code=%q want=%q body=%s",e.Error.Code,code,e.Raw)} }
func i06Count(t *testing.T, ctx context.Context, p *pgxpool.Pool, q string, a ...any) int { t.Helper(); var n int; if err:=p.QueryRow(ctx,q,a...).Scan(&n);err!=nil {t.Fatal(err)}; return n }

type i06Detail struct { ContentItem struct { ID uuid.UUID `json:"id"`; Status string `json:"status"`; CurrentVersionID uuid.UUID `json:"current_version_id"`; ReviewedAt *string `json:"reviewed_at"` } `json:"content_item"`; CurrentVersion struct { ID uuid.UUID `json:"id"`; VersionNo int `json:"version_no"`; Version int `json:"version"`; Status string `json:"status"`; Source string `json:"source"`; Title string `json:"title"`; Content string `json:"content"`; Summary *string `json:"summary"`; WordCount int `json:"word_count"`; FrozenAt *string `json:"frozen_at"` } `json:"current_version"` }
func i06DetailOf(t *testing.T, e i06Envelope) i06Detail { t.Helper(); var d i06Detail; if err:=json.Unmarshal(e.Data,&d);err!=nil {t.Fatal(err)}; return d }

type i06Fixture struct { project, confirmed, pending, storyline, material, fores uuid.UUID }
func i06FixtureData(t *testing.T, ctx context.Context, p *pgxpool.Pool) i06Fixture {
	t.Helper(); f:=i06Fixture{uuid.New(),uuid.New(),uuid.New(),uuid.New(),uuid.New(),uuid.New()}; tx,err:=p.Begin(ctx);if err!=nil{t.Fatal(err)};defer tx.Rollback(ctx)
	_,err=tx.Exec(ctx,"INSERT INTO projects(id,name,type,created_by) VALUES($1,'i06 project','novel','i06')",f.project);if err!=nil{t.Fatal(err)}
	_,err=tx.Exec(ctx,"INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root','i06 line','active',0,'i06')",f.storyline,f.project);if err!=nil{t.Fatal(err)}
	_,err=tx.Exec(ctx,"INSERT INTO materials(id,type,name,created_by) VALUES($1,'reference','i06 material','i06')",f.material);if err!=nil{t.Fatal(err)}; _,err=tx.Exec(ctx,"INSERT INTO project_material_usages(id,project_id,material_id,usage_type,created_by) VALUES($1,$2,$3,'reference','i06')",uuid.New(),f.project,f.material);if err!=nil{t.Fatal(err)}
	_,err=tx.Exec(ctx,"INSERT INTO foreshadowings(id,project_id,title,priority,status,created_by) VALUES($1,$2,'i06 foreshadowing','medium','planned','i06')",f.fores,f.project);if err!=nil{t.Fatal(err)}
	for _, x := range []struct{id uuid.UUID; n int; s string; confirmed bool}{{f.confirmed,1,"confirmed plan",true},{f.pending,2,"pending plan",false}} { if x.confirmed {_,err=tx.Exec(ctx,"INSERT INTO chapter_plans(id,project_id,chapter_no,title,status,source,confirmed_at,created_by) VALUES($1,$2,$3,$4,'confirmed','mock_generated',NOW(),'i06')",x.id,f.project,x.n,x.s)} else {_,err=tx.Exec(ctx,"INSERT INTO chapter_plans(id,project_id,chapter_no,title,status,source,created_by) VALUES($1,$2,$3,$4,'pending_confirmation','mock_generated','i06')",x.id,f.project,x.n,x.s)};if err!=nil{t.Fatal(err)} }
	if err=tx.Commit(ctx);err!=nil{t.Fatal(err)}; return f
}
func i06Params(f i06Fixture, goal string) map[string]any { return map[string]any{"expected_version":3,"parameters":map[string]any{"chapter_goal":goal,"creation_notes":nil,"storyline_refs_json":[]string{f.storyline.String()},"material_refs_json":[]string{f.material.String()},"foreshadowing_refs_json":[]string{f.fores.String()}}} }
func i06Safe(t *testing.T, e i06Envelope) { t.Helper(); s:=strings.ToLower(string(e.Raw)); for _, x:=range []string{"sql","postgres","connection","repository","stack"} {if strings.Contains(s,x){t.Fatalf("unsafe error leak %q in %s",x,e.Raw)}} }

func TestIteration06EditorReviewHTTPPostgresEndToEnd(t *testing.T) {
	p,ctx:=i06Open(t); f:=i06FixtureData(t,ctx,p); s:=i06Server(p); defer s.Close(); base:=s.URL+"/api/v1"; c:=s.Client()
	// D1 create/get: confirmed creates v1 exactly once; pending is a real 409.
	r,e:=i06Call(t,c,http.MethodPost,base+"/chapter-plans/"+f.confirmed.String()+"/content",nil,""); i06Status(t,r,e,201); d:=i06DetailOf(t,e); if d.ContentItem.Status!="draft"||d.CurrentVersion.VersionNo!=1||d.CurrentVersion.Version!=1||d.CurrentVersion.Status!="editable_draft"||d.CurrentVersion.Source!="manual_created"||d.ContentItem.CurrentVersionID!=d.CurrentVersion.ID {t.Fatalf("create shape: %s",e.Raw)}; item,version:=d.ContentItem.ID,d.CurrentVersion.ID
	r,e=i06Call(t,c,http.MethodPost,base+"/chapter-plans/"+f.confirmed.String()+"/content",nil,"");i06Status(t,r,e,200);if i06DetailOf(t,e).ContentItem.ID!=item||i06Count(t,ctx,p,"SELECT COUNT(*) FROM content_items WHERE chapter_plan_id=$1",f.confirmed)!=1||i06Count(t,ctx,p,"SELECT COUNT(*) FROM content_versions WHERE content_item_id=$1",item)!=1 {t.Fatal("duplicate create wrote rows")}
	r,e=i06Call(t,c,http.MethodPost,base+"/chapter-plans/"+f.pending.String()+"/content",nil,"");i06Status(t,r,e,409);i06Code(t,e,"chapter_plan_not_confirmed")
	// D1 get and draft tri-state/locking. The service recomputes words from content.
	r,e=i06Call(t,c,http.MethodGet,base+"/content-items/"+item.String(),nil,"");i06Status(t,r,e,200);if i06DetailOf(t,e).CurrentVersion.ID!=version{t.Fatal("get did not return current version")}
	r,e=i06Call(t,c,http.MethodPut,base+"/content-items/"+item.String()+"/draft",map[string]any{"expected_version":1,"title":"Draft title","content":"one two three","summary":"keep"},"");i06Status(t,r,e,200);d=i06DetailOf(t,e);if d.CurrentVersion.Version!=2||d.CurrentVersion.WordCount!=3{t.Fatalf("draft normal: %s",e.Raw)}
	r,e=i06Call(t,c,http.MethodPut,base+"/content-items/"+item.String()+"/draft",map[string]any{"expected_version":2,"content":"","summary":nil},"");i06Status(t,r,e,200);d=i06DetailOf(t,e);if d.CurrentVersion.Version!=3||d.CurrentVersion.Content!=""||d.CurrentVersion.Summary!=nil||d.CurrentVersion.Title!="Draft title"||d.CurrentVersion.WordCount!=0{t.Fatalf("draft tri-state: %s",e.Raw)}
	r,e=i06Call(t,c,http.MethodPut,base+"/content-items/"+item.String()+"/draft",map[string]any{"expected_version":2,"content":"must not persist"},"");i06Status(t,r,e,409);i06Code(t,e,"version_conflict");var content string;if err:=p.QueryRow(ctx,"SELECT content FROM content_versions WHERE id=$1",version).Scan(&content);err!=nil||content!=""{t.Fatalf("stale partial write content=%q err=%v",content,err)}
	for _, b:=range []any{map[string]any{"expected_version":3,"content":"x","unknown":1}, json.RawMessage(`{"expected_version":3,"content":`)} {r,e=i06Call(t,c,http.MethodPut,base+"/content-items/"+item.String()+"/draft",b,"");i06Status(t,r,e,400);i06Code(t,e,"validation_error")}
	// Mock generation keeps v1 but advances that version's optimistic counter.
	r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/mock-generate",i06Params(f,"goal"),"");i06Status(t,r,e,400);i06Code(t,e,"idempotency_key_required")
	r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/mock-generate",i06Params(f,"goal"),"generate-1");i06Status(t,r,e,200);var gen struct { CurrentVersion struct { VersionNo int `json:"version_no"`; Version int `json:"version"`; Source string `json:"source"`; Content string `json:"content"`; Summary *string `json:"summary"`; Title string `json:"title"`; WordCount int `json:"word_count"` } `json:"current_version"`; WorkflowRun struct { ID uuid.UUID `json:"id"`; Status string `json:"status"` } `json:"workflow_run"`};if err:=json.Unmarshal(e.Data,&gen);err!=nil{t.Fatal(err)};if gen.CurrentVersion.VersionNo!=1||gen.CurrentVersion.Version!=4||gen.CurrentVersion.Source!="mock_generated"||gen.CurrentVersion.Content==""||gen.CurrentVersion.Summary==nil||gen.CurrentVersion.WordCount!=len(strings.Fields(gen.CurrentVersion.Content))||gen.WorkflowRun.Status!="succeeded"{t.Fatalf("generate response: %s",e.Raw)}; genRun:=gen.WorkflowRun.ID
	r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/mock-generate",i06Params(f,"goal"),"generate-1");i06Status(t,r,e,200);var again struct{WorkflowRun struct{ID uuid.UUID `json:"id"`} `json:"workflow_run"`};_ = json.Unmarshal(e.Data,&again);if again.WorkflowRun.ID!=genRun||i06Count(t,ctx,p,"SELECT COUNT(*) FROM workflow_runs WHERE content_item_id=$1 AND workflow_key='content_mock_generate'",item)!=1 {t.Fatal("generate idempotency failed")}
	r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/mock-generate",i06Params(f,"different"),"generate-1");i06Status(t,r,e,409);i06Code(t,e,"idempotency_key_reused_with_different_payload")
	// D2 freezes exactly this v1 and creates a persisted review tree.
	reviewBody:=map[string]any{"content_version_id":version.String(),"expected_version":4};r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/reviews/mock",reviewBody,"review-1");i06Status(t,r,e,200);var review struct { Review struct{ID uuid.UUID `json:"id"`; ContentVersionID uuid.UUID `json:"content_version_id"`} `json:"review"`; Findings []json.RawMessage `json:"findings"`; Recommendations []json.RawMessage `json:"recommendations"`; WorkflowRun struct{ID uuid.UUID `json:"id"`;Status string `json:"status"`} `json:"workflow_run"`};if err:=json.Unmarshal(e.Data,&review);err!=nil{t.Fatal(err)};if review.Review.ID==uuid.Nil||review.Review.ContentVersionID!=version||len(review.Findings)!=2||len(review.Recommendations)!=2||review.WorkflowRun.Status!="succeeded"{t.Fatalf("review response: %s",e.Raw)};reviewID:=review.Review.ID
	var status,vs string;var reviewed,frozen *time.Time;if err:=p.QueryRow(ctx,"SELECT ci.status,ci.reviewed_at,cv.status,cv.frozen_at FROM content_items ci JOIN content_versions cv ON cv.id=ci.current_version_id WHERE ci.id=$1",item).Scan(&status,&reviewed,&vs,&frozen);err!=nil||status!="reviewed"||reviewed==nil||vs!="frozen"||frozen==nil{t.Fatalf("review freeze DB: %s %v %s %v %v",status,reviewed,vs,frozen,err)}
	r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/reviews/mock",reviewBody,"review-1");i06Status(t,r,e,200);var replay struct{Review struct{ID uuid.UUID `json:"id"`} `json:"review"`};_ = json.Unmarshal(e.Data,&replay);if replay.Review.ID!=reviewID||i06Count(t,ctx,p,"SELECT COUNT(*) FROM review_reports WHERE content_item_id=$1",item)!=1||i06Count(t,ctx,p,"SELECT COUNT(*) FROM workflow_runs WHERE content_item_id=$1 AND workflow_key='content_mock_review'",item)!=1 {t.Fatal("review idempotency wrote duplicate data")}
	r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/reviews/mock",map[string]any{"content_version_id":version.String(),"expected_version":99},"review-1");i06Status(t,r,e,409);i06Code(t,e,"idempotency_key_reused_with_different_payload");r,e=i06Call(t,c,http.MethodPost,base+"/content-items/"+item.String()+"/reviews/mock",reviewBody,"review-new");i06Status(t,r,e,409);i06Code(t,e,"content_version_already_reviewed")
	// List and detail are served by the PostgreSQL repository and retain the frozen version/run/tree.
	// A second fully-linked report is fixture data solely to assert the documented
	// created_at DESC, id DESC pagination query; it never bypasses the HTTP path
	// for the review under test.
	otherRun,otherReview:=uuid.New(),uuid.New(); _,err:=p.Exec(ctx,"INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,started_at,finished_at,created_at,updated_at) SELECT $1,project_id,id,current_version_id,'mock','content_mock_review','content_item',id,'succeeded','fixture-list','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','{}','{}',NOW()+INTERVAL '1 second',NOW()+INTERVAL '1 second',NOW()+INTERVAL '1 second',NOW()+INTERVAL '1 second' FROM content_items WHERE id=$2",otherRun,item);if err!=nil{t.Fatal(err)};_,err=p.Exec(ctx,"INSERT INTO review_reports(id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary,created_at,completed_at) SELECT $1,project_id,id,current_version_id,$2,'mock','completed','pass',100,'fixture pagination',NOW()+INTERVAL '1 second',NOW()+INTERVAL '1 second' FROM content_items WHERE id=$3",otherReview,otherRun,item);if err!=nil{t.Fatal(err)}
	r,e=i06Call(t,c,http.MethodGet,base+"/content-items/"+item.String()+"/reviews?limit=1&offset=0",nil,"");i06Status(t,r,e,200);var list struct{Items []struct{ID uuid.UUID `json:"id"`} `json:"items"`;Total,Limit,Offset int};if err:=json.Unmarshal(e.Data,&list);err!=nil{t.Fatal(err)};if list.Total!=2||list.Limit!=1||list.Offset!=0||len(list.Items)!=1||list.Items[0].ID!=otherReview{t.Fatalf("review list ordering: %s",e.Raw)}
	r,e=i06Call(t,c,http.MethodGet,base+"/content-items/"+item.String()+"/reviews?limit=1&offset=1",nil,"");i06Status(t,r,e,200);if err:=json.Unmarshal(e.Data,&list);err!=nil{t.Fatal(err)};if list.Total!=2||list.Offset!=1||len(list.Items)!=1||list.Items[0].ID!=reviewID{t.Fatalf("review list offset: %s",e.Raw)}
	r,e=i06Call(t,c,http.MethodGet,base+"/content-items/"+item.String()+"/reviews?limit=0",nil,"");i06Status(t,r,e,400);i06Code(t,e,"invalid_pagination")
	r,e=i06Call(t,c,http.MethodGet,base+"/reviews/"+reviewID.String(),nil,"");i06Status(t,r,e,200);var detail struct{ContentVersion struct{ID uuid.UUID `json:"id"`;VersionNo int `json:"version_no"`;Source string `json:"source"`;FrozenAt string `json:"frozen_at"`} `json:"content_version"`; Findings []json.RawMessage `json:"findings"`;Recommendations []json.RawMessage `json:"recommendations"`;WorkflowRun struct{ID uuid.UUID `json:"id"`;Status string `json:"status"`} `json:"workflow_run"`};if err:=json.Unmarshal(e.Data,&detail);err!=nil{t.Fatal(err)};if detail.ContentVersion.ID!=version||detail.ContentVersion.VersionNo!=1||detail.ContentVersion.Source!="mock_generated"||detail.ContentVersion.FrozenAt==""||detail.WorkflowRun.Status!="succeeded"||len(detail.Findings)!=2||len(detail.Recommendations)!=2{t.Fatalf("review detail: %s",e.Raw)}
	// Boundary errors are safe envelopes, including malformed UUID, missing resources, and a 422 validation contract.
	for _, x:=range []struct{method,url string; body any; want int; code string}{{http.MethodGet,base+"/content-items/not-a-uuid",nil,400,"invalid_uuid"},{http.MethodGet,base+"/content-items/"+uuid.New().String(),nil,404,"content_item_not_found"},{http.MethodGet,base+"/reviews/"+uuid.New().String(),nil,404,"review_not_found"},{http.MethodPost,base+"/content-items/"+item.String()+"/reviews/mock",map[string]any{"content_version_id":version.String(),"expected_version":0},422,"invalid_review_parameters"}} {r,e=i06Call(t,c,x.method,x.url,x.body,"bad-key");i06Status(t,r,e,x.want);i06Code(t,e,x.code);i06Safe(t,e)}
	if i06Count(t,ctx,p,"SELECT COUNT(*) FROM content_versions WHERE content_item_id=$1 AND version_no=2",item)!=0||i06Count(t,ctx,p,"SELECT COUNT(*) FROM workflow_runs WHERE content_item_id=$1 AND status='running'",item)!=0||i06Count(t,ctx,p,"SELECT COUNT(*) FROM review_findings WHERE review_id=$1",reviewID)!=2||i06Count(t,ctx,p,"SELECT COUNT(*) FROM review_recommendations WHERE review_id=$1",reviewID)!=2 {t.Fatal("final DB invariant failed")}
	// Closing the real pool produces a repository failure through the real
	// handler/application mapping, proving the 500 envelope is sanitized.
	p.Close(); r,e=i06Call(t,c,http.MethodGet,base+"/content-items/"+item.String(),nil,"");i06Status(t,r,e,500);i06Code(t,e,"internal_error");i06Safe(t,e)
}
