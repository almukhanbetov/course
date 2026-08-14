package qa

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"lms-backend/internal/access"
	"lms-backend/internal/audit"
	"lms-backend/internal/logging"
	"lms-backend/internal/ownership"
	"lms-backend/internal/pagination"
)

var (
	ErrNotEnrolled    = errors.New("user is not enrolled in this course")
	ErrAccessRequired = errors.New("user does not currently have access to this course")
	ErrForbidden      = errors.New("user may not answer in this course")
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
	audit     *audit.Service
	access    *access.Service
}

func NewService(repo *Repository, ownershipService *ownership.Service, auditService *audit.Service, accessService *access.Service) *Service {
	return &Service{repo: repo, ownership: ownershipService, audit: auditService, access: accessService}
}

func validateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return &ValidationError{Message: "body is required"}
	}
	return nil
}

// ListForLesson requires the same "enrolled and currently entitled" check
// tests.Service.checkAccess applies to reading a test — Q&A is lesson
// content like any other, and this is the codebase's established pattern
// for gating a *read* of lesson-scoped content (not just the write side,
// which CreateQuestion already gated). Before this check existed, any
// authenticated user could read a lesson's Q&A thread — including
// subscription-gated course content — without ever enrolling (Stage 30B1
// finding).
func (s *Service) ListForLesson(ctx context.Context, userID, lessonID uuid.UUID, page, limit int) (pagination.Result[QuestionView], error) {
	courseID, err := s.ownership.CourseIDForLesson(ctx, lessonID)
	if errors.Is(err, ownership.ErrNotFound) {
		return pagination.Result[QuestionView]{}, ErrLessonMissing
	}
	if err != nil {
		return pagination.Result[QuestionView]{}, err
	}

	enrolled, err := s.repo.IsEnrolled(ctx, userID, courseID)
	if err != nil {
		return pagination.Result[QuestionView]{}, err
	}
	if !enrolled {
		return pagination.Result[QuestionView]{}, ErrNotEnrolled
	}

	canAccess, err := s.access.CanAccessCourse(ctx, userID, courseID)
	if err != nil {
		return pagination.Result[QuestionView]{}, err
	}
	if !canAccess {
		return pagination.Result[QuestionView]{}, ErrAccessRequired
	}

	items, total, err := s.repo.ListForLesson(ctx, lessonID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[QuestionView]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

// ListForLessonModeration is the hide/show-aware counterpart to
// ListForLesson (Stage 21C): requires the same course-ownership check as
// SetQuestionPublished/SetAnswerPublished (admin: any course; instructor:
// only courses they own) before returning hidden content alongside
// published content, so a moderator can actually find and un-hide what
// they previously hid.
func (s *Service) ListForLessonModeration(ctx context.Context, userID uuid.UUID, role string, lessonID uuid.UUID, page, limit int) (pagination.Result[QuestionView], error) {
	courseID, err := s.ownership.CourseIDForLesson(ctx, lessonID)
	if errors.Is(err, ownership.ErrNotFound) {
		return pagination.Result[QuestionView]{}, ErrLessonMissing
	}
	if err != nil {
		return pagination.Result[QuestionView]{}, err
	}

	canManage, err := s.ownership.CanManageCourse(ctx, userID, role, courseID)
	if err != nil {
		return pagination.Result[QuestionView]{}, err
	}
	if !canManage {
		return pagination.Result[QuestionView]{}, ErrForbidden
	}

	items, total, err := s.repo.ListForLessonModeration(ctx, lessonID, limit, pagination.Offset(page, limit))
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

// SetQuestionPublished implements the hide/show moderation action Stage 20
// deferred (see STAGE20_PROGRESS.md's Stage 20B2 section: "hide/show...
// require real backend endpoints that don't exist"). Authorization reuses
// the exact same ownership.CanManageCourse check CreateAnswer already uses
// for its instructor/admin branch: an admin may moderate any course's
// questions (CanManageCourse confirms the course exists), a course-owning
// instructor may moderate only their own course's questions, and anyone
// else — including a non-owning instructor or any student — gets
// ErrForbidden. Never deletes: only flips the existing published flag,
// so ListForLesson's published-only filter is the sole place hidden
// content stops being visible to students.
//
// Stage 25A2: after a successful update, records an audit event via
// logPublishedChange. userID/role are already sourced from the verified
// JWT (see handler.go's currentUser) for the authorization check above —
// the same values are reused for the audit actor, never re-derived from
// anything client-supplied.
func (s *Service) SetQuestionPublished(ctx context.Context, userID uuid.UUID, role string, questionID uuid.UUID, published bool) (*Question, error) {
	question, err := s.repo.GetQuestion(ctx, questionID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	canManage, err := s.ownership.CanManageCourse(ctx, userID, role, question.CourseID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, ErrForbidden
	}

	updated, err := s.repo.SetQuestionPublished(ctx, questionID, published)
	if err != nil {
		return nil, err
	}

	s.logPublishedChange(ctx, userID, role, audit.EntityTypeQuestion, questionID, question.CourseID, published)
	return updated, nil
}

// SetAnswerPublished mirrors SetQuestionPublished for answers, resolving
// the owning course one level deeper via GetAnswerCourseID.
func (s *Service) SetAnswerPublished(ctx context.Context, userID uuid.UUID, role string, answerID uuid.UUID, published bool) (*Answer, error) {
	courseID, err := s.repo.GetAnswerCourseID(ctx, answerID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	canManage, err := s.ownership.CanManageCourse(ctx, userID, role, courseID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, ErrForbidden
	}

	updated, err := s.repo.SetAnswerPublished(ctx, answerID, published)
	if err != nil {
		return nil, err
	}

	s.logPublishedChange(ctx, userID, role, audit.EntityTypeAnswer, answerID, courseID, published)
	return updated, nil
}

// logPublishedChange records a hide/show audit event. A failure to write
// it is logged but never propagated — the real moderation action already
// succeeded and must not be undone or reported as failed just because the
// accountability trail couldn't be appended (matches the roadmap's own
// Stage 25 requirement: "logging failures never block the underlying
// action").
func (s *Service) logPublishedChange(ctx context.Context, actorUserID uuid.UUID, actorRole, entityType string, entityID, courseID uuid.UUID, published bool) {
	action := audit.ActionContentShown
	if !published {
		action = audit.ActionContentHidden
	}

	_, err := s.audit.Log(ctx, audit.LogInput{
		ActorUserID: &actorUserID,
		ActorRole:   &actorRole,
		Action:      action,
		EntityType:  entityType,
		EntityID:    &entityID,
		Metadata:    map[string]any{"course_id": courseID, "published": published},
	})
	if err != nil {
		logging.FromContext(ctx).Error("audit: failed to log", "action", action, "entity_type", entityType, "entity_id", entityID, "error", err)
	}
}
