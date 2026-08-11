package specialities

import (
	"time"

	"github.com/google/uuid"
)

type Speciality struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	Published   bool      `json:"published"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SpecialityCourse is a course entry within a speciality's public roadmap.
type SpecialityCourse struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Level       string    `json:"level"`
	Position    int       `json:"position"`
	Required    bool      `json:"required"`
}

type SpecialityDetail struct {
	Speciality
	Courses []SpecialityCourse `json:"courses"`
}

// CourseProgress is a course entry annotated with the current user's
// progress, used by GET /api/v1/me/specialities/:id.
type CourseProgress struct {
	CourseID        uuid.UUID `json:"course_id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Position        int       `json:"position"`
	Required        bool      `json:"required"`
	ProgressPercent int       `json:"progress_percent"`
	Completed       bool      `json:"completed"`
}

type MyRoadmap struct {
	SpecialityID    uuid.UUID        `json:"speciality_id"`
	Title           string           `json:"title"`
	Slug            string           `json:"slug"`
	Description     string           `json:"description"`
	ProgressPercent int              `json:"progress_percent"`
	Completed       bool             `json:"completed"`
	Courses         []CourseProgress `json:"courses"`
}

type SpecialityInput struct {
	Title       string
	Slug        string
	Description string
	ImageURL    string
	Published   bool
}

// RoadmapReorderItem is one entry of a bulk speciality_courses reorder
// request — it updates both position and required in a single pass.
type RoadmapReorderItem struct {
	CourseID uuid.UUID
	Position int
	Required bool
}
