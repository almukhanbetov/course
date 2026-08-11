package learning

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/achievements"
	"lms-backend/internal/activity"
	"lms-backend/internal/notifications"
	"lms-backend/internal/wishlist"
)

var (
	ErrNotFound       = errors.New("resource not found")
	ErrCourseNotFound = errors.New("course not found")
	ErrLessonNotFound = errors.New("lesson not found")
)

// assignmentApprovedForLesson is Stage 15's completion-rule addition: a
// lesson only "counts" toward course completion if it has no required,
// published assignment, or the given user's submission for it is approved.
// Every query that decides "is this lesson actually done" (below, and the
// identical copy in internal/tests) folds this in alongside the lesson's
// own lesson_progress.completed flag — so a course with no assignments at
// all behaves exactly as it did before Stage 15 (the NOT EXISTS branch is
// always true). Always used against a `lessons` row aliased `l` and a `$1`
// placeholder bound to the user id — every call site here already satisfies
// both.
const assignmentApprovedForLesson = `(
		NOT EXISTS (
			SELECT 1 FROM assignments a
			WHERE a.lesson_id = l.id AND a.published = true AND a.required = true
		)
		OR EXISTS (
			SELECT 1 FROM assignment_submissions asub
			JOIN assignments a2 ON a2.id = asub.assignment_id
			WHERE a2.lesson_id = l.id AND a2.published = true AND a2.required = true
			  AND asub.user_id = $1 AND asub.status = 'approved'
		)
	)`

// codingExerciseOK is Stage 16's completion-rule addition — same shape as
// assignmentApprovedForLesson above, but for a lesson's required, published
// coding exercise: it "counts" only if the user has a passing submit-mode
// submission. Kept byte-for-byte identical to internal/coding's own copy
// (and the copies in internal/tests and internal/assignments).
const codingExerciseOK = `(
		NOT EXISTS (
			SELECT 1 FROM coding_exercises ce
			WHERE ce.lesson_id = l.id AND ce.published = true AND ce.required = true
		)
		OR EXISTS (
			SELECT 1 FROM code_submissions cs
			JOIN coding_exercises ce2 ON ce2.id = cs.exercise_id
			WHERE ce2.lesson_id = l.id AND ce2.published = true AND ce2.required = true
			  AND cs.user_id = $1 AND cs.mode = 'submit' AND cs.status = 'passed'
		)
	)`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CourseExists(ctx context.Context, courseID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&exists)
	return exists, err
}

func scanEnrollment(row pgx.Row) (*Enrollment, error) {
	var e Enrollment
	err := row.Scan(&e.ID, &e.UserID, &e.CourseID, &e.EnrolledAt, &e.CompletedAt, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) GetEnrollment(ctx context.Context, userID, courseID uuid.UUID) (*Enrollment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, course_id, enrolled_at, completed_at, created_at, updated_at
		FROM course_enrollments
		WHERE user_id = $1 AND course_id = $2
	`, userID, courseID)
	return scanEnrollment(row)
}

// CreateEnrollment returns ErrNotFound when the (user_id, course_id) pair
// already exists (ON CONFLICT DO NOTHING yields no returned row) — callers
// should treat that as "already enrolled" rather than a real error. On a
// genuine new enrollment, the "you started a course" notification is
// enqueued in the same transaction — in-app only (item 11: email here would
// just be noise for something the student did themselves a second ago).
func (r *Repository) CreateEnrollment(ctx context.Context, userID, courseID uuid.UUID) (*Enrollment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO course_enrollments (user_id, course_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, course_id) DO NOTHING
		RETURNING id, user_id, course_id, enrolled_at, completed_at, created_at, updated_at
	`, userID, courseID)
	enrollment, err := scanEnrollment(row)
	if err != nil {
		return nil, err
	}

	var courseTitle string
	if err := tx.QueryRow(ctx, `SELECT title FROM courses WHERE id = $1`, courseID).Scan(&courseTitle); err != nil {
		return nil, err
	}

	if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID:    userID,
		Type:      notifications.TypeEnrolled,
		Data:      map[string]any{"course_id": courseID, "course_title": courseTitle},
		DedupeKey: "enrolled:" + userID.String() + ":" + courseID.String(),
		Channels:  []string{notifications.ChannelInApp},
	}); err != nil {
		return nil, err
	}

	// Stage 18 item 20: successful enrollment automatically clears any
	// wishlist entry for the same course, atomically with the enrollment
	// itself — a no-op DELETE if it was never wishlisted.
	if err := wishlist.RemoveTx(ctx, tx, userID, courseID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return enrollment, nil
}

func (r *Repository) ListEnrolledCourses(ctx context.Context, userID uuid.UUID) ([]EnrolledCourse, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			c.id, c.title, c.slug, c.image_url,
			ce.enrolled_at, ce.completed_at,
			COUNT(l.id) FILTER (WHERE l.published) AS total_lessons,
			COUNT(l.id) FILTER (WHERE l.published AND lp.completed AND `+assignmentApprovedForLesson+` AND `+codingExerciseOK+`) AS completed_lessons,
			(
				-- Bug fix (Stage 18 item 6): this used to order by the raw
				-- video-only lesson_progress.completed flag, so a lesson
				-- with an approved assignment/passed coding exercise still
				-- pending would never surface as "next" even though
				-- completed_lessons above (correctly) didn't count it done
				-- either — the two could disagree. Now uses the exact same
				-- three-factor formula as completed_lessons. Reuses aliases
				-- l/m (shadowing the outer query's own l/m within this
				-- subquery's local scope) purely so assignmentApprovedForLesson/
				-- codingExerciseOK's hardcoded "l.id" resolve to the lesson
				-- being ranked here, not the outer query's.
				SELECT l.id
				FROM lessons l
				JOIN modules m ON m.id = l.module_id
				LEFT JOIN lesson_progress lp2 ON lp2.lesson_id = l.id AND lp2.user_id = ce.user_id
				WHERE m.course_id = c.id AND l.published
				ORDER BY (COALESCE(lp2.completed, false) AND `+assignmentApprovedForLesson+` AND `+codingExerciseOK+`) ASC,
				         m.position ASC, l.position ASC
				LIMIT 1
			) AS next_lesson_id,
			(
				SELECT t.id FROM tests t
				WHERE t.course_id = c.id AND t.published = true AND t.is_final = true
				LIMIT 1
			) AS final_test_id,
			EXISTS (
				SELECT 1 FROM test_attempts ta
				JOIN tests t ON t.id = ta.test_id
				WHERE t.course_id = c.id AND t.published = true AND t.is_final = true
				  AND ta.user_id = ce.user_id AND ta.passed = true
			) AS final_test_passed,
			(
				SELECT MAX(ta.score) FROM test_attempts ta
				JOIN tests t ON t.id = ta.test_id
				WHERE t.course_id = c.id AND t.published = true AND t.is_final = true
				  AND ta.user_id = ce.user_id
			) AS final_test_best_score
		FROM course_enrollments ce
		JOIN courses c ON c.id = ce.course_id
		LEFT JOIN modules m ON m.course_id = c.id
		LEFT JOIN lessons l ON l.module_id = m.id
		LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = ce.user_id
		WHERE ce.user_id = $1
		GROUP BY c.id, c.title, c.slug, c.image_url, ce.enrolled_at, ce.completed_at, ce.user_id
		ORDER BY ce.enrolled_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []EnrolledCourse{}
	for rows.Next() {
		var ec EnrolledCourse
		if err := rows.Scan(&ec.CourseID, &ec.Title, &ec.Slug, &ec.ImageURL, &ec.EnrolledAt, &ec.CompletedAt,
			&ec.TotalLessons, &ec.CompletedLessons, &ec.NextLessonID, &ec.FinalTestID, &ec.FinalTestPassed,
			&ec.FinalTestBestScore); err != nil {
			return nil, err
		}
		ec.ProgressPercent = calculatePercent(ec.CompletedLessons, ec.TotalLessons)
		ec.LessonsProgressPercent = ec.ProgressPercent
		ec.HasFinalTest = ec.FinalTestID != nil
		ec.Completed = ec.CompletedAt != nil
		result = append(result, ec)
	}
	return result, rows.Err()
}

// GetContinueLearning backs GET /me/continue-learning (Stage 18 item 5) —
// enrolled, not-yet-completed courses only, one query (a `candidates` CTE
// for progress/last-activity aggregates, a `next_lessons` CTE using the
// exact same window-function-free DISTINCT ON trick as the next_lesson_id
// fix above), no N+1 regardless of how many courses the user has in
// flight. Sorted by most recent activity first, then by progress — a
// course touched five minutes ago but barely started still outranks one
// untouched for a month even if further along.
func (r *Repository) GetContinueLearning(ctx context.Context, userID uuid.UUID) ([]ContinueLearningItem, error) {
	rows, err := r.pool.Query(ctx, `
		WITH candidates AS (
			SELECT
				c.id AS course_id, c.title, c.image_url,
				COUNT(l.id) FILTER (WHERE l.published) AS total_lessons,
				COUNT(l.id) FILTER (WHERE l.published AND lp.completed AND `+assignmentApprovedForLesson+` AND `+codingExerciseOK+`) AS completed_lessons,
				MAX(lp.updated_at) AS last_activity_at
			FROM course_enrollments ce
			JOIN courses c ON c.id = ce.course_id
			LEFT JOIN modules m ON m.course_id = c.id
			LEFT JOIN lessons l ON l.module_id = m.id
			LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = ce.user_id
			WHERE ce.user_id = $1 AND ce.completed_at IS NULL
			GROUP BY c.id, c.title, c.image_url
		),
		next_lessons AS (
			SELECT DISTINCT ON (m.course_id)
				m.course_id, l.id AS lesson_id, l.title AS lesson_title
			FROM lessons l
			JOIN modules m ON m.id = l.module_id
			LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
			WHERE l.published = true
			  AND m.course_id IN (SELECT course_id FROM candidates)
			  AND NOT (COALESCE(lp.completed, false) AND `+assignmentApprovedForLesson+` AND `+codingExerciseOK+`)
			ORDER BY m.course_id, m.position ASC, l.position ASC
		)
		SELECT cd.course_id, cd.title, cd.image_url, cd.total_lessons, cd.completed_lessons, cd.last_activity_at,
		       nl.lesson_id, nl.lesson_title
		FROM candidates cd
		LEFT JOIN next_lessons nl ON nl.course_id = cd.course_id
		ORDER BY cd.last_activity_at DESC NULLS LAST,
		         CASE WHEN cd.total_lessons > 0 THEN cd.completed_lessons::float / cd.total_lessons ELSE 0 END DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ContinueLearningItem{}
	for rows.Next() {
		var item ContinueLearningItem
		var totalLessons, completedLessons int
		if err := rows.Scan(&item.CourseID, &item.Title, &item.ImageURL, &totalLessons, &completedLessons,
			&item.LastActivityAt, &item.NextLessonID, &item.NextLessonTitle); err != nil {
			return nil, err
		}
		item.ProgressPercent = calculatePercent(completedLessons, totalLessons)
		result = append(result, item)
	}
	return result, rows.Err()
}

type enrolledCourseInfo struct {
	EnrolledAt      time.Time
	CompletedAt     *time.Time
	Title           string
	Slug            string
	Description     string
	Level           string
	ImageURL        string
	FinalTestID     *uuid.UUID
	FinalTestPassed bool
}

// GetEnrolledCourseInfo returns ErrNotFound if the user is not enrolled in
// the course (or the course does not exist).
func (r *Repository) GetEnrolledCourseInfo(ctx context.Context, userID, courseID uuid.UUID) (*enrolledCourseInfo, error) {
	var info enrolledCourseInfo
	err := r.pool.QueryRow(ctx, `
		SELECT
			ce.enrolled_at, ce.completed_at, c.title, c.slug, c.description, c.level, c.image_url,
			(
				SELECT t.id FROM tests t
				WHERE t.course_id = c.id AND t.published = true AND t.is_final = true
				LIMIT 1
			) AS final_test_id,
			EXISTS (
				SELECT 1 FROM test_attempts ta
				JOIN tests t ON t.id = ta.test_id
				WHERE t.course_id = c.id AND t.published = true AND t.is_final = true
				  AND ta.user_id = ce.user_id AND ta.passed = true
			) AS final_test_passed
		FROM course_enrollments ce
		JOIN courses c ON c.id = ce.course_id
		WHERE ce.user_id = $1 AND ce.course_id = $2
	`, userID, courseID).Scan(&info.EnrolledAt, &info.CompletedAt, &info.Title, &info.Slug, &info.Description, &info.Level, &info.ImageURL,
		&info.FinalTestID, &info.FinalTestPassed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (r *Repository) ListModulesByCourse(ctx context.Context, courseID uuid.UUID) ([]MyModule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, position
		FROM modules
		WHERE course_id = $1
		ORDER BY position ASC
	`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []MyModule{}
	for rows.Next() {
		var m MyModule
		if err := rows.Scan(&m.ID, &m.Title, &m.Position); err != nil {
			return nil, err
		}
		m.Lessons = []MyLesson{}
		result = append(result, m)
	}
	return result, rows.Err()
}

type moduleLessonRow struct {
	ModuleID uuid.UUID
	Lesson   MyLesson
}

func (r *Repository) ListLessonsWithProgressByModules(ctx context.Context, userID uuid.UUID, moduleIDs []uuid.UUID) ([]moduleLessonRow, error) {
	if len(moduleIDs) == 0 {
		return []moduleLessonRow{}, nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT l.module_id, l.id, l.title, l.slug, l.description, l.video_url, l.duration_seconds, l.position, l.is_free, l.published,
		       COALESCE(lp.progress_seconds, 0), COALESCE(lp.completed, false), lv.status,
		       a.id IS NOT NULL, asub.status,
		       ce.id IS NOT NULL, latest_sub.status,
		       EXISTS(
		           SELECT 1 FROM code_submissions cs
		           WHERE cs.exercise_id = ce.id AND cs.user_id = $1 AND cs.mode = 'submit' AND cs.status = 'passed'
		       )
		FROM lessons l
		LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
		LEFT JOIN lesson_videos lv ON lv.lesson_id = l.id
		LEFT JOIN assignments a ON a.lesson_id = l.id AND a.published = true AND a.required = true
		LEFT JOIN assignment_submissions asub ON asub.assignment_id = a.id AND asub.user_id = $1
		LEFT JOIN coding_exercises ce ON ce.lesson_id = l.id AND ce.published = true AND ce.required = true
		LEFT JOIN LATERAL (
			SELECT status FROM code_submissions
			WHERE exercise_id = ce.id AND user_id = $1 AND mode = 'submit'
			ORDER BY created_at DESC LIMIT 1
		) latest_sub ON true
		WHERE l.module_id = ANY($2)
		ORDER BY l.module_id, l.position ASC
	`, userID, moduleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []moduleLessonRow{}
	for rows.Next() {
		var row moduleLessonRow
		if err := rows.Scan(&row.ModuleID, &row.Lesson.ID, &row.Lesson.Title, &row.Lesson.Slug, &row.Lesson.Description,
			&row.Lesson.VideoURL, &row.Lesson.DurationSeconds, &row.Lesson.Position, &row.Lesson.IsFree, &row.Lesson.Published,
			&row.Lesson.ProgressSeconds, &row.Lesson.VideoCompleted, &row.Lesson.VideoStatus,
			&row.Lesson.AssignmentRequired, &row.Lesson.AssignmentStatus,
			&row.Lesson.CodingExerciseRequired, &row.Lesson.CodingExerciseStatus, &row.Lesson.CodingExercisePassed); err != nil {
			return nil, err
		}
		row.Lesson.AssignmentApproved = row.Lesson.AssignmentStatus != nil && *row.Lesson.AssignmentStatus == "approved"
		row.Lesson.Completed = row.Lesson.VideoCompleted &&
			(!row.Lesson.AssignmentRequired || row.Lesson.AssignmentApproved) &&
			(!row.Lesson.CodingExerciseRequired || row.Lesson.CodingExercisePassed)
		result = append(result, row)
	}
	return result, rows.Err()
}

type lessonContext struct {
	CourseID        uuid.UUID
	Published       bool
	DurationSeconds int
}

func (r *Repository) GetLessonContext(ctx context.Context, lessonID uuid.UUID) (*lessonContext, error) {
	var lc lessonContext
	err := r.pool.QueryRow(ctx, `
		SELECT m.course_id, l.published, l.duration_seconds
		FROM lessons l
		JOIN modules m ON m.id = l.module_id
		WHERE l.id = $1
	`, lessonID).Scan(&lc.CourseID, &lc.Published, &lc.DurationSeconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLessonNotFound
	}
	if err != nil {
		return nil, err
	}
	return &lc, nil
}

func scanLessonProgress(row pgx.Row) (*LessonProgress, error) {
	var lp LessonProgress
	err := row.Scan(&lp.ID, &lp.UserID, &lp.LessonID, &lp.ProgressSeconds, &lp.Completed, &lp.CompletedAt, &lp.CreatedAt, &lp.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &lp, nil
}

func (r *Repository) GetLessonProgress(ctx context.Context, userID, lessonID uuid.UUID) (*LessonProgress, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, lesson_id, progress_seconds, completed, completed_at, created_at, updated_at
		FROM lesson_progress
		WHERE user_id = $1 AND lesson_id = $2
	`, userID, lessonID)
	return scanLessonProgress(row)
}

// UpsertLessonProgress writes the lesson progress row and, in the same
// transaction, recomputes whether the owning course should be considered
// completed for this user.
func (r *Repository) UpsertLessonProgress(ctx context.Context, userID, lessonID, courseID uuid.UUID, progressSeconds int, completed bool) (*LessonProgress, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// The "old" CTE captures the pre-update completed flag so we can tell a
	// genuine incomplete->complete transition (record lesson_completed,
	// Stage 17) apart from an idempotent re-save of an already-completed
	// lesson (e.g. the student rewatches or re-saves progress) — mirrors
	// the same NULL/old-value-snapshot trick recalculateCourseCompletionSQL
	// already uses for course-level completion below.
	row := tx.QueryRow(ctx, `
		WITH old AS (
			SELECT completed FROM lesson_progress WHERE user_id = $1 AND lesson_id = $2
		)
		INSERT INTO lesson_progress (user_id, lesson_id, progress_seconds, completed, completed_at)
		VALUES ($1, $2, $3, $4, CASE WHEN $4 THEN now() ELSE NULL END)
		ON CONFLICT (user_id, lesson_id) DO UPDATE
		SET progress_seconds = EXCLUDED.progress_seconds,
		    completed = EXCLUDED.completed,
		    completed_at = CASE
		        WHEN EXCLUDED.completed THEN COALESCE(lesson_progress.completed_at, now())
		        ELSE NULL
		    END,
		    updated_at = now()
		RETURNING id, user_id, lesson_id, progress_seconds, completed, completed_at, created_at, updated_at,
		          (SELECT completed FROM old)
	`, userID, lessonID, progressSeconds, completed)

	var wasCompletedBefore *bool
	var progress LessonProgress
	err = row.Scan(&progress.ID, &progress.UserID, &progress.LessonID, &progress.ProgressSeconds, &progress.Completed,
		&progress.CompletedAt, &progress.CreatedAt, &progress.UpdatedAt, &wasCompletedBefore)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	lessonJustCompleted := progress.Completed && (wasCompletedBefore == nil || !*wasCompletedBefore)
	if lessonJustCompleted {
		var lessonTitle string
		if err := tx.QueryRow(ctx, `SELECT title FROM lessons WHERE id = $1`, lessonID).Scan(&lessonTitle); err != nil {
			return nil, err
		}
		if err := activity.Record(ctx, tx, activity.RecordInput{
			UserID: userID, Type: activity.TypeLessonCompleted,
			EntityType: "lesson", EntityID: &lessonID,
			Metadata:  map[string]any{"title": lessonTitle},
			DedupeKey: "lesson_completed:" + userID.String() + ":" + lessonID.String(),
		}); err != nil {
			return nil, err
		}
		if err := achievements.Evaluate(ctx, tx, userID); err != nil {
			return nil, err
		}
	}

	justCompleted, err := recalculateCourseCompletion(ctx, tx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if justCompleted {
		if err := onCourseJustCompleted(ctx, tx, userID, courseID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &progress, nil
}

// RecalculateCourseCompletion re-evaluates completed_at for a single
// enrollment. Exported so other domains (e.g. a passed/failed final test
// attempt) can trigger the same recomputation without duplicating the rule.
// It opens its own transaction (unlike UpsertLessonProgress, which already
// has one) so the completion flip and its notification are still one atomic
// outcome regardless of which caller triggered the recalculation.
func (r *Repository) RecalculateCourseCompletion(ctx context.Context, userID, courseID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	justCompleted, err := recalculateCourseCompletion(ctx, tx, userID, courseID)
	if err != nil {
		return err
	}
	if justCompleted {
		if err := onCourseJustCompleted(ctx, tx, userID, courseID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// recalculateCourseCompletionSQL marks an enrollment completed once every
// published lesson is done — a lesson counting as "done" only when its
// video/progress requirement is met AND (Stage 15) any required, published
// assignment on it is approved, see assignmentApprovedForLesson — AND, when
// the course has a published, is_final test, the user has at least one
// passed attempt at it. Courses without a final test keep the original
// lessons-only rule (the "NOT EXISTS" branch); lessons without an
// assignment behave exactly as before (assignmentApprovedForLesson's own
// NOT EXISTS branch is always true for them). Losing any condition (e.g.
// un-completing a lesson) clears completed_at again, matching the existing
// "completion can regress" behavior.
//
// The leading "old" CTE snapshots completed_at *before* this UPDATE touches
// it, and RETURNING hands back both values — the only way to tell "just now
// became complete" apart from "was already complete" or "flipped back to
// incomplete", which nothing in this query could distinguish before.
//
// Kept byte-for-byte identical to internal/tests's copy of this constant —
// see that package's comment for why it's duplicated rather than shared via
// import.
const recalculateCourseCompletionSQL = `
	WITH old AS (
		SELECT completed_at FROM course_enrollments WHERE user_id = $1 AND course_id = $2
	),
	lesson_stats AS (
		SELECT
			COUNT(l.id) FILTER (WHERE l.published) AS total,
			COUNT(l.id) FILTER (WHERE l.published AND lp.completed AND ` + assignmentApprovedForLesson + ` AND ` + codingExerciseOK + `) AS done
		FROM modules m
		LEFT JOIN lessons l ON l.module_id = m.id
		LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
		WHERE m.course_id = $2
	),
	final_test AS (
		SELECT id FROM tests WHERE course_id = $2 AND published = true AND is_final = true LIMIT 1
	),
	final_test_status AS (
		SELECT EXISTS (
			SELECT 1 FROM test_attempts ta
			WHERE ta.test_id = (SELECT id FROM final_test) AND ta.user_id = $1 AND ta.passed = true
		) AS passed
	)
	UPDATE course_enrollments
	SET completed_at = CASE
	        WHEN (SELECT total FROM lesson_stats) > 0
	             AND (SELECT done FROM lesson_stats) >= (SELECT total FROM lesson_stats)
	             AND (NOT EXISTS (SELECT 1 FROM final_test) OR (SELECT passed FROM final_test_status))
	        THEN COALESCE(completed_at, now())
	        ELSE NULL
	    END,
	    updated_at = now()
	WHERE user_id = $1 AND course_id = $2
	RETURNING completed_at, (SELECT completed_at FROM old)
`

func recalculateCourseCompletion(ctx context.Context, tx pgx.Tx, userID, courseID uuid.UUID) (justCompleted bool, err error) {
	var newCompleted, oldCompleted *time.Time
	err = tx.QueryRow(ctx, recalculateCourseCompletionSQL, userID, courseID).Scan(&newCompleted, &oldCompleted)
	if err != nil {
		return false, err
	}
	return oldCompleted == nil && newCompleted != nil, nil
}

func enqueueCourseCompletedNotification(ctx context.Context, tx pgx.Tx, userID, courseID uuid.UUID) error {
	var courseTitle string
	if err := tx.QueryRow(ctx, `SELECT title FROM courses WHERE id = $1`, courseID).Scan(&courseTitle); err != nil {
		return err
	}
	return notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID:    userID,
		Type:      notifications.TypeCourseCompleted,
		Data:      map[string]any{"course_id": courseID, "course_title": courseTitle},
		DedupeKey: "course_completed:" + userID.String() + ":" + courseID.String(),
		Channels:  []string{notifications.ChannelInApp, notifications.ChannelEmail},
	})
}

// onCourseJustCompleted is Stage 17's addition, called alongside the
// existing course-completed notification at every recalculateCourseCompletion
// call site: it records the course_completed activity and re-evaluates
// achievements (FIRST_COURSE) in the same transaction as the completion
// flip itself, so activity/achievement/notification all commit atomically
// with the enrollment update.
func onCourseJustCompleted(ctx context.Context, tx pgx.Tx, userID, courseID uuid.UUID) error {
	if err := enqueueCourseCompletedNotification(ctx, tx, userID, courseID); err != nil {
		return err
	}
	var courseTitle string
	if err := tx.QueryRow(ctx, `SELECT title FROM courses WHERE id = $1`, courseID).Scan(&courseTitle); err != nil {
		return err
	}
	if err := activity.Record(ctx, tx, activity.RecordInput{
		UserID: userID, Type: activity.TypeCourseCompleted,
		EntityType: "course", EntityID: &courseID,
		Metadata:  map[string]any{"title": courseTitle},
		DedupeKey: "course_completed_activity:" + userID.String() + ":" + courseID.String(),
	}); err != nil {
		return err
	}
	return achievements.Evaluate(ctx, tx, userID)
}

func calculatePercent(completed, total int) int {
	if total <= 0 {
		return 0
	}
	return int(float64(completed) / float64(total) * 100)
}
