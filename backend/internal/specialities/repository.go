package specialities

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrDuplicateSlug    = errors.New("slug already exists")
	ErrCourseNotFound   = errors.New("course not found")
	ErrAlreadyInRoadmap = errors.New("course is already part of this speciality's roadmap")
)

func isUniqueViolation(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	return pgErr, errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListPublished(ctx context.Context) ([]Speciality, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, slug, description, image_url, published, created_at, updated_at
		FROM specialities
		WHERE published = true
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Speciality{}
	for rows.Next() {
		var s Speciality
		if err := rows.Scan(&s.ID, &s.Title, &s.Slug, &s.Description, &s.ImageURL, &s.Published, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetByID intentionally does not filter on published, matching the existing
// courses.GetCourse behavior: direct-by-id lookups are not gated on
// published status, only the catalog listing is.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Speciality, error) {
	var s Speciality
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, slug, description, image_url, published, created_at, updated_at
		FROM specialities
		WHERE id = $1
	`, id).Scan(&s.ID, &s.Title, &s.Slug, &s.Description, &s.ImageURL, &s.Published, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) ListCoursesBySpeciality(ctx context.Context, specialityID uuid.UUID) ([]SpecialityCourse, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.title, c.slug, c.description, c.level, sc.position, sc.required
		FROM speciality_courses sc
		JOIN courses c ON c.id = sc.course_id
		WHERE sc.speciality_id = $1
		ORDER BY sc.position ASC
	`, specialityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SpecialityCourse{}
	for rows.Next() {
		var sc SpecialityCourse
		if err := rows.Scan(&sc.ID, &sc.Title, &sc.Slug, &sc.Description, &sc.Level, &sc.Position, &sc.Required); err != nil {
			return nil, err
		}
		result = append(result, sc)
	}
	return result, rows.Err()
}

// ListCourseProgress computes, for each course in the speciality's roadmap,
// the user's progress based on their existing lesson_progress rows — there
// is no separate speciality-enrollment concept, so a course the user never
// started simply comes back at 0%.
func (r *Repository) ListCourseProgress(ctx context.Context, userID, specialityID uuid.UUID) ([]CourseProgress, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			c.id, c.title, c.slug, sc.position, sc.required,
			COUNT(l.id) FILTER (WHERE l.published) AS total_lessons,
			COUNT(lp.id) FILTER (WHERE l.published AND lp.completed) AS completed_lessons
		FROM speciality_courses sc
		JOIN courses c ON c.id = sc.course_id
		LEFT JOIN modules m ON m.course_id = c.id
		LEFT JOIN lessons l ON l.module_id = m.id
		LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
		WHERE sc.speciality_id = $2
		GROUP BY c.id, c.title, c.slug, sc.position, sc.required
		ORDER BY sc.position ASC
	`, userID, specialityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []CourseProgress{}
	for rows.Next() {
		var cp CourseProgress
		var totalLessons, completedLessons int
		if err := rows.Scan(&cp.CourseID, &cp.Title, &cp.Slug, &cp.Position, &cp.Required, &totalLessons, &completedLessons); err != nil {
			return nil, err
		}
		cp.ProgressPercent = calculatePercent(completedLessons, totalLessons)
		cp.Completed = totalLessons > 0 && completedLessons >= totalLessons
		result = append(result, cp)
	}
	return result, rows.Err()
}

func calculatePercent(completed, total int) int {
	if total <= 0 {
		return 0
	}
	return int(float64(completed) / float64(total) * 100)
}

// ListAllAdmin returns every speciality regardless of published status — no
// pagination, matching the task's scope (only users/courses/certificates
// asked for it) and the small expected number of specialities.
func (r *Repository) ListAllAdmin(ctx context.Context) ([]Speciality, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, slug, description, image_url, published, created_at, updated_at
		FROM specialities
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Speciality{}
	for rows.Next() {
		var s Speciality
		if err := rows.Scan(&s.ID, &s.Title, &s.Slug, &s.Description, &s.ImageURL, &s.Published, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *Repository) CreateSpeciality(ctx context.Context, input SpecialityInput) (*Speciality, error) {
	var s Speciality
	err := r.pool.QueryRow(ctx, `
		INSERT INTO specialities (title, slug, description, image_url, published)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, title, slug, description, image_url, published, created_at, updated_at
	`, input.Title, input.Slug, input.Description, input.ImageURL, input.Published).
		Scan(&s.ID, &s.Title, &s.Slug, &s.Description, &s.ImageURL, &s.Published, &s.CreatedAt, &s.UpdatedAt)
	if _, ok := isUniqueViolation(err); ok {
		return nil, ErrDuplicateSlug
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) UpdateSpeciality(ctx context.Context, id uuid.UUID, input SpecialityInput) (*Speciality, error) {
	var s Speciality
	err := r.pool.QueryRow(ctx, `
		UPDATE specialities
		SET title = $2, slug = $3, description = $4, image_url = $5, published = $6, updated_at = now()
		WHERE id = $1
		RETURNING id, title, slug, description, image_url, published, created_at, updated_at
	`, id, input.Title, input.Slug, input.Description, input.ImageURL, input.Published).
		Scan(&s.ID, &s.Title, &s.Slug, &s.Description, &s.ImageURL, &s.Published, &s.CreatedAt, &s.UpdatedAt)
	if _, ok := isUniqueViolation(err); ok {
		return nil, ErrDuplicateSlug
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteSpeciality is a plain delete with no dependency check: unlike
// courses, a speciality has no directly-attached user data (no
// speciality_enrollments table exists — see the Stage 5 design note in this
// package). Only the speciality_courses junction rows cascade, which is
// just roadmap metadata, not student progress.
func (r *Repository) DeleteSpeciality(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM specialities WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CourseExists(ctx context.Context, courseID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&exists)
	return exists, err
}

func (r *Repository) AddCourseToSpeciality(ctx context.Context, specialityID, courseID uuid.UUID, required bool) (*SpecialityCourse, error) {
	var sc SpecialityCourse
	err := r.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO speciality_courses (speciality_id, course_id, required, position)
			VALUES ($1, $2, $3, (SELECT COALESCE(MAX(position), 0) + 1 FROM speciality_courses WHERE speciality_id = $1))
			RETURNING course_id, position, required
		)
		SELECT c.id, c.title, c.slug, c.description, c.level, inserted.position, inserted.required
		FROM inserted
		JOIN courses c ON c.id = inserted.course_id
	`, specialityID, courseID, required).
		Scan(&sc.ID, &sc.Title, &sc.Slug, &sc.Description, &sc.Level, &sc.Position, &sc.Required)
	if _, ok := isUniqueViolation(err); ok {
		return nil, ErrAlreadyInRoadmap
	}
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (r *Repository) RemoveCourseFromSpeciality(ctx context.Context, specialityID, courseID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM speciality_courses WHERE speciality_id = $1 AND course_id = $2
	`, specialityID, courseID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderSpecialityCourses mirrors the courses domain's module/lesson
// reorder algorithm: shift touched rows to negative positions first so the
// per-row updates in the second pass never transiently collide with
// UNIQUE(speciality_id, position). required is updated in the same pass.
func (r *Repository) ReorderSpecialityCourses(ctx context.Context, specialityID uuid.UUID, items []RoadmapReorderItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	courseIDs := make([]uuid.UUID, len(items))
	for i, it := range items {
		courseIDs[i] = it.CourseID
	}

	if _, err := tx.Exec(ctx, `
		UPDATE speciality_courses SET position = -position WHERE speciality_id = $1 AND course_id = ANY($2)
	`, specialityID, courseIDs); err != nil {
		return err
	}

	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE speciality_courses SET position = $2, required = $3
			WHERE speciality_id = $1 AND course_id = $4
		`, specialityID, it.Position, it.Required, it.CourseID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
