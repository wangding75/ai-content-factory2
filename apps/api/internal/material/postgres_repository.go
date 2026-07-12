package material

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db queryer }

type queryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: pool}
}
func NewPostgresRepositoryTx(tx pgx.Tx) *PostgresRepository { return &PostgresRepository{db: tx} }

const materialColumns = "id, type, name, summary, content_json, tags_json, created_by, version, created_at, updated_at"
const usageColumns = "id, project_id, material_id, usage_type, role_name, notes, start_chapter, end_chapter, status, created_by, version, created_at, updated_at"

func scanMaterial(row pgx.Row) (Material, error) {
	var value Material
	var tags json.RawMessage
	if err := row.Scan(&value.ID, &value.Type, &value.Name, &value.Summary, &value.ContentJSON, &tags, &value.CreatedBy, &value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Material{}, err
	}
	if !json.Valid(value.ContentJSON) || !json.Valid(tags) || json.Unmarshal(tags, &value.Tags) != nil {
		return Material{}, ErrInvalidJSON
	}
	return value, nil
}

func scanUsage(row pgx.Row) (ProjectMaterialUsage, error) {
	var value ProjectMaterialUsage
	err := row.Scan(&value.ID, &value.ProjectID, &value.MaterialID, &value.UsageType, &value.RoleName, &value.Notes, &value.StartChapter, &value.EndChapter, &value.Status, &value.CreatedBy, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func validateMaterial(value Material) ([]byte, error) {
	if !json.Valid(value.ContentJSON) {
		return nil, ErrInvalidJSON
	}
	tags, err := json.Marshal(value.Tags)
	if err != nil {
		return nil, fmt.Errorf("marshal material tags: %w", err)
	}
	if value.Type != TypeCharacter && value.Type != TypeWorldview && value.Type != TypeLocation && value.Type != TypeOrganization && value.Type != TypeItem && value.Type != TypeReference {
		return nil, fmt.Errorf("invalid material type: %w", ErrInvalidJSON)
	}
	return tags, nil
}

func (r *PostgresRepository) Create(ctx context.Context, value Material) (Material, error) {
	tags, err := validateMaterial(value)
	if err != nil {
		return Material{}, err
	}
	created, err := scanMaterial(r.db.QueryRow(ctx, "INSERT INTO materials (id, type, name, summary, content_json, tags_json, created_by, version) VALUES ($1,$2,$3,$4,$5,$6,$7,1) RETURNING "+materialColumns, value.ID, value.Type, value.Name, value.Summary, value.ContentJSON, tags, value.CreatedBy))
	if err != nil {
		return Material{}, fmt.Errorf("create material: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (Material, error) {
	value, err := scanMaterial(r.db.QueryRow(ctx, "SELECT "+materialColumns+" FROM materials WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Material{}, ErrNotFound
	}
	if err != nil {
		return Material{}, fmt.Errorf("get material: %w", err)
	}
	return value, nil
}

func materialOrder(sort string) (string, error) {
	switch sort {
	case "", "updated_at_desc":
		return "updated_at DESC, id ASC", nil
	case "updated_at_asc":
		return "updated_at ASC, id ASC", nil
	case "name_asc":
		return "name ASC, id ASC", nil
	case "name_desc":
		return "name DESC, id ASC", nil
	default:
		return "", ErrInvalidSort
	}
}

func (r *PostgresRepository) List(ctx context.Context, options ListOptions) ([]Material, int, error) {
	order, err := materialOrder(options.Sort)
	if err != nil {
		return nil, 0, err
	}
	where, args := []string{}, []any{}
	if query := strings.TrimSpace(options.Query); query != "" {
		args = append(args, "%"+query+"%")
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR summary ILIKE $%d OR tags_json::text ILIKE $%d)", len(args), len(args), len(args)))
	}
	if options.Type != "" {
		args = append(args, options.Type)
		where = append(where, fmt.Sprintf("type = $%d", len(args)))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM materials"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count materials: %w", err)
	}
	args = append(args, options.Limit, options.Offset)
	rows, err := r.db.Query(ctx, "SELECT "+materialColumns+" FROM materials"+clause+fmt.Sprintf(" ORDER BY %s LIMIT $%d OFFSET $%d", order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list materials: %w", err)
	}
	defer rows.Close()
	items := make([]Material, 0)
	for rows.Next() {
		value, scanErr := scanMaterial(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan material: %w", scanErr)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate materials: %w", err)
	}
	return items, total, nil
}

func (r *PostgresRepository) UpdateWithVersion(ctx context.Context, value Material, expectedVersion int) (Material, error) {
	tags, err := validateMaterial(value)
	if err != nil {
		return Material{}, err
	}
	updated, err := scanMaterial(r.db.QueryRow(ctx, "UPDATE materials SET name=$2, summary=$3, content_json=$4, tags_json=$5, version=version+1, updated_at=NOW() WHERE id=$1 AND version=$6 RETURNING "+materialColumns, value.ID, value.Name, value.Summary, value.ContentJSON, tags, expectedVersion))
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return Material{}, fmt.Errorf("update material: %w", err)
		}
		return updated, nil
	}
	_, getErr := r.GetByID(ctx, value.ID)
	if errors.Is(getErr, ErrNotFound) {
		return Material{}, ErrNotFound
	}
	if getErr != nil {
		return Material{}, getErr
	}
	return Material{}, ErrVersionConflict
}

func (r *PostgresRepository) CreateUsage(ctx context.Context, value ProjectMaterialUsage) (ProjectMaterialUsage, error) {
	created, err := scanUsage(r.db.QueryRow(ctx, "INSERT INTO project_material_usages (id, project_id, material_id, usage_type, role_name, notes, start_chapter, end_chapter, status, created_by, version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1) RETURNING "+usageColumns, value.ID, value.ProjectID, value.MaterialID, value.UsageType, value.RoleName, value.Notes, value.StartChapter, value.EndChapter, StatusActive, value.CreatedBy))
	if err != nil {
		if isUniqueViolation(err) {
			return ProjectMaterialUsage{}, ErrAlreadyBound
		}
		return ProjectMaterialUsage{}, fmt.Errorf("create project material usage: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) GetByProjectAndMaterial(ctx context.Context, projectID, materialID uuid.UUID) (ProjectMaterialUsage, error) {
	value, err := scanUsage(r.db.QueryRow(ctx, "SELECT "+usageColumns+" FROM project_material_usages WHERE project_id=$1 AND material_id=$2", projectID, materialID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectMaterialUsage{}, ErrUsageNotFound
	}
	if err != nil {
		return ProjectMaterialUsage{}, fmt.Errorf("get project material usage: %w", err)
	}
	return value, nil
}

func (r *PostgresRepository) listUsages(ctx context.Context, query string, id uuid.UUID) ([]ProjectMaterialUsage, error) {
	rows, err := r.db.Query(ctx, "SELECT "+usageColumns+" FROM project_material_usages WHERE "+query+"=$1 ORDER BY created_at ASC, id ASC", id)
	if err != nil {
		return nil, fmt.Errorf("list project material usages: %w", err)
	}
	defer rows.Close()
	items := make([]ProjectMaterialUsage, 0)
	for rows.Next() {
		value, scanErr := scanUsage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
func (r *PostgresRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]ProjectMaterialUsage, error) {
	return r.listUsages(ctx, "project_id", projectID)
}
func (r *PostgresRepository) ListByMaterial(ctx context.Context, materialID uuid.UUID) ([]ProjectMaterialUsage, error) {
	return r.listUsages(ctx, "material_id", materialID)
}
func (r *PostgresRepository) CountByMaterial(ctx context.Context, materialID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM project_material_usages WHERE material_id=$1", materialID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count material usages: %w", err)
	}
	return count, nil
}
func (r *PostgresRepository) ExistsByProjectAndMaterial(ctx context.Context, projectID, materialID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM project_material_usages WHERE project_id=$1 AND material_id=$2)", projectID, materialID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check project material usage: %w", err)
	}
	return exists, nil
}

func (r *PostgresRepository) UpdateUsageWithVersion(ctx context.Context, value ProjectMaterialUsage, expectedVersion int) (ProjectMaterialUsage, error) {
	updated, err := scanUsage(r.db.QueryRow(ctx, "UPDATE project_material_usages SET usage_type=$3,role_name=$4,notes=$5,start_chapter=$6,end_chapter=$7,version=version+1,updated_at=NOW() WHERE project_id=$1 AND material_id=$2 AND version=$8 RETURNING "+usageColumns, value.ProjectID, value.MaterialID, value.UsageType, value.RoleName, value.Notes, value.StartChapter, value.EndChapter, expectedVersion))
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return ProjectMaterialUsage{}, fmt.Errorf("update project material usage: %w", err)
		}
		return updated, nil
	}
	_, getErr := r.GetByProjectAndMaterial(ctx, value.ProjectID, value.MaterialID)
	if errors.Is(getErr, ErrUsageNotFound) {
		return ProjectMaterialUsage{}, ErrUsageNotFound
	}
	if getErr != nil {
		return ProjectMaterialUsage{}, getErr
	}
	return ProjectMaterialUsage{}, ErrVersionConflict
}
func (r *PostgresRepository) DeleteUsageWithVersion(ctx context.Context, projectID, materialID uuid.UUID, expectedVersion int) error {
	tag, err := r.db.Exec(ctx, "DELETE FROM project_material_usages WHERE project_id=$1 AND material_id=$2 AND version=$3", projectID, materialID, expectedVersion)
	if err != nil {
		return fmt.Errorf("delete project material usage: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	_, getErr := r.GetByProjectAndMaterial(ctx, projectID, materialID)
	if errors.Is(getErr, ErrUsageNotFound) {
		return ErrUsageNotFound
	}
	if getErr != nil {
		return getErr
	}
	return ErrVersionConflict
}
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
