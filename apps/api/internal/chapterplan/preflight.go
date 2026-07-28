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
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

var ErrWorkflowNotConfigured = errors.New("chapter planning workflow is not configured")
var ErrPreflightInputChanged = errors.New("chapter planning preflight input changed")

type PreflightRequest struct {
	GenerationMode         string                  `json:"generationMode"`
	Target                 GenerationTargetRequest `json:"target"`
	StorylineIDs           []uuid.UUID             `json:"storylineIds"`
	StorylineSelectionMode string                  `json:"storylineSelectionMode"`
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
	// Snapshot is intentionally not serialized by the HTTP layer.  It is the
	// exact immutable value whose digest is bound into the preflight token.
	Snapshot GenerationContextSnapshot
}

type frozenStorylineSnapshot struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	ParentID     *uuid.UUID `json:"parent_id"`
	Type         string     `json:"type"`
	Relation     string     `json:"relation"`
	Name         string     `json:"name"`
	Summary      string     `json:"summary"`
	StartChapter *int       `json:"start_chapter"`
	EndChapter   *int       `json:"end_chapter"`
	Status       string     `json:"status"`
	SortOrder    int        `json:"sort_order"`
	Version      int        `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func freezeStorylines(values []storyline.PlotLine) []frozenStorylineSnapshot {
	frozen := make([]frozenStorylineSnapshot, len(values))
	for i, value := range values {
		frozen[i] = frozenStorylineSnapshot{
			ID:           value.ID,
			ProjectID:    value.ProjectID,
			ParentID:     value.ParentID,
			Type:         value.Type,
			Relation:     value.Relation,
			Name:         value.Name,
			Summary:      value.Summary,
			StartChapter: value.StartChapter,
			EndChapter:   value.EndChapter,
			Status:       value.Status,
			SortOrder:    value.SortOrder,
			Version:      value.Version,
			CreatedAt:    value.CreatedAt,
			UpdatedAt:    value.UpdatedAt,
		}
	}
	return frozen
}

func canonicalDigest(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	// JSONB normalizes object key order when the generation context is persisted.
	// Normalize the complete value before hashing so the database representation can
	// always be used to reproduce the digest.
	var normalized any
	if err := json.Unmarshal(b, &normalized); err != nil {
		return "", err
	}
	b, err = json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// digestGenerationContext avoids a self-referential hash: inputDigest is a
// derived field and is not itself an input to the digest.
func digestGenerationContext(snapshot GenerationContextSnapshot) (string, error) {
	snapshot.InputDigest = ""
	return canonicalDigest(snapshot)
}

func (s *Service) snapshot(ctx context.Context, projectID uuid.UUID, request PreflightRequest, target BatchTarget) (GenerationContextSnapshot, error) {
	storylineIDs := append([]uuid.UUID(nil), request.StorylineIDs...)
	if request.StorylineSelectionMode == "auto_balanced" {
		storylineIDs = nil
	}
	sort.Slice(storylineIDs, func(i, j int) bool { return storylineIDs[i].String() < storylineIDs[j].String() })
	storylineIDs = compactUUIDs(storylineIDs)
	plans, err := s.plans.ListByProject(ctx, projectID)
	if err != nil {
		return GenerationContextSnapshot{}, err
	}
	base := make([]BaseChapterPlanContext, 0, len(plans))
	for _, plan := range plans {
		raw, err := s.baseChapterPlanSnapshot(ctx, projectID, plan)
		if err != nil {
			return GenerationContextSnapshot{}, err
		}
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
	input, err := json.Marshal(map[string]any{"generationMode": request.GenerationMode, "target": target, "storylineSelectionMode": request.StorylineSelectionMode, "storylineIds": storylineIDs, "contextOptions": json.RawMessage(defaultObject(request.ContextOptions)), "projectId": projectID})
	if err != nil {
		return GenerationContextSnapshot{}, err
	}
	story, err := json.Marshal(map[string]any{"selected": storylineIDs, "available": freezeStorylines(storylines), "materials": materials, "foreshadowings": foreshadowings})
	if err != nil {
		return GenerationContextSnapshot{}, err
	}
	return GenerationContextSnapshot{InputSnapshot: input, StorylineSnapshot: story, ContextOptions: json.RawMessage(defaultObject(request.ContextOptions)), AdditionalInstructions: request.AdditionalInstructions, BaseChapterPlans: base}, nil
}

func compactUUIDs(values []uuid.UUID) []uuid.UUID {
	if len(values) == 0 {
		return nil
	}
	compact := values[:1]
	for _, value := range values[1:] {
		if value != compact[len(compact)-1] {
			compact = append(compact, value)
		}
	}
	return compact
}

// baseChapterPlanSnapshot preserves the precise current revision when one exists.
// Historical plans without a revision deliberately contain only their actual current
// fields; missing revision-only fields are not synthesized into a false frozen state.
func (s *Service) baseChapterPlanSnapshot(ctx context.Context, projectID uuid.UUID, plan Plan) (json.RawMessage, error) {
	if plan.CurrentRevisionID != nil {
		repo, ok := s.plans.(*Repository)
		if !ok {
			return nil, ErrInternal
		}
		var raw json.RawMessage
		err := repo.db.QueryRow(ctx, `SELECT snapshot FROM chapter_plan_revisions WHERE id=$1 AND chapter_plan_id=$2 AND project_id=$3`, *plan.CurrentRevisionID, plan.ID, projectID).Scan(&raw)
		if err != nil {
			return nil, err
		}
		var snapshot candidateSnapshotStruct
		if !json.Valid(raw) || json.Unmarshal(raw, &snapshot) != nil {
			return nil, ErrInvalidReference
		}
		return raw, nil
	}
	return json.Marshal(struct {
		ChapterNo         int            `json:"chapterNo"`
		Title             string         `json:"title"`
		Summary           string         `json:"summary"`
		StorylineRefs     []StorylineRef `json:"storylineRefs"`
		MaterialRefs      []uuid.UUID    `json:"materialRefs"`
		ForeshadowingRefs []uuid.UUID    `json:"foreshadowingRefs"`
	}{plan.ChapterNo, plan.Title, plan.Summary, plan.Storylines, plan.Materials, plan.Foreshadowings})
}
func defaultObject(v json.RawMessage) []byte {
	if len(v) == 0 || !json.Valid(v) || !strings.HasPrefix(strings.TrimSpace(string(v)), "{") {
		return []byte("{}")
	}
	return v
}

// validContextOptions mirrors the frozen request schema.  Preflight is a
// report, so invalid generation input is represented as a normal blocker,
// never silently replaced with an empty object.
func validContextOptions(v json.RawMessage) bool {
	if len(v) == 0 || !json.Valid(v) {
		return false
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(v, &values); err != nil || len(values) != 4 {
		return false
	}
	for _, key := range []string{"includeProjectMaterials", "includeUnpaidForeshadowings", "includePriorChapterSummaries", "coreSettingsOnly"} {
		value, ok := values[key]
		if !ok {
			return false
		}
		var b *bool
		if err := json.Unmarshal(value, &b); err != nil || b == nil {
			return false
		}
	}
	return true
}

func blockedPreflight(result PreflightResult, code, message, retryAction, safeReason string) PreflightResult {
	result.Passed = false
	result.Token = ""
	result.ExpiresAt = time.Time{}
	result.Blockers = append(result.Blockers, PreflightBlocker{Code: code, Message: message, RetryAction: retryAction, SafeReason: safeReason})
	return result
}

// invalidPreflightInputDigest uses the same normalized request representation for
// every early invalid-input blocker. These branches cannot build a generation
// snapshot, but must still return a stable digest for the exact request.
func invalidPreflightInputDigest(projectID uuid.UUID, request PreflightRequest) (string, error) {
	return canonicalDigest(struct {
		ProjectID uuid.UUID
		Request   PreflightRequest
	}{projectID, request})
}

func (s *Service) Preflight(ctx context.Context, projectID uuid.UUID, request PreflightRequest) (PreflightResult, error) {
	if projectID == uuid.Nil || strings.TrimSpace(request.ActorID) == "" {
		return PreflightResult{}, ErrValidation
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return PreflightResult{}, err
	}
	if !validContextOptions(request.ContextOptions) {
		digest, err := invalidPreflightInputDigest(projectID, request)
		if err != nil {
			return PreflightResult{}, err
		}
		return blockedPreflight(PreflightResult{InputDigest: digest}, "generation_input_invalid", "generation input is invalid", "review_generation_input", "All generation options must be provided."), nil
	}
	if request.StorylineSelectionMode != "auto_balanced" && request.StorylineSelectionMode != "specified" || request.StorylineSelectionMode == "specified" && len(request.StorylineIDs) == 0 {
		digest, err := invalidPreflightInputDigest(projectID, request)
		if err != nil {
			return PreflightResult{}, err
		}
		return blockedPreflight(PreflightResult{InputDigest: digest}, "storyline_reference_invalid", "storyline selection is invalid", "review_storyline_selection", "Choose valid project storylines."), nil
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
		digest, digestErr := invalidPreflightInputDigest(projectID, request)
		if digestErr != nil {
			return PreflightResult{}, digestErr
		}
		return blockedPreflight(PreflightResult{InputDigest: digest}, "generation_input_invalid", "generation target is invalid", "review_generation_target", "The requested chapter range is not available."), nil
	}
	snapshot, err := s.snapshot(ctx, projectID, request, target)
	if err != nil {
		return PreflightResult{}, err
	}
	result := PreflightResult{Target: target}
	digest, err := digestGenerationContext(snapshot)
	if err != nil {
		return result, err
	}
	snapshot.InputDigest, result.InputDigest, result.Snapshot = digest, digest, snapshot
	var frozenStorylines struct {
		Available []frozenStorylineSnapshot `json:"available"`
	}
	if err := json.Unmarshal(snapshot.StorylineSnapshot, &frozenStorylines); err != nil {
		return result, ErrInternal
	}
	if len(frozenStorylines.Available) == 0 {
		return blockedPreflight(result, "storyline_reference_invalid", "no project storyline is available", "configure_project_storyline", "At least one project storyline is required by the chapter-planning output contract."), nil
	}
	if request.StorylineSelectionMode == "specified" {
		available := make(map[uuid.UUID]struct{}, len(frozenStorylines.Available))
		for _, value := range frozenStorylines.Available {
			if value.ProjectID == projectID && value.Status == "active" {
				available[value.ID] = struct{}{}
			}
		}
		for _, storylineID := range request.StorylineIDs {
			if _, ok := available[storylineID]; !ok {
				return blockedPreflight(result, "storyline_reference_invalid", "storyline selection contains an unavailable reference", "review_storyline_selection", "Choose active storylines from the current project."), nil
			}
		}
	}
	if s.bindingReader == nil || s.workflowReader == nil || s.connectionReader == nil {
		return blockedPreflight(result, "project_binding_missing", "chapter-planning workflow binding is missing", "configure_workflow", "This project has no executable chapter-planning workflow."), nil
	}
	binding, err := s.bindingReader.GetByProjectAndStage(ctx, projectID, workflowbinding.StageChapterPlanning)
	if err != nil {
		return blockedPreflight(result, "project_binding_missing", "chapter-planning workflow binding is missing", "configure_workflow", "This project has no executable chapter-planning workflow."), nil
	}
	workflow, err := s.workflowReader.GetWorkflow(ctx, binding.WorkflowConfigurationID)
	if err != nil {
		return blockedPreflight(result, "execution_integration_unavailable", "workflow configuration is unavailable", "review_workflow_configuration", "The configured workflow cannot execute."), nil
	}
	connection, err := s.connectionReader.GetConnection(ctx, workflow.ConnectionID)
	if err != nil {
		return blockedPreflight(result, "execution_integration_unavailable", "workflow connection is unavailable", "review_workflow_connection", "The configured workflow cannot execute."), nil
	}
	if !workflow.Enabled || !connection.Enabled {
		return blockedPreflight(result, "execution_integration_unavailable", "workflow execution is unavailable", "enable_workflow", "The configured workflow cannot execute."), nil
	}
	if repo, ok := s.plans.(*Repository); ok {
		active, err := repo.ActiveChapterPlanningRun(ctx, projectID)
		if err != nil {
			return result, err
		}
		if active {
			return blockedPreflight(result, "active_run_conflict", "a chapter-planning run is already active", "wait_for_active_run", "Only one queued or running chapter-planning run is allowed."), nil
		}
	}
	bindingSnap, err := json.Marshal(map[string]any{"stage": "chapter_planning", "workflowBindingId": binding.ID, "workflowBindingVersion": binding.Version, "workflowConfigurationId": workflow.ID, "workflowConfigurationVersion": workflow.Version, "workflowConfigurationSource": workflow.WorkflowType})
	if err != nil {
		return result, err
	}
	snapshot.WorkflowBindingSnapshot = bindingSnap
	digest, err = digestGenerationContext(snapshot)
	if err != nil {
		return result, err
	}
	snapshot.InputDigest = digest
	result.InputDigest, result.BindingID, result.BindingVersion, result.Snapshot = digest, binding.ID, binding.Version, snapshot
	repo, ok := s.plans.(*Repository)
	if !ok {
		return result, ErrInternal
	}
	now := s.now().UTC()
	claims := PreflightTokenClaims{ProjectID: projectID, ActorID: request.ActorID, Stage: "chapter_planning", GenerationMode: request.GenerationMode, StorylineSelectionMode: request.StorylineSelectionMode, StorylineIDs: request.StorylineIDs, ContextOptions: json.RawMessage(defaultObject(request.ContextOptions)), AdditionalInstructions: request.AdditionalInstructions, Target: target, InputDigest: digest, BindingID: binding.ID, BindingVersion: binding.Version, IssuedAt: now.Unix(), ExpiresAt: now.Add(10 * time.Minute).Unix(), Nonce: uuid.NewString()}
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
	prepare := func() (workflowrun.CreateRunCommand, error) {
		claims, err := VerifyPreflightToken(repo.HMACSecret(), token, s.now())
		if err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		if claims.ProjectID != projectID || claims.ActorID != actorID {
			return workflowrun.CreateRunCommand{}, ErrPreflightInputChanged
		}
		if claims.StorylineSelectionMode == "auto_balanced" && len(claims.StorylineIDs) != 0 {
			return workflowrun.CreateRunCommand{}, ErrPreflightInputChanged
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
		request := PreflightRequest{GenerationMode: claims.GenerationMode, Target: target, StorylineSelectionMode: claims.StorylineSelectionMode, StorylineIDs: claims.StorylineIDs, ContextOptions: claims.ContextOptions, AdditionalInstructions: claims.AdditionalInstructions, ActorID: actorID}
		current, err := s.Preflight(ctx, projectID, request)
		if err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		if current.InputDigest != claims.InputDigest || current.Target != claims.Target || current.BindingID != claims.BindingID || current.BindingVersion != claims.BindingVersion {
			return workflowrun.CreateRunCommand{}, ErrPreflightInputChanged
		}
		snapshot := current.Snapshot
		if snapshot.InputDigest != claims.InputDigest {
			return workflowrun.CreateRunCommand{}, ErrPreflightInputChanged
		}
		persistedDigest, err := digestGenerationContext(snapshot)
		if err != nil || persistedDigest != claims.InputDigest {
			return workflowrun.CreateRunCommand{}, ErrPreflightInputChanged
		}
		payload, err := json.Marshal(map[string]any{"generationContextDigest": claims.InputDigest, "target": claims.Target, "stage": "chapter_planning", "generationContext": snapshot})
		if err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		return workflowrun.CreateRunCommand{ProjectID: projectID, Stage: "chapter_planning", InputPayload: payload, TriggerSource: "manual"}, nil
	}
	if creator, supported := s.runCreator.(interface {
		CreateRunIdempotent(context.Context, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error)
	}); supported {
		requestHash := workflowrun.Fingerprint(struct {
			ProjectID uuid.UUID
			ActorID   string
			Token     string
		}{ProjectID: projectID, ActorID: actorID, Token: token})
		return creator.CreateRunIdempotent(ctx, projectID, key, requestHash, prepare)
	}
	command, err := prepare()
	if err != nil {
		return workflowrun.WorkflowRun{}, err
	}
	command.IdempotencyKey = key
	return s.runCreator.CreateRun(ctx, command)
}
