package planning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type projectReader interface {
	Get(context.Context, uuid.UUID) (project.Project, error)
}

type AuditWriter interface {
	Insert(context.Context, audit.Entry) error
}

type TransactionRunner interface {
	Run(context.Context, func(Repository, AuditWriter) error) error
}

type Service struct {
	projects     projectReader
	plannings    Repository
	transactions TransactionRunner
}

func NewService(projects projectReader, plannings Repository, transactions TransactionRunner) *Service {
	return &Service{projects: projects, plannings: plannings, transactions: transactions}
}

func NewPostgresService(projects projectReader, pool *pgxpool.Pool) *Service {
	return NewService(projects, NewPostgresRepository(pool), postgresTransactionRunner{pool: pool})
}

func (s *Service) GetProjectPlanning(ctx context.Context, projectID uuid.UUID) (Response, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return Response{}, err
	}
	value, err := s.plannings.GetByProjectID(ctx, projectID)
	if errors.Is(err, ErrNotFound) {
		return emptyResponse(projectID), nil
	}
	if err != nil {
		return Response{}, err
	}
	return responseFrom(value), nil
}

func (s *Service) PutProjectPlanning(ctx context.Context, projectID uuid.UUID, request SaveRequest, actorID string) (Response, error) {
	value, err := valueFromRequest(projectID, request, actorID)
	if err != nil {
		return Response{}, err
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return Response{}, err
	}

	var result Response
	err = s.transactions.Run(ctx, func(repository Repository, audits AuditWriter) error {
		current, getErr := repository.GetByProjectID(ctx, projectID)
		switch {
		case errors.Is(getErr, ErrNotFound):
			if *request.ExpectedVersion != 0 {
				return ErrVersionConflict
			}
			created, createErr := repository.Create(ctx, value)
			if createErr != nil {
				return createErr
			}
			result = responseFrom(created)
			return writeAudit(ctx, audits, actorID, "project_planning.created", projectID, nil, result)
		case getErr != nil:
			return getErr
		}

		if current.Version != *request.ExpectedVersion {
			return ErrVersionConflict
		}
		if sameContent(current, value) {
			result = responseFrom(current)
			return nil
		}
		before := responseFrom(current)
		updated, updateErr := repository.UpdateWithVersion(ctx, value, *request.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		result = responseFrom(updated)
		return writeAudit(ctx, audits, actorID, "project_planning.updated", projectID, &before, result)
	})
	if err != nil {
		return Response{}, err
	}
	return result, nil
}

func emptyResponse(projectID uuid.UUID) Response {
	return Response{
		ProjectID:       projectID,
		GoalsJSON:       json.RawMessage(`{"selling_points":[],"plot_summary":""}`),
		ConstraintsJSON: json.RawMessage(`{"emotional_tone":""}`),
	}
}

func responseFrom(value ProjectPlanning) Response {
	createdAt, updatedAt := value.CreatedAt, value.UpdatedAt
	return Response{
		ProjectID: value.ProjectID, Premise: value.Premise, Audience: value.Audience, Style: value.Style,
		GoalsJSON: value.GoalsJSON, ConstraintsJSON: value.ConstraintsJSON, Version: value.Version,
		CreatedAt: &createdAt, UpdatedAt: &updatedAt,
	}
}

func sameContent(current, replacement ProjectPlanning) bool {
	return current.Premise == replacement.Premise &&
		current.Audience == replacement.Audience &&
		current.Style == replacement.Style &&
		bytes.Equal(current.GoalsJSON, replacement.GoalsJSON) &&
		bytes.Equal(current.ConstraintsJSON, replacement.ConstraintsJSON)
}

func writeAudit(ctx context.Context, repository AuditWriter, actorID, action string, projectID uuid.UUID, before *Response, after Response) error {
	payload, err := json.Marshal(struct {
		Before *Response `json:"before"`
		After  Response  `json:"after"`
	}{Before: before, After: after})
	if err != nil {
		return fmt.Errorf("marshal planning audit payload: %w", err)
	}
	return repository.Insert(ctx, audit.Entry{
		ID: uuid.New(), ActorID: actorID, Action: action, SubjectType: "project_planning",
		SubjectID: projectID.String(), Payload: payload,
	})
}

func valueFromRequest(projectID uuid.UUID, request SaveRequest, actorID string) (ProjectPlanning, error) {
	if request.Premise == nil || request.Audience == nil || request.Style == nil ||
		request.ExpectedVersion == nil || request.GoalsJSON == nil || request.ConstraintsJSON == nil ||
		*request.ExpectedVersion < 0 || len(*request.Premise) > 500 ||
		len(*request.Audience) > 500 || len(*request.Style) > 120 {
		return ProjectPlanning{}, ErrValidation
	}
	goalsJSON, constraintsJSON, err := validateAndCanonicalizeJSON(request.GoalsJSON, request.ConstraintsJSON)
	if err != nil {
		return ProjectPlanning{}, err
	}
	return ProjectPlanning{
		ProjectID: projectID, Premise: *request.Premise, Audience: *request.Audience, Style: *request.Style,
		GoalsJSON: goalsJSON, ConstraintsJSON: constraintsJSON, CreatedBy: actorID,
	}, nil
}

type goals struct {
	SellingPoints *[]string `json:"selling_points"`
	PlotSummary   *string   `json:"plot_summary"`
}
type constraints struct {
	EmotionalTone *string `json:"emotional_tone"`
}

func validateAndCanonicalizeJSON(goalsJSON, constraintsJSON json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	var parsedGoals goals
	if err := decodeStrict(goalsJSON, &parsedGoals); err != nil ||
		parsedGoals.SellingPoints == nil || parsedGoals.PlotSummary == nil ||
		len(*parsedGoals.SellingPoints) > 20 || len(*parsedGoals.PlotSummary) > 10000 {
		return nil, nil, ErrValidation
	}
	seen := make(map[string]struct{}, len(*parsedGoals.SellingPoints))
	for _, item := range *parsedGoals.SellingPoints {
		if len(item) < 1 || len(item) > 100 {
			return nil, nil, ErrValidation
		}
		if _, ok := seen[item]; ok {
			return nil, nil, ErrValidation
		}
		seen[item] = struct{}{}
	}
	var parsedConstraints constraints
	if err := decodeStrict(constraintsJSON, &parsedConstraints); err != nil ||
		parsedConstraints.EmotionalTone == nil || len(*parsedConstraints.EmotionalTone) > 500 {
		return nil, nil, ErrValidation
	}
	canonicalGoals, _ := json.Marshal(struct {
		SellingPoints []string `json:"selling_points"`
		PlotSummary   string   `json:"plot_summary"`
	}{SellingPoints: *parsedGoals.SellingPoints, PlotSummary: *parsedGoals.PlotSummary})
	canonicalConstraints, _ := json.Marshal(struct {
		EmotionalTone string `json:"emotional_tone"`
	}{EmotionalTone: *parsedConstraints.EmotionalTone})
	return canonicalGoals, canonicalConstraints, nil
}

func decodeStrict(value json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) == nil {
		return errors.New("multiple JSON values")
	}
	return nil
}

type postgresTransactionRunner struct{ pool *pgxpool.Pool }

func (r postgresTransactionRunner) Run(ctx context.Context, fn func(Repository, AuditWriter) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin project planning transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := fn(NewPostgresRepositoryTx(tx), audit.NewRepository(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit project planning transaction: %w", err)
	}
	return nil
}
