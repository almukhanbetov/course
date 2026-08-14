package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type WorkerConfig struct {
	MaxAttempts  int
	RetryBackoff time.Duration
}

// Worker is used only by cmd/notification-worker. It materializes both
// channels of the outbox — in_app rows and actual emails — through the
// same claim/retry machinery; see the package doc comment on why one
// consistent model was chosen over treating in_app as a synchronous write.
type Worker struct {
	repo  *Repository
	email EmailSender
	cfg   WorkerConfig
	log   *slog.Logger
}

// slog.With, not a package-level var — see videos.NewWorker's identical
// comment: this must run after cmd/notification-worker's main() has
// already called logging.Init(), which a package-level var would not.
func NewWorker(repo *Repository, email EmailSender, cfg WorkerConfig) *Worker {
	return &Worker{repo: repo, email: email, cfg: cfg, log: slog.With("service", "notification-worker")}
}

// ClaimAndProcess mirrors video.Worker.ClaimAndProcess's exact shape —
// claims at most one job, processes it, and reports whether the queue had
// anything at all so the caller's loop knows whether to sleep.
func (w *Worker) ClaimAndProcess(ctx context.Context) (claimed bool, err error) {
	job, err := w.repo.ClaimNextJob(ctx)
	if errors.Is(err, ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	w.log.Info("claimed job", "job_id", job.ID, "user_id", job.UserID, "type", job.Type, "channel", job.Channel, "attempt", job.Attempts)

	if procErr := w.processJob(ctx, job); procErr != nil {
		w.log.Error("job failed", "job_id", job.ID, "error", procErr)

		retried, markErr := w.repo.MarkJobFailedOrRetry(ctx, job.ID, job.Attempts, w.cfg.MaxAttempts, truncateError(procErr), w.cfg.RetryBackoff)
		if markErr != nil {
			return true, markErr
		}
		if retried {
			w.log.Warn("job will retry", "job_id", job.ID, "attempt", job.Attempts, "max_attempts", w.cfg.MaxAttempts)
		} else {
			w.log.Error("job exhausted retries, marked failed", "job_id", job.ID)
		}
		return true, nil
	}

	if err := w.repo.MarkJobCompleted(ctx, job.ID); err != nil {
		return true, err
	}
	w.log.Info("job completed", "job_id", job.ID)
	return true, nil
}

func (w *Worker) processJob(ctx context.Context, job *Job) error {
	switch job.Channel {
	case ChannelInApp:
		rendered := renderInApp(job.Type, job.Payload)
		return w.repo.CreateInAppNotification(ctx, job.UserID, job.Type, rendered.Title, rendered.Message, job.Payload)

	case ChannelEmail:
		subject, body, ok := renderEmail(job.Type, job.Payload)
		if !ok {
			// No email template for this event type — not an error, just
			// nothing to send (see templates.go for the deliberate list).
			return nil
		}
		contact, err := w.repo.GetUserContact(ctx, job.UserID)
		if err != nil {
			return fmt.Errorf("look up recipient: %w", err)
		}
		return w.email.Send(ctx, EmailMessage{To: contact.Email, Subject: subject, HTMLBody: body})

	default:
		return fmt.Errorf("unknown notification channel %q", job.Channel)
	}
}

// RunExpiryScan is called on a plain interval timer by cmd/notification-worker
// — not a cron baked into the HTTP backend (see package docs). Both scans
// are idempotent no-ops on any tick where nothing changed, thanks to the
// dedupe_key unique index, so calling this too often costs nothing beyond
// two cheap queries.
func (w *Worker) RunExpiryScan(ctx context.Context, warningDays int) {
	warned, err := w.repo.ScanExpiringSoon(ctx, warningDays)
	if err != nil {
		w.log.Error("expiry-warning scan failed", "error", err)
	} else if warned > 0 {
		w.log.Info("expiry-warning scan enqueued jobs", "count", warned)
	}

	expired, err := w.repo.ScanNewlyExpired(ctx)
	if err != nil {
		w.log.Error("expired scan failed", "error", err)
	} else if expired > 0 {
		w.log.Info("expired scan enqueued notifications", "count", expired)
	}
}

// truncateError is what lands in notification_jobs.last_error — an
// operations-only field, never serialized by any student-facing API. The
// admin monitoring endpoint does expose it (item 21 asks for "attempts" and
// job visibility), but SMTP errors don't carry credentials in their text
// (net/smtp's errors are protocol/network errors, not the auth payload), so
// this is safe for admin eyes without further scrubbing.
func truncateError(err error) string {
	msg := err.Error()
	const max = 2000
	if len(msg) > max {
		return msg[:max]
	}
	return msg
}
