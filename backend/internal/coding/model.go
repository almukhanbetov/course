package coding

import (
	"time"

	"github.com/google/uuid"
)

var SupportedLanguages = map[string]bool{
	"go":         true,
	"python":     true,
	"javascript": true,
}

const (
	ModeRun    = "run"
	ModeSubmit = "submit"
)

// Submission status machine. "queued"/"running" are transient (owned by
// cmd/code-runner); everything else is terminal — see repository.go's
// ClaimNextJob/MarkSubmissionResult for the only two places that write them.
const (
	StatusQueued        = "queued"
	StatusRunning       = "running"
	StatusPassed        = "passed"
	StatusFailed        = "failed"
	StatusCompileError  = "compile_error"
	StatusRuntimeError  = "runtime_error"
	StatusTimeout       = "timeout"
	StatusInternalError = "internal_error"
)

// TerminalStatuses is what the frontend's poll loop treats as "stop
// polling" — mirrors StatusQueued/StatusRunning being the only non-terminal
// values.
var TerminalStatuses = map[string]bool{
	StatusPassed:        true,
	StatusFailed:        true,
	StatusCompileError:  true,
	StatusRuntimeError:  true,
	StatusTimeout:       true,
	StatusInternalError: true,
}

const (
	JobPending    = "pending"
	JobProcessing = "processing"
	JobCompleted  = "completed"
	JobFailed     = "failed"
)

// Exercise is the full instructor-owned row, including SolutionCode — the
// student-facing service methods must never select this type straight into
// a JSON response (see StudentExercise, which structurally excludes it).
type Exercise struct {
	ID            uuid.UUID `json:"id"`
	LessonID      uuid.UUID `json:"lesson_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Language      string    `json:"language"`
	StarterCode   string    `json:"starter_code"`
	SolutionCode  *string   `json:"solution_code,omitempty"`
	TimeLimitMS   int       `json:"time_limit_ms"`
	MemoryLimitMB int       `json:"memory_limit_mb"`
	Published     bool      `json:"published"`
	Required      bool      `json:"required"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ExerciseInput struct {
	Title         string
	Description   string
	Language      string
	StarterCode   string
	SolutionCode  *string
	TimeLimitMS   int
	MemoryLimitMB int
	Published     bool
	Required      bool
}

// StudentExercise is what GET /lessons/:id/coding-exercise returns —
// mirrors Stage 15's StudentAssignment convention: there simply is no
// SolutionCode/Published/CreatedAt/UpdatedAt field here to leak.
type StudentExercise struct {
	ID            uuid.UUID `json:"id"`
	LessonID      uuid.UUID `json:"lesson_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Language      string    `json:"language"`
	StarterCode   string    `json:"starter_code"`
	Required      bool      `json:"required"`
	TimeLimitMS   int       `json:"time_limit_ms"`
	MemoryLimitMB int       `json:"memory_limit_mb"`
}

// TestCase is the full instructor-owned row (input/expected_output always
// present, hidden or not).
type TestCase struct {
	ID               uuid.UUID `json:"id"`
	CodingExerciseID uuid.UUID `json:"coding_exercise_id"`
	Input            *string   `json:"input,omitempty"`
	ExpectedOutput   string    `json:"expected_output"`
	Position         int       `json:"position"`
	Hidden           bool      `json:"hidden"`
	CreatedAt        time.Time `json:"created_at"`
}

type TestCaseInput struct {
	Input          *string
	ExpectedOutput string
	Position       int
	Hidden         bool
}

// StudentTestCase is the only shape a non-hidden test case is ever
// serialized as to a student — a worked example. Hidden cases never appear
// in any student-facing response at all (see repository.ListVisibleTestCases).
type StudentTestCase struct {
	ID             uuid.UUID `json:"id"`
	Input          *string   `json:"input,omitempty"`
	ExpectedOutput string    `json:"expected_output"`
	Position       int       `json:"position"`
}

// Submission is the full row. Stdout is populated only for mode="run" —
// service.go deliberately nils it out before persisting a mode="submit"
// result, since a submit run executes against hidden test cases and their
// input could otherwise leak back to the student verbatim via echoed
// stdout (see service.go's evaluate()).
type Submission struct {
	ID              uuid.UUID  `json:"id"`
	ExerciseID      uuid.UUID  `json:"exercise_id"`
	UserID          uuid.UUID  `json:"user_id"`
	Language        string     `json:"language"`
	SourceCode      string     `json:"source_code"`
	Mode            string     `json:"mode"`
	Status          string     `json:"status"`
	PassedTests     int        `json:"passed_tests"`
	TotalTests      int        `json:"total_tests"`
	ExecutionTimeMS *int       `json:"execution_time_ms,omitempty"`
	MemoryUsedKB    *int       `json:"memory_used_kb,omitempty"`
	Stdout          *string    `json:"stdout,omitempty"`
	CompileOutput   *string    `json:"compile_output,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

type ExecutionJob struct {
	ID           uuid.UUID  `json:"id"`
	SubmissionID uuid.UUID  `json:"submission_id"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	AvailableAt  time.Time  `json:"available_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	LastError    *string    `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ExecutionResult is what cmd/code-runner produces for one submission and
// hands back to Repository.MarkSubmissionResult — the boundary type between
// runner.go's sandboxed execution and the persisted Submission row.
type ExecutionResult struct {
	Status          string
	PassedTests     int
	TotalTests      int
	ExecutionTimeMS int
	Stdout          string // raw combined stdout across executed cases; caller decides whether to persist it
	CompileOutput   string
}
