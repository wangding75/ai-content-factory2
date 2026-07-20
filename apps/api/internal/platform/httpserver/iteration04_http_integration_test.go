package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/foreshadowing"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

const iteration04HTTPTestDatabase = "ai_content_factory_http_test"

type i04Envelope struct {
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
	Raw       []byte
}
type i04Line struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id"`
	Version  int        `json:"version"`
	Children []i04Line  `json:"children"`
}
type i04Tree struct {
	Items []i04Line `json:"items"`
}
type i04F struct {
	ID      uuid.UUID  `json:"id"`
	Planted *uuid.UUID `json:"planted_plot_line_id"`
	Payoff  *uuid.UUID `json:"payoff_plot_line_id"`
	Status  string     `json:"status"`
	Version int        `json:"version"`
}
type i04FList struct {
	Items []i04F `json:"items"`
}

func openI04HTTP(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; Iteration 04 HTTP PostgreSQL integration test skipped")
	}
	c, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	if c.ConnConfig.Database != iteration04HTTPTestDatabase {
		t.Fatalf("TEST_DATABASE_URL must target isolated database %q; got database %q", iteration04HTTPTestDatabase, c.ConnConfig.Database)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	p, e := pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	var v int
	e = p.QueryRow(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&v)
	// Minimum version check: this HTTP E2E test verifies Iteration 04 functionality
	// remains compatible on the latest schema. Precise Migration 4 boundary validation
	// is covered by the migration integration tests in cmd/migrate.
	if e != nil || v < 4 {
		t.Fatalf("expected migrations through version 4, got %d: %v", v, e)
	}
	return p, ctx
}
func i04Server(p *pgxpool.Pool) *httptest.Server {
	projects := project.NewService(project.NewPostgresRepository(p))
	return httptest.NewServer(New(":0", projects, storyline.NewPostgresService(project.NewPostgresRepository(p), p), foreshadowing.NewPostgresService(project.NewPostgresRepository(p), p)).httpServer.Handler)
}
func i04Call(t *testing.T, c *http.Client, m, u string, b any) (*http.Response, i04Envelope) {
	t.Helper()
	var r io.Reader
	if b != nil {
		x, e := json.Marshal(b)
		if e != nil {
			t.Fatal(e)
		}
		r = bytes.NewReader(x)
	}
	q, e := http.NewRequest(m, u, r)
	if e != nil {
		t.Fatal(e)
	}
	if b != nil {
		q.Header.Set("Content-Type", "application/json")
	}
	res, e := c.Do(q)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(res.Body)
	res.Body.Close()
	if e != nil {
		t.Fatal(e)
	}
	var out i04Envelope
	if e = json.Unmarshal(raw, &out); e != nil {
		t.Fatalf("%d %s", res.StatusCode, raw)
	}
	out.Raw = raw
	if out.RequestID == "" {
		t.Fatalf("missing request_id %s", raw)
	}
	return res, out
}
func i04OK(t *testing.T, r *http.Response, e i04Envelope, s int) {
	t.Helper()
	if r.StatusCode != s {
		t.Fatalf("status=%d want=%d body=%s", r.StatusCode, s, e.Raw)
	}
}
func i04Project(t *testing.T, ctx context.Context, p *pgxpool.Pool, name string) uuid.UUID {
	id := uuid.New()
	if _, e := p.Exec(ctx, "INSERT INTO projects(id,name,type,created_by)VALUES($1,$2,'novel','i04-http')", id, name); e != nil {
		t.Fatal(e)
	}
	return id
}
func i04Count(t *testing.T, ctx context.Context, p *pgxpool.Pool, q string, args ...any) int {
	var n int
	if e := p.QueryRow(ctx, q, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func TestIteration04HTTPPostgresEndToEnd(t *testing.T) {
	p, ctx := openI04HTTP(t)
	projectID, otherID := i04Project(t, ctx, p, "http primary"), i04Project(t, ctx, p, "http other")
	t.Cleanup(func() {
		_, _ = p.Exec(context.Background(), "DELETE FROM audit_logs WHERE payload->'after'->>'ProjectID'=ANY($1)", []string{projectID.String(), otherID.String()})
		_, _ = p.Exec(context.Background(), "DELETE FROM projects WHERE id=ANY($1)", []uuid.UUID{projectID, otherID})
	})
	srv := i04Server(p)
	defer srv.Close()
	c := srv.Client()
	rootBody := map[string]any{"name": "root", "summary": "main", "start_chapter": 1, "end_chapter": 12, "status": "active", "sort_order": 10}
	r, e := i04Call(t, c, "POST", srv.URL+"/api/v1/projects/"+projectID.String()+"/storylines", rootBody)
	i04OK(t, r, e, 201)
	var root i04Line
	json.Unmarshal(e.Data, &root)
	if root.ID == uuid.Nil || root.ParentID != nil || root.Version != 1 {
		t.Fatal(string(e.Raw))
	}
	r, e = i04Call(t, c, "POST", srv.URL+"/api/v1/projects/"+projectID.String()+"/storylines", map[string]any{"name": "earlier", "summary": "s", "start_chapter": 1, "end_chapter": 12, "status": "active", "sort_order": 5})
	i04OK(t, r, e, 201)
	var first i04Line
	json.Unmarshal(e.Data, &first)
	childBody := map[string]any{"name": "child", "summary": "s", "start_chapter": 2, "end_chapter": 8, "status": "active", "sort_order": 0}
	r, e = i04Call(t, c, "POST", srv.URL+"/api/v1/storylines/"+root.ID.String()+"/children", childBody)
	i04OK(t, r, e, 201)
	var child i04Line
	json.Unmarshal(e.Data, &child)
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatal(string(e.Raw))
	}
	r, e = i04Call(t, c, "GET", srv.URL+"/api/v1/projects/"+projectID.String()+"/storylines", nil)
	i04OK(t, r, e, 200)
	var tree i04Tree
	json.Unmarshal(e.Data, &tree)
	if len(tree.Items) != 2 || tree.Items[0].ID != first.ID || tree.Items[1].ID != root.ID || len(tree.Items[1].Children) != 1 {
		t.Fatal(string(e.Raw))
	}
	if i04Count(t, ctx, p, "SELECT COUNT(*) FROM storylines WHERE project_id=$1", projectID) != 3 {
		t.Fatal("storylines not persisted")
	}
	r, e = i04Call(t, c, "PATCH", srv.URL+"/api/v1/storylines/"+root.ID.String(), map[string]any{"expected_version": 1, "name": "root updated"})
	i04OK(t, r, e, 200)
	json.Unmarshal(e.Data, &root)
	if root.Version != 2 || root.Name != "root updated" {
		t.Fatal(string(e.Raw))
	}
	r, e = i04Call(t, c, "PATCH", srv.URL+"/api/v1/storylines/"+root.ID.String(), map[string]any{"expected_version": 1, "name": "stale"})
	if r.StatusCode != 409 || !bytes.Contains(e.Raw, []byte("version_conflict")) {
		t.Fatal(string(e.Raw))
	}
	before := i04Count(t, ctx, p, "SELECT COUNT(*) FROM audit_logs WHERE subject_type='storyline'")
	for _, x := range []struct {
		u string
		b any
		s int
	}{{"/api/v1/storylines/" + uuid.New().String() + "/children", childBody, 404}, {"/api/v1/storylines/" + root.ID.String() + "/children", map[string]any{"name": "bad", "summary": "s", "start_chapter": 1, "end_chapter": 13, "status": "active", "sort_order": 0}, 400}} {
		r, e = i04Call(t, c, "POST", srv.URL+x.u, x.b)
		if r.StatusCode != x.s {
			t.Fatal(string(e.Raw))
		}
	}
	if i04Count(t, ctx, p, "SELECT COUNT(*) FROM audit_logs WHERE subject_type='storyline'") != before || i04Count(t, ctx, p, "SELECT COUNT(*) FROM storylines WHERE project_id=$1", projectID) != 3 {
		t.Fatal("invalid child dirty state")
	}
	r, e = i04Call(t, c, "POST", srv.URL+"/api/v1/projects/"+otherID.String()+"/storylines", rootBody)
	i04OK(t, r, e, 201)
	var other i04Line
	json.Unmarshal(e.Data, &other)
	fu := srv.URL + "/api/v1/projects/" + projectID.String() + "/foreshadowings"
	createF := func(title string, a, b any) i04F {
		r, e := i04Call(t, c, "POST", fu, map[string]any{"title": title, "description": "d", "priority": "high", "status": "planned", "planted_plot_line_id": a, "payoff_plot_line_id": b, "planned_plant_chapter": 2, "planned_payoff_chapter": 8})
		i04OK(t, r, e, 201)
		var f i04F
		json.Unmarshal(e.Data, &f)
		return f
	}
	cross := createF("cross", root.ID.String(), child.ID.String())
	nilRef := createF("nil", nil, nil)
	plant := createF("plant", root.ID.String(), nil)
	pay := createF("pay", nil, child.ID.String())
	invalidTransition := createF("invalid transition", nil, nil)
	r, e = i04Call(t, c, "PATCH", srv.URL+"/api/v1/foreshadowings/"+invalidTransition.ID.String(), map[string]any{"expected_version": 1, "status": "paid_off"})
	if r.StatusCode != 400 {
		t.Fatalf("planned to paid_off accepted: %s", e.Raw)
	}
	if cross.Planted == nil || cross.Payoff == nil || nilRef.Planted != nil || nilRef.Payoff != nil || plant.Payoff != nil || pay.Planted != nil {
		t.Fatal("nullable create")
	}
	r, e = i04Call(t, c, "PATCH", srv.URL+"/api/v1/foreshadowings/"+cross.ID.String(), map[string]any{"expected_version": 1, "planted_plot_line_id": nil})
	i04OK(t, r, e, 200)
	var f i04F
	json.Unmarshal(e.Data, &f)
	if f.Version != 2 || f.Planted != nil {
		t.Fatal(string(e.Raw))
	}
	r, e = i04Call(t, c, "PATCH", srv.URL+"/api/v1/foreshadowings/"+cross.ID.String(), map[string]any{"expected_version": 2, "status": "planted"})
	i04OK(t, r, e, 200)
	r, e = i04Call(t, c, "PATCH", srv.URL+"/api/v1/foreshadowings/"+cross.ID.String(), map[string]any{"expected_version": 3, "status": "paid_off"})
	i04OK(t, r, e, 200)
	json.Unmarshal(e.Data, &f)
	if f.Version != 4 || f.Status != "paid_off" {
		t.Fatal(string(e.Raw))
	}
	ab := i04Count(t, ctx, p, "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1", cross.ID.String())
	for _, b := range []any{map[string]any{"expected_version": 4, "status": "planted"}, map[string]any{"expected_version": 3, "title": "stale"}} {
		r, e = i04Call(t, c, "PATCH", srv.URL+"/api/v1/foreshadowings/"+cross.ID.String(), b)
		if r.StatusCode != 400 && r.StatusCode != 409 {
			t.Fatal(string(e.Raw))
		}
	}
	if i04Count(t, ctx, p, "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1", cross.ID.String()) != ab {
		t.Fatal("invalid update audit")
	}
	before = i04Count(t, ctx, p, "SELECT COUNT(*) FROM audit_logs WHERE subject_type='foreshadowing'")
	for _, b := range []any{map[string]any{"title": "missing", "description": "d", "priority": "low", "status": "planned", "planted_plot_line_id": uuid.New().String(), "payoff_plot_line_id": nil, "planned_plant_chapter": nil, "planned_payoff_chapter": nil}, map[string]any{"title": "other", "description": "d", "priority": "low", "status": "planned", "planted_plot_line_id": other.ID.String(), "payoff_plot_line_id": nil, "planned_plant_chapter": nil, "planned_payoff_chapter": nil}, map[string]any{"title": "range", "description": "d", "priority": "low", "status": "planned", "planted_plot_line_id": nil, "payoff_plot_line_id": nil, "planned_plant_chapter": 9, "planned_payoff_chapter": 2}} {
		r, e = i04Call(t, c, "POST", fu, b)
		if r.StatusCode != 400 && r.StatusCode != 404 {
			t.Fatal(string(e.Raw))
		}
	}
	if i04Count(t, ctx, p, "SELECT COUNT(*) FROM audit_logs WHERE subject_type='foreshadowing'") != before {
		t.Fatal("invalid create audit")
	}
	r, e = i04Call(t, c, "GET", fu, nil)
	i04OK(t, r, e, 200)
	var list i04FList
	json.Unmarshal(e.Data, &list)
	if len(list.Items) != 5 || i04Count(t, ctx, p, "SELECT COUNT(*) FROM foreshadowings WHERE project_id=$1", projectID) != 5 {
		t.Fatal(string(e.Raw))
	}
	if i04Count(t, ctx, p, "SELECT COUNT(*) FROM audit_logs WHERE action IN('storyline.created','storyline.updated','foreshadowing.created','foreshadowing.updated') AND subject_id=ANY($1)", []string{root.ID.String(), first.ID.String(), child.ID.String(), cross.ID.String(), nilRef.ID.String(), plant.ID.String(), pay.ID.String()}) < 10 {
		t.Fatal("audits missing")
	}
	srv.Close()
	srv = i04Server(p)
	defer srv.Close()
	c = srv.Client()
	r, e = i04Call(t, c, "GET", srv.URL+"/api/v1/projects/"+projectID.String()+"/storylines", nil)
	i04OK(t, r, e, 200)
	json.Unmarshal(e.Data, &tree)
	if len(tree.Items) != 2 || tree.Items[1].Name != "root updated" {
		t.Fatal("restart tree")
	}
	r, e = i04Call(t, c, "GET", srv.URL+"/api/v1/projects/"+projectID.String()+"/foreshadowings", nil)
	i04OK(t, r, e, 200)
	json.Unmarshal(e.Data, &list)
	found := false
	for _, v := range list.Items {
		if v.ID == cross.ID {
			found = v.Status == "paid_off" && v.Version == 4 && v.Planted == nil
		}
	}
	if !found {
		t.Fatal("restart foreshadowing")
	}
}
