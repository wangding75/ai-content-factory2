package material

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
)

type UpdateProjectMaterialUsageRequest struct {
	ExpectedVersion *int            `json:"expected_version"`
	UsageType       *string         `json:"usage_type"`
	RoleName        *string         `json:"role_name"`
	Notes           *string         `json:"notes"`
	StartChapter    json.RawMessage `json:"start_chapter"`
	EndChapter      json.RawMessage `json:"end_chapter"`
}

func (s *ProjectMaterialService) UpdateProjectMaterialUsage(ctx context.Context, projectID, materialID uuid.UUID, request UpdateProjectMaterialUsageRequest, actor string) (ProjectMaterialItem, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return ProjectMaterialItem{}, err
	}
	if s.pool == nil {
		return ProjectMaterialItem{}, ErrValidation
	}
	materialValue, err := NewPostgresRepository(s.pool).GetByID(ctx, materialID)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	if request.ExpectedVersion == nil || *request.ExpectedVersion < 1 || request.UsageType == nil && request.RoleName == nil && request.Notes == nil && request.StartChapter == nil && request.EndChapter == nil {
		return ProjectMaterialItem{}, ErrValidation
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	defer tx.Rollback(ctx)
	repo := NewPostgresRepositoryTx(tx)
	current, err := repo.GetByProjectAndMaterial(ctx, projectID, materialID)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	if current.Version != *request.ExpectedVersion {
		return ProjectMaterialItem{}, ErrVersionConflict
	}
	next, err := mergeUsage(current, request)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	if sameUsage(current, next) {
		if err := tx.Commit(ctx); err != nil {
			return ProjectMaterialItem{}, err
		}
		return ProjectMaterialItem{Material: materialValue, Usage: current, LastUpdatedAt: later(materialValue.UpdatedAt, current.UpdatedAt)}, nil
	}
	updated, err := repo.UpdateUsageWithVersion(ctx, next, *request.ExpectedVersion)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	payload, _ := json.Marshal(map[string]any{"project_id": projectID, "material_id": materialID, "usage_id": updated.ID, "before": current, "after": updated})
	if err := audit.NewRepository(tx).Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: actor, Action: "project_material.usage_updated", SubjectType: "project_material_usage", SubjectID: updated.ID.String(), Payload: payload}); err != nil {
		return ProjectMaterialItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProjectMaterialItem{}, err
	}
	return ProjectMaterialItem{Material: materialValue, Usage: updated, LastUpdatedAt: later(materialValue.UpdatedAt, updated.UpdatedAt)}, nil
}

func mergeUsage(current ProjectMaterialUsage, request UpdateProjectMaterialUsageRequest) (ProjectMaterialUsage, error) {
	next := current
	if request.UsageType != nil {
		next.UsageType = *request.UsageType
	}
	if request.RoleName != nil {
		next.RoleName = *request.RoleName
	}
	if request.Notes != nil {
		next.Notes = *request.Notes
	}
	start, supplied, err := chapterValue(request.StartChapter)
	if err != nil {
		return ProjectMaterialUsage{}, err
	}
	if supplied {
		next.StartChapter = start
	}
	end, supplied, err := chapterValue(request.EndChapter)
	if err != nil {
		return ProjectMaterialUsage{}, err
	}
	if supplied {
		next.EndChapter = end
	}
	if next.UsageType == "" || len(next.UsageType) > 120 || len(next.RoleName) > 120 || len(next.Notes) > 300 || next.StartChapter != nil && *next.StartChapter < 1 || next.EndChapter != nil && *next.EndChapter < 1 || next.StartChapter != nil && next.EndChapter != nil && *next.StartChapter > *next.EndChapter {
		return ProjectMaterialUsage{}, ErrValidation
	}
	return next, nil
}

func chapterValue(raw json.RawMessage) (*int, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		return nil, true, nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil || value < 1 {
		return nil, false, ErrValidation
	}
	return &value, true, nil
}

func sameUsage(a, b ProjectMaterialUsage) bool {
	return a.UsageType == b.UsageType && a.RoleName == b.RoleName && a.Notes == b.Notes && equalChapter(a.StartChapter, b.StartChapter) && equalChapter(a.EndChapter, b.EndChapter)
}

func equalChapter(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
