// Package reports implements content abuse reporting: storage (Stage
// 24A1), a student-facing submission endpoint (Stage 24A2), and an
// admin-only moderation queue (Stage 24A3) — a student (or anyone with
// account access to the content) can flag a Q&A question, a Q&A answer, or
// a course review as inappropriate, and an admin can list/filter open
// reports and move them through their status vocabulary. No content
// hide/show action lives here — resolving a report is a status change
// only; actually hiding the underlying content still goes through Stage
// 21's existing Q&A hide/show or the review-publish toggle, unchanged.
package reports

import (
	"time"

	"github.com/google/uuid"
)

// Report is one content_reports row.
type Report struct {
	ID             uuid.UUID `json:"id"`
	ReporterUserID uuid.UUID `json:"reporter_user_id"`
	ContentType    string    `json:"content_type"`
	ContentID      uuid.UUID `json:"content_id"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ContentType vocabulary — bounded, never arbitrary client text. Mirrors
// the migration's CHECK (content_type IN (...)) constraint exactly; kept
// in sync by hand since goose migrations aren't introspected at build time.
const (
	ContentTypeQuestion = "question"
	ContentTypeAnswer   = "answer"
	ContentTypeReview   = "review"
)

var allowedContentTypes = map[string]bool{
	ContentTypeQuestion: true,
	ContentTypeAnswer:   true,
	ContentTypeReview:   true,
}

// Status vocabulary — same bounded-vocabulary treatment as ContentType.
// Only StatusOpen is ever written this session (Create relies on the
// column's own DB default); Resolved/Dismissed exist now so the future
// admin-resolve action (Stage 24A2+) has real values to write without a
// second migration.
const (
	StatusOpen      = "open"
	StatusResolved  = "resolved"
	StatusDismissed = "dismissed"
)

var allowedStatuses = map[string]bool{
	StatusOpen:      true,
	StatusResolved:  true,
	StatusDismissed: true,
}

// AdminReport is GET /admin/reports' row shape (Stage 24A3): adds the
// reporter's display name — never their email — mirroring
// internal/reviews.AdminReview's identical identity-exposure convention.
// Deliberately narrower than Report (no updated_at): just enough context
// for a moderator to triage a queue entry, per this stage's own field list.
type AdminReport struct {
	ID             uuid.UUID `json:"id"`
	ReporterUserID uuid.UUID `json:"reporter_user_id"`
	ReporterName   string    `json:"reporter_name"`
	ContentType    string    `json:"content_type"`
	ContentID      uuid.UUID `json:"content_id"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// AdminListParams drives GET /admin/reports' filter/pagination contract.
// Status/ContentType are validated against the same whitelists as
// CreateReport when non-empty (see Service.ListAdmin) — an empty string
// means "no filter," not "match the empty string."
type AdminListParams struct {
	Status      string
	ContentType string
	Page        int
	Limit       int
}
