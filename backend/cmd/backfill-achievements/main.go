// Command backfill-achievements is a one-off, safely re-runnable operation
// for existing Stage 1-16 data (Stage 17 item 23): "не нужно генерировать
// идеальную полную историческую activity timeline из ничего", but existing
// completed courses, certificates, and passed coding exercises can and
// should be reflected in learning_activity and evaluated for achievements.
//
// It deliberately backfills only those three event types — every other
// activity type (lesson_completed, assignment_submitted/approved,
// test_passed) has no unambiguous single "this happened at exactly this
// instant" timestamp to backfill from without guessing, so this command
// never invents one (item 23/25: "Не создавать ложные timestamps
// исторической активности"). Where a real historical timestamp DOES exist
// (course_enrollments.completed_at, certificates.issued_at,
// code_submissions.finished_at), that real value is used as occurred_at —
// this is a retroactive ledger entry for something that genuinely
// happened, not a fabricated one.
//
// STREAK_* achievements are intentionally not eligible for backfill: a
// streak is inherently about *consecutive* days, which this command has no
// way to honestly reconstruct from three isolated event types, so those
// three achievements are earned only going forward from real Stage 17
// activity.
//
// Idempotent: every insert uses the exact same dedupe_key format the live
// Stage 17 hooks use (see internal/learning, internal/coding,
// internal/certificates), so running this after Stage 17 has been live for
// a while — or running it twice — never double-counts or double-notifies.
package main

import (
	"context"
	"log"

	"github.com/google/uuid"

	"lms-backend/internal/achievements"
	"lms-backend/internal/config"
	"lms-backend/internal/db"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("backfill-achievements: database connection failed: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("backfill-achievements: begin transaction failed: %v", err)
	}
	defer tx.Rollback(ctx)

	coursesInserted, err := tx.Exec(ctx, `
		INSERT INTO learning_activity (user_id, activity_type, entity_type, entity_id, occurred_at, dedupe_key)
		SELECT user_id, 'course_completed', 'course', course_id, completed_at,
		       'course_completed_activity:' || user_id || ':' || course_id
		FROM course_enrollments
		WHERE completed_at IS NOT NULL
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`)
	if err != nil {
		log.Fatalf("backfill-achievements: backfill course_completed failed: %v", err)
	}

	certsInserted, err := tx.Exec(ctx, `
		INSERT INTO learning_activity (user_id, activity_type, entity_type, entity_id, occurred_at, dedupe_key)
		SELECT user_id, 'certificate_issued', 'certificate', id, issued_at,
		       'certificate_issued_activity:' || id
		FROM certificates
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`)
	if err != nil {
		log.Fatalf("backfill-achievements: backfill certificate_issued failed: %v", err)
	}

	codingInserted, err := tx.Exec(ctx, `
		INSERT INTO learning_activity (user_id, activity_type, entity_type, entity_id, occurred_at, dedupe_key)
		SELECT user_id, 'coding_exercise_passed', 'code_submission', id, COALESCE(finished_at, created_at),
		       'coding_exercise_passed_activity:' || id
		FROM code_submissions
		WHERE mode = 'submit' AND status = 'passed'
		ON CONFLICT (dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`)
	if err != nil {
		log.Fatalf("backfill-achievements: backfill coding_exercise_passed failed: %v", err)
	}

	log.Printf("backfill-achievements: activity rows inserted — courses=%d certificates=%d coding=%d",
		coursesInserted.RowsAffected(), certsInserted.RowsAffected(), codingInserted.RowsAffected())

	// Evaluate every user who now has at least one backfilled (or
	// pre-existing) meaningful/course/certificate activity row — one
	// transaction per user keeps a single bad row from aborting the whole
	// run, and achievements.Evaluate is idempotent regardless of how many
	// times it's called for the same user.
	rows, err := tx.Query(ctx, `SELECT DISTINCT user_id FROM learning_activity`)
	if err != nil {
		log.Fatalf("backfill-achievements: list affected users failed: %v", err)
	}
	var userIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			log.Fatalf("backfill-achievements: scan user id failed: %v", err)
		}
		userIDs = append(userIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Fatalf("backfill-achievements: iterate affected users failed: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("backfill-achievements: commit activity backfill failed: %v", err)
	}

	evaluated, failed := 0, 0
	for _, userID := range userIDs {
		evalTx, err := pool.Begin(ctx)
		if err != nil {
			log.Printf("backfill-achievements: begin eval tx for user=%s failed: %v", userID, err)
			failed++
			continue
		}
		if err := achievements.Evaluate(ctx, evalTx, userID); err != nil {
			log.Printf("backfill-achievements: evaluate user=%s failed: %v", userID, err)
			evalTx.Rollback(ctx)
			failed++
			continue
		}
		if err := evalTx.Commit(ctx); err != nil {
			log.Printf("backfill-achievements: commit eval tx for user=%s failed: %v", userID, err)
			failed++
			continue
		}
		evaluated++
	}

	log.Printf("backfill-achievements: done — %d users evaluated, %d failed", evaluated, failed)
}
