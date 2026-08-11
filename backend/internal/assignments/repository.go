package assignments

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
	"lms-backend/internal/pagination"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrDuplicateLesson  = errors.New("this lesson already has an assignment")
	ErrHasSubmissions   = errors.New("assignment has student submissions")
	ErrLessonNotFound   = errors.New("lesson not found")
	ErrSubmissionExists = errors.New("submission already exists")
	ErrNotEditable      = errors.New("submission cannot be edited in its current status")
	ErrEmptySubmission  = errors.New("submission must contain text or at least one file")
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

// assignmentApprovedForLesson and recalculateCourseCompletionSQL are kept
// byte-for-byte identical to internal/learning's copies (see that package
// for the full rationale) — this package never imports internal/learning,
// so the completion rule is duplicated here for the one place assignments
// itself can flip a lesson from "not done" to "done" (an approval).
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
// byte identical to internal/coding's copy — see that package for the full
// rationale. Duplicated here so a required, published coding exercise
// behaves exactly like a required assignment in the completion rule.
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
	// Stage 17: record the activity + re-evaluate achievements (FIRST_COURSE)
	// in the same transaction as the completion flip itself.
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

const assignmentColumns = `id, lesson_id, title, description, instructions, required, max_score, published, created_at, updated_at`

func scanAssignment(row pgx.Row) (*Assignment, error) {
	var a Assignment
	err := row.Scan(&a.ID, &a.LessonID, &a.Title, &a.Description, &a.Instructions, &a.Required, &a.MaxScore, &a.Published, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Repository) CreateAssignment(ctx context.Context, lessonID uuid.UUID, input AssignmentInput) (*Assignment, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO assignments (lesson_id, title, description, instructions, required, max_score, published)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+assignmentColumns,
		lessonID, input.Title, input.Description, input.Instructions, input.Required, input.MaxScore, input.Published)
	a, err := scanAssignment(row)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateLesson
	}
	return a, err
}

func (r *Repository) GetAssignmentByLesson(ctx context.Context, lessonID uuid.UUID) (*Assignment, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+assignmentColumns+` FROM assignments WHERE lesson_id = $1`, lessonID)
	return scanAssignment(row)
}

// GetPublishedAssignmentByLesson is the student-facing lookup — an
// unpublished (draft) assignment is treated as not existing at all, same
// convention tests.Repository.GetTestContext uses for unpublished tests.
func (r *Repository) GetPublishedAssignmentByLesson(ctx context.Context, lessonID uuid.UUID) (*Assignment, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+assignmentColumns+` FROM assignments WHERE lesson_id = $1 AND published = true`, lessonID)
	return scanAssignment(row)
}

func (r *Repository) GetAssignment(ctx context.Context, id uuid.UUID) (*Assignment, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+assignmentColumns+` FROM assignments WHERE id = $1`, id)
	return scanAssignment(row)
}

func (r *Repository) UpdateAssignment(ctx context.Context, id uuid.UUID, input AssignmentInput) (*Assignment, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE assignments
		SET title = $2, description = $3, instructions = $4, required = $5, max_score = $6, published = $7, updated_at = now()
		WHERE id = $1
		RETURNING `+assignmentColumns,
		id, input.Title, input.Description, input.Instructions, input.Required, input.MaxScore, input.Published)
	return scanAssignment(row)
}

func (r *Repository) CountSubmissionsForAssignment(ctx context.Context, assignmentID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM assignment_submissions WHERE assignment_id = $1`, assignmentID).Scan(&count)
	return count, err
}

func (r *Repository) DeleteAssignment(ctx context.Context, id uuid.UUID) error {
	count, err := r.CountSubmissionsForAssignment(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrHasSubmissions
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM assignments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LessonContext mirrors learning.Repository.GetLessonContext — duplicated
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

// IsEnrolled mirrors tests.Repository.IsEnrolled — duplicated per
// convention (assignments never imports learning/tests).
func (r *Repository) IsEnrolled(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM course_enrollments WHERE user_id = $1 AND course_id = $2)
	`, userID, courseID).Scan(&exists)
	return exists, err
}

const submissionColumns = `id, assignment_id, user_id, text_content, status, score, instructor_feedback, submitted_at, reviewed_at, reviewed_by, created_at, updated_at`

func scanSubmission(row pgx.Row) (*Submission, error) {
	var s Submission
	err := row.Scan(&s.ID, &s.AssignmentID, &s.UserID, &s.TextContent, &s.Status, &s.Score, &s.InstructorFeedback,
		&s.SubmittedAt, &s.ReviewedAt, &s.ReviewedBy, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) GetSubmission(ctx context.Context, assignmentID, userID uuid.UUID) (*Submission, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+submissionColumns+` FROM assignment_submissions WHERE assignment_id = $1 AND user_id = $2`, assignmentID, userID)
	s, err := scanSubmission(row)
	if err != nil {
		return nil, err
	}
	files, err := r.ListFiles(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	s.Files = files
	return s, nil
}

func (r *Repository) GetSubmissionByID(ctx context.Context, id uuid.UUID) (*Submission, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+submissionColumns+` FROM assignment_submissions WHERE id = $1`, id)
	s, err := scanSubmission(row)
	if err != nil {
		return nil, err
	}
	files, err := r.ListFiles(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	s.Files = files
	return s, nil
}

// EnsureDraft creates an empty draft row if one doesn't already exist yet
// (idempotent) — used before attaching a file so there's always a
// submission id to hang it off, even if the student uploads a file before
// typing any text.
func (r *Repository) EnsureDraft(ctx context.Context, assignmentID, userID uuid.UUID) (*Submission, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO assignment_submissions (assignment_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (assignment_id, user_id) DO NOTHING
	`, assignmentID, userID)
	if err != nil {
		return nil, err
	}
	return r.GetSubmission(ctx, assignmentID, userID)
}

func (r *Repository) SaveDraftText(ctx context.Context, assignmentID, userID uuid.UUID, textContent string) (*Submission, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO assignment_submissions (assignment_id, user_id, text_content)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (assignment_id, user_id) DO UPDATE
		SET text_content = NULLIF($3, ''), updated_at = now()
		RETURNING `+submissionColumns,
		assignmentID, userID, textContent)
	s, err := scanSubmission(row)
	if err != nil {
		return nil, err
	}
	files, err := r.ListFiles(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	s.Files = files
	return s, nil
}

func (r *Repository) ListFiles(ctx context.Context, submissionID uuid.UUID) ([]SubmissionFile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, original_filename, mime_type, size_bytes, created_at
		FROM assignment_submission_files
		WHERE submission_id = $1
		ORDER BY created_at ASC
	`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SubmissionFile{}
	for rows.Next() {
		var f SubmissionFile
		if err := rows.Scan(&f.ID, &f.OriginalFilename, &f.MimeType, &f.SizeBytes, &f.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

func (r *Repository) CountFiles(ctx context.Context, submissionID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM assignment_submission_files WHERE submission_id = $1`, submissionID).Scan(&count)
	return count, err
}

func (r *Repository) InsertFile(ctx context.Context, submissionID uuid.UUID, objectKey, originalFilename, mimeType string, sizeBytes int64) (*SubmissionFile, error) {
	var f SubmissionFile
	err := r.pool.QueryRow(ctx, `
		INSERT INTO assignment_submission_files (submission_id, object_key, original_filename, mime_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, original_filename, mime_type, size_bytes, created_at
	`, submissionID, objectKey, originalFilename, mimeType, sizeBytes).Scan(&f.ID, &f.OriginalFilename, &f.MimeType, &f.SizeBytes, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// FileDownloadContext carries everything the download handler needs to
// authorize a request: who owns the submission, and which course it
// belongs to (for the instructor-ownership check).
type FileDownloadContext struct {
	ObjectKey        string
	OriginalFilename string
	MimeType         string
	SubmissionUserID uuid.UUID
	CourseID         uuid.UUID
}

func (r *Repository) GetFileDownloadContext(ctx context.Context, fileID uuid.UUID) (*FileDownloadContext, error) {
	var fc FileDownloadContext
	err := r.pool.QueryRow(ctx, `
		SELECT f.object_key, f.original_filename, f.mime_type, s.user_id, m.course_id
		FROM assignment_submission_files f
		JOIN assignment_submissions s ON s.id = f.submission_id
		JOIN assignments a ON a.id = s.assignment_id
		JOIN lessons l ON l.id = a.lesson_id
		JOIN modules m ON m.id = l.module_id
		WHERE f.id = $1
	`, fileID).Scan(&fc.ObjectKey, &fc.OriginalFilename, &fc.MimeType, &fc.SubmissionUserID, &fc.CourseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &fc, nil
}

// Submit transitions a draft/needs_revision submission to "submitted" and
// enqueues the instructor notification (if the course has an assigned
// instructor) in the same transaction.
func (r *Repository) Submit(ctx context.Context, assignmentID, userID uuid.UUID) (*Submission, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `SELECT `+submissionColumns+` FROM assignment_submissions WHERE assignment_id = $1 AND user_id = $2 FOR UPDATE`, assignmentID, userID)
	current, err := scanSubmission(row)
	if err != nil {
		return nil, err
	}
	if !EditableStatuses[current.Status] {
		return nil, ErrNotEditable
	}

	var fileCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM assignment_submission_files WHERE submission_id = $1`, current.ID).Scan(&fileCount); err != nil {
		return nil, err
	}
	hasText := current.TextContent != nil && *current.TextContent != ""
	if !hasText && fileCount == 0 {
		return nil, ErrEmptySubmission
	}

	row = tx.QueryRow(ctx, `
		UPDATE assignment_submissions
		SET status = 'submitted', submitted_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING `+submissionColumns,
		current.ID)
	updated, err := scanSubmission(row)
	if err != nil {
		return nil, err
	}

	var courseID, instructorID *uuid.UUID
	var assignmentTitle, courseTitle string
	err = tx.QueryRow(ctx, `
		SELECT m.course_id, c.instructor_id, a.title, c.title
		FROM assignments a
		JOIN lessons l ON l.id = a.lesson_id
		JOIN modules m ON m.id = l.module_id
		JOIN courses c ON c.id = m.course_id
		WHERE a.id = $1
	`, assignmentID).Scan(&courseID, &instructorID, &assignmentTitle, &courseTitle)
	if err != nil {
		return nil, err
	}

	if instructorID != nil {
		if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
			UserID: *instructorID,
			Type:   notifications.TypeAssignmentSubmitted,
			Data: map[string]any{
				"assignment_title": assignmentTitle,
				"course_title":     courseTitle,
			},
			// updated.UpdatedAt is fresh now() from the UPDATE above, so a
			// genuine resubmit (a later, later timestamp) always produces a
			// new dedupe key — see item 20.
			DedupeKey: "assignment_submitted:" + updated.ID.String() + ":" + updated.UpdatedAt.Format(time.RFC3339Nano),
			Channels:  []string{notifications.ChannelInApp},
		}); err != nil {
			return nil, err
		}
	}

	if err := activity.Record(ctx, tx, activity.RecordInput{
		UserID: userID, Type: activity.TypeAssignmentSubmitted,
		EntityType: "assignment", EntityID: &assignmentID,
		Metadata:  map[string]any{"title": assignmentTitle},
		DedupeKey: "assignment_submitted_activity:" + updated.ID.String() + ":" + updated.UpdatedAt.Format(time.RFC3339Nano),
	}); err != nil {
		return nil, err
	}

	files, err := listFilesTx(ctx, tx, updated.ID)
	if err != nil {
		return nil, err
	}
	updated.Files = files

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return updated, nil
}

func listFilesTx(ctx context.Context, tx pgx.Tx, submissionID uuid.UUID) ([]SubmissionFile, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, original_filename, mime_type, size_bytes, created_at
		FROM assignment_submission_files
		WHERE submission_id = $1
		ORDER BY created_at ASC
	`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SubmissionFile{}
	for rows.Next() {
		var f SubmissionFile
		if err := rows.Scan(&f.ID, &f.OriginalFilename, &f.MimeType, &f.SizeBytes, &f.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

// Review is Stage 15's one big transaction (item 25): update the
// submission's current-state snapshot, insert an immutable review-history
// row, enqueue the student's notification, and — only on approval of a
// required assignment — recalculate course completion, all atomically.
func (r *Repository) Review(ctx context.Context, submissionID, reviewerID uuid.UUID, input ReviewInput) (*Submission, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		UPDATE assignment_submissions
		SET status = $2, score = $3, instructor_feedback = $4, reviewed_at = now(), reviewed_by = $5, updated_at = now()
		WHERE id = $1
		RETURNING `+submissionColumns,
		submissionID, input.Status, input.Score, input.Feedback, reviewerID)
	submission, err := scanSubmission(row)
	if err != nil {
		return nil, err
	}

	var reviewID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO assignment_submission_reviews (submission_id, reviewer_id, status, score, feedback)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, submissionID, reviewerID, input.Status, input.Score, input.Feedback).Scan(&reviewID); err != nil {
		return nil, err
	}

	var courseID uuid.UUID
	var required bool
	var assignmentTitle, courseTitle string
	if err := tx.QueryRow(ctx, `
		SELECT m.course_id, a.required, a.title, c.title
		FROM assignment_submissions s
		JOIN assignments a ON a.id = s.assignment_id
		JOIN lessons l ON l.id = a.lesson_id
		JOIN modules m ON m.id = l.module_id
		JOIN courses c ON c.id = m.course_id
		WHERE s.id = $1
	`, submissionID).Scan(&courseID, &required, &assignmentTitle, &courseTitle); err != nil {
		return nil, err
	}

	notifType := notifications.TypeAssignmentApproved
	if input.Status == "needs_revision" {
		notifType = notifications.TypeAssignmentNeedsRevision
	}
	if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
		UserID: submission.UserID,
		Type:   notifType,
		Data: map[string]any{
			"assignment_title": assignmentTitle,
			"course_title":     courseTitle,
			"feedback":         input.Feedback,
		},
		// Every review creates a brand-new assignment_submission_reviews
		// row with its own id — using that id as the dedupe key means a
		// resubmit + re-review is structurally a new event, never
		// swallowed by an earlier review's dedupe row (item 20).
		DedupeKey: "assignment_review:" + reviewID.String(),
		Channels:  []string{notifications.ChannelInApp},
	}); err != nil {
		return nil, err
	}

	if input.Status == "approved" {
		if err := activity.Record(ctx, tx, activity.RecordInput{
			UserID: submission.UserID, Type: activity.TypeAssignmentApproved,
			EntityType: "assignment_submission", EntityID: &submissionID,
			Metadata:  map[string]any{"title": assignmentTitle},
			DedupeKey: "assignment_approved_activity:" + reviewID.String(),
		}); err != nil {
			return nil, err
		}
		if err := achievements.Evaluate(ctx, tx, submission.UserID); err != nil {
			return nil, err
		}
	}

	if input.Status == "approved" && required {
		if err := recalculateCourseCompletion(ctx, tx, submission.UserID, courseID); err != nil {
			return nil, err
		}
	}

	files, err := listFilesTx(ctx, tx, submission.ID)
	if err != nil {
		return nil, err
	}
	submission.Files = files

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return submission, nil
}

func (r *Repository) ListReviews(ctx context.Context, submissionID uuid.UUID) ([]SubmissionReview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, submission_id, reviewer_id, status, score, feedback, created_at
		FROM assignment_submission_reviews
		WHERE submission_id = $1
		ORDER BY created_at ASC
	`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []SubmissionReview{}
	for rows.Next() {
		var rv SubmissionReview
		if err := rows.Scan(&rv.ID, &rv.SubmissionID, &rv.ReviewerID, &rv.Status, &rv.Score, &rv.Feedback, &rv.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, rv)
	}
	return result, rows.Err()
}

// ListSubmissions backs both the global instructor inbox and the
// per-assignment list — a single query, no N+1, scoped by whichever of
// InstructorID/CourseID/AssignmentID/Status are set.
func (r *Repository) ListSubmissions(ctx context.Context, params ListSubmissionsParams) ([]InstructorSubmissionRow, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			s.id, s.assignment_id, a.title, a.lesson_id, l.title, m.course_id, c.title,
			s.user_id, TRIM(u.first_name || ' ' || u.last_name), s.status, s.score, s.submitted_at,
			COUNT(*) OVER() AS total
		FROM assignment_submissions s
		JOIN assignments a ON a.id = s.assignment_id
		JOIN lessons l ON l.id = a.lesson_id
		JOIN modules m ON m.id = l.module_id
		JOIN courses c ON c.id = m.course_id
		JOIN users u ON u.id = s.user_id
		WHERE s.status != 'draft'
			AND ($1::uuid IS NULL OR c.instructor_id = $1)
			AND ($2::uuid IS NULL OR c.id = $2)
			AND ($3::uuid IS NULL OR a.id = $3)
			AND ($4 = '' OR s.status = $4)
		ORDER BY s.submitted_at DESC NULLS LAST, s.updated_at DESC
		LIMIT $5 OFFSET $6
	`, params.InstructorID, params.CourseID, params.AssignmentID, params.Status, params.Limit, pagination.Offset(params.Page, params.Limit))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []InstructorSubmissionRow{}
	total := 0
	for rows.Next() {
		var row InstructorSubmissionRow
		if err := rows.Scan(&row.SubmissionID, &row.AssignmentID, &row.AssignmentTitle, &row.LessonID, &row.LessonTitle,
			&row.CourseID, &row.CourseTitle, &row.StudentID, &row.StudentName, &row.Status, &row.Score, &row.SubmittedAt, &total); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	return result, total, rows.Err()
}

// GetSubmissionDetail is the full review-screen payload for one submission.
func (r *Repository) GetSubmissionDetail(ctx context.Context, submissionID uuid.UUID) (*SubmissionDetail, error) {
	var d SubmissionDetail
	err := r.pool.QueryRow(ctx, `
		SELECT
			s.id, s.assignment_id, s.user_id, s.text_content, s.status, s.score, s.instructor_feedback,
			s.submitted_at, s.reviewed_at, s.reviewed_by, s.created_at, s.updated_at,
			a.title, a.max_score, l.id, l.title, m.course_id, c.title, TRIM(u.first_name || ' ' || u.last_name)
		FROM assignment_submissions s
		JOIN assignments a ON a.id = s.assignment_id
		JOIN lessons l ON l.id = a.lesson_id
		JOIN modules m ON m.id = l.module_id
		JOIN courses c ON c.id = m.course_id
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1
	`, submissionID).Scan(&d.ID, &d.AssignmentID, &d.UserID, &d.TextContent, &d.Status, &d.Score, &d.InstructorFeedback,
		&d.SubmittedAt, &d.ReviewedAt, &d.ReviewedBy, &d.CreatedAt, &d.UpdatedAt,
		&d.AssignmentTitle, &d.MaxScore, &d.LessonID, &d.LessonTitle, &d.CourseID, &d.CourseTitle, &d.StudentName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	files, err := r.ListFiles(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	d.Files = files

	reviews, err := r.ListReviews(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	d.Reviews = reviews

	return &d, nil
}
