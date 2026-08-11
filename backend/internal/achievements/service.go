package achievements

import (
	"context"

	"github.com/google/uuid"

	"lms-backend/internal/activity"
	"lms-backend/internal/notifications"
)

// Evaluate runs every rule for one user and awards whichever newly match —
// call it right after internal/activity.Record for a meaningful event, in
// the SAME transaction (db here is typically a pgx.Tx already in scope from
// the caller's own state change), so the activity row, any newly-earned
// achievement, and its notification all commit or roll back together as
// one atomic outcome. Never called from anywhere in this package — every
// call site is one of internal/learning, internal/assignments,
// internal/coding, internal/tests, internal/certificates, or
// cmd/backfill-achievements.
func Evaluate(ctx context.Context, db activity.Execer, userID uuid.UUID) error {
	achievementsByCode, err := ListActiveByCode(ctx, db)
	if err != nil {
		return err
	}
	earned, err := ListEarnedAchievementIDs(ctx, db, userID)
	if err != nil {
		return err
	}

	counts, err := activity.GetCounts(ctx, db, userID)
	if err != nil {
		return err
	}
	tz, err := activity.GetUserTimezone(ctx, db, userID)
	if err != nil {
		return err
	}
	dates, err := activity.GetActiveLocalDates(ctx, db, userID, tz)
	if err != nil {
		return err
	}
	today, err := activity.LocalToday(tz)
	if err != nil {
		return err
	}
	currentStreak, _ := activity.ComputeStreaks(dates, today)

	for _, rule := range Rules {
		a, ok := achievementsByCode[rule.Code]
		if !ok || earned[a.ID] {
			continue
		}
		if !rule.Check(counts, currentStreak) {
			continue
		}

		earnedNow, err := InsertIfNew(ctx, db, userID, a.ID)
		if err != nil {
			return err
		}
		if !earnedNow {
			continue // another concurrent evaluation already awarded it
		}

		if err := notifications.Enqueue(ctx, db, notifications.EnqueueInput{
			UserID: userID,
			Type:   notifications.TypeAchievementEarned,
			Data:   map[string]any{"achievement_title": a.Title, "achievement_code": a.Code},
			// Every award is a brand-new user_achievements row, and
			// InsertIfNew already guarantees this branch only runs once per
			// (user, achievement) ever — a plain per-achievement dedupe key
			// is enough (no timestamp needed, unlike a resubmit-able event).
			DedupeKey: "achievement_earned:" + userID.String() + ":" + a.Code,
			Channels:  []string{notifications.ChannelInApp},
		}); err != nil {
			return err
		}
	}
	return nil
}

// --- HTTP-facing read service ---------------------------------------------

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetView(ctx context.Context, userID uuid.UUID) (*AchievementsView, error) {
	all, err := s.repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	earnedRows, err := s.repo.ListEarned(ctx, userID)
	if err != nil {
		return nil, err
	}

	earnedCodes := make(map[string]bool, len(earnedRows))
	for _, e := range earnedRows {
		earnedCodes[e.Code] = true
	}

	view := &AchievementsView{Earned: earnedRows, Locked: []LockedAchievement{}}
	for _, a := range all {
		if earnedCodes[a.Code] {
			continue
		}
		view.Locked = append(view.Locked, LockedAchievement{Code: a.Code, Title: a.Title, Description: a.Description, Icon: a.Icon})
	}
	return view, nil
}
