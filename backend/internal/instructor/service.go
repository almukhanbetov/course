package instructor

import (
	"context"

	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListStudents(ctx context.Context, instructorID uuid.UUID, page, limit int) (pagination.Result[StudentRow], error) {
	items, total, err := s.repo.ListStudents(ctx, instructorID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[StudentRow]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

func (s *Service) ListCourseStudents(ctx context.Context, courseID uuid.UUID, page, limit int) (pagination.Result[CourseStudentRow], error) {
	items, total, err := s.repo.ListCourseStudents(ctx, courseID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[CourseStudentRow]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

func (s *Service) Stats(ctx context.Context, instructorID uuid.UUID) (Stats, error) {
	return s.repo.Stats(ctx, instructorID)
}

func (s *Service) CourseStats(ctx context.Context, courseID uuid.UUID) (CourseStats, error) {
	return s.repo.CourseStats(ctx, courseID)
}

func (s *Service) ListCourseReviews(ctx context.Context, courseID uuid.UUID, page, limit int) (pagination.Result[CourseReview], error) {
	items, total, err := s.repo.ListCourseReviews(ctx, courseID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[CourseReview]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

func (s *Service) ListCourseTests(ctx context.Context, courseID uuid.UUID) ([]TestSummary, error) {
	return s.repo.ListCourseTests(ctx, courseID)
}
