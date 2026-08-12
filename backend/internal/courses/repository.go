package courses

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/notifications"
	"lms-backend/internal/pagination"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrDuplicateSlug    = errors.New("slug already exists")
	ErrHasEnrollments   = errors.New("course has active enrollments")
	ErrCategoryNotFound = errors.New("category not found")
)

// courseColumns/courseJoins are shared by every read that needs the full
// Course shape (category name/slug, rating aggregate, instructor name,
// publication state) — SearchCourses, ListAllAdmin, GetCourse,
// ListByInstructor. RETURNING clauses on INSERT/UPDATE deliberately don't
// use these (RETURNING can't join), so they stick to plain base columns.
const courseColumns = `
	courses.id, courses.title, courses.slug, courses.description, courses.level,
	courses.image_url, courses.published, courses.access_type,
	courses.created_at, courses.updated_at,
	courses.category_id, cat.name, cat.slug,
	COALESCE(rating.avg_rating, 0) AS rating_avg, COALESCE(rating.review_count, 0) AS rating_count,
	courses.instructor_id, TRIM(instr.first_name || ' ' || instr.last_name),
	courses.publication_status, courses.rejection_reason
`

const courseJoins = `
	LEFT JOIN categories cat ON cat.id = courses.category_id
	LEFT JOIN users instr ON instr.id = courses.instructor_id
	LEFT JOIN LATERAL (
		SELECT AVG(cr.rating)::float8 AS avg_rating, COUNT(*) AS review_count
		FROM course_reviews cr
		WHERE cr.course_id = courses.id AND cr.published = true
	) rating ON true
`

// scanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query, one
// row at a time). extra holds additional trailing SELECT columns a
// particular query needs beyond the fixed Course shape (e.g. search_rank,
// the COUNT(*) OVER() total).
type scanner interface {
	Scan(dest ...any) error
}

func scanCourseFull(row scanner, c *Course, extra ...any) error {
	dest := []any{&c.ID, &c.Title, &c.Slug, &c.Description, &c.Level, &c.ImageURL, &c.Published, &c.AccessType,
		&c.CreatedAt, &c.UpdatedAt, &c.CategoryID, &c.CategoryName, &c.CategorySlug,
		&c.RatingAverage, &c.RatingCount, &c.InstructorID, &c.InstructorName,
		&c.PublicationStatus, &c.RejectionReason}
	dest = append(dest, extra...)
	return row.Scan(dest...)
}

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

// courseSortClause maps a pre-validated sort name (see service.allowedSorts)
// to a fixed ORDER BY clause. It never sees raw request input — the service
// rejects anything not in the whitelist before this is called — so there is
// no path from user-controlled text into ORDER BY.
func courseSortClause(sort string) string {
	switch sort {
	case "rating":
		return "rating_avg DESC NULLS LAST, rating_count DESC, courses.created_at DESC"
	case "title":
		return "courses.title ASC"
	case "relevance":
		return "search_rank DESC, courses.created_at DESC"
	default: // "newest"
		return "courses.created_at DESC"
	}
}

// splitSearchQuery implements the shared "3+ characters use
// websearch_to_tsquery against the GIN-indexed search_vector column
// (title weighted above description); 1-2 characters fall back to a plain
// ILIKE on title" convention SearchCourses established — very short input
// rarely tokenizes into a usable tsquery. Reused as-is by SuggestCourses so
// both entry points rank/match the same query text identically. At
// real-world catalog scale this ILIKE fallback should become a pg_trgm
// trigram index instead; the current demo catalog is small enough that the
// resulting sequential scan is inconsequential (see both callers' own
// EXPLAIN ANALYZE verification).
func splitSearchQuery(q string) (tsQuery *string, ilikeQuery string) {
	if q == "" {
		return nil, ""
	}
	if len([]rune(q)) >= 3 {
		return &q, ""
	}
	return nil, q
}

// SearchCourses is the public catalog — published courses only, filtered by
// category/level/access_type, searched via PostgreSQL full-text search
// (falling back to ILIKE for queries too short to tokenize meaningfully),
// and ranked/sorted per a whitelisted sort mode. Rating is aggregated via a
// LATERAL join so this stays a single query regardless of page size (no
// N+1 per course).
func (r *Repository) SearchCourses(ctx context.Context, params ListCoursesParams) ([]Course, int, error) {
	tsQuery, ilikeQuery := splitSearchQuery(params.Query)

	rows, err := r.pool.Query(ctx, `
		SELECT `+courseColumns+`,
			CASE WHEN $4::text IS NOT NULL
				THEN ts_rank(courses.search_vector, websearch_to_tsquery('russian', $4))
				ELSE 0
			END AS search_rank,
			COUNT(*) OVER() AS total
		FROM courses
		`+courseJoins+`
		WHERE courses.published = true
			AND ($1 = '' OR cat.slug = $1)
			AND ($2 = '' OR courses.level = $2)
			AND ($3 = '' OR courses.access_type = $3)
			AND (
				($4::text IS NULL AND $5 = '')
				OR ($4::text IS NOT NULL AND courses.search_vector @@ websearch_to_tsquery('russian', $4))
				OR ($4::text IS NULL AND $5 <> '' AND courses.title ILIKE '%' || $5 || '%')
			)
		ORDER BY `+courseSortClause(params.Sort)+`
		LIMIT $6 OFFSET $7
	`, params.CategorySlug, params.Level, params.AccessType, tsQuery, ilikeQuery,
		params.Limit, pagination.Offset(params.Page, params.Limit))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []Course{}
	total := 0
	for rows.Next() {
		var c Course
		var searchRank float64
		if err := scanCourseFull(rows, &c, &searchRank, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

// SuggestCourses backs search-as-you-type (Stage 22A1): the same
// published-only, tsquery/ILIKE-fallback matching and ts_rank ordering
// SearchCourses uses (via the shared splitSearchQuery), but against the
// narrow CourseSuggestion shape — no LATERAL rating join, no
// COUNT(*) OVER() total — and hard-capped by limit rather than paginated,
// since a per-keystroke query has nothing to page through. Query
// normalization/bounding (trimming, empty checks, the limit value itself)
// is the service's job, matching SearchCourses' own service/repository
// split.
func (r *Repository) SuggestCourses(ctx context.Context, query string, limit int) ([]CourseSuggestion, error) {
	tsQuery, ilikeQuery := splitSearchQuery(query)

	rows, err := r.pool.Query(ctx, `
		SELECT courses.id, courses.title, courses.slug, cat.name
		FROM courses
		LEFT JOIN categories cat ON cat.id = courses.category_id
		WHERE courses.published = true
			AND (
				($1::text IS NOT NULL AND courses.search_vector @@ websearch_to_tsquery('russian', $1))
				OR ($1::text IS NULL AND $2 <> '' AND courses.title ILIKE '%' || $2 || '%')
			)
		ORDER BY
			CASE WHEN $1::text IS NOT NULL
				THEN ts_rank(courses.search_vector, websearch_to_tsquery('russian', $1))
				ELSE 0
			END DESC,
			courses.title ASC
		LIMIT $3
	`, tsQuery, ilikeQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []CourseSuggestion{}
	for rows.Next() {
		var s CourseSuggestion
		if err := rows.Scan(&s.ID, &s.Title, &s.Slug, &s.CategoryName); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ListAllAdmin returns every course regardless of published status, paged
// and optionally filtered by a case-insensitive match on title/slug.
func (r *Repository) ListAllAdmin(ctx context.Context, search string, limit, offset int) ([]Course, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+courseColumns+`,
			COUNT(*) OVER() AS total
		FROM courses
		`+courseJoins+`
		WHERE $1 = '' OR courses.title ILIKE '%'||$1||'%' OR courses.slug ILIKE '%'||$1||'%'
		ORDER BY courses.created_at DESC
		LIMIT $2 OFFSET $3
	`, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []Course{}
	total := 0
	for rows.Next() {
		var c Course
		if err := scanCourseFull(rows, &c, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

// ListByInstructor is the instructor's own catalog — every course they own
// regardless of publication status, paginated newest-first.
func (r *Repository) ListByInstructor(ctx context.Context, instructorID uuid.UUID, limit, offset int) ([]Course, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+courseColumns+`,
			COUNT(*) OVER() AS total
		FROM courses
		`+courseJoins+`
		WHERE courses.instructor_id = $1
		ORDER BY courses.created_at DESC
		LIMIT $2 OFFSET $3
	`, instructorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []Course{}
	total := 0
	for rows.Next() {
		var c Course
		if err := scanCourseFull(rows, &c, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

// ListPendingReview is the admin moderation queue — courses instructors have
// submitted, oldest first (first submitted, first reviewed).
func (r *Repository) ListPendingReview(ctx context.Context, limit, offset int) ([]Course, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+courseColumns+`,
			COUNT(*) OVER() AS total
		FROM courses
		`+courseJoins+`
		WHERE courses.publication_status = 'pending_review'
		ORDER BY courses.updated_at ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []Course{}
	total := 0
	for rows.Next() {
		var c Course
		if err := scanCourseFull(rows, &c, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

func (r *Repository) GetCourse(ctx context.Context, id uuid.UUID) (*Course, error) {
	var c Course
	row := r.pool.QueryRow(ctx, `
		SELECT `+courseColumns+`
		FROM courses
		`+courseJoins+`
		WHERE courses.id = $1
	`, id)
	if err := scanCourseFull(row, &c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListModulesByCourse(ctx context.Context, courseID uuid.UUID) ([]Module, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, course_id, title, position, created_at, updated_at
		FROM modules
		WHERE course_id = $1
		ORDER BY position ASC
	`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Module{}
	for rows.Next() {
		var m Module
		if err := rows.Scan(&m.ID, &m.CourseID, &m.Title, &m.Position, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Lessons = []Lesson{}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *Repository) ListLessonsByModules(ctx context.Context, moduleIDs []uuid.UUID) ([]Lesson, error) {
	if len(moduleIDs) == 0 {
		return []Lesson{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, module_id, title, slug, description, video_url, duration_seconds, position, is_free, published, created_at, updated_at
		FROM lessons
		WHERE module_id = ANY($1)
		ORDER BY module_id, position ASC
	`, moduleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Lesson{}
	for rows.Next() {
		var l Lesson
		if err := rows.Scan(&l.ID, &l.ModuleID, &l.Title, &l.Slug, &l.Description, &l.VideoURL, &l.DurationSeconds, &l.Position, &l.IsFree, &l.Published, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

// baseCourseColumns/scanCourseBase back every INSERT/UPDATE RETURNING clause
// below — RETURNING can't join, so these stay limited to the courses table's
// own columns (no category name/rating/instructor name).
const baseCourseColumns = `id, title, slug, description, level, image_url, published, access_type,
	created_at, updated_at, category_id, instructor_id, publication_status, rejection_reason`

func scanCourseBase(row scanner, c *Course) error {
	return row.Scan(&c.ID, &c.Title, &c.Slug, &c.Description, &c.Level, &c.ImageURL, &c.Published, &c.AccessType,
		&c.CreatedAt, &c.UpdatedAt, &c.CategoryID, &c.InstructorID, &c.PublicationStatus, &c.RejectionReason)
}

// publicationStatusFor keeps publication_status in lockstep with the legacy
// published boolean for admin-authored writes — see migration 00032's
// comment on why the boolean was kept instead of being replaced outright.
func publicationStatusFor(published bool) string {
	if published {
		return "published"
	}
	return "draft"
}

// CreateCourse is the admin path — admin is fully trusted, so instructor_id
// and published come directly from input, and publication_status is
// derived from published (see publicationStatusFor).
func (r *Repository) CreateCourse(ctx context.Context, input CourseInput) (*Course, error) {
	var c Course
	row := r.pool.QueryRow(ctx, `
		INSERT INTO courses (title, slug, description, level, image_url, published, access_type, category_id, instructor_id, publication_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+baseCourseColumns,
		input.Title, input.Slug, input.Description, input.Level, input.ImageURL, input.Published, input.AccessType,
		input.CategoryID, input.InstructorID, publicationStatusFor(input.Published))
	err := scanCourseBase(row, &c)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	if isForeignKeyViolation(err) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) UpdateCourse(ctx context.Context, id uuid.UUID, input CourseInput) (*Course, error) {
	var c Course
	row := r.pool.QueryRow(ctx, `
		UPDATE courses
		SET title = $2, slug = $3, description = $4, level = $5, image_url = $6, published = $7, access_type = $8,
			category_id = $9, instructor_id = $10, publication_status = $11, updated_at = now()
		WHERE id = $1
		RETURNING `+baseCourseColumns,
		id, input.Title, input.Slug, input.Description, input.Level, input.ImageURL, input.Published, input.AccessType,
		input.CategoryID, input.InstructorID, publicationStatusFor(input.Published))
	err := scanCourseBase(row, &c)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	if isForeignKeyViolation(err) {
		return nil, ErrCategoryNotFound
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateForInstructor always starts a course as an unpublished draft owned
// by instructorID — publication_status/published/instructor_id are never
// derived from caller input (see InstructorCourseInput, which has no field
// for any of them).
func (r *Repository) CreateForInstructor(ctx context.Context, instructorID uuid.UUID, input InstructorCourseInput) (*Course, error) {
	var c Course
	row := r.pool.QueryRow(ctx, `
		INSERT INTO courses (title, slug, description, level, image_url, access_type, category_id, instructor_id, published, publication_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false, 'draft')
		RETURNING `+baseCourseColumns,
		input.Title, input.Slug, input.Description, input.Level, input.ImageURL, input.AccessType, input.CategoryID, instructorID)
	err := scanCourseBase(row, &c)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	if isForeignKeyViolation(err) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateForInstructor is scoped by instructor_id in the WHERE clause as a
// storage-level line of defense against IDOR, in addition to the
// ownership.Service check the handler already performs — even a bypassed
// pre-check still can't touch a course this instructor doesn't own. It never
// touches published/publication_status: editing content doesn't change
// where a course sits in the review workflow.
func (r *Repository) UpdateForInstructor(ctx context.Context, id, instructorID uuid.UUID, input InstructorCourseInput) (*Course, error) {
	var c Course
	row := r.pool.QueryRow(ctx, `
		UPDATE courses
		SET title = $3, slug = $4, description = $5, level = $6, image_url = $7, access_type = $8, category_id = $9, updated_at = now()
		WHERE id = $1 AND instructor_id = $2
		RETURNING `+baseCourseColumns,
		id, instructorID, input.Title, input.Slug, input.Description, input.Level, input.ImageURL, input.AccessType, input.CategoryID)
	err := scanCourseBase(row, &c)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	if isForeignKeyViolation(err) {
		return nil, ErrCategoryNotFound
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SubmitForReview transitions draft/rejected -> pending_review, clearing any
// prior rejection reason. Scoped by instructor_id for the same IDOR-defense
// reason as UpdateForInstructor. Returns ErrNotFound if the course doesn't
// exist, isn't owned by instructorID, or isn't currently draft/rejected.
func (r *Repository) SubmitForReview(ctx context.Context, id, instructorID uuid.UUID) (*Course, error) {
	var c Course
	row := r.pool.QueryRow(ctx, `
		UPDATE courses
		SET publication_status = 'pending_review', rejection_reason = NULL, updated_at = now()
		WHERE id = $1 AND instructor_id = $2 AND publication_status IN ('draft', 'rejected')
		RETURNING `+baseCourseColumns,
		id, instructorID)
	err := scanCourseBase(row, &c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SetPublicationStatus is the admin moderation transition: pending_review ->
// published or rejected. published is kept in sync (true only when status
// becomes "published") so every existing `WHERE published = true` query
// keeps working unchanged. Returns ErrNotFound if the course doesn't exist
// or isn't currently pending_review.
//
// The notification to the instructor is enqueued in the same transaction as
// the status change — either both happen or neither does, same pattern
// certificates/subscriptions use for their own triggers.
func (r *Repository) SetPublicationStatus(ctx context.Context, id uuid.UUID, status string, rejectionReason *string) (*Course, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var c Course
	row := tx.QueryRow(ctx, `
		UPDATE courses
		SET publication_status = $2, published = ($2 = 'published'), rejection_reason = $3, updated_at = now()
		WHERE id = $1 AND publication_status = 'pending_review'
		RETURNING `+baseCourseColumns,
		id, status, rejectionReason)
	if err := scanCourseBase(row, &c); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if c.InstructorID != nil {
		notifType := notifications.TypeCourseApproved
		data := map[string]any{"course_title": c.Title}
		if status == "rejected" {
			notifType = notifications.TypeCourseRejected
			reason := ""
			if rejectionReason != nil {
				reason = *rejectionReason
			}
			data["rejection_reason"] = reason
		}
		if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
			UserID:   *c.InstructorID,
			Type:     notifType,
			Data:     data,
			Channels: []string{notifications.ChannelInApp},
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}

// CountEnrollments queries the learning domain's table directly (same
// cross-domain-via-SQL convention used throughout this codebase) so course
// deletion can refuse to silently wipe out real student progress.
func (r *Repository) CountEnrollments(ctx context.Context, courseID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM course_enrollments WHERE course_id = $1`, courseID).Scan(&count)
	return count, err
}

func (r *Repository) DeleteCourse(ctx context.Context, id uuid.UUID) error {
	count, err := r.CountEnrollments(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrHasEnrollments
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM courses WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetModule(ctx context.Context, id uuid.UUID) (*Module, error) {
	var m Module
	err := r.pool.QueryRow(ctx, `
		SELECT id, course_id, title, position, created_at, updated_at
		FROM modules WHERE id = $1
	`, id).Scan(&m.ID, &m.CourseID, &m.Title, &m.Position, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateModule assumes the caller (service layer) already verified the
// course exists — position is auto-appended, so the only failure mode left
// is a generic database error.
func (r *Repository) CreateModule(ctx context.Context, courseID uuid.UUID, input ModuleInput) (*Module, error) {
	var m Module
	err := r.pool.QueryRow(ctx, `
		INSERT INTO modules (course_id, title, position)
		VALUES ($1, $2, (SELECT COALESCE(MAX(position), 0) + 1 FROM modules WHERE course_id = $1))
		RETURNING id, course_id, title, position, created_at, updated_at
	`, courseID, input.Title).Scan(&m.ID, &m.CourseID, &m.Title, &m.Position, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) UpdateModule(ctx context.Context, id uuid.UUID, input ModuleInput) (*Module, error) {
	var m Module
	err := r.pool.QueryRow(ctx, `
		UPDATE modules SET title = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, course_id, title, position, created_at, updated_at
	`, id, input.Title).Scan(&m.ID, &m.CourseID, &m.Title, &m.Position, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *Repository) DeleteModule(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM modules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderModules requires the full set of a course's module ids (validated
// by the service before this is called). It shifts the touched rows to
// negative positions first so the second pass can assign the new positions
// one at a time without transiently colliding with the UNIQUE(course_id,
// position) constraint — no need for a deferrable constraint migration.
func (r *Repository) ReorderModules(ctx context.Context, courseID uuid.UUID, items []PositionItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}

	if _, err := tx.Exec(ctx, `UPDATE modules SET position = -position WHERE course_id = $1 AND id = ANY($2)`, courseID, ids); err != nil {
		return err
	}

	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE modules SET position = $2, updated_at = now() WHERE id = $1 AND course_id = $3
		`, it.ID, it.Position, courseID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetLesson(ctx context.Context, id uuid.UUID) (*Lesson, error) {
	var l Lesson
	err := r.pool.QueryRow(ctx, `
		SELECT id, module_id, title, slug, description, video_url, duration_seconds, position, is_free, published, created_at, updated_at
		FROM lessons WHERE id = $1
	`, id).Scan(&l.ID, &l.ModuleID, &l.Title, &l.Slug, &l.Description, &l.VideoURL, &l.DurationSeconds, &l.Position, &l.IsFree, &l.Published, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *Repository) CreateLesson(ctx context.Context, moduleID uuid.UUID, input LessonInput) (*Lesson, error) {
	var l Lesson
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lessons (module_id, title, slug, description, video_url, duration_seconds, is_free, published, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT COALESCE(MAX(position), 0) + 1 FROM lessons WHERE module_id = $1))
		RETURNING id, module_id, title, slug, description, video_url, duration_seconds, position, is_free, published, created_at, updated_at
	`, moduleID, input.Title, input.Slug, input.Description, input.VideoURL, input.DurationSeconds, input.IsFree, input.Published).
		Scan(&l.ID, &l.ModuleID, &l.Title, &l.Slug, &l.Description, &l.VideoURL, &l.DurationSeconds, &l.Position, &l.IsFree, &l.Published, &l.CreatedAt, &l.UpdatedAt)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *Repository) UpdateLesson(ctx context.Context, id uuid.UUID, input LessonInput) (*Lesson, error) {
	var l Lesson
	err := r.pool.QueryRow(ctx, `
		UPDATE lessons
		SET title = $2, slug = $3, description = $4, video_url = $5, duration_seconds = $6, is_free = $7, published = $8, updated_at = now()
		WHERE id = $1
		RETURNING id, module_id, title, slug, description, video_url, duration_seconds, position, is_free, published, created_at, updated_at
	`, id, input.Title, input.Slug, input.Description, input.VideoURL, input.DurationSeconds, input.IsFree, input.Published).
		Scan(&l.ID, &l.ModuleID, &l.Title, &l.Slug, &l.Description, &l.VideoURL, &l.DurationSeconds, &l.Position, &l.IsFree, &l.Published, &l.CreatedAt, &l.UpdatedAt)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateSlug
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *Repository) DeleteLesson(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderLessons mirrors ReorderModules exactly, scoped to a module.
func (r *Repository) ReorderLessons(ctx context.Context, moduleID uuid.UUID, items []PositionItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}

	if _, err := tx.Exec(ctx, `UPDATE lessons SET position = -position WHERE module_id = $1 AND id = ANY($2)`, moduleID, ids); err != nil {
		return err
	}

	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			UPDATE lessons SET position = $2, updated_at = now() WHERE id = $1 AND module_id = $3
		`, it.ID, it.Position, moduleID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
