package reviews

import (
	"time"

	"github.com/google/uuid"
)

type Review struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	CourseID   uuid.UUID `json:"course_id"`
	Rating     int       `json:"rating"`
	ReviewText *string   `json:"review_text,omitempty"`
	Published  bool      `json:"published"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PublicReview is what GET /courses/:id/reviews returns: a display name and
// the review, and nothing that could identify the reviewer beyond that
// (never an email).
type PublicReview struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Rating      int       `json:"rating"`
	ReviewText  *string   `json:"review_text,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminReview additionally carries identifiers and the published flag so
// /admin/reviews can filter, show context, and moderate.
type AdminReview struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	UserName    string    `json:"user_name"`
	CourseID    uuid.UUID `json:"course_id"`
	CourseTitle string    `json:"course_title"`
	Rating      int       `json:"rating"`
	ReviewText  *string   `json:"review_text,omitempty"`
	Published   bool      `json:"published"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ReviewInput struct {
	Rating     int
	ReviewText string
}

type AdminListParams struct {
	CourseID *uuid.UUID
	Rating   *int
	Page     int
	Limit    int
}

// Eligibility reports whether a user is enrolled in a course and, if so,
// whether they've made enough progress to leave a review.
type Eligibility struct {
	Enrolled bool `json:"enrolled"`
	Eligible bool `json:"eligible"`
}
