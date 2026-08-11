// Package wishlist is a small, self-contained domain: a pure intent signal
// ("I want to learn this") with no side effects of its own — adding or
// removing a course never records a learning_activity event (Stage 17 item
// 22 applies retroactively: wishlist is not learning) and never enqueues a
// notification (item 21: no noise). Its only cross-domain effect is
// read-only (recommendations' wishlist boost) plus one deliberate write
// internal/learning makes directly (auto-remove on enrollment, in the same
// transaction — see CreateEnrollment).
package wishlist

import (
	"time"

	"github.com/google/uuid"
)

// Item is one row of GET /me/wishlist — enough fields to render a course
// card without a second round trip. Deliberately a narrower shape than
// courses.Course (no publication workflow fields, no instructor name) since
// this is always the student-facing view.
type Item struct {
	CourseID      uuid.UUID `json:"course_id"`
	Title         string    `json:"title"`
	Slug          string    `json:"slug"`
	ImageURL      string    `json:"image_url"`
	AccessType    string    `json:"access_type"`
	CategoryName  *string   `json:"category_name,omitempty"`
	RatingAverage float64   `json:"rating_average"`
	RatingCount   int       `json:"rating_count"`
	AddedAt       time.Time `json:"added_at"`
}
