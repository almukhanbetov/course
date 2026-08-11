package categories

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListActive(ctx context.Context) ([]Category, error) {
	return s.repo.ListActive(ctx)
}

func (s *Service) ListAllAdmin(ctx context.Context) ([]Category, error) {
	return s.repo.ListAllAdmin(ctx)
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*Category, error) {
	return s.repo.GetBySlug(ctx, slug)
}

func (s *Service) Create(ctx context.Context, input CategoryInput) (*Category, error) {
	if err := validate(input); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, input)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input CategoryInput) (*Category, error) {
	if err := validate(input); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, input)
}

func validate(input CategoryInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return &ValidationError{Message: "name is required"}
	}
	if strings.TrimSpace(input.Slug) == "" {
		return &ValidationError{Message: "slug is required"}
	}
	return nil
}
