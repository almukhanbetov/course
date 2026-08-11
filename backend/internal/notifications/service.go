package notifications

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

func (s *Service) ListMyNotifications(ctx context.Context, userID uuid.UUID, page, limit int) (pagination.Result[Notification], error) {
	items, total, err := s.repo.ListMyNotifications(ctx, userID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[Notification]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.repo.UnreadCount(ctx, userID)
}

// MarkRead's only authorization check is the repository's WHERE user_id =
// $2 clause — a caller can never mark someone else's notification read
// because that row simply won't match and RowsAffected will be zero.
func (s *Service) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	return s.repo.MarkRead(ctx, userID, notificationID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}

// --- admin -----------------------------------------------------------

func (s *Service) ListJobsAdmin(ctx context.Context, status, channel string, page, limit int) (pagination.Result[AdminJobSummary], error) {
	items, total, err := s.repo.ListJobsAdmin(ctx, status, channel, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[AdminJobSummary]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

func (s *Service) RetryJob(ctx context.Context, jobID uuid.UUID) error {
	return s.repo.RetryJob(ctx, jobID)
}

// AnnounceCourse validates the course is actually published (announcing an
// unpublished course would be misleading — students would follow a link
// that 404s) before fanning out.
func (s *Service) AnnounceCourse(ctx context.Context, courseID uuid.UUID) (int64, error) {
	title, err := s.repo.GetPublishedCourseTitle(ctx, courseID)
	if err != nil {
		return 0, err
	}
	return s.repo.AnnounceCourse(ctx, courseID, title)
}
