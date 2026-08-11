package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	PaymentProvider string

	// S3Endpoint is used for server-to-storage calls (Put/Delete/HeadBucket),
	// reachable only from inside the Docker network. S3PublicEndpoint is
	// used solely when presigning GET URLs, since those are handed to the
	// browser and must resolve from outside that network — the same
	// internal/public split the frontend already uses for its API URL.
	S3Endpoint       string
	S3PublicEndpoint string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3Region         string

	VideoMaxUploadMB     int64
	VideoSignedURLTTLSec int

	// AssignmentMaxUploadMB caps one homework submission file — deliberately
	// separate from VideoMaxUploadMB since assignment attachments (PDF/TXT/
	// ZIP/PNG/JPEG) are expected to be far smaller than lesson videos.
	AssignmentMaxUploadMB int64

	// HLS transcoding (video-worker only reads the job-related ones, but
	// they're loaded from the same Config for both binaries).
	HLSSegmentDurationSec int
	VideoJobMaxAttempts   int
	VideoJobPollIntervalS int
	VideoJobRetryBackoffS int

	// SMTP is only ever read by cmd/notification-worker — the API backend
	// never sends email itself, it only enqueues jobs (see internal/notifications).
	SMTPHost      string
	SMTPPort      string
	SMTPUser      string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string

	NotificationJobMaxAttempts   int
	NotificationJobPollIntervalS int
	NotificationJobRetryBackoffS int

	SubscriptionExpiryWarningDays int
	NotificationScanIntervalS     int

	// Code runner (cmd/code-runner reads all of these; the API backend only
	// reads the queue-poll/retry ones plus the rate-limit ones). System
	// maximums — an exercise's own time_limit_ms/memory_limit_mb may be
	// stricter than these but never looser (see internal/coding/service.go).
	CodeRunnerTimeoutMS            int
	CodeRunnerMemoryMB             int
	CodeRunnerMaxOutputKB          int
	CodeRunnerJobMaxAttempts       int
	CodeRunnerJobPollIntervalS     int
	CodeRunnerJobRetryBackoffS     int
	CodeRunnerMaxSubmissionsPerMin int
	CodeRunnerMaxConcurrentPerUser int
}

func Load() Config {
	s3Endpoint := getEnv("S3_ENDPOINT", "")
	return Config{
		Port:            getEnv("BACKEND_PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		JWTSecret:       getEnv("JWT_SECRET", ""),
		PaymentProvider: getEnv("PAYMENT_PROVIDER", "mock"),

		S3Endpoint:       s3Endpoint,
		S3PublicEndpoint: getEnv("S3_PUBLIC_ENDPOINT", s3Endpoint),
		S3AccessKey:      getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:      getEnv("S3_SECRET_KEY", ""),
		S3Bucket:         getEnv("S3_BUCKET", "lms-videos"),
		S3Region:         getEnv("S3_REGION", "us-east-1"),

		VideoMaxUploadMB:     getEnvInt64("VIDEO_MAX_UPLOAD_MB", 500),
		VideoSignedURLTTLSec: getEnvInt("VIDEO_SIGNED_URL_TTL", 300),

		AssignmentMaxUploadMB: getEnvInt64("ASSIGNMENT_MAX_UPLOAD_MB", 20),

		HLSSegmentDurationSec: getEnvInt("HLS_SEGMENT_DURATION", 6),
		VideoJobMaxAttempts:   getEnvInt("VIDEO_JOB_MAX_ATTEMPTS", 3),
		VideoJobPollIntervalS: getEnvInt("VIDEO_JOB_POLL_INTERVAL_SEC", 3),
		VideoJobRetryBackoffS: getEnvInt("VIDEO_JOB_RETRY_BACKOFF_SEC", 30),

		SMTPHost:      getEnv("SMTP_HOST", ""),
		SMTPPort:      getEnv("SMTP_PORT", "1025"),
		SMTPUser:      getEnv("SMTP_USER", ""),
		SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail: getEnv("SMTP_FROM_EMAIL", "no-reply@lms.local"),
		SMTPFromName:  getEnv("SMTP_FROM_NAME", "LMS Platform"),

		NotificationJobMaxAttempts:   getEnvInt("NOTIFICATION_JOB_MAX_ATTEMPTS", 3),
		NotificationJobPollIntervalS: getEnvInt("NOTIFICATION_JOB_POLL_INTERVAL_SEC", 3),
		NotificationJobRetryBackoffS: getEnvInt("NOTIFICATION_JOB_RETRY_BACKOFF_SEC", 30),

		SubscriptionExpiryWarningDays: getEnvInt("SUBSCRIPTION_EXPIRY_WARNING_DAYS", 3),
		NotificationScanIntervalS:     getEnvInt("NOTIFICATION_SCAN_INTERVAL", 60),

		CodeRunnerTimeoutMS:            getEnvInt("CODE_RUNNER_TIMEOUT_MS", 5000),
		CodeRunnerMemoryMB:             getEnvInt("CODE_RUNNER_MEMORY_MB", 128),
		CodeRunnerMaxOutputKB:          getEnvInt("CODE_RUNNER_MAX_OUTPUT_KB", 64),
		CodeRunnerJobMaxAttempts:       getEnvInt("CODE_RUNNER_JOB_MAX_ATTEMPTS", 2),
		CodeRunnerJobPollIntervalS:     getEnvInt("CODE_RUNNER_JOB_POLL_INTERVAL_SEC", 2),
		CodeRunnerJobRetryBackoffS:     getEnvInt("CODE_RUNNER_JOB_RETRY_BACKOFF_SEC", 10),
		CodeRunnerMaxSubmissionsPerMin: getEnvInt("CODE_RUNNER_MAX_SUBMISSIONS_PER_MINUTE", 15),
		CodeRunnerMaxConcurrentPerUser: getEnvInt("CODE_RUNNER_MAX_CONCURRENT_PER_USER", 3),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}
