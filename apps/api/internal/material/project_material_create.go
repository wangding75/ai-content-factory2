package material

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
)

const createProjectMaterialScope = "project_material:create"

type ProjectMaterialUsageRequest struct {
	UsageType    *string `json:"usage_type"`
	RoleName     *string `json:"role_name"`
	Notes        *string `json:"notes"`
	StartChapter *int    `json:"start_chapter"`
	EndChapter   *int    `json:"end_chapter"`
}

type CreateProjectMaterialRequest struct {
	Material CreateRequest               `json:"material"`
	Usage    ProjectMaterialUsageRequest `json:"usage"`
}

func NewPostgresProjectMaterialService(projects projectFinder, pool *pgxpool.Pool) *ProjectMaterialService {
	return &ProjectMaterialService{projects: projects, repo: NewPostgresRepository(pool), pool: pool}
}

func (s *ProjectMaterialService) CreateAndBindMaterial(ctx context.Context, projectID uuid.UUID, request CreateProjectMaterialRequest, key, actor string) (ProjectMaterialItem, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return ProjectMaterialItem{}, err
	}
	if s.pool == nil || strings.TrimSpace(key) == "" || len(key) > 128 {
		return ProjectMaterialItem{}, ErrValidation
	}
	materialValue, err := createValue(request.Material, actor)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	usageValue, err := createUsageValue(projectID, materialValue.ID, request.Usage, actor)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	hash, err := projectMaterialRequestHash(projectID, request)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	idem := idempotency.NewPostgresRepository(s.pool)
	if record, err := idem.Get(ctx, createProjectMaterialScope, key); err == nil {
		if record.RequestHash != hash {
			return ProjectMaterialItem{}, ErrIdempotencyReused
		}
		var item ProjectMaterialItem
		if err := json.Unmarshal(record.ResponseBody, &item); err != nil {
			return ProjectMaterialItem{}, err
		}
		return item, nil
	} else if !errors.Is(err, idempotency.ErrNotFound) {
		return ProjectMaterialItem{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	defer tx.Rollback(ctx)
	repo, txIdem := NewPostgresRepositoryTx(tx), idempotency.NewPostgresRepositoryTx(tx)
	if record, err := txIdem.Get(ctx, createProjectMaterialScope, key); err == nil {
		if record.RequestHash != hash {
			return ProjectMaterialItem{}, ErrIdempotencyReused
		}
		var item ProjectMaterialItem
		if err := json.Unmarshal(record.ResponseBody, &item); err != nil {
			return ProjectMaterialItem{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ProjectMaterialItem{}, err
		}
		return item, nil
	} else if !errors.Is(err, idempotency.ErrNotFound) {
		return ProjectMaterialItem{}, err
	}
	createdMaterial, err := repo.Create(ctx, materialValue)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	createdUsage, err := repo.CreateUsage(ctx, usageValue)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	auditRepo := audit.NewRepository(tx)
	materialPayload, _ := json.Marshal(map[string]any{"before": nil, "after": createdMaterial})
	if err := auditRepo.Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: actor, Action: "material.created", SubjectType: "material", SubjectID: createdMaterial.ID.String(), Payload: materialPayload}); err != nil {
		return ProjectMaterialItem{}, err
	}
	usagePayload, _ := json.Marshal(map[string]any{"project_id": projectID, "material_id": createdMaterial.ID, "usage_id": createdUsage.ID, "usage": createdUsage})
	if err := auditRepo.Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: actor, Action: "project_material.bound", SubjectType: "project_material_usage", SubjectID: createdUsage.ID.String(), Payload: usagePayload}); err != nil {
		return ProjectMaterialItem{}, err
	}
	result := ProjectMaterialItem{Material: createdMaterial, Usage: createdUsage, LastUpdatedAt: later(createdMaterial.UpdatedAt, createdUsage.UpdatedAt)}
	body, err := json.Marshal(result)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	if _, err := txIdem.Create(ctx, idempotency.Record{ID: uuid.New(), Scope: createProjectMaterialScope, Key: key, RequestHash: hash, ResponseStatus: 201, ResponseBody: body}); err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			_ = tx.Rollback(ctx)
			record, getErr := idem.Get(ctx, createProjectMaterialScope, key)
			if getErr == nil && record.RequestHash == hash {
				var replay ProjectMaterialItem
				if getErr = json.Unmarshal(record.ResponseBody, &replay); getErr == nil {
					return replay, nil
				}
			}
			if getErr != nil {
				return ProjectMaterialItem{}, getErr
			}
			return ProjectMaterialItem{}, ErrIdempotencyReused
		}
		return ProjectMaterialItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectMaterialItem{}, err
	}
	return result, nil
}

func createUsageValue(projectID, materialID uuid.UUID, request ProjectMaterialUsageRequest, actor string) (ProjectMaterialUsage, error) {
	if request.UsageType == nil || request.RoleName == nil || request.Notes == nil {
		return ProjectMaterialUsage{}, ErrValidation
	}
	if strings.TrimSpace(*request.UsageType) == "" || len(*request.UsageType) > 120 || len(*request.RoleName) > 120 || len(*request.Notes) > 300 || request.StartChapter != nil && *request.StartChapter < 1 || request.EndChapter != nil && *request.EndChapter < 1 || request.StartChapter != nil && request.EndChapter != nil && *request.StartChapter > *request.EndChapter {
		return ProjectMaterialUsage{}, ErrValidation
	}
	return ProjectMaterialUsage{ID: uuid.New(), ProjectID: projectID, MaterialID: materialID, UsageType: *request.UsageType, RoleName: *request.RoleName, Notes: *request.Notes, StartChapter: request.StartChapter, EndChapter: request.EndChapter, Status: StatusActive, CreatedBy: actor}, nil
}

func projectMaterialRequestHash(projectID uuid.UUID, request CreateProjectMaterialRequest) (string, error) {
	body, err := json.Marshal(struct {
		ProjectID uuid.UUID                    `json:"project_id"`
		Request   CreateProjectMaterialRequest `json:"request"`
	}{projectID, request})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
