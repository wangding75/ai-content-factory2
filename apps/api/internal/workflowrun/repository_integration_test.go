package workflowrun

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test skipped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	db, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db, ctx
}
func fixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	p, c, w := uuid.New(), uuid.New(), uuid.New()
	_, err := db.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','workflowrun-test')", p, "workflowrun-"+p.String()[:8])
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES($1,$2,'n8n','http://localhost','api_key',30,'{}')", c, "workflowrun-"+c.String()[:8])
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version) VALUES($1,$2,$3,'[\"review\"]','{}','v1','v1')", w, "workflowrun-"+w.String()[:8], c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", p)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", w)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", c)
	})
	return p, w
}
func newRun(t *testing.T, p, w uuid.UUID, n string) WorkflowRun {
	t.Helper()
	v, err := New(uuid.New(), p, w, n, "review", "project", json.RawMessage(`{"connection":{"type":"n8n"}}`), json.RawMessage(`{"content":"safe"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestRepositoryCRUDEventsAndSummary(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	first, err := repo.Create(ctx, newRun(t, p, w, "WR-001"))
	if err != nil {
		t.Fatal(err)
	}
	running, err := first.Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	running, err = repo.UpdateStatus(ctx, running)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := running.Fail(time.Now().UTC(), Failure{Code: "TIMEOUT", Message: "request timed out", Details: json.RawMessage(`{"safe":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.UpdateStatus(ctx, failed); err != nil {
		t.Fatal(err)
	}
	event, err := repo.AddEvent(ctx, Event{ID: uuid.New(), RunID: first.ID, EventType: "failed", Status: StatusFailed, Payload: json.RawMessage(`{"safe":true}`), CreatedAt: time.Now().UTC()})
	if err != nil || event.ID == uuid.Nil {
		t.Fatalf("add event=%+v err=%v", event, err)
	}
	got, err := repo.GetByID(ctx, first.ID)
	if err != nil || got.Status != StatusFailed {
		t.Fatalf("get=%+v err=%v", got, err)
	}
	list, err := repo.List(ctx, ListFilter{ProjectID: &p, Status: string(StatusFailed)})
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}
	summary, err := repo.QuerySummary(ctx, p, 5)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRuns != 1 || summary.RunningCount != 0 || summary.LatestFailure == nil || summary.LatestRun == nil || len(summary.RecentRuns) != 1 {
		t.Fatalf("summary=%+v", summary)
	}
}
