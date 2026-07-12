package planning

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

func TestProjectPlanningServiceIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	id := uuid.New()
	if _, err = pool.Exec(ctx, "INSERT INTO projects (id,name,type,created_by) VALUES ($1,$2,'novel','planning-service-test')", id, "planning service"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE subject_type='project_planning' AND subject_id=$1", id.String())
		_, _ = pool.Exec(context.Background(), "DELETE FROM project_plannings WHERE project_id=$1", id)
		_, _ = pool.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", id)
	}()
	projects := project.NewPostgresRepository(pool)
	service := NewPostgresService(projects, pool)
	audience, style := "readers", "plain"
	premise := "first"
	zero := 0
	request := SaveRequest{Premise: &premise, Audience: &audience, Style: &style, ExpectedVersion: &zero, GoalsJSON: json.RawMessage("{\"selling_points\":[\"hook\"],\"plot_summary\":\"summary\"}"), ConstraintsJSON: json.RawMessage("{\"emotional_tone\":\"warm\"}")}
	created, err := service.PutProjectPlanning(ctx, id, request, "integration")
	if err != nil || created.Version != 1 {
		t.Fatalf("create=%#v err=%v", created, err)
	}
	got, err := service.GetProjectPlanning(ctx, id)
	if err != nil || got.Premise != "first" {
		t.Fatalf("get=%#v err=%v", got, err)
	}
	premise = "second"
	one := 1
	request.Premise = &premise
	request.ExpectedVersion = &one
	updated, err := service.PutProjectPlanning(ctx, id, request, "integration")
	if err != nil || updated.Version != 2 {
		t.Fatalf("update=%#v err=%v", updated, err)
	}
	if _, err = service.PutProjectPlanning(ctx, id, request, "integration"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict=%v", err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE subject_type='project_planning' AND subject_id=$1", id.String()).Scan(&count); err != nil || count != 2 {
		t.Fatalf("audits=%d err=%v", count, err)
	}
}
