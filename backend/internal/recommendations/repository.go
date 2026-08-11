package recommendations

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const candidateColumns = `
	c.id, c.title, c.slug, c.image_url, c.access_type,
	c.category_id, cat.name,
	COALESCE(rating.avg_rating, 0), COALESCE(rating.review_count, 0),
	COALESCE(enr.enrollment_count, 0), c.created_at
`

const candidateJoins = `
	LEFT JOIN categories cat ON cat.id = c.category_id
	LEFT JOIN LATERAL (
		SELECT AVG(cr.rating)::float8 AS avg_rating, COUNT(*) AS review_count
		FROM course_reviews cr WHERE cr.course_id = c.id AND cr.published = true
	) rating ON true
	LEFT JOIN LATERAL (
		SELECT COUNT(*) AS enrollment_count FROM course_enrollments WHERE course_id = c.id
	) enr ON true
`

func scanCandidates(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
},
) ([]Candidate, error) {
	result := []Candidate{}
	for rows.Next() {
		var cand Candidate
		if err := rows.Scan(&cand.CourseID, &cand.Title, &cand.Slug, &cand.ImageURL, &cand.AccessType,
			&cand.CategoryID, &cand.CategoryName, &cand.RatingAverage, &cand.RatingCount,
			&cand.EnrollmentCount, &cand.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, cand)
	}
	return result, rows.Err()
}

// ListCandidatesForUser is the personalized candidate set (item 9): published
// courses, excluding any the user is already enrolled in (completed or not
// — "not completed" is a hard exclude, "not already enrolled" is honored
// too since an in-progress course belongs to Continue Learning, not here)
// and excluding courses the user themselves instructs. One query.
func (r *Repository) ListCandidatesForUser(ctx context.Context, userID uuid.UUID) ([]Candidate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+candidateColumns+`
		FROM courses c
		`+candidateJoins+`
		WHERE c.published = true
		  AND (c.instructor_id IS NULL OR c.instructor_id != $1)
		  AND NOT EXISTS (SELECT 1 FROM course_enrollments ce WHERE ce.course_id = c.id AND ce.user_id = $1)
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCandidates(rows)
}

// CategoryAffinity is one row of "how much has this user engaged with this
// category" — TouchedCount counts every enrollment (in progress or done),
// CompletedCount counts only finished ones, so the scorer can weigh a
// finished course in a category more than a merely-started one.
type CategoryAffinity struct {
	CategoryID     uuid.UUID
	TouchedCount   int
	CompletedCount int
}

func (r *Repository) GetCategoryAffinity(ctx context.Context, userID uuid.UUID) ([]CategoryAffinity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.category_id, COUNT(*), COUNT(*) FILTER (WHERE ce.completed_at IS NOT NULL)
		FROM course_enrollments ce
		JOIN courses c ON c.id = ce.course_id
		WHERE ce.user_id = $1 AND c.category_id IS NOT NULL
		GROUP BY c.category_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []CategoryAffinity{}
	for rows.Next() {
		var a CategoryAffinity
		if err := rows.Scan(&a.CategoryID, &a.TouchedCount, &a.CompletedCount); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// GetWishlistCourseIDs duplicates a one-column lookup against
// course_wishlist rather than importing internal/wishlist, per this
// codebase's "share schema, not code" convention between domains.
func (r *Repository) GetWishlistCourseIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
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

// GetLearningPathNextCourses is item 15's "roadmap" signal: for every
// speciality where the user has completed at least one required course,
// the single next required course by position — the strongest single
// factor in the whole scorer. One query across every speciality the user
// has touched, not one query per speciality.
func (r *Repository) GetLearningPathNextCourses(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		WITH progress AS (
			SELECT sc.speciality_id, sc.course_id, sc.position, sc.required,
			       (ce.completed_at IS NOT NULL) AS completed
			FROM speciality_courses sc
			LEFT JOIN course_enrollments ce ON ce.course_id = sc.course_id AND ce.user_id = $1
			JOIN specialities s ON s.id = sc.speciality_id AND s.published = true
		),
		max_completed AS (
			SELECT speciality_id, MAX(position) AS max_pos
			FROM progress
			WHERE completed = true
			GROUP BY speciality_id
		)
		SELECT p.course_id
		FROM progress p
		JOIN max_completed mc ON mc.speciality_id = p.speciality_id
		WHERE p.required = true AND p.position > mc.max_pos
		  AND p.position = (
			SELECT MIN(p2.position) FROM progress p2
			WHERE p2.speciality_id = p.speciality_id AND p2.required = true AND p2.position > mc.max_pos
		  )
	`, userID)
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

// --- similar courses (public, context-based, item 18/19) -------------------

// GetCourseContext loads just enough about one course (category +
// specialities it belongs to) to score its siblings.
type CourseContext struct {
	CategoryID    *uuid.UUID
	SpecialityIDs []uuid.UUID
}

func (r *Repository) GetCourseContext(ctx context.Context, courseID uuid.UUID) (*CourseContext, error) {
	var cc CourseContext
	if err := r.pool.QueryRow(ctx, `SELECT category_id FROM courses WHERE id = $1`, courseID).Scan(&cc.CategoryID); err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `SELECT speciality_id FROM speciality_courses WHERE course_id = $1`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		cc.SpecialityIDs = append(cc.SpecialityIDs, id)
	}
	return &cc, rows.Err()
}

// ListSimilarCandidates is every other published course plus its
// speciality-overlap count against the source course — one query (the
// overlap is computed via a correlated COUNT subquery against a small,
// indexed join table, not a per-candidate round trip).
func (r *Repository) ListSimilarCandidates(ctx context.Context, courseID uuid.UUID) ([]Candidate, []int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+candidateColumns+`,
		       (SELECT COUNT(*) FROM speciality_courses sc1
		        JOIN speciality_courses sc2 ON sc2.speciality_id = sc1.speciality_id
		        WHERE sc1.course_id = c.id AND sc2.course_id = $1) AS speciality_overlap
		FROM courses c
		`+candidateJoins+`
		WHERE c.published = true AND c.id != $1
	`, courseID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	candidates := []Candidate{}
	overlaps := []int{}
	for rows.Next() {
		var cand Candidate
		var overlap int
		if err := rows.Scan(&cand.CourseID, &cand.Title, &cand.Slug, &cand.ImageURL, &cand.AccessType,
			&cand.CategoryID, &cand.CategoryName, &cand.RatingAverage, &cand.RatingCount,
			&cand.EnrollmentCount, &cand.CreatedAt, &overlap); err != nil {
			return nil, nil, err
		}
		candidates = append(candidates, cand)
		overlaps = append(overlaps, overlap)
	}
	return candidates, overlaps, rows.Err()
}
