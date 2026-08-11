package achievements

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/activity"
)

// ListActiveByCode loads every active achievement keyed by code — used by
// Evaluate to resolve a Rule's Code to its row id, and by the handler to
// build the locked/earned view. A free function (accepts activity.Execer)
// so Evaluate can call it against an in-flight tx.
func ListActiveByCode(ctx context.Context, db activity.Execer) (map[string]Achievement, error) {
	rows, err := db.Query(ctx, `SELECT id, code, title, description, icon, active, created_at FROM achievements WHERE active = true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]Achievement)
	for rows.Next() {
		var a Achievement
		if err := rows.Scan(&a.ID, &a.Code, &a.Title, &a.Description, &a.Icon, &a.Active, &a.CreatedAt); err != nil {
			return nil, err
		}
		result[a.Code] = a
	}
	return result, rows.Err()
}

// ListEarnedAchievementIDs returns the set of achievement ids this user
// already has, so Evaluate never re-checks (let alone re-inserts) one it's
// already awarded.
func ListEarnedAchievementIDs(ctx context.Context, db activity.Execer, userID uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := db.Query(ctx, `SELECT achievement_id FROM user_achievements WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]bool)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

// InsertIfNew is the idempotency boundary (item 12: "Идемпотентность
// обеспечивается unique constraint") — ON CONFLICT DO NOTHING means a
// duplicate Evaluate call (e.g. a retried transaction) can never award the
// same achievement twice, and the RETURNING clause is how the caller knows
// whether THIS call was the one that actually earned it (vs. already having
// it), which is what decides whether to send a notification.
func InsertIfNew(ctx context.Context, db activity.Execer, userID, achievementID uuid.UUID) (earnedNow bool, err error) {
	row := db.QueryRow(ctx, `
		INSERT INTO user_achievements (user_id, achievement_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, achievement_id) DO NOTHING
		RETURNING id
	`, userID, achievementID)
	var id uuid.UUID
	err = row.Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// --- pool-backed reads for the HTTP handler --------------------------------

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListActive(ctx context.Context) (map[string]Achievement, error) {
	return ListActiveByCode(ctx, r.pool)
}

func (r *Repository) ListEarned(ctx context.Context, userID uuid.UUID) ([]EarnedAchievement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.code, a.title, a.description, a.icon, ua.earned_at
		FROM user_achievements ua
		JOIN achievements a ON a.id = ua.achievement_id
		WHERE ua.user_id = $1
		ORDER BY ua.earned_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []EarnedAchievement{}
	for rows.Next() {
		var e EarnedAchievement
		if err := rows.Scan(&e.Code, &e.Title, &e.Description, &e.Icon, &e.EarnedAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
