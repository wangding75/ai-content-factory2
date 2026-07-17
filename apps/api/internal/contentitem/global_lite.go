package contentitem

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// GlobalLiteService is the read-only Iteration 08 projection boundary. It
// deliberately reuses QueryService and persisted content records; it owns no
// lifecycle and has no write methods.
type GlobalLiteService struct{ query *QueryService }

func NewGlobalLiteService(query *QueryService) *GlobalLiteService {
	return &GlobalLiteService{query: query}
}

type GlobalWorkList struct {
	Items                []WorkRead
	Total, Limit, Offset int
}

func (s *GlobalLiteService) ListWorks(ctx context.Context, limit, offset int) (GlobalWorkList, error) {
	var total int
	if err := s.query.repo.db.QueryRow(ctx, "SELECT count(*) FROM content_items ci JOIN chapter_plans cp ON cp.id=ci.chapter_plan_id JOIN projects p ON p.id=ci.project_id").Scan(&total); err != nil {
		return GlobalWorkList{}, rewriteDatabaseError(err)
	}
	rows, err := s.query.repo.db.Query(ctx, "SELECT ci.id FROM content_items ci JOIN chapter_plans cp ON cp.id=ci.chapter_plan_id JOIN projects p ON p.id=ci.project_id ORDER BY p.updated_at DESC, cp.chapter_no ASC, ci.id ASC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return GlobalWorkList{}, rewriteDatabaseError(err)
	}
	defer rows.Close()
	out := GlobalWorkList{Total: total, Limit: limit, Offset: offset, Items: make([]WorkRead, 0)}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return GlobalWorkList{}, rewriteDatabaseError(err)
		}
		work, err := s.query.GetWork(ctx, id)
		if err != nil {
			return GlobalWorkList{}, err
		}
		out.Items = append(out.Items, work)
	}
	if err := rows.Err(); err != nil {
		return GlobalWorkList{}, rewriteDatabaseError(err)
	}
	return out, nil
}

type GlobalWorkflowRun struct {
	Run                        WorkflowRun
	ProjectID                  *uuid.UUID
	ProjectName, ProjectStatus string
}
type GlobalWorkflowRunList struct {
	Items                []GlobalWorkflowRun
	Total, Limit, Offset int
}

func (s *GlobalLiteService) ListWorkflowRuns(ctx context.Context, limit, offset int) (GlobalWorkflowRunList, error) {
	var total int
	if err := s.query.repo.db.QueryRow(ctx, "SELECT count(*) FROM workflow_runs").Scan(&total); err != nil {
		return GlobalWorkflowRunList{}, rewriteDatabaseError(err)
	}
	rows, err := s.query.repo.db.Query(ctx, "SELECT wr.id,wr.project_id,wr.content_item_id,wr.content_version_id,wr.provider_key,wr.workflow_key,wr.subject_type,wr.subject_id,wr.status,wr.idempotency_key,wr.request_fingerprint,wr.input_json,wr.output_json,wr.error_code,wr.error_summary,wr.started_at,wr.finished_at,wr.created_at,wr.updated_at,p.id,p.name,p.status FROM workflow_runs wr LEFT JOIN projects p ON p.id=wr.project_id ORDER BY wr.started_at DESC,wr.id DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return GlobalWorkflowRunList{}, rewriteDatabaseError(err)
	}
	defer rows.Close()
	out := GlobalWorkflowRunList{Total: total, Limit: limit, Offset: offset, Items: make([]GlobalWorkflowRun, 0)}
	for rows.Next() {
		var x GlobalWorkflowRun
		var id *uuid.UUID
		if err := rows.Scan(&x.Run.ID, &x.Run.ProjectID, &x.Run.ContentItemID, &x.Run.ContentVersionID, &x.Run.ProviderKey, &x.Run.WorkflowKey, &x.Run.SubjectType, &x.Run.SubjectID, &x.Run.Status, &x.Run.IdempotencyKey, &x.Run.RequestFingerprint, &x.Run.InputJSON, &x.Run.OutputJSON, &x.Run.ErrorCode, &x.Run.ErrorSummary, &x.Run.StartedAt, &x.Run.FinishedAt, &x.Run.CreatedAt, &x.Run.UpdatedAt, &id, &x.ProjectName, &x.ProjectStatus); err != nil {
			return GlobalWorkflowRunList{}, fmt.Errorf("global workflow run scan: %w", rewriteDatabaseError(err))
		}
		x.ProjectID = id
		out.Items = append(out.Items, x)
	}
	if err := rows.Err(); err != nil {
		return GlobalWorkflowRunList{}, rewriteDatabaseError(err)
	}
	return out, nil
}

type BuiltinWorkflowDefinition struct{ WorkflowKey, ProviderKey, Label, Description, Status, ResultKind string }
type CapabilityDescriptor struct {
	Key, Label, Status, Description string
	WorkflowKeys                    []string
}
type IntegrationDescriptor struct{ Key, Label, Category, Status, Description string }

func BuiltinWorkflows() []BuiltinWorkflowDefinition {
	return []BuiltinWorkflowDefinition{
		{"chapter_plan_mock_generate", "mock", "Chapter plan generation", "Built-in deterministic chapter-plan generation.", "enabled", "chapter_plan"},
		{"content_mock_generate", "mock", "Content generation", "Built-in deterministic content generation.", "enabled", "content"},
		{"content_mock_review", "mock", "Content review", "Built-in deterministic content review.", "enabled", "review"},
		{"content_mock_rewrite", "mock", "Content rewrite", "Built-in deterministic content rewrite.", "enabled", "rewrite"},
	}
}
func Capabilities() []CapabilityDescriptor {
	return []CapabilityDescriptor{
		{"mock_content", "Mock content", "enabled", "Built-in mock content workflows are enabled.", []string{"chapter_plan_mock_generate", "content_mock_generate", "content_mock_review", "content_mock_rewrite"}},
		{"real_ai", "Real AI", "not_configured", "Real AI is not configured in P0.", []string{}},
	}
}
func Integrations() []IntegrationDescriptor {
	return []IntegrationDescriptor{
		{"wechat", "WeChat", "publish", "not_available", "Publishing integrations are not available in P0."},
		{"douyin", "Douyin", "publish", "not_available", "Publishing integrations are not available in P0."},
		{"youtube", "YouTube", "publish", "not_available", "Publishing integrations are not available in P0."},
		{"n8n", "n8n", "workflow", "not_available", "External workflow integrations are not available in P0."},
		{"coze", "Coze", "workflow", "not_available", "External workflow integrations are not available in P0."},
		{"comfyui", "ComfyUI", "workflow", "not_available", "External workflow integrations are not available in P0."},
	}
}
