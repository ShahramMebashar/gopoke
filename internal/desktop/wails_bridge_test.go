package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopoke/internal/app"
	"gopoke/internal/execution"
	"gopoke/internal/lsp"
	"gopoke/internal/playground"
	"gopoke/internal/project"
	"gopoke/internal/runner"
	"gopoke/internal/settings"
	"gopoke/internal/storage"
)

type fakeApplication struct {
	startErr error

	healthResp          storage.HealthReport
	healthErr           error
	openResp            project.OpenProjectResult
	openErr             error
	recentResp          []storage.ProjectRecord
	recentErr           error
	discoverTargetsResp []project.RunTarget
	discoverTargetsErr  error
	setDefaultResp      storage.ProjectRecord
	setDefaultErr       error
	projectEnvVarsResp  []storage.EnvVarRecord
	projectEnvVarsErr   error
	upsertEnvResp       storage.EnvVarRecord
	upsertEnvErr        error
	deleteEnvErr        error
	setWorkingDirResp   storage.ProjectRecord
	setWorkingDirErr    error
	toolchainsResp      []project.ToolchainInfo
	toolchainsErr       error
	setToolchainResp    storage.ProjectRecord
	setToolchainErr     error
	projectSnippetsResp []storage.SnippetRecord
	projectSnippetsErr  error
	saveSnippetResp     storage.SnippetRecord
	saveSnippetErr      error
	deleteSnippetErr    error
	formatResp          string
	formatErr           error
	runResp             execution.Result
	runErr              error
	runStdoutChunks     []string
	runStderrChunks     []string
	canceledRunIDs      []string
	cancelRunErr        error
	startWorkerResp     runner.Worker
	startWorkerErr      error
	stopWorkerErr       error
	lspStatus           lsp.StatusResult
	lspWSPort           int
	lspWorkspaceInfo    lsp.WorkspaceInfo
	openGoFileResp      app.OpenGoFileResult
	openGoFileErr       error
	saveGoFileErr       error
	savedGoFilePath     string
	savedGoFileContent  string
}

func (f *fakeApplication) Start(ctx context.Context) error {
	return f.startErr
}

func (f *fakeApplication) Stop(ctx context.Context) error {
	return nil
}

func (f *fakeApplication) Health(ctx context.Context) (storage.HealthReport, error) {
	return f.healthResp, f.healthErr
}

func (f *fakeApplication) OpenProject(ctx context.Context, path string) (project.OpenProjectResult, error) {
	return f.openResp, f.openErr
}

func (f *fakeApplication) RecentProjects(ctx context.Context, limit int) ([]storage.ProjectRecord, error) {
	return f.recentResp, f.recentErr
}

func (f *fakeApplication) DiscoverRunTargets(ctx context.Context, path string) ([]project.RunTarget, error) {
	return f.discoverTargetsResp, f.discoverTargetsErr
}

func (f *fakeApplication) SetProjectDefaultPackage(ctx context.Context, projectPath string, packagePath string) (storage.ProjectRecord, error) {
	return f.setDefaultResp, f.setDefaultErr
}

func (f *fakeApplication) ProjectEnvVars(ctx context.Context, projectPath string) ([]storage.EnvVarRecord, error) {
	return f.projectEnvVarsResp, f.projectEnvVarsErr
}

func (f *fakeApplication) UpsertProjectEnvVar(ctx context.Context, projectPath string, key string, value string, masked bool) (storage.EnvVarRecord, error) {
	return f.upsertEnvResp, f.upsertEnvErr
}

func (f *fakeApplication) DeleteProjectEnvVar(ctx context.Context, projectPath string, key string) error {
	return f.deleteEnvErr
}

func (f *fakeApplication) SetProjectWorkingDirectory(ctx context.Context, projectPath string, workingDirectory string) (storage.ProjectRecord, error) {
	return f.setWorkingDirResp, f.setWorkingDirErr
}

func (f *fakeApplication) AvailableToolchains(ctx context.Context) ([]project.ToolchainInfo, error) {
	return f.toolchainsResp, f.toolchainsErr
}

func (f *fakeApplication) SetProjectToolchain(ctx context.Context, projectPath string, toolchain string) (storage.ProjectRecord, error) {
	return f.setToolchainResp, f.setToolchainErr
}

func (f *fakeApplication) ProjectSnippets(ctx context.Context, projectPath string) ([]storage.SnippetRecord, error) {
	return f.projectSnippetsResp, f.projectSnippetsErr
}

func (f *fakeApplication) SaveProjectSnippet(ctx context.Context, projectPath string, snippetID string, name string, content string) (storage.SnippetRecord, error) {
	return f.saveSnippetResp, f.saveSnippetErr
}

func (f *fakeApplication) DeleteProjectSnippet(ctx context.Context, projectPath string, snippetID string) error {
	return f.deleteSnippetErr
}

func (f *fakeApplication) FormatSnippet(ctx context.Context, source string) (string, error) {
	return f.formatResp, f.formatErr
}

func (f *fakeApplication) RunSnippet(
	ctx context.Context,
	request execution.RunRequest,
	onStdoutChunk execution.StdoutChunkHandler,
	onStderrChunk execution.StderrChunkHandler,
) (execution.Result, error) {
	for _, chunk := range f.runStdoutChunks {
		if onStdoutChunk != nil {
			onStdoutChunk(chunk)
		}
	}
	for _, chunk := range f.runStderrChunks {
		if onStderrChunk != nil {
			onStderrChunk(chunk)
		}
	}
	return f.runResp, f.runErr
}

func (f *fakeApplication) CancelRun(ctx context.Context, runID string) error {
	f.canceledRunIDs = append(f.canceledRunIDs, runID)
	return f.cancelRunErr
}

func (f *fakeApplication) StartProjectWorker(ctx context.Context, projectPath string) (runner.Worker, error) {
	return f.startWorkerResp, f.startWorkerErr
}

func (f *fakeApplication) StopProjectWorker(ctx context.Context, projectPath string) error {
	return f.stopWorkerErr
}

func (f *fakeApplication) StartLSP(ctx context.Context, projectPath string) error {
	return nil
}

func (f *fakeApplication) StopLSP(ctx context.Context) error {
	return nil
}

func (f *fakeApplication) LSPWebSocketPort(ctx context.Context) int {
	return f.lspWSPort
}

func (f *fakeApplication) LSPWorkspaceInfo(ctx context.Context) lsp.WorkspaceInfo {
	return f.lspWorkspaceInfo
}

func (f *fakeApplication) LSPStatus(ctx context.Context) lsp.StatusResult {
	return f.lspStatus
}

func (f *fakeApplication) PlaygroundShare(ctx context.Context, source string) (playground.ShareResult, error) {
	return playground.ShareResult{URL: "https://go.dev/play/p/test", Hash: "test"}, nil
}

func (f *fakeApplication) PlaygroundImport(ctx context.Context, urlOrHash string) (string, error) {
	return "package main\n", nil
}

func (f *fakeApplication) OpenGoFile(ctx context.Context, filePath string) (app.OpenGoFileResult, error) {
	return f.openGoFileResp, f.openGoFileErr
}

func (f *fakeApplication) SaveGoFile(ctx context.Context, filePath string, content string) error {
	f.savedGoFilePath = filePath
	f.savedGoFileContent = content
	return f.saveGoFileErr
}

func (f *fakeApplication) GetGlobalSettings(ctx context.Context) (settings.GlobalSettings, error) {
	return settings.Defaults(), nil
}

func (f *fakeApplication) UpdateGlobalSettings(ctx context.Context, gs settings.GlobalSettings) (settings.GlobalSettings, error) {
	return gs, nil
}

func (f *fakeApplication) DetectToolVersions(ctx context.Context) app.ToolVersions {
	return app.ToolVersions{}
}

func (f *fakeApplication) ScratchDir() string { return "" }

func TestWailsBridgeRequiresStartup(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{})
	if _, err := bridge.Health(); err == nil {
		t.Fatal("Health() error = nil, want non-nil")
	}
}

func TestWailsBridgeStartupError(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{startErr: fmt.Errorf("boom")})
	bridge.Startup(context.Background())

	if got := bridge.StartupError(); got == "" {
		t.Fatal("StartupError() is empty, want non-empty")
	}
	if _, err := bridge.OpenProject("/tmp/project"); err == nil {
		t.Fatal("OpenProject() error = nil, want non-nil")
	}
}

func TestWailsBridgeForwardsMethods(t *testing.T) {
	t.Parallel()

	projectRecord := storage.ProjectRecord{ID: "p1", Path: "/tmp/project"}
	targets := []project.RunTarget{
		{Package: ".", Command: "go run .", Path: "/tmp/project"},
	}

	bridge := NewWailsBridge(&fakeApplication{
		healthResp: storage.HealthReport{Ready: true, SchemaVersion: 1},
		openResp: project.OpenProjectResult{
			Project: projectRecord,
			Module:  project.ModuleInfo{Path: "/tmp/project", HasModule: true},
			Targets: targets,
		},
		recentResp:          []storage.ProjectRecord{projectRecord},
		discoverTargetsResp: targets,
	})
	bridge.Startup(context.Background())

	report, err := bridge.Health()
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if !report.Ready {
		t.Fatal("report.Ready = false, want true")
	}

	result, err := bridge.OpenProject("/tmp/project")
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	if got, want := result.Project.ID, "p1"; got != want {
		t.Fatalf("result.Project.ID = %q, want %q", got, want)
	}

	recent, err := bridge.RecentProjects(10)
	if err != nil {
		t.Fatalf("RecentProjects() error = %v", err)
	}
	if got, want := len(recent), 1; got != want {
		t.Fatalf("len(recent) = %d, want %d", got, want)
	}

	discovered, err := bridge.DiscoverRunTargets("/tmp/project")
	if err != nil {
		t.Fatalf("DiscoverRunTargets() error = %v", err)
	}
	if got, want := len(discovered), 1; got != want {
		t.Fatalf("len(discovered) = %d, want %d", got, want)
	}
}

func TestWailsBridgeChooseProjectDirectory(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{})
	bridge.Startup(context.Background())
	bridge.openDirectoryDialog = func(ctx context.Context) (string, error) {
		return "/tmp/project", nil
	}

	path, err := bridge.ChooseProjectDirectory()
	if err != nil {
		t.Fatalf("ChooseProjectDirectory() error = %v", err)
	}
	if got, want := path, "/tmp/project"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestWailsBridgeSetProjectDefaultPackage(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{
		setDefaultResp: storage.ProjectRecord{
			ID:         "prj_1",
			Path:       "/tmp/project",
			DefaultPkg: "./cmd/api",
		},
	})
	bridge.Startup(context.Background())

	record, err := bridge.SetProjectDefaultPackage("/tmp/project", "./cmd/api")
	if err != nil {
		t.Fatalf("SetProjectDefaultPackage() error = %v", err)
	}
	if got, want := record.DefaultPkg, "./cmd/api"; got != want {
		t.Fatalf("record.DefaultPkg = %q, want %q", got, want)
	}
}

func TestWailsBridgeProjectSettingsAndSnippets(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{
		projectEnvVarsResp: []storage.EnvVarRecord{
			{ProjectID: "p1", Key: "TOKEN", Value: "abc", Masked: true},
		},
		upsertEnvResp: storage.EnvVarRecord{ProjectID: "p1", Key: "NEW_KEY", Value: "v", Masked: false},
		setWorkingDirResp: storage.ProjectRecord{
			ID:         "prj_1",
			Path:       "/tmp/project",
			WorkingDir: "/tmp/project/cmd/api",
		},
		toolchainsResp: []project.ToolchainInfo{
			{Name: "go", Path: "/usr/bin/go", Version: "go version go1.25 darwin/arm64"},
		},
		setToolchainResp: storage.ProjectRecord{
			ID:        "prj_1",
			Path:      "/tmp/project",
			Toolchain: "/usr/bin/go",
		},
		projectSnippetsResp: []storage.SnippetRecord{
			{ID: "sn_1", ProjectID: "prj_1", Name: "One", Content: "package main\nfunc main() {}\n"},
		},
		saveSnippetResp: storage.SnippetRecord{
			ID:        "sn_2",
			ProjectID: "prj_1",
			Name:      "Two",
			Content:   "package main\nfunc main(){println(\"x\")}\n",
		},
	})
	bridge.Startup(context.Background())

	vars, err := bridge.ProjectEnvVars("/tmp/project")
	if err != nil {
		t.Fatalf("ProjectEnvVars() error = %v", err)
	}
	if got, want := len(vars), 1; got != want {
		t.Fatalf("len(vars) = %d, want %d", got, want)
	}

	upserted, err := bridge.UpsertProjectEnvVar("/tmp/project", "NEW_KEY", "v", false)
	if err != nil {
		t.Fatalf("UpsertProjectEnvVar() error = %v", err)
	}
	if got, want := upserted.Key, "NEW_KEY"; got != want {
		t.Fatalf("upserted.Key = %q, want %q", got, want)
	}
	if err := bridge.DeleteProjectEnvVar("/tmp/project", "NEW_KEY"); err != nil {
		t.Fatalf("DeleteProjectEnvVar() error = %v", err)
	}

	workingDirRecord, err := bridge.SetProjectWorkingDirectory("/tmp/project", "/tmp/project/cmd/api")
	if err != nil {
		t.Fatalf("SetProjectWorkingDirectory() error = %v", err)
	}
	if got, want := workingDirRecord.WorkingDir, "/tmp/project/cmd/api"; got != want {
		t.Fatalf("workingDirRecord.WorkingDir = %q, want %q", got, want)
	}

	toolchains, err := bridge.AvailableToolchains()
	if err != nil {
		t.Fatalf("AvailableToolchains() error = %v", err)
	}
	if got, want := len(toolchains), 1; got != want {
		t.Fatalf("len(toolchains) = %d, want %d", got, want)
	}

	toolchainRecord, err := bridge.SetProjectToolchain("/tmp/project", "go")
	if err != nil {
		t.Fatalf("SetProjectToolchain() error = %v", err)
	}
	if got, want := toolchainRecord.Toolchain, "/usr/bin/go"; got != want {
		t.Fatalf("toolchainRecord.Toolchain = %q, want %q", got, want)
	}

	snippets, err := bridge.ProjectSnippets("/tmp/project")
	if err != nil {
		t.Fatalf("ProjectSnippets() error = %v", err)
	}
	if got, want := len(snippets), 1; got != want {
		t.Fatalf("len(snippets) = %d, want %d", got, want)
	}

	savedSnippet, err := bridge.SaveProjectSnippet("/tmp/project", "", "Two", "package main\nfunc main(){println(\"x\")}\n")
	if err != nil {
		t.Fatalf("SaveProjectSnippet() error = %v", err)
	}
	if got, want := savedSnippet.ID, "sn_2"; got != want {
		t.Fatalf("savedSnippet.ID = %q, want %q", got, want)
	}
	if err := bridge.DeleteProjectSnippet("/tmp/project", "sn_2"); err != nil {
		t.Fatalf("DeleteProjectSnippet() error = %v", err)
	}
}

func TestWailsBridgeFormatSnippet(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{
		formatResp: "package main\n\nfunc main() {}\n",
	})
	bridge.Startup(context.Background())

	formatted, err := bridge.FormatSnippet("package main\nfunc main(){}\n")
	if err != nil {
		t.Fatalf("FormatSnippet() error = %v", err)
	}
	if got, want := formatted, "package main\n\nfunc main() {}\n"; got != want {
		t.Fatalf("formatted = %q, want %q", got, want)
	}
}

func TestWailsBridgeRunSnippet(t *testing.T) {
	t.Parallel()

	emitted := make([]RunStdoutChunkEvent, 0)
	emittedErr := make([]RunStderrChunkEvent, 0)
	bridge := NewWailsBridge(&fakeApplication{
		runResp: execution.Result{
			Stdout:     "ok\nstreamed\n",
			Stderr:     "warn-1\nwarn-2\n",
			ExitCode:   0,
			DurationMS: 12,
		},
		runStdoutChunks: []string{"ok\n", "streamed\n"},
		runStderrChunks: []string{"warn-1\n", "warn-2\n"},
	})
	bridge.emitEvent = func(ctx context.Context, eventName string, payload interface{}) {
		switch eventName {
		case runStdoutChunkEventName:
			event, ok := payload.(RunStdoutChunkEvent)
			if !ok {
				t.Fatalf("payload type = %T, want RunStdoutChunkEvent", payload)
			}
			emitted = append(emitted, event)
		case runStderrChunkEventName:
			event, ok := payload.(RunStderrChunkEvent)
			if !ok {
				t.Fatalf("payload type = %T, want RunStderrChunkEvent", payload)
			}
			emittedErr = append(emittedErr, event)
		default:
			t.Fatalf("eventName = %q, want stdout/stderr event", eventName)
		}
	}
	bridge.Startup(context.Background())

	result, err := bridge.RunSnippet(execution.RunRequest{
		RunID:       "run_test_1",
		ProjectPath: "/tmp/project",
		Source:      "package main\nfunc main(){}\n",
	})
	if err != nil {
		t.Fatalf("RunSnippet() error = %v", err)
	}
	if got, want := result.Stdout, "ok\nstreamed\n"; got != want {
		t.Fatalf("result.Stdout = %q, want %q", got, want)
	}
	if got, want := result.Stderr, "warn-1\nwarn-2\n"; got != want {
		t.Fatalf("result.Stderr = %q, want %q", got, want)
	}
	if got, want := len(emitted), 2; got != want {
		t.Fatalf("len(emitted) = %d, want %d", got, want)
	}
	if got, want := strings.Join([]string{emitted[0].Chunk, emitted[1].Chunk}, ""), "ok\nstreamed\n"; got != want {
		t.Fatalf("emitted stdout = %q, want %q", got, want)
	}
	if got, want := emitted[0].RunID, "run_test_1"; got != want {
		t.Fatalf("emitted[0].RunID = %q, want %q", got, want)
	}
	if got, want := len(emittedErr), 2; got != want {
		t.Fatalf("len(emittedErr) = %d, want %d", got, want)
	}
	if got, want := strings.Join([]string{emittedErr[0].Chunk, emittedErr[1].Chunk}, ""), "warn-1\nwarn-2\n"; got != want {
		t.Fatalf("emitted stderr = %q, want %q", got, want)
	}
	if got, want := emittedErr[0].RunID, "run_test_1"; got != want {
		t.Fatalf("emittedErr[0].RunID = %q, want %q", got, want)
	}
}

func TestWailsBridgeProjectWorkerLifecycle(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{
		startWorkerResp: runner.Worker{
			ProjectPath: "/tmp/project",
			PID:         1234,
			Running:     true,
		},
	})
	bridge.Startup(context.Background())

	worker, err := bridge.StartProjectWorker("/tmp/project")
	if err != nil {
		t.Fatalf("StartProjectWorker() error = %v", err)
	}
	if !worker.Running {
		t.Fatal("worker.Running = false, want true")
	}
	if err := bridge.StopProjectWorker("/tmp/project"); err != nil {
		t.Fatalf("StopProjectWorker() error = %v", err)
	}
}

func TestWailsBridgeLSPWebSocketPort(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{
		lspWSPort: 8080,
	})
	bridge.Startup(context.Background())

	port, err := bridge.LSPWebSocketPort()
	if err != nil {
		t.Fatalf("LSPWebSocketPort() error = %v", err)
	}
	if got, want := port, 8080; got != want {
		t.Fatalf("port = %d, want %d", got, want)
	}
}

func TestWailsBridgeLSPStatus(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{
		lspStatus: lsp.StatusResult{Ready: true},
	})
	bridge.Startup(context.Background())

	status, err := bridge.LSPStatus()
	if err != nil {
		t.Fatalf("LSPStatus() error = %v", err)
	}
	if !status.Ready {
		t.Fatal("status.Ready = false, want true")
	}
}

func TestWailsBridgeCancelRun(t *testing.T) {
	t.Parallel()

	fake := &fakeApplication{}
	bridge := NewWailsBridge(fake)
	bridge.Startup(context.Background())

	if err := bridge.CancelRun("run_cancel_1"); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}
	if got, want := len(fake.canceledRunIDs), 1; got != want {
		t.Fatalf("len(canceledRunIDs) = %d, want %d", got, want)
	}
	if got, want := fake.canceledRunIDs[0], "run_cancel_1"; got != want {
		t.Fatalf("canceled run ID = %q, want %q", got, want)
	}
}

func TestWailsBridgeChooseGoFile(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{})
	bridge.Startup(context.Background())
	bridge.openFileDialog = func(ctx context.Context) (string, error) {
		return "/tmp/main.go", nil
	}

	path, err := bridge.ChooseGoFile()
	if err != nil {
		t.Fatalf("ChooseGoFile() error = %v", err)
	}
	if got, want := path, "/tmp/main.go"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestWailsBridgeOpenGoFile(t *testing.T) {
	t.Parallel()

	bridge := NewWailsBridge(&fakeApplication{
		openGoFileResp: app.OpenGoFileResult{
			Content:  "package main\n",
			FilePath: "/tmp/project/main.go",
			ProjectResult: project.OpenProjectResult{
				Project: storage.ProjectRecord{ID: "p1", Path: "/tmp/project"},
			},
		},
	})
	bridge.Startup(context.Background())

	result, err := bridge.OpenGoFile("/tmp/project/main.go")
	if err != nil {
		t.Fatalf("OpenGoFile() error = %v", err)
	}
	if got, want := result.Content, "package main\n"; got != want {
		t.Fatalf("result.Content = %q, want %q", got, want)
	}
	if got, want := result.FilePath, "/tmp/project/main.go"; got != want {
		t.Fatalf("result.FilePath = %q, want %q", got, want)
	}
	if got, want := result.ProjectResult.Project.Path, "/tmp/project"; got != want {
		t.Fatalf("result.ProjectResult.Project.Path = %q, want %q", got, want)
	}
}

func TestWailsBridgeSaveGoFile(t *testing.T) {
	t.Parallel()

	fake := &fakeApplication{}
	bridge := NewWailsBridge(fake)
	bridge.Startup(context.Background())

	content := "package main\n\nfunc main() {}\n"
	if err := bridge.SaveGoFile("/tmp/project/main.go", content); err != nil {
		t.Fatalf("SaveGoFile() error = %v", err)
	}
	if got, want := fake.savedGoFilePath, "/tmp/project/main.go"; got != want {
		t.Fatalf("saved path = %q, want %q", got, want)
	}
	if got, want := fake.savedGoFileContent, content; got != want {
		t.Fatalf("saved content = %q, want %q", got, want)
	}
}

func TestAppOpenGoFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	// Create go.mod so OpenProject succeeds
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	a := app.NewWithDataRoot(t.TempDir())
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer a.Stop(context.Background())

	result, err := a.OpenGoFile(context.Background(), goFile)
	if err != nil {
		t.Fatalf("OpenGoFile() error = %v", err)
	}
	if got, want := result.Content, "package main\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if got, want := result.FilePath, goFile; got != want {
		t.Fatalf("filePath = %q, want %q", got, want)
	}
}

func TestAppSaveGoFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	a := app.NewWithDataRoot(t.TempDir())
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer a.Stop(context.Background())

	newContent := "package main\n\nfunc main() {}\n"
	if err := a.SaveGoFile(context.Background(), goFile, newContent); err != nil {
		t.Fatalf("SaveGoFile() error = %v", err)
	}

	read, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := string(read), newContent; got != want {
		t.Fatalf("file content = %q, want %q", got, want)
	}
}

func TestAppOpenGoFileRejectsNonGo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	txtFile := filepath.Join(dir, "main.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	a := app.NewWithDataRoot(t.TempDir())
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer a.Stop(context.Background())

	if _, err := a.OpenGoFile(context.Background(), txtFile); err == nil {
		t.Fatal("OpenGoFile() error = nil, want error for non-.go file")
	}
}

func TestAppSaveGoFileRejectsNonExistent(t *testing.T) {
	t.Parallel()

	a := app.NewWithDataRoot(t.TempDir())
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer a.Stop(context.Background())

	nonExistent := filepath.Join(t.TempDir(), "does-not-exist.go")
	if err := a.SaveGoFile(context.Background(), nonExistent, "package main\n"); err == nil {
		t.Fatal("SaveGoFile() error = nil, want error for non-existent file")
	}
}
