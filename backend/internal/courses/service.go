package courses

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"lms-backend/internal/pagination"
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

// allowedSorts is the whitelist for the public ?sort= param — the
// repository maps each of these to a fixed, hardcoded ORDER BY clause, so
// nothing here ever reaches SQL as raw, attacker-controlled text.
var allowedSorts = map[string]bool{
	"relevance": true,
	"newest":    true,
	"rating":    true,
	"title":     true,
}

// SearchCourses backs GET /courses: full-text search, category/level/access
// filters, whitelisted sort, and pagination all in one query.
func (s *Service) SearchCourses(ctx context.Context, params ListCoursesParams) (pagination.Result[Course], error) {
	if params.Level != "" && !AllowedLevels[params.Level] {
		return pagination.Result[Course]{}, &ValidationError{Message: "level must be one of: beginner, intermediate, advanced"}
	}
	if params.AccessType != "" && !AllowedAccessTypes[params.AccessType] {
		return pagination.Result[Course]{}, &ValidationError{Message: "access_type must be one of: free, subscription"}
	}
	if params.Sort != "" && !allowedSorts[params.Sort] {
		return pagination.Result[Course]{}, &ValidationError{Message: "sort must be one of: relevance, newest, rating, title"}
	}

	params.Query = strings.TrimSpace(params.Query)
	if params.Sort == "" {
		// relevance only means something once there's a query to rank
		// against; otherwise fall back to the more useful default.
		if params.Query != "" {
			params.Sort = "relevance"
		} else {
			params.Sort = "newest"
		}
	}

	items, total, err := s.repo.SearchCourses(ctx, params)
	if err != nil {
		return pagination.Result[Course]{}, err
	}
	return pagination.New(items, params.Page, params.Limit, total), nil
}

// suggestionLimit is fixed, not caller-controlled — a suggestion dropdown
// has no pagination UI, so there is no legitimate reason for a client to
// ask for more than this (roadmap's own "top 5-8" target).
const suggestionLimit = 8

// suggestionQueryMaxRunes bounds input length before it ever reaches SQL.
// A per-keystroke endpoint has no natural request-size limit otherwise,
// and nothing meaningful can be suggested from a query this long anyway.
const suggestionQueryMaxRunes = 100

// SuggestCourses backs search-as-you-type: unlike SearchCourses, an empty
// or whitespace-only query deliberately returns zero suggestions rather
// than "browse everything" — a dropdown with no typed input yet has
// nothing useful to suggest, whereas SearchCourses' empty-query browse
// behavior exists for the full catalog page. Overlong input is truncated
// to suggestionQueryMaxRunes before reaching the repository, so a
// pathological client can't force an expensive query via a huge string.
func (s *Service) SuggestCourses(ctx context.Context, query string) ([]CourseSuggestion, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []CourseSuggestion{}, nil
	}
	if runes := []rune(query); len(runes) > suggestionQueryMaxRunes {
		query = string(runes[:suggestionQueryMaxRunes])
	}

	return s.repo.SuggestCourses(ctx, query, suggestionLimit)
}

func (s *Service) GetCourseDetail(ctx context.Context, id uuid.UUID) (*CourseDetail, error) {
	course, err := s.repo.GetCourse(ctx, id)
	if err != nil {
		return nil, err
	}

	modules, err := s.repo.ListModulesByCourse(ctx, id)
	if err != nil {
		return nil, err
	}

	moduleIDs := make([]uuid.UUID, len(modules))
	for i, m := range modules {
		moduleIDs[i] = m.ID
	}

	lessons, err := s.repo.ListLessonsByModules(ctx, moduleIDs)
	if err != nil {
		return nil, err
	}

	// VideoURL is the legacy, pre-internal/videos field (Stage 2) — every
	// current lesson video goes through internal/videos' own
	// enrollment/subscription-checked, presigned-URL delivery instead, but
	// this column is still live and admin-writable. Stripped here for any
	// lesson that isn't a free preview: this is a public, unauthenticated
	// endpoint, and without this it would hand out a playable link to
	// paid-course video content with none of internal/videos' access
	// checks (Stage 30B1 finding). Every other field is left as-is —
	// description/title etc. are not access-controlled elsewhere either,
	// only this field is a direct, working link to the asset itself.
	lessonsByModule := make(map[uuid.UUID][]Lesson, len(modules))
	for _, l := range lessons {
		if !l.IsFree {
			l.VideoURL = ""
		}
		lessonsByModule[l.ModuleID] = append(lessonsByModule[l.ModuleID], l)
	}

	for i := range modules {
		if ls, ok := lessonsByModule[modules[i].ID]; ok {
			modules[i].Lessons = ls
		}
	}

	return &CourseDetail{Course: *course, Modules: modules}, nil
}

func (s *Service) CreateCourse(ctx context.Context, input CourseInput) (*Course, error) {
	if err := validateCourseInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateCourse(ctx, input)
}

func (s *Service) UpdateCourse(ctx context.Context, id uuid.UUID, input CourseInput) (*Course, error) {
	if err := validateCourseInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateCourse(ctx, id, input)
}

func (s *Service) DeleteCourse(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteCourse(ctx, id)
}

func validateCourseInput(input CourseInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Message: "title is required"}
	}
	if strings.TrimSpace(input.Slug) == "" {
		return &ValidationError{Message: "slug is required"}
	}
	if !AllowedLevels[input.Level] {
		return &ValidationError{Message: "level must be one of: beginner, intermediate, advanced"}
	}
	if !AllowedAccessTypes[input.AccessType] {
		return &ValidationError{Message: "access_type must be one of: free, subscription"}
	}
	return nil
}

func (s *Service) ListAllAdmin(ctx context.Context, search string, page, limit int) ([]Course, int, error) {
	return s.repo.ListAllAdmin(ctx, search, limit, (page-1)*limit)
}

// --- instructor authoring workflow ------------------------------------

func validateInstructorCourseInput(input InstructorCourseInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Message: "title is required"}
	}
	if strings.TrimSpace(input.Slug) == "" {
		return &ValidationError{Message: "slug is required"}
	}
	if !AllowedLevels[input.Level] {
		return &ValidationError{Message: "level must be one of: beginner, intermediate, advanced"}
	}
	if !AllowedAccessTypes[input.AccessType] {
		return &ValidationError{Message: "access_type must be one of: free, subscription"}
	}
	return nil
}

func (s *Service) ListByInstructor(ctx context.Context, instructorID uuid.UUID, page, limit int) (pagination.Result[Course], error) {
	items, total, err := s.repo.ListByInstructor(ctx, instructorID, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[Course]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

func (s *Service) CreateForInstructor(ctx context.Context, instructorID uuid.UUID, input InstructorCourseInput) (*Course, error) {
	if err := validateInstructorCourseInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateForInstructor(ctx, instructorID, input)
}

func (s *Service) UpdateForInstructor(ctx context.Context, id, instructorID uuid.UUID, input InstructorCourseInput) (*Course, error) {
	if err := validateInstructorCourseInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateForInstructor(ctx, id, instructorID, input)
}

func (s *Service) SubmitForReview(ctx context.Context, id, instructorID uuid.UUID) (*Course, error) {
	return s.repo.SubmitForReview(ctx, id, instructorID)
}

// --- admin moderation --------------------------------------------------

func (s *Service) ListPendingReview(ctx context.Context, page, limit int) (pagination.Result[Course], error) {
	items, total, err := s.repo.ListPendingReview(ctx, limit, pagination.Offset(page, limit))
	if err != nil {
		return pagination.Result[Course]{}, err
	}
	return pagination.New(items, page, limit, total), nil
}

// SetPublicationStatus is the admin approve/reject action. A rejection must
// carry a reason — that's the whole point of rejecting instead of just
// leaving the course in pending_review — so it's validated here rather than
// left to the database CHECK constraint (which only knows about the enum,
// not this cross-field rule).
func (s *Service) SetPublicationStatus(ctx context.Context, id uuid.UUID, status string, rejectionReason string) (*Course, error) {
	if status != "published" && status != "rejected" {
		return nil, &ValidationError{Message: "status must be one of: published, rejected"}
	}
	var reason *string
	if status == "rejected" {
		if strings.TrimSpace(rejectionReason) == "" {
			return nil, &ValidationError{Message: "rejection_reason is required when rejecting a course"}
		}
		reason = &rejectionReason
	}
	return s.repo.SetPublicationStatus(ctx, id, status, reason)
}

func (s *Service) CreateModule(ctx context.Context, courseID uuid.UUID, input ModuleInput) (*Module, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, &ValidationError{Message: "title is required"}
	}
	if _, err := s.repo.GetCourse(ctx, courseID); err != nil {
		return nil, err
	}
	return s.repo.CreateModule(ctx, courseID, input)
}

func (s *Service) UpdateModule(ctx context.Context, id uuid.UUID, input ModuleInput) (*Module, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, &ValidationError{Message: "title is required"}
	}
	return s.repo.UpdateModule(ctx, id, input)
}

func (s *Service) DeleteModule(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteModule(ctx, id)
}

type ReorderItem struct {
	ID       uuid.UUID
	Position int
}

// ReorderModules requires the request to name every module belonging to the
// course exactly once — a partial reorder could otherwise leave the
// untouched rows' positions colliding with the newly-assigned ones.
func (s *Service) ReorderModules(ctx context.Context, courseID uuid.UUID, items []ReorderItem) error {
	existing, err := s.repo.ListModulesByCourse(ctx, courseID)
	if err != nil {
		return err
	}

	positionItems, err := validateReorder(existing2IDs(existing), items)
	if err != nil {
		return err
	}

	return s.repo.ReorderModules(ctx, courseID, positionItems)
}

func (s *Service) CreateLesson(ctx context.Context, moduleID uuid.UUID, input LessonInput) (*Lesson, error) {
	if err := validateLessonInput(input); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetModule(ctx, moduleID); err != nil {
		return nil, err
	}
	return s.repo.CreateLesson(ctx, moduleID, input)
}

func (s *Service) UpdateLesson(ctx context.Context, id uuid.UUID, input LessonInput) (*Lesson, error) {
	if err := validateLessonInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateLesson(ctx, id, input)
}

func (s *Service) DeleteLesson(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteLesson(ctx, id)
}

func (s *Service) ReorderLessons(ctx context.Context, moduleID uuid.UUID, items []ReorderItem) error {
	module, err := s.repo.GetModule(ctx, moduleID)
	if err != nil {
		return err
	}

	courseDetail, err := s.repo.ListLessonsByModules(ctx, []uuid.UUID{module.ID})
	if err != nil {
		return err
	}

	positionItems, err := validateReorder(existing2LessonIDs(courseDetail), items)
	if err != nil {
		return err
	}

	return s.repo.ReorderLessons(ctx, moduleID, positionItems)
}

func validateLessonInput(input LessonInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Message: "title is required"}
	}
	if strings.TrimSpace(input.Slug) == "" {
		return &ValidationError{Message: "slug is required"}
	}
	if input.DurationSeconds < 0 {
		return &ValidationError{Message: "duration_seconds must not be negative"}
	}
	return nil
}

func existing2IDs(modules []Module) map[uuid.UUID]bool {
	ids := make(map[uuid.UUID]bool, len(modules))
	for _, m := range modules {
		ids[m.ID] = true
	}
	return ids
}

func existing2LessonIDs(lessons []Lesson) map[uuid.UUID]bool {
	ids := make(map[uuid.UUID]bool, len(lessons))
	for _, l := range lessons {
		ids[l.ID] = true
	}
	return ids
}

// validateReorder checks that items is exactly a permutation of
// existingIDs with distinct, positive positions — see ReorderModules for why
// a partial list isn't safe to accept.
func validateReorder(existingIDs map[uuid.UUID]bool, items []ReorderItem) ([]PositionItem, error) {
	if len(items) != len(existingIDs) {
		return nil, &ValidationError{Message: "reorder request must include every item exactly once"}
	}

	seenPositions := map[int]bool{}
	result := make([]PositionItem, 0, len(items))
	for _, it := range items {
		if !existingIDs[it.ID] {
			return nil, &ValidationError{Message: "unknown id in reorder request"}
		}
		if it.Position < 1 {
			return nil, &ValidationError{Message: "position must be >= 1"}
		}
		if seenPositions[it.Position] {
			return nil, &ValidationError{Message: "duplicate position in reorder request"}
		}
		seenPositions[it.Position] = true
		result = append(result, PositionItem{ID: it.ID, Position: it.Position})
	}
	return result, nil
}
