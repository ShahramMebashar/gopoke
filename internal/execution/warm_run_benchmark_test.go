package execution

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func BenchmarkWarmRunLatency(b *testing.B) {
	if _, err := exec.LookPath("go"); err != nil {
		b.Skip("go binary not available")
	}

	projectDir := b.TempDir()
	snippet := "package main\n\nimport \"fmt\"\n\nfunc main(){fmt.Println(\"warm-run\")}\n"

	// Prime module/build caches before benchmark iterations.
	if warmupResult, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
		Timeout: 5 * time.Second,
	}); err != nil {
		b.Fatalf("warmup run error = %v", err)
	} else if warmupResult.ExitCode != 0 {
		b.Fatalf("warmup run exit code = %d, want 0", warmupResult.ExitCode)
	}

	var mu sync.Mutex
	var totalFirstOutput time.Duration
	var totalDuration time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startedAt := time.Now()
		firstOutputAt := time.Time{}
		result, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
			Timeout: 5 * time.Second,
			OnStdoutChunk: func(chunk string) {
				if chunk == "" {
					return
				}
				mu.Lock()
				if firstOutputAt.IsZero() {
					firstOutputAt = time.Now()
				}
				mu.Unlock()
			},
		})
		if err != nil {
			b.Fatalf("benchmark run error = %v", err)
		}
		if result.ExitCode != 0 {
			b.Fatalf("benchmark run exit code = %d, want 0", result.ExitCode)
		}

		completedAt := time.Now()
		if firstOutputAt.IsZero() {
			firstOutputAt = completedAt
		}
		totalFirstOutput += firstOutputAt.Sub(startedAt)
		totalDuration += completedAt.Sub(startedAt)
	}
	b.StopTimer()

	firstOutputMillis := float64(totalFirstOutput.Milliseconds()) / float64(b.N)
	totalRunMillis := float64(totalDuration.Milliseconds()) / float64(b.N)
	b.ReportMetric(firstOutputMillis, "first_output_ms/op")
	b.ReportMetric(totalRunMillis, "run_duration_ms/op")
	b.Logf("warm-run trigger-to-first-output mean: %.2fms", firstOutputMillis)
	b.Logf("warm-run total-duration mean: %.2fms", totalRunMillis)
}
