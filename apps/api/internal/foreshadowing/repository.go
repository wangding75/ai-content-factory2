package foreshadowing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("foreshadowing not found")
	ErrInvalidReference  = errors.New("invalid foreshadowing reference")
	ErrProjectMismatch   = errors.New("foreshadowing project mismatch")
	ErrVersionConflict   = errors.New("foreshadowing version conflict")
	ErrInvalidTransition = errors.New("invalid foreshadowing status transition")
)

type Foreshadowing struct {
	ID, ProjectID                             uuid.UUID
	Title, Description, Priority, Status      string
	PlantedPlotLineID, PayoffPlotLineID       *uuid.UUID
	PlannedPlantChapter, PlannedPayoffChapter *int
	CreatedBy                                 string
	Version                                   int
	CreatedAt, UpdatedAt                      time.Time
}

type Repository interface {
	ListByProject(context.Context, uuid.UUID) ([]Foreshadowing, error)
	GetByID(context.Context, uuid.UUID) (Foreshadowing, error)
	Create(context.Context, Foreshadowing) (Foreshadowing, error)
	UpdateWithVersion(context.Context, Foreshadowing, int) (Foreshadowing, error)
	ValidateReferences(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) error
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresRepository struct{ db queryer }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: pool}
}
func NewPostgresRepositoryTx(tx pgx.Tx) *PostgresRepository { return &PostgresRepository{db: tx} }

const columns = "id,project_id,title,description,priority,status,planted_plot_line_id,payoff_plot_line_id,planned_plant_chapter,planned_payoff_chapter,created_by,version,created_at,updated_at"

func scan(row pgx.Row) (Foreshadowing, error) {
	var value Foreshadowing
	err := row.Scan(&value.ID, &value.ProjectID, &value.Title, &value.Description, &value.Priority, &value.Status, &value.PlantedPlotLineID, &value.PayoffPlotLineID, &value.PlannedPlantChapter, &value.PlannedPayoffChapter, &value.CreatedBy, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (Foreshadowing, error) {
	value, err := scan(r.db.QueryRow(ctx, "SELECT "+columns+" FROM foreshadowings WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Foreshadowing{}, ErrNotFound
	}
	if err != nil {
		return Foreshadowing{}, fmt.Errorf("get foreshadowing: %w", err)
	}
	return value, nil
}

func (r *PostgresRepository) validateReference(ctx context.Context, projectID uuid.UUID, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	var referenceProjectID uuid.UUID
	err := r.db.QueryRow(ctx, "SELECT project_id FROM storylines WHERE id=$1", *id).Scan(&referenceProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidReference
	}
	if err != nil {
		return fmt.Errorf("get referenced storyline: %w", err)
	}
	if referenceProjectID != projectID {
		return ErrProjectMismatch
	}
	return nil
}

func (r *PostgresRepository) ValidateReferences(ctx context.Context, projectID uuid.UUID, plantedID, payoffID *uuid.UUID) error {
	if err := r.validateReference(ctx, projectID, plantedID); err != nil {
		return err
	}
	return r.validateReference(ctx, projectID, payoffID)
}

func (r *PostgresRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]Foreshadowing, error) {
	rows, err := r.db.Query(ctx, "SELECT "+columns+" FROM foreshadowings WHERE project_id=$1 ORDER BY CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 END,planned_plant_chapter NULLS LAST,id ASC", projectID)
	if err != nil {
		return nil, fmt.Errorf("list foreshadowings: %w", err)
	}
	defer rows.Close()

	values := make([]Foreshadowing, 0)
	for rows.Next() {
		value, scanErr := scan(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan foreshadowing: %w", scanErr)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate foreshadowings: %w", err)
	}
	return values, nil
}

func (r *PostgresRepository) Create(ctx context.Context, value Foreshadowing) (Foreshadowing, error) {
	if err := r.ValidateReferences(ctx, value.ProjectID, value.PlantedPlotLineID, value.PayoffPlotLineID); err != nil {
		return Foreshadowing{}, err
	}
	created, err := scan(r.db.QueryRow(ctx, "INSERT INTO foreshadowings(id,project_id,title,description,priority,status,planted_plot_line_id,payoff_plot_line_id,planned_plant_chapter,planned_payoff_chapter,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING "+columns, value.ID, value.ProjectID, value.Title, value.Description, value.Priority, value.Status, value.PlantedPlotLineID, value.PayoffPlotLineID, value.PlannedPlantChapter, value.PlannedPayoffChapter, value.CreatedBy))
	if err != nil {
		return Foreshadowing{}, fmt.Errorf("create foreshadowing: %w", err)
	}
	return created, nil
}

func valid(from, to string) bool {
	return from == to || (from == "planned" && to == "planted") || (from == "planted" && to == "paid_off")
}

func sameUUIDPointer(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func sameIntPointer(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func same(a, b Foreshadowing) bool {
	return a.Title == b.Title && a.Description == b.Description && a.Priority == b.Priority && a.Status == b.Status && sameUUIDPointer(a.PlantedPlotLineID, b.PlantedPlotLineID) && sameUUIDPointer(a.PayoffPlotLineID, b.PayoffPlotLineID) && sameIntPointer(a.PlannedPlantChapter, b.PlannedPlantChapter) && sameIntPointer(a.PlannedPayoffChapter, b.PlannedPayoffChapter)
}

func (r *PostgresRepository) UpdateWithVersion(ctx context.Context, value Foreshadowing, expectedVersion int) (Foreshadowing, error) {
	current, err := r.GetByID(ctx, value.ID)
	if err != nil {
		return Foreshadowing{}, err
	}
	if current.Version != expectedVersion {
		return Foreshadowing{}, ErrVersionConflict
	}
	if !valid(current.Status, value.Status) {
		return Foreshadowing{}, ErrInvalidTransition
	}
	if err := r.ValidateReferences(ctx, current.ProjectID, value.PlantedPlotLineID, value.PayoffPlotLineID); err != nil {
		return Foreshadowing{}, err
	}
	if same(current, value) {
		return current, nil
	}
	updated, err := scan(r.db.QueryRow(ctx, "UPDATE foreshadowings SET title=$2,description=$3,priority=$4,status=$5,planted_plot_line_id=$6,payoff_plot_line_id=$7,planned_plant_chapter=$8,planned_payoff_chapter=$9,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$10 RETURNING "+columns, value.ID, value.Title, value.Description, value.Priority, value.Status, value.PlantedPlotLineID, value.PayoffPlotLineID, value.PlannedPlantChapter, value.PlannedPayoffChapter, expectedVersion))
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return Foreshadowing{}, fmt.Errorf("update foreshadowing: %w", err)
		}
		return updated, nil
	}
	if _, getErr := r.GetByID(ctx, value.ID); errors.Is(getErr, ErrNotFound) {
		return Foreshadowing{}, ErrNotFound
	} else if getErr != nil {
		return Foreshadowing{}, getErr
	}
	return Foreshadowing{}, ErrVersionConflict
}
