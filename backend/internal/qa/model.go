// Package qa implements lesson-level Q&A (Stage 20A): a student asks a
// question about a specific lesson; enrolled peers, the course's own
// instructor, or an admin may answer. This is unrelated to the "questions"/
// "answers" vocabulary used by internal/tests (quiz questions) — that
// domain lives entirely under /admin/* and /instructor/*, this one under
// the bare /api/v1/* student-facing surface, so the two never collide at
// the route level despite the shared English word.
package qa

import (
	"time"

	"github.com/google/uuid"
)

// Question is the row shape. CourseID is denormalized from LessonID at
// creation time (see the migration's doc comment) so eligibility/ownership
// checks and moderation queries never need to re-derive it via a
// lesson->module->course join.
type Question struct {
	ID        uuid.UUID `json:"id"`
	LessonID  uuid.UUID `json:"lesson_id"`
	CourseID  uuid.UUID `json:"course_id"`
	UserID    uuid.UUID `json:"user_id"`
	Body      string    `json:"body"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Answer struct {
	ID                 uuid.UUID `json:"id"`
	QuestionID         uuid.UUID `json:"question_id"`
	UserID             uuid.UUID `json:"user_id"`
	Body               string    `json:"body"`
	IsInstructorAnswer bool      `json:"is_instructor_answer"`
	Published          bool      `json:"published"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// AnswerView adds the answerer's display name — never their email — for
// the public lesson Q&A thread, mirroring internal/reviews.PublicReview's
// identity-exposure convention.
type AnswerView struct {
	Answer
	DisplayName string `json:"display_name"`
}

// QuestionView is what GET /lessons/:id/questions returns: one row per
// question plus its full answer list, assembled by the repository in
// exactly two queries total (see repository.go's ListForLesson) regardless
// of how many questions or answers exist — never N+1.
type QuestionView struct {
	Question
	DisplayName string       `json:"display_name"`
	Answers     []AnswerView `json:"answers"`
}

type QuestionInput struct {
	Body string
}

type AnswerInput struct {
	Body string
}

// Eligibility mirrors internal/reviews.Eligibility's shape deliberately —
// same decision (Stage 20 planning: "keep this consistent with the
// existing review/enrollment eligibility model") — but Q&A only ever cares
// about enrollment, not review-style progress-based eligibility, so it's
// a single bool rather than reviews' two-field struct.
type Eligibility struct {
	Enrolled bool
}
