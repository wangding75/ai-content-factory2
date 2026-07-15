package chapterplan

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

var (
	ErrNotFound          = errors.New("chapter plan not found")
	ErrVersionConflict   = errors.New("chapter plan version conflict")
	ErrChapterNoConflict = errors.New("chapter plan chapter number conflict")
	ErrInvalidReference  = errors.New("chapter plan invalid reference")
	ErrProjectMismatch   = errors.New("chapter plan project mismatch")
)

type StorylineRef struct {
	ID       uuid.UUID
	Relation string
}
type Plan struct {
	ID, ProjectID, RunID                      uuid.UUID
	ChapterNo                                 int
	Title, Summary, Status, Source, CreatedBy string
	Goal, Notes                               *string
	ConfirmedAt                               *time.Time
	Version                                   int
	CreatedAt, UpdatedAt                      time.Time
	Storylines                                []StorylineRef
	Materials, Foreshadowings                 []uuid.UUID
}
type Run struct {
	ID, ProjectID        uuid.UUID
	CreatedAt, UpdatedAt time.Time
}
type Selection struct {
	ID              uuid.UUID
	ExpectedVersion int
}
type Repository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *Repository { return &Repository{db} }

const cols = "id,project_id,COALESCE(mock_generation_run_id,'00000000-0000-0000-0000-000000000000'),chapter_no,title,summary,chapter_goal,creation_notes,status,source,created_by,confirmed_at,version,created_at,updated_at"

func scan(r pgx.Row) (Plan, error) {
	var p Plan
	e := r.Scan(&p.ID, &p.ProjectID, &p.RunID, &p.ChapterNo, &p.Title, &p.Summary, &p.Goal, &p.Notes, &p.Status, &p.Source, &p.CreatedBy, &p.ConfirmedAt, &p.Version, &p.CreatedAt, &p.UpdatedAt)
	return p, e
}
func (r *Repository) ListByProject(c context.Context, id uuid.UUID) ([]Plan, error) {
	rows, e := r.db.Query(c, "SELECT "+cols+" FROM chapter_plans WHERE project_id=$1 ORDER BY chapter_no,id", id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var a []Plan
	for rows.Next() {
		p, e := scan(rows)
		if e != nil {
			return nil, e
		}
		if e = r.loadRefs(c, &p); e != nil {
			return nil, e
		}
		a = append(a, p)
	}
	return a, rows.Err()
}
func (r *Repository) GetByID(c context.Context, id uuid.UUID) (Plan, error) {
	p, e := scan(r.db.QueryRow(c, "SELECT "+cols+" FROM chapter_plans WHERE id=$1", id))
	if errors.Is(e, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	if e == nil {
		e = r.loadRefs(c, &p)
	}
	return p, e
}
func (r *Repository) loadRefs(c context.Context, p *Plan) error {
	rows, e := r.db.Query(c, "SELECT storyline_id,relation FROM chapter_plan_storylines WHERE chapter_plan_id=$1 ORDER BY position", p.ID)
	if e != nil {
		return e
	}
	for rows.Next() {
		var x StorylineRef
		if e = rows.Scan(&x.ID, &x.Relation); e != nil {
			return e
		}
		p.Storylines = append(p.Storylines, x)
	}
	rows.Close()
	for _, q := range []struct {
		sql string
		dst *[]uuid.UUID
	}{{"SELECT material_id FROM chapter_plan_materials WHERE chapter_plan_id=$1 ORDER BY position", &p.Materials}, {"SELECT foreshadowing_id FROM chapter_plan_foreshadowings WHERE chapter_plan_id=$1 ORDER BY position", &p.Foreshadowings}} {
		rows, e = r.db.Query(c, q.sql, p.ID)
		if e != nil {
			return e
		}
		for rows.Next() {
			var x uuid.UUID
			if e = rows.Scan(&x); e != nil {
				return e
			}
			*q.dst = append(*q.dst, x)
		}
		rows.Close()
	}
	return nil
}
func (r *Repository) SaveMock(c context.Context, run Run, plans []Plan) error {
	tx, e := r.db.Begin(c)
	if e != nil {
		return e
	}
	defer tx.Rollback(c)
	_, e = tx.Exec(c, "INSERT INTO mock_generation_runs(id,project_id,provider_key,workflow_key,status) VALUES($1,$2,'mock','chapter_plan_mock_generate','succeeded')", run.ID, run.ProjectID)
	if e == nil {
		for i := range plans {
			plans[i].RunID = run.ID
			e = r.insert(c, tx, plans[i])
			if e != nil {
				break
			}
		}
	}
	if e != nil {
		return classify(e)
	}
	return tx.Commit(c)
}
func (r *Repository) insert(c context.Context, tx pgx.Tx, p Plan) error {
	_, e := tx.Exec(c, "INSERT INTO chapter_plans(id,project_id,mock_generation_run_id,chapter_no,title,summary,chapter_goal,creation_notes,status,source,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'pending_confirmation','mock_generated',$9)", p.ID, p.ProjectID, p.RunID, p.ChapterNo, p.Title, p.Summary, p.Goal, p.Notes, p.CreatedBy)
	if e != nil {
		return e
	}
	return r.replace(c, tx, p)
}
func (r *Repository) replace(c context.Context, tx pgx.Tx, p Plan) error {
	for _, q := range []string{"DELETE FROM chapter_plan_storylines WHERE chapter_plan_id=$1", "DELETE FROM chapter_plan_materials WHERE chapter_plan_id=$1", "DELETE FROM chapter_plan_foreshadowings WHERE chapter_plan_id=$1"} {
		if _, e := tx.Exec(c, q, p.ID); e != nil {
			return e
		}
	}
	for i, x := range p.Storylines {
		if _, e := tx.Exec(c, "INSERT INTO chapter_plan_storylines(chapter_plan_id,project_id,storyline_id,relation,position) VALUES($1,$2,$3,$4,$5)", p.ID, p.ProjectID, x.ID, x.Relation, i); e != nil {
			return e
		}
	}
	for i, x := range p.Materials {
		if _, e := tx.Exec(c, "INSERT INTO chapter_plan_materials(chapter_plan_id,project_id,material_id,position) VALUES($1,$2,$3,$4)", p.ID, p.ProjectID, x, i); e != nil {
			return e
		}
	}
	for i, x := range p.Foreshadowings {
		if _, e := tx.Exec(c, "INSERT INTO chapter_plan_foreshadowings(chapter_plan_id,project_id,foreshadowing_id,position) VALUES($1,$2,$3,$4)", p.ID, p.ProjectID, x, i); e != nil {
			return e
		}
	}
	return nil
}
func classify(e error) error {
	var x *pgconn.PgError
	if errors.As(e, &x) {
		if x.Code == "23505" {
			if x.ConstraintName == "chapter_plans_project_chapter_no_unique" {
				return ErrChapterNoConflict
			}
			return ErrInvalidReference
		}
		if x.Code == "23503" {
			return ErrInvalidReference
		}
	}
	return fmt.Errorf("chapter plan repository: %w", e)
}
func (r *Repository) Update(c context.Context, p Plan, expected int) (Plan, error) {
	tx, e := r.db.Begin(c)
	if e != nil {
		return Plan{}, e
	}
	defer tx.Rollback(c)
	q := "UPDATE chapter_plans SET chapter_no=$2,title=$3,summary=$4,chapter_goal=$5,creation_notes=$6,version=version+1,updated_at=NOW() WHERE id=$1 AND status='pending_confirmation' AND version=$7 RETURNING " + cols
	out, e := scan(tx.QueryRow(c, q, p.ID, p.ChapterNo, p.Title, p.Summary, p.Goal, p.Notes, expected))
	if errors.Is(e, pgx.ErrNoRows) {
		return Plan{}, ErrVersionConflict
	}
	if e != nil {
		return Plan{}, classify(e)
	}
	p.ProjectID = out.ProjectID
	if e = r.replace(c, tx, p); e != nil {
		return Plan{}, classify(e)
	}
	if e = tx.Commit(c); e != nil {
		return Plan{}, e
	}
	return r.GetByID(c, p.ID)
}
func (r *Repository) Delete(c context.Context, id uuid.UUID, expected int) error {
	tag, e := r.db.Exec(c, "DELETE FROM chapter_plans WHERE id=$1 AND status='pending_confirmation' AND version=$2", id, expected)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		if _, e = r.GetByID(c, id); errors.Is(e, ErrNotFound) {
			return ErrNotFound
		}
		return ErrVersionConflict
	}
	return nil
}
func (r *Repository) Confirm(c context.Context, s []Selection) ([]Plan, error) {
	tx, e := r.db.Begin(c)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(c)
	out := make([]Plan, 0, len(s))
	for _, x := range s {
		p, e := scan(tx.QueryRow(c, "UPDATE chapter_plans SET status='confirmed',confirmed_at=NOW(),version=version+1,updated_at=NOW() WHERE id=$1 AND status='pending_confirmation' AND version=$2 RETURNING "+cols, x.ID, x.ExpectedVersion))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, ErrVersionConflict
		}
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	if e = tx.Commit(c); e != nil {
		return nil, e
	}
	for i := range out {
		out[i], e = r.GetByID(c, out[i].ID)
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
