package tests

import (
	"time"

	"github.com/google/uuid"
)

// Test is the full internal row. Never serialize this directly to a public
// endpoint — use PublicTest for anything a student can see before grading.
type Test struct {
	ID           uuid.UUID  `json:"id"`
	CourseID     *uuid.UUID `json:"course_id,omitempty"`
	ModuleID     *uuid.UUID `json:"module_id,omitempty"`
	LessonID     *uuid.UUID `json:"lesson_id,omitempty"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	PassingScore int        `json:"passing_score"`
	Published    bool       `json:"published"`
	IsFinal      bool       `json:"is_final"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PublicAnswer/PublicQuestion/PublicTest deliberately have no is_correct
// field at all (not just an omitted one) — the correctness of an answer is
// structurally impossible to leak through these types, and the repository
// query backing them never even selects the is_correct column.
type PublicAnswer struct {
	ID       uuid.UUID `json:"id"`
	Text     string    `json:"text"`
	Position int       `json:"position"`
}

type PublicQuestion struct {
	ID       uuid.UUID      `json:"id"`
	Text     string         `json:"text"`
	Position int            `json:"position"`
	Answers  []PublicAnswer `json:"answers"`
}

type PublicTest struct {
	ID           uuid.UUID        `json:"id"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	PassingScore int              `json:"passing_score"`
	IsFinal      bool             `json:"is_final"`
	Questions    []PublicQuestion `json:"questions"`
}

type Question struct {
	ID        uuid.UUID `json:"id"`
	TestID    uuid.UUID `json:"test_id"`
	Text      string    `json:"text"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Answer struct {
	ID         uuid.UUID `json:"id"`
	QuestionID uuid.UUID `json:"question_id"`
	Text       string    `json:"text"`
	IsCorrect  bool      `json:"is_correct"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminQuestion/AdminTestDetail are admin-only views that expose is_correct.
// They are structurally distinct from PublicQuestion/PublicTest above and
// are never returned by any student-facing handler.
type AdminQuestion struct {
	ID       uuid.UUID `json:"id"`
	TestID   uuid.UUID `json:"test_id"`
	Text     string    `json:"text"`
	Position int       `json:"position"`
	Answers  []Answer  `json:"answers"`
}

type AdminTestDetail struct {
	Test
	Questions []AdminQuestion `json:"questions"`
}

// TestSummary annotates a Test with its resolved parent course title (the
// course it belongs to directly, or via its module/lesson), for admin lists.
type TestSummary struct {
	Test
	CourseTitle string `json:"course_title,omitempty"`
}

type TestInput struct {
	CourseID     *uuid.UUID
	ModuleID     *uuid.UUID
	LessonID     *uuid.UUID
	Title        string
	Description  string
	PassingScore int
	Published    bool
	IsFinal      bool
}

type QuestionInput struct {
	Text string
}

type AnswerInput struct {
	Text      string
	IsCorrect bool
}

type SubmitAnswer struct {
	QuestionID uuid.UUID `json:"question_id"`
	AnswerID   uuid.UUID `json:"answer_id"`
}

type SubmitResult struct {
	AttemptID      uuid.UUID `json:"attempt_id"`
	Score          int       `json:"score"`
	PassingScore   int       `json:"passing_score"`
	Passed         bool      `json:"passed"`
	CorrectAnswers int       `json:"correct_answers"`
	TotalQuestions int       `json:"total_questions"`
}

type Attempt struct {
	ID          uuid.UUID `json:"id"`
	TestID      uuid.UUID `json:"test_id"`
	UserID      uuid.UUID `json:"user_id"`
	Score       int       `json:"score"`
	Passed      bool      `json:"passed"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// AttemptSummary is the shape used by the attempts list endpoint.
type AttemptSummary struct {
	Attempt
	TestTitle string `json:"test_title"`
}

// AttemptAnswerReview reveals the correct answer for a question — safe only
// because the attempt this belongs to is already graded and owned by the
// caller (enforced in the service before this is ever built).
type AttemptAnswerReview struct {
	QuestionID         uuid.UUID `json:"question_id"`
	QuestionText       string    `json:"question_text"`
	SelectedAnswerID   uuid.UUID `json:"selected_answer_id"`
	SelectedAnswerText string    `json:"selected_answer_text"`
	CorrectAnswerID    uuid.UUID `json:"correct_answer_id"`
	CorrectAnswerText  string    `json:"correct_answer_text"`
	Correct            bool      `json:"correct"`
}

type AttemptDetail struct {
	Attempt
	TestTitle      string                `json:"test_title"`
	PassingScore   int                   `json:"passing_score"`
	TotalQuestions int                   `json:"total_questions"`
	Answers        []AttemptAnswerReview `json:"answers"`
}
