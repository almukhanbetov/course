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

// Feedback is one recommendation_feedback row (Stage 23A1) — a user's
// explicit "not for me" signal against a course, distinct from Reason
// above: reasons explain why the engine surfaced a course, Feedback is the
// user telling the engine to stop.
type Feedback struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	CourseID  uuid.UUID `json:"course_id"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

// Feedback action vocabulary — bounded, never arbitrary client text. Both
// values currently produce the identical storage/exclusion effect (see the
// migration's doc comment: one row per user/course regardless of which
// action was last submitted); kept as two distinct values because the
// roadmap and product intent distinguish "dismiss this specific
// suggestion" from "I'm generally not interested in this course" even
// though Stage 23A1 doesn't yet give them different downstream behavior.
const (
	FeedbackDismiss       = "dismiss"
	FeedbackNotInterested = "not_interested"
)

var allowedFeedbackActions = map[string]bool{
	FeedbackDismiss:       true,
	FeedbackNotInterested: true,
}
