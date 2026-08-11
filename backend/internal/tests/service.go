package tests

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"lms-backend/internal/access"
)

var (
	ErrNotEnrolled         = errors.New("not enrolled in this course")
	ErrLessonsNotCompleted = errors.New("lessons must be completed before taking the final test")
	ErrInvalidAnswer       = errors.New("answer does not belong to the given question in this test")
	ErrForbidden           = errors.New("you may only view your own attempts")
	ErrAccessRequired      = errors.New("an active subscription is required to access this test")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type Service struct {
	repo   *Repository
	access *access.Service
}

func NewService(repo *Repository, accessService *access.Service) *Service {
	return &Service{repo: repo, access: accessService}
}

// checkAccess verifies the caller is enrolled in the test's owning course,
// currently entitled to that course's access policy (a subscription course
// requires a still-active subscription — see internal/access), and, for a
// final test, has completed every published lesson first. It is shared by
// both the read path (GetTest) and the write path (SubmitTest) so the rule
// can't be bypassed by only guarding one of them.
func (s *Service) checkAccess(ctx context.Context, userID uuid.UUID, tc *TestContext) error {
	if tc.CourseID != nil {
		enrolled, err := s.repo.IsEnrolled(ctx, userID, *tc.CourseID)
		if err != nil {
			return err
		}
		if !enrolled {
			return ErrNotEnrolled
		}

		canAccess, err := s.access.CanAccessCourse(ctx, userID, *tc.CourseID)
		if err != nil {
			return err
		}
		if !canAccess {
			return ErrAccessRequired
		}
	}

	if tc.Test.IsFinal {
		if tc.CourseID == nil {
			return ErrLessonsNotCompleted
		}
		allDone, err := s.repo.AllPublishedLessonsCompleted(ctx, userID, *tc.CourseID)
		if err != nil {
			return err
		}
		if !allDone {
			return ErrLessonsNotCompleted
		}
	}

	return nil
}

func (s *Service) GetTest(ctx context.Context, userID, testID uuid.UUID) (*PublicTest, error) {
	tc, err := s.repo.GetTestContext(ctx, testID)
	if err != nil {
		return nil, err
	}

	if err := s.checkAccess(ctx, userID, tc); err != nil {
		return nil, err
	}

	questions, err := s.repo.ListPublicQuestions(ctx, testID)
	if err != nil {
		return nil, err
	}

	return &PublicTest{
		ID:           tc.Test.ID,
		Title:        tc.Test.Title,
		Description:  tc.Test.Description,
		PassingScore: tc.Test.PassingScore,
		IsFinal:      tc.Test.IsFinal,
		Questions:    questions,
	}, nil
}

func (s *Service) SubmitTest(ctx context.Context, userID, testID uuid.UUID, submitted []SubmitAnswer) (*SubmitResult, error) {
	tc, err := s.repo.GetTestContext(ctx, testID)
	if err != nil {
		return nil, err
	}

	if err := s.checkAccess(ctx, userID, tc); err != nil {
		return nil, err
	}

	answerRows, err := s.repo.ListAnswerRowsForTest(ctx, testID)
	if err != nil {
		return nil, err
	}
	if len(answerRows) == 0 {
		return nil, &ValidationError{Message: "this test has no questions"}
	}

	questionText := map[uuid.UUID]string{}
	answerQuestion := map[uuid.UUID]uuid.UUID{}
	correctAnswerID := map[uuid.UUID]uuid.UUID{}
	for _, row := range answerRows {
		questionText[row.QuestionID] = row.QuestionText
		answerQuestion[row.AnswerID] = row.QuestionID
		if row.IsCorrect {
			correctAnswerID[row.QuestionID] = row.AnswerID
		}
	}
	totalQuestions := len(questionText)

	if len(submitted) != totalQuestions {
		return nil, &ValidationError{Message: "all questions must be answered"}
	}

	seen := map[uuid.UUID]bool{}
	graded := make([]gradedAnswer, 0, len(submitted))

	for _, sub := range submitted {
		owningQuestion, ok := answerQuestion[sub.AnswerID]
		if !ok || owningQuestion != sub.QuestionID {
			return nil, ErrInvalidAnswer
		}
		if seen[sub.QuestionID] {
			return nil, &ValidationError{Message: "duplicate answer for the same question"}
		}
		seen[sub.QuestionID] = true

		graded = append(graded, gradedAnswer{
			QuestionID: sub.QuestionID,
			AnswerID:   sub.AnswerID,
			Correct:    correctAnswerID[sub.QuestionID] == sub.AnswerID,
		})
	}

	correctCount := 0
	for _, g := range graded {
		if g.Correct {
			correctCount++
		}
	}

	score := calculatePercent(correctCount, totalQuestions)
	passed := score >= tc.Test.PassingScore

	attempt, err := s.repo.SaveAttempt(ctx, userID, testID, score, passed, graded)
	if err != nil {
		return nil, err
	}

	if tc.CourseID != nil {
		if err := s.repo.RecalculateCourseCompletion(ctx, userID, *tc.CourseID); err != nil {
			return nil, err
		}
	}

	return &SubmitResult{
		AttemptID:      attempt.ID,
		Score:          score,
		PassingScore:   tc.Test.PassingScore,
		Passed:         passed,
		CorrectAnswers: correctCount,
		TotalQuestions: totalQuestions,
	}, nil
}

func (s *Service) ListMyAttempts(ctx context.Context, userID uuid.UUID) ([]AttemptSummary, error) {
	return s.repo.ListAttemptsByUser(ctx, userID)
}

func (s *Service) GetMyAttemptDetail(ctx context.Context, userID, attemptID uuid.UUID) (*AttemptDetail, error) {
	attempt, testTitle, passingScore, err := s.repo.GetAttemptWithTest(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if attempt.UserID != userID {
		return nil, ErrForbidden
	}

	answers, err := s.repo.ListAttemptAnswerReviews(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	return &AttemptDetail{
		Attempt:        *attempt,
		TestTitle:      testTitle,
		PassingScore:   passingScore,
		TotalQuestions: len(answers),
		Answers:        answers,
	}, nil
}

func calculatePercent(correct, total int) int {
	if total <= 0 {
		return 0
	}
	return int(float64(correct) / float64(total) * 100)
}

// --- Admin CRUD -------------------------------------------------------

func (s *Service) ListTestsAdmin(ctx context.Context) ([]TestSummary, error) {
	return s.repo.ListTestsAdmin(ctx)
}

func (s *Service) GetAdminTestDetail(ctx context.Context, id uuid.UUID) (*AdminTestDetail, error) {
	return s.repo.GetAdminTestDetail(ctx, id)
}

func (s *Service) CreateTest(ctx context.Context, input TestInput) (*Test, error) {
	if err := validateTestInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateTest(ctx, input)
}

func (s *Service) UpdateTest(ctx context.Context, id uuid.UUID, input TestInput) (*Test, error) {
	if err := validateTestInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateTest(ctx, id, input)
}

func (s *Service) DeleteTest(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteTest(ctx, id)
}

func (s *Service) CreateQuestion(ctx context.Context, testID uuid.UUID, input QuestionInput) (*Question, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, &ValidationError{Message: "text is required"}
	}
	if _, err := s.repo.GetTest(ctx, testID); err != nil {
		return nil, err
	}
	return s.repo.CreateQuestion(ctx, testID, input)
}

func (s *Service) UpdateQuestion(ctx context.Context, id uuid.UUID, input QuestionInput) (*Question, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, &ValidationError{Message: "text is required"}
	}
	return s.repo.UpdateQuestion(ctx, id, input)
}

func (s *Service) DeleteQuestion(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteQuestion(ctx, id)
}

func (s *Service) CreateAnswer(ctx context.Context, questionID uuid.UUID, input AnswerInput) (*Answer, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, &ValidationError{Message: "text is required"}
	}
	if _, err := s.repo.GetQuestion(ctx, questionID); err != nil {
		return nil, err
	}
	return s.repo.CreateAnswer(ctx, questionID, input)
}

func (s *Service) UpdateAnswer(ctx context.Context, id uuid.UUID, input AnswerInput) (*Answer, error) {
	if strings.TrimSpace(input.Text) == "" {
		return nil, &ValidationError{Message: "text is required"}
	}
	return s.repo.UpdateAnswer(ctx, id, input)
}

func (s *Service) DeleteAnswer(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteAnswer(ctx, id)
}

func validateTestInput(input TestInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Message: "title is required"}
	}
	if input.PassingScore < 0 || input.PassingScore > 100 {
		return &ValidationError{Message: "passing_score must be between 0 and 100"}
	}

	parents := 0
	if input.CourseID != nil {
		parents++
	}
	if input.ModuleID != nil {
		parents++
	}
	if input.LessonID != nil {
		parents++
	}
	if parents != 1 {
		return &ValidationError{Message: "a test must belong to exactly one of course_id, module_id, or lesson_id"}
	}

	return nil
}
