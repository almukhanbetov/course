package coding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"lms-backend/internal/access"
)

var (
	ErrNotEnrolled       = errors.New("not enrolled in this course")
	ErrAccessRequired    = errors.New("an active subscription is required to access this course")
	ErrLessonUnpublished = errors.New("lesson is not published")
	ErrRateLimited       = errors.New("submission rate limit exceeded")
	ErrTooManyExecutions = errors.New("too many queued or running executions")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Limits is the system-wide ceiling every exercise's own time_limit_ms/
// memory_limit_mb is validated (at authoring time) and clamped (at
// execution time, see runner.go's Worker.clampLimits) against — an
// exercise may ask for less, never more.
type Limits struct {
	SystemTimeoutMS         int
	SystemMemoryMB          int
	MaxSubmissionsPerMinute int
	MaxConcurrentPerUser    int
}

type Service struct {
	repo   *Repository
	access *access.Service
	limits Limits
}

func NewService(repo *Repository, accessService *access.Service, limits Limits) *Service {
	return &Service{repo: repo, access: accessService, limits: limits}
}

func validateExerciseInput(input ExerciseInput, limits Limits) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Message: "title is required"}
	}
	if !SupportedLanguages[input.Language] {
		return &ValidationError{Message: "language must be one of: go, python, javascript"}
	}
	if input.TimeLimitMS <= 0 || input.TimeLimitMS > limits.SystemTimeoutMS {
		return &ValidationError{Message: fmt.Sprintf("time_limit_ms must be between 1 and %d", limits.SystemTimeoutMS)}
	}
	if input.MemoryLimitMB <= 0 || input.MemoryLimitMB > limits.SystemMemoryMB {
		return &ValidationError{Message: fmt.Sprintf("memory_limit_mb must be between 1 and %d", limits.SystemMemoryMB)}
	}
	return nil
}

func validateTestCaseInput(input TestCaseInput) error {
	if input.Position < 0 {
		return &ValidationError{Message: "position must not be negative"}
	}
	return nil
}

// --- instructor CRUD: exercises -------------------------------------------

func (s *Service) CreateExercise(ctx context.Context, lessonID uuid.UUID, input ExerciseInput) (*Exercise, error) {
	if err := validateExerciseInput(input, s.limits); err != nil {
		return nil, err
	}
	return s.repo.CreateExercise(ctx, lessonID, input)
}

func (s *Service) GetExerciseByLesson(ctx context.Context, lessonID uuid.UUID) (*Exercise, error) {
	return s.repo.GetExerciseByLesson(ctx, lessonID)
}

func (s *Service) GetExercise(ctx context.Context, id uuid.UUID) (*Exercise, error) {
	return s.repo.GetExercise(ctx, id)
}

func (s *Service) UpdateExercise(ctx context.Context, id uuid.UUID, input ExerciseInput) (*Exercise, error) {
	if err := validateExerciseInput(input, s.limits); err != nil {
		return nil, err
	}
	return s.repo.UpdateExercise(ctx, id, input)
}

func (s *Service) DeleteExercise(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteExercise(ctx, id)
}

// --- instructor CRUD: test cases -------------------------------------------

func (s *Service) CreateTestCase(ctx context.Context, exerciseID uuid.UUID, input TestCaseInput) (*TestCase, error) {
	if err := validateTestCaseInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateTestCase(ctx, exerciseID, input)
}

func (s *Service) ListTestCases(ctx context.Context, exerciseID uuid.UUID) ([]TestCase, error) {
	return s.repo.ListTestCases(ctx, exerciseID)
}

func (s *Service) UpdateTestCase(ctx context.Context, id uuid.UUID, input TestCaseInput) (*TestCase, error) {
	if err := validateTestCaseInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateTestCase(ctx, id, input)
}

func (s *Service) DeleteTestCase(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteTestCase(ctx, id)
}

// --- student access checks -------------------------------------------------

// checkStudentAccess mirrors assignments.Service.checkStudentAccess exactly
// (enrolled AND active course access) — duplicated per this codebase's
// cross-domain convention rather than importing the assignments package.
func (s *Service) checkStudentAccess(ctx context.Context, userID, courseID uuid.UUID) error {
	enrolled, err := s.repo.IsEnrolled(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if !enrolled {
		return ErrNotEnrolled
	}
	canAccess, err := s.access.CanAccessCourse(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if !canAccess {
		return ErrAccessRequired
	}
	return nil
}

// StudentExerciseView is the full GET /lessons/:id/coding-exercise payload:
// the narrow exercise DTO plus every non-hidden test case as a worked
// example. There is structurally no way for SolutionCode or a hidden
// test's expected_output to end up in here.
type StudentExerciseView struct {
	Exercise StudentExercise   `json:"exercise"`
	Examples []StudentTestCase `json:"examples"`
}

// GetStudentExercise resolves and authorizes the student-facing view of a
// lesson's coding exercise: lesson must exist and be published, the
// student must be enrolled with active course access, and the exercise
// itself must be published — mirrors assignments.Service.GetStudentAssignment.
func (s *Service) GetStudentExercise(ctx context.Context, userID, lessonID uuid.UUID) (*StudentExerciseView, error) {
	lc, err := s.repo.GetLessonContext(ctx, lessonID)
	if err != nil {
		return nil, err
	}
	if !lc.Published {
		return nil, ErrLessonUnpublished
	}
	if err := s.checkStudentAccess(ctx, userID, lc.CourseID); err != nil {
		return nil, err
	}

	e, err := s.repo.GetPublishedExerciseByLesson(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	visible, err := s.repo.ListVisibleTestCases(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	examples := make([]StudentTestCase, len(visible))
	for i, tc := range visible {
		examples[i] = StudentTestCase{ID: tc.ID, Input: tc.Input, ExpectedOutput: tc.ExpectedOutput, Position: tc.Position}
	}

	return &StudentExerciseView{
		Exercise: StudentExercise{
			ID: e.ID, LessonID: e.LessonID, Title: e.Title, Description: e.Description,
			Language: e.Language, StarterCode: e.StarterCode, Required: e.Required,
			TimeLimitMS: e.TimeLimitMS, MemoryLimitMB: e.MemoryLimitMB,
		},
		Examples: examples,
	}, nil
}

// authorizeForExercise re-derives and checks student access from an
// exercise id alone — used by every /coding-exercises/:id/* student route.
func (s *Service) authorizeForExercise(ctx context.Context, userID, exerciseID uuid.UUID) (*Exercise, error) {
	e, err := s.repo.GetExercise(ctx, exerciseID)
	if err != nil {
		return nil, err
	}
	if !e.Published {
		return nil, ErrNotFound
	}
	lc, err := s.repo.GetLessonContext(ctx, e.LessonID)
	if err != nil {
		return nil, err
	}
	if err := s.checkStudentAccess(ctx, userID, lc.CourseID); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Service) checkRateLimit(ctx context.Context, userID uuid.UUID) error {
	recent, err := s.repo.CountRecentSubmissions(ctx, userID, time.Now().Add(-time.Minute))
	if err != nil {
		return err
	}
	if recent >= s.limits.MaxSubmissionsPerMinute {
		return ErrRateLimited
	}
	active, err := s.repo.CountActiveSubmissions(ctx, userID)
	if err != nil {
		return err
	}
	if active >= s.limits.MaxConcurrentPerUser {
		return ErrTooManyExecutions
	}
	return nil
}

// Execute is the shared body of Run and Submit — both authorize, rate-limit
// and enqueue identically; only mode differs (see model.go on why "run" and
// "submit" share one table/status machine instead of being split into two
// endpoints backed by two schemas, per item 18's fallback allowance).
func (s *Service) execute(ctx context.Context, userID, exerciseID uuid.UUID, sourceCode, mode string) (*Submission, error) {
	exercise, err := s.authorizeForExercise(ctx, userID, exerciseID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sourceCode) == "" {
		return nil, &ValidationError{Message: "source_code is required"}
	}
	if err := s.checkRateLimit(ctx, userID); err != nil {
		return nil, err
	}
	return s.repo.CreateSubmission(ctx, exercise.ID, userID, exercise.Language, sourceCode, mode)
}

func (s *Service) Run(ctx context.Context, userID, exerciseID uuid.UUID, sourceCode string) (*Submission, error) {
	return s.execute(ctx, userID, exerciseID, sourceCode, ModeRun)
}

func (s *Service) Submit(ctx context.Context, userID, exerciseID uuid.UUID, sourceCode string) (*Submission, error) {
	return s.execute(ctx, userID, exerciseID, sourceCode, ModeSubmit)
}

// GetSubmission is owner-only (no instructor/admin bypass) — the API
// contract is explicit that GET /code-submissions/:id never serves anyone
// but the student who created it, so a leaked submission id is never
// enough to read someone else's source code or results.
func (s *Service) GetSubmission(ctx context.Context, userID, submissionID uuid.UUID) (*Submission, error) {
	sub, err := s.repo.GetSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	if sub.UserID != userID {
		return nil, ErrNotFound // 404, not 403 — never confirm the submission exists to a non-owner
	}
	return sub, nil
}

// ListMyAttempts backs the attempt-history panel — most recent first,
// capped, and authorized exactly like Run/Submit.
func (s *Service) ListMyAttempts(ctx context.Context, userID, exerciseID uuid.UUID, limit int) ([]Submission, error) {
	if _, err := s.authorizeForExercise(ctx, userID, exerciseID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.ListSubmissionsForUserExercise(ctx, exerciseID, userID, limit)
}
