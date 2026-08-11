package tests

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
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidParent = errors.New("a test must belong to exactly one of course, module, or lesson")
)

// assignmentApprovedForLesson is kept byte-for-byte identical to
// internal/learning's copy (see that package for the full rationale) —
// this package never imports internal/learning, so the completion rule's
// SQL is duplicated here rather than shared.
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

// codingExerciseOK is Stage 16's completion-rule addition, kept byte-for-
// byte identical to internal/coding's copy (see that package for the full
// rationale).
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

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// TestContext is the test row plus the course it ultimately belongs to,
// resolved regardless of whether the test is attached at the course,
// module, or lesson level.
type TestContext struct {
	Test     Test
	CourseID *uuid.UUID
}

// GetTestContext returns ErrNotFound if the test doesn't exist or isn't
// published — an unpublished test is treated as not-yet-available, not as a
// distinct "forbidden" case, to avoid revealing draft content exists at all.
func (r *Repository) GetTestContext(ctx context.Context, testID uuid.UUID) (*TestContext, error) {
	var tc TestContext
	var t Test
	err := r.pool.QueryRow(ctx, `
		SELECT
			t.id, t.course_id, t.module_id, t.lesson_id, t.title, t.description,
			t.passing_score, t.published, t.is_final, t.created_at, t.updated_at,
			COALESCE(t.course_id, m.course_id, lm.course_id) AS resolved_course_id
		FROM tests t
		LEFT JOIN modules m ON m.id = t.module_id
		LEFT JOIN lessons l ON l.id = t.lesson_id
		LEFT JOIN modules lm ON lm.id = l.module_id
		WHERE t.id = $1 AND t.published = true
	`, testID).Scan(&t.ID, &t.CourseID, &t.ModuleID, &t.LessonID, &t.Title, &t.Description,
		&t.PassingScore, &t.Published, &t.IsFinal, &t.CreatedAt, &t.UpdatedAt, &tc.CourseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	tc.Test = t
	return &tc, nil
}

func (r *Repository) IsEnrolled(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM course_enrollments WHERE user_id = $1 AND course_id = $2)
	`, userID, courseID).Scan(&exists)
	return exists, err
}

// AllPublishedLessonsCompleted mirrors the same lesson-completion formula
// used by the learning domain (published lessons only). Duplicated rather
// than imported, consistent with how courses/learning/specialities each own
// their SQL against the shared schema instead of importing one another.
func (r *Repository) AllPublishedLessonsCompleted(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	var total, done int
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(l.id) FILTER (WHERE l.published) AS total,
			COUNT(lp.id) FILTER (WHERE l.published AND lp.completed) AS done
		FROM modules m
		LEFT JOIN lessons l ON l.module_id = m.id
		LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
		WHERE m.course_id = $2
	`, userID, courseID).Scan(&total, &done)
	if err != nil {
		return false, err
	}
	return total > 0 && done >= total, nil
}

func (r *Repository) ListPublicQuestions(ctx context.Context, testID uuid.UUID) ([]PublicQuestion, error) {
	// Note: is_correct is not part of this SELECT at all.
	rows, err := r.pool.Query(ctx, `
		SELECT q.id, q.text, q.position, a.id, a.text, a.position
		FROM questions q
		JOIN answers a ON a.question_id = q.id
		WHERE q.test_id = $1
		ORDER BY q.position ASC, a.position ASC
	`, testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	questionsByID := map[uuid.UUID]*PublicQuestion{}
	order := []uuid.UUID{}

	for rows.Next() {
		var qID uuid.UUID
		var qText string
		var qPosition int
		var answer PublicAnswer

		if err := rows.Scan(&qID, &qText, &qPosition, &answer.ID, &answer.Text, &answer.Position); err != nil {
			return nil, err
		}

		q, exists := questionsByID[qID]
		if !exists {
			q = &PublicQuestion{ID: qID, Text: qText, Position: qPosition, Answers: []PublicAnswer{}}
			questionsByID[qID] = q
			order = append(order, qID)
		}
		q.Answers = append(q.Answers, answer)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]PublicQuestion, 0, len(order))
	for _, id := range order {
		result = append(result, *questionsByID[id])
	}
	return result, nil
}

// answerRow carries everything needed to both grade a submission and build
// a post-submit review, in a single query.
type answerRow struct {
	QuestionID   uuid.UUID
	QuestionText string
	AnswerID     uuid.UUID
	AnswerText   string
	IsCorrect    bool
}

func (r *Repository) ListAnswerRowsForTest(ctx context.Context, testID uuid.UUID) ([]answerRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT q.id, q.text, a.id, a.text, a.is_correct
		FROM questions q
		JOIN answers a ON a.question_id = q.id
		WHERE q.test_id = $1
		ORDER BY q.position ASC, a.position ASC
	`, testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []answerRow{}
	for rows.Next() {
		var row answerRow
		if err := rows.Scan(&row.QuestionID, &row.QuestionText, &row.AnswerID, &row.AnswerText, &row.IsCorrect); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

type gradedAnswer struct {
	QuestionID uuid.UUID
	AnswerID   uuid.UUID
	Correct    bool
}

// SaveAttempt inserts the attempt and its per-question answers atomically.
func (r *Repository) SaveAttempt(ctx context.Context, userID, testID uuid.UUID, score int, passed bool, answers []gradedAnswer) (*Attempt, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var attempt Attempt
	err = tx.QueryRow(ctx, `
		INSERT INTO test_attempts (test_id, user_id, score, passed)
		VALUES ($1, $2, $3, $4)
		RETURNING id, test_id, user_id, score, passed, started_at, completed_at, created_at
	`, testID, userID, score, passed).Scan(&attempt.ID, &attempt.TestID, &attempt.UserID, &attempt.Score,
		&attempt.Passed, &attempt.StartedAt, &attempt.CompletedAt, &attempt.CreatedAt)
	if err != nil {
		return nil, err
	}

	for _, a := range answers {
		if _, err := tx.Exec(ctx, `
			INSERT INTO test_attempt_answers (attempt_id, question_id, answer_id, correct)
			VALUES ($1, $2, $3, $4)
		`, attempt.ID, a.QuestionID, a.AnswerID, a.Correct); err != nil {
			return nil, err
		}
	}

	if passed {
		var testTitle string
		if err := tx.QueryRow(ctx, `SELECT title FROM tests WHERE id = $1`, testID).Scan(&testTitle); err != nil {
			return nil, err
		}
		if err := activity.Record(ctx, tx, activity.RecordInput{
			UserID: userID, Type: activity.TypeTestPassed,
			EntityType: "test", EntityID: &testID,
			Metadata:  map[string]any{"title": testTitle},
			DedupeKey: "test_passed_activity:" + attempt.ID.String(),
		}); err != nil {
			return nil, err
		}
		if err := achievements.Evaluate(ctx, tx, userID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) ListAttemptsByUser(ctx context.Context, userID uuid.UUID) ([]AttemptSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ta.id, ta.test_id, ta.user_id, ta.score, ta.passed, ta.started_at, ta.completed_at, ta.created_at, t.title
		FROM test_attempts ta
		JOIN tests t ON t.id = ta.test_id
		WHERE ta.user_id = $1
		ORDER BY ta.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []AttemptSummary{}
	for rows.Next() {
		var a AttemptSummary
		if err := rows.Scan(&a.ID, &a.TestID, &a.UserID, &a.Score, &a.Passed, &a.StartedAt, &a.CompletedAt, &a.CreatedAt, &a.TestTitle); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (r *Repository) GetAttemptWithTest(ctx context.Context, attemptID uuid.UUID) (*Attempt, string, int, error) {
	var a Attempt
	var testTitle string
	var passingScore int
	err := r.pool.QueryRow(ctx, `
		SELECT ta.id, ta.test_id, ta.user_id, ta.score, ta.passed, ta.started_at, ta.completed_at, ta.created_at,
		       t.title, t.passing_score
		FROM test_attempts ta
		JOIN tests t ON t.id = ta.test_id
		WHERE ta.id = $1
	`, attemptID).Scan(&a.ID, &a.TestID, &a.UserID, &a.Score, &a.Passed, &a.StartedAt, &a.CompletedAt, &a.CreatedAt,
		&testTitle, &passingScore)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", 0, ErrNotFound
	}
	if err != nil {
		return nil, "", 0, err
	}
	return &a, testTitle, passingScore, nil
}

func (r *Repository) ListAttemptAnswerReviews(ctx context.Context, attemptID uuid.UUID) ([]AttemptAnswerReview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT taa.question_id, q.text, taa.answer_id, sa.text, taa.correct, ca.id, ca.text
		FROM test_attempt_answers taa
		JOIN questions q ON q.id = taa.question_id
		JOIN answers sa ON sa.id = taa.answer_id
		JOIN answers ca ON ca.question_id = taa.question_id AND ca.is_correct = true
		WHERE taa.attempt_id = $1
		ORDER BY q.position ASC
	`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []AttemptAnswerReview{}
	for rows.Next() {
		var review AttemptAnswerReview
		if err := rows.Scan(&review.QuestionID, &review.QuestionText, &review.SelectedAnswerID, &review.SelectedAnswerText,
			&review.Correct, &review.CorrectAnswerID, &review.CorrectAnswerText); err != nil {
			return nil, err
		}
		result = append(result, review)
	}
	return result, rows.Err()
}

// RecalculateCourseCompletion opens its own transaction so the completion
// flip (if any) and its notification are one atomic outcome — same
// reasoning as internal/learning's copy of this method.
func (r *Repository) RecalculateCourseCompletion(ctx context.Context, userID, courseID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var newCompleted, oldCompleted *time.Time
	err = tx.QueryRow(ctx, recalculateCourseCompletionSQL, userID, courseID).Scan(&newCompleted, &oldCompleted)
	if err != nil {
		return err
	}

	if oldCompleted == nil && newCompleted != nil {
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
		if err := achievements.Evaluate(ctx, tx, userID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Kept identical to internal/learning's copy on purpose — see the note on
// AllPublishedLessonsCompleted above. The leading "old" CTE + RETURNING
// exist only to detect the NULL->non-NULL transition for the notification
// above; see internal/learning's identical comment for the full rationale.
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

// --- Admin CRUD -------------------------------------------------------

const testColumns = `id, course_id, module_id, lesson_id, title, description, passing_score, published, is_final, created_at, updated_at`

func scanTest(row pgx.Row) (*Test, error) {
	var t Test
	err := row.Scan(&t.ID, &t.CourseID, &t.ModuleID, &t.LessonID, &t.Title, &t.Description,
		&t.PassingScore, &t.Published, &t.IsFinal, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTestsAdmin resolves each test's owning course title regardless of
// whether the test is attached at the course, module, or lesson level.
func (r *Repository) ListTestsAdmin(ctx context.Context) ([]TestSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			t.id, t.course_id, t.module_id, t.lesson_id, t.title, t.description,
			t.passing_score, t.published, t.is_final, t.created_at, t.updated_at,
			COALESCE(c.title, mc.title, lc.title, '') AS course_title
		FROM tests t
		LEFT JOIN courses c ON c.id = t.course_id
		LEFT JOIN modules m ON m.id = t.module_id
		LEFT JOIN courses mc ON mc.id = m.course_id
		LEFT JOIN lessons l ON l.id = t.lesson_id
		LEFT JOIN modules lm ON lm.id = l.module_id
		LEFT JOIN courses lc ON lc.id = lm.course_id
		ORDER BY t.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []TestSummary{}
	for rows.Next() {
		var ts TestSummary
		if err := rows.Scan(&ts.ID, &ts.CourseID, &ts.ModuleID, &ts.LessonID, &ts.Title, &ts.Description,
			&ts.PassingScore, &ts.Published, &ts.IsFinal, &ts.CreatedAt, &ts.UpdatedAt, &ts.CourseTitle); err != nil {
			return nil, err
		}
		result = append(result, ts)
	}
	return result, rows.Err()
}

func (r *Repository) GetTest(ctx context.Context, id uuid.UUID) (*Test, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+testColumns+` FROM tests WHERE id = $1`, id)
	return scanTest(row)
}

// GetAdminTestDetail includes is_correct — admin-only.
func (r *Repository) GetAdminTestDetail(ctx context.Context, testID uuid.UUID) (*AdminTestDetail, error) {
	test, err := r.GetTest(ctx, testID)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT q.id, q.test_id, q.text, q.position, a.id, a.text, a.is_correct, a.position, a.created_at
		FROM questions q
		LEFT JOIN answers a ON a.question_id = q.id
		WHERE q.test_id = $1
		ORDER BY q.position ASC, a.position ASC
	`, testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	questionsByID := map[uuid.UUID]*AdminQuestion{}
	order := []uuid.UUID{}

	for rows.Next() {
		var qID uuid.UUID
		var qTestID uuid.UUID
		var qText string
		var qPosition int
		var answerID *uuid.UUID
		var answerText *string
		var answerCorrect *bool
		var answerPosition *int
		var answerCreatedAt *time.Time

		if err := rows.Scan(&qID, &qTestID, &qText, &qPosition, &answerID, &answerText, &answerCorrect, &answerPosition, &answerCreatedAt); err != nil {
			return nil, err
		}

		q, exists := questionsByID[qID]
		if !exists {
			q = &AdminQuestion{ID: qID, TestID: qTestID, Text: qText, Position: qPosition, Answers: []Answer{}}
			questionsByID[qID] = q
			order = append(order, qID)
		}
		if answerID != nil {
			q.Answers = append(q.Answers, Answer{
				ID: *answerID, QuestionID: qID, Text: *answerText, IsCorrect: *answerCorrect,
				Position: *answerPosition, CreatedAt: *answerCreatedAt,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	questions := make([]AdminQuestion, 0, len(order))
	for _, id := range order {
		questions = append(questions, *questionsByID[id])
	}

	return &AdminTestDetail{Test: *test, Questions: questions}, nil
}

func (r *Repository) CreateTest(ctx context.Context, input TestInput) (*Test, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO tests (course_id, module_id, lesson_id, title, description, passing_score, published, is_final)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+testColumns, input.CourseID, input.ModuleID, input.LessonID, input.Title, input.Description,
		input.PassingScore, input.Published, input.IsFinal)
	test, err := scanTest(row)
	if isCheckViolation(err) {
		return nil, ErrInvalidParent
	}
	return test, err
}

func (r *Repository) UpdateTest(ctx context.Context, id uuid.UUID, input TestInput) (*Test, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE tests
		SET course_id = $2, module_id = $3, lesson_id = $4, title = $5, description = $6,
		    passing_score = $7, published = $8, is_final = $9, updated_at = now()
		WHERE id = $1
		RETURNING `+testColumns, id, input.CourseID, input.ModuleID, input.LessonID, input.Title, input.Description,
		input.PassingScore, input.Published, input.IsFinal)
	test, err := scanTest(row)
	if isCheckViolation(err) {
		return nil, ErrInvalidParent
	}
	return test, err
}

func (r *Repository) DeleteTest(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tests WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetQuestion(ctx context.Context, id uuid.UUID) (*Question, error) {
	var q Question
	err := r.pool.QueryRow(ctx, `
		SELECT id, test_id, text, position, created_at, updated_at FROM questions WHERE id = $1
	`, id).Scan(&q.ID, &q.TestID, &q.Text, &q.Position, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *Repository) CreateQuestion(ctx context.Context, testID uuid.UUID, input QuestionInput) (*Question, error) {
	var q Question
	err := r.pool.QueryRow(ctx, `
		INSERT INTO questions (test_id, text, position)
		VALUES ($1, $2, (SELECT COALESCE(MAX(position), 0) + 1 FROM questions WHERE test_id = $1))
		RETURNING id, test_id, text, position, created_at, updated_at
	`, testID, input.Text).Scan(&q.ID, &q.TestID, &q.Text, &q.Position, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *Repository) UpdateQuestion(ctx context.Context, id uuid.UUID, input QuestionInput) (*Question, error) {
	var q Question
	err := r.pool.QueryRow(ctx, `
		UPDATE questions SET text = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, test_id, text, position, created_at, updated_at
	`, id, input.Text).Scan(&q.ID, &q.TestID, &q.Text, &q.Position, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *Repository) DeleteQuestion(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM questions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateAnswer, when isCorrect is true, unsets is_correct on every other
// answer of the same question in the same transaction — the scoring and
// review logic (Stage 6) assumes exactly one correct answer per question,
// so the admin API enforces that invariant here instead of trusting the UI.
func (r *Repository) CreateAnswer(ctx context.Context, questionID uuid.UUID, input AnswerInput) (*Answer, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var a Answer
	err = tx.QueryRow(ctx, `
		INSERT INTO answers (question_id, text, is_correct, position)
		VALUES ($1, $2, $3, (SELECT COALESCE(MAX(position), 0) + 1 FROM answers WHERE question_id = $1))
		RETURNING id, question_id, text, is_correct, position, created_at
	`, questionID, input.Text, input.IsCorrect).Scan(&a.ID, &a.QuestionID, &a.Text, &a.IsCorrect, &a.Position, &a.CreatedAt)
	if err != nil {
		return nil, err
	}

	if input.IsCorrect {
		if _, err := tx.Exec(ctx, `UPDATE answers SET is_correct = false WHERE question_id = $1 AND id != $2`, questionID, a.ID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) UpdateAnswer(ctx context.Context, id uuid.UUID, input AnswerInput) (*Answer, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var a Answer
	err = tx.QueryRow(ctx, `
		UPDATE answers SET text = $2, is_correct = $3
		WHERE id = $1
		RETURNING id, question_id, text, is_correct, position, created_at
	`, id, input.Text, input.IsCorrect).Scan(&a.ID, &a.QuestionID, &a.Text, &a.IsCorrect, &a.Position, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if input.IsCorrect {
		if _, err := tx.Exec(ctx, `UPDATE answers SET is_correct = false WHERE question_id = $1 AND id != $2`, a.QuestionID, a.ID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) DeleteAnswer(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM answers WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
