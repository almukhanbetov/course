// Command notification-worker processes the notification_jobs outbox
// (materializing in_app rows and sending email) and periodically scans
// subscriptions for expiry warnings/expirations. It shares this
// repository's Go module and internal/notifications package with cmd/api —
// same modular monolith, split into a separate binary only so a slow SMTP
// call or a scheduled scan never competes with the HTTP backend.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"lms-backend/internal/config"
	"lms-backend/internal/db"
	"lms-backend/internal/notifications"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("notification-worker: database connection failed: %v", err)
	}
	defer pool.Close()

	var emailSender notifications.EmailSender
	if cfg.SMTPHost == "" {
		log.Println("notification-worker: SMTP_HOST not set, using LogSender (emails will only be logged, not sent)")
		emailSender = notifications.LogSender{}
	} else {
		emailSender = notifications.NewSMTPSender(notifications.SMTPConfig{
			Host:      cfg.SMTPHost,
			Port:      cfg.SMTPPort,
			User:      cfg.SMTPUser,
			Password:  cfg.SMTPPassword,
			FromEmail: cfg.SMTPFromEmail,
			FromName:  cfg.SMTPFromName,
		})
	}

	repo := notifications.NewRepository(pool)
	worker := notifications.NewWorker(repo, emailSender, notifications.WorkerConfig{
		MaxAttempts:  cfg.NotificationJobMaxAttempts,
		RetryBackoff: time.Duration(cfg.NotificationJobRetryBackoffS) * time.Second,
	})

	// Internal-only health listener, mirroring video-worker's — never
	// published to the host (see docker-compose.yml).
	go serveHealthz()

	pollInterval := time.Duration(cfg.NotificationJobPollIntervalS) * time.Second
	scanInterval := time.Duration(cfg.NotificationScanIntervalS) * time.Second
	log.Printf("notification-worker: started (poll=%s scan=%s max_attempts=%d warning_days=%d)",
		pollInterval, scanInterval, cfg.NotificationJobMaxAttempts, cfg.SubscriptionExpiryWarningDays)

	lastScan := time.Time{}
	for {
		if time.Since(lastScan) >= scanInterval {
			worker.RunExpiryScan(ctx, cfg.SubscriptionExpiryWarningDays)
			lastScan = time.Now()
		}

		claimed, err := worker.ClaimAndProcess(ctx)
		if err != nil {
			log.Printf("notification-worker: error claiming/processing job: %v", err)
			time.Sleep(pollInterval)
			continue
		}
		if !claimed {
			time.Sleep(pollInterval)
		}
	}
}

func serveHealthz() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if err := http.ListenAndServe(":8091", mux); err != nil {
		log.Printf("notification-worker: healthz listener stopped: %v", err)
	}
}
