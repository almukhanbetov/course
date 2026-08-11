package specialities

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListPublished(ctx context.Context) ([]Speciality, error) {
	return s.repo.ListPublished(ctx)
}

func (s *Service) GetSpecialityDetail(ctx context.Context, id uuid.UUID) (*SpecialityDetail, error) {
	speciality, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	courses, err := s.repo.ListCoursesBySpeciality(ctx, id)
	if err != nil {
		return nil, err
	}

	return &SpecialityDetail{Speciality: *speciality, Courses: courses}, nil
}

// GetMyRoadmap computes overall progress as the average progress_percent
// across every course in the roadmap (required and optional alike), while
// "completed" only depends on the required courses — an elective course can
// be left unfinished without blocking speciality completion.
func (s *Service) GetMyRoadmap(ctx context.Context, userID, specialityID uuid.UUID) (*MyRoadmap, error) {
	speciality, err := s.repo.GetByID(ctx, specialityID)
	if err != nil {
		return nil, err
	}

	courses, err := s.repo.ListCourseProgress(ctx, userID, specialityID)
	if err != nil {
		return nil, err
	}

	sumPercent := 0
	hasRequired := false
	allRequiredCompleted := true
	for _, c := range courses {
		sumPercent += c.ProgressPercent
		if c.Required {
			hasRequired = true
			if !c.Completed {
				allRequiredCompleted = false
			}
		}
	}

	overall := 0
	if len(courses) > 0 {
		overall = sumPercent / len(courses)
	}

	return &MyRoadmap{
		SpecialityID:    speciality.ID,
		Title:           speciality.Title,
		Slug:            speciality.Slug,
		Description:     speciality.Description,
		ProgressPercent: overall,
		Completed:       hasRequired && allRequiredCompleted,
		Courses:         courses,
	}, nil
}

func (s *Service) ListAllAdmin(ctx context.Context) ([]Speciality, error) {
	return s.repo.ListAllAdmin(ctx)
}

func (s *Service) CreateSpeciality(ctx context.Context, input SpecialityInput) (*Speciality, error) {
	if err := validateSpecialityInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateSpeciality(ctx, input)
}

func (s *Service) UpdateSpeciality(ctx context.Context, id uuid.UUID, input SpecialityInput) (*Speciality, error) {
	if err := validateSpecialityInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateSpeciality(ctx, id, input)
}

func (s *Service) DeleteSpeciality(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSpeciality(ctx, id)
}

func (s *Service) AddCourseToSpeciality(ctx context.Context, specialityID, courseID uuid.UUID, required bool) (*SpecialityCourse, error) {
	if _, err := s.repo.GetByID(ctx, specialityID); err != nil {
		return nil, err
	}
	exists, err := s.repo.CourseExists(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrCourseNotFound
	}
	return s.repo.AddCourseToSpeciality(ctx, specialityID, courseID, required)
}

func (s *Service) RemoveCourseFromSpeciality(ctx context.Context, specialityID, courseID uuid.UUID) error {
	return s.repo.RemoveCourseFromSpeciality(ctx, specialityID, courseID)
}

// ReorderSpecialityCourses requires the request to name every course in the
// speciality's roadmap exactly once — see the courses domain's identical
// module/lesson reorder rule for why a partial list isn't safe.
func (s *Service) ReorderSpecialityCourses(ctx context.Context, specialityID uuid.UUID, items []RoadmapReorderItem) error {
	existing, err := s.repo.ListCoursesBySpeciality(ctx, specialityID)
	if err != nil {
		return err
	}

	if len(items) != len(existing) {
		return &ValidationError{Message: "reorder request must include every course in the roadmap exactly once"}
	}

	existingIDs := make(map[uuid.UUID]bool, len(existing))
	for _, c := range existing {
		existingIDs[c.ID] = true
	}

	seenPositions := map[int]bool{}
	for _, it := range items {
		if !existingIDs[it.CourseID] {
			return &ValidationError{Message: "unknown course id in reorder request"}
		}
		if it.Position < 1 {
			return &ValidationError{Message: "position must be >= 1"}
		}
		if seenPositions[it.Position] {
			return &ValidationError{Message: "duplicate position in reorder request"}
		}
		seenPositions[it.Position] = true
	}

	return s.repo.ReorderSpecialityCourses(ctx, specialityID, items)
}

func validateSpecialityInput(input SpecialityInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Message: "title is required"}
	}
	if strings.TrimSpace(input.Slug) == "" {
		return &ValidationError{Message: "slug is required"}
	}
	return nil
}
