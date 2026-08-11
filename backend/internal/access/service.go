// Package access is the single, centralized place that decides whether a
// user may access a course's content. Every domain that needs this decision
// (learning, tests) calls Service.CanAccessCourse instead of re-implementing
// the rule, so the policy can't drift between enroll, lesson-progress, and
// test handlers.
//
// It intentionally queries the courses/subscriptions tables directly with
// its own SQL rather than importing the courses or subscriptions packages —
// this codebase's domains never import each other's Go packages (they share
// the SQL schema and duplicate small query helpers instead), and access is a
// small enough shared utility to follow the same rule pagination/authctx do.
package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCourseNotFound = errors.New("course not found")

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CanAccessCourse returns true when userID may access courseID's content
// right now. A free course is always accessible. A subscription course
// requires an active subscription whose expires_at is still in the future —
// evaluated live against now(), never trusting a possibly-stale
// status='active' row (see subscriptions package docs on lazy expiry sync).
func (s *Service) CanAccessCourse(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	var accessType string
	err := s.pool.QueryRow(ctx, `SELECT access_type FROM courses WHERE id = $1`, courseID).Scan(&accessType)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrCourseNotFound
	}
	if err != nil {
		return false, err
	}

	if accessType == "free" {
		return true, nil
	}

	var hasActive bool
	err = s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM subscriptions
			WHERE user_id = $1 AND status = 'active' AND expires_at > now()
		)
	`, userID).Scan(&hasActive)
	if err != nil {
		return false, err
	}
	return hasActive, nil
}
