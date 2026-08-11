package activity

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (r *Repository) GetUserTimezone(ctx context.Context, userID uuid.UUID) (string, error) {
	return GetUserTimezone(ctx, r.pool, userID)
}

func (s *Service) GetStats(ctx context.Context, userID uuid.UUID) (*Stats, error) {
	counts, err := s.repo.GetCounts(ctx, userID)
	if err != nil {
		return nil, err
	}

	tz, err := s.repo.GetUserTimezone(ctx, userID)
	if err != nil {
		return nil, err
	}

	dates, err := s.repo.GetActiveLocalDates(ctx, userID, tz)
	if err != nil {
		return nil, err
	}
	today, err := LocalToday(tz)
	if err != nil {
		return nil, err
	}
	current, longest := ComputeStreaks(dates, today)

	return &Stats{Counts: counts, CurrentStreak: current, LongestStreak: longest}, nil
}

const maxCalendarRangeDays = 400

func (s *Service) GetCalendar(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]DayCount, error) {
	tz, err := s.repo.GetUserTimezone(ctx, userID)
	if err != nil {
		return nil, err
	}
	if to.Before(from) {
		from, to = to, from
	}
	if to.Sub(from) > maxCalendarRangeDays*24*time.Hour {
		to = from.AddDate(0, 0, maxCalendarRangeDays)
	}
	return s.repo.GetCalendar(ctx, userID, tz, from, to)
}

func (s *Service) ListRecent(ctx context.Context, userID uuid.UUID, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.ListRecent(ctx, userID, limit)
}
