package learning

import (
	"time"

	"github.com/google/uuid"
)

type Enrollment struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	CourseID    uuid.UUID  `json:"course_id"`
	EnrolledAt  time.Time  `json:"enrolled_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type LessonProgress struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	LessonID        uuid.UUID  `json:"lesson_id"`
	ProgressSeconds int        `json:"progress_seconds"`
	Completed       bool       `json:"completed"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// EnrolledCourse is the summary shape used by GET /api/v1/me/courses.
type EnrolledCourse struct {
	CourseID         uuid.UUID  `json:"course_id"`
	Title            string     `json:"title"`
	Slug             string     `json:"slug"`
	ImageURL         string     `json:"image_url"`
	EnrolledAt       time.Time  `json:"enrolled_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	ProgressPercent  int        `json:"progress_percent"`
	CompletedLessons int        `json:"completed_lessons"`
	TotalLessons     int        `json:"total_lessons"`
	// NextLessonID is the first not-yet-completed published lesson (course
	// order), or the very first published lesson if everything is done.
	// Lets the frontend link "Продолжить обучение" straight into /learn.
	NextLessonID *uuid.UUID `json:"next_lesson_id,omitempty"`

	// LessonsProgressPercent duplicates ProgressPercent under an explicit
	// name so clients that care about the final-test rule can't confuse
	// "lessons done" with "course completed" now that the two can diverge.
	LessonsProgressPercent int        `json:"lessons_progress_percent"`
	HasFinalTest           bool       `json:"has_final_test"`
	FinalTestID            *uuid.UUID `json:"final_test_id,omitempty"`
	FinalTestPassed        bool       `json:"final_test_passed"`
	FinalTestBestScore     *int       `json:"final_test_best_score,omitempty"`
	// Completed mirrors CompletedAt != nil as an explicit boolean.
	Completed bool `json:"completed"`
}

// ContinueLearningItem is one row of GET /me/continue-learning (Stage 18
// item 5) — enrolled, not-completed courses only, with the next lesson
// computed server-side (item 6) using the exact same three-factor
// (video/assignment/coding) completion rule as everywhere else, never
// frontend progress state. NextLessonID/NextLessonTitle are both omitted
// (nil) only in the edge case where every published lesson is already
// individually complete but the course-level completed_at hasn't flipped
// yet (e.g. still waiting on a final test) — vanishingly rare, but real.
type ContinueLearningItem struct {
	CourseID        uuid.UUID  `json:"course_id"`
	Title           string     `json:"title"`
	ImageURL        string     `json:"image_url"`
	ProgressPercent int        `json:"progress_percent"`
	NextLessonID    *uuid.UUID `json:"next_lesson_id,omitempty"`
	NextLessonTitle *string    `json:"next_lesson_title,omitempty"`
	LastActivityAt  *time.Time `json:"last_activity_at,omitempty"`
}

// MyLesson is a lesson annotated with the current user's progress, used by
// GET /api/v1/me/courses/:id. It intentionally does not reuse the public
// courses.Lesson type so the public course endpoint stays untouched.
//
// Completed is intentionally not a single independent flag (Stage 15 item
// 18: "один frontend boolean не должен решать completion") — it's computed
// as VideoCompleted && (!AssignmentRequired || AssignmentApproved), so a
// lesson with a required homework assignment isn't "done" from video
// progress alone. Lessons with no required assignment behave exactly as
// before: Completed == VideoCompleted.
type MyLesson struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Description     string    `json:"description"`
	VideoURL        string    `json:"video_url"`
	DurationSeconds int       `json:"duration_seconds"`
	Position        int       `json:"position"`
	IsFree          bool      `json:"is_free"`
	Published       bool      `json:"published"`
	ProgressSeconds int       `json:"progress_seconds"`
	Completed       bool      `json:"completed"`
	// VideoStatus is populated from lesson_videos (Stage 10 object storage
	// flow) — "ready" means the frontend should fetch a signed URL and show
	// a player; empty/absent means no video has been uploaded yet.
	VideoStatus *string `json:"video_status,omitempty"`

	VideoCompleted     bool    `json:"video_completed"`
	AssignmentRequired bool    `json:"assignment_required"`
	AssignmentApproved bool    `json:"assignment_approved"`
	AssignmentStatus   *string `json:"assignment_status,omitempty"`

	// CodingExercisePassed is true once at least one submit-mode submission
	// has ever passed — unlike AssignmentApproved it doesn't track the
	// *latest* submission (CodingExerciseStatus does that, for display),
	// since a later failed re-submit must never un-complete a lesson.
	CodingExerciseRequired bool    `json:"coding_exercise_required"`
	CodingExercisePassed   bool    `json:"coding_exercise_passed"`
	CodingExerciseStatus   *string `json:"coding_exercise_status,omitempty"`
}

type MyModule struct {
	ID       uuid.UUID  `json:"id"`
	Title    string     `json:"title"`
	Position int        `json:"position"`
	Lessons  []MyLesson `json:"lessons"`
}

type MyCourseDetail struct {
	CourseID         uuid.UUID  `json:"course_id"`
	Title            string     `json:"title"`
	Slug             string     `json:"slug"`
	Description      string     `json:"description"`
	Level            string     `json:"level"`
	ImageURL         string     `json:"image_url"`
	EnrolledAt       time.Time  `json:"enrolled_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	ProgressPercent  int        `json:"progress_percent"`
	CompletedLessons int        `json:"completed_lessons"`
	TotalLessons     int        `json:"total_lessons"`
	Modules          []MyModule `json:"modules"`

	LessonsProgressPercent int        `json:"lessons_progress_percent"`
	HasFinalTest           bool       `json:"has_final_test"`
	FinalTestID            *uuid.UUID `json:"final_test_id,omitempty"`
	FinalTestPassed        bool       `json:"final_test_passed"`
	Completed              bool       `json:"completed"`
}
