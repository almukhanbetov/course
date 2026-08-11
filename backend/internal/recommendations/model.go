// Package recommendations is Stage 18's rule-based (never ML) scoring
// engine for two related but distinct surfaces: GET /me/recommendations
// (personalized, authenticated) and GET /courses/:id/similar (public,
// context-only). Both share the same candidate/scoring shape but weight
// different signals — see service.go's two entry points.
package recommendations

import (
	"time"

	"github.com/google/uuid"
)

// Candidate is one published course plus every aggregate feature the
// scorer needs, gathered by a small fixed number of SQL queries (never one
// query per candidate — item 13).
type Candidate struct {
	CourseID        uuid.UUID
	Title           string
	Slug            string
	ImageURL        string
	AccessType      string
	CategoryID      *uuid.UUID
	CategoryName    *string
	RatingAverage   float64
	RatingCount     int
	EnrollmentCount int
	CreatedAt       time.Time
}

// Recommendation is one row of the API response — the numeric weights that
// produced Score are never exposed, only the final integer and the reason
// codes that contributed to it (item 11).
type Recommendation struct {
	CourseID      uuid.UUID `json:"course_id"`
	Title         string    `json:"title"`
	Slug          string    `json:"slug"`
	ImageURL      string    `json:"image_url"`
	AccessType    string    `json:"access_type"`
	CategoryName  *string   `json:"category_name,omitempty"`
	RatingAverage float64   `json:"rating_average"`
	RatingCount   int       `json:"rating_count"`
	Score         int       `json:"score"`
	Reasons       []string  `json:"reasons"`
}

// Reason codes — item 11's exact vocabulary. The frontend maps these to
// Russian display strings itself (e.g. same_category -> "Потому что вы
// изучаете <категория>"); the backend never sends pre-rendered text.
const (
	ReasonSameCategory      = "same_category"
	ReasonLearningPath      = "in_learning_path"
	ReasonWishlist          = "in_wishlist"
	ReasonHighRating        = "high_rating"
	ReasonPopular           = "popular"
	ReasonNewCourse         = "new_course"
	ReasonSpecialityOverlap = "speciality_overlap"
)
