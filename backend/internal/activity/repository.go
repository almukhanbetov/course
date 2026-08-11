package activity

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Record inserts one learning_activity row — a package-level function
// (mirrors notifications.Enqueue) so a domain already holding a pgx.Tx from
// its own state-change transaction can call it without a second
// transaction. occurred_at is always the database's own now() — never
// accepted from a caller-supplied timestamp (item 24/25: a client must
// never be able to backdate or future-date an event to manipulate a streak).
func Record(ctx context.Context, db Execer, input RecordInput) error {
	var metadataJSON []byte
	if input.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(input.Metadata)
		if err != nil {
			return err
		}
	}

	var dedupeKey *string
	if input.DedupeKey != "" {
		dedupeKey = &input.DedupeKey
	}
	var entityType *string
	if input.EntityType != "" {
		entityType = &input.EntityType
	}

	_, err := db.Exec(ctx, `
		INSERT INTO learning_activity (user_id, activity_type, entity_type, entity_id, metadata, dedupe_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, input.UserID, input.Type, entityType, input.EntityID, metadataJSON, dedupeKey)
	return err
}

// GetUserTimezone duplicates a one-column lookup against `users` rather
// than importing internal/users, per this codebase's "share schema, not
// code" convention between domains. A free function (like Record/GetCounts)
// so achievement evaluation can call it against an in-flight tx.
func GetUserTimezone(ctx context.Context, db Execer, userID uuid.UUID) (string, error) {
	var tz string
	err := db.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1`, userID).Scan(&tz)
	return tz, err
}

// GetCounts backs both GET /me/analytics and achievement rule evaluation —
// one round trip, no N+1. Enrollment/completion/certificate counts read the
// live tables directly (they're already the authoritative "current state");
// the four event counts read the activity ledger, which is the natural
// source of truth for "how many times did X happen" and is guaranteed
// at-most-once per real transition via each write site's dedupe_key.
func GetCounts(ctx context.Context, db Execer, userID uuid.UUID) (Counts, error) {
	var c Counts
	err := db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM course_enrollments WHERE user_id = $1),
			(SELECT COUNT(*) FROM course_enrollments WHERE user_id = $1 AND completed_at IS NOT NULL),
			(SELECT COUNT(*) FROM learning_activity WHERE user_id = $1 AND activity_type = 'lesson_completed'),
			(SELECT COUNT(*) FROM learning_activity WHERE user_id = $1 AND activity_type = 'assignment_approved'),
			(SELECT COUNT(*) FROM learning_activity WHERE user_id = $1 AND activity_type = 'coding_exercise_passed'),
			(SELECT COUNT(*) FROM learning_activity WHERE user_id = $1 AND activity_type = 'test_passed'),
			(SELECT COUNT(*) FROM certificates WHERE user_id = $1)
	`, userID).Scan(&c.CoursesEnrolled, &c.CoursesCompleted, &c.LessonsCompleted,
		&c.AssignmentsApproved, &c.CodingExercisesPassed, &c.TestsPassed, &c.Certificates)
	return c, err
}

// GetActiveLocalDates returns the distinct calendar dates (in the caller's
// timezone) on which the user had at least one meaningful learning event —
// the raw input to both the streak algorithm and (indirectly) the
// STREAK_* achievements. timezone must already be a validated IANA zone
// name (see internal/users' timezone update handler) — an invalid zone
// makes Postgres's AT TIME ZONE error the whole query rather than silently
// misbehaving.
func GetActiveLocalDates(ctx context.Context, db Execer, userID uuid.UUID, timezone string) ([]time.Time, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT (occurred_at AT TIME ZONE $2)::date AS d
		FROM learning_activity
		WHERE user_id = $1 AND activity_type = ANY($3)
		ORDER BY d DESC
	`, userID, timezone, MeaningfulTypes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC))
	}
	return dates, rows.Err()
}

// ComputeStreaks is pure Go (item 9: "не нужно Redis cache", service-side
// computation over a small per-user date list is plenty fast). today must
// already be normalized to midnight UTC-in-name-only (see
// internal/activity.LocalToday) representing the user's *local* calendar
// date — every date in activeDates is normalized the same way by
// GetActiveLocalDates, so equality/AddDate arithmetic is safe.
//
// current streak: if today has activity, count backward from today. If
// today has none yet but yesterday did, count backward from yesterday
// instead — item 8 explicitly requires not zeroing the streak just because
// the local day isn't over yet. Only when neither today nor yesterday has
// activity does the streak read 0.
func ComputeStreaks(activeDates []time.Time, today time.Time) (current, longest int) {
	dateSet := make(map[time.Time]bool, len(activeDates))
	for _, d := range activeDates {
		dateSet[d] = true
	}

	start := today
	if !dateSet[today] {
		yesterday := today.AddDate(0, 0, -1)
		if dateSet[yesterday] {
			start = yesterday
		} else {
			start = time.Time{}
		}
	}
	if !start.IsZero() {
		for cursor := start; dateSet[cursor]; cursor = cursor.AddDate(0, 0, -1) {
			current++
		}
	}

	sorted := append([]time.Time(nil), activeDates...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	run := 0
	for i, d := range sorted {
		if i > 0 && d.Equal(sorted[i-1].AddDate(0, 0, 1)) {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
	}
	return current, longest
}

// LocalToday converts the current server instant into the user's local
// calendar date, normalized the same way GetActiveLocalDates' SQL does, so
// the two are always comparable.
func LocalToday(timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
}

// --- pool-backed reads for the HTTP handler --------------------------------

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetCounts(ctx context.Context, userID uuid.UUID) (Counts, error) {
	return GetCounts(ctx, r.pool, userID)
}

func (r *Repository) GetActiveLocalDates(ctx context.Context, userID uuid.UUID, timezone string) ([]time.Time, error) {
	return GetActiveLocalDates(ctx, r.pool, userID, timezone)
}

// GetCalendar aggregates activity_count per local calendar day within
// [from, to] (inclusive) — backs GET /me/activity (item 5). Every
// meaningful and non-meaningful event type is counted here (unlike the
// streak, which only counts MeaningfulTypes) since the calendar is meant to
// show "how much did I do that day", not just streak-eligible events.
func (r *Repository) GetCalendar(ctx context.Context, userID uuid.UUID, timezone string, from, to time.Time) ([]DayCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT (occurred_at AT TIME ZONE $2)::date AS d, COUNT(*)
		FROM learning_activity
		WHERE user_id = $1
		  AND (occurred_at AT TIME ZONE $2)::date BETWEEN $3::date AND $4::date
		GROUP BY d
		ORDER BY d ASC
	`, userID, timezone, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []DayCount{}
	for rows.Next() {
		var d time.Time
		var count int
		if err := rows.Scan(&d, &count); err != nil {
			return nil, err
		}
		result = append(result, DayCount{Date: d.Format("2006-01-02"), ActivityCount: count})
	}
	return result, rows.Err()
}

func (r *Repository) ListRecent(ctx context.Context, userID uuid.UUID, limit int) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, activity_type, entity_type, entity_id, occurred_at, metadata
		FROM learning_activity
		WHERE user_id = $1
		ORDER BY occurred_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Entry{}
	for rows.Next() {
		var e Entry
		var metadataJSON []byte
		if err := rows.Scan(&e.ID, &e.ActivityType, &e.EntityType, &e.EntityID, &e.OccurredAt, &metadataJSON); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
				return nil, err
			}
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
