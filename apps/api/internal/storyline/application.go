package storyline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

var (
	ErrValidation            = errors.New("storyline validation failed")
	ErrParentNotFound        = errors.New("storyline parent not found")
	ErrMissingParent         = errors.New("storyline tree parent missing")
	ErrCycle                 = errors.New("storyline tree cycle")
	ErrDuplicateStoryline    = errors.New("duplicate storyline in tree")
	ErrChapterRange          = errors.New("storyline chapter range invalid")
	ErrChildOutOfRange       = errors.New("storyline child range outside parent")
	ErrDescendantOutOfRange  = errors.New("storyline descendants outside updated parent range")
	ErrInvalidTypeOrRelation = errors.New("storyline type or relation invalid")
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

type StorylineTreeNode struct {
	PlotLine
	Children []*StorylineTreeNode `json:"children"`
}

type GetTreeResult struct {
	Items []*StorylineTreeNode
}

type CreateRootCommand struct {
	Name, Summary, Status    string
	StartChapter, EndChapter *int
	SortOrder                int
	ActorID                  string
}

type CreateChildCommand = CreateRootCommand

// OptionalInt represents a PATCH field that may be absent, set to an integer, or set to null.
type OptionalInt struct {
	Set   bool
	Value *int
}

type UpdateCommand struct {
	ExpectedVersion int
	Name            *string
	Summary         *string
	Status          *string
	StartChapter    OptionalInt
	EndChapter      OptionalInt
	SortOrder       *int
	ActorID         string
}

type Application interface {
	GetTree(context.Context, uuid.UUID) (GetTreeResult, error)
	CreateRoot(context.Context, uuid.UUID, CreateRootCommand) (PlotLine, error)
	CreateChild(context.Context, uuid.UUID, uuid.UUID, CreateChildCommand) (PlotLine, error)
	Update(context.Context, uuid.UUID, UpdateCommand) (PlotLine, error)
}

type Service struct {
	projects     projectReader
	repository   Repository
	transactions TransactionRunner
}

func NewService(projects projectReader, repository Repository, transactions TransactionRunner) *Service {
	return &Service{projects: projects, repository: repository, transactions: transactions}
}

func NewPostgresService(projects projectReader, pool *pgxpool.Pool) *Service {
	return NewService(projects, NewPostgresRepository(pool), postgresTransactionRunner{pool: pool})
}

func (s *Service) GetTree(ctx context.Context, projectID uuid.UUID) (GetTreeResult, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return GetTreeResult{}, err
	}
	values, err := s.repository.ListByProject(ctx, projectID)
	if err != nil {
		return GetTreeResult{}, err
	}
	return buildTree(values)
}

func buildTree(values []PlotLine) (GetTreeResult, error) {
	nodes := make(map[uuid.UUID]*StorylineTreeNode, len(values))
	for _, value := range values {
		if _, exists := nodes[value.ID]; exists {
			return GetTreeResult{}, ErrDuplicateStoryline
		}
		copy := value
		nodes[value.ID] = &StorylineTreeNode{PlotLine: copy, Children: make([]*StorylineTreeNode, 0)}
	}
	roots := make([]*StorylineTreeNode, 0)
	for _, node := range nodes {
		if node.ParentID == nil {
			roots = append(roots, node)
			continue
		}
		if *node.ParentID == node.ID {
			return GetTreeResult{}, ErrCycle
		}
		parent := nodes[*node.ParentID]
		if parent == nil {
			return GetTreeResult{}, ErrMissingParent
		}
		parent.Children = append(parent.Children, node)
	}
	states := make(map[uuid.UUID]uint8, len(nodes))
	var visit func(*StorylineTreeNode) error
	visit = func(node *StorylineTreeNode) error {
		switch states[node.ID] {
		case 1:
			return ErrCycle
		case 2:
			return nil
		}
		states[node.ID] = 1
		sortNodes(node.Children)
		for _, child := range node.Children {
			if err := visit(child); err != nil {
				return err
			}
		}
		states[node.ID] = 2
		return nil
	}
	sortNodes(roots)
	for _, root := range roots {
		if err := visit(root); err != nil {
			return GetTreeResult{}, err
		}
	}
	remaining := make([]*StorylineTreeNode, 0, len(nodes))
	for _, node := range nodes {
		if states[node.ID] == 0 {
			remaining = append(remaining, node)
		}
	}
	sortNodes(remaining)
	for _, node := range remaining {
		if err := visit(node); err != nil {
			return GetTreeResult{}, err
		}
	}
	return GetTreeResult{Items: roots}, nil
}

func sortNodes(nodes []*StorylineTreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].SortOrder == nodes[j].SortOrder {
			return nodes[i].ID.String() < nodes[j].ID.String()
		}
		return nodes[i].SortOrder < nodes[j].SortOrder
	})
}

func (s *Service) CreateRoot(ctx context.Context, projectID uuid.UUID, command CreateRootCommand) (PlotLine, error) {
	if err := validateCreate(command); err != nil {
		return PlotLine{}, err
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return PlotLine{}, err
	}
	value := PlotLine{ID: uuid.New(), ProjectID: projectID, Type: "main", Relation: "root", Name: command.Name, Summary: command.Summary, StartChapter: command.StartChapter, EndChapter: command.EndChapter, Status: command.Status, SortOrder: command.SortOrder, CreatedBy: command.ActorID}
	return s.createWithAudit(ctx, value, "storyline.created")
}

func (s *Service) CreateChild(ctx context.Context, projectID, parentID uuid.UUID, command CreateChildCommand) (PlotLine, error) {
	if err := validateCreate(command); err != nil {
		return PlotLine{}, err
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return PlotLine{}, err
	}
	value := PlotLine{ID: uuid.New(), ProjectID: projectID, ParentID: &parentID, Type: "child", Relation: "child", Name: command.Name, Summary: command.Summary, StartChapter: command.StartChapter, EndChapter: command.EndChapter, Status: command.Status, SortOrder: command.SortOrder, CreatedBy: command.ActorID}
	var created PlotLine
	err := s.transactions.Run(ctx, func(repository Repository, audits AuditWriter) error {
		parent, err := repository.GetByID(ctx, parentID)
		if errors.Is(err, ErrNotFound) {
			return ErrParentNotFound
		}
		if err != nil {
			return err
		}
		if parent.ProjectID != projectID {
			return ErrProjectMismatch
		}
		if !within(value.StartChapter, value.EndChapter, parent.StartChapter, parent.EndChapter) {
			return ErrChildOutOfRange
		}
		created, err = repository.Create(ctx, value)
		if err != nil {
			return err
		}
		return writeAudit(ctx, audits, command.ActorID, "storyline.created", created.ID, nil, created)
	})
	if err != nil {
		return PlotLine{}, err
	}
	return created, nil
}

func (s *Service) createWithAudit(ctx context.Context, value PlotLine, action string) (PlotLine, error) {
	var created PlotLine
	err := s.transactions.Run(ctx, func(repository Repository, audits AuditWriter) error {
		var err error
		created, err = repository.Create(ctx, value)
		if err != nil {
			return err
		}
		return writeAudit(ctx, audits, value.CreatedBy, action, created.ID, nil, created)
	})
	if err != nil {
		return PlotLine{}, err
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, storylineID uuid.UUID, command UpdateCommand) (PlotLine, error) {
	if command.ExpectedVersion < 1 || !hasUpdate(command) {
		return PlotLine{}, ErrValidation
	}
	var result PlotLine
	err := s.transactions.Run(ctx, func(repository Repository, audits AuditWriter) error {
		current, err := repository.GetByID(ctx, storylineID)
		if err != nil {
			return err
		}
		if current.Version != command.ExpectedVersion {
			return ErrVersionConflict
		}
		next := applyUpdate(current, command)
		if err := validatePlotLine(next); err != nil {
			return err
		}
		all, err := repository.ListByProject(ctx, current.ProjectID)
		if err != nil {
			return err
		}
		if err := validateUpdateBounds(current, next, all); err != nil {
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
		return writeAudit(ctx, audits, command.ActorID, "storyline.updated", result.ID, &current, result)
	})
	if err != nil {
		return PlotLine{}, err
	}
	return result, nil
}

func validateUpdateBounds(current, next PlotLine, values []PlotLine) error {
	if current.ParentID != nil {
		parents := make(map[uuid.UUID]PlotLine, len(values))
		for _, value := range values {
			parents[value.ID] = value
		}
		parent, ok := parents[*current.ParentID]
		if !ok {
			return ErrMissingParent
		}
		if !within(next.StartChapter, next.EndChapter, parent.StartChapter, parent.EndChapter) {
			return ErrChildOutOfRange
		}
	}
	result, err := buildTree(values)
	if err != nil {
		return err
	}
	var find func([]*StorylineTreeNode) *StorylineTreeNode
	find = func(nodes []*StorylineTreeNode) *StorylineTreeNode {
		for _, node := range nodes {
			if node.ID == current.ID {
				return node
			}
			if found := find(node.Children); found != nil {
				return found
			}
		}
		return nil
	}
	node := find(result.Items)
	if node == nil {
		return ErrNotFound
	}
	var descendantsFit func([]*StorylineTreeNode) bool
	descendantsFit = func(nodes []*StorylineTreeNode) bool {
		for _, child := range nodes {
			if !within(child.StartChapter, child.EndChapter, next.StartChapter, next.EndChapter) || !descendantsFit(child.Children) {
				return false
			}
		}
		return true
	}
	if !descendantsFit(node.Children) {
		return ErrDescendantOutOfRange
	}
	return nil
}

func applyUpdate(current PlotLine, command UpdateCommand) PlotLine {
	next := current
	if command.Name != nil {
		next.Name = *command.Name
	}
	if command.Summary != nil {
		next.Summary = *command.Summary
	}
	if command.Status != nil {
		next.Status = *command.Status
	}
	if command.StartChapter.Set {
		next.StartChapter = command.StartChapter.Value
	}
	if command.EndChapter.Set {
		next.EndChapter = command.EndChapter.Value
	}
	if command.SortOrder != nil {
		next.SortOrder = *command.SortOrder
	}
	return next
}

func hasUpdate(command UpdateCommand) bool {
	return command.Name != nil || command.Summary != nil || command.Status != nil || command.StartChapter.Set || command.EndChapter.Set || command.SortOrder != nil
}

func validateCreate(command CreateRootCommand) error {
	return validatePlotLine(PlotLine{Name: command.Name, Summary: command.Summary, Status: command.Status, StartChapter: command.StartChapter, EndChapter: command.EndChapter, SortOrder: command.SortOrder})
}

func validatePlotLine(value PlotLine) error {
	if strings.TrimSpace(value.Name) == "" || len(value.Name) > 120 || len(value.Summary) > 5000 || value.SortOrder < 0 {
		return ErrValidation
	}
	if value.Status != "active" && value.Status != "completed" && value.Status != "archived" {
		return ErrValidation
	}
	if !validRange(value.StartChapter, value.EndChapter) {
		return ErrChapterRange
	}
	return nil
}

func validRange(start, end *int) bool {
	return (start == nil || *start >= 1) && (end == nil || *end >= 1) && (start == nil || end == nil || *start <= *end)
}

func within(start, end, parentStart, parentEnd *int) bool {
	return validRange(start, end) && validRange(parentStart, parentEnd) &&
		(parentStart == nil || start == nil || *start >= *parentStart) &&
		(parentEnd == nil || end == nil || *end <= *parentEnd)
}

func writeAudit(ctx context.Context, writer AuditWriter, actorID, action string, storylineID uuid.UUID, before *PlotLine, after PlotLine) error {
	payload, err := json.Marshal(struct {
		Before *PlotLine `json:"before"`
		After  PlotLine  `json:"after"`
	}{Before: before, After: after})
	if err != nil {
		return fmt.Errorf("marshal storyline audit payload: %w", err)
	}
	return writer.Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: actorID, Action: action, SubjectType: "storyline", SubjectID: storylineID.String(), Payload: payload})
}

type postgresTransactionRunner struct{ pool *pgxpool.Pool }

func (r postgresTransactionRunner) Run(ctx context.Context, fn func(Repository, AuditWriter) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin storyline transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := fn(NewPostgresRepositoryTx(tx), audit.NewRepository(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit storyline transaction: %w", err)
	}
	return nil
}

// CreateChildForParent adapts the parent-scoped HTTP contract to the existing project-scoped use case.
func (s *Service) CreateChildForParent(ctx context.Context, parentID uuid.UUID, command CreateChildCommand) (PlotLine, error) {
	parent, err := s.repository.GetByID(ctx, parentID)
	if errors.Is(err, ErrNotFound) {
		return PlotLine{}, ErrParentNotFound
	}
	if err != nil {
		return PlotLine{}, err
	}
	return s.CreateChild(ctx, parent.ProjectID, parentID, command)
}
