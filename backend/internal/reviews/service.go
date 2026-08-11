package reviews

import (
	"context"

	"github.com/google/uuid"

	"lms-backend/internal/pagination"
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

func validateInput(input ReviewInput) error {
	if input.Rating < 1 || input.Rating > 5 {
		return &ValidationError{Message: "rating must be between 1 and 5"}
	}
	return nil
}

func (s *Service) ListPublicByCourse(ctx context.Context, courseID uuid.UUID, page, limit int) (pagination.Result[PublicReview], error) {
	items, total, err := s.repo.ListPublicByCourse(ctx, courseID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[PublicReview]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

// CheckEligibility exposes the repo's eligibility check so the handler can
// decide 403 vs proceeding, and so the frontend can know whether to show a
// review form at all.
func (s *Service) CheckEligibility(ctx context.Context, userID, courseID uuid.UUID) (Eligibility, error) {
	return s.repo.CheckEligibility(ctx, userID, courseID)
}

func (s *Service) Create(ctx context.Context, userID, courseID uuid.UUID, input ReviewInput) (*Review, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}
	elig, err := s.repo.CheckEligibility(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if !elig.Eligible {
		return nil, ErrNotEligible
	}
	return s.repo.Create(ctx, userID, courseID, input)
}

func (s *Service) GetMine(ctx context.Context, userID, courseID uuid.UUID) (*Review, error) {
	return s.repo.GetMine(ctx, userID, courseID)
}

func (s *Service) Update(ctx context.Context, userID, courseID uuid.UUID, input ReviewInput) (*Review, error) {
	if err := validateInput(input); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, userID, courseID, input)
}

func (s *Service) Delete(ctx context.Context, userID, courseID uuid.UUID) error {
	return s.repo.Delete(ctx, userID, courseID)
}

func (s *Service) ListAdmin(ctx context.Context, params AdminListParams) (pagination.Result[AdminReview], error) {
	items, total, err := s.repo.ListAdmin(ctx, params)
	if err != nil {
		return pagination.Result[AdminReview]{}, err
	}
	return pagination.New(items, params.Page, params.Limit, total), nil
}

func (s *Service) SetPublished(ctx context.Context, id uuid.UUID, published bool) (*AdminReview, error) {
	return s.repo.SetPublished(ctx, id, published)
}
