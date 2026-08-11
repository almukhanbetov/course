// Command code-runner polls code_execution_jobs and runs untrusted student
// code out of process from the HTTP backend — the backend never executes
// student code itself, never mounts the Docker socket, and never runs in
// privileged mode (see internal/coding/runner.go's package doc for the full
// isolation design and its disclosed limitations). It shares this
// repository's Go module and internal/coding package with cmd/api, same as
// cmd/video-worker/cmd/notification-worker — still one modular monolith,
// split into a separate binary/container only so a runaway or malicious
// submission can never share a process (or even a container) with the API,
// PostgreSQL, or the frontend.
package main

import (
	"context"
	"log"
	"net/http"
	"os/exec"
	"time"

	"lms-backend/internal/coding"
	"lms-backend/internal/config"
	"lms-backend/internal/db"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("code-runner: database connection failed: %v", err)
	}
	defer pool.Close()

	checkSandboxTooling()

	repo := coding.NewRepository(pool)
	worker := coding.NewWorker(repo, coding.WorkerConfig{
		MaxAttempts:       cfg.CodeRunnerJobMaxAttempts,
		RetryBackoff:      time.Duration(cfg.CodeRunnerJobRetryBackoffS) * time.Second,
		SystemTimeoutMS:   cfg.CodeRunnerTimeoutMS,
		SystemMemoryMB:    cfg.CodeRunnerMemoryMB,
		SystemMaxOutputKB: cfg.CodeRunnerMaxOutputKB,
	}, coding.DefaultRunners())

	// Internal-only health listener — never published to the host (see
	// docker-compose.yml, code-runner has no "ports:"), same pattern as
	// video-worker's :8090 and notification-worker's :8091.
	go serveHealthz()

	pollInterval := time.Duration(cfg.CodeRunnerJobPollIntervalS) * time.Second
	log.Printf("code-runner: started (poll=%s max_attempts=%d timeout=%dms memory=%dMB max_output=%dKB)",
		pollInterval, cfg.CodeRunnerJobMaxAttempts, cfg.CodeRunnerTimeoutMS, cfg.CodeRunnerMemoryMB, cfg.CodeRunnerMaxOutputKB)

	for {
		claimed, err := worker.ClaimAndProcess(ctx)
		if err != nil {
			log.Printf("code-runner: error claiming/processing job: %v", err)
			time.Sleep(pollInterval)
			continue
		}
		if !claimed {
			time.Sleep(pollInterval)
		}
	}
}

// checkSandboxTooling verifies the isolation wrapper's dependencies exist
// at startup — a missing `unshare`/`timeout` would otherwise fail silently
// on the very first submission. Failure here doesn't stop the process (the
// container might still be useful for e.g. Go-only exercises if a later
// image build breaks Python), it only warns loudly.
func checkSandboxTooling() {
	for _, bin := range []string{"timeout", "unshare", "env", "sh"} {
		if _, err := exec.LookPath(bin); err != nil {
			log.Printf("code-runner: WARNING sandbox dependency %q not found on PATH — submissions will fail: %v", bin, err)
		}
	}
}

// languageAvailable reports whether a language's toolchain is on PATH,
// without ever revealing its resolved path or version to the health probe
// response body (item "не показывай version/paths").
func languageAvailable(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func serveHealthz() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		languages := map[string]bool{
			"go":         languageAvailable("go"),
			"python":     languageAvailable("python3"),
			"javascript": languageAvailable("node"),
		}
		allHealthy := true
		for _, ok := range languages {
			if !ok {
				allHealthy = false
			}
		}
		if !allHealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("degraded"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if err := http.ListenAndServe(":8092", mux); err != nil {
		log.Printf("code-runner: healthz listener stopped: %v", err)
	}
}
