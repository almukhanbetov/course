package qa

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/notifications"
)

var (
	ErrNotFound      = errors.New("resource not found")
	ErrLessonMissing = errors.New("lesson not found")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// IsEnrolled backs the "asking requires enrollment" rule (Stage 20 planning
// decision: consistent with internal/reviews.CheckEligibility) and the
// "answer allowed for enrolled student" half of the answer-authorization
// rule. One query, no eligibility subtleties beyond enrollment existing —
// unlike reviews, Q&A doesn't require completed progress.
func (r *Repository) IsEnrolled(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	var enrolled bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM course_enrollments WHERE user_id = $1 AND course_id = $2)
	`, userID, courseID).Scan(&enrolled)
	return enrolled, err
}

func (r *Repository) CreateQuestion(ctx context.Context, lessonID, courseID, userID uuid.UUID, body string) (*Question, error) {
	var q Question
	err := r.pool.QueryRow(ctx, `
		INSERT INTO lesson_questions (lesson_id, course_id, user_id, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, lesson_id, course_id, user_id, body, published, created_at, updated_at
	`, lessonID, courseID, userID, body).Scan(
		&q.ID, &q.LessonID, &q.CourseID, &q.UserID, &q.Body, &q.Published, &q.CreatedAt, &q.UpdatedAt)
	if isForeignKeyViolation(err) {
		return nil, ErrLessonMissing
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *Repository) GetQuestion(ctx context.Context, id uuid.UUID) (*Question, error) {
	var q Question
	err := r.pool.QueryRow(ctx, `
		SELECT id, lesson_id, course_id, user_id, body, published, created_at, updated_at
		FROM lesson_questions WHERE id = $1
	`, id).Scan(&q.ID, &q.LessonID, &q.CourseID, &q.UserID, &q.Body, &q.Published, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// ListForLesson answers "questions for this lesson, each with its answers"
// in exactly two queries total, never N+1: one paginated query for the
// questions themselves (with the asker's display name and a window-function
// total), and one query for every answer belonging to that page's question
// ids via WHERE question_id = ANY($1), merged here in Go keyed by question
// id. Both published-only, matching internal/reviews' public-list
// convention of never surfacing hidden rows outside moderation.
func (r *Repository) ListForLesson(ctx context.Context, lessonID uuid.UUID, limit, offset int) ([]QuestionView, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT lq.id, lq.lesson_id, lq.course_id, lq.user_id, lq.body, lq.published, lq.created_at, lq.updated_at,
			TRIM(u.first_name || ' ' || u.last_name),
			COUNT(*) OVER() AS total
		FROM lesson_questions lq
		JOIN users u ON u.id = lq.user_id
		WHERE lq.lesson_id = $1 AND lq.published = true
		ORDER BY lq.created_at DESC
		LIMIT $2 OFFSET $3
	`, lessonID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	questions := []QuestionView{}
	questionIDs := make([]uuid.UUID, 0, limit)
	total := 0
	for rows.Next() {
		var qv QuestionView
		if err := rows.Scan(&qv.ID, &qv.LessonID, &qv.CourseID, &qv.UserID, &qv.Body, &qv.Published, &qv.CreatedAt, &qv.UpdatedAt,
			&qv.DisplayName, &total); err != nil {
			rows.Close()
			return nil, 0, err
		}
		qv.Answers = []AnswerView{}
		questions = append(questions, qv)
		questionIDs = append(questionIDs, qv.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()

	if len(questionIDs) == 0 {
		return questions, total, nil
	}

	answerRows, err := r.pool.Query(ctx, `
		SELECT qa.id, qa.question_id, qa.user_id, qa.body, qa.is_instructor_answer, qa.published, qa.created_at, qa.updated_at,
			TRIM(u.first_name || ' ' || u.last_name)
		FROM question_answers qa
		JOIN users u ON u.id = qa.user_id
		WHERE qa.question_id = ANY($1) AND qa.published = true
		ORDER BY qa.created_at ASC
	`, questionIDs)
	if err != nil {
		return nil, 0, err
	}
	defer answerRows.Close()

	byQuestion := make(map[uuid.UUID][]AnswerView, len(questionIDs))
	for answerRows.Next() {
		var av AnswerView
		if err := answerRows.Scan(&av.ID, &av.QuestionID, &av.UserID, &av.Body, &av.IsInstructorAnswer, &av.Published, &av.CreatedAt, &av.UpdatedAt,
			&av.DisplayName); err != nil {
			return nil, 0, err
		}
		byQuestion[av.QuestionID] = append(byQuestion[av.QuestionID], av)
	}
	if err := answerRows.Err(); err != nil {
		return nil, 0, err
	}

	for i := range questions {
		if answers, ok := byQuestion[questions[i].ID]; ok {
			questions[i].Answers = answers
		}
	}
	return questions, total, nil
}

// CreateAnswer inserts the answer and, in the same transaction, notifies
// the question's original asker — unless the asker is answering their own
// question, in which case nothing is enqueued. DedupeKey is keyed off the
// new answer's own id, which pgx generates fresh per INSERT, so a genuine
// second answer always produces a new notification and a retried request
// (if the client ever resends) can never produce two.
func (r *Repository) CreateAnswer(ctx context.Context, questionID, userID uuid.UUID, body string, isInstructorAnswer bool) (*Answer, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var a Answer
	err = tx.QueryRow(ctx, `
		INSERT INTO question_answers (question_id, user_id, body, is_instructor_answer)
		VALUES ($1, $2, $3, $4)
		RETURNING id, question_id, user_id, body, is_instructor_answer, published, created_at, updated_at
	`, questionID, userID, body, isInstructorAnswer).Scan(
		&a.ID, &a.QuestionID, &a.UserID, &a.Body, &a.IsInstructorAnswer, &a.Published, &a.CreatedAt, &a.UpdatedAt)
	if isForeignKeyViolation(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var askerID uuid.UUID
	var lessonTitle, courseTitle string
	err = tx.QueryRow(ctx, `
		SELECT lq.user_id, l.title, c.title
		FROM lesson_questions lq
		JOIN lessons l ON l.id = lq.lesson_id
		JOIN courses c ON c.id = lq.course_id
		WHERE lq.id = $1
	`, questionID).Scan(&askerID, &lessonTitle, &courseTitle)
	if err != nil {
		return nil, err
	}

	if askerID != userID {
		if err := notifications.Enqueue(ctx, tx, notifications.EnqueueInput{
			UserID: askerID,
			Type:   notifications.TypeQuestionAnswered,
			Data: map[string]any{
				"lesson_title": lessonTitle,
				"course_title": courseTitle,
			},
			DedupeKey: "question_answered:" + a.ID.String(),
			Channels:  []string{notifications.ChannelInApp},
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteQuestion and DeleteAnswer are IDOR-safe by construction: the WHERE
// clause scopes to the caller's own user_id, so a row that exists but
// belongs to someone else produces the exact same RowsAffected()==0 ->
// ErrNotFound outcome as a row that doesn't exist at all — this endpoint
// never distinguishes "not yours" from "doesn't exist" (same convention as
// internal/reviews.Delete and internal/wishlist).
func (r *Repository) DeleteQuestion(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM lesson_questions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteAnswer(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM question_answers WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
