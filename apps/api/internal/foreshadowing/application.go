package foreshadowing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

var (
	ErrValidation        = errors.New("foreshadowing validation failed")
	ErrInvalidPriority   = errors.New("foreshadowing priority invalid")
	ErrInvalidStatus     = errors.New("foreshadowing status invalid")
	ErrChapterRange      = errors.New("foreshadowing chapter range invalid")
	ErrStorylineNotFound = errors.New("referenced storyline not found")
)

type projectReader interface {
	Get(context.Context, uuid.UUID) (project.Project, error)
}

type storylineReader interface {
	GetByID(context.Context, uuid.UUID) (storyline.PlotLine, error)
}

type AuditWriter interface {
	Insert(context.Context, audit.Entry) error
}

type TransactionRunner interface {
	Run(context.Context, func(Repository, AuditWriter) error) error
}

type ListResult struct {
	Items []Foreshadowing
}

type CreateCommand struct {
	Title, Description, Priority, Status      string
	PlantedPlotLineID, PayoffPlotLineID       *uuid.UUID
	PlannedPlantChapter, PlannedPayoffChapter *int
	ActorID                                   string
}

// OptionalUUID represents an omitted PATCH field, a UUID value, or an explicit null.
type OptionalUUID struct {
	Set   bool
	Value *uuid.UUID
}

// OptionalInt represents an omitted PATCH field, an integer value, or an explicit null.
type OptionalInt struct {
	Set   bool
	Value *int
}

type UpdateCommand struct {
	ExpectedVersion                           int
	Title, Description, Priority, Status      *string
	PlantedPlotLineID, PayoffPlotLineID       OptionalUUID
	PlannedPlantChapter, PlannedPayoffChapter OptionalInt
	ActorID                                   string
}

type Application interface {
	List(context.Context, uuid.UUID) (ListResult, error)
	Create(context.Context, uuid.UUID, CreateCommand) (Foreshadowing, error)
	Update(context.Context, uuid.UUID, UpdateCommand) (Foreshadowing, error)
}

type Service struct {
	projects     projectReader
	storylines   storylineReader
	repository   Repository
	transactions TransactionRunner
}

func NewService(projects projectReader, storylines storylineReader, repository Repository, transactions TransactionRunner) *Service {
	return &Service{projects: projects, storylines: storylines, repository: repository, transactions: transactions}
}

func NewPostgresService(projects projectReader, pool *pgxpool.Pool) *Service {
	return NewService(projects, storyline.NewPostgresRepository(pool), NewPostgresRepository(pool), postgresTransactionRunner{pool: pool})
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) (ListResult, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return ListResult{}, err
	}
	items, err := s.repository.ListByProject(ctx, projectID)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: items}, nil
}

func (s *Service) Create(ctx context.Context, projectID uuid.UUID, command CreateCommand) (Foreshadowing, error) {
	value := Foreshadowing{ID: uuid.New(), ProjectID: projectID, Title: command.Title, Description: command.Description, Priority: command.Priority, Status: command.Status, PlantedPlotLineID: command.PlantedPlotLineID, PayoffPlotLineID: command.PayoffPlotLineID, PlannedPlantChapter: command.PlannedPlantChapter, PlannedPayoffChapter: command.PlannedPayoffChapter, CreatedBy: command.ActorID}
	if err := validate(value); err != nil {
		return Foreshadowing{}, err
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return Foreshadowing{}, err
	}
	if err := s.validateReferences(ctx, projectID, value.PlantedPlotLineID, value.PayoffPlotLineID); err != nil {
		return Foreshadowing{}, err
	}
	var created Foreshadowing
	err := s.transactions.Run(ctx, func(repository Repository, audits AuditWriter) error {
		var err error
		created, err = repository.Create(ctx, value)
		if err != nil {
			return err
		}
		return writeAudit(ctx, audits, command.ActorID, "foreshadowing.created", created.ID, nil, created)
	})
	if err != nil {
		return Foreshadowing{}, err
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, foreshadowingID uuid.UUID, command UpdateCommand) (Foreshadowing, error) {
	if command.ExpectedVersion < 1 || !hasUpdate(command) {
		return Foreshadowing{}, ErrValidation
	}
	var result Foreshadowing
	err := s.transactions.Run(ctx, func(repository Repository, audits AuditWriter) error {
		current, err := repository.GetByID(ctx, foreshadowingID)
		if err != nil {
			return err
		}
		if current.Version != command.ExpectedVersion {
			return ErrVersionConflict
		}
		next := applyUpdate(current, command)
		if err := validate(next); err != nil {
			return err
		}
		if !valid(current.Status, next.Status) {
			return ErrInvalidTransition
		}
		if err := s.validateReferences(ctx, current.ProjectID, next.PlantedPlotLineID, next.PayoffPlotLineID); err != nil {
			return err
		}
		if same(current, next) {
			result = current
			return nil
		}
		result, err = repository.UpdateWithVersion(ctx, next, command.ExpectedVersion)
		if err != nil {
			return err
		}
		return writeAudit(ctx, audits, command.ActorID, "foreshadowing.updated", result.ID, &current, result)
	})
	if err != nil {
		return Foreshadowing{}, err
	}
	return result, nil
}

func (s *Service) validateReferences(ctx context.Context, projectID uuid.UUID, plantedID, payoffID *uuid.UUID) error {
	for _, id := range []*uuid.UUID{plantedID, payoffID} {
		if id == nil {
			continue
		}
		line, err := s.storylines.GetByID(ctx, *id)
		if errors.Is(err, storyline.ErrNotFound) {
			return ErrStorylineNotFound
		}
		if err != nil {
			return err
		}
		if line.ProjectID != projectID {
			return ErrProjectMismatch
		}
	}
	return nil
}

func applyUpdate(current Foreshadowing, command UpdateCommand) Foreshadowing {
	next := current
	if command.Title != nil {
		next.Title = *command.Title
	}
	if command.Description != nil {
		next.Description = *command.Description
	}
	if command.Priority != nil {
		next.Priority = *command.Priority
	}
	if command.Status != nil {
		next.Status = *command.Status
	}
	if command.PlantedPlotLineID.Set {
		next.PlantedPlotLineID = command.PlantedPlotLineID.Value
	}
	if command.PayoffPlotLineID.Set {
		next.PayoffPlotLineID = command.PayoffPlotLineID.Value
	}
	if command.PlannedPlantChapter.Set {
		next.PlannedPlantChapter = command.PlannedPlantChapter.Value
	}
	if command.PlannedPayoffChapter.Set {
		next.PlannedPayoffChapter = command.PlannedPayoffChapter.Value
	}
	return next
}

func hasUpdate(command UpdateCommand) bool {
	return command.Title != nil || command.Description != nil || command.Priority != nil || command.Status != nil || command.PlantedPlotLineID.Set || command.PayoffPlotLineID.Set || command.PlannedPlantChapter.Set || command.PlannedPayoffChapter.Set
}

func validate(value Foreshadowing) error {
	if strings.TrimSpace(value.Title) == "" || len(value.Title) > 120 || len(value.Description) > 5000 {
		return ErrValidation
	}
	switch value.Priority {
	case "low", "medium", "high":
	default:
		return ErrInvalidPriority
	}
	switch value.Status {
	case "planned", "planted", "paid_off":
	default:
		return ErrInvalidStatus
	}
	if !validRange(value.PlannedPlantChapter, value.PlannedPayoffChapter) {
		return ErrChapterRange
	}
	return nil
}

func validRange(plant, payoff *int) bool {
	return (plant == nil || *plant >= 1) && (payoff == nil || *payoff >= 1) && (plant == nil || payoff == nil || *plant <= *payoff)
}

func writeAudit(ctx context.Context, writer AuditWriter, actorID, action string, id uuid.UUID, before *Foreshadowing, after Foreshadowing) error {
	payload, err := json.Marshal(struct {
		Before *Foreshadowing `json:"before"`
		After  Foreshadowing  `json:"after"`
	}{Before: before, After: after})
	if err != nil {
		return fmt.Errorf("marshal foreshadowing audit payload: %w", err)
	}
	return writer.Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: actorID, Action: action, SubjectType: "foreshadowing", SubjectID: id.String(), Payload: payload})
}

type postgresTransactionRunner struct{ pool *pgxpool.Pool }

func (r postgresTransactionRunner) Run(ctx context.Context, fn func(Repository, AuditWriter) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin foreshadowing transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := fn(NewPostgresRepositoryTx(tx), audit.NewRepository(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit foreshadowing transaction: %w", err)
	}
	return nil
}
