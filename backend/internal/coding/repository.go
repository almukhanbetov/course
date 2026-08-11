package coding

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/achievements"
	"lms-backend/internal/activity"
	"lms-backend/internal/notifications"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrDuplicateLesson  = errors.New("this lesson already has a coding exercise")
	ErrHasSubmissions   = errors.New("coding exercise has student submissions")
	ErrLessonNotFound   = errors.New("lesson not found")
	ErrPositionConflict = errors.New("a test case with this position already exists")
	ErrNoJobAvailable   = errors.New("no execution job available")
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

// codingExerciseOK and recalculateCourseCompletionSQL are kept byte-for-
// byte identical to the copies in internal/learning, internal/tests and
// internal/assignments (see those packages for the full rationale) — this
// package never imports them, so the completion rule is duplicated here for
// the one place a coding submission can flip a lesson from "not done" to
// "done" (a passing submit-mode submission).
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

func recalculateCourseCompletion(ctx context.Context, tx pgx.Tx, userID, courseID uuid.UUID) error {
	var newCompleted, oldCompleted *time.Time
	if err := tx.QueryRow(ctx, recalculateCourseCompletionSQL, userID, courseID).Scan(&newCompleted, &oldCompleted); err != nil {
		return err
	}
	if oldCompleted != nil || newCompleted == nil {
		return nil
	}
	var courseTitle string
	if err := tx.QueryRow(ctx, `SELECT title FROM courses WHERE id = $1`, courseID).Scan(&courseTitle); err != nil {
		return err
	}
	if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID:    userID,
		Type:      notifications.TypeCourseCompleted,
		Data:      map[string]any{"course_id": courseID, "course_title": courseTitle},
		DedupeKey: "course_completed:" + userID.String() + ":" + courseID.String(),
		Channels:  []string{notifications.ChannelInApp, notifications.ChannelEmail},
	}); err != nil {
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

// --- coding_exercises ------------------------------------------------

const exerciseColumns = `id, lesson_id, title, description, language, starter_code, solution_code, time_limit_ms, memory_limit_mb, published, required, created_at, updated_at`

func scanExercise(row pgx.Row) (*Exercise, error) {
	var e Exercise
	err := row.Scan(&e.ID, &e.LessonID, &e.Title, &e.Description, &e.Language, &e.StarterCode, &e.SolutionCode,
		&e.TimeLimitMS, &e.MemoryLimitMB, &e.Published, &e.Required, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) CreateExercise(ctx context.Context, lessonID uuid.UUID, input ExerciseInput) (*Exercise, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO coding_exercises (lesson_id, title, description, language, starter_code, solution_code, time_limit_ms, memory_limit_mb, published, required)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+exerciseColumns,
		lessonID, input.Title, input.Description, input.Language, input.StarterCode, input.SolutionCode,
		input.TimeLimitMS, input.MemoryLimitMB, input.Published, input.Required)
	e, err := scanExercise(row)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateLesson
	}
	return e, err
}

func (r *Repository) GetExerciseByLesson(ctx context.Context, lessonID uuid.UUID) (*Exercise, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+exerciseColumns+` FROM coding_exercises WHERE lesson_id = $1`, lessonID)
	return scanExercise(row)
}

// GetPublishedExerciseByLesson is the student-facing lookup — an
// unpublished (draft) exercise is treated as not existing, same convention
// assignments.Repository.GetPublishedAssignmentByLesson uses.
func (r *Repository) GetPublishedExerciseByLesson(ctx context.Context, lessonID uuid.UUID) (*Exercise, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+exerciseColumns+` FROM coding_exercises WHERE lesson_id = $1 AND published = true`, lessonID)
	return scanExercise(row)
}

func (r *Repository) GetExercise(ctx context.Context, id uuid.UUID) (*Exercise, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+exerciseColumns+` FROM coding_exercises WHERE id = $1`, id)
	return scanExercise(row)
}

func (r *Repository) UpdateExercise(ctx context.Context, id uuid.UUID, input ExerciseInput) (*Exercise, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE coding_exercises
		SET title = $2, description = $3, language = $4, starter_code = $5, solution_code = $6,
		    time_limit_ms = $7, memory_limit_mb = $8, published = $9, required = $10, updated_at = now()
		WHERE id = $1
		RETURNING `+exerciseColumns,
		id, input.Title, input.Description, input.Language, input.StarterCode, input.SolutionCode,
		input.TimeLimitMS, input.MemoryLimitMB, input.Published, input.Required)
	return scanExercise(row)
}

func (r *Repository) CountSubmissionsForExercise(ctx context.Context, exerciseID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM code_submissions WHERE exercise_id = $1`, exerciseID).Scan(&count)
	return count, err
}

func (r *Repository) DeleteExercise(ctx context.Context, id uuid.UUID) error {
	count, err := r.CountSubmissionsForExercise(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrHasSubmissions
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM coding_exercises WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LessonContext mirrors assignments.Repository.LessonContext — duplicated
// per this codebase's "share schema, not code" convention between domains.
type LessonContext struct {
	CourseID  uuid.UUID
	Published bool
}

func (r *Repository) GetLessonContext(ctx context.Context, lessonID uuid.UUID) (*LessonContext, error) {
	var lc LessonContext
	err := r.pool.QueryRow(ctx, `
		SELECT m.course_id, l.published
		FROM lessons l
		JOIN modules m ON m.id = l.module_id
		WHERE l.id = $1
	`, lessonID).Scan(&lc.CourseID, &lc.Published)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLessonNotFound
	}
	if err != nil {
		return nil, err
	}
	return &lc, nil
}

// IsEnrolled mirrors assignments.Repository.IsEnrolled — duplicated per
// convention (coding never imports learning/tests/assignments).
func (r *Repository) IsEnrolled(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM course_enrollments WHERE user_id = $1 AND course_id = $2)
	`, userID, courseID).Scan(&exists)
	return exists, err
}

// --- coding_test_cases -------------------------------------------------

const testCaseColumns = `id, coding_exercise_id, input, expected_output, position, hidden, created_at`

func scanTestCase(row pgx.Row) (*TestCase, error) {
	var tc TestCase
	err := row.Scan(&tc.ID, &tc.CodingExerciseID, &tc.Input, &tc.ExpectedOutput, &tc.Position, &tc.Hidden, &tc.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tc, nil
}

func (r *Repository) CreateTestCase(ctx context.Context, exerciseID uuid.UUID, input TestCaseInput) (*TestCase, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO coding_test_cases (coding_exercise_id, input, expected_output, position, hidden)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+testCaseColumns,
		exerciseID, input.Input, input.ExpectedOutput, input.Position, input.Hidden)
	tc, err := scanTestCase(row)
	if isUniqueViolation(err) {
		return nil, ErrPositionConflict
	}
	return tc, err
}

// ListTestCases is the instructor-facing listing — every case, hidden or
// not, with its full expected_output. Never exposed to a student caller.
func (r *Repository) ListTestCases(ctx context.Context, exerciseID uuid.UUID) ([]TestCase, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+testCaseColumns+` FROM coding_test_cases WHERE coding_exercise_id = $1 ORDER BY position ASC`, exerciseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []TestCase{}
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.CodingExerciseID, &tc.Input, &tc.ExpectedOutput, &tc.Position, &tc.Hidden, &tc.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, tc)
	}
	return result, rows.Err()
}

// ListVisibleTestCases is the student-facing listing (worked examples) —
// hidden=false only, and the caller (service.go) maps rows into
// StudentTestCase so a Hidden/CreatedAt field can never leak either.
func (r *Repository) ListVisibleTestCases(ctx context.Context, exerciseID uuid.UUID) ([]TestCase, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+testCaseColumns+` FROM coding_test_cases WHERE coding_exercise_id = $1 AND hidden = false ORDER BY position ASC`, exerciseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []TestCase{}
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(&tc.ID, &tc.CodingExerciseID, &tc.Input, &tc.ExpectedOutput, &tc.Position, &tc.Hidden, &tc.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, tc)
	}
	return result, rows.Err()
}

func (r *Repository) GetTestCase(ctx context.Context, id uuid.UUID) (*TestCase, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+testCaseColumns+` FROM coding_test_cases WHERE id = $1`, id)
	return scanTestCase(row)
}

func (r *Repository) UpdateTestCase(ctx context.Context, id uuid.UUID, input TestCaseInput) (*TestCase, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE coding_test_cases
		SET input = $2, expected_output = $3, position = $4, hidden = $5
		WHERE id = $1
		RETURNING `+testCaseColumns,
		id, input.Input, input.ExpectedOutput, input.Position, input.Hidden)
	tc, err := scanTestCase(row)
	if isUniqueViolation(err) {
		return nil, ErrPositionConflict
	}
	return tc, err
}

func (r *Repository) DeleteTestCase(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM coding_test_cases WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- code_submissions ----------------------------------------------------

const submissionColumns = `id, exercise_id, user_id, language, source_code, mode, status, passed_tests, total_tests, execution_time_ms, memory_used_kb, stdout, compile_output, created_at, finished_at`

func scanSubmission(row pgx.Row) (*Submission, error) {
	var s Submission
	err := row.Scan(&s.ID, &s.ExerciseID, &s.UserID, &s.Language, &s.SourceCode, &s.Mode, &s.Status,
		&s.PassedTests, &s.TotalTests, &s.ExecutionTimeMS, &s.MemoryUsedKB, &s.Stdout, &s.CompileOutput,
		&s.CreatedAt, &s.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateSubmission inserts the submission row and its execution job in one
// transaction — a submission can never exist without a job to process it
// (and vice versa), mirroring how videos.Repository.CreateJob is always
// called immediately after a lesson_videos insert.
func (r *Repository) CreateSubmission(ctx context.Context, exerciseID, userID uuid.UUID, language, sourceCode, mode string) (*Submission, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO code_submissions (exercise_id, user_id, language, source_code, mode)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+submissionColumns,
		exerciseID, userID, language, sourceCode, mode)
	s, err := scanSubmission(row)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `INSERT INTO code_execution_jobs (submission_id) VALUES ($1)`, s.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) GetSubmission(ctx context.Context, id uuid.UUID) (*Submission, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+submissionColumns+` FROM code_submissions WHERE id = $1`, id)
	return scanSubmission(row)
}

// ListSubmissionsForUserExercise backs the student-facing attempt history —
// most recent first, capped by the caller.
func (r *Repository) ListSubmissionsForUserExercise(ctx context.Context, exerciseID, userID uuid.UUID, limit int) ([]Submission, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+submissionColumns+`
		FROM code_submissions
		WHERE exercise_id = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`, exerciseID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Submission{}
	for rows.Next() {
		var s Submission
		if err := rows.Scan(&s.ID, &s.ExerciseID, &s.UserID, &s.Language, &s.SourceCode, &s.Mode, &s.Status,
			&s.PassedTests, &s.TotalTests, &s.ExecutionTimeMS, &s.MemoryUsedKB, &s.Stdout, &s.CompileOutput,
			&s.CreatedAt, &s.FinishedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// CountRecentSubmissions backs the per-minute rate limiter (item 29) — uses
// idx_code_submissions_user_created.
func (r *Repository) CountRecentSubmissions(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM code_submissions WHERE user_id = $1 AND created_at >= $2
	`, userID, since).Scan(&count)
	return count, err
}

// CountActiveSubmissions backs the concurrency cap (item 30) — queued or
// running submissions across every exercise for this user.
func (r *Repository) CountActiveSubmissions(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM code_submissions WHERE user_id = $1 AND status IN ('queued', 'running')
	`, userID).Scan(&count)
	return count, err
}

// --- execution context for the runner ------------------------------------

// ExecutionContext is everything cmd/code-runner needs to run one
// submission: its own source/language, the exercise's resource limits, and
// every test case (hidden and visible) it must be checked against.
type ExecutionContext struct {
	Submission    Submission
	TimeLimitMS   int
	MemoryLimitMB int
	TestCases     []TestCase
}

func (r *Repository) GetExecutionContext(ctx context.Context, submissionID uuid.UUID) (*ExecutionContext, error) {
	s, err := r.GetSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	var timeLimitMS, memoryLimitMB int
	if err := r.pool.QueryRow(ctx, `SELECT time_limit_ms, memory_limit_mb FROM coding_exercises WHERE id = $1`, s.ExerciseID).
		Scan(&timeLimitMS, &memoryLimitMB); err != nil {
		return nil, err
	}

	testCases, err := r.ListTestCases(ctx, s.ExerciseID)
	if err != nil {
		return nil, err
	}

	return &ExecutionContext{Submission: *s, TimeLimitMS: timeLimitMS, MemoryLimitMB: memoryLimitMB, TestCases: testCases}, nil
}

// MarkSubmissionRunning flips a claimed submission to "running" so a
// concurrent GET /code-submissions/:id shows live progress instead of
// stale "queued" for the whole duration of a slow test suite.
func (r *Repository) MarkSubmissionRunning(ctx context.Context, submissionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE code_submissions SET status = 'running' WHERE id = $1`, submissionID)
	return err
}

// MarkSubmissionResult persists the final outcome and, only for a passing
// mode="submit" submission of a required exercise, recalculates course
// completion in the same transaction — mirrors assignments.Repository.Review's
// "one big transaction" shape (update snapshot, then conditionally
// recalculate). stdout is written only when includeStdout is true (the
// caller passes false for a submit-mode result — see model.go's Submission
// doc comment on why).
func (r *Repository) MarkSubmissionResult(ctx context.Context, submissionID uuid.UUID, result ExecutionResult, includeStdout bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var stdout *string
	if includeStdout && result.Stdout != "" {
		stdout = &result.Stdout
	}
	var compileOutput *string
	if result.CompileOutput != "" {
		compileOutput = &result.CompileOutput
	}
	var execMS *int
	if result.ExecutionTimeMS > 0 {
		execMS = &result.ExecutionTimeMS
	}

	row := tx.QueryRow(ctx, `
		UPDATE code_submissions
		SET status = $2, passed_tests = $3, total_tests = $4, execution_time_ms = $5,
		    stdout = $6, compile_output = $7, finished_at = now()
		WHERE id = $1
		RETURNING `+submissionColumns,
		submissionID, result.Status, result.PassedTests, result.TotalTests, execMS, stdout, compileOutput)
	submission, err := scanSubmission(row)
	if err != nil {
		return err
	}

	if submission.Mode == ModeSubmit && result.Status == StatusPassed {
		var exerciseTitle string
		if err := tx.QueryRow(ctx, `SELECT title FROM coding_exercises WHERE id = $1`, submission.ExerciseID).Scan(&exerciseTitle); err != nil {
			return err
		}
		if err := activity.Record(ctx, tx, activity.RecordInput{
			UserID: submission.UserID, Type: activity.TypeCodingExercisePassed,
			EntityType: "code_submission", EntityID: &submission.ID,
			Metadata:  map[string]any{"title": exerciseTitle},
			DedupeKey: "coding_exercise_passed_activity:" + submission.ID.String(),
		}); err != nil {
			return err
		}
		if err := achievements.Evaluate(ctx, tx, submission.UserID); err != nil {
			return err
		}

		var courseID uuid.UUID
		var required bool
		if err := tx.QueryRow(ctx, `
			SELECT m.course_id, ce.required
			FROM coding_exercises ce
			JOIN lessons l ON l.id = ce.lesson_id
			JOIN modules m ON m.id = l.module_id
			WHERE ce.id = $1
		`, submission.ExerciseID).Scan(&courseID, &required); err != nil {
			return err
		}
		if required {
			if err := recalculateCourseCompletion(ctx, tx, submission.UserID, courseID); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// --- code_execution_jobs (durable queue) ----------------------------------

func (r *Repository) GetJobBySubmission(ctx context.Context, submissionID uuid.UUID) (*ExecutionJob, error) {
	var j ExecutionJob
	err := r.pool.QueryRow(ctx, `
		SELECT id, submission_id, status, attempts, available_at, started_at, finished_at, last_error, created_at
		FROM code_execution_jobs WHERE submission_id = $1
	`, submissionID).Scan(&j.ID, &j.SubmissionID, &j.Status, &j.Attempts, &j.AvailableAt, &j.StartedAt, &j.FinishedAt, &j.LastError, &j.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// ClaimNextJob atomically picks one pending, due job and marks it
// processing — FOR UPDATE SKIP LOCKED means a second code-runner replica
// polling at the same instant simply skips a row already locked by this
// query instead of blocking or double-claiming it. Byte-for-byte the same
// shape as videos.Repository.ClaimNextJob.
func (r *Repository) ClaimNextJob(ctx context.Context) (*ExecutionJob, error) {
	var j ExecutionJob
	err := r.pool.QueryRow(ctx, `
		UPDATE code_execution_jobs
		SET status = 'processing', started_at = now(), attempts = attempts + 1
		WHERE id = (
			SELECT id FROM code_execution_jobs
			WHERE status = 'pending' AND available_at <= now()
			ORDER BY available_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, submission_id, status, attempts, available_at, started_at, finished_at, last_error, created_at
	`).Scan(&j.ID, &j.SubmissionID, &j.Status, &j.Attempts, &j.AvailableAt, &j.StartedAt, &j.FinishedAt, &j.LastError, &j.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoJobAvailable
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *Repository) MarkJobCompleted(ctx context.Context, jobID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE code_execution_jobs SET status = 'completed', finished_at = now() WHERE id = $1`, jobID)
	return err
}

// MarkJobFailedOrRetry requeues the job (available after a backoff delay)
// if it still has attempts left, or marks it permanently failed once
// attempts (already incremented by ClaimNextJob) reaches maxAttempts.
// Byte-for-byte the same shape as videos.Repository.MarkJobFailedOrRetry.
func (r *Repository) MarkJobFailedOrRetry(ctx context.Context, jobID uuid.UUID, attempts, maxAttempts int, errMsg string, backoff time.Duration) (retried bool, err error) {
	if attempts >= maxAttempts {
		_, err = r.pool.Exec(ctx, `
			UPDATE code_execution_jobs SET status = 'failed', finished_at = now(), last_error = $2 WHERE id = $1
		`, jobID, errMsg)
		return false, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE code_execution_jobs SET status = 'pending', available_at = now() + make_interval(secs => $2), last_error = $3 WHERE id = $1
	`, jobID, backoff.Seconds(), errMsg)
	return true, err
}
