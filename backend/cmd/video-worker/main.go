// Command video-worker polls video_jobs and runs FFmpeg transcoding out of
// process from the HTTP backend, so a multi-minute encode never blocks a
// request. It shares this repository's Go module and internal/videos
// package with cmd/api — this is still the same modular monolith, not a
// separate service architecture, split into two binaries only so FFmpeg's
// CPU-bound work can't stall the API.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"lms-backend/internal/config"
	"lms-backend/internal/db"
	"lms-backend/internal/videos"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("video-worker: database connection failed: %v", err)
	}
	defer pool.Close()

	storage, err := videos.NewS3Storage(ctx, videos.S3Config{
		Endpoint:       cfg.S3Endpoint,
		PublicEndpoint: cfg.S3PublicEndpoint,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		Bucket:         cfg.S3Bucket,
		Region:         cfg.S3Region,
	})
	if err != nil {
		log.Fatalf("video-worker: failed to configure storage client: %v", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := storage.Ping(pingCtx); err != nil {
		log.Printf("video-worker: WARNING storage not reachable yet (bucket %q via %s): %v", cfg.S3Bucket, cfg.S3Endpoint, err)
	}
	cancel()

	repo := videos.NewRepository(pool)
	worker := videos.NewWorker(repo, storage, videos.WorkerConfig{
		MaxAttempts:        cfg.VideoJobMaxAttempts,
		SegmentDurationSec: cfg.HLSSegmentDurationSec,
		RetryBackoff:       time.Duration(cfg.VideoJobRetryBackoffS) * time.Second,
	})

	// Internal-only health listener — never published to the host (see
	// docker-compose.yml, video-worker has no "ports:"), it exists purely
	// for Docker's own healthcheck to probe over the compose network.
	go serveHealthz()

	pollInterval := time.Duration(cfg.VideoJobPollIntervalS) * time.Second
	log.Printf("video-worker: started (poll=%s max_attempts=%d segment_duration=%ds)", pollInterval, cfg.VideoJobMaxAttempts, cfg.HLSSegmentDurationSec)

	for {
		claimed, err := worker.ClaimAndProcess(ctx)
		if err != nil {
			log.Printf("video-worker: error claiming/processing job: %v", err)
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
	if err := http.ListenAndServe(":8090", mux); err != nil {
		log.Printf("video-worker: healthz listener stopped: %v", err)
	}
}
