//go:build stress

package stress

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gopoke/internal/app"
	"gopoke/internal/execution"
	"gopoke/internal/testutil"
)

const (
	defaultStressIterations  = 10
	reliabilityTarget        = 0.99
	stressRunTimeout         = 20000
	cancelRequestDelay       = 250 * time.Millisecond
	stressCancelWaitTimeout  = 3 * time.Second
	stressShutdownWaitTimout = app.DefaultShutdownTimeout
)

type stressOutcome struct {
	result execution.Result
	err    error
}

func TestRunCancelReliabilityStress(t *testing.T) {
	requireGoToolchain(t)

	iterations := readStressIterations(t)
	projectPath := createProject(t)

	application := app.NewWithDataRoot(filepath.Join(t.TempDir(), "stress-home"))
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), stressShutdownWaitTimout)
		defer cancel()
		if err := application.Stop(shutdownCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	}()

	if _, err := application.OpenProject(context.Background(), projectPath); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	snippet := strings.Join([]string{
		"package main",
		"",
		"import \"time\"",
		"",
		"func main() {",
		"\tfor {",
		"\t\ttime.Sleep(100 * time.Millisecond)",
		"\t}",
		"}",
		"",
	}, "\n")

	goroutinesBefore := runtime.NumGoroutine()

	successes := 0
	failures := make([]string, 0)
	var wg sync.WaitGroup

	for iteration := 1; iteration <= iterations; iteration++ {
		t.Logf("STRESS_ITERATION start=%d/%d", iteration, iterations)
		runID := fmt.Sprintf("stress-run-%03d", iteration)
		outcomeCh := make(chan stressOutcome, 1)

		iterCtx, iterCancel := testutil.TestRunContext(t)

		wg.Add(1)
		go func(runID string) {
			defer wg.Done()
			result, err := application.RunSnippet(
				iterCtx,
				execution.RunRequest{
					RunID:       runID,
					ProjectPath: projectPath,
					TimeoutMS:   stressRunTimeout,
					Source:      snippet,
				},
				nil,
				nil,
			)
			outcomeCh <- stressOutcome{result: result, err: err}
		}(runID)

		time.Sleep(cancelRequestDelay)

		if err := application.CancelRun(context.Background(), runID); err != nil {
			iterCancel()
			failures = append(failures, fmt.Sprintf("iteration=%d runID=%s reason=cancel_error error=%q", iteration, runID, err.Error()))
			continue
		}

		select {
		case outcome := <-outcomeCh:
			iterCancel()
			if outcome.err != nil {
				failures = append(failures, fmt.Sprintf("iteration=%d runID=%s reason=run_error error=%q", iteration, runID, outcome.err.Error()))
				continue
			}
			if !outcome.result.Canceled {
				failures = append(failures, fmt.Sprintf(
					"iteration=%d runID=%s reason=unexpected_result canceled=%t timed_out=%t exit_code=%d duration_ms=%d stderr_tail=%q",
					iteration,
					runID,
					outcome.result.Canceled,
					outcome.result.TimedOut,
					outcome.result.ExitCode,
					outcome.result.DurationMS,
					tail(outcome.result.Stderr, 160),
				))
				continue
			}
			successes++
		case <-time.After(stressCancelWaitTimeout):
			iterCancel()
			failures = append(failures, fmt.Sprintf("iteration=%d runID=%s reason=cancel_did_not_complete_within=%s", iteration, runID, stressCancelWaitTimeout))
		}
	}

	// Wait for all spawned goroutines before checking leak counts.
	wg.Wait()

	reliability := float64(successes) / float64(iterations)
	t.Logf("STRESS_SUMMARY iterations=%d successes=%d failures=%d reliability=%.4f target=%.4f", iterations, successes, len(failures), reliability, reliabilityTarget)
	for _, failure := range failures {
		t.Logf("STRESS_FAILURE %s", failure)
	}

	if reliability < reliabilityTarget {
		t.Fatalf("run/cancel reliability %.4f below target %.4f (%d/%d successes)", reliability, reliabilityTarget, successes, iterations)
	}

	// Allow runtime goroutines to settle after all iterations.
	time.Sleep(500 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	if delta := goroutinesAfter - goroutinesBefore; delta > 5 {
		t.Errorf("goroutine leak detected: before=%d after=%d delta=%d", goroutinesBefore, goroutinesAfter, delta)
	}
}

func readStressIterations(t *testing.T) int {
	t.Helper()
	value := strings.TrimSpace(os.Getenv("GOPOKE_STRESS_ITERATIONS"))
	if value == "" {
		return defaultStressIterations
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		t.Fatalf("invalid GOPOKE_STRESS_ITERATIONS value %q", value)
	}
	return parsed
}

func createProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/gopoke/stress\n\ngo 1.20\n")
	writeFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	return root
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func requireGoToolchain(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}
}

func tail(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
