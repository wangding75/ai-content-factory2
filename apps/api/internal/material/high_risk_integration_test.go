package material

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func highRiskPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS test_material_audit_failure ON audit_logs; DROP FUNCTION IF EXISTS test_material_audit_failure()")
	})
	return pool, ctx
}

func highRiskProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO projects (id,name,type,created_by) VALUES ($1,$2,'novel','high-risk')", id, "high-risk "+id.String()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", id) })
	return id
}

func highRiskCreateRequest(name string) CreateRequest {
	summary, tag := "summary", "tag"
	return CreateRequest{Type: TypeItem, Name: &name, Summary: &summary, ContentJSON: json.RawMessage("{}"), Tags: &[]string{tag}}
}

func highRiskUsageRequest() ProjectMaterialUsageRequest {
	usage, role, notes := "lead", "role", "notes"
	return ProjectMaterialUsageRequest{UsageType: &usage, RoleName: &role, Notes: &notes}
}

func highRiskAuditFailure(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, "CREATE OR REPLACE FUNCTION test_material_audit_failure() RETURNS trigger AS $$ BEGIN IF NEW.actor_id = 'high-risk' THEN RAISE EXCEPTION 'forced audit failure'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql; CREATE TRIGGER test_material_audit_failure BEFORE INSERT ON audit_logs FOR EACH ROW WHEN (NEW.actor_id = 'high-risk') EXECUTE FUNCTION test_material_audit_failure();")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = pool.Exec(cleanCtx, "DROP TRIGGER IF EXISTS test_material_audit_failure ON audit_logs; DROP FUNCTION IF EXISTS test_material_audit_failure()")
	})
}

func countHighRisk(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, where string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table+" WHERE "+where, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMaterialHighRiskConcurrentIdempotency(t *testing.T) {
	pool, ctx := highRiskPool(t)
	global := NewService(pool)
	runGlobal := func() (Material, error) {
		return global.CreateMaterial(ctx, highRiskCreateRequest("concurrent-global"), "concurrent-global-"+uuid.New().String(), "high-risk")
	}
	_ = runGlobal
	key := "concurrent-global-" + uuid.New().String()
	results := make([]Material, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = global.CreateMaterial(ctx, highRiskCreateRequest("concurrent-global"), key, "high-risk")
		}(i)
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("global create error: %v", err)
		}
	}
	if results[0].ID != results[1].ID {
		t.Fatalf("global IDs differ: %s %s", results[0].ID, results[1].ID)
	}
	if countHighRisk(t, ctx, pool, "materials", "id=$1", results[0].ID) != 1 || countHighRisk(t, ctx, pool, "audit_logs", "subject_id=$1 AND action='material.created'", results[0].ID.String()) != 1 || countHighRisk(t, ctx, pool, "idempotency_records", "scope='material:create' AND idempotency_key=$1", key) != 1 {
		t.Fatal("global final counts are not 1/1/1")
	}

	projectID := highRiskProject(t, ctx, pool)
	projectService := NewPostgresProjectMaterialService(projectMaterialTestProjects{}, pool)
	projectKey := "concurrent-project-" + uuid.New().String()
	items := make([]ProjectMaterialItem, 2)
	errs = make([]error, 2)
	start = make(chan struct{})
	wg = sync.WaitGroup{}
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			items[i], errs[i] = projectService.CreateAndBindMaterial(ctx, projectID, CreateProjectMaterialRequest{Material: highRiskCreateRequest("concurrent-project"), Usage: highRiskUsageRequest()}, projectKey, "high-risk")
		}(i)
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("project create error: %v", err)
		}
	}
	if items[0].Material.ID != items[1].Material.ID || items[0].Usage.ID != items[1].Usage.ID {
		t.Fatalf("project results differ: %#v %#v", items[0], items[1])
	}
	if countHighRisk(t, ctx, pool, "materials", "id=$1", items[0].Material.ID) != 1 || countHighRisk(t, ctx, pool, "project_material_usages", "id=$1", items[0].Usage.ID) != 1 || countHighRisk(t, ctx, pool, "audit_logs", "subject_id IN ($1,$2)", items[0].Material.ID.String(), items[0].Usage.ID.String()) != 2 || countHighRisk(t, ctx, pool, "idempotency_records", "scope='project_material:create' AND idempotency_key=$1", projectKey) != 1 {
		t.Fatal("project final counts are not 1/1/2/1")
	}
}

func TestMaterialHighRiskReferencesAndPatchIsolation(t *testing.T) {
	pool, ctx := highRiskPool(t)
	repo := NewPostgresRepository(pool)
	service := NewService(pool)
	projects := []uuid.UUID{highRiskProject(t, ctx, pool), highRiskProject(t, ctx, pool), highRiskProject(t, ctx, pool)}
	m := Material{ID: uuid.New(), Type: TypeItem, Name: "shared", Summary: "summary", ContentJSON: json.RawMessage("{}"), Tags: []string{"tag"}, CreatedBy: "high-risk"}
	created, err := repo.Create(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = repo.CreateUsage(ctx, ProjectMaterialUsage{ID: uuid.New(), ProjectID: projects[i], MaterialID: created.ID, UsageType: "lead", RoleName: "role", Notes: "private", Status: StatusActive, CreatedBy: "high-risk"}); err != nil {
			t.Fatal(err)
		}
	}
	detail, err := service.GetMaterial(ctx, created.ID)
	if err != nil || detail.ReferenceCount != 2 || len(detail.References) != 2 {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	seen := map[uuid.UUID]bool{}
	for _, ref := range detail.References {
		if seen[ref.ProjectID] || ref.ProjectName == "" || ref.ProjectType != "novel" {
			t.Fatalf("bad ref %#v", ref)
		}
		seen[ref.ProjectID] = true
	}
	body, _ := json.Marshal(detail)
	for _, private := range []string{"usage_type", "role_name", "notes", "start_chapter", "end_chapter", "status"} {
		if string(body) != "" && containsJSONField(body, private) {
			t.Fatalf("leaked usage field %s: %s", private, body)
		}
	}
	before := make([]ProjectMaterialUsage, 2)
	for i := range before {
		before[i], err = repo.GetByProjectAndMaterial(ctx, projects[i], created.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	name := "shared updated"
	version := created.Version
	updated, err := service.UpdateMaterial(ctx, created.ID, UpdateRequest{ExpectedVersion: &version, Name: &name}, "high-risk")
	if err != nil {
		t.Fatal(err)
	}
	for i := range before {
		after, getErr := repo.GetByProjectAndMaterial(ctx, projects[i], created.ID)
		if getErr != nil || !sameUsage(before[i], after) || before[i].Version != after.Version || !before[i].UpdatedAt.Equal(after.UpdatedAt) {
			t.Fatalf("usage changed before=%#v after=%#v err=%v", before[i], after, getErr)
		}
	}
	for _, projectID := range projects[:2] {
		items, _, getErr := repo.ListProjectMaterials(ctx, projectID, ListOptions{Limit: 20})
		if getErr != nil || len(items) != 1 || items[0].Material.ID != updated.ID || items[0].Material.Name != name {
			t.Fatalf("project=%s items=%#v err=%v", projectID, items, getErr)
		}
	}
	auditBefore := countHighRisk(t, ctx, pool, "audit_logs", "subject_id=$1 AND action='material.updated'", created.ID.String())
	unchanged, err := service.UpdateMaterial(ctx, created.ID, UpdateRequest{ExpectedVersion: &updated.Version, Name: &name}, "high-risk")
	if err != nil || unchanged.Version != updated.Version || !unchanged.UpdatedAt.Equal(updated.UpdatedAt) || countHighRisk(t, ctx, pool, "audit_logs", "subject_id=$1 AND action='material.updated'", created.ID.String()) != auditBefore {
		t.Fatalf("unchanged=%#v err=%v", unchanged, err)
	}
}

func containsJSONField(body []byte, field string) bool {
	return string(body) != "" && (string(body) == "\""+field+"\"" || len(body) > 0 && jsonContains(body, field))
}
func jsonContains(body []byte, field string) bool {
	var value map[string]any
	_ = json.Unmarshal(body, &value)
	encoded, _ := json.Marshal(value)
	return string(encoded) != "" && containsString(string(encoded), "\""+field+"\":")
}
func containsString(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestMaterialHighRiskAuditRollback(t *testing.T) {
	pool, ctx := highRiskPool(t)
	global := NewService(pool)
	repo := NewPostgresRepository(pool)
	projects := NewPostgresProjectMaterialService(projectMaterialTestProjects{}, pool)
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS test_material_audit_failure ON audit_logs; DROP FUNCTION IF EXISTS test_material_audit_failure()")
	}()
	seed := func(t *testing.T, projectID uuid.UUID, name string) (Material, ProjectMaterialUsage) {
		t.Helper()
		m, err := repo.Create(ctx, Material{ID: uuid.New(), Type: TypeItem, Name: name, Summary: "summary", ContentJSON: json.RawMessage("{}"), Tags: []string{"tag"}, CreatedBy: "high-risk"})
		if err != nil {
			t.Fatal(err)
		}
		u, err := repo.CreateUsage(ctx, ProjectMaterialUsage{ID: uuid.New(), ProjectID: projectID, MaterialID: m.ID, UsageType: "lead", RoleName: "role", Notes: "notes", Status: StatusActive, CreatedBy: "high-risk"})
		if err != nil {
			t.Fatal(err)
		}
		return m, u
	}
	t.Run("create global", func(t *testing.T) {
		key := "rollback-global-" + uuid.New().String()
		highRiskAuditFailure(t, ctx, pool)
		_, err := global.CreateMaterial(ctx, highRiskCreateRequest("rollback global"), key, "high-risk")
		if err == nil {
			t.Fatal("expected audit failure")
		}
		if countHighRisk(t, ctx, pool, "materials", "created_by='high-risk' AND name='rollback global'") != 0 || countHighRisk(t, ctx, pool, "idempotency_records", "scope='material:create' AND idempotency_key=$1", key) != 0 || countHighRisk(t, ctx, pool, "audit_logs", "action='material.created' AND payload->'after'->>'name'='rollback global'") != 0 {
			t.Fatal("global rollback left rows")
		}
	})
	t.Run("patch global", func(t *testing.T) {
		projectID := highRiskProject(t, ctx, pool)
		before, _ := seed(t, projectID, "rollback patch")
		highRiskAuditFailure(t, ctx, pool)
		name, version := "changed", before.Version
		if _, err := global.UpdateMaterial(ctx, before.ID, UpdateRequest{ExpectedVersion: &version, Name: &name}, "high-risk"); err == nil {
			t.Fatal("expected audit failure")
		}
		after, err := repo.GetByID(ctx, before.ID)
		if err != nil || after.Name != before.Name || after.Version != before.Version || !after.UpdatedAt.Equal(before.UpdatedAt) || countHighRisk(t, ctx, pool, "audit_logs", "subject_id=$1 AND action='material.updated'", before.ID.String()) != 0 {
			t.Fatalf("after=%#v err=%v", after, err)
		}
	})
	t.Run("create project material", func(t *testing.T) {
		projectID := highRiskProject(t, ctx, pool)
		key := "rollback-project-" + uuid.New().String()
		highRiskAuditFailure(t, ctx, pool)
		if _, err := projects.CreateAndBindMaterial(ctx, projectID, CreateProjectMaterialRequest{Material: highRiskCreateRequest("rollback project"), Usage: highRiskUsageRequest()}, key, "high-risk"); err == nil {
			t.Fatal("expected audit failure")
		}
		if countHighRisk(t, ctx, pool, "materials", "created_by='high-risk' AND name='rollback project'") != 0 || countHighRisk(t, ctx, pool, "project_material_usages", "project_id=$1", projectID) != 0 || countHighRisk(t, ctx, pool, "idempotency_records", "scope='project_material:create' AND idempotency_key=$1", key) != 0 || countHighRisk(t, ctx, pool, "audit_logs", "action IN ('material.created','project_material.bound') AND payload::text LIKE '%rollback project%'") != 0 {
			t.Fatal("project create rollback left rows")
		}
	})
	t.Run("patch usage", func(t *testing.T) {
		projectID := highRiskProject(t, ctx, pool)
		materialValue, before := seed(t, projectID, "rollback usage")
		notes, version := "changed", before.Version
		highRiskAuditFailure(t, ctx, pool)
		if _, err := projects.UpdateProjectMaterialUsage(ctx, projectID, materialValue.ID, UpdateProjectMaterialUsageRequest{ExpectedVersion: &version, Notes: &notes}, "high-risk"); err == nil {
			t.Fatal("expected audit failure")
		}
		after, err := repo.GetByProjectAndMaterial(ctx, projectID, materialValue.ID)
		if err != nil || after.Notes != before.Notes || after.Version != before.Version || !after.UpdatedAt.Equal(before.UpdatedAt) || countHighRisk(t, ctx, pool, "audit_logs", "subject_id=$1 AND action='project_material.usage_updated'", before.ID.String()) != 0 {
			t.Fatalf("after=%#v err=%v", after, err)
		}
	})
	t.Run("unbind usage", func(t *testing.T) {
		projectID := highRiskProject(t, ctx, pool)
		materialValue, before := seed(t, projectID, "rollback unbind")
		highRiskAuditFailure(t, ctx, pool)
		if _, err := projects.UnbindProjectMaterial(ctx, projectID, materialValue.ID, before.Version, "high-risk"); err == nil {
			t.Fatal("expected audit failure")
		}
		after, err := repo.GetByProjectAndMaterial(ctx, projectID, materialValue.ID)
		if err != nil || after.ID != before.ID || after.Version != before.Version || !after.UpdatedAt.Equal(before.UpdatedAt) || countHighRisk(t, ctx, pool, "audit_logs", "subject_id=$1 AND action='project_material.unbound'", before.ID.String()) != 0 {
			t.Fatalf("after=%#v err=%v", after, err)
		}
	})
}
