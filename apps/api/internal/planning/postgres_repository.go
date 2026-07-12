package planning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db queryer }

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: pool}
}
func NewPostgresRepositoryTx(tx pgx.Tx) *PostgresRepository { return &PostgresRepository{db: tx} }

const columns = "project_id, premise, audience, style, goals_json, constraints_json, created_by, version, created_at, updated_at"

func scan(row pgx.Row) (ProjectPlanning, error) {
	var value ProjectPlanning
	err := row.Scan(&value.ProjectID, &value.Premise, &value.Audience, &value.Style, &value.GoalsJSON, &value.ConstraintsJSON, &value.CreatedBy, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return ProjectPlanning{}, err
	}
	if !json.Valid(value.GoalsJSON) || !json.Valid(value.ConstraintsJSON) {
		return ProjectPlanning{}, ErrInvalidJSON
	}
	return value, nil
}
func validateJSON(value ProjectPlanning) error {
	if !json.Valid(value.GoalsJSON) || !json.Valid(value.ConstraintsJSON) {
		return ErrInvalidJSON
	}
	return nil
}
func (r *PostgresRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) (ProjectPlanning, error) {
	value, err := scan(r.db.QueryRow(ctx, "SELECT "+columns+" FROM project_plannings WHERE project_id=$1", projectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectPlanning{}, ErrNotFound
	}
	if err != nil {
		return ProjectPlanning{}, fmt.Errorf("get project planning: %w", err)
	}
	return value, nil
}
func (r *PostgresRepository) Create(ctx context.Context, value ProjectPlanning) (ProjectPlanning, error) {
	if err := validateJSON(value); err != nil {
		return ProjectPlanning{}, err
	}
	created, err := scan(r.db.QueryRow(ctx, "INSERT INTO project_plannings (project_id,premise,audience,style,goals_json,constraints_json,created_by,version) VALUES ($1,$2,$3,$4,$5,$6,$7,1) RETURNING "+columns, value.ProjectID, value.Premise, value.Audience, value.Style, value.GoalsJSON, value.ConstraintsJSON, value.CreatedBy))
	if err != nil {
		return ProjectPlanning{}, fmt.Errorf("create project planning: %w", err)
	}
	return created, nil
}
func (r *PostgresRepository) UpdateWithVersion(ctx context.Context, value ProjectPlanning, expectedVersion int) (ProjectPlanning, error) {
	if err := validateJSON(value); err != nil {
		return ProjectPlanning{}, err
	}
	updated, err := scan(r.db.QueryRow(ctx, "UPDATE project_plannings SET premise=$2,audience=$3,style=$4,goals_json=$5,constraints_json=$6,version=version+1,updated_at=NOW() WHERE project_id=$1 AND version=$7 RETURNING "+columns, value.ProjectID, value.Premise, value.Audience, value.Style, value.GoalsJSON, value.ConstraintsJSON, expectedVersion))
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return ProjectPlanning{}, fmt.Errorf("update project planning: %w", err)
		}
		return updated, nil
	}
	_, getErr := r.GetByProjectID(ctx, value.ProjectID)
	if errors.Is(getErr, ErrNotFound) {
		return ProjectPlanning{}, ErrNotFound
	}
	if getErr != nil {
		return ProjectPlanning{}, getErr
	}
	return ProjectPlanning{}, ErrVersionConflict
}
