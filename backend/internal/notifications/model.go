package notifications

import (
	"time"

	"github.com/google/uuid"
)

const (
	ChannelInApp = "in_app"
	ChannelEmail = "email"
)

const (
	JobPending    = "pending"
	JobProcessing = "processing"
	JobCompleted  = "completed"
	JobFailed     = "failed"
)

// Event types. Each has a fixed set of channels it's ever enqueued on (see
// Enqueue callers) and a renderer in templates.go for every channel it uses.
const (
	TypeWelcome                 = "welcome"
	TypeEnrolled                = "enrolled"
	TypeCourseCompleted         = "course_completed"
	TypeCertificateIssued       = "certificate_issued"
	TypePaymentPaid             = "payment_paid"
	TypeSubscriptionActivated   = "subscription_activated"
	TypeSubscriptionExpiring    = "subscription_expiring"
	TypeSubscriptionExpired     = "subscription_expired"
	TypeCourseAnnouncement      = "course_announcement"
	TypeCourseApproved          = "course_approved"
	TypeCourseRejected          = "course_rejected"
	TypeAssignmentSubmitted     = "assignment_submitted"
	TypeAssignmentApproved      = "assignment_approved"
	TypeAssignmentNeedsRevision = "assignment_needs_revision"

	// TypeAchievementEarned is Stage 17's addition — in-app only, never
	// emailed (item 13: "Не отправлять email на каждое достижение"), so
	// every enqueue call site passes Channels: []string{ChannelInApp}.
	TypeAchievementEarned = "achievement_earned"

	// TypeQuestionAnswered is Stage 20A's addition (internal/qa) — in-app
	// only, same minimal-scope reasoning as the assignment-status types
	// above; only the original question's asker is ever notified, never
	// every participant in a thread.
	TypeQuestionAnswered = "question_answered"
)

// Notification is one row of the in-app read model — see
// migrations/00028_create_notifications.sql. It is only ever written by
// notification-worker materializing a completed "in_app" job; the API
// backend never inserts into this table directly.
type Notification struct {
	ID        uuid.UUID      `json:"id"`
	UserID    uuid.UUID      `json:"user_id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Job is one row of the transactional outbox — see Enqueue.
type Job struct {
	ID          uuid.UUID      `json:"id"`
	UserID      uuid.UUID      `json:"user_id"`
	Type        string         `json:"type"`
	Payload     map[string]any `json:"payload,omitempty"`
	Channel     string         `json:"channel"`
	Status      string         `json:"status"`
	Attempts    int            `json:"attempts"`
	AvailableAt time.Time      `json:"available_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	LastError   *string        `json:"last_error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// AdminJobSummary annotates a job with the recipient's identity for the
// admin monitoring list — see handler_admin.go.
type AdminJobSummary struct {
	Job
	UserEmail string `json:"user_email"`
}

// EnqueueInput describes one event to fan out across channels. Callers
// supply structured Data, never pre-rendered text — title/message/email
// copy is rendered centrally in templates.go so "how to phrase this" lives
// in exactly one place instead of being duplicated at every call site.
type EnqueueInput struct {
	UserID uuid.UUID
	Type   string
	Data   map[string]any
	// DedupeKey, if non-empty, is combined with each channel to form the
	// actual per-row dedupe key (e.g. "course_completed:<u>:<c>:in_app") —
	// callers only need to describe the event once, not once per channel.
	DedupeKey string
	Channels  []string
}
