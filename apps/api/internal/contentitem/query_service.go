package contentitem

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrProjectNotFound = errors.New("project not found")

// QueryService is the Iteration 07 read boundary. It deliberately composes the
// existing persisted records rather than introducing a Work entity.
type QueryService struct{ repo *PostgresRepository }

func NewQueryService(repo *PostgresRepository) *QueryService { return &QueryService{repo: repo} }

type Iteration07Application struct {
	Rewrite *MockRewriteService
	Query   *QueryService
}

func NewIteration07Application(rewrite *MockRewriteService, query *QueryService) *Iteration07Application {
	return &Iteration07Application{Rewrite: rewrite, Query: query}
}

type VersionRead struct {
	Version   ContentVersion
	Item      ContentItem
	Lineage   ContentVersionLineage
	IsCurrent bool
}
type VersionListRead struct {
	Items                []VersionRead
	Total, Limit, Offset int
}
type WorkRead struct {
	WorkID, ProjectID           uuid.UUID
	ProjectName, ProjectStatus  string
	ChapterID                   uuid.UUID
	ChapterNo                   int
	ChapterTitle, ChapterStatus string
	Item                        ContentItem
	Current                     VersionRead
	VersionCount                int
	LatestReview                *ReviewReport
	LatestRun                   *WorkflowRun
	RewriteRun                  *WorkflowRun
}

func (s *QueryService) versionRead(ctx context.Context, v ContentVersion) (VersionRead, error) {
	d, err := s.repo.GetByID(ctx, v.ContentItemID)
	if err != nil {
		return VersionRead{}, err
	}
	out := VersionRead{Version: v, Item: d.Item, IsCurrent: v.ID == d.Item.CurrentVersionID}
	if v.Source != ContentVersionSourceMockRewrite {
		return out, nil
	}
	var source ContentVersion
	err = s.repo.db.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE content_item_id=$1 AND version_no=1", v.ContentItemID).Scan(&source.ID, &source.ContentItemID, &source.VersionNo, &source.Title, &source.Content, &source.Summary, &source.WordCount, &source.Source, &source.Status, &source.GenerationParameters, &source.Version, &source.FrozenAt, &source.CreatedAt, &source.UpdatedAt)
	if err != nil {
		return VersionRead{}, rewriteDatabaseError(err)
	}
	out.Lineage.SourceContentVersion = &source
	var report ReviewReport
	err = s.repo.db.QueryRow(ctx, "SELECT "+reportColumns+" FROM review_reports WHERE project_id=$1 AND content_item_id=$2 AND content_version_id=$3 ORDER BY created_at DESC,id DESC LIMIT 1", d.Item.ProjectID, v.ContentItemID, source.ID).Scan(&report.ID, &report.ProjectID, &report.ContentItemID, &report.ContentVersionID, &report.WorkflowRunID, &report.ProviderKey, &report.Status, &report.Conclusion, &report.Score, &report.Summary, &report.CreatedAt, &report.CompletedAt)
	if err == nil {
		out.Lineage.SourceReviewReport = &report
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return VersionRead{}, rewriteDatabaseError(err)
	}
	var run WorkflowRun
	err = s.repo.db.QueryRow(ctx, "SELECT "+rewriteRunColumns+" FROM workflow_runs WHERE target_content_version_id=$1 AND workflow_key=$2 ORDER BY started_at DESC,id DESC LIMIT 1", v.ID, WorkflowKeyMockRewrite).Scan(&run.ID, &run.ProjectID, &run.ContentItemID, &run.ContentVersionID, &run.TargetContentVersionID, &run.SourceReviewReportID, &run.ProviderKey, &run.WorkflowKey, &run.SubjectType, &run.SubjectID, &run.Status, &run.IdempotencyKey, &run.RequestFingerprint, &run.InputJSON, &run.OutputJSON, &run.ErrorCode, &run.ErrorSummary, &run.StartedAt, &run.FinishedAt, &run.CreatedAt, &run.UpdatedAt)
	if err == nil {
		out.Lineage.RewriteWorkflowRun = &run
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return VersionRead{}, rewriteDatabaseError(err)
	}
	return out, nil
}
func (s *QueryService) GetVersion(ctx context.Context, id uuid.UUID) (VersionRead, error) {
	v, e := s.repo.GetContentVersion(ctx, id)
	if e != nil {
		return VersionRead{}, e
	}
	return s.versionRead(ctx, v)
}
func (s *QueryService) GetItem(ctx context.Context, id uuid.UUID) (ContentItem, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ContentItem{}, err
	}
	return d.Item, nil
}
func (s *QueryService) ListVersions(ctx context.Context, item uuid.UUID, limit, offset int) (VersionListRead, error) {
	if _, e := s.repo.GetByID(ctx, item); e != nil {
		return VersionListRead{}, e
	}
	p, e := s.repo.ListContentVersions(ctx, item, ContentVersionPageOptions{Limit: limit, Offset: offset})
	if e != nil {
		return VersionListRead{}, e
	}
	out := VersionListRead{Total: p.Total, Limit: p.Limit, Offset: p.Offset}
	for _, v := range p.Items {
		x, e := s.versionRead(ctx, v)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	return out, nil
}
func (s *QueryService) GetRun(ctx context.Context, id uuid.UUID) (WorkflowRun, error) {
	return s.repo.GetWorkflowRun(ctx, id)
}
func (s *QueryService) GetReview(ctx context.Context, id uuid.UUID) (ReviewReport, error) {
	v, err := s.repo.GetReview(ctx, id)
	if err != nil {
		return ReviewReport{}, err
	}
	return v.Review, nil
}

func (s *QueryService) ListWorks(ctx context.Context, project uuid.UUID, limit, offset int) ([]WorkRead, int, error) {
	var exists bool
	if e := s.repo.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1)", project).Scan(&exists); e != nil {
		return nil, 0, rewriteDatabaseError(e)
	}
	if !exists {
		return nil, 0, ErrProjectNotFound
	}
	var total int
	if e := s.repo.db.QueryRow(ctx, "SELECT count(*) FROM content_items ci JOIN chapter_plans cp ON cp.id=ci.chapter_plan_id WHERE ci.project_id=$1", project).Scan(&total); e != nil {
		return nil, 0, rewriteDatabaseError(e)
	}
	rows, e := s.repo.db.Query(ctx, "SELECT ci.id FROM content_items ci JOIN chapter_plans cp ON cp.id=ci.chapter_plan_id WHERE ci.project_id=$1 ORDER BY cp.chapter_no ASC,ci.id ASC LIMIT $2 OFFSET $3", project, limit, offset)
	if e != nil {
		return nil, 0, rewriteDatabaseError(e)
	}
	defer rows.Close()
	var out []WorkRead
	for rows.Next() {
		var id uuid.UUID
		if e = rows.Scan(&id); e != nil {
			return nil, 0, rewriteDatabaseError(e)
		}
		w, e := s.GetWork(ctx, id)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, w)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, rewriteDatabaseError(e)
	}
	return out, total, nil
}
func (s *QueryService) GetWork(ctx context.Context, id uuid.UUID) (WorkRead, error) {
	d, e := s.repo.GetByID(ctx, id)
	if e != nil {
		return WorkRead{}, e
	}
	var w WorkRead
	w.WorkID = id
	w.ProjectID = d.Item.ProjectID
	w.Item = d.Item
	e = s.repo.db.QueryRow(ctx, "SELECT p.name,p.status,cp.id,cp.chapter_no,cp.title,cp.status FROM projects p JOIN chapter_plans cp ON cp.project_id=p.id WHERE p.id=$1 AND cp.id=$2", d.Item.ProjectID, d.Item.ChapterPlanID).Scan(&w.ProjectName, &w.ProjectStatus, &w.ChapterID, &w.ChapterNo, &w.ChapterTitle, &w.ChapterStatus)
	if e != nil {
		return WorkRead{}, rewriteDatabaseError(e)
	}
	w.Current, e = s.GetVersion(ctx, d.Item.CurrentVersionID)
	if e != nil {
		return WorkRead{}, e
	}
	if e = s.repo.db.QueryRow(ctx, "SELECT count(*) FROM content_versions WHERE content_item_id=$1", id).Scan(&w.VersionCount); e != nil {
		return WorkRead{}, rewriteDatabaseError(e)
	}
	var rr ReviewReport
	e = s.repo.db.QueryRow(ctx, "SELECT "+reportColumns+" FROM review_reports WHERE content_item_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1", id).Scan(&rr.ID, &rr.ProjectID, &rr.ContentItemID, &rr.ContentVersionID, &rr.WorkflowRunID, &rr.ProviderKey, &rr.Status, &rr.Conclusion, &rr.Score, &rr.Summary, &rr.CreatedAt, &rr.CompletedAt)
	if e == nil {
		w.LatestReview = &rr
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return WorkRead{}, rewriteDatabaseError(e)
	}
	var run WorkflowRun
	e = s.repo.db.QueryRow(ctx, "SELECT "+rewriteRunColumns+" FROM workflow_runs WHERE content_item_id=$1 ORDER BY started_at DESC,id DESC LIMIT 1", id).Scan(&run.ID, &run.ProjectID, &run.ContentItemID, &run.ContentVersionID, &run.TargetContentVersionID, &run.SourceReviewReportID, &run.ProviderKey, &run.WorkflowKey, &run.SubjectType, &run.SubjectID, &run.Status, &run.IdempotencyKey, &run.RequestFingerprint, &run.InputJSON, &run.OutputJSON, &run.ErrorCode, &run.ErrorSummary, &run.StartedAt, &run.FinishedAt, &run.CreatedAt, &run.UpdatedAt)
	if e == nil {
		w.LatestRun = &run
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return WorkRead{}, rewriteDatabaseError(e)
	}
	var rewrite WorkflowRun
	e = s.repo.db.QueryRow(ctx, "SELECT "+rewriteRunColumns+" FROM workflow_runs WHERE content_item_id=$1 AND workflow_key=$2 ORDER BY started_at DESC,id DESC LIMIT 1", id, WorkflowKeyMockRewrite).Scan(&rewrite.ID, &rewrite.ProjectID, &rewrite.ContentItemID, &rewrite.ContentVersionID, &rewrite.TargetContentVersionID, &rewrite.SourceReviewReportID, &rewrite.ProviderKey, &rewrite.WorkflowKey, &rewrite.SubjectType, &rewrite.SubjectID, &rewrite.Status, &rewrite.IdempotencyKey, &rewrite.RequestFingerprint, &rewrite.InputJSON, &rewrite.OutputJSON, &rewrite.ErrorCode, &rewrite.ErrorSummary, &rewrite.StartedAt, &rewrite.FinishedAt, &rewrite.CreatedAt, &rewrite.UpdatedAt)
	if e == nil {
		w.RewriteRun = &rewrite
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return WorkRead{}, fmt.Errorf("rewrite run: %w", rewriteDatabaseError(e))
	}
	return w, nil
}
