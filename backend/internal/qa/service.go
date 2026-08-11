package qa

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"lms-backend/internal/ownership"
	"lms-backend/internal/pagination"
)

var (
	ErrNotEnrolled = errors.New("user is not enrolled in this course")
	ErrForbidden   = errors.New("user may not answer in this course")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Service depends on ownership.Service directly (not through a duplicated
// query) — ownership is this codebase's one deliberate exception to
// "share schema, not code" for course-ownership resolution, already
// imported the same way by internal/assignments, internal/coding and
// internal/instructor.
type Service struct {
	repo      *Repository
	ownership *ownership.Service
}

func NewService(repo *Repository, ownershipService *ownership.Service) *Service {
	return &Service{repo: repo, ownership: ownershipService}
}

func validateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return &ValidationError{Message: "body is required"}
	}
	return nil
}

func (s *Service) ListForLesson(ctx context.Context, lessonID uuid.UUID, page, limit int) (pagination.Result[QuestionView], error) {
	items, total, err := s.repo.ListForLesson(ctx, lessonID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[QuestionView]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

// CreateQuestion enforces "asking requires enrollment" (Stage 20 planning
// decision, consistent with internal/reviews' eligibility model): resolves
// the lesson's course via ownership.CourseIDForLesson (404 if the lesson
// itself doesn't exist), then requires the caller be enrolled in that
// course (403 otherwise) before writing anything.
func (s *Service) CreateQuestion(ctx context.Context, userID, lessonID uuid.UUID, input QuestionInput) (*Question, error) {
	if err := validateBody(input.Body); err != nil {
		return nil, err
	}

	courseID, err := s.ownership.CourseIDForLesson(ctx, lessonID)
	if errors.Is(err, ownership.ErrNotFound) {
		return nil, ErrLessonMissing
	}
	if err != nil {
		return nil, err
	}

	enrolled, err := s.repo.IsEnrolled(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	if !enrolled {
		return nil, ErrNotEnrolled
	}

	return s.repo.CreateQuestion(ctx, lessonID, courseID, userID, input.Body)
}

// CreateAnswer implements the three-way authorization rule from the Stage
// 20 plan: an admin may always answer; a course-owning instructor may
// answer their own course's questions (via ownership.CanManageCourse, the
// same check every other instructor-facing write in this codebase uses);
// an enrolled student may answer questions in a course they're enrolled
// in. Anyone else gets ErrForbidden. IsInstructorAnswer is set from
// whichever of the first two conditions matched, not from a second query.
func (s *Service) CreateAnswer(ctx context.Context, userID uuid.UUID, role string, questionID uuid.UUID, input AnswerInput) (*Answer, error) {
	if err := validateBody(input.Body); err != nil {
		return nil, err
	}

	question, err := s.repo.GetQuestion(ctx, questionID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	isInstructorAnswer := false
	authorized := false

	if role == "admin" {
		authorized = true
		isInstructorAnswer = true
	} else {
		canManage, err := s.ownership.CanManageCourse(ctx, userID, role, question.CourseID)
		if err != nil {
			return nil, err
		}
		if canManage {
			authorized = true
			isInstructorAnswer = true
		} else {
			enrolled, err := s.repo.IsEnrolled(ctx, userID, question.CourseID)
			if err != nil {
				return nil, err
			}
			authorized = enrolled
		}
	}

	if !authorized {
		return nil, ErrForbidden
	}

	return s.repo.CreateAnswer(ctx, questionID, userID, input.Body, isInstructorAnswer)
}

func (s *Service) DeleteQuestion(ctx context.Context, userID, questionID uuid.UUID) error {
	return s.repo.DeleteQuestion(ctx, questionID, userID)
}

func (s *Service) DeleteAnswer(ctx context.Context, userID, answerID uuid.UUID) error {
	return s.repo.DeleteAnswer(ctx, answerID, userID)
}
