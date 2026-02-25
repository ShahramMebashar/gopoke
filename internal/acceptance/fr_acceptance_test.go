package acceptance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopoke/internal/app"
	"gopoke/internal/execution"
	"gopoke/internal/storage"
	"gopoke/internal/testutil"
)

const acceptanceRunTimeoutMS = 30000

func TestFRAcceptanceSuite(t *testing.T) {
	requireGoToolchain(t)

	suiteRoot := t.TempDir()
	sharedHome := filepath.Join(suiteRoot, "shared-home")
	application, stop := startApplication(t, sharedHome)
	defer stop()

	moduleProject := createProject(t, true)
	nonModuleProject := createProject(t, false)

	t.Run("FR-1", func(t *testing.T) {
		moduleResult, err := application.OpenProject(context.Background(), moduleProject)
		if err != nil {
			t.Fatalf("OpenProject(module) error = %v", err)
		}
		if !moduleResult.Module.HasModule {
			t.Fatal("moduleResult.Module.HasModule = false, want true")
		}

		nonModuleResult, err := application.OpenProject(context.Background(), nonModuleProject)
		if err != nil {
			t.Fatalf("OpenProject(non-module) error = %v", err)
		}
		if nonModuleResult.Module.HasModule {
			t.Fatal("nonModuleResult.Module.HasModule = true, want false")
		}
	})

	t.Run("FR-2", func(t *testing.T) {
		openResult, err := application.OpenProject(context.Background(), moduleProject)
		if err != nil {
			t.Fatalf("OpenProject() error = %v", err)
		}
		if _, err := application.SetProjectDefaultPackage(context.Background(), moduleProject, "./cmd/api"); err != nil {
			t.Fatalf("SetProjectDefaultPackage() error = %v", err)
		}
		if _, err := application.SetProjectWorkingDirectory(context.Background(), moduleProject, filepath.Join(moduleProject, "cmd", "api")); err != nil {
			t.Fatalf("SetProjectWorkingDirectory() error = %v", err)
		}
		if _, err := application.UpsertProjectEnvVar(context.Background(), moduleProject, "FR2_ENV", "configured", false); err != nil {
			t.Fatalf("UpsertProjectEnvVar() error = %v", err)
		}

		runCtx, runCancel := testutil.TestRunContext(t)
		defer runCancel()
		result, err := application.RunSnippet(
			runCtx,
			execution.RunRequest{
				ProjectPath: moduleProject,
				PackagePath: "./cmd/api",
				TimeoutMS:   acceptanceRunTimeoutMS,
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
					"\tfmt.Printf(\"%s|%s\", wd, os.Getenv(\"FR2_ENV\"))",
					"}",
					"",
				}, "\n"),
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

		parts := strings.SplitN(result.Stdout, "|", 2)
		if got, want := len(parts), 2; got != want {
			t.Fatalf("stdout split count = %d, want %d (%q)", got, want, result.Stdout)
		}
		if got, want := canonicalPath(parts[0]), canonicalPath(filepath.Join(moduleProject, "cmd", "api")); got != want {
			t.Fatalf("working directory = %q, want %q", got, want)
		}
		if got, want := parts[1], "configured"; got != want {
			t.Fatalf("env value = %q, want %q", got, want)
		}
		if openResult.Project.ID == "" {
			t.Fatal("project ID empty after open")
		}
	})

	t.Run("FR-3", func(t *testing.T) {
		if _, err := application.OpenProject(context.Background(), moduleProject); err != nil {
			t.Fatalf("OpenProject() error = %v", err)
		}

		runID := "fr3-cancel-run"
		started := make(chan struct{}, 1)
		type outcome struct {
			result execution.Result
			err    error
		}
		outcomeCh := make(chan outcome, 1)

		runCtx, runCancel := testutil.TestRunContext(t)
		defer runCancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := application.RunSnippet(
				runCtx,
				execution.RunRequest{
					RunID:       runID,
					ProjectPath: moduleProject,
					TimeoutMS:   acceptanceRunTimeoutMS,
					Source: strings.Join([]string{
						"package main",
						"",
						"import (",
						"\t\"fmt\"",
						"\t\"time\"",
						")",
						"",
						"func main() {",
						"\tfmt.Print(\"started\\n\")",
						"\ttime.Sleep(3 * time.Second)",
						"\tfmt.Print(\"done\\n\")",
						"}",
						"",
					}, "\n"),
				},
				func(chunk string) {
					if strings.Contains(chunk, "started") {
						select {
						case started <- struct{}{}:
						default:
						}
					}
				},
				nil,
			)
			outcomeCh <- outcome{result: result, err: err}
		}()
		t.Cleanup(func() { wg.Wait() })

		select {
		case <-started:
		case <-time.After(15 * time.Second):
			t.Fatal("run did not start in time")
		}

		if err := application.CancelRun(context.Background(), runID); err != nil {
			t.Fatalf("CancelRun() error = %v", err)
		}

		select {
		case finished := <-outcomeCh:
			if finished.err != nil {
				t.Fatalf("RunSnippet(cancel) error = %v", finished.err)
			}
			if !finished.result.Canceled {
				t.Fatalf("finished.result.Canceled = %v, want true", finished.result.Canceled)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("canceled run did not complete in time")
		}

		rerunCtx, rerunCancel := testutil.TestRunContext(t)
		defer rerunCancel()
		rerunResult, err := application.RunSnippet(
			rerunCtx,
			execution.RunRequest{
				ProjectPath: moduleProject,
				Source:      "package main\nimport \"fmt\"\nfunc main(){fmt.Print(\"rerun-ok\")}\n",
				TimeoutMS:   acceptanceRunTimeoutMS,
			},
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("RunSnippet(rerun) error = %v", err)
		}
		if got, want := rerunResult.ExitCode, 0; got != want {
			t.Fatalf("rerun ExitCode = %d, want %d", got, want)
		}
		if got, want := rerunResult.Stdout, "rerun-ok"; got != want {
			t.Fatalf("rerun Stdout = %q, want %q", got, want)
		}
	})

	t.Run("FR-4", func(t *testing.T) {
		if _, err := application.OpenProject(context.Background(), moduleProject); err != nil {
			t.Fatalf("OpenProject() error = %v", err)
		}

		runCtx, runCancel := testutil.TestRunContext(t)
		defer runCancel()
		result, err := application.RunSnippet(
			runCtx,
			execution.RunRequest{
				ProjectPath: moduleProject,
				TimeoutMS:   acceptanceRunTimeoutMS,
				Source: strings.Join([]string{
					"package main",
					"",
					"import (",
					"\t\"fmt\"",
					"\t\"os\"",
					")",
					"",
					"func main() {",
					"\tfmt.Print(\"stdout-channel\")",
					"\tfmt.Fprint(os.Stderr, \"stderr-channel\")",
					"}",
					"",
				}, "\n"),
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
		if !strings.Contains(result.Stdout, "stdout-channel") {
			t.Fatalf("Stdout = %q, want stdout marker", result.Stdout)
		}
		if !strings.Contains(result.Stderr, "stderr-channel") {
			t.Fatalf("Stderr = %q, want stderr marker", result.Stderr)
		}
		if strings.Contains(result.Stdout, "stderr-channel") {
			t.Fatalf("Stdout unexpectedly contains stderr marker: %q", result.Stdout)
		}
	})

	t.Run("FR-5", func(t *testing.T) {
		if _, err := application.OpenProject(context.Background(), moduleProject); err != nil {
			t.Fatalf("OpenProject() error = %v", err)
		}

		runCtx, runCancel := testutil.TestRunContext(t)
		defer runCancel()
		compileResult, err := application.RunSnippet(
			runCtx,
			execution.RunRequest{
				ProjectPath: moduleProject,
				Source:      "package main\nfunc main(){missingSymbol()}\n",
				TimeoutMS:   acceptanceRunTimeoutMS,
			},
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("RunSnippet(compile error) error = %v", err)
		}
		if compileResult.ExitCode == 0 {
			t.Fatalf("compileResult.ExitCode = %d, want non-zero", compileResult.ExitCode)
		}
		if !hasDiagnostic(compileResult.Diagnostics, "compile") {
			t.Fatalf("compile diagnostics missing: %#v", compileResult.Diagnostics)
		}

		panicCtx, panicCancel := testutil.TestRunContext(t)
		defer panicCancel()
		panicResult, err := application.RunSnippet(
			panicCtx,
			execution.RunRequest{
				ProjectPath: moduleProject,
				TimeoutMS:   acceptanceRunTimeoutMS,
				Source: strings.Join([]string{
					"package main",
					"",
					"func boom() {",
					"\tpanic(\"explode\")",
					"}",
					"",
					"func main() {",
					"\tboom()",
					"}",
					"",
				}, "\n"),
			},
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("RunSnippet(panic) error = %v", err)
		}
		if panicResult.ExitCode == 0 {
			t.Fatalf("panicResult.ExitCode = %d, want non-zero", panicResult.ExitCode)
		}
		if !hasDiagnostic(panicResult.Diagnostics, "panic") {
			t.Fatalf("panic diagnostics missing: %#v", panicResult.Diagnostics)
		}
	})

	t.Run("FR-6", func(t *testing.T) {
		if _, err := application.OpenProject(context.Background(), moduleProject); err != nil {
			t.Fatalf("OpenProject() error = %v", err)
		}

		created, err := application.SaveProjectSnippet(
			context.Background(),
			moduleProject,
			"",
			"FR6 Snippet",
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
			moduleProject,
			created.ID,
			"FR6 Snippet Renamed",
			"package main\nfunc main(){println(\"ok\")}\n",
		)
		if err != nil {
			t.Fatalf("SaveProjectSnippet(update) error = %v", err)
		}
		if got, want := updated.Name, "FR6 Snippet Renamed"; got != want {
			t.Fatalf("updated.Name = %q, want %q", got, want)
		}

		snippets, err := application.ProjectSnippets(context.Background(), moduleProject)
		if err != nil {
			t.Fatalf("ProjectSnippets() error = %v", err)
		}
		if got, want := len(snippets), 1; got != want {
			t.Fatalf("len(snippets) = %d, want %d", got, want)
		}
		if got, want := snippets[0].ID, created.ID; got != want {
			t.Fatalf("snippets[0].ID = %q, want %q", got, want)
		}

		if err := application.DeleteProjectSnippet(context.Background(), moduleProject, created.ID); err != nil {
			t.Fatalf("DeleteProjectSnippet() error = %v", err)
		}
		snippets, err = application.ProjectSnippets(context.Background(), moduleProject)
		if err != nil {
			t.Fatalf("ProjectSnippets(after delete) error = %v", err)
		}
		if got, want := len(snippets), 0; got != want {
			t.Fatalf("len(snippets) = %d, want %d", got, want)
		}
	})

	t.Run("FR-7", func(t *testing.T) {
		persistenceHome := filepath.Join(suiteRoot, "persistence-home")
		projectDir := createProject(t, true)

		applicationOne, stopOne := startApplication(t, persistenceHome)
		openResult, err := applicationOne.OpenProject(context.Background(), projectDir)
		if err != nil {
			stopOne()
			t.Fatalf("OpenProject(app1) error = %v", err)
		}

		if _, err := applicationOne.UpsertProjectEnvVar(context.Background(), projectDir, "FR7_KEY", "persisted", true); err != nil {
			stopOne()
			t.Fatalf("UpsertProjectEnvVar(app1) error = %v", err)
		}
		savedSnippet, err := applicationOne.SaveProjectSnippet(
			context.Background(),
			projectDir,
			"",
			"FR7 Persisted Snippet",
			"package main\nfunc main(){}\n",
		)
		if err != nil {
			stopOne()
			t.Fatalf("SaveProjectSnippet(app1) error = %v", err)
		}
		recentOne, err := applicationOne.RecentProjects(context.Background(), 10)
		if err != nil {
			stopOne()
			t.Fatalf("RecentProjects(app1) error = %v", err)
		}
		if len(recentOne) == 0 {
			stopOne()
			t.Fatal("RecentProjects(app1) returned no projects")
		}
		stopOne()

		applicationTwo, stopTwo := startApplication(t, persistenceHome)
		defer stopTwo()

		recentTwo, err := applicationTwo.RecentProjects(context.Background(), 10)
		if err != nil {
			t.Fatalf("RecentProjects(app2) error = %v", err)
		}
		if len(recentTwo) == 0 {
			t.Fatal("RecentProjects(app2) returned no projects")
		}
		if got, want := canonicalPath(recentTwo[0].Path), canonicalPath(projectDir); got != want {
			t.Fatalf("recentTwo[0].Path = %q, want %q", got, want)
		}

		envVars, err := applicationTwo.ProjectEnvVars(context.Background(), projectDir)
		if err != nil {
			t.Fatalf("ProjectEnvVars(app2) error = %v", err)
		}
		if !containsEnvVar(envVars, "FR7_KEY", "persisted", true) {
			t.Fatalf("persisted env var missing: %#v", envVars)
		}

		snippets, err := applicationTwo.ProjectSnippets(context.Background(), projectDir)
		if err != nil {
			t.Fatalf("ProjectSnippets(app2) error = %v", err)
		}
		if !containsSnippet(snippets, savedSnippet.ID, "FR7 Persisted Snippet") {
			t.Fatalf("persisted snippet missing: %#v", snippets)
		}
		if openResult.Project.ID == "" {
			t.Fatal("openResult.Project.ID is empty")
		}
	})
}

func startApplication(t *testing.T, dataRoot string) (*app.Application, func()) {
	t.Helper()

	application := app.NewWithDataRoot(dataRoot)
	if err := application.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	stopped := false
	stopFn := func() {
		if stopped {
			return
		}
		stopped = true
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.DefaultShutdownTimeout)
		defer cancel()
		if err := application.Stop(shutdownCtx); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	}

	return application, stopFn
}

func createProject(t *testing.T, withModule bool) string {
	t.Helper()

	root := t.TempDir()
	if withModule {
		writeFile(t, filepath.Join(root, "go.mod"), "module example.com/gopoke/acceptance\n\ngo 1.20\n")
	}
	writeFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(root, "cmd", "api", "main.go"), "package main\n\nfunc main() {}\n")
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

func hasDiagnostic(items []execution.Diagnostic, kind string) bool {
	for _, item := range items {
		if item.Kind == kind && item.Line > 0 {
			return true
		}
	}
	return false
}

func containsEnvVar(items []storage.EnvVarRecord, key string, value string, masked bool) bool {
	for _, item := range items {
		if item.Key == key && item.Value == value && item.Masked == masked {
			return true
		}
	}
	return false
}

func containsSnippet(items []storage.SnippetRecord, id string, name string) bool {
	for _, item := range items {
		if item.ID == id && item.Name == name {
			return true
		}
	}
	return false
}

func canonicalPath(value string) string {
	cleaned := filepath.Clean(value)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return cleaned
	}
	return filepath.Clean(resolved)
}
