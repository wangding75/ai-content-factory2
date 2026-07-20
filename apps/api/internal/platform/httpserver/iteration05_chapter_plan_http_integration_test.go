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
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

const iteration05HTTPTestDatabase = "ai_content_factory_http_test"

type i05Envelope struct {
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
	Raw       []byte
}

type i05Plan struct {
	ID                uuid.UUID `json:"id"`
	ProjectID         uuid.UUID `json:"project_id"`
	ChapterNo         int       `json:"chapter_no"`
	Status            string    `json:"status"`
	ChapterGoal       *string   `json:"chapter_goal"`
	CreationNotes     *string   `json:"creation_notes"`
	ConfirmedAt       *string   `json:"confirmed_at"`
	Version           int       `json:"version"`
	CreatedAt         string    `json:"created_at"`
	UpdatedAt         string    `json:"updated_at"`
	StorylineRefsJSON []struct {
		StorylineID uuid.UUID `json:"storyline_id"`
		Relation    string    `json:"relation"`
	} `json:"storyline_refs_json"`
	MaterialRefsJSON      []uuid.UUID `json:"material_refs_json"`
	ForeshadowingRefsJSON []uuid.UUID `json:"foreshadowing_refs_json"`
}

type i05List struct {
	Items  []i05Plan `json:"items"`
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

func openI05HTTP(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; Iteration 05 HTTP PostgreSQL integration test skipped")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Database != iteration05HTTPTestDatabase {
		t.Fatalf("TEST_DATABASE_URL must target isolated database %q; got %q", iteration05HTTPTestDatabase, cfg.ConnConfig.Database)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var version int
	if err = pool.QueryRow(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil || version < 5 {
		t.Fatalf("expected migrations through version 5, got %d: %v", version, err)
	}
	return pool, ctx
}

func i05Server(pool *pgxpool.Pool) *httptest.Server {
	projects := project.NewService(project.NewPostgresRepository(pool))
	plans := chapterplan.NewPostgresService(project.NewPostgresRepository(pool), pool)
	return httptest.NewServer(New(":0", projects, plans).httpServer.Handler)
}

func i05Call(t *testing.T, client *http.Client, method, url string, body any) (*http.Response, i05Envelope) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		if raw, ok := body.([]byte); ok {
			reader = bytes.NewReader(raw)
		} else {
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(raw)
		}
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	out := i05Envelope{Raw: raw}
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("status=%d invalid json=%s", res.StatusCode, raw)
		}
		if out.RequestID == "" {
			t.Fatalf("missing request ID: %s", raw)
		}
	}
	return res, out
}

func i05RequireStatus(t *testing.T, response *http.Response, envelope i05Envelope, want int) {
	t.Helper()
	if response.StatusCode != want {
		t.Fatalf("status=%d want=%d body=%s", response.StatusCode, want, envelope.Raw)
	}
}

type i05Fixture struct {
	project, emptyProject        uuid.UUID
	storylines, materials, fores []uuid.UUID
}

func i05FixtureData(t *testing.T, ctx context.Context, pool *pgxpool.Pool) i05Fixture {
	t.Helper()
	f := i05Fixture{project: uuid.New(), emptyProject: uuid.New()}
	for i := 0; i < 2; i++ {
		f.storylines = append(f.storylines, uuid.New())
		f.materials = append(f.materials, uuid.New())
		f.fores = append(f.fores, uuid.New())
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, id := range []uuid.UUID{f.project, f.emptyProject} {
		if _, err = tx.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','i05-http')", id, "i05-http-"+id.String()); err != nil {
			t.Fatal(err)
		}
	}
	for i := range f.storylines {
		if _, err = tx.Exec(ctx, "INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root',$3,'active',$4,'i05-http')", f.storylines[i], f.project, "line", i); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO materials(id,type,name,created_by) VALUES($1,'reference',$2,'i05-http')", f.materials[i], "material"); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO project_material_usages(id,project_id,material_id,usage_type,created_by) VALUES($1,$2,$3,'reference','i05-http')", uuid.New(), f.project, f.materials[i]); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO foreshadowings(id,project_id,title,priority,status,created_by,planned_plant_chapter,planned_payoff_chapter) VALUES($1,$2,$3,'medium','planned','i05-http',1,20)", f.fores[i], f.project, "foreshadowing"); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM projects WHERE id=ANY($1)", []uuid.UUID{f.project, f.emptyProject})
	})
	return f
}

func i05MockBody(f i05Fixture, start, end int) map[string]any {
	return map[string]any{"target_storyline_id": f.storylines[0].String(), "start_chapter_no": start, "end_chapter_no": end, "chapter_count": end - start + 1, "include_main_storyline": true, "include_child_storylines": false, "include_project_materials": true, "include_unpaid_foreshadowings": true, "include_prior_chapter_summaries": true, "summary_length": "medium", "chapter_pace": "balanced", "generation_notes": nil}
}

func i05Generated(t *testing.T, response *http.Response, envelope i05Envelope, want int) ([]i05Plan, uuid.UUID) {
	t.Helper()
	i05RequireStatus(t, response, envelope, http.StatusCreated)
	var data struct {
		Run struct {
			ID uuid.UUID `json:"id"`
		} `json:"run"`
		Items []i05Plan `json:"items"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Run.ID == uuid.Nil || len(data.Items) != want {
		t.Fatalf("unexpected generation response: %s", envelope.Raw)
	}
	return data.Items, data.Run.ID
}

func i05Count(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestIteration05ChapterPlanHTTPPostgresEndToEnd(t *testing.T) {
	pool, ctx := openI05HTTP(t)
	f := i05FixtureData(t, ctx, pool)
	server := i05Server(pool)
	defer server.Close()
	client := server.Client()
	base := server.URL + "/api/v1"

	// 1. Mock generation is deterministic, persists a run and plans, and never calls an LLM.
	response, envelope := i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/mock-generate", i05MockBody(f, 1, 2))
	generated, runID := i05Generated(t, response, envelope, 2)
	if generated[0].Status != "pending_confirmation" || generated[0].Version != 1 || len(generated[0].StorylineRefsJSON) == 0 || len(generated[0].MaterialRefsJSON) != 2 || len(generated[0].ForeshadowingRefsJSON) != 2 {
		t.Fatalf("mock generation mapping: %s", envelope.Raw)
	}
	if i05Count(t, ctx, pool, "SELECT COUNT(*) FROM mock_generation_runs WHERE id=$1 AND provider_key='mock' AND workflow_key='chapter_plan_mock_generate'", runID) != 1 || i05Count(t, ctx, pool, "SELECT COUNT(*) FROM chapter_plans WHERE mock_generation_run_id=$1", runID) != 2 {
		t.Fatal("mock data not persisted")
	}
	response, envelope = i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/mock-generate", i05MockBody(f, 1, 2))
	i05RequireStatus(t, response, envelope, http.StatusConflict)
	if i05Count(t, ctx, pool, "SELECT COUNT(*) FROM mock_generation_runs WHERE project_id=$1", f.project) != 1 || i05Count(t, ctx, pool, "SELECT COUNT(*) FROM chapter_plans WHERE project_id=$1", f.project) != 2 {
		t.Fatal("conflicting generation created partial data")
	}

	// 2. List uses the frozen query contract and maps nulls/relations.
	response, envelope = i05Call(t, client, http.MethodGet, base+"/projects/"+f.project.String()+"/chapter-plans?status=pending_confirmation&limit=1&offset=1", nil)
	i05RequireStatus(t, response, envelope, http.StatusOK)
	var page i05List
	if err := json.Unmarshal(envelope.Data, &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Limit != 1 || page.Offset != 1 || len(page.Items) != 1 || page.Items[0].ChapterNo != 2 || page.Items[0].CreationNotes != nil {
		t.Fatalf("list contract: %s", envelope.Raw)
	}
	response, envelope = i05Call(t, client, http.MethodGet, base+"/projects/"+f.emptyProject.String()+"/chapter-plans", nil)
	i05RequireStatus(t, response, envelope, http.StatusOK)
	if !bytes.Contains(envelope.Raw, []byte(`"items":[]`)) {
		t.Fatalf("empty list: %s", envelope.Raw)
	}

	// 3. Single-item retrieval exposes timestamps, associations and version; missing is 404.
	first, second := generated[0], generated[1]
	response, envelope = i05Call(t, client, http.MethodGet, base+"/chapter-plans/"+first.ID.String(), nil)
	i05RequireStatus(t, response, envelope, http.StatusOK)
	var fetched i05Plan
	if err := json.Unmarshal(envelope.Data, &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.ID != first.ID || fetched.CreatedAt == "" || fetched.UpdatedAt == "" || fetched.Version != 1 || len(fetched.StorylineRefsJSON) == 0 {
		t.Fatalf("get mapping: %s", envelope.Raw)
	}
	response, envelope = i05Call(t, client, http.MethodGet, base+"/chapter-plans/"+uuid.New().String(), nil)
	i05RequireStatus(t, response, envelope, http.StatusNotFound)

	// 4. PATCH covers ordinary fields, nullable tri-state, complete association replacement and optimistic locking.
	patch := map[string]any{"expected_version": 1, "title": "edited", "chapter_goal": "", "creation_notes": "notes", "storyline_refs_json": []any{map[string]any{"storyline_id": f.storylines[1].String(), "relation": "primary"}}, "material_refs_json": []string{f.materials[1].String()}, "foreshadowing_refs_json": []string{f.fores[1].String()}}
	response, envelope = i05Call(t, client, http.MethodPatch, base+"/chapter-plans/"+first.ID.String(), patch)
	i05RequireStatus(t, response, envelope, http.StatusOK)
	if err := json.Unmarshal(envelope.Data, &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.Version != 2 || fetched.ChapterGoal == nil || *fetched.ChapterGoal != "" || fetched.CreationNotes == nil || *fetched.CreationNotes != "notes" || len(fetched.StorylineRefsJSON) != 1 || fetched.StorylineRefsJSON[0].StorylineID != f.storylines[1] || len(fetched.MaterialRefsJSON) != 1 || fetched.MaterialRefsJSON[0] != f.materials[1] || len(fetched.ForeshadowingRefsJSON) != 1 || fetched.ForeshadowingRefsJSON[0] != f.fores[1] {
		t.Fatalf("patch replacement: %s", envelope.Raw)
	}
	response, envelope = i05Call(t, client, http.MethodPatch, base+"/chapter-plans/"+first.ID.String(), map[string]any{"expected_version": 1, "title": "stale"})
	i05RequireStatus(t, response, envelope, http.StatusConflict)
	response, envelope = i05Call(t, client, http.MethodPatch, base+"/chapter-plans/"+first.ID.String(), map[string]any{"expected_version": 2, "chapter_goal": nil, "material_refs_json": []string{}, "foreshadowing_refs_json": []string{}})
	i05RequireStatus(t, response, envelope, http.StatusOK)
	if err := json.Unmarshal(envelope.Data, &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.Version != 3 || fetched.ChapterGoal != nil || fetched.CreationNotes == nil || *fetched.CreationNotes != "notes" || len(fetched.MaterialRefsJSON) != 0 || len(fetched.ForeshadowingRefsJSON) != 0 {
		t.Fatalf("patch nullable/clear: %s", envelope.Raw)
	}
	var goal, notes *string
	if err := pool.QueryRow(ctx, "SELECT chapter_goal,creation_notes FROM chapter_plans WHERE id=$1", first.ID).Scan(&goal, &notes); err != nil || goal != nil || notes == nil || *notes != "notes" {
		t.Fatalf("patch database state: goal=%v notes=%v err=%v", goal, notes, err)
	}

	// 5. DELETE is bodyless, cascades associations, and honors version/state guards.
	response, envelope = i05Call(t, client, http.MethodDelete, base+"/chapter-plans/"+second.ID.String()+"?expected_version=1", nil)
	i05RequireStatus(t, response, envelope, http.StatusNoContent)
	if len(envelope.Raw) != 0 || i05Count(t, ctx, pool, "SELECT COUNT(*) FROM chapter_plans WHERE id=$1", second.ID) != 0 || i05Count(t, ctx, pool, "SELECT COUNT(*) FROM chapter_plan_storylines WHERE chapter_plan_id=$1", second.ID) != 0 {
		t.Fatal("delete did not fully remove plan or returned a body")
	}
	response, envelope = i05Call(t, client, http.MethodDelete, base+"/chapter-plans/"+first.ID.String()+"?expected_version=2", nil)
	i05RequireStatus(t, response, envelope, http.StatusConflict)

	// 6. Confirm is batch-atomic and idempotent, without content-item writes.
	response, envelope = i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/mock-generate", i05MockBody(f, 3, 4))
	confirmedCandidates, _ := i05Generated(t, response, envelope, 2)
	selections := []map[string]any{{"chapter_plan_id": confirmedCandidates[0].ID.String(), "expected_version": 1}, {"chapter_plan_id": confirmedCandidates[1].ID.String(), "expected_version": 1}}
	response, envelope = i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/confirm", map[string]any{"selections": selections})
	i05RequireStatus(t, response, envelope, http.StatusOK)
	var confirmed i05List
	if err := json.Unmarshal(envelope.Data, &confirmed); err != nil {
		t.Fatal(err)
	}
	if len(confirmed.Items) != 2 || confirmed.Items[0].Status != "confirmed" || confirmed.Items[0].ConfirmedAt == nil || confirmed.Items[0].Version != 2 || i05Count(t, ctx, pool, "SELECT COUNT(*) FROM chapter_plans WHERE id=ANY($1) AND status='confirmed' AND confirmed_at IS NOT NULL AND version=2", []uuid.UUID{confirmedCandidates[0].ID, confirmedCandidates[1].ID}) != 2 {
		t.Fatalf("confirm: %s", envelope.Raw)
	}
	response, envelope = i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/confirm", map[string]any{"selections": selections})
	i05RequireStatus(t, response, envelope, http.StatusOK)
	response, envelope = i05Call(t, client, http.MethodPatch, base+"/chapter-plans/"+confirmedCandidates[0].ID.String(), map[string]any{"expected_version": 2, "title": "blocked"})
	i05RequireStatus(t, response, envelope, http.StatusConflict)
	response, envelope = i05Call(t, client, http.MethodDelete, base+"/chapter-plans/"+confirmedCandidates[0].ID.String()+"?expected_version=2", nil)
	i05RequireStatus(t, response, envelope, http.StatusConflict)

	response, envelope = i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/mock-generate", i05MockBody(f, 5, 6))
	rollbackCandidates, _ := i05Generated(t, response, envelope, 2)
	response, envelope = i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/confirm", map[string]any{"selections": []map[string]any{{"chapter_plan_id": rollbackCandidates[0].ID.String(), "expected_version": 1}, {"chapter_plan_id": rollbackCandidates[1].ID.String(), "expected_version": 99}}})
	i05RequireStatus(t, response, envelope, http.StatusConflict)
	response, envelope = i05Call(t, client, http.MethodPost, base+"/projects/"+f.project.String()+"/chapter-plans/confirm", map[string]any{"selections": []map[string]any{{"chapter_plan_id": rollbackCandidates[0].ID.String(), "expected_version": 1}, {"chapter_plan_id": uuid.New().String(), "expected_version": 1}}})
	i05RequireStatus(t, response, envelope, http.StatusNotFound)
	if i05Count(t, ctx, pool, "SELECT COUNT(*) FROM chapter_plans WHERE id=ANY($1) AND status='pending_confirmation' AND version=1", []uuid.UUID{rollbackCandidates[0].ID, rollbackCandidates[1].ID}) != 2 {
		t.Fatal("failed confirmation changed a pending item")
	}

	// Boundary errors are envelopes only and must not expose database details.
	for _, request := range []struct {
		method, url string
		body        any
		status      int
	}{
		{http.MethodPatch, base + "/chapter-plans/" + first.ID.String(), []byte("{"), http.StatusBadRequest},
		{http.MethodGet, base + "/chapter-plans/not-a-uuid", nil, http.StatusBadRequest},
		{http.MethodGet, base + "/projects/" + uuid.New().String() + "/chapter-plans", nil, http.StatusNotFound},
	} {
		response, envelope = i05Call(t, client, request.method, request.url, request.body)
		i05RequireStatus(t, response, envelope, request.status)
		lower := strings.ToLower(string(envelope.Raw))
		if strings.Contains(lower, "sql") || strings.Contains(lower, "postgres") || strings.Contains(lower, "password") {
			t.Fatalf("internal error leaked: %s", envelope.Raw)
		}
	}

	server.Close()
	server = i05Server(pool)
	defer server.Close()
	response, envelope = i05Call(t, server.Client(), http.MethodGet, server.URL+"/api/v1/chapter-plans/"+confirmedCandidates[0].ID.String(), nil)
	i05RequireStatus(t, response, envelope, http.StatusOK)
	if err := json.Unmarshal(envelope.Data, &fetched); err != nil || fetched.Status != "confirmed" || fetched.Version != 2 {
		t.Fatalf("restart persistence: %s err=%v", envelope.Raw, err)
	}
}

var _ = time.RFC3339Nano
