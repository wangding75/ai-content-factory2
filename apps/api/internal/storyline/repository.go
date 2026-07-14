package storyline

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
	ErrNotFound         = errors.New("storyline not found")
	ErrInvalidReference = errors.New("invalid storyline reference")
	ErrProjectMismatch  = errors.New("storyline project mismatch")
	ErrVersionConflict  = errors.New("storyline version conflict")
)

type PlotLine struct {
	ID, ProjectID                 uuid.UUID
	ParentID                      *uuid.UUID
	Type, Relation, Name, Summary string
	StartChapter, EndChapter      *int
	Status                        string
	SortOrder, Version            int
	CreatedBy                     string
	CreatedAt, UpdatedAt          time.Time
}

type Repository interface {
	ListByProject(context.Context, uuid.UUID) ([]PlotLine, error)
	GetByID(context.Context, uuid.UUID) (PlotLine, error)
	Create(context.Context, PlotLine) (PlotLine, error)
	UpdateWithVersion(context.Context, PlotLine, int) (PlotLine, error)
	ParentInProject(context.Context, uuid.UUID, uuid.UUID) (bool, error)
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

const columns = "id,project_id,parent_id,type,relation,name,summary,start_chapter,end_chapter,status,sort_order,created_by,version,created_at,updated_at"

func scan(row pgx.Row) (PlotLine, error) {
	var value PlotLine
	err := row.Scan(&value.ID, &value.ProjectID, &value.ParentID, &value.Type, &value.Relation, &value.Name, &value.Summary, &value.StartChapter, &value.EndChapter, &value.Status, &value.SortOrder, &value.CreatedBy, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (PlotLine, error) {
	value, err := scan(r.db.QueryRow(ctx, "SELECT "+columns+" FROM storylines WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return PlotLine{}, ErrNotFound
	}
	if err != nil {
		return PlotLine{}, fmt.Errorf("get storyline: %w", err)
	}
	return value, nil
}

func (r *PostgresRepository) ParentInProject(ctx context.Context, id, projectID uuid.UUID) (bool, error) {
	parent, err := r.GetByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, ErrInvalidReference
	}
	if err != nil {
		return false, err
	}
	if parent.ProjectID != projectID {
		return false, ErrProjectMismatch
	}
	return true, nil
}

func (r *PostgresRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]PlotLine, error) {
	rows, err := r.db.Query(ctx, "SELECT "+columns+" FROM storylines WHERE project_id=$1 ORDER BY parent_id NULLS FIRST,sort_order ASC,id ASC", projectID)
	if err != nil {
		return nil, fmt.Errorf("list storylines: %w", err)
	}
	defer rows.Close()

	values := make([]PlotLine, 0)
	for rows.Next() {
		value, scanErr := scan(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan storyline: %w", scanErr)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate storylines: %w", err)
	}
	return values, nil
}

func (r *PostgresRepository) Create(ctx context.Context, value PlotLine) (PlotLine, error) {
	if value.ParentID != nil {
		if _, err := r.ParentInProject(ctx, *value.ParentID, value.ProjectID); err != nil {
			return PlotLine{}, err
		}
	}
	created, err := scan(r.db.QueryRow(ctx, "INSERT INTO storylines (id,project_id,parent_id,type,relation,name,summary,start_chapter,end_chapter,status,sort_order,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING "+columns, value.ID, value.ProjectID, value.ParentID, value.Type, value.Relation, value.Name, value.Summary, value.StartChapter, value.EndChapter, value.Status, value.SortOrder, value.CreatedBy))
	if err != nil {
		return PlotLine{}, fmt.Errorf("create storyline: %w", err)
	}
	return created, nil
}

func sameIntPointer(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func same(a, b PlotLine) bool {
	return a.Name == b.Name && a.Summary == b.Summary && a.Status == b.Status && a.SortOrder == b.SortOrder && sameIntPointer(a.StartChapter, b.StartChapter) && sameIntPointer(a.EndChapter, b.EndChapter)
}

func (r *PostgresRepository) UpdateWithVersion(ctx context.Context, value PlotLine, expectedVersion int) (PlotLine, error) {
	current, err := r.GetByID(ctx, value.ID)
	if err != nil {
		return PlotLine{}, err
	}
	if current.Version != expectedVersion {
		return PlotLine{}, ErrVersionConflict
	}
	if same(current, value) {
		return current, nil
	}
	updated, err := scan(r.db.QueryRow(ctx, "UPDATE storylines SET name=$2,summary=$3,start_chapter=$4,end_chapter=$5,status=$6,sort_order=$7,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$8 RETURNING "+columns, value.ID, value.Name, value.Summary, value.StartChapter, value.EndChapter, value.Status, value.SortOrder, expectedVersion))
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return PlotLine{}, fmt.Errorf("update storyline: %w", err)
		}
		return updated, nil
	}
	if _, getErr := r.GetByID(ctx, value.ID); errors.Is(getErr, ErrNotFound) {
		return PlotLine{}, ErrNotFound
	} else if getErr != nil {
		return PlotLine{}, getErr
	}
	return PlotLine{}, ErrVersionConflict
}
