package chapterplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

var ErrWorkflowNotConfigured = errors.New("chapter planning workflow is not configured")
var ErrPreflightInputChanged = errors.New("chapter planning preflight input changed")

type PreflightRequest struct {
	GenerationMode         string                  `json:"generationMode"`
	Target                 GenerationTargetRequest `json:"target"`
	StorylineIDs           []uuid.UUID             `json:"storylineIds"`
	ContextOptions         json.RawMessage         `json:"contextOptions"`
	AdditionalInstructions *string                 `json:"additionalInstructions"`
	ActorID                string                  `json:"-"`
}
type PreflightBlocker struct{ Code, Message, RetryAction, SafeReason string }
type PreflightResult struct {
	Passed         bool
	Token          string
	ExpiresAt      time.Time
	InputDigest    string
	Target         BatchTarget
	BindingID      uuid.UUID
	BindingVersion int
	Blockers       []PreflightBlocker
}

func canonicalDigest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) snapshot(ctx context.Context, projectID uuid.UUID, request PreflightRequest, target BatchTarget) (GenerationContextSnapshot, error) {
	plans, err := s.plans.ListByProject(ctx, projectID)
	if err != nil {
		return GenerationContextSnapshot{}, err
	}
	base := make([]BaseChapterPlanContext, 0, len(plans))
	for _, plan := range plans {
		raw, _ := json.Marshal(map[string]any{"id": plan.ID, "chapterNo": plan.ChapterNo, "title": plan.Title, "summary": plan.Summary, "version": plan.Version})
		base = append(base, BaseChapterPlanContext{ID: plan.ID, ChapterNo: plan.ChapterNo, Version: plan.Version, RevisionID: plan.CurrentRevisionID, Snapshot: raw})
	}
	sort.Slice(base, func(i, j int) bool { return base[i].ChapterNo < base[j].ChapterNo })
	storylines, err := s.storylines.ListByProject(ctx, projectID)
	if err != nil {
		return GenerationContextSnapshot{}, err
	}
	sort.Slice(storylines, func(i, j int) bool { return storylines[i].ID.String() < storylines[j].ID.String() })
	materials, err := s.materials.ListByProject(ctx, projectID)
	if err != nil {
		return GenerationContextSnapshot{}, err
	}
	sort.Slice(materials, func(i, j int) bool { return materials[i].MaterialID.String() < materials[j].MaterialID.String() })
	foreshadowings, err := s.foreshadowings.ListByProject(ctx, projectID)
	if err != nil {
		return GenerationContextSnapshot{}, err
	}
	sort.Slice(foreshadowings, func(i, j int) bool { return foreshadowings[i].ID.String() < foreshadowings[j].ID.String() })
	input, _ := json.Marshal(map[string]any{"generationMode": request.GenerationMode, "target": target, "storylineIds": request.StorylineIDs, "contextOptions": json.RawMessage(defaultObject(request.ContextOptions)), "projectId": projectID})
	story, _ := json.Marshal(map[string]any{"selected": request.StorylineIDs, "available": storylines, "materials": materials, "foreshadowings": foreshadowings})
	return GenerationContextSnapshot{InputSnapshot: input, StorylineSnapshot: story, ContextOptions: json.RawMessage(defaultObject(request.ContextOptions)), AdditionalInstructions: request.AdditionalInstructions, BaseChapterPlans: base}, nil
}
func defaultObject(v json.RawMessage) []byte {
	if len(v) == 0 || !json.Valid(v) || !strings.HasPrefix(strings.TrimSpace(string(v)), "{") {
		return []byte("{}")
	}
	return v
}

func (s *Service) Preflight(ctx context.Context, projectID uuid.UUID, request PreflightRequest) (PreflightResult, error) {
	if projectID == uuid.Nil || strings.TrimSpace(request.ActorID) == "" {
		return PreflightResult{}, ErrValidation
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return PreflightResult{}, err
	}
	plans, err := s.plans.ListByProject(ctx, projectID)
	if err != nil {
		return PreflightResult{}, err
	}
	max := 0
	for _, p := range plans {
		if p.ChapterNo > max {
			max = p.ChapterNo
		}
	}
	target, err := NormalizeGenerationTarget(request.GenerationMode, request.Target, max)
	if err != nil {
		return PreflightResult{}, err
	}
	snapshot, err := s.snapshot(ctx, projectID, request, target)
	if err != nil {
		return PreflightResult{}, err
	}
	result := PreflightResult{Target: target}
	if s.bindingReader == nil || s.workflowReader == nil || s.connectionReader == nil {
		return result, ErrWorkflowNotConfigured
	}
	binding, err := s.bindingReader.GetByProjectAndStage(ctx, projectID, workflowbinding.StageChapterPlanning)
	if err != nil {
		return result, ErrWorkflowNotConfigured
	}
	workflow, err := s.workflowReader.GetWorkflow(ctx, binding.WorkflowConfigurationID)
	if err != nil {
		return result, ErrWorkflowNotConfigured
	}
	connection, err := s.connectionReader.GetConnection(ctx, workflow.ConnectionID)
	if err != nil {
		return result, ErrWorkflowNotConfigured
	}
	if !workflow.Enabled || !connection.Enabled {
		return result, ErrWorkflowNotConfigured
	}
	bindingSnap, _ := json.Marshal(map[string]any{"stage": "chapter_planning", "workflowBindingId": binding.ID, "workflowBindingVersion": binding.Version, "workflowConfigurationId": workflow.ID, "workflowConfigurationVersion": workflow.Version, "workflowConfigurationSource": workflow.WorkflowType})
	snapshot.WorkflowBindingSnapshot = bindingSnap
	digest, err := canonicalDigest(snapshot)
	if err != nil {
		return result, err
	}
	snapshot.InputDigest = digest
	result.InputDigest, result.BindingID, result.BindingVersion = digest, binding.ID, binding.Version
	repo, ok := s.plans.(*Repository)
	if !ok {
		return result, ErrInternal
	}
	claims := PreflightTokenClaims{ProjectID: projectID, ActorID: request.ActorID, Stage: "chapter_planning", GenerationMode: request.GenerationMode, StorylineIDs: request.StorylineIDs, ContextOptions: json.RawMessage(defaultObject(request.ContextOptions)), AdditionalInstructions: request.AdditionalInstructions, Target: target, InputDigest: digest, BindingID: binding.ID, BindingVersion: binding.Version, IssuedAt: s.now().UTC().Unix(), ExpiresAt: s.now().UTC().Add(10 * time.Minute).Unix(), Nonce: uuid.NewString()}
	token, err := SignPreflightToken(repo.HMACSecret(), claims)
	if err != nil {
		return result, err
	}
	result.Passed, result.Token, result.ExpiresAt = true, token, time.Unix(claims.ExpiresAt, 0).UTC()
	return result, nil
}

func (s *Service) CreateChapterPlanningRun(ctx context.Context, projectID uuid.UUID, actorID, token, key string) (workflowrun.WorkflowRun, error) {
	if s.runCreator == nil {
		return workflowrun.WorkflowRun{}, ErrWorkflowNotConfigured
	}
	repo, ok := s.plans.(*Repository)
	if !ok {
		return workflowrun.WorkflowRun{}, ErrInternal
	}
	claims, err := VerifyPreflightToken(repo.HMACSecret(), token, s.now())
	if err != nil {
		return workflowrun.WorkflowRun{}, err
	}
	if claims.ProjectID != projectID || claims.ActorID != actorID {
		return workflowrun.WorkflowRun{}, ErrPreflightInputChanged
	}
	// Rebuild the immutable snapshot facts before delegation; the token digest is the
	// authoritative comparison and prevents business drift between preflight/create.
	target := GenerationTargetRequest{StartChapterNo: claims.Target.StartChapterNo, EndChapterNo: claims.Target.EndChapterNo}
	if claims.GenerationMode == "full" {
		target.TargetTotalChapters = claims.Target.RequestedChapterCount
	}
	if claims.GenerationMode == "append" {
		target.ChapterCount = claims.Target.RequestedChapterCount
	}
	request := PreflightRequest{GenerationMode: claims.GenerationMode, Target: target, StorylineIDs: claims.StorylineIDs, ContextOptions: claims.ContextOptions, AdditionalInstructions: claims.AdditionalInstructions, ActorID: actorID}
	current, err := s.Preflight(ctx, projectID, request)
	if err != nil {
		return workflowrun.WorkflowRun{}, err
	}
	if current.InputDigest != claims.InputDigest || current.BindingID != claims.BindingID || current.BindingVersion != claims.BindingVersion {
		return workflowrun.WorkflowRun{}, ErrPreflightInputChanged
	}
	snapshot, err := s.snapshot(ctx, projectID, request, claims.Target)
	if err != nil {
		return workflowrun.WorkflowRun{}, err
	}
	bindingSnapshot, _ := json.Marshal(map[string]any{"stage": "chapter_planning", "workflowBindingId": claims.BindingID, "workflowBindingVersion": claims.BindingVersion})
	snapshot.WorkflowBindingSnapshot = bindingSnapshot
	snapshot.InputDigest = claims.InputDigest
	payload, _ := json.Marshal(map[string]any{"generationContextDigest": claims.InputDigest, "target": claims.Target, "stage": "chapter_planning", "generationContext": snapshot})
	return s.runCreator.CreateRun(ctx, workflowrun.CreateRunCommand{ProjectID: projectID, Stage: "chapter_planning", InputPayload: payload, TriggerSource: "manual", IdempotencyKey: key})
}
