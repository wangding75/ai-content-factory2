package material

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
)

const bindProjectMaterialScope = "project_material:bind"

func (s *ProjectMaterialService) BindExistingMaterial(ctx context.Context, projectID, materialID uuid.UUID, request ProjectMaterialUsageRequest, key, actor string) (ProjectMaterialItem, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return ProjectMaterialItem{}, err
	}
	if s.pool == nil || strings.TrimSpace(key) == "" || len(key) > 128 {
		return ProjectMaterialItem{}, ErrValidation
	}
	materialValue, err := NewPostgresRepository(s.pool).GetByID(ctx, materialID)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	usageValue, err := createUsageValue(projectID, materialID, request, actor)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	hash, err := bindProjectMaterialHash(projectID, materialID, request)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	idem := idempotency.NewPostgresRepository(s.pool)
	if record, err := idem.Get(ctx, bindProjectMaterialScope, key); err == nil {
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
	repo := NewPostgresRepositoryTx(tx)
	txIdem := idempotency.NewPostgresRepositoryTx(tx)
	if record, err := txIdem.Get(ctx, bindProjectMaterialScope, key); err == nil {
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
	exists, err := repo.ExistsByProjectAndMaterial(ctx, projectID, materialID)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	if exists {
		return ProjectMaterialItem{}, ErrAlreadyBound
	}
	createdUsage, err := repo.CreateUsage(ctx, usageValue)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	payload, _ := json.Marshal(map[string]any{"project_id": projectID, "material_id": materialID, "usage_id": createdUsage.ID, "usage": createdUsage})
	if err := audit.NewRepository(tx).Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: actor, Action: "project_material.bound", SubjectType: "project_material_usage", SubjectID: createdUsage.ID.String(), Payload: payload}); err != nil {
		return ProjectMaterialItem{}, err
	}
	result := ProjectMaterialItem{Material: materialValue, Usage: createdUsage, LastUpdatedAt: later(materialValue.UpdatedAt, createdUsage.UpdatedAt)}
	body, err := json.Marshal(result)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	if _, err := txIdem.Create(ctx, idempotency.Record{ID: uuid.New(), Scope: bindProjectMaterialScope, Key: key, RequestHash: hash, ResponseStatus: 201, ResponseBody: body}); err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			return ProjectMaterialItem{}, ErrIdempotencyReused
		}
		return ProjectMaterialItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectMaterialItem{}, err
	}
	return result, nil
}

func bindProjectMaterialHash(projectID, materialID uuid.UUID, request ProjectMaterialUsageRequest) (string, error) {
	return projectMaterialRequestHash(projectID, CreateProjectMaterialRequest{Usage: request, Material: CreateRequest{Type: materialID.String()}})
}
