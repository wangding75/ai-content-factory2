package material

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	TypeCharacter    = "character"
	TypeWorldview    = "worldview"
	TypeLocation     = "location"
	TypeOrganization = "organization"
	TypeItem         = "item"
	TypeReference    = "reference"
	StatusActive     = "active"
)

var (
	ErrNotFound        = errors.New("material not found")
	ErrUsageNotFound   = errors.New("project material usage not found")
	ErrVersionConflict = errors.New("material version conflict")
	ErrAlreadyBound    = errors.New("material already bound to project")
	ErrInvalidJSON     = errors.New("material contains invalid json")
	ErrInvalidSort     = errors.New("invalid material sort")
)

type Material struct {
	ID          uuid.UUID       `json:"id"`
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Summary     string          `json:"summary"`
	ContentJSON json.RawMessage `json:"content_json"`
	Tags        []string        `json:"tags_json"`
	CreatedBy   string          `json:"-"`
	Version     int             `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type ProjectMaterialUsage struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	MaterialID   uuid.UUID `json:"material_id"`
	UsageType    string    `json:"usage_type"`
	RoleName     string    `json:"role_name"`
	Notes        string    `json:"notes"`
	StartChapter *int      `json:"start_chapter"`
	EndChapter   *int      `json:"end_chapter"`
	Status       string    `json:"status"`
	CreatedBy    string    `json:"-"`
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ListOptions struct {
	Query  string
	Type   string
	Sort   string
	Limit  int
	Offset int
}

type Repository interface {
	Create(ctx context.Context, value Material) (Material, error)
	GetByID(ctx context.Context, id uuid.UUID) (Material, error)
	List(ctx context.Context, options ListOptions) ([]Material, int, error)
	UpdateWithVersion(ctx context.Context, value Material, expectedVersion int) (Material, error)
	CreateUsage(ctx context.Context, value ProjectMaterialUsage) (ProjectMaterialUsage, error)
	GetByProjectAndMaterial(ctx context.Context, projectID, materialID uuid.UUID) (ProjectMaterialUsage, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]ProjectMaterialUsage, error)
	ListByMaterial(ctx context.Context, materialID uuid.UUID) ([]ProjectMaterialUsage, error)
	CountByMaterial(ctx context.Context, materialID uuid.UUID) (int, error)
	UpdateUsageWithVersion(ctx context.Context, value ProjectMaterialUsage, expectedVersion int) (ProjectMaterialUsage, error)
	DeleteUsageWithVersion(ctx context.Context, projectID, materialID uuid.UUID, expectedVersion int) error
	ExistsByProjectAndMaterial(ctx context.Context, projectID, materialID uuid.UUID) (bool, error)
}
