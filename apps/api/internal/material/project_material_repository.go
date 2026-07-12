package material

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const projectMaterialColumns = "m.id, m.type, m.name, m.summary, m.content_json, m.tags_json, m.created_by, m.version, m.created_at, m.updated_at, u.id, u.project_id, u.material_id, u.usage_type, u.role_name, u.notes, u.start_chapter, u.end_chapter, u.status, u.created_by, u.version, u.created_at, u.updated_at, GREATEST(m.updated_at, u.updated_at)"

func scanProjectMaterialItem(row interface{ Scan(...any) error }) (ProjectMaterialItem, error) {
	var item ProjectMaterialItem
	var tags json.RawMessage
	err := row.Scan(
		&item.Material.ID, &item.Material.Type, &item.Material.Name, &item.Material.Summary, &item.Material.ContentJSON, &tags, &item.Material.CreatedBy, &item.Material.Version, &item.Material.CreatedAt, &item.Material.UpdatedAt,
		&item.Usage.ID, &item.Usage.ProjectID, &item.Usage.MaterialID, &item.Usage.UsageType, &item.Usage.RoleName, &item.Usage.Notes, &item.Usage.StartChapter, &item.Usage.EndChapter, &item.Usage.Status, &item.Usage.CreatedBy, &item.Usage.Version, &item.Usage.CreatedAt, &item.Usage.UpdatedAt,
		&item.LastUpdatedAt,
	)
	if err != nil {
		return ProjectMaterialItem{}, err
	}
	if !json.Valid(item.Material.ContentJSON) || !json.Valid(tags) || json.Unmarshal(tags, &item.Material.Tags) != nil {
		return ProjectMaterialItem{}, ErrInvalidJSON
	}
	return item, nil
}

func projectMaterialOrder(sort string) (string, error) {
	switch sort {
	case "", "updated_at_desc":
		return "GREATEST(m.updated_at, u.updated_at) DESC, m.id ASC", nil
	case "updated_at_asc":
		return "GREATEST(m.updated_at, u.updated_at) ASC, m.id ASC", nil
	case "name_asc":
		return "m.name ASC, m.id ASC", nil
	case "name_desc":
		return "m.name DESC, m.id ASC", nil
	default:
		return "", ErrInvalidSort
	}
}

func (r *PostgresRepository) ListProjectMaterials(ctx context.Context, projectID uuid.UUID, options ListOptions) ([]ProjectMaterialItem, int, error) {
	order, err := projectMaterialOrder(options.Sort)
	if err != nil {
		return nil, 0, err
	}
	args := []any{projectID}
	where := []string{"u.project_id = $1", "u.status = 'active'"}
	if query := strings.TrimSpace(options.Query); query != "" {
		args = append(args, "%"+query+"%")
		marker := len(args)
		where = append(where, fmt.Sprintf("(m.name ILIKE $%d OR m.summary ILIKE $%d OR m.tags_json::text ILIKE $%d OR u.usage_type ILIKE $%d OR u.role_name ILIKE $%d OR u.notes ILIKE $%d)", marker, marker, marker, marker, marker, marker))
	}
	if options.Type != "" {
		args = append(args, options.Type)
		where = append(where, fmt.Sprintf("m.type = $%d", len(args)))
	}
	clause := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM project_material_usages u JOIN materials m ON m.id = u.material_id"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count project materials: %w", err)
	}
	args = append(args, options.Limit, options.Offset)
	rows, err := r.db.Query(ctx, "SELECT "+projectMaterialColumns+" FROM project_material_usages u JOIN materials m ON m.id = u.material_id"+clause+fmt.Sprintf(" ORDER BY %s LIMIT $%d OFFSET $%d", order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list project materials: %w", err)
	}
	defer rows.Close()
	items := make([]ProjectMaterialItem, 0)
	for rows.Next() {
		item, err := scanProjectMaterialItem(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan project material: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate project materials: %w", err)
	}
	return items, total, nil
}

func (r *PostgresRepository) ProjectMaterialTypeCounts(ctx context.Context, projectID uuid.UUID) (ProjectMaterialTypeCounts, error) {
	var counts ProjectMaterialTypeCounts
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FILTER (WHERE m.type='character'), COUNT(*) FILTER (WHERE m.type='worldview'), COUNT(*) FILTER (WHERE m.type='location'), COUNT(*) FILTER (WHERE m.type='organization'), COUNT(*) FILTER (WHERE m.type='item'), COUNT(*) FILTER (WHERE m.type='reference') FROM project_material_usages u JOIN materials m ON m.id=u.material_id WHERE u.project_id=$1 AND u.status='active'", projectID).Scan(&counts.Character, &counts.Worldview, &counts.Location, &counts.Organization, &counts.Item, &counts.Reference)
	if err != nil {
		return ProjectMaterialTypeCounts{}, fmt.Errorf("count project material types: %w", err)
	}
	return counts, nil
}
