package wishlist

import (
	"context"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Add(ctx context.Context, userID, courseID uuid.UUID) error {
	return s.repo.Add(ctx, userID, courseID)
}

func (s *Service) Remove(ctx context.Context, userID, courseID uuid.UUID) error {
	return s.repo.Remove(ctx, userID, courseID)
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]Item, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *Service) ListCourseIDsForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.ListCourseIDsForUser(ctx, userID)
}
