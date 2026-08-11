package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrJobNotFound  = errors.New("notification job not found")
	ErrJobNotFailed = errors.New("only a failed job can be retried")
	ErrNoJob        = errors.New("no notification job available")
)

// Execer is satisfied by both *pgxpool.Pool and pgx.Tx, so Enqueue can be
// called either standalone or — the common case — inside whatever
// transaction the caller's own state change is already running in. This is
// the same convention internal/learning uses for its own execer interface.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Enqueue inserts one notification_jobs row per requested channel. It is a
// package-level function (not a Repository method) specifically so domains
// that already hold a pgx.Tx (e.g. certificates.Repository.CreateCertificate)
// can call it without needing a *notifications.Repository at all — just the
// transaction they're already inside. A duplicate (event, channel) pair is
// silently ignored via the partial unique index on dedupe_key, which is
// what makes every trigger in this stage idempotent under retries/reruns.
func Enqueue(ctx context.Context, db Execer, input EnqueueInput) error {
	var payloadJSON []byte
	if input.Data != nil {
		var err error
		payloadJSON, err = json.Marshal(input.Data)
		if err != nil {
			return err
		}
	}

	for _, channel := range input.Channels {
		var dedupeKey *string
		if input.DedupeKey != "" {
			k := input.DedupeKey + ":" + channel
			dedupeKey = &k
		}

		_, err := db.Exec(ctx, `
			INSERT INTO notification_jobs (user_id, type, payload, channel, dedupe_key)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
		`, input.UserID, input.Type, payloadJSON, channel, dedupeKey)
		if err != nil {
			return err
		}
	}
	return nil
}

// --- worker: claim + process -------------------------------------------

func scanJob(row pgx.Row) (*Job, error) {
	var j Job
	var payload []byte
	err := row.Scan(&j.ID, &j.UserID, &j.Type, &payload, &j.Channel, &j.Status, &j.Attempts,
		&j.AvailableAt, &j.StartedAt, &j.FinishedAt, &j.LastError, &j.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &j.Payload)
	}
	return &j, nil
}

const jobColumns = "id, user_id, type, payload, channel, status, attempts, available_at, started_at, finished_at, last_error, created_at"

// ClaimNextJob atomically claims one due, pending job — FOR UPDATE SKIP
// LOCKED means concurrent workers never block on or double-process the same
// row, exactly like video_jobs' claim query.
func (r *Repository) ClaimNextJob(ctx context.Context) (*Job, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE notification_jobs
		SET status = 'processing', started_at = now(), attempts = attempts + 1
		WHERE id = (
			SELECT id FROM notification_jobs
			WHERE status = 'pending' AND available_at <= now()
			ORDER BY available_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING `+jobColumns)
	job, err := scanJob(row)
	if errors.Is(err, ErrJobNotFound) {
		return nil, ErrNoJob
	}
	return job, err
}

func (r *Repository) MarkJobCompleted(ctx context.Context, jobID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE notification_jobs SET status = 'completed', finished_at = now() WHERE id = $1`, jobID)
	return err
}

// MarkJobFailedOrRetry mirrors video_jobs' identical retry policy: requeue
// with a backoff delay while attempts remain, otherwise fail permanently
// and keep the diagnostic in last_error (never surfaced to students/admins
// verbatim — see templates.go / handler_admin.go for what IS shown).
func (r *Repository) MarkJobFailedOrRetry(ctx context.Context, jobID uuid.UUID, attempts, maxAttempts int, errMsg string, backoff time.Duration) (retried bool, err error) {
	if attempts >= maxAttempts {
		_, err = r.pool.Exec(ctx, `
			UPDATE notification_jobs SET status = 'failed', finished_at = now(), last_error = $2 WHERE id = $1
		`, jobID, errMsg)
		return false, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE notification_jobs SET status = 'pending', available_at = now() + make_interval(secs => $2), last_error = $3 WHERE id = $1
	`, jobID, backoff.Seconds(), errMsg)
	return true, err
}

// CreateInAppNotification materializes a completed "in_app" job into the
// actual read-model row a student sees at GET /me/notifications.
func (r *Repository) CreateInAppNotification(ctx context.Context, userID uuid.UUID, notifType, title, message string, data map[string]any) error {
	var dataJSON []byte
	if data != nil {
		var err error
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return err
		}
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, data) VALUES ($1, $2, $3, $4, $5)
	`, userID, notifType, title, message, dataJSON)
	return err
}

// GetUserContact is used by the email channel to resolve who to send to —
// duplicated one-line query against the users table rather than importing
// the users package, per this codebase's share-the-schema convention.
type UserContact struct {
	Email     string
	FirstName string
}

func (r *Repository) GetUserContact(ctx context.Context, userID uuid.UUID) (*UserContact, error) {
	var c UserContact
	err := r.pool.QueryRow(ctx, `SELECT email, first_name FROM users WHERE id = $1`, userID).Scan(&c.Email, &c.FirstName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// --- student-facing read model -----------------------------------------

func (r *Repository) ListMyNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Notification, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, type, title, message, data, read_at, created_at, COUNT(*) OVER() AS total
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []Notification{}
	total := 0
	for rows.Next() {
		var n Notification
		var data []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &data, &n.ReadAt, &n.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		if len(data) > 0 {
			_ = json.Unmarshal(data, &n.Data)
		}
		result = append(result, n)
	}
	return result, total, rows.Err()
}

func (r *Repository) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&count)
	return count, err
}

// MarkRead only ever touches a row that belongs to userID — the WHERE
// clause is the entire authorization check (see service.go), so a request
// for someone else's notification id simply matches zero rows.
func (r *Repository) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notifications SET read_at = COALESCE(read_at, now()) WHERE id = $1 AND user_id = $2
	`, notificationID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL
	`, userID)
	return err
}

// --- admin monitoring ---------------------------------------------------

func (r *Repository) ListJobsAdmin(ctx context.Context, status, channel string, limit, offset int) ([]AdminJobSummary, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT j.id, j.user_id, j.type, j.payload, j.channel, j.status, j.attempts,
			j.available_at, j.started_at, j.finished_at, j.last_error, j.created_at,
			u.email, COUNT(*) OVER() AS total
		FROM notification_jobs j
		JOIN users u ON u.id = j.user_id
		WHERE ($1 = '' OR j.status = $1) AND ($2 = '' OR j.channel = $2)
		ORDER BY j.created_at DESC
		LIMIT $3 OFFSET $4
	`, status, channel, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := []AdminJobSummary{}
	total := 0
	for rows.Next() {
		var j AdminJobSummary
		var payload []byte
		if err := rows.Scan(&j.ID, &j.UserID, &j.Type, &payload, &j.Channel, &j.Status, &j.Attempts,
			&j.AvailableAt, &j.StartedAt, &j.FinishedAt, &j.LastError, &j.CreatedAt, &j.UserEmail, &total); err != nil {
			return nil, 0, err
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &j.Payload)
		}
		result = append(result, j)
	}
	return result, total, rows.Err()
}

// RetryJob resets a failed job to pending with a clean attempt budget — an
// explicit admin action gets a full fresh set of retries rather than
// immediately re-failing because attempts was already at the ceiling. It
// only ever flips status/attempts/available_at/last_error: the recipient
// (user_id) and payload are never touched, so a retry cannot be used to
// redirect a notification to someone else or change what it says.
func (r *Repository) RetryJob(ctx context.Context, jobID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE notification_jobs
		SET status = 'pending', attempts = 0, available_at = now(), last_error = NULL, finished_at = NULL
		WHERE id = $1 AND status = 'failed'
	`, jobID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrJobNotFailed
	}
	return nil
}

// --- admin: course announcement -----------------------------------------

// AnnounceCourse fans out one in-app job per active student in a single
// server-side INSERT...SELECT statement rather than looping per-user in Go —
// Postgres does the batching internally, so this stays one round trip
// regardless of how many students exist. See the package docs for why this
// is "reasonable batching" for Stage 12's scope rather than chunking.
func (r *Repository) AnnounceCourse(ctx context.Context, courseID uuid.UUID, courseTitle string) (int64, error) {
	payload, err := json.Marshal(map[string]any{"course_id": courseID, "course_title": courseTitle})
	if err != nil {
		return 0, err
	}

	tag, err := r.pool.Exec(ctx, `
		INSERT INTO notification_jobs (user_id, type, payload, channel, dedupe_key)
		SELECT u.id, 'course_announcement', $2, 'in_app', 'course_announcement:' || u.id || ':' || $1::text
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.active = true AND r.name = 'student'
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, courseID, payload)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) GetPublishedCourseTitle(ctx context.Context, courseID uuid.UUID) (string, error) {
	var title string
	err := r.pool.QueryRow(ctx, `SELECT title FROM courses WHERE id = $1 AND published = true`, courseID).Scan(&title)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return title, err
}

// --- subscription expiry scheduler (cmd/notification-worker only) ------

// ScanExpiringSoon fans out a warning to every active subscription whose
// expires_at falls inside the configured window — the dedupe_key
// (subscription_expiring:<id>:<days>d) makes re-running this scan every
// tick a no-op for a subscription already warned about, so no interval
// bookkeeping is needed beyond the unique index itself.
func (r *Repository) ScanExpiringSoon(ctx context.Context, warningDays int) (int64, error) {
	// The dedupe-key suffix is formatted in Go and passed as a plain string
	// ($2), rather than casting the same warningDays value to text inline —
	// binding one Go int to two differently-cast placeholders ($1::int and
	// $1/$2::text) left pgx unable to pick a single wire encoding for it.
	dedupeSuffix := fmt.Sprintf("%dd", warningDays)

	tag, err := r.pool.Exec(ctx, `
		INSERT INTO notification_jobs (user_id, type, payload, channel, dedupe_key)
		SELECT s.user_id, 'subscription_expiring',
			jsonb_build_object('subscription_id', s.id, 'expires_at', s.expires_at, 'plan_name', p.name),
			channel.name,
			'subscription_expiring:' || s.id || ':' || $2 || ':' || channel.name
		FROM subscriptions s
		JOIN subscription_plans p ON p.id = s.plan_id
		CROSS JOIN (VALUES ('in_app'), ('email')) AS channel(name)
		WHERE s.status = 'active'
			AND s.expires_at > now()
			AND s.expires_at <= now() + make_interval(days => $1::int)
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, warningDays, dedupeSuffix)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ScanNewlyExpired flips any subscription whose time has passed to
// 'expired' (the same correction GetActiveSubscription already does lazily
// on read — this just also does it proactively) and enqueues exactly one
// notification per subscription via the dedupe key, regardless of how many
// scan ticks later a lazy read might also observe the same row.
func (r *Repository) ScanNewlyExpired(ctx context.Context) (int64, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE subscriptions SET status = 'expired', updated_at = now()
		WHERE status = 'active' AND expires_at <= now()
		RETURNING id, user_id
	`)
	if err != nil {
		return 0, err
	}

	type expired struct {
		ID     uuid.UUID
		UserID uuid.UUID
	}
	var list []expired
	for rows.Next() {
		var e expired
		if err := rows.Scan(&e.ID, &e.UserID); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var enqueued int64
	for _, e := range list {
		if err := Enqueue(ctx, r.pool, EnqueueInput{
			UserID:    e.UserID,
			Type:      "subscription_expired",
			Data:      map[string]any{"subscription_id": e.ID},
			DedupeKey: "subscription_expired:" + e.ID.String(),
			Channels:  []string{ChannelInApp},
		}); err != nil {
			return enqueued, err
		}
		enqueued++
	}
	return enqueued, nil
}
