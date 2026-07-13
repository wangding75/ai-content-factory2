package material

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
	"strings"
	"time"
)

var ErrValidation = errors.New("material validation failed")
var ErrIdempotencyReused = errors.New("idempotency key reused")

type CreateRequest struct {
	Type        string          `json:"type"`
	Name        *string         `json:"name"`
	Summary     *string         `json:"summary"`
	ContentJSON json.RawMessage `json:"content_json"`
	Tags        *[]string       `json:"tags_json"`
}
type UpdateRequest struct {
	ExpectedVersion *int            `json:"expected_version"`
	Name            *string         `json:"name"`
	Summary         *string         `json:"summary"`
	ContentJSON     json.RawMessage `json:"content_json"`
	Tags            *[]string       `json:"tags_json"`
}
type Reference struct {
	UsageID      uuid.UUID `json:"usage_id"`
	ProjectID    uuid.UUID `json:"project_id"`
	ProjectName  string    `json:"project_name"`
	ProjectType  string    `json:"project_type"`
	UsageType    string    `json:"-"`
	RoleName     string    `json:"-"`
	Notes        string    `json:"-"`
	StartChapter *int      `json:"-"`
	EndChapter   *int      `json:"-"`
	Status       string    `json:"-"`
	Version      int       `json:"-"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}
type Detail struct {
	Material       Material    `json:"material"`
	References     []Reference `json:"references"`
	ReferenceCount int         `json:"reference_count"`
}
type Service struct {
	pool *pgxpool.Pool
	repo *PostgresRepository
}

func NewService(p *pgxpool.Pool) *Service { return &Service{p, NewPostgresRepository(p)} }
func (s *Service) ListMaterials(c context.Context, o ListOptions) ([]Material, int, error) {
	return s.repo.List(c, o)
}
func (s *Service) GetMaterial(c context.Context, id uuid.UUID) (Detail, error) {
	m, e := s.repo.GetByID(c, id)
	if e != nil {
		return Detail{}, e
	}
	rs, e := s.refs(c, id)
	return Detail{m, rs, len(rs)}, e
}
func (s *Service) refs(c context.Context, id uuid.UUID) ([]Reference, error) {
	rows, e := s.pool.Query(c, "SELECT u.id,u.project_id,p.name,p.type,u.usage_type,u.role_name,u.notes,u.start_chapter,u.end_chapter,u.status,u.version,u.created_at,u.updated_at FROM project_material_usages u JOIN projects p ON p.id=u.project_id WHERE u.material_id=$1 AND u.status='active' ORDER BY u.id", id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Reference{}
	for rows.Next() {
		var x Reference
		e = rows.Scan(&x.UsageID, &x.ProjectID, &x.ProjectName, &x.ProjectType, &x.UsageType, &x.RoleName, &x.Notes, &x.StartChapter, &x.EndChapter, &x.Status, &x.Version, &x.CreatedAt, &x.UpdatedAt)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Service) CreateMaterial(c context.Context, r CreateRequest, k, actor string) (Material, error) {
	v, e := createValue(r, actor)
	if e != nil {
		return Material{}, e
	}
	if len(strings.TrimSpace(k)) == 0 || len(k) > 128 {
		return Material{}, ErrValidation
	}
	hash, e := hash(r)
	if e != nil {
		return Material{}, e
	}
	ir := idempotency.NewPostgresRepository(s.pool)
	if x, e := ir.Get(c, "material:create", k); e == nil {
		if x.RequestHash != hash {
			return Material{}, ErrIdempotencyReused
		}
		var m Material
		e = json.Unmarshal(x.ResponseBody, &m)
		return m, e
	} else if !errors.Is(e, idempotency.ErrNotFound) {
		return Material{}, e
	}
	var result Material
	e = s.tx(c, func(repo *PostgresRepository, idem *idempotency.PostgresRepository, a *audit.Repository) error {
		if x, z := idem.Get(c, "material:create", k); z == nil {
			if x.RequestHash != hash {
				return ErrIdempotencyReused
			}
			return json.Unmarshal(x.ResponseBody, &result)
		}
		m, z := repo.Create(c, v)
		if z != nil {
			return z
		}
		p, _ := json.Marshal(map[string]any{"before": nil, "after": m})
		if z = a.Insert(c, audit.Entry{ID: uuid.New(), ActorID: actor, Action: "material.created", SubjectType: "material", SubjectID: m.ID.String(), Payload: p}); z != nil {
			return z
		}
		b, _ := json.Marshal(m)
		if _, z = idem.Create(c, idempotency.Record{ID: uuid.New(), Scope: "material:create", Key: k, RequestHash: hash, ResponseStatus: 201, ResponseBody: b}); z != nil {
			return z
		}
		result = m
		return nil
	})
	if errors.Is(e, idempotency.ErrConflict) {
		x, z := ir.Get(c, "material:create", k)
		if z == nil && x.RequestHash == hash {
			z = json.Unmarshal(x.ResponseBody, &result)
			return result, z
		}
		return Material{}, ErrIdempotencyReused
	}
	return result, e
}
func (s *Service) UpdateMaterial(c context.Context, id uuid.UUID, r UpdateRequest, actor string) (Material, error) {
	if r.ExpectedVersion == nil || *r.ExpectedVersion < 1 || r.Name == nil && r.Summary == nil && r.ContentJSON == nil && r.Tags == nil {
		return Material{}, ErrValidation
	}
	var result Material
	e := s.tx(c, func(repo *PostgresRepository, _ *idempotency.PostgresRepository, a *audit.Repository) error {
		cur, z := repo.GetByID(c, id)
		if z != nil {
			return z
		}
		if cur.Version != *r.ExpectedVersion {
			return ErrVersionConflict
		}
		next := cur
		if r.Name != nil {
			next.Name = *r.Name
		}
		if r.Summary != nil {
			next.Summary = *r.Summary
		}
		if r.ContentJSON != nil {
			next.ContentJSON = r.ContentJSON
		}
		if r.Tags != nil {
			next.Tags = *r.Tags
		}
		if z = validate(next); z != nil {
			return z
		}
		if same(cur, next) {
			result = cur
			return nil
		}
		before := cur
		up, z := repo.UpdateWithVersion(c, next, *r.ExpectedVersion)
		if z != nil {
			return z
		}
		p, _ := json.Marshal(map[string]any{"before": before, "after": up})
		if z = a.Insert(c, audit.Entry{ID: uuid.New(), ActorID: actor, Action: "material.updated", SubjectType: "material", SubjectID: id.String(), Payload: p}); z != nil {
			return z
		}
		result = up
		return nil
	})
	return result, e
}
func (s *Service) tx(c context.Context, f func(*PostgresRepository, *idempotency.PostgresRepository, *audit.Repository) error) error {
	t, e := s.pool.Begin(c)
	if e != nil {
		return e
	}
	defer t.Rollback(c)
	if e = f(NewPostgresRepositoryTx(t), idempotency.NewPostgresRepositoryTx(t), audit.NewRepository(t)); e != nil {
		return e
	}
	return t.Commit(c)
}
func createValue(r CreateRequest, a string) (Material, error) {
	if r.Name == nil || r.Summary == nil || r.Tags == nil || r.ContentJSON == nil {
		return Material{}, ErrValidation
	}
	v := Material{ID: uuid.New(), Type: r.Type, Name: *r.Name, Summary: *r.Summary, ContentJSON: r.ContentJSON, Tags: *r.Tags, CreatedBy: a}
	return v, validate(v)
}
func validate(v Material) error {
	if !valid(v.Type) || strings.TrimSpace(v.Name) == "" || len(v.Name) > 120 || len(v.Summary) > 5000 || len(v.Tags) > 20 || !json.Valid(v.ContentJSON) {
		return ErrValidation
	}
	var o map[string]json.RawMessage
	if json.Unmarshal(v.ContentJSON, &o) != nil {
		return ErrValidation
	}
	seen := map[string]bool{}
	for _, x := range v.Tags {
		if len(x) < 1 || len(x) > 50 || seen[x] {
			return ErrValidation
		}
		seen[x] = true
	}
	return nil
}
func valid(x string) bool {
	return x == TypeCharacter || x == TypeWorldview || x == TypeLocation || x == TypeOrganization || x == TypeItem || x == TypeReference
}
func same(a, b Material) bool {
	return a.Name == b.Name && a.Summary == b.Summary && bytes.Equal(a.ContentJSON, b.ContentJSON) && strings.Join(a.Tags, "\000") == strings.Join(b.Tags, "\000")
}
func hash(r CreateRequest) (string, error) {
	b, e := json.Marshal(r)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), e
}
