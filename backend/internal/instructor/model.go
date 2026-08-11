package instructor

import (
	"time"

	"github.com/google/uuid"
)

// StudentRow is one (student, course) pair for GET /instructor/students —
// every course this instructor teaches, flattened across all their
// students. Deliberately carries no email (see final report on the
// "email only if justified" call).
type StudentRow struct {
	UserID          uuid.UUID  `json:"user_id"`
	DisplayName     string     `json:"display_name"`
	CourseID        uuid.UUID  `json:"course_id"`
	CourseTitle     string     `json:"course_title"`
	EnrolledAt      time.Time  `json:"enrolled_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	ProgressPercent int        `json:"progress_percent"`
	Completed       bool       `json:"completed"`
}

// CourseStudentRow is one student's standing within a single course — the
// shape for GET /instructor/courses/:id/students.
type CourseStudentRow struct {
	UserID           uuid.UUID  `json:"user_id"`
	DisplayName      string     `json:"display_name"`
	EnrolledAt       time.Time  `json:"enrolled_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CompletedLessons int        `json:"completed_lessons"`
	TotalLessons     int        `json:"total_lessons"`
	ProgressPercent  int        `json:"progress_percent"`
	FinalTestPassed  bool       `json:"final_test_passed"`
	Completed        bool       `json:"completed"`
}

// Stats backs GET /instructor/stats — the dashboard-level summary across
// every course this instructor owns. AwaitingReview/NeedsRevision (Stage
// 15 item 21) are the two dashboard cards pointing an instructor at their
// homework queue without having to open every course individually.
type Stats struct {
	CoursesCount              int     `json:"courses_count"`
	PublishedCourses          int     `json:"published_courses"`
	StudentsCount             int     `json:"students_count"`
	ActiveEnrollments         int     `json:"active_enrollments"`
	CompletedEnrollments      int     `json:"completed_enrollments"`
	AverageCompletionPct      float64 `json:"average_completion_percent"`
	CertificatesIssued        int     `json:"certificates_issued"`
	SubmissionsAwaitingReview int     `json:"submissions_awaiting_review"`
	SubmissionsNeedsRevision  int     `json:"submissions_needs_revision"`
}

// CourseStats backs GET /instructor/courses/:id/stats. The Assignment*
// fields are Stage 15's addition (item 26): a course with no assignments
// at all simply reports AssignmentsCount == 0 and zeroes elsewhere.
type CourseStats struct {
	Enrollments           int     `json:"enrollments"`
	CompletionRatePct     float64 `json:"completion_rate_percent"`
	AverageLessonProgress float64 `json:"average_lesson_progress_percent"`
	FinalTestPassRatePct  float64 `json:"final_test_pass_rate_percent"`
	AverageRating         float64 `json:"average_rating"`
	ReviewCount           int     `json:"review_count"`

	AssignmentsCount    int     `json:"assignments_count"`
	SubmittedCount      int     `json:"submitted_count"`
	AwaitingReviewCount int     `json:"awaiting_review_count"`
	ApprovalRatePct     float64 `json:"approval_rate_percent"`
	AverageScore        float64 `json:"average_score"`

	// Coding* fields are Stage 16's addition: a course with no coding
	// exercises simply reports CodingExercisesCount == 0 and zeroes
	// elsewhere, same convention Assignment* above uses.
	CodingExercisesCount          int     `json:"coding_exercises_count"`
	CodeSubmissionsCount          int     `json:"code_submissions_count"`
	CodePassRatePct               float64 `json:"code_pass_rate_percent"`
	CodeAverageAttemptsBeforePass float64 `json:"code_average_attempts_before_pass"`

	// Last7Days* fields are Stage 17's addition (item 18): aggregate,
	// course-wide activity only — deliberately computed from the live
	// lesson_progress/assignment_submissions/code_submissions tables
	// (course-scoped via the same lessons->modules join every other field
	// above already uses), never from learning_activity's per-user streak
	// data, so an instructor never sees any individual student's personal
	// streak or activity calendar through this endpoint.
	ActiveStudentsLast7Days        int     `json:"active_students_last_7_days"`
	LessonsCompletedLast7Days      int     `json:"lessons_completed_last_7_days"`
	AssignmentSubmissionsLast7Days int     `json:"assignment_submissions_last_7_days"`
	CodingSubmissionsLast7Days     int     `json:"coding_submissions_last_7_days"`
	CodingPassRateLast7DaysPct     float64 `json:"coding_pass_rate_last_7_days_percent"`
}

// CourseReview is the read-only shape instructors see for their own
// courses' reviews — same published-only visibility a student gets, plus
// nothing an instructor could use to identify or contact the reviewer
// beyond the display name already public on the course page.
type CourseReview struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Rating      int       `json:"rating"`
	ReviewText  *string   `json:"review_text,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// TestSummary is a lightweight test listing scoped to one course, for the
// course editor's Tests section — unlike tests.TestSummary (admin, global),
// this never leaves the boundary of a single owned course.
type TestSummary struct {
	ID           uuid.UUID  `json:"id"`
	CourseID     *uuid.UUID `json:"course_id,omitempty"`
	ModuleID     *uuid.UUID `json:"module_id,omitempty"`
	LessonID     *uuid.UUID `json:"lesson_id,omitempty"`
	Title        string     `json:"title"`
	PassingScore int        `json:"passing_score"`
	Published    bool       `json:"published"`
	IsFinal      bool       `json:"is_final"`
}
