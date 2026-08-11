// Package admin holds cross-domain concerns that don't belong to any single
// existing domain — right now, just the dashboard stats aggregate. It reads
// other domains' tables directly (the same convention used throughout this
// codebase) rather than importing their Go packages.
package admin

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/activity"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetStats(ctx context.Context) (*Stats, error) {
	var s Stats
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM courses),
			(SELECT COUNT(*) FROM courses WHERE published = true),
			(SELECT COUNT(*) FROM specialities),
			(SELECT COUNT(*) FROM course_enrollments),
			(SELECT COUNT(*) FROM course_enrollments WHERE completed_at IS NOT NULL),
			(SELECT COUNT(*) FROM certificates),
			(SELECT COUNT(*) FROM test_attempts),
			(SELECT COUNT(DISTINCT user_id) FROM learning_activity
				WHERE activity_type = ANY($1) AND occurred_at::date = CURRENT_DATE),
			(SELECT COUNT(DISTINCT user_id) FROM learning_activity
				WHERE activity_type = ANY($1) AND occurred_at >= now() - interval '7 days'),
			(SELECT COUNT(*) FROM learning_activity
				WHERE activity_type = 'lesson_completed' AND occurred_at::date = CURRENT_DATE),
			(SELECT COUNT(*) FROM learning_activity
				WHERE activity_type = 'course_completed' AND occurred_at >= date_trunc('month', now())),
			(SELECT COUNT(*) FROM certificates WHERE issued_at >= date_trunc('month', now()))
	`, activity.MeaningfulTypes).Scan(&s.UsersCount, &s.CoursesCount, &s.PublishedCoursesCount, &s.SpecialitiesCount,
		&s.EnrollmentsCount, &s.CompletedCoursesCount, &s.CertificatesCount, &s.TestAttemptsCount,
		&s.DailyActiveLearners, &s.WeeklyActiveLearners, &s.LessonsCompletedToday,
		&s.CoursesCompletedThisMonth, &s.CertificatesThisMonth)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
