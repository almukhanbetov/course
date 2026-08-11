package reviews

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("resource not found")
	ErrDuplicate     = errors.New("review already exists")
	ErrNotEligible   = errors.New("user is not eligible to review this course")
	ErrCourseMissing = errors.New("course not found")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// CheckEligibility reports whether userID is enrolled in courseID, and (if
// enrolled) whether they qualify to review it: the course is marked
// completed, or they've completed at least one lesson in it. A single query
// avoids a round trip per check.
func (r *Repository) CheckEligibility(ctx context.Context, userID, courseID uuid.UUID) (Eligibility, error) {
	var e Eligibility
	err := r.pool.QueryRow(ctx, `
		SELECT
			ce.id IS NOT NULL AS enrolled,
			ce.id IS NOT NULL AND (
				ce.completed_at IS NOT NULL
				OR EXISTS (
					SELECT 1
					FROM lesson_progress lp
					JOIN lessons l ON l.id = lp.lesson_id
					JOIN modules m ON m.id = l.module_id
					WHERE m.course_id = ce.course_id
					AND lp.user_id = ce.user_id
					AND lp.completed = true
				)
			) AS eligible
		FROM (SELECT $1::uuid AS user_id, $2::uuid AS course_id) target
		LEFT JOIN course_enrollments ce ON ce.user_id = target.user_id AND ce.course_id = target.course_id
	`, userID, courseID).Scan(&e.Enrolled, &e.Eligible)
	return e, err
}

func (r *Repository) Create(ctx context.Context, userID, courseID uuid.UUID, input ReviewInput) (*Review, error) {
	var rv Review
	err := r.pool.QueryRow(ctx, `
		INSERT INTO course_reviews (user_id, course_id, rating, review_text)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, user_id, course_id, rating, review_text, published, created_at, updated_at
	`, userID, courseID, input.Rating, input.ReviewText).Scan(
		&rv.ID, &rv.UserID, &rv.CourseID, &rv.Rating, &rv.ReviewText, &rv.Published, &rv.CreatedAt, &rv.UpdatedAt)
	if isUniqueViolation(err) {
		return nil, ErrDuplicate
	}
	if isForeignKeyViolation(err) {
		return nil, ErrCourseMissing
	}
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

func (r *Repository) GetMine(ctx context.Context, userID, courseID uuid.UUID) (*Review, error) {
	var rv Review
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, rating, review_text, published, created_at, updated_at
		FROM course_reviews
		WHERE user_id = $1 AND course_id = $2
	`, userID, courseID).Scan(
		&rv.ID, &rv.UserID, &rv.CourseID, &rv.Rating, &rv.ReviewText, &rv.Published, &rv.CreatedAt, &rv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

func (r *Repository) Update(ctx context.Context, userID, courseID uuid.UUID, input ReviewInput) (*Review, error) {
	var rv Review
	err := r.pool.QueryRow(ctx, `
		UPDATE course_reviews
		SET rating = $3, review_text = NULLIF($4, ''), updated_at = now()
		WHERE user_id = $1 AND course_id = $2
		RETURNING id, user_id, course_id, rating, review_text, published, created_at, updated_at
	`, userID, courseID, input.Rating, input.ReviewText).Scan(
		&rv.ID, &rv.UserID, &rv.CourseID, &rv.Rating, &rv.ReviewText, &rv.Published, &rv.CreatedAt, &rv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rv, nil
}

func (r *Repository) Delete(ctx context.Context, userID, courseID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM course_reviews WHERE user_id = $1 AND course_id = $2`, userID, courseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPublicByCourse returns published reviews for a course, newest first,
// with the reviewer's display name (never their email).
func (r *Repository) ListPublicByCourse(ctx context.Context, courseID uuid.UUID, limit, offset int) ([]PublicReview, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT cr.id, TRIM(u.first_name || ' ' || u.last_name), cr.rating, cr.review_text, cr.created_at,
			COUNT(*) OVER() AS total
		FROM course_reviews cr
		JOIN users u ON u.id = cr.user_id
		WHERE cr.course_id = $1 AND cr.published = true
		ORDER BY cr.created_at DESC
		LIMIT $2 OFFSET $3
	`, courseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []PublicReview{}
	total := 0
	for rows.Next() {
		var pr PublicReview
		if err := rows.Scan(&pr.ID, &pr.DisplayName, &pr.Rating, &pr.ReviewText, &pr.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, pr)
	}
	return result, total, rows.Err()
}

// ListAdmin returns every review (published or hidden) regardless of
// course, optionally filtered by course and/or rating, for moderation.
func (r *Repository) ListAdmin(ctx context.Context, params AdminListParams) ([]AdminReview, int, error) {
	limit := params.Limit
	offset := (params.Page - 1) * params.Limit

	rows, err := r.pool.Query(ctx, `
		SELECT cr.id, cr.user_id, TRIM(u.first_name || ' ' || u.last_name), cr.course_id, c.title,
			cr.rating, cr.review_text, cr.published, cr.created_at, cr.updated_at,
			COUNT(*) OVER() AS total
		FROM course_reviews cr
		JOIN users u ON u.id = cr.user_id
		JOIN courses c ON c.id = cr.course_id
		WHERE ($1::uuid IS NULL OR cr.course_id = $1)
		AND ($2::int IS NULL OR cr.rating = $2)
		ORDER BY cr.created_at DESC
		LIMIT $3 OFFSET $4
	`, params.CourseID, params.Rating, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []AdminReview{}
	total := 0
	for rows.Next() {
		var ar AdminReview
		if err := rows.Scan(&ar.ID, &ar.UserID, &ar.UserName, &ar.CourseID, &ar.CourseTitle,
			&ar.Rating, &ar.ReviewText, &ar.Published, &ar.CreatedAt, &ar.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, ar)
	}
	return result, total, rows.Err()
}

func (r *Repository) SetPublished(ctx context.Context, id uuid.UUID, published bool) (*AdminReview, error) {
	var ar AdminReview
	err := r.pool.QueryRow(ctx, `
		UPDATE course_reviews cr
		SET published = $2, updated_at = now()
		FROM users u, courses c
		WHERE cr.id = $1 AND u.id = cr.user_id AND c.id = cr.course_id
		RETURNING cr.id, cr.user_id, TRIM(u.first_name || ' ' || u.last_name), cr.course_id, c.title,
			cr.rating, cr.review_text, cr.published, cr.created_at, cr.updated_at
	`, id, published).Scan(&ar.ID, &ar.UserID, &ar.UserName, &ar.CourseID, &ar.CourseTitle,
		&ar.Rating, &ar.ReviewText, &ar.Published, &ar.CreatedAt, &ar.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ar, nil
}
