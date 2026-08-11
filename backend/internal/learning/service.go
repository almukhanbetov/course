package learning

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"lms-backend/internal/access"
)

var (
	ErrNotEnrolled    = errors.New("not enrolled in this course")
	ErrAccessRequired = errors.New("an active subscription is required to access this course")
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

// EnrollInCourse creates an enrollment, or returns the existing one (created=false)
// if the user is already enrolled. It never creates a duplicate row. A
// subscription-gated course additionally requires a currently active
// subscription — see internal/access — checked here so a student can never
// create an enrollment for a course they aren't entitled to, regardless of
// what any frontend button decided to show them.
func (s *Service) EnrollInCourse(ctx context.Context, userID, courseID uuid.UUID) (enrollment *Enrollment, created bool, err error) {
	exists, err := s.repo.CourseExists(ctx, courseID)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, ErrCourseNotFound
	}

	canAccess, err := s.access.CanAccessCourse(ctx, userID, courseID)
	if err != nil {
		return nil, false, err
	}
	if !canAccess {
		return nil, false, ErrAccessRequired
	}

	enrollment, err = s.repo.CreateEnrollment(ctx, userID, courseID)
	if errors.Is(err, ErrNotFound) {
		existing, getErr := s.repo.GetEnrollment(ctx, userID, courseID)
		if getErr != nil {
			return nil, false, getErr
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return enrollment, true, nil
}

func (s *Service) ListMyCourses(ctx context.Context, userID uuid.UUID) ([]EnrolledCourse, error) {
	return s.repo.ListEnrolledCourses(ctx, userID)
}

func (s *Service) GetContinueLearning(ctx context.Context, userID uuid.UUID) ([]ContinueLearningItem, error) {
	return s.repo.GetContinueLearning(ctx, userID)
}

// GetMyCourseDetail returns the full course content (modules, lessons,
// video/description) — this is the "open the course/lesson content" path,
// so unlike ListMyCourses (the dashboard summary, which stays available
// even after a subscription lapses) it re-checks the access policy on top
// of enrollment.
func (s *Service) GetMyCourseDetail(ctx context.Context, userID, courseID uuid.UUID) (*MyCourseDetail, error) {
	info, err := s.repo.GetEnrolledCourseInfo(ctx, userID, courseID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotEnrolled
	}
	if err != nil {
		return nil, err
	}

	if err := s.requireCourseAccess(ctx, userID, courseID); err != nil {
		return nil, err
	}

	modules, err := s.repo.ListModulesByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	moduleIDs := make([]uuid.UUID, len(modules))
	for i, m := range modules {
		moduleIDs[i] = m.ID
	}

	lessonRows, err := s.repo.ListLessonsWithProgressByModules(ctx, userID, moduleIDs)
	if err != nil {
		return nil, err
	}

	lessonsByModule := make(map[uuid.UUID][]MyLesson, len(modules))
	for _, row := range lessonRows {
		lessonsByModule[row.ModuleID] = append(lessonsByModule[row.ModuleID], row.Lesson)
	}

	totalLessons, completedLessons := 0, 0
	for i := range modules {
		if ls, ok := lessonsByModule[modules[i].ID]; ok {
			modules[i].Lessons = ls
		}
		for _, l := range modules[i].Lessons {
			if !l.Published {
				continue
			}
			totalLessons++
			if l.Completed {
				completedLessons++
			}
		}
	}

	lessonsProgressPercent := calculatePercent(completedLessons, totalLessons)

	return &MyCourseDetail{
		CourseID:         courseID,
		Title:            info.Title,
		Slug:             info.Slug,
		Description:      info.Description,
		Level:            info.Level,
		ImageURL:         info.ImageURL,
		EnrolledAt:       info.EnrolledAt,
		CompletedAt:      info.CompletedAt,
		ProgressPercent:  lessonsProgressPercent,
		CompletedLessons: completedLessons,
		TotalLessons:     totalLessons,
		Modules:          modules,

		LessonsProgressPercent: lessonsProgressPercent,
		HasFinalTest:           info.FinalTestID != nil,
		FinalTestID:            info.FinalTestID,
		FinalTestPassed:        info.FinalTestPassed,
		Completed:              info.CompletedAt != nil,
	}, nil
}

func (s *Service) GetLessonProgress(ctx context.Context, userID, lessonID uuid.UUID) (*LessonProgress, error) {
	lessonCtx, err := s.repo.GetLessonContext(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.GetEnrolledCourseInfo(ctx, userID, lessonCtx.CourseID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotEnrolled
		}
		return nil, err
	}

	if err := s.requireCourseAccess(ctx, userID, lessonCtx.CourseID); err != nil {
		return nil, err
	}

	progress, err := s.repo.GetLessonProgress(ctx, userID, lessonID)
	if errors.Is(err, ErrNotFound) {
		return &LessonProgress{UserID: userID, LessonID: lessonID}, nil
	}
	if err != nil {
		return nil, err
	}
	return progress, nil
}

func (s *Service) UpdateLessonProgress(ctx context.Context, userID, lessonID uuid.UUID, progressSeconds int, completed bool) (*LessonProgress, error) {
	if progressSeconds < 0 {
		return nil, &ValidationError{Message: "progress_seconds must not be negative"}
	}

	lessonCtx, err := s.repo.GetLessonContext(ctx, lessonID)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.GetEnrolledCourseInfo(ctx, userID, lessonCtx.CourseID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotEnrolled
		}
		return nil, err
	}

	if err := s.requireCourseAccess(ctx, userID, lessonCtx.CourseID); err != nil {
		return nil, err
	}

	completed = enforceCompletionThreshold(completed, progressSeconds, lessonCtx.DurationSeconds)

	return s.repo.UpsertLessonProgress(ctx, userID, lessonID, lessonCtx.CourseID, progressSeconds, completed)
}

// enforceCompletionThreshold is the server-side guard against a client
// simply POSTing completed=true after a second of playback. When the
// lesson's admin-set duration is known, completion requires having watched
// at least 90% of it — chosen as a tolerant-but-real threshold that survives
// a player firing "ended" slightly before the nominal duration (rounding,
// trailing silence) without accepting a token effort. When duration is
// unknown (0, never set by the admin), there's nothing to check against, so
// the caller's completed flag is trusted as before Stage 10.
func enforceCompletionThreshold(completed bool, progressSeconds, durationSeconds int) bool {
	if !completed || durationSeconds <= 0 {
		return completed
	}
	threshold := int(float64(durationSeconds) * 0.9)
	return progressSeconds >= threshold
}

// requireCourseAccess re-checks the course's access policy even though the
// caller is already enrolled — enrollment alone isn't enough once a
// subscription course's subscription has expired. Only lesson *content*
// (this and GetLessonProgress) is gated this way; ListMyCourses and
// GetMyCourseDetail deliberately skip this check so a lapsed student can
// still see their dashboard, history, and progress numbers.
func (s *Service) requireCourseAccess(ctx context.Context, userID, courseID uuid.UUID) error {
	canAccess, err := s.access.CanAccessCourse(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if !canAccess {
		return ErrAccessRequired
	}
	return nil
}
