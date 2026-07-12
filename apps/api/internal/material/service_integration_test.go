package material

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMaterialServiceIntegrationCreateReplayDetailAndUpdate(t *testing.T) {
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
	service := NewService(pool)
	name, summary := "integration material", "summary"
	tags := []string{"integration"}
	req := CreateRequest{Type: TypeItem, Name: &name, Summary: &summary, ContentJSON: json.RawMessage("{}"), Tags: &tags}
	key := "material-service-" + uuid.New().String()
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE subject_type='material' AND actor_id='integration'")
		_, _ = pool.Exec(context.Background(), "DELETE FROM idempotency_records WHERE scope='material:create' AND idempotency_key=$1", key)
		_, _ = pool.Exec(context.Background(), "DELETE FROM materials WHERE created_by='integration'")
	}()
	first, err := service.CreateMaterial(ctx, req, key, "integration")
	if err != nil || first.Version != 1 {
		t.Fatalf("create %#v %v", first, err)
	}
	replay, err := service.CreateMaterial(ctx, req, key, "integration")
	if err != nil || replay.ID != first.ID {
		t.Fatalf("replay %#v %v", replay, err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM materials WHERE id=$1", first.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("material count %d %v", count, err)
	}
	detail, err := service.GetMaterial(ctx, first.ID)
	if err != nil || detail.ReferenceCount != 0 {
		t.Fatalf("detail %#v %v", detail, err)
	}
	changed := "changed"
	version := first.Version
	updated, err := service.UpdateMaterial(ctx, first.ID, UpdateRequest{ExpectedVersion: &version, Name: &changed}, "integration")
	if err != nil || updated.Version != 2 || updated.CreatedBy != first.CreatedBy {
		t.Fatalf("update %#v %v", updated, err)
	}
	if _, err = service.UpdateMaterial(ctx, first.ID, UpdateRequest{ExpectedVersion: &version, Name: &changed}, "integration"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict %v", err)
	}
}
