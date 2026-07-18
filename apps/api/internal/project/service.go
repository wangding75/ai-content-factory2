package project

import (
	"context"
	"github.com/google/uuid"
)

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }
func (s *Service) Create(ctx context.Context, name, projectType, description, actorID string) (Project, error) {
	p, err := New(name, projectType, description)
	if err != nil {
		return Project{}, err
	}
	return s.repository.Create(ctx, p, actorID)
}
func (s *Service) List(ctx context.Context, options ListOptions) ([]Project, int, error) {
	return s.repository.List(ctx, options)
}
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Project, error) {
	return s.repository.Get(ctx, id)
}
func (s *Service) Update(ctx context.Context, id uuid.UUID, name, description *string) (Project, error) {
	if err := ValidateUpdate(name, description); err != nil {
		return Project{}, err
	}
	return s.repository.Update(ctx, id, name, description)
}
func (s *Service) Workspace(ctx context.Context, id uuid.UUID) (Workspace, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return Workspace{}, err
	}
	if reader, ok := s.repository.(ProgressReader); ok {
		progress, err := reader.Progress(ctx, id)
		if err != nil {
			return Workspace{}, err
		}
		return Workspace{Project: p, Progress: progress}, nil
	}
	return Workspace{Project: p}, nil
}
