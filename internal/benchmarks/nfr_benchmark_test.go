package benchmarks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gopoke/internal/app"
	"gopoke/internal/execution"
	"gopoke/internal/testutil"
)

const nfrBenchmarkRunTimeoutMS = 30000

func BenchmarkNFRStartupLatency(b *testing.B) {
	baseDataRoot := b.TempDir()

	var totalStartup time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dataRoot := filepath.Join(baseDataRoot, fmt.Sprintf("startup-%d", i))
		application := app.NewWithDataRoot(dataRoot)

		startedAt := time.Now()
		if err := application.Start(context.Background()); err != nil {
			b.Fatalf("Start() error = %v", err)
		}
		totalStartup += time.Since(startedAt)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.DefaultShutdownTimeout)
		if err := application.Stop(shutdownCtx); err != nil {
			cancel()
			b.Fatalf("Stop() error = %v", err)
		}
		cancel()
	}
	b.StopTimer()

	startupMillis := totalStartup.Seconds() * 1000 / float64(b.N)
	b.ReportMetric(startupMillis, "startup_ms/op")
	b.Logf("startup mean: %.2fms", startupMillis)
}

func BenchmarkNFRWarmRunTriggerLatency(b *testing.B) {
	requireGoToolchain(b)

	application, projectPath, stop := setupBenchmarkApplication(b)
	defer stop()

	// Prime worker process so trigger latency represents warm-run behavior.
	if _, err := application.StartProjectWorker(context.Background(), projectPath); err != nil {
		b.Fatalf("StartProjectWorker(warmup) error = %v", err)
	}

	var totalTrigger time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startedAt := time.Now()
		if _, err := application.StartProjectWorker(context.Background(), projectPath); err != nil {
			b.Fatalf("StartProjectWorker(iteration %d) error = %v", i, err)
		}
		totalTrigger += time.Since(startedAt)
	}
	b.StopTimer()

	triggerMillis := totalTrigger.Seconds() * 1000 / float64(b.N)
	b.ReportMetric(triggerMillis, "warm_trigger_ms/op")
	b.Logf("warm run trigger mean: %.2fms", triggerMillis)
}

func BenchmarkNFRFirstFeedbackLatency(b *testing.B) {
	requireGoToolchain(b)

	application, projectPath, stop := setupBenchmarkApplication(b)
	defer stop()

	snippet := "package main\n\nimport \"fmt\"\n\nfunc main(){fmt.Print(\"feedback\")}\n"
	warmupCtx, warmupCancel := testutil.TestRunContext(b)
	defer warmupCancel()
	if _, err := application.RunSnippet(
		warmupCtx,
		execution.RunRequest{
			ProjectPath: projectPath,
			TimeoutMS:   nfrBenchmarkRunTimeoutMS,
			Source:      snippet,
		},
		nil,
		nil,
	); err != nil {
		b.Fatalf("RunSnippet(warmup) error = %v", err)
	}

	var totalFirstFeedback time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startedAt := time.Now()
		firstFeedbackAt := time.Time{}
		var firstFeedbackOnce sync.Once

		iterCtx, iterCancel := testutil.TestRunContext(b)
		result, err := application.RunSnippet(
			iterCtx,
			execution.RunRequest{
				RunID:       fmt.Sprintf("nfr-first-feedback-%d", i),
				ProjectPath: projectPath,
				TimeoutMS:   nfrBenchmarkRunTimeoutMS,
				Source:      snippet,
			},
			func(chunk string) {
				if chunk == "" {
					return
				}
				firstFeedbackOnce.Do(func() {
					firstFeedbackAt = time.Now()
				})
			},
			func(chunk string) {
				if chunk == "" {
					return
				}
				firstFeedbackOnce.Do(func() {
					firstFeedbackAt = time.Now()
				})
			},
		)
		if err != nil {
			b.Fatalf("RunSnippet(iteration %d) error = %v", i, err)
		}
		if result.ExitCode != 0 {
			b.Fatalf("RunSnippet(iteration %d) exit code = %d, want 0", i, result.ExitCode)
		}
		if firstFeedbackAt.IsZero() {
			firstFeedbackAt = time.Now()
		}
		totalFirstFeedback += firstFeedbackAt.Sub(startedAt)
		iterCancel()
	}
	b.StopTimer()

	firstFeedbackMillis := totalFirstFeedback.Seconds() * 1000 / float64(b.N)
	b.ReportMetric(firstFeedbackMillis, "first_feedback_ms/op")
	b.Logf("first feedback mean: %.2fms", firstFeedbackMillis)
}

func setupBenchmarkApplication(b *testing.B) (*app.Application, string, func()) {
	b.Helper()

	suiteRoot := b.TempDir()
	dataRoot := filepath.Join(suiteRoot, "app-data")
	projectPath := filepath.Join(suiteRoot, "project")

	writeFile(b, filepath.Join(projectPath, "go.mod"), "module example.com/gopoke/bench\n\ngo 1.20\n")
	writeFile(b, filepath.Join(projectPath, "main.go"), "package main\n\nfunc main() {}\n")

	application := app.NewWithDataRoot(dataRoot)
	if err := application.Start(context.Background()); err != nil {
		b.Fatalf("Start() error = %v", err)
	}

	if _, err := application.OpenProject(context.Background(), projectPath); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.DefaultShutdownTimeout)
		_ = application.Stop(shutdownCtx)
		cancel()
		b.Fatalf("OpenProject() error = %v", err)
	}

	stopFn := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.DefaultShutdownTimeout)
		defer cancel()
		if err := application.Stop(shutdownCtx); err != nil {
			b.Fatalf("Stop() error = %v", err)
		}
	}
	return application, projectPath, stopFn
}

func requireGoToolchain(b *testing.B) {
	b.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		b.Skip("go binary not available")
	}
}

func writeFile(b *testing.B, path string, contents string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		b.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
