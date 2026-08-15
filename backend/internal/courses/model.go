package courses

import (
	"time"

	"github.com/google/uuid"
)

var AllowedLevels = map[string]bool{
	"beginner":     true,
	"intermediate": true,
	"advanced":     true,
}

var AllowedAccessTypes = map[string]bool{
	"free":         true,
	"subscription": true,
}

// AllowedPublicationStatuses is the full set of states in the authoring
// workflow (see migration 00032). Not every transition between them is
// legal — that's enforced in service.go, not here.
var AllowedPublicationStatuses = map[string]bool{
	"draft":          true,
	"pending_review": true,
	"published":      true,
	"rejected":       true,
}

type Course struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Level       string    `json:"level"`
	ImageURL    string    `json:"image_url"`
	Published   bool      `json:"published"`
	AccessType  string    `json:"access_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Title/Description above are always Russian (NOT NULL) — the original,
	// still-canonical content and the fallback whenever a translation below
	// is missing. These four are optional per-locale overrides, populated
	// only where a real translation exists (Stage: multilingual catalog).
	TitleKk       *string `json:"title_kk,omitempty"`
	TitleEn       *string `json:"title_en,omitempty"`
	DescriptionKk *string `json:"description_kk,omitempty"`
	DescriptionEn *string `json:"description_en,omitempty"`

	CategoryID   *uuid.UUID `json:"category_id,omitempty"`
	CategoryName *string    `json:"category_name,omitempty"`
	CategorySlug *string    `json:"category_slug,omitempty"`

	// RatingAverage/RatingCount are computed server-side from published
	// course_reviews on every read — never trust a client-supplied rating.
	RatingAverage float64 `json:"rating_average"`
	RatingCount   int     `json:"rating_count"`

	InstructorID      *uuid.UUID `json:"instructor_id,omitempty"`
	InstructorName    *string    `json:"instructor_name,omitempty"`
	PublicationStatus string     `json:"publication_status"`
	RejectionReason   *string    `json:"rejection_reason,omitempty"`
}

type Module struct {
	ID        uuid.UUID `json:"id"`
	CourseID  uuid.UUID `json:"course_id"`
	Title     string    `json:"title"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Lessons   []Lesson  `json:"lessons"`
}

type Lesson struct {
	ID              uuid.UUID `json:"id"`
	ModuleID        uuid.UUID `json:"module_id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Description     string    `json:"description"`
	VideoURL        string    `json:"video_url"`
	DurationSeconds int       `json:"duration_seconds"`
	Position        int       `json:"position"`
	IsFree          bool      `json:"is_free"`
	Published       bool      `json:"published"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CourseDetail struct {
	Course
	Modules []Module `json:"modules"`
}

// CourseInput is the admin-facing course DTO — admin is fully trusted, so it
// carries Published and InstructorID directly. See InstructorCourseInput for
// the deliberately narrower instructor-facing counterpart.
type CourseInput struct {
	Title        string
	Slug         string
	Description  string
	Level        string
	ImageURL     string
	Published    bool
	AccessType   string
	CategoryID   *uuid.UUID
	InstructorID *uuid.UUID
}

// InstructorCourseInput is structurally incapable of setting instructor_id,
// published, or publication_status — there is simply no field for them.
// The service always derives instructor_id from the authenticated caller and
// forces publication_status through SubmitForReview instead of this input.
type InstructorCourseInput struct {
	Title       string
	Slug        string
	Description string
	Level       string
	ImageURL    string
	AccessType  string
	CategoryID  *uuid.UUID
}

// ListCoursesParams drives the public search/filter/sort/pagination
// contract on GET /courses. Sort is validated and defaulted by the service
// (never interpolated raw into SQL — the repository maps it to one of a
// fixed set of ORDER BY clauses).
type ListCoursesParams struct {
	Query        string
	CategorySlug string
	Level        string
	AccessType   string
	Sort         string
	Page         int
	Limit        int
}

// CourseSuggestion is the deliberately narrow shape for search-as-you-type
// (Stage 22A1) — just enough to render and link a suggestion (title +
// slug for the URL, category name for a subtitle), not the full Course
// shape SearchCourses returns. No rating/instructor/description: those
// cost the same LATERAL join SearchCourses already pays for a full result
// page, which a per-keystroke suggestion query should never pay.
type CourseSuggestion struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	Slug         string    `json:"slug"`
	CategoryName *string   `json:"category_name,omitempty"`
}

type ModuleInput struct {
	Title string
}

type LessonInput struct {
	Title           string
	Slug            string
	Description     string
	VideoURL        string
	DurationSeconds int
	IsFree          bool
	Published       bool
}

// PositionItem is one entry of a bulk reorder request.
type PositionItem struct {
	ID       uuid.UUID
	Position int
}
