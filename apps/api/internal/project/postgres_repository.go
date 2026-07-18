package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const projectColumns = "id, name, type, status, description, current_stage, created_at, updated_at"

func scanProject(row pgx.Row) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.Name, &p.Type, &p.Status, &p.Description, &p.CurrentStage, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (r *PostgresRepository) Create(ctx context.Context, p Project, actorID string) (Project, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Project{}, fmt.Errorf("begin project transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	created, err := scanProject(tx.QueryRow(ctx, "INSERT INTO projects (id, name, type, status, description, current_stage, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING "+projectColumns, p.ID, p.Name, p.Type, p.Status, p.Description, p.CurrentStage, actorID))
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}
	payload, err := json.Marshal(map[string]string{"project_id": created.ID.String()})
	if err != nil {
		return Project{}, fmt.Errorf("marshal audit payload: %w", err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO audit_logs (id, actor_id, action, subject_type, subject_id, payload) VALUES ($1, $2, $3, $4, $5, $6)", uuid.New(), actorID, "project.created", "project", created.ID.String(), payload)
	if err != nil {
		return Project{}, fmt.Errorf("insert audit log: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, fmt.Errorf("commit project transaction: %w", err)
	}
	return created, nil
}
func (r *PostgresRepository) List(ctx context.Context, options ListOptions) ([]Project, int, error) {
	where, args := "", []any{}
	if options.Status != "" {
		args = append(args, options.Status)
		where = " WHERE status = $1"
	}
	if options.Query != "" {
		args = append(args, "%"+options.Query+"%")
		marker := len(args)
		if where == "" {
			where = fmt.Sprintf(" WHERE name ILIKE $%d", marker)
		} else {
			where += fmt.Sprintf(" AND name ILIKE $%d", marker)
		}
	}
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM projects"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}
	args = append(args, options.Limit, options.Offset)
	rows, err := r.pool.Query(ctx, "SELECT "+projectColumns+" FROM projects"+where+fmt.Sprintf(" ORDER BY updated_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	items := make([]Project, 0)
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate projects: %w", err)
	}
	return items, total, nil
}
func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Project, error) {
	p, err := scanProject(r.pool.QueryRow(ctx, "SELECT "+projectColumns+" FROM projects WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}
func (r *PostgresRepository) Progress(ctx context.Context, id uuid.UUID) (Progress, error) {
	var progress Progress
	err := r.pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM project_material_usages WHERE project_id = $1 AND status = 'active'),
		(SELECT COUNT(*) FROM storylines WHERE project_id = $1),
		(SELECT COUNT(*) FROM chapter_plans WHERE project_id = $1 AND status = 'confirmed'),
		(SELECT COUNT(*) FROM content_items WHERE project_id = $1)`, id).Scan(
		&progress.MaterialCount, &progress.StorylineCount, &progress.ConfirmedChapterCount, &progress.WorkCount,
	)
	if err != nil {
		return Progress{}, fmt.Errorf("get project progress: %w", err)
	}
	return progress, nil
}
func (r *PostgresRepository) Update(ctx context.Context, id uuid.UUID, name, description *string) (Project, error) {
	p, err := scanProject(r.pool.QueryRow(ctx, "UPDATE projects SET name = COALESCE($2, name), description = COALESCE($3, description), updated_at = NOW() WHERE id = $1 RETURNING "+projectColumns, id, name, description))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}
	return p, nil
}
