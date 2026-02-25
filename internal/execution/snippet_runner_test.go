package execution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunGoSnippet(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	t.Run("success", func(t *testing.T) {
		t.Parallel()

		snippet := "package main\nimport \"fmt\"\nfunc main(){fmt.Print(\"ok\")}\n"
		result, err := RunGoSnippet(context.Background(), projectDir, snippet, 0)
		if err != nil {
			t.Fatalf("RunGoSnippet() error = %v", err)
		}
		if got, want := result.ExitCode, 0; got != want {
			t.Fatalf("ExitCode = %d, want %d", got, want)
		}
		if got, want := result.Stdout, "ok"; got != want {
			t.Fatalf("Stdout = %q, want %q", got, want)
		}
	})

	t.Run("compile error", func(t *testing.T) {
		t.Parallel()

		result, err := RunGoSnippet(context.Background(), projectDir, "package main\nfunc main( {}\n", 0)
		if err != nil {
			t.Fatalf("RunGoSnippet() error = %v", err)
		}
		if result.ExitCode == 0 {
			t.Fatal("ExitCode = 0, want non-zero")
		}
		if result.Stderr == "" {
			t.Fatal("Stderr is empty, want compiler output")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		snippet := "package main\nimport \"time\"\nfunc main(){time.Sleep(2*time.Second)}\n"
		result, err := RunGoSnippet(context.Background(), projectDir, snippet, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("RunGoSnippet() error = %v", err)
		}
		if !result.TimedOut {
			t.Fatal("TimedOut = false, want true")
		}
	})
}

func TestRunGoSnippetWithOptionsUsesWorkingDirectoryAndEnvironment(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	workingDir := filepath.Join(projectDir, "cmd", "api")
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workingDir) error = %v", err)
	}

	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"os\"",
		")",
		"",
		"func main() {",
		"\twd, err := os.Getwd()",
		"\tif err != nil {",
		"\t\tpanic(err)",
		"\t}",
		"\tfmt.Printf(\"%s|%s\", wd, os.Getenv(\"GOPOKE_SNIPPET_ENV\"))",
		"}",
		"",
	}, "\n")

	result, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
		WorkingDirectory: workingDir,
		Environment: map[string]string{
			"GOPOKE_SNIPPET_ENV": "enabled",
		},
	})
	if err != nil {
		t.Fatalf("RunGoSnippetWithOptions() error = %v", err)
	}
	if got, want := result.ExitCode, 0; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}

	parts := strings.SplitN(result.Stdout, "|", 2)
	if got, want := len(parts), 2; got != want {
		t.Fatalf("stdout parts = %d, want %d (%q)", got, want, result.Stdout)
	}
	if got, want := canonicalPath(t, parts[0]), canonicalPath(t, workingDir); got != want {
		t.Fatalf("working directory = %q, want %q", got, want)
	}
	if got, want := parts[1], "enabled"; got != want {
		t.Fatalf("env output = %q, want %q", got, want)
	}
}

func TestRunGoSnippetWithOptionsUsesToolchainSelection(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	snippet := "package main\nimport \"fmt\"\nfunc main(){fmt.Print(\"toolchain\")}\n"

	result, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
		Toolchain: "go",
	})
	if err != nil {
		t.Fatalf("RunGoSnippetWithOptions(go) error = %v", err)
	}
	if got, want := result.Stdout, "toolchain"; got != want {
		t.Fatalf("result.Stdout = %q, want %q", got, want)
	}

	if _, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
		Toolchain: "go-toolchain-missing",
	}); err == nil {
		t.Fatal("RunGoSnippetWithOptions(invalid toolchain) error = nil, want non-nil")
	}
}

func TestRunGoSnippetWithOptionsStreamsStdoutChunks(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"time\"",
		")",
		"",
		"func main() {",
		"\tfmt.Print(\"line-1\\n\")",
		"\ttime.Sleep(150 * time.Millisecond)",
		"\tfmt.Print(\"line-2\\n\")",
		"}",
		"",
	}, "\n")

	var mu sync.Mutex
	streamed := make([]string, 0)
	firstChunkAt := time.Time{}
	result, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
		Timeout: 10 * time.Second,
		OnStdoutChunk: func(chunk string) {
			mu.Lock()
			defer mu.Unlock()
			if firstChunkAt.IsZero() {
				firstChunkAt = time.Now()
			}
			streamed = append(streamed, chunk)
		},
	})
	if err != nil {
		t.Fatalf("RunGoSnippetWithOptions() error = %v", err)
	}
	if got, want := result.ExitCode, 0; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}

	mu.Lock()
	streamedOutput := strings.Join(streamed, "")
	firstSeenAt := firstChunkAt
	mu.Unlock()

	if streamedOutput == "" {
		t.Fatal("streamed output is empty, want incremental stdout chunks")
	}
	if got, want := streamedOutput, result.Stdout; got != want {
		t.Fatalf("streamed output = %q, want %q", got, want)
	}
	if !strings.Contains(streamedOutput, "line-1\nline-2\n") {
		t.Fatalf("streamed output = %q, want multiline content", streamedOutput)
	}
	if firstSeenAt.IsZero() {
		t.Fatal("first chunk timestamp not recorded")
	}
	finishedAt := time.Now()
	if delay := finishedAt.Sub(firstSeenAt); delay < 100*time.Millisecond {
		t.Fatalf("first chunk arrived too late (delay %v), want incremental streaming", delay)
	}
}

func TestRunGoSnippetWithOptionsStreamsStderrChunks(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"os\"",
		"\t\"time\"",
		")",
		"",
		"func main() {",
		"\tfmt.Print(\"out-1\\n\")",
		"\tfmt.Fprint(os.Stderr, \"err-1\\n\")",
		"\ttime.Sleep(150 * time.Millisecond)",
		"\tfmt.Fprint(os.Stderr, \"err-2\\n\")",
		"\tfmt.Print(\"out-2\\n\")",
		"}",
		"",
	}, "\n")

	var mu sync.Mutex
	stdoutChunks := make([]string, 0)
	stderrChunks := make([]string, 0)

	result, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
		Timeout: 10 * time.Second,
		OnStdoutChunk: func(chunk string) {
			mu.Lock()
			defer mu.Unlock()
			stdoutChunks = append(stdoutChunks, chunk)
		},
		OnStderrChunk: func(chunk string) {
			mu.Lock()
			defer mu.Unlock()
			stderrChunks = append(stderrChunks, chunk)
		},
	})
	if err != nil {
		t.Fatalf("RunGoSnippetWithOptions() error = %v", err)
	}
	if got, want := result.ExitCode, 0; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}

	mu.Lock()
	streamedStdout := strings.Join(stdoutChunks, "")
	streamedStderr := strings.Join(stderrChunks, "")
	mu.Unlock()

	if got, want := streamedStdout, result.Stdout; got != want {
		t.Fatalf("streamed stdout = %q, want %q", got, want)
	}
	if got, want := streamedStderr, result.Stderr; got != want {
		t.Fatalf("streamed stderr = %q, want %q", got, want)
	}
	if !strings.Contains(streamedStdout, "out-1\nout-2\n") {
		t.Fatalf("streamed stdout = %q, want stdout content", streamedStdout)
	}
	if !strings.Contains(streamedStderr, "err-1\nerr-2\n") {
		t.Fatalf("streamed stderr = %q, want stderr content", streamedStderr)
	}
}

func TestRunGoSnippetWithOptionsCanceled(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"time\"",
		")",
		"",
		"func main() {",
		"\tfmt.Print(\"begin\\n\")",
		"\ttime.Sleep(3 * time.Second)",
		"}",
		"",
	}, "\n")

	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(200*time.Millisecond, cancel)
	t.Cleanup(func() {
		timer.Stop()
		cancel()
	})

	result, err := RunGoSnippetWithOptions(ctx, projectDir, snippet, RunOptions{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunGoSnippetWithOptions() error = %v", err)
	}
	if !result.Canceled {
		t.Fatalf("result.Canceled = %v, want true", result.Canceled)
	}
	if got, want := result.ExitCode, -1; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}
}

func TestRunGoSnippetWithOptionsOutputCap(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"os\"",
		"\t\"strings\"",
		")",
		"",
		"func main() {",
		"\tfmt.Print(strings.Repeat(\"o\", 2048))",
		"\tfmt.Fprint(os.Stderr, strings.Repeat(\"e\", 2048))",
		"}",
		"",
	}, "\n")

	result, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, RunOptions{
		MaxStdoutBytes: 128,
		MaxStderrBytes: 96,
		Timeout:        10 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunGoSnippetWithOptions() error = %v", err)
	}
	if got, want := len(result.Stdout), 128; got != want {
		t.Fatalf("len(stdout) = %d, want %d", got, want)
	}
	if got, want := len(result.Stderr), 96; got != want {
		t.Fatalf("len(stderr) = %d, want %d", got, want)
	}
	if !result.StdoutTruncated {
		t.Fatal("StdoutTruncated = false, want true")
	}
	if !result.StderrTruncated {
		t.Fatal("StderrTruncated = false, want true")
	}
}

func TestRunGoSnippetWithOptionsHardKillFallback(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"os\"",
		"\t\"os/signal\"",
		"\t\"time\"",
		")",
		"",
		"func main() {",
		"\tsignal.Ignore(os.Interrupt)",
		"\tfmt.Print(\"ready\\n\")",
		"\tfor {",
		"\t\ttime.Sleep(time.Second)",
		"\t}",
		"}",
		"",
	}, "\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 1)

	type runOutcome struct {
		result Result
		err    error
	}
	outcomeCh := make(chan runOutcome, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := RunGoSnippetWithOptions(ctx, projectDir, snippet, RunOptions{
			Timeout:         10 * time.Second,
			KillGracePeriod: 200 * time.Millisecond,
			OnStdoutChunk: func(chunk string) {
				if strings.Contains(chunk, "ready\n") {
					select {
					case started <- struct{}{}:
					default:
					}
				}
			},
		})
		outcomeCh <- runOutcome{result: result, err: err}
	}()
	t.Cleanup(func() { wg.Wait() })

	select {
	case <-started:
		cancel()
	case <-time.After(4 * time.Second):
		cancel()
		t.Fatal("snippet did not start in time")
	}

	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("RunGoSnippetWithOptions() error = %v", outcome.err)
		}
		if !outcome.result.Canceled {
			t.Fatalf("result.Canceled = %v, want true", outcome.result.Canceled)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("hard-kill fallback did not stop process in time")
	}
}

func TestRunGoSnippetWithOptionsCancelStress(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	projectDir := t.TempDir()
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

	for iteration := range 3 {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		timer := time.AfterFunc(120*time.Millisecond, cancel)
		t.Cleanup(func() {
			timer.Stop()
			cancel()
		})

		result, err := RunGoSnippetWithOptions(ctx, projectDir, snippet, RunOptions{
			Timeout:         5 * time.Second,
			KillGracePeriod: 150 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("iteration %d: RunGoSnippetWithOptions() error = %v", iteration, err)
		}
		if !(result.Canceled || result.TimedOut) {
			t.Fatalf("iteration %d: expected canceled/timed out result, got %+v", iteration, result)
		}
	}
}

func TestRunGoSnippetWithOptionsUsesStableSnippetCachePath(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	toolchainDir := t.TempDir()
	logPath := filepath.Join(toolchainDir, "toolchain.log")
	toolchainPath := filepath.Join(toolchainDir, "fake-go.sh")

	script := "#!/usr/bin/env bash\nset -euo pipefail\necho \"$@\" >> \"$GOPOKE_TOOLCHAIN_LOG\"\n"
	if err := os.WriteFile(toolchainPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(fake toolchain) error = %v", err)
	}

	snippet := "package main\nfunc main() {}\n"
	options := RunOptions{
		Toolchain: toolchainPath,
		Environment: map[string]string{
			"GOPOKE_TOOLCHAIN_LOG": logPath,
		},
	}

	if _, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, options); err != nil {
		t.Fatalf("RunGoSnippetWithOptions(first run) error = %v", err)
	}
	if _, err := RunGoSnippetWithOptions(context.Background(), projectDir, snippet, options); err != nil {
		t.Fatalf("RunGoSnippetWithOptions(second run) error = %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(toolchain log) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if got, want := len(lines), 2; got != want {
		t.Fatalf("logged command count = %d, want %d (log: %q)", got, want, string(content))
	}

	parseRunPath := func(line string) string {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			t.Fatalf("unexpected toolchain invocation line %q", line)
		}
		if got, want := fields[0], "run"; got != want {
			t.Fatalf("toolchain arg[0] = %q, want %q", got, want)
		}
		return fields[1]
	}

	firstRunPath := parseRunPath(lines[0])
	secondRunPath := parseRunPath(lines[1])
	if got, want := secondRunPath, firstRunPath; got != want {
		t.Fatalf("snippet path changed between runs: second=%q first=%q", got, want)
	}

	cacheDir := filepath.Join(projectDir, ".gopoke-run-cache")
	if got, want := filepath.Dir(firstRunPath), cacheDir; canonicalPath(t, got) != canonicalPath(t, want) {
		t.Fatalf("cache dir = %q, want %q", got, want)
	}
}

func TestStableSnippetFilePath(t *testing.T) {
	t.Parallel()

	cacheDir := filepath.Join(t.TempDir(), ".gopoke-run-cache")
	snippetOne := "package main\nfunc main(){println(\"one\")}\n"
	snippetTwo := "package main\nfunc main(){println(\"two\")}\n"

	pathOneA, err := stableSnippetFilePath(cacheDir, snippetOne)
	if err != nil {
		t.Fatalf("stableSnippetFilePath(snippetOne) error = %v", err)
	}
	pathOneB, err := stableSnippetFilePath(cacheDir, snippetOne)
	if err != nil {
		t.Fatalf("stableSnippetFilePath(snippetOne, second call) error = %v", err)
	}
	pathTwo, err := stableSnippetFilePath(cacheDir, snippetTwo)
	if err != nil {
		t.Fatalf("stableSnippetFilePath(snippetTwo) error = %v", err)
	}

	if got, want := pathOneA, pathOneB; got != want {
		t.Fatalf("pathOneA = %q, want %q", got, want)
	}
	if pathOneA == pathTwo {
		t.Fatalf("pathOneA and pathTwo are equal (%q), want distinct paths", pathOneA)
	}
	if !strings.HasSuffix(pathOneA, ".go") {
		t.Fatalf("pathOneA = %q, want .go suffix", pathOneA)
	}
	if got, want := filepath.Dir(pathOneA), cacheDir; got != want {
		t.Fatalf("pathOneA dir = %q, want %q", got, want)
	}
}

func canonicalPath(t *testing.T, value string) string {
	t.Helper()

	cleaned := filepath.Clean(value)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return cleaned
	}
	return filepath.Clean(resolved)
}
