package material

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
)

type UnbindProjectMaterialResult struct {
	ProjectID        uuid.UUID `json:"project_id"`
	MaterialID       uuid.UUID `json:"material_id"`
	Unbound          bool      `json:"unbound"`
	MaterialRetained bool      `json:"material_retained"`
}

func (s *ProjectMaterialService) UnbindProjectMaterial(ctx context.Context, projectID, materialID uuid.UUID, expectedVersion int, actor string) (UnbindProjectMaterialResult, error) {
	result := UnbindProjectMaterialResult{ProjectID: projectID, MaterialID: materialID, MaterialRetained: true}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return UnbindProjectMaterialResult{}, err
	}
	if s.pool == nil || expectedVersion < 1 {
		return UnbindProjectMaterialResult{}, ErrValidation
	}
	if _, err := NewPostgresRepository(s.pool).GetByID(ctx, materialID); err != nil {
		return UnbindProjectMaterialResult{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return UnbindProjectMaterialResult{}, err
	}
	defer tx.Rollback(ctx)
	repo := NewPostgresRepositoryTx(tx)
	current, err := repo.GetByProjectAndMaterial(ctx, projectID, materialID)
	if err == ErrUsageNotFound {
		if err := tx.Commit(ctx); err != nil {
			return UnbindProjectMaterialResult{}, err
		}
		return result, nil
	}
	if err != nil {
		return UnbindProjectMaterialResult{}, err
	}
	if current.Version != expectedVersion {
		return UnbindProjectMaterialResult{}, ErrVersionConflict
	}
	if err := repo.DeleteUsageWithVersion(ctx, projectID, materialID, expectedVersion); err != nil {
		return UnbindProjectMaterialResult{}, err
	}
	payload, _ := json.Marshal(map[string]any{"project_id": projectID, "material_id": materialID, "usage_id": current.ID, "before": current, "after": nil})
	if err := audit.NewRepository(tx).Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: actor, Action: "project_material.unbound", SubjectType: "project_material_usage", SubjectID: current.ID.String(), Payload: payload}); err != nil {
		return UnbindProjectMaterialResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return UnbindProjectMaterialResult{}, err
	}
	result.Unbound = true
	return result, nil
}
