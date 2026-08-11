// Sandboxed execution of untrusted student code.
//
// Trust boundary (disclosed explicitly, not glossed over — see the Stage 16
// final report): every submission is executed via
//
//	timeout -k 2 <wallClockSeconds> \
//	  unshare --user --map-root-user --net --pid --fork -- \
//	  env -i PATH=... HOME=<scratch> GOTOOLCHAIN=local \
//	  sh -c 'ulimit -t <cpu>; ulimit -u <nproc>; ulimit -f <files>; exec "$@"' sh <program>
//
// This was empirically validated (live docker run probing, not just read
// about) before any of this file was written: --user creates a fresh user
// namespace (the process is "root" only inside it, mapped to an unrelated
// uid outside), --net gives it an isolated network stack with no interfaces
// besides loopback (no internet, no access to Postgres/MinIO/SMTP/the
// backend), and --pid --fork gives it its own PID namespace so it can never
// see or signal any process outside the sandbox — killing the wrapper
// collapses the whole namespace. env -i strips every environment variable
// (DATABASE_URL, S3 credentials, JWT secret — none of it is visible to
// student code). The ulimits bound CPU time, process/thread count
// (fork-bomb defense) and file size independently of the wall-clock
// timeout, which is the backstop that unconditionally kills the whole tree
// once CODE_RUNNER_TIMEOUT_MS elapses. Physical memory is bounded at the
// container level instead of per-process (see runSandboxed's doc comment
// on why `ulimit -v` was tried and rejected — it breaks the Go runtime
// outright).
//
// What this is NOT: a mount namespace/chroot jail (attempts to add
// --mount-proc failed in this environment — see the Stage 16 plan) and the
// runner's own process runs as container-root to be able to create these
// namespaces at all (an unprivileged, capability-only invocation could not
// be made to work here either). The container itself never gets the Docker
// socket, --privileged, or any host mount, so "root inside this specific,
// secrets-free, network-isolated container" is the accepted boundary — this
// is an honest development-grade sandbox, not a claim of production-grade
// multi-tenant isolation (Kubernetes/Firecracker/gVisor were explicitly out
// of scope for this stage).
package coding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// sandboxPATH is deliberately narrow — only where the three toolchains
// live in the code-runner image (see Dockerfile.runner). Nothing under
// /usr/local/sbin, no user-writable directories.
const sandboxPATH = "/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"

// maxProcesses/maxFileBlocks are fixed, conservative sandbox-wide caps
// (fork-bomb and disk-fill defense) — not exposed as config since they're
// an implementation detail of the sandbox, not a per-exercise tuning knob.
const (
	maxProcesses  = 64
	maxFileBlocks = 65536 // 512-byte blocks => 32MB of total file writes
)

type sandboxResult struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	TimedOut  bool
	Truncated bool
}

// limitedBuffer caps how much of a process's output we ever hold in memory
// or persist — CODE_RUNNER_MAX_OUTPUT_KB (item 15). Writes past the cap are
// silently dropped (never causes the child to block or error) with
// Truncated recorded once, so the caller can attach a clear "output
// truncated" marker instead of pretending the capture is complete.
type limitedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len() < b.max {
		remaining := b.max - b.buf.Len()
		if len(p) <= remaining {
			b.buf.Write(p)
		} else {
			b.buf.Write(p[:remaining])
			b.truncated = true
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	if b.truncated {
		return b.buf.String() + "\n[... output truncated ...]"
	}
	return b.buf.String()
}

// runSandboxed executes argv (relative to workDir, e.g. "./prog" or
// []string{"python3", "main.py"}) inside the isolation wrapper described
// above. A non-nil error means the sandbox machinery itself failed to even
// start (timeout/unshare missing, workDir unusable) — an infrastructure
// failure distinct from the *sandboxed program* exiting non-zero, timing
// out, or being killed, all of which are reported via sandboxResult.
func runSandboxed(ctx context.Context, workDir string, timeoutMS, memoryMB, maxOutputKB int, stdin string, argv []string) (*sandboxResult, error) {
	wallSeconds := float64(timeoutMS) / 1000.0
	if wallSeconds < 1 {
		wallSeconds = 1
	}
	cpuSeconds := int(math.Ceil(wallSeconds)) + 1
	_ = memoryMB // see comment below — kept in the signature for the caller's clamped-limits bookkeeping, not passed to ulimit

	// No `ulimit -v` here — empirically, the Go runtime (both the compiler
	// and any compiled binary) reserves roughly 1GB of *virtual* address
	// space just to initialize its page allocator, regardless of actual
	// usage; RLIMIT_AS (what `ulimit -v` sets) counts that reservation, not
	// resident memory, so a per-exercise-sized -v (e.g. 128MB) makes every
	// single Go submission fail at startup with "failed to reserve page
	// summary memory" before it runs a line of student code. RLIMIT_AS was
	// tested and rejected for this reason. Physical memory is instead
	// bounded at the container level (see docker-compose.yml's code-runner
	// `mem_limit`) — which is sufficient because cmd/code-runner's poll
	// loop processes exactly one job at a time (see main.go), so the
	// container's own memory ceiling doubles as a de facto per-execution
	// cap. This is a disclosed trade-off, not an oversight: it bounds the
	// host (the mandatory requirement) but doesn't give each individual
	// execution its own hard, isolated memory ceiling the way CPU/wall-
	// clock/process-count/output-size do below.
	innerScript := fmt.Sprintf(
		`ulimit -t %d; ulimit -u %d; ulimit -f %d; exec "$@"`,
		cpuSeconds, maxProcesses, maxFileBlocks,
	)

	args := []string{
		"timeout", "-k", "2", fmt.Sprintf("%.1f", wallSeconds),
		"unshare", "--user", "--map-root-user", "--net", "--pid", "--fork", "--",
		"env", "-i",
		"PATH=" + sandboxPATH,
		"HOME=" + workDir,
		// No TMPDIR override: Go's toolchain refuses to treat a directory
		// as a module root when it equals $TMPDIR ("ignoring go.mod in
		// system temp root") — empirically confirmed to fire even for a
		// freshly created, randomly named scratch dir once TMPDIR was set
		// to that same path. Leaving TMPDIR unset defaults it to /tmp,
		// which is never equal to workDir (workDir is itself a fresh
		// subdirectory under system temp), so the check no longer
		// misfires.
		//
		// GOTOOLCHAIN=local stops `go build` from ever trying to fetch a
		// different Go version over the network to satisfy go.mod's `go`
		// directive — it must use exactly whatever `go` is on PATH inside
		// the sandbox, since the sandbox has no network access at all.
		"GOTOOLCHAIN=local",
		"sh", "-c", innerScript, "sh",
	}
	args = append(args, argv...)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(stdin)

	stdout := &limitedBuffer{max: maxOutputKB * 1024}
	stderr := &limitedBuffer{max: maxOutputKB * 1024}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	exitCode := 0
	timedOut := false
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
			// GNU `timeout` exits 124 when it had to TERM the child, and
			// 128+signal (137 for SIGKILL, since -k escalates to KILL) when
			// the child ignored TERM and had to be force-killed.
			if exitCode == 124 || exitCode == 137 {
				timedOut = true
			}
		} else {
			return nil, fmt.Errorf("sandbox exec failed: %w", runErr)
		}
	}

	return &sandboxResult{
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		ExitCode:  exitCode,
		TimedOut:  timedOut,
		Truncated: stdout.truncated || stderr.truncated,
	}, nil
}

// LanguageRunner is the one seam between "how do I compile/run this
// language" and everything else (queue claiming, test evaluation, resource
// limits) — adding a fourth language later means writing one small type
// that satisfies this interface, never touching service.go, handler.go, or
// the sandbox wrapper above.
type LanguageRunner interface {
	// Prepare writes sourceCode into workDir in whatever layout this
	// language needs.
	Prepare(workDir, sourceCode string) error
	// Compile returns the sandboxed argv to run to produce a runnable
	// program, or nil if this language has no separate compile step.
	Compile(workDir string) []string
	// Run returns the sandboxed argv that executes the prepared program.
	Run(workDir string) []string
}

// --- Go --------------------------------------------------------------

type GoRunner struct{}

func (GoRunner) Prepare(workDir, sourceCode string) error {
	// A minimal go.mod (no requires) keeps `go build` in module mode
	// without ever needing network access — student code is a single file
	// limited to the standard library.
	// The `go` directive is deliberately conservative (not this repo's own
	// go.mod version) — it only needs to be <= whatever Go toolchain
	// Dockerfile.runner actually installs, and GOTOOLCHAIN=local (see
	// runSandboxed) means a mismatch fails the build instead of trying to
	// fetch a newer toolchain over network the sandbox doesn't have.
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module sandbox\n\ngo 1.21\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workDir, "main.go"), []byte(sourceCode), 0o644)
}

func (GoRunner) Compile(workDir string) []string {
	return []string{"go", "build", "-o", "prog", "."}
}

func (GoRunner) Run(workDir string) []string {
	return []string{"./prog"}
}

// --- Python ------------------------------------------------------------

type PythonRunner struct{}

func (PythonRunner) Prepare(workDir, sourceCode string) error {
	return os.WriteFile(filepath.Join(workDir, "main.py"), []byte(sourceCode), 0o644)
}

func (PythonRunner) Compile(workDir string) []string {
	return nil // interpreted — syntax errors surface as a runtime_error traceback, not a separate compile step
}

func (PythonRunner) Run(workDir string) []string {
	// -I: isolated mode — ignores PYTHONPATH/user site-packages and other
	// environment-derived configuration, on top of env -i already having
	// stripped the environment itself.
	return []string{"python3", "-I", "main.py"}
}

// --- JavaScript / Node ---------------------------------------------------

type NodeRunner struct{}

func (NodeRunner) Prepare(workDir, sourceCode string) error {
	return os.WriteFile(filepath.Join(workDir, "main.js"), []byte(sourceCode), 0o644)
}

func (NodeRunner) Compile(workDir string) []string {
	return nil
}

func (NodeRunner) Run(workDir string) []string {
	return []string{"node", "main.js"}
}

// DefaultRunners is what cmd/code-runner wires up — keyed by the same
// language strings the coding_exercises.language CHECK constraint allows.
func DefaultRunners() map[string]LanguageRunner {
	return map[string]LanguageRunner{
		"go":         GoRunner{},
		"python":     PythonRunner{},
		"javascript": NodeRunner{},
	}
}

// --- output comparison ---------------------------------------------------

// normalizeOutput implements item 17's "normalized (not fuzzy) comparison":
// CRLF -> LF, trailing whitespace trimmed per line, and trailing blank
// lines trimmed — never a semantic/whitespace-insensitive diff beyond that.
func normalizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// --- compile diagnostics sanitization -------------------------------------

// sanitizeCompileOutput strips the sandbox scratch directory's absolute
// path from compiler diagnostics — go build is always invoked with Dir=
// workDir and a relative filename, so diagnostics are relative by
// construction ("./main.go:5:2: ..."), but this is a defense-in-depth pass
// in case a future language runner's compiler ever emits an absolute path,
// GOROOT location, or environment value into its error text.
func sanitizeCompileOutput(raw, workDir string) string {
	s := raw
	s = strings.ReplaceAll(s, workDir+"/", "")
	s = strings.ReplaceAll(s, workDir, "")
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		s = strings.ReplaceAll(s, goroot, "<go>")
	}
	const maxLen = 8000
	if len(s) > maxLen {
		s = s[:maxLen] + "\n[... truncated ...]"
	}
	return s
}

// --- one submission's execution -------------------------------------------

// executionLimits is the effective, already-clamped set of limits for one
// submission — see Worker.execute for the clamp (exercise limit may be
// stricter than the system max, but never looser — item "система лимитов").
type executionLimits struct {
	TimeoutMS   int
	MemoryMB    int
	MaxOutputKB int
}

// runOne runs the prepared program once against a single stdin and reports
// its outcome — the unit both run-mode (one execution) and submit-mode
// (one execution per test case) build on.
func runOne(ctx context.Context, workDir string, argv []string, limits executionLimits, stdin string) (*sandboxResult, error) {
	return runSandboxed(ctx, workDir, limits.TimeoutMS, limits.MemoryMB, limits.MaxOutputKB, stdin, argv)
}

// --- worker: claim + process one job at a time ----------------------------

type WorkerConfig struct {
	MaxAttempts  int
	RetryBackoff time.Duration

	// System-wide maximums (CODE_RUNNER_TIMEOUT_MS / _MEMORY_MB /
	// _MAX_OUTPUT_KB) — an exercise's own time_limit_ms/memory_limit_mb
	// may be stricter but is always clamped down to these, never up.
	SystemTimeoutMS   int
	SystemMemoryMB    int
	SystemMaxOutputKB int
}

// Worker holds everything cmd/code-runner needs to claim and process one
// submission at a time. It never serves HTTP (see cmd/code-runner/main.go
// for the tiny internal-only health listener, same shape as video-worker's).
type Worker struct {
	repo    *Repository
	cfg     WorkerConfig
	runners map[string]LanguageRunner
}

func NewWorker(repo *Repository, cfg WorkerConfig, runners map[string]LanguageRunner) *Worker {
	return &Worker{repo: repo, cfg: cfg, runners: runners}
}

// ClaimAndProcess claims at most one pending job and processes it fully (or
// records its failure/retry) before returning. claimed=false means the
// queue was empty, so the caller's poll loop knows to sleep before
// checking again — same contract as videos.Worker.ClaimAndProcess.
func (w *Worker) ClaimAndProcess(ctx context.Context) (claimed bool, err error) {
	job, err := w.repo.ClaimNextJob(ctx)
	if errors.Is(err, ErrNoJobAvailable) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	log.Printf("code-runner: claimed job=%s submission=%s attempt=%d", job.ID, job.SubmissionID, job.Attempts)

	execCtx, loadErr := w.repo.GetExecutionContext(ctx, job.SubmissionID)
	if loadErr != nil {
		return true, w.retryOrFail(ctx, job, fmt.Errorf("load submission: %w", loadErr))
	}

	if err := w.repo.MarkSubmissionRunning(ctx, job.SubmissionID); err != nil {
		return true, w.retryOrFail(ctx, job, fmt.Errorf("mark running: %w", err))
	}

	result, procErr := w.execute(ctx, execCtx)
	if procErr != nil {
		log.Printf("code-runner: job=%s submission=%s infrastructure failure: %v", job.ID, job.SubmissionID, procErr)
		return true, w.retryOrFail(ctx, job, procErr)
	}

	includeStdout := execCtx.Submission.Mode == ModeRun
	if err := w.repo.MarkSubmissionResult(ctx, job.SubmissionID, *result, includeStdout); err != nil {
		return true, w.retryOrFail(ctx, job, fmt.Errorf("persist result: %w", err))
	}
	if err := w.repo.MarkJobCompleted(ctx, job.ID); err != nil {
		return true, err
	}
	log.Printf("code-runner: job=%s submission=%s finished status=%s passed=%d/%d", job.ID, job.SubmissionID, result.Status, result.PassedTests, result.TotalTests)
	return true, nil
}

// retryOrFail requeues or permanently fails the job on an infrastructure
// error (never on the student's code merely failing/timing out — that's a
// normal, successfully-processed outcome, not an error). If retries are
// exhausted, the submission itself is marked internal_error so it never
// hangs forever in "queued"/"running" for the student (item "submission
// никогда не теряется").
func (w *Worker) retryOrFail(ctx context.Context, job *ExecutionJob, procErr error) error {
	retried, markErr := w.repo.MarkJobFailedOrRetry(ctx, job.ID, job.Attempts, w.cfg.MaxAttempts, truncateError(procErr), w.cfg.RetryBackoff)
	if markErr != nil {
		return markErr
	}
	if retried {
		log.Printf("code-runner: job=%s will retry (attempt %d/%d): %v", job.ID, job.Attempts, w.cfg.MaxAttempts, procErr)
		return nil
	}
	log.Printf("code-runner: job=%s exhausted retries, marking submission=%s internal_error: %v", job.ID, job.SubmissionID, procErr)
	return w.repo.MarkSubmissionResult(ctx, job.SubmissionID, ExecutionResult{Status: StatusInternalError}, false)
}

func truncateError(err error) string {
	msg := err.Error()
	const max = 2000
	if len(msg) > max {
		return msg[:max]
	}
	return msg
}

// clampLimits enforces "exercise limits may be stricter, never looser than
// the system max".
func (w *Worker) clampLimits(exerciseTimeoutMS, exerciseMemoryMB int) executionLimits {
	timeoutMS := exerciseTimeoutMS
	if timeoutMS <= 0 || timeoutMS > w.cfg.SystemTimeoutMS {
		timeoutMS = w.cfg.SystemTimeoutMS
	}
	memoryMB := exerciseMemoryMB
	if memoryMB <= 0 || memoryMB > w.cfg.SystemMemoryMB {
		memoryMB = w.cfg.SystemMemoryMB
	}
	return executionLimits{TimeoutMS: timeoutMS, MemoryMB: memoryMB, MaxOutputKB: w.cfg.SystemMaxOutputKB}
}

// execute prepares and runs one submission end to end. A non-nil error is
// strictly an infrastructure failure (bad workDir, sandbox tooling
// missing) — every other outcome (compile error, wrong answer, timeout,
// crash) is a normal, successfully-computed ExecutionResult.
func (w *Worker) execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	submission := execCtx.Submission
	runner, ok := w.runners[submission.Language]
	if !ok {
		return nil, fmt.Errorf("no runner registered for language %q", submission.Language)
	}
	limits := w.clampLimits(execCtx.TimeLimitMS, execCtx.MemoryLimitMB)

	workDir, err := os.MkdirTemp("", "code-exec-*")
	if err != nil {
		return nil, fmt.Errorf("create scratch dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := runner.Prepare(workDir, submission.SourceCode); err != nil {
		return nil, fmt.Errorf("prepare source: %w", err)
	}

	totalTests := len(execCtx.TestCases)

	if compileArgv := runner.Compile(workDir); compileArgv != nil {
		compileResult, err := runOne(ctx, workDir, compileArgv, limits, "")
		if err != nil {
			return nil, fmt.Errorf("run compile step: %w", err)
		}
		if compileResult.ExitCode != 0 {
			diag := sanitizeCompileOutput(compileResult.Stderr+compileResult.Stdout, workDir)
			return &ExecutionResult{Status: StatusCompileError, TotalTests: totalTests, CompileOutput: diag}, nil
		}
	}

	runArgv := runner.Run(workDir)

	if submission.Mode == ModeRun {
		return w.executeRun(ctx, workDir, runArgv, limits, execCtx.TestCases)
	}
	return w.executeSubmit(ctx, workDir, runArgv, limits, execCtx.TestCases)
}

// executeRun is the "Run Code" button: a single execution against the
// first worked example (if any), purely to show raw output — never graded,
// so PassedTests/TotalTests stay 0 regardless of what happens.
func (w *Worker) executeRun(ctx context.Context, workDir string, argv []string, limits executionLimits, testCases []TestCase) (*ExecutionResult, error) {
	stdin := ""
	for _, tc := range testCases {
		if !tc.Hidden {
			if tc.Input != nil {
				stdin = *tc.Input
			}
			break
		}
	}

	result, err := runOne(ctx, workDir, argv, limits, stdin)
	if err != nil {
		return nil, fmt.Errorf("run program: %w", err)
	}

	status := StatusPassed
	switch {
	case result.TimedOut:
		status = StatusTimeout
	case result.ExitCode != 0:
		status = StatusRuntimeError
	}

	return &ExecutionResult{
		Status:          status,
		ExecutionTimeMS: 0,
		Stdout:          combinedOutput(result),
	}, nil
}

// executeSubmit is the official, completion-eligible grading run: every
// test case (hidden and visible), stopping at the first timeout/crash so a
// broken program doesn't burn the full suite's worth of sandbox time.
// Stdout is intentionally left unset on the returned result — repository.go's
// MarkSubmissionResult is always called with includeStdout=false for
// mode="submit" (see model.go's Submission doc comment for why: a hidden
// test case's input could otherwise leak back to the student verbatim).
func (w *Worker) executeSubmit(ctx context.Context, workDir string, argv []string, limits executionLimits, testCases []TestCase) (*ExecutionResult, error) {
	total := len(testCases)
	passed := 0

	for _, tc := range testCases {
		stdin := ""
		if tc.Input != nil {
			stdin = *tc.Input
		}

		result, err := runOne(ctx, workDir, argv, limits, stdin)
		if err != nil {
			return nil, fmt.Errorf("run test case %d: %w", tc.Position, err)
		}

		if result.TimedOut {
			return &ExecutionResult{Status: StatusTimeout, PassedTests: passed, TotalTests: total}, nil
		}
		if result.ExitCode != 0 {
			return &ExecutionResult{Status: StatusRuntimeError, PassedTests: passed, TotalTests: total}, nil
		}
		if normalizeOutput(result.Stdout) == normalizeOutput(tc.ExpectedOutput) {
			passed++
		}
	}

	status := StatusFailed
	if passed >= total {
		status = StatusPassed
	}
	return &ExecutionResult{Status: status, PassedTests: passed, TotalTests: total}, nil
}

func combinedOutput(r *sandboxResult) string {
	if r.Stderr == "" {
		return r.Stdout
	}
	if r.Stdout == "" {
		return r.Stderr
	}
	return r.Stdout + "\n" + r.Stderr
}
