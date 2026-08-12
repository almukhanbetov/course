// Package audit implements the platform accountability trail (Stage 25A1):
// storage only this session — a single AuditLog struct, an append-only
// table, and one service method (Service.Log) ready for a future stage to
// call from real mutations (role changes, Q&A/review hides, report
// resolutions, certificate issuance, subscription/payment overrides — see
// ROADMAP_STAGE_21_30.md's Stage 25 goal). No handler, no admin endpoint,
// nothing wired into any existing domain's handlers yet — see
// STAGE25_PROGRESS.md for the deliberate scope boundary.
package audit

import (
	"time"

	"github.com/google/uuid"
)

// AuditLog is one audit_logs row. ActorUserID/ActorRole are both nullable
// — legitimate for a system-generated event with no human actor, or for a
// human actor whose account has since been deleted (ActorUserID goes NULL
// via ON DELETE SET NULL, but the row itself survives; see the
// migration's own doc comment). EntityID is nullable too: not every
// audited action has one specific instance (e.g. a bulk operation).
// Metadata is arbitrary structured context — callers must never put a
// password, token, or payment secret in it (see Service.Log's doc
// comment).
type AuditLog struct {
	ID          uuid.UUID      `json:"id"`
	ActorUserID *uuid.UUID     `json:"actor_user_id,omitempty"`
	ActorRole   *string        `json:"actor_role,omitempty"`
	Action      string         `json:"action"`
	EntityType  string         `json:"entity_type"`
	EntityID    *uuid.UUID     `json:"entity_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// Known action/entity_type values, anticipating the roadmap's own Stage 25
// goal (role changes, Q&A/review hides, report resolutions, certificate
// issuance, subscription/payment overrides). Documented here as a starting
// vocabulary for future call sites to reuse verbatim — consistent naming
// matters for a queryable log — NOT as a closed whitelist enforced anywhere
// in this package. Unlike internal/reports' content_type/status (a
// genuinely fixed 3-value set with a DB CHECK constraint), audit
// actions/entity types are inherently open-ended: new call sites in future
// stages will add new values routinely, and forcing every addition through
// a migration would work directly against "extensible enough for future
// stages." Service.Log validates structure (non-empty, bounded length),
// never membership in this list.
const (
	ActionRoleChanged          = "role_changed"
	ActionContentHidden        = "content_hidden"
	ActionContentShown         = "content_shown"
	ActionReportResolved       = "report_resolved"
	ActionReportDismissed      = "report_dismissed"
	ActionReportReopened       = "report_reopened"
	ActionCertificateIssued    = "certificate_issued"
	ActionSubscriptionOverride = "subscription_overridden"
	ActionPaymentOverride      = "payment_overridden"
)

const (
	EntityTypeUser         = "user"
	EntityTypeQuestion     = "question"
	EntityTypeAnswer       = "answer"
	EntityTypeReview       = "review"
	EntityTypeReport       = "report"
	EntityTypeCertificate  = "certificate"
	EntityTypeSubscription = "subscription"
	EntityTypePayment      = "payment"
)

// AdminAuditLog is GET /admin/audit-log's row shape (Stage 25A3): adds the
// actor's display name when resolvable — never their email — mirroring
// internal/reviews.AdminReview's and internal/reports.AdminReport's
// identical identity-exposure convention. ActorUserID/ActorName/ActorRole/
// EntityID are all nullable, exactly like AuditLog itself: a NULL actor is
// a legitimate system event or a since-deleted account (see the
// migration's ON DELETE SET NULL doc comment), not an error, and still
// appears in the list.
type AdminAuditLog struct {
	ID          uuid.UUID      `json:"id"`
	ActorUserID *uuid.UUID     `json:"actor_user_id,omitempty"`
	ActorName   *string        `json:"actor_name,omitempty"`
	ActorRole   *string        `json:"actor_role,omitempty"`
	Action      string         `json:"action"`
	EntityType  string         `json:"entity_type"`
	EntityID    *uuid.UUID     `json:"entity_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// AdminListParams drives GET /admin/audit-log's filter/pagination
// contract. All three filters are optional — empty means "no filter," not
// "reject." None are validated against a whitelist (matching this
// package's own "action/entity_type are open-ended" design decision;
// actor_role is a genuinely fixed small set in practice, but an
// unrecognized filter value here just yields zero matching rows, the same
// safe behavior as any other filter, not a 400).
type AdminListParams struct {
	ActorRole  string
	Action     string
	EntityType string
	Page       int
	Limit      int
}
