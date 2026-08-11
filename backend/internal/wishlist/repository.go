package wishlist

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrCourseNotFound = errors.New("course not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Add is idempotent (item 2: "Предпочтительно idempotent") — adding a
// course already on the wishlist just returns without error, never a 409.
//
// Bug fix (found during Stage 18 verification): this used to check only
// that the course row existed, not that it was published — a student who
// guessed or otherwise obtained an unpublished/draft course's id could
// successfully wishlist it (confirmed live: POST returned 200
// in_wishlist:true for a freshly created draft course), even though that
// same course is invisible to them everywhere else (public listing,
// recommendations, similar-courses all filter on published = true). Now
// requires published = true and returns the same ErrCourseNotFound either
// way — never distinguishing "doesn't exist" from "exists but
// unpublished" to the caller, consistent with this codebase's existing
// 404-not-403 convention for not confirming the existence of something the
// caller isn't entitled to see.
func (r *Repository) Add(ctx context.Context, userID, courseID uuid.UUID) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1 AND published = true)`, courseID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrCourseNotFound
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO course_wishlist (user_id, course_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, course_id) DO NOTHING
	`, userID, courseID)
	return err
}

func (r *Repository) Remove(ctx context.Context, userID, courseID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM course_wishlist WHERE user_id = $1 AND course_id = $2`, userID, courseID)
	return err
}

// ListForUser is the full GET /me/wishlist payload — one query, no N+1,
// joined the same way courses.courseColumns is (category name, rating
// aggregate via LATERAL) but scoped to this package's narrower Item shape.
func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID) ([]Item, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.title, c.slug, c.image_url, c.access_type, cat.name,
		       COALESCE(rating.avg_rating, 0), COALESCE(rating.review_count, 0),
		       w.created_at
		FROM course_wishlist w
		JOIN courses c ON c.id = w.course_id
		LEFT JOIN categories cat ON cat.id = c.category_id
		LEFT JOIN LATERAL (
			SELECT AVG(cr.rating)::float8 AS avg_rating, COUNT(*) AS review_count
			FROM course_reviews cr
			WHERE cr.course_id = c.id AND cr.published = true
		) rating ON true
		WHERE w.user_id = $1
		ORDER BY w.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.CourseID, &it.Title, &it.Slug, &it.ImageURL, &it.AccessType, &it.CategoryName,
			&it.RatingAverage, &it.RatingCount, &it.AddedAt); err != nil {
			return nil, err
		}
		result = append(result, it)
	}
	return result, rows.Err()
}

// ListCourseIDsForUser backs GET /me/wishlist/course-ids — the "user-aware
// enrichment" endpoint (item 3) the frontend calls separately from the
// public GET /courses, since this codebase has no optional-auth middleware
// to attach in_wishlist to the public listing itself.
func (r *Repository) ListCourseIDsForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT course_id FROM course_wishlist WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

// CountForCourse backs the read-only wishlist_count field on instructor/
// admin course stats (item 24) — never a separate recommendation/wishlist
// management UI.
func (r *Repository) CountForCourse(ctx context.Context, courseID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM course_wishlist WHERE course_id = $1`, courseID).Scan(&count)
	return count, err
}

// RemoveTx is Remove's transaction-capable twin: internal/learning's
// CreateEnrollment calls this directly inside its own transaction so
// "enroll" and "clear the wishlist entry" commit or roll back together
// (item 20).
func RemoveTx(ctx context.Context, tx pgx.Tx, userID, courseID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM course_wishlist WHERE user_id = $1 AND course_id = $2`, userID, courseID)
	return err
}
