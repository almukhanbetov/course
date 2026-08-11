// Package activity is the single append-only ledger of "meaningful"
// learning events (Stage 17) — student statistics, the activity calendar,
// streaks, and achievement evaluation all read from learning_activity
// rather than re-deriving state from lesson_progress/assignment_submissions/
// code_submissions/test_attempts/certificates directly. Like
// internal/notifications, its write path (Record) is a package-level
// function taking an Execer so any domain that already holds a pgx.Tx can
// call it without a second round trip or a second transaction.
package activity

import (
	"context"
	"time"

	// Embeds the full IANA time zone database into every binary that
	// imports this package (cmd/api, cmd/code-runner, cmd/backfill-
	// achievements), so time.LoadLocation works regardless of whether the
	// container it's running in ships /usr/share/zoneinfo — none of this
	// repo's Dockerfiles (all Alpine-based) install a `tzdata` package.
	// Empirically confirmed necessary: every real IANA zone name (e.g.
	// "Europe/Moscow") failed LoadLocation inside both the backend and
	// code-runner containers until this import was added.
	_ "time/tzdata"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is satisfied by both *pgxpool.Pool and pgx.Tx — mirrors
// notifications.Execer but adds Query/QueryRow since this package, unlike
// notifications, also needs to read (achievement evaluation counts/streak
// must see this same transaction's own uncommitted writes).
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Activity type constants — Stage 17 item 2's minimum list. Every one of
// these is written exactly once per real state transition; see each
// domain's call site (internal/learning, internal/assignments,
// internal/coding, internal/tests, internal/certificates) for the
// transition it guards and the dedupe_key it uses.
const (
	TypeLessonCompleted      = "lesson_completed"
	TypeAssignmentSubmitted  = "assignment_submitted"
	TypeAssignmentApproved   = "assignment_approved"
	TypeCodingExercisePassed = "coding_exercise_passed"
	TypeTestPassed           = "test_passed"
	TypeCourseCompleted      = "course_completed"
	TypeCertificateIssued    = "certificate_issued"
)

// MeaningfulTypes is what counts as an "active learning day" for streak
// purposes — item 6's exact list (completed lesson / submitted assignment /
// passed coding exercise / passed test). Explicitly excludes
// course_completed/certificate_issued (derived events, not the student's
// own act of learning that day) and, obviously, login.
var MeaningfulTypes = []string{
	TypeLessonCompleted,
	TypeAssignmentSubmitted,
	TypeCodingExercisePassed,
	TypeTestPassed,
}

// RecordInput mirrors notifications.EnqueueInput's shape on purpose — same
// package, same convention, different table.
type RecordInput struct {
	UserID     uuid.UUID
	Type       string
	EntityType string
	EntityID   *uuid.UUID
	Metadata   map[string]any
	DedupeKey  string
}

type Entry struct {
	ID           uuid.UUID      `json:"id"`
	ActivityType string         `json:"activity_type"`
	EntityType   *string        `json:"entity_type,omitempty"`
	EntityID     *uuid.UUID     `json:"entity_id,omitempty"`
	OccurredAt   time.Time      `json:"occurred_at"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// DayCount is one row of the activity calendar (item 5).
type DayCount struct {
	Date          string `json:"date"`
	ActivityCount int    `json:"activity_count"`
}

// Counts backs GET /me/analytics (item 4) and achievement rule evaluation —
// one query, no N+1.
type Counts struct {
	CoursesEnrolled       int `json:"courses_enrolled"`
	CoursesCompleted      int `json:"courses_completed"`
	LessonsCompleted      int `json:"lessons_completed"`
	AssignmentsApproved   int `json:"assignments_approved"`
	CodingExercisesPassed int `json:"coding_exercises_passed"`
	TestsPassed           int `json:"tests_passed"`
	Certificates          int `json:"certificates"`
}

type Stats struct {
	Counts
	CurrentStreak int `json:"current_streak"`
	LongestStreak int `json:"longest_streak"`
}
