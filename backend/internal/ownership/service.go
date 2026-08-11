// Package ownership is the single, centralized place that decides whether a
// user may manage (create/edit/delete content in) a course — as opposed to
// package access, which decides whether a student may consume a course's
// content. Every instructor-facing write in this codebase resolves the
// target's owning course through here before touching anything, instead of
// re-deriving the "is this mine" chain (lesson -> module -> course ->
// instructor) inline in each handler.
//
// Like package access, this queries the shared schema directly rather than
// importing the courses/tests packages.
package ownership

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("resource not found")

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CanManageCourse is the one rule: admins can manage every course; an
// instructor can manage only a course whose instructor_id is their own; a
// student can manage none. Returns (false, nil) for "no", never an error,
// so callers can respond 403 uniformly regardless of *why* the answer is no.
func (s *Service) CanManageCourse(ctx context.Context, userID uuid.UUID, role string, courseID uuid.UUID) (bool, error) {
	if role == "admin" {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1)`, courseID).Scan(&exists); err != nil {
			return false, err
		}
		return exists, nil
	}
	if role != "instructor" {
		return false, nil
	}

	var owns bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1 AND instructor_id = $2)
	`, courseID, userID).Scan(&owns)
	return owns, err
}

// CourseIDForModule resolves the course a module belongs to, for the IDOR
// chain check on module-scoped writes (update/delete/reorder).
func (s *Service) CourseIDForModule(ctx context.Context, moduleID uuid.UUID) (uuid.UUID, error) {
	var courseID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT course_id FROM modules WHERE id = $1`, moduleID).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	return courseID, err
}

// CourseIDForLesson resolves the course a lesson belongs to via its module —
// used for lesson writes and for gating video upload/replace/delete.
func (s *Service) CourseIDForLesson(ctx context.Context, lessonID uuid.UUID) (uuid.UUID, error) {
	var courseID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT m.course_id FROM lessons l JOIN modules m ON m.id = l.module_id WHERE l.id = $1
	`, lessonID).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	return courseID, err
}

// CourseIDForTest resolves the course a test ultimately belongs to,
// regardless of whether it's attached directly to a course, to one of its
// modules, or to one of its lessons — mirrors tests.Repository.GetTestContext's
// resolution, duplicated here since ownership never imports the tests package.
func (s *Service) CourseIDForTest(ctx context.Context, testID uuid.UUID) (uuid.UUID, error) {
	var courseID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(t.course_id, m.course_id, lm.course_id)
		FROM tests t
		LEFT JOIN modules m ON m.id = t.module_id
		LEFT JOIN lessons l ON l.id = t.lesson_id
		LEFT JOIN modules lm ON lm.id = l.module_id
		WHERE t.id = $1
	`, testID).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	return courseID, err
}

// CourseIDForQuestion resolves the course a question's test belongs to.
func (s *Service) CourseIDForQuestion(ctx context.Context, questionID uuid.UUID) (uuid.UUID, error) {
	var testID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT test_id FROM questions WHERE id = $1`, questionID).Scan(&testID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.CourseIDForTest(ctx, testID)
}

// CourseIDForAnswer resolves the course an answer's question/test belongs to.
func (s *Service) CourseIDForAnswer(ctx context.Context, answerID uuid.UUID) (uuid.UUID, error) {
	var questionID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT question_id FROM answers WHERE id = $1`, answerID).Scan(&questionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.CourseIDForQuestion(ctx, questionID)
}

// CourseIDForAssignment resolves the course an assignment's lesson belongs to.
func (s *Service) CourseIDForAssignment(ctx context.Context, assignmentID uuid.UUID) (uuid.UUID, error) {
	var lessonID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT lesson_id FROM assignments WHERE id = $1`, assignmentID).Scan(&lessonID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.CourseIDForLesson(ctx, lessonID)
}

// CourseIDForSubmission resolves the course a submission's assignment
// belongs to — used to gate instructor review/read of one student's
// homework to only the courses that instructor actually owns.
func (s *Service) CourseIDForSubmission(ctx context.Context, submissionID uuid.UUID) (uuid.UUID, error) {
	var assignmentID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT assignment_id FROM assignment_submissions WHERE id = $1`, submissionID).Scan(&assignmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.CourseIDForAssignment(ctx, assignmentID)
}

// CourseIDForCodingExercise resolves the course a coding exercise's lesson
// belongs to.
func (s *Service) CourseIDForCodingExercise(ctx context.Context, exerciseID uuid.UUID) (uuid.UUID, error) {
	var lessonID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT lesson_id FROM coding_exercises WHERE id = $1`, exerciseID).Scan(&lessonID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.CourseIDForLesson(ctx, lessonID)
}

// CourseIDForTestCase resolves the course a coding test case's exercise
// belongs to — used to gate instructor test-case CRUD (item 21's IDOR
// concern applies here exactly as it does to assignments).
func (s *Service) CourseIDForTestCase(ctx context.Context, testCaseID uuid.UUID) (uuid.UUID, error) {
	var exerciseID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT coding_exercise_id FROM coding_test_cases WHERE id = $1`, testCaseID).Scan(&exerciseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.CourseIDForCodingExercise(ctx, exerciseID)
}

// CourseIDForCodeSubmission resolves the course a code submission's
// exercise belongs to — used to gate a foreign instructor out of another
// instructor's submissions if instructor-scoped submission viewing is ever
// added; currently used only by ownership checks that need it directly.
func (s *Service) CourseIDForCodeSubmission(ctx context.Context, submissionID uuid.UUID) (uuid.UUID, error) {
	var exerciseID uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT exercise_id FROM code_submissions WHERE id = $1`, submissionID).Scan(&exerciseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, err
	}
	return s.CourseIDForCodingExercise(ctx, exerciseID)
}
