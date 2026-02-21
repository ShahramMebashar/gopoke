package app

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopad/internal/execution"
	"gopad/internal/project"
	"gopad/internal/storage"
	"gopad/internal/telemetry"
)

func TestApplicationRunSnippetUsesSelectedPackageAndEnv(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	openResult, err := application.OpenProject(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	if _, err := application.store.UpdateProjectEnvVar(
		context.Background(),
		openResult.Project.ID,
		"GP016_ENV",
		"configured",
		false,
	); err != nil {
		t.Fatalf("UpdateProjectEnvVar() error = %v", err)
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
		"\tfmt.Printf(\"%s|%s\", wd, os.Getenv(\"GP016_ENV\"))",
		"}",
		"",
	}, "\n")

	runResult, err := application.RunSnippet(context.Background(), execution.RunRequest{
		ProjectPath: projectDir,
		PackagePath: "./cmd/api",
		Source:      snippet,
	}, nil, nil)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := runResult.ExitCode, 0; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}

	parts := strings.SplitN(runResult.Stdout, "|", 2)
	if got, want := len(parts), 2; got != want {
		t.Fatalf("stdout parts = %d, want %d (%q)", got, want, runResult.Stdout)
	}
	if got, want := canonicalPath(t, parts[0]), canonicalPath(t, filepath.Join(projectDir, "cmd", "api")); got != want {
		t.Fatalf("working directory = %q, want %q", got, want)
	}
	if got, want := parts[1], "configured"; got != want {
		t.Fatalf("env output = %q, want %q", got, want)
	}
}

func TestApplicationOpenProjectExpandsHomePath(t *testing.T) {
	application := newTestApplication(t)

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	projectDir := filepath.Join(homeDir, "Projects", "gopad")
	setupRunnableProject(t, projectDir)

	openResult, err := application.OpenProject(context.Background(), "~/Projects/gopad")
	if err != nil {
		t.Fatalf("OpenProject(~/Projects/gopad) error = %v", err)
	}
	if got, want := canonicalPath(t, openResult.Project.Path), canonicalPath(t, projectDir); got != want {
		t.Fatalf("openResult.Project.Path = %q, want %q", got, want)
	}
}

func TestApplicationRunSnippetFallsBackToDefaultPackage(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	if _, err := application.SetProjectDefaultPackage(context.Background(), projectDir, "./cmd/api"); err != nil {
		t.Fatalf("SetProjectDefaultPackage() error = %v", err)
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
		"\tfmt.Print(wd)",
		"}",
		"",
	}, "\n")

	runResult, err := application.RunSnippet(context.Background(), execution.RunRequest{
		ProjectPath: projectDir,
		Source:      snippet,
	}, nil, nil)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := canonicalPath(t, runResult.Stdout), canonicalPath(t, filepath.Join(projectDir, "cmd", "api")); got != want {
		t.Fatalf("working directory = %q, want %q", got, want)
	}
}

func TestApplicationProjectEnvVarCRUD(t *testing.T) {
	t.Parallel()

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	openResult, err := application.OpenProject(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	if openResult.Project.ID == "" {
		t.Fatal("project ID is empty")
	}

	_, err = application.UpsertProjectEnvVar(context.Background(), projectDir, "TOKEN", "abc", true)
	if err != nil {
		t.Fatalf("UpsertProjectEnvVar() error = %v", err)
	}
	vars, err := application.ProjectEnvVars(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("ProjectEnvVars() error = %v", err)
	}
	if got, want := len(vars), 1; got != want {
		t.Fatalf("len(vars) = %d, want %d", got, want)
	}
	if !vars[0].Masked {
		t.Fatal("vars[0].Masked = false, want true")
	}

	if err := application.DeleteProjectEnvVar(context.Background(), projectDir, "TOKEN"); err != nil {
		t.Fatalf("DeleteProjectEnvVar() error = %v", err)
	}
	vars, err = application.ProjectEnvVars(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("ProjectEnvVars(after delete) error = %v", err)
	}
	if got, want := len(vars), 0; got != want {
		t.Fatalf("len(vars) = %d, want %d", got, want)
	}
}

func TestApplicationSetProjectWorkingDirectoryAffectsExecution(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	if _, err := application.SetProjectWorkingDirectory(
		context.Background(),
		projectDir,
		filepath.Join(projectDir, "cmd", "api"),
	); err != nil {
		t.Fatalf("SetProjectWorkingDirectory() error = %v", err)
	}

	runResult, err := application.RunSnippet(context.Background(), execution.RunRequest{
		ProjectPath: projectDir,
		Source: strings.Join([]string{
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
			"\tfmt.Print(wd)",
			"}",
			"",
		}, "\n"),
	}, nil, nil)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := canonicalPath(t, runResult.Stdout), canonicalPath(t, filepath.Join(projectDir, "cmd", "api")); got != want {
		t.Fatalf("working directory = %q, want %q", got, want)
	}
}

func TestApplicationSetProjectToolchainAffectsExecution(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	if _, err := application.SetProjectToolchain(context.Background(), projectDir, "go"); err != nil {
		t.Fatalf("SetProjectToolchain(go) error = %v", err)
	}

	result, err := application.RunSnippet(context.Background(), execution.RunRequest{
		ProjectPath: projectDir,
		Source:      "package main\nimport \"fmt\"\nfunc main(){fmt.Print(\"ok\")}\n",
	}, nil, nil)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := result.Stdout, "ok"; got != want {
		t.Fatalf("result.Stdout = %q, want %q", got, want)
	}

	if _, err := application.SetProjectToolchain(context.Background(), projectDir, "go-toolchain-missing"); err == nil {
		t.Fatal("SetProjectToolchain(invalid) error = nil, want non-nil")
	}
}

func TestApplicationProjectSnippetCRUD(t *testing.T) {
	t.Parallel()

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	created, err := application.SaveProjectSnippet(
		context.Background(),
		projectDir,
		"",
		"Snippet One",
		"package main\nfunc main(){}\n",
	)
	if err != nil {
		t.Fatalf("SaveProjectSnippet(create) error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("created.ID is empty")
	}

	updated, err := application.SaveProjectSnippet(
		context.Background(),
		projectDir,
		created.ID,
		"Snippet One Renamed",
		"package main\nfunc main(){println(\"ok\")}\n",
	)
	if err != nil {
		t.Fatalf("SaveProjectSnippet(update) error = %v", err)
	}
	if got, want := updated.Name, "Snippet One Renamed"; got != want {
		t.Fatalf("updated.Name = %q, want %q", got, want)
	}

	snippets, err := application.ProjectSnippets(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("ProjectSnippets() error = %v", err)
	}
	if got, want := len(snippets), 1; got != want {
		t.Fatalf("len(snippets) = %d, want %d", got, want)
	}
	if got, want := snippets[0].ID, created.ID; got != want {
		t.Fatalf("snippets[0].ID = %q, want %q", got, want)
	}

	if err := application.DeleteProjectSnippet(context.Background(), projectDir, created.ID); err != nil {
		t.Fatalf("DeleteProjectSnippet() error = %v", err)
	}
	snippets, err = application.ProjectSnippets(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("ProjectSnippets(after delete) error = %v", err)
	}
	if got, want := len(snippets), 0; got != want {
		t.Fatalf("len(snippets) = %d, want %d", got, want)
	}
}

func TestApplicationRunSnippetRejectsUnknownPackage(t *testing.T) {
	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	_, err := application.RunSnippet(context.Background(), execution.RunRequest{
		ProjectPath: projectDir,
		PackagePath: "./cmd/does-not-exist",
		Source:      "package main\nfunc main() {}\n",
	}, nil, nil)
	if err == nil {
		t.Fatal("RunSnippet() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "not a runnable target") {
		t.Fatalf("RunSnippet() error = %q, want package validation message", err)
	}
}

func TestApplicationRunSnippetStreamsStdout(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"time\"",
		")",
		"",
		"func main() {",
		"\tfmt.Print(\"stream-a\\n\")",
		"\ttime.Sleep(150 * time.Millisecond)",
		"\tfmt.Print(\"stream-b\\n\")",
		"}",
		"",
	}, "\n")

	var mu sync.Mutex
	chunks := make([]string, 0)
	firstChunkAt := time.Time{}

	result, err := application.RunSnippet(context.Background(), execution.RunRequest{
		ProjectPath: projectDir,
		Source:      snippet,
	}, func(chunk string) {
		mu.Lock()
		defer mu.Unlock()
		if firstChunkAt.IsZero() {
			firstChunkAt = time.Now()
		}
		chunks = append(chunks, chunk)
	}, nil)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := result.ExitCode, 0; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}

	mu.Lock()
	streamedOutput := strings.Join(chunks, "")
	firstSeen := firstChunkAt
	mu.Unlock()

	if streamedOutput == "" {
		t.Fatal("streamed output is empty, want stdout chunks")
	}
	if got, want := streamedOutput, result.Stdout; got != want {
		t.Fatalf("streamed output = %q, want %q", got, want)
	}
	if !strings.Contains(streamedOutput, "stream-a\nstream-b\n") {
		t.Fatalf("streamed output = %q, want multiline output", streamedOutput)
	}
	if firstSeen.IsZero() {
		t.Fatal("first chunk timestamp is zero, want recorded streaming time")
	}
	if delay := time.Since(firstSeen); delay < 100*time.Millisecond {
		t.Fatalf("first chunk delay = %v, want early chunk before run completion", delay)
	}
}

func TestApplicationRunSnippetStreamsStderr(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

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
		"\tfmt.Print(\"stdout-live\\n\")",
		"\tfmt.Fprint(os.Stderr, \"stderr-live-1\\n\")",
		"\ttime.Sleep(100 * time.Millisecond)",
		"\tfmt.Fprint(os.Stderr, \"stderr-live-2\\n\")",
		"}",
		"",
	}, "\n")

	var mu sync.Mutex
	stderrChunks := make([]string, 0)
	result, err := application.RunSnippet(
		context.Background(),
		execution.RunRequest{
			ProjectPath: projectDir,
			Source:      snippet,
		},
		nil,
		func(chunk string) {
			mu.Lock()
			defer mu.Unlock()
			stderrChunks = append(stderrChunks, chunk)
		},
	)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := result.ExitCode, 0; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}

	mu.Lock()
	streamedStderr := strings.Join(stderrChunks, "")
	mu.Unlock()
	if streamedStderr == "" {
		t.Fatal("streamed stderr is empty, want stderr chunks")
	}
	if got, want := streamedStderr, result.Stderr; got != want {
		t.Fatalf("streamed stderr = %q, want %q", got, want)
	}
	if !strings.Contains(result.Stdout, "stdout-live\n") {
		t.Fatalf("result.Stdout = %q, want stdout content", result.Stdout)
	}
}

func TestApplicationCancelRunIdleNoop(t *testing.T) {
	application := newTestApplication(t)
	if err := application.CancelRun(context.Background(), "run_missing"); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}
	if err := application.CancelRun(context.Background(), ""); err != nil {
		t.Fatalf("CancelRun(empty) error = %v", err)
	}
}

func TestApplicationCancelRunActive(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	snippet := strings.Join([]string{
		"package main",
		"",
		"import (",
		"\t\"fmt\"",
		"\t\"time\"",
		")",
		"",
		"func main() {",
		"\tfmt.Print(\"start\\n\")",
		"\ttime.Sleep(3 * time.Second)",
		"\tfmt.Print(\"end\\n\")",
		"}",
		"",
	}, "\n")

	type runOutcome struct {
		result execution.Result
		err    error
	}
	outcomeCh := make(chan runOutcome, 1)
	startedCh := make(chan struct{}, 1)
	runID := "run_cancel_active"

	go func() {
		result, err := application.RunSnippet(
			context.Background(),
			execution.RunRequest{
				RunID:       runID,
				ProjectPath: projectDir,
				Source:      snippet,
			},
			func(chunk string) {
				if strings.Contains(chunk, "start\n") {
					select {
					case startedCh <- struct{}{}:
					default:
					}
				}
			},
			nil,
		)
		outcomeCh <- runOutcome{result: result, err: err}
	}()

	select {
	case <-startedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not start in time for cancel")
	}

	if err := application.CancelRun(context.Background(), runID); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}

	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("RunSnippet() error = %v", outcome.err)
		}
		if !outcome.result.Canceled {
			t.Fatalf("result.Canceled = %v, want true", outcome.result.Canceled)
		}
		if got, want := outcome.result.ExitCode, -1; got != want {
			t.Fatalf("ExitCode = %d, want %d", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled run did not return in time")
	}
}

func TestApplicationCancelRunEarlyReturnsCanceledResult(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	runID := "run_cancel_early"
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

	type runOutcome struct {
		result execution.Result
		err    error
	}
	outcomeCh := make(chan runOutcome, 1)
	go func() {
		result, err := application.RunSnippet(
			context.Background(),
			execution.RunRequest{
				RunID:       runID,
				ProjectPath: projectDir,
				Source:      snippet,
			},
			nil,
			nil,
		)
		outcomeCh <- runOutcome{result: result, err: err}
	}()

	activeDeadline := time.Now().Add(2 * time.Second)
	active := false
	for time.Now().Before(activeDeadline) {
		application.runMu.Lock()
		_, active = application.activeRuns[runID]
		application.runMu.Unlock()
		if active {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !active {
		t.Fatal("run did not become active in time")
	}

	if err := application.CancelRun(context.Background(), runID); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}

	select {
	case outcome := <-outcomeCh:
		if outcome.err != nil {
			t.Fatalf("RunSnippet() error = %v", outcome.err)
		}
		if !outcome.result.Canceled {
			t.Fatalf("result.Canceled = %v, want true", outcome.result.Canceled)
		}
		if got, want := outcome.result.ExitCode, -1; got != want {
			t.Fatalf("ExitCode = %d, want %d", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled run did not return in time")
	}
}

func TestApplicationRunSnippetRecordsRunMetadataSuccess(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	openResult, err := application.OpenProject(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	runID := "run_record_success"
	result, err := application.RunSnippet(
		context.Background(),
		execution.RunRequest{
			RunID:       runID,
			ProjectPath: projectDir,
			Source:      "package main\n\nimport \"fmt\"\n\nfunc main(){fmt.Print(\"ok\")}\n",
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := result.ExitCode, 0; got != want {
		t.Fatalf("ExitCode = %d, want %d", got, want)
	}

	runs, err := application.store.ProjectRuns(context.Background(), openResult.Project.ID, 10)
	if err != nil {
		t.Fatalf("ProjectRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].ID, runID; got != want {
		t.Fatalf("run.ID = %q, want %q", got, want)
	}
	if got, want := runs[0].ExitCode, result.ExitCode; got != want {
		t.Fatalf("run.ExitCode = %d, want %d", got, want)
	}
	if got, want := runs[0].DurationMS, result.DurationMS; got != want {
		t.Fatalf("run.DurationMS = %d, want %d", got, want)
	}
	if got, want := runs[0].Status, runStatusSuccess; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
}

func TestApplicationRunSnippetRecordsRunMetadataFailure(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	openResult, err := application.OpenProject(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	runID := "run_record_failed"
	result, err := application.RunSnippet(
		context.Background(),
		execution.RunRequest{
			RunID:       runID,
			ProjectPath: projectDir,
			Source:      "package main\nfunc main( {}\n",
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if result.ExitCode == 0 {
		t.Fatalf("ExitCode = %d, want non-zero", result.ExitCode)
	}

	runs, err := application.store.ProjectRuns(context.Background(), openResult.Project.ID, 10)
	if err != nil {
		t.Fatalf("ProjectRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].ID, runID; got != want {
		t.Fatalf("run.ID = %q, want %q", got, want)
	}
	if got, want := runs[0].Status, runStatusFailed; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got, want := runs[0].ExitCode, result.ExitCode; got != want {
		t.Fatalf("run.ExitCode = %d, want %d", got, want)
	}
}

func TestApplicationRunSnippetParsesCompileDiagnostics(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	_, err := application.OpenProject(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	result, err := application.RunSnippet(
		context.Background(),
		execution.RunRequest{
			RunID:       "run_diag_compile",
			ProjectPath: projectDir,
			Source:      "package main\n\nfunc main(){missing()}\n",
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got := len(result.Diagnostics); got == 0 {
		t.Fatal("len(result.Diagnostics) = 0, want compile diagnostics")
	}
	if got, want := result.Diagnostics[0].Kind, "compile"; got != want {
		t.Fatalf("diagnostic kind = %q, want %q", got, want)
	}
	if result.Diagnostics[0].Line <= 0 {
		t.Fatalf("diagnostic line = %d, want > 0", result.Diagnostics[0].Line)
	}
}

func TestApplicationRunSnippetParsesPanicDiagnostics(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	_, err := application.OpenProject(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	snippet := strings.Join([]string{
		"package main",
		"",
		"func explode() {",
		"\tpanic(\"boom\")",
		"}",
		"",
		"func main() {",
		"\texplode()",
		"}",
		"",
	}, "\n")

	result, runErr := application.RunSnippet(
		context.Background(),
		execution.RunRequest{
			RunID:       "run_diag_panic",
			ProjectPath: projectDir,
			Source:      snippet,
		},
		nil,
		nil,
	)
	if runErr != nil {
		t.Fatalf("RunSnippet() error = %v", runErr)
	}
	if result.ExitCode == 0 {
		t.Fatalf("ExitCode = %d, want non-zero", result.ExitCode)
	}
	if got := len(result.Diagnostics); got == 0 {
		t.Fatal("len(result.Diagnostics) = 0, want panic diagnostics")
	}
	foundPanic := false
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Kind == "panic" {
			foundPanic = true
			if diagnostic.Line <= 0 {
				t.Fatalf("panic diagnostic line = %d, want > 0", diagnostic.Line)
			}
			break
		}
	}
	if !foundPanic {
		t.Fatalf("panic diagnostics missing in %#v", result.Diagnostics)
	}
}

func TestApplicationRunSnippetTimeoutEnforced(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	openResult, err := application.OpenProject(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	result, runErr := application.RunSnippet(
		context.Background(),
		execution.RunRequest{
			RunID:       "run_timeout_enforced",
			ProjectPath: projectDir,
			TimeoutMS:   50,
			Source: strings.Join([]string{
				"package main",
				"",
				"import \"time\"",
				"",
				"func main() {",
				"\ttime.Sleep(2 * time.Second)",
				"}",
				"",
			}, "\n"),
		},
		nil,
		nil,
	)
	if runErr != nil {
		t.Fatalf("RunSnippet() error = %v", runErr)
	}
	if !result.TimedOut {
		t.Fatalf("result.TimedOut = %v, want true", result.TimedOut)
	}
	if !strings.Contains(result.Stderr, "timed out") {
		t.Fatalf("stderr = %q, want timeout reason", result.Stderr)
	}

	runs, err := application.store.ProjectRuns(context.Background(), openResult.Project.ID, 10)
	if err != nil {
		t.Fatalf("ProjectRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].Status, runStatusTimedOut; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
}

func TestApplicationRunSnippetOutputGuardrail(t *testing.T) {
	requireGoToolchain(t)

	application := newTestApplication(t)
	projectDir := t.TempDir()
	setupRunnableProject(t, projectDir)

	if _, err := application.OpenProject(context.Background(), projectDir); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}

	result, runErr := application.RunSnippet(
		context.Background(),
		execution.RunRequest{
			RunID:       "run_output_guardrail",
			ProjectPath: projectDir,
			Source: strings.Join([]string{
				"package main",
				"",
				"import (",
				"\t\"fmt\"",
				"\t\"os\"",
				"\t\"strings\"",
				")",
				"",
				"func main() {",
				"\tfmt.Print(strings.Repeat(\"s\", 1024*200))",
				"\tfmt.Fprint(os.Stderr, strings.Repeat(\"e\", 1024*200))",
				"}",
				"",
			}, "\n"),
		},
		nil,
		nil,
	)
	if runErr != nil {
		t.Fatalf("RunSnippet() error = %v", runErr)
	}
	if !result.StdoutTruncated {
		t.Fatal("StdoutTruncated = false, want true")
	}
	if !result.StderrTruncated {
		t.Fatal("StderrTruncated = false, want true")
	}
	if got, max := len(result.Stdout), execution.DefaultMaxOutputBytes; got > max {
		t.Fatalf("len(stdout) = %d, want <= %d", got, max)
	}
	if got, max := len(result.Stderr), execution.DefaultMaxOutputBytes; got > max {
		t.Fatalf("len(stderr) = %d, want <= %d", got, max)
	}
}

func newTestApplication(t *testing.T) *Application {
	t.Helper()

	store := storage.New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	return &Application{
		logger:    slog.Default(),
		store:     store,
		projects:  project.NewService(store),
		telemetry: telemetry.NewRecorder(),
	}
}

func setupRunnableProject(t *testing.T, root string) {
	t.Helper()

	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/gopadtest\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "cmd", "api", "main.go"), "package main\n\nfunc main() {}\n")
}

func writeTestFile(t *testing.T, path string, contents string) {
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

func canonicalPath(t *testing.T, value string) string {
	t.Helper()

	cleaned := filepath.Clean(value)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return cleaned
	}
	return filepath.Clean(resolved)
}
