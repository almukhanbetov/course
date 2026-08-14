package health

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lms-backend/internal/logging"
)

// checkTimeout bounds every individual dependency check (instruct 8: no
// hanging external calls) — a hung Postgres/MinIO/query must never hang the
// health request itself, whether that's the public shallow endpoint or the
// admin deep one.
const checkTimeout = 5 * time.Second

// notificationQueueDegradedAfter is how long the oldest pending
// notification_jobs row can sit unclaimed before the deep check considers
// notification-worker degraded rather than just momentarily busy. A tight
// poll loop (NOTIFICATION_JOB_POLL_INTERVAL_SEC defaults to 3s) normally
// claims a pending job within seconds; a few minutes' backlog is a real
// signal the worker isn't keeping up (or isn't running at all), while
// staying generous enough that a legitimate burst (e.g. AnnounceCourse
// enqueuing many jobs at once) doesn't trip a false alarm.
const notificationQueueDegradedAfter = 5 * time.Minute

// StoragePinger is the one method this package needs from *videos.S3Storage
// — Stage 29A3 reuses that existing client (it already has Ping) rather
// than adding a second MinIO client anywhere.
type StoragePinger interface {
	Ping(ctx context.Context) error
}

// NotificationQueueChecker is the one method this package needs from
// *notifications.Repository — reads existing persisted queue state
// (notification_jobs), not a new liveness/heartbeat mechanism.
type NotificationQueueChecker interface {
	PendingQueueDepth(ctx context.Context) (count int, oldestAge time.Duration, err error)
}

type Service struct {
	pool          *pgxpool.Pool
	storage       StoragePinger
	notifications NotificationQueueChecker
}

func NewService(pool *pgxpool.Pool, storage StoragePinger, notifications NotificationQueueChecker) *Service {
	return &Service{pool: pool, storage: storage, notifications: notifications}
}

// Status is the public, unauthenticated-safe response — unchanged shape
// from before Stage 29A3 (instruction 2), just with a bounded timeout now
// backing the same database check.
type Status struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func (s *Service) Check(ctx context.Context) Status {
	db := s.checkDatabase(ctx)
	if db.Status != "ok" {
		return Status{Status: "degraded", Database: "unreachable"}
	}
	return Status{Status: "ok", Database: "ok"}
}

// ComponentStatus is deliberately minimal even on the admin-only deep
// response — Detail is short, operational text (e.g. a pending-job count),
// never a raw error, hostname, credential, or stack trace, matching the
// same discipline the public endpoint has always had, just applied one
// level deeper rather than dropped once auth is added.
type ComponentStatus struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// DeepStatus is served only behind admin auth (see handler.go/main.go's
// adminGroup) — never to an unauthenticated caller.
type DeepStatus struct {
	Status        string          `json:"status"`
	Database      ComponentStatus `json:"database"`
	Storage       ComponentStatus `json:"storage"`
	Notifications ComponentStatus `json:"notifications"`
}

func (s *Service) CheckDeep(ctx context.Context) DeepStatus {
	db := s.checkDatabase(ctx)
	storage := s.checkStorage(ctx)
	notifications := s.checkNotifications(ctx)

	overall := "ok"
	if db.Status != "ok" || storage.Status != "ok" || notifications.Status != "ok" {
		overall = "degraded"
	}

	return DeepStatus{Status: overall, Database: db, Storage: storage, Notifications: notifications}
}

func (s *Service) checkDatabase(ctx context.Context) ComponentStatus {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		logging.FromContext(ctx).Warn("health check: database unreachable", "error", err)
		return ComponentStatus{Status: "degraded", Detail: "unreachable"}
	}
	return ComponentStatus{Status: "ok"}
}

func (s *Service) checkStorage(ctx context.Context) ComponentStatus {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	if err := s.storage.Ping(ctx); err != nil {
		logging.FromContext(ctx).Warn("health check: object storage unreachable", "error", err)
		return ComponentStatus{Status: "degraded", Detail: "unreachable"}
	}
	return ComponentStatus{Status: "ok"}
}

func (s *Service) checkNotifications(ctx context.Context) ComponentStatus {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	count, oldestAge, err := s.notifications.PendingQueueDepth(ctx)
	if err != nil {
		logging.FromContext(ctx).Warn("health check: notification queue depth query failed", "error", err)
		return ComponentStatus{Status: "degraded", Detail: "queue depth unavailable"}
	}
	if count > 0 && oldestAge > notificationQueueDegradedAfter {
		logging.FromContext(ctx).Warn("health check: notification queue backlog", "pending", count, "oldest_age", oldestAge.Round(time.Second).String())
		return ComponentStatus{Status: "degraded", Detail: fmt.Sprintf("%d pending, oldest %s", count, oldestAge.Round(time.Second))}
	}
	return ComponentStatus{Status: "ok", Detail: fmt.Sprintf("%d pending", count)}
}
