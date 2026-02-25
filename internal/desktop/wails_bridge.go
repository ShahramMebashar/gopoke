package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"gopoke/internal/app"
	"gopoke/internal/execution"
	"gopoke/internal/lsp"
	"gopoke/internal/playground"
	"gopoke/internal/project"
	"gopoke/internal/runner"
	"gopoke/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// NativeToolbarUpdater is set by main to update the native toolbar run/stop state.
var NativeToolbarUpdater func(isRunning bool)

const runStdoutChunkEventName = "gopoke:run:stdout-chunk"
const runStderrChunkEventName = "gopoke:run:stderr-chunk"

// RunStdoutChunkEvent contains streamed stdout payload for one run.
type RunStdoutChunkEvent struct {
	RunID string `json:"runId"`
	Chunk string `json:"chunk"`
}

// RunStderrChunkEvent contains streamed stderr payload for one run.
type RunStderrChunkEvent struct {
	RunID string `json:"runId"`
	Chunk string `json:"chunk"`
}

// ApplicationService captures app methods used by Wails bindings.
type ApplicationService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) (storage.HealthReport, error)
	OpenProject(ctx context.Context, path string) (project.OpenProjectResult, error)
	RecentProjects(ctx context.Context, limit int) ([]storage.ProjectRecord, error)
	DiscoverRunTargets(ctx context.Context, path string) ([]project.RunTarget, error)
	SetProjectDefaultPackage(ctx context.Context, projectPath string, packagePath string) (storage.ProjectRecord, error)
	ProjectEnvVars(ctx context.Context, projectPath string) ([]storage.EnvVarRecord, error)
	UpsertProjectEnvVar(ctx context.Context, projectPath string, key string, value string, masked bool) (storage.EnvVarRecord, error)
	DeleteProjectEnvVar(ctx context.Context, projectPath string, key string) error
	SetProjectWorkingDirectory(ctx context.Context, projectPath string, workingDirectory string) (storage.ProjectRecord, error)
	AvailableToolchains(ctx context.Context) ([]project.ToolchainInfo, error)
	SetProjectToolchain(ctx context.Context, projectPath string, toolchain string) (storage.ProjectRecord, error)
	ProjectSnippets(ctx context.Context, projectPath string) ([]storage.SnippetRecord, error)
	SaveProjectSnippet(ctx context.Context, projectPath string, snippetID string, name string, content string) (storage.SnippetRecord, error)
	DeleteProjectSnippet(ctx context.Context, projectPath string, snippetID string) error
	FormatSnippet(ctx context.Context, source string) (string, error)
	RunSnippet(
		ctx context.Context,
		request execution.RunRequest,
		onStdoutChunk execution.StdoutChunkHandler,
		onStderrChunk execution.StderrChunkHandler,
	) (execution.Result, error)
	CancelRun(ctx context.Context, runID string) error
	StartProjectWorker(ctx context.Context, projectPath string) (runner.Worker, error)
	StopProjectWorker(ctx context.Context, projectPath string) error
	StartLSP(ctx context.Context, projectPath string) error
	StopLSP(ctx context.Context) error
	LSPWebSocketPort(ctx context.Context) int
	LSPWorkspaceInfo(ctx context.Context) lsp.WorkspaceInfo
	LSPStatus(ctx context.Context) lsp.StatusResult
	OpenGoFile(ctx context.Context, filePath string) (app.OpenGoFileResult, error)
	SaveGoFile(ctx context.Context, filePath string, content string) error
	PlaygroundShare(ctx context.Context, source string) (playground.ShareResult, error)
	PlaygroundImport(ctx context.Context, urlOrHash string) (string, error)
	ScratchDir() string
}

// WailsBridge exposes backend methods to the Wails frontend.
type WailsBridge struct {
	app ApplicationService

	mu          sync.RWMutex
	ctx         context.Context
	started     bool
	startupErr  error
	shutdownErr error

	openDirectoryDialog func(ctx context.Context) (string, error)
	openFileDialog      func(ctx context.Context) (string, error)
	emitEvent           func(ctx context.Context, eventName string, payload interface{})
}

// NewWailsBridge creates a binding bridge for a running app service.
func NewWailsBridge(app ApplicationService) *WailsBridge {
	return &WailsBridge{
		app:                 app,
		ctx:                 context.Background(),
		openDirectoryDialog: defaultOpenDirectoryDialog,
		openFileDialog:      defaultOpenFileDialog,
		emitEvent:           defaultEmitEvent,
	}
}

// Startup is called by Wails at app startup.
func (b *WailsBridge) Startup(ctx context.Context) {
	b.mu.Lock()
	b.ctx = ctx
	b.started = true
	b.startupErr = b.app.Start(ctx)
	b.mu.Unlock()

	// Start LSP against scratch workspace for immediate completions.
	// Synchronous so the port is available when the frontend mounts.
	scratchDir := b.app.ScratchDir()
	if scratchDir != "" {
		if lspErr := b.app.StartLSP(context.Background(), scratchDir); lspErr != nil {
			fmt.Printf("scratch gopls start: %v\n", lspErr)
		}
	}
}

// Shutdown is called by Wails at app shutdown.
func (b *WailsBridge) Shutdown(ctx context.Context) {
	_ = b.app.StopLSP(ctx)
	if err := b.app.Stop(ctx); err != nil {
		b.mu.Lock()
		b.shutdownErr = fmt.Errorf("shutdown app: %w", err)
		b.mu.Unlock()
	}
}

// StartupError returns the startup error string if startup failed.
func (b *WailsBridge) StartupError() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.startupErr == nil {
		return ""
	}
	return b.startupErr.Error()
}

// Health returns backend health for frontend readiness checks.
func (b *WailsBridge) Health() (storage.HealthReport, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return storage.HealthReport{}, err
	}
	report, err := b.app.Health(ctx)
	if err != nil {
		return storage.HealthReport{}, fmt.Errorf("health: %w", err)
	}
	return report, nil
}

// OpenProject opens and indexes a project path, then starts gopls in background.
func (b *WailsBridge) OpenProject(path string) (project.OpenProjectResult, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return project.OpenProjectResult{}, err
	}
	result, err := b.app.OpenProject(ctx, path)
	if err != nil {
		return project.OpenProjectResult{}, fmt.Errorf("open project: %w", err)
	}

	// Start LSP synchronously so the port is available when the frontend
	// immediately calls LSPWebSocketPort() after OpenProject returns.
	// StartLSP is fast (find binary + create workspace + bind port);
	// the actual Serve loop runs in a background goroutine inside the manager.
	if lspErr := b.app.StartLSP(context.Background(), path); lspErr != nil {
		fmt.Printf("gopls start: %v\n", lspErr)
	}

	return result, nil
}

// RecentProjects returns recently opened projects for the home screen.
func (b *WailsBridge) RecentProjects(limit int) ([]storage.ProjectRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return nil, err
	}
	records, err := b.app.RecentProjects(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("recent projects: %w", err)
	}
	return records, nil
}

// DiscoverRunTargets loads runnable package targets for a project.
func (b *WailsBridge) DiscoverRunTargets(path string) ([]project.RunTarget, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return nil, err
	}
	targets, err := b.app.DiscoverRunTargets(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("discover run targets: %w", err)
	}
	return targets, nil
}

// SetProjectDefaultPackage persists the selected default package for a project.
func (b *WailsBridge) SetProjectDefaultPackage(projectPath string, packagePath string) (storage.ProjectRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	record, err := b.app.SetProjectDefaultPackage(ctx, projectPath, packagePath)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("set project default package: %w", err)
	}
	return record, nil
}

// ProjectEnvVars returns project environment variables.
func (b *WailsBridge) ProjectEnvVars(projectPath string) ([]storage.EnvVarRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return nil, err
	}
	vars, err := b.app.ProjectEnvVars(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("project env vars: %w", err)
	}
	return vars, nil
}

// UpsertProjectEnvVar creates or updates one project environment variable.
func (b *WailsBridge) UpsertProjectEnvVar(projectPath string, key string, value string, masked bool) (storage.EnvVarRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return storage.EnvVarRecord{}, err
	}
	record, err := b.app.UpsertProjectEnvVar(ctx, projectPath, key, value, masked)
	if err != nil {
		return storage.EnvVarRecord{}, fmt.Errorf("upsert project env var: %w", err)
	}
	return record, nil
}

// DeleteProjectEnvVar removes one project environment variable.
func (b *WailsBridge) DeleteProjectEnvVar(projectPath string, key string) error {
	ctx, err := b.requestContext()
	if err != nil {
		return err
	}
	if err := b.app.DeleteProjectEnvVar(ctx, projectPath, key); err != nil {
		return fmt.Errorf("delete project env var: %w", err)
	}
	return nil
}

// SetProjectWorkingDirectory persists project working directory.
func (b *WailsBridge) SetProjectWorkingDirectory(projectPath string, workingDirectory string) (storage.ProjectRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	record, err := b.app.SetProjectWorkingDirectory(ctx, projectPath, workingDirectory)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("set project working directory: %w", err)
	}
	return record, nil
}

// AvailableToolchains returns detected Go toolchains from PATH.
func (b *WailsBridge) AvailableToolchains() ([]project.ToolchainInfo, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return nil, err
	}
	toolchains, err := b.app.AvailableToolchains(ctx)
	if err != nil {
		return nil, fmt.Errorf("available toolchains: %w", err)
	}
	return toolchains, nil
}

// SetProjectToolchain persists selected Go toolchain for a project.
func (b *WailsBridge) SetProjectToolchain(projectPath string, toolchain string) (storage.ProjectRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	record, err := b.app.SetProjectToolchain(ctx, projectPath, toolchain)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("set project toolchain: %w", err)
	}
	return record, nil
}

// ProjectSnippets returns snippets for a project.
func (b *WailsBridge) ProjectSnippets(projectPath string) ([]storage.SnippetRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return nil, err
	}
	snippets, err := b.app.ProjectSnippets(ctx, projectPath)
	if err != nil {
		return nil, fmt.Errorf("project snippets: %w", err)
	}
	return snippets, nil
}

// SaveProjectSnippet creates or updates a project snippet.
func (b *WailsBridge) SaveProjectSnippet(projectPath string, snippetID string, name string, content string) (storage.SnippetRecord, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return storage.SnippetRecord{}, err
	}
	snippet, err := b.app.SaveProjectSnippet(ctx, projectPath, snippetID, name, content)
	if err != nil {
		return storage.SnippetRecord{}, fmt.Errorf("save project snippet: %w", err)
	}
	return snippet, nil
}

// DeleteProjectSnippet removes one snippet from a project.
func (b *WailsBridge) DeleteProjectSnippet(projectPath string, snippetID string) error {
	ctx, err := b.requestContext()
	if err != nil {
		return err
	}
	if err := b.app.DeleteProjectSnippet(ctx, projectPath, snippetID); err != nil {
		return fmt.Errorf("delete project snippet: %w", err)
	}
	return nil
}

// FormatSnippet runs gofmt formatting over snippet source.
func (b *WailsBridge) FormatSnippet(source string) (string, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return "", err
	}
	formatted, err := b.app.FormatSnippet(ctx, source)
	if err != nil {
		return "", fmt.Errorf("format snippet: %w", err)
	}
	return formatted, nil
}

// RunSnippet executes snippet source against a project context.
func (b *WailsBridge) RunSnippet(request execution.RunRequest) (execution.Result, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return execution.Result{}, err
	}

	runID := strings.TrimSpace(request.RunID)
	if runID == "" {
		runID = generateBridgeRunID()
	}
	request.RunID = runID

	if NativeToolbarUpdater != nil {
		NativeToolbarUpdater(true)
		defer NativeToolbarUpdater(false)
	}

	result, err := b.app.RunSnippet(
		ctx,
		request,
		func(chunk string) {
			if chunk == "" {
				return
			}
			b.emitEvent(ctx, runStdoutChunkEventName, RunStdoutChunkEvent{
				RunID: runID,
				Chunk: chunk,
			})
		},
		func(chunk string) {
			if chunk == "" {
				return
			}
			b.emitEvent(ctx, runStderrChunkEventName, RunStderrChunkEvent{
				RunID: runID,
				Chunk: chunk,
			})
		},
	)
	if err != nil {
		return execution.Result{}, fmt.Errorf("run snippet: %w", err)
	}
	return result, nil
}

// CancelRun requests cancellation for an active run.
func (b *WailsBridge) CancelRun(runID string) error {
	ctx, err := b.requestContext()
	if err != nil {
		return err
	}
	if err := b.app.CancelRun(ctx, runID); err != nil {
		return fmt.Errorf("cancel run: %w", err)
	}
	return nil
}

// StartProjectWorker ensures a long-lived worker process exists for a project.
func (b *WailsBridge) StartProjectWorker(projectPath string) (runner.Worker, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return runner.Worker{}, err
	}
	worker, err := b.app.StartProjectWorker(ctx, projectPath)
	if err != nil {
		return runner.Worker{}, fmt.Errorf("start project worker: %w", err)
	}
	return worker, nil
}

// StopProjectWorker stops a worker process for a project.
func (b *WailsBridge) StopProjectWorker(projectPath string) error {
	ctx, err := b.requestContext()
	if err != nil {
		return err
	}
	if err := b.app.StopProjectWorker(ctx, projectPath); err != nil {
		return fmt.Errorf("stop project worker: %w", err)
	}
	return nil
}

// ChooseProjectDirectory opens a native directory picker and returns the selected path.
func (b *WailsBridge) ChooseProjectDirectory() (string, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return "", err
	}
	path, err := b.openDirectoryDialog(ctx)
	if err != nil {
		return "", fmt.Errorf("choose project directory: %w", err)
	}
	return path, nil
}

// LSPWebSocketPort returns the WebSocket proxy port for monaco-languageclient.
func (b *WailsBridge) LSPWebSocketPort() (int, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return 0, err
	}
	return b.app.LSPWebSocketPort(ctx), nil
}

// LSPWorkspaceInfo returns workspace details for the frontend LSP client.
func (b *WailsBridge) LSPWorkspaceInfo() (lsp.WorkspaceInfo, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return lsp.WorkspaceInfo{}, err
	}
	return b.app.LSPWorkspaceInfo(ctx), nil
}

// LSPStatus returns LSP readiness status.
func (b *WailsBridge) LSPStatus() (lsp.StatusResult, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return lsp.StatusResult{}, err
	}
	return b.app.LSPStatus(ctx), nil
}

// ChooseGoFile opens a native file picker filtered to .go files.
func (b *WailsBridge) ChooseGoFile() (string, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return "", err
	}
	path, err := b.openFileDialog(ctx)
	if err != nil {
		return "", fmt.Errorf("choose go file: %w", err)
	}
	return path, nil
}

// OpenGoFile reads a .go file and opens its parent directory as project context.
func (b *WailsBridge) OpenGoFile(filePath string) (app.OpenGoFileResult, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return app.OpenGoFileResult{}, err
	}
	result, err := b.app.OpenGoFile(ctx, filePath)
	if err != nil {
		return app.OpenGoFileResult{}, fmt.Errorf("open go file: %w", err)
	}

	// Start LSP synchronously for the parent directory.
	if lspErr := b.app.StartLSP(context.Background(), result.ProjectResult.Project.Path); lspErr != nil {
		fmt.Printf("gopls start (file): %v\n", lspErr)
	}

	return result, nil
}

// SaveGoFile writes content back to a .go file on disk.
func (b *WailsBridge) SaveGoFile(filePath string, content string) error {
	ctx, err := b.requestContext()
	if err != nil {
		return err
	}
	if err := b.app.SaveGoFile(ctx, filePath, content); err != nil {
		return fmt.Errorf("save go file: %w", err)
	}
	return nil
}

// PlaygroundShare uploads source to the Go Playground and returns the URL.
func (b *WailsBridge) PlaygroundShare(source string) (playground.ShareResult, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return playground.ShareResult{}, err
	}
	return b.app.PlaygroundShare(ctx, source)
}

// PlaygroundImport fetches source from a Go Playground URL.
func (b *WailsBridge) PlaygroundImport(urlOrHash string) (string, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return "", err
	}
	return b.app.PlaygroundImport(ctx, urlOrHash)
}

func (b *WailsBridge) requestContext() (context.Context, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if !b.started {
		return nil, fmt.Errorf("wails bridge is not started")
	}
	if b.startupErr != nil {
		return nil, fmt.Errorf("wails bridge startup failed: %w", b.startupErr)
	}
	return b.ctx, nil
}

func defaultOpenDirectoryDialog(ctx context.Context) (string, error) {
	if preferredPath, handled, err := preferredOpenDirectoryDialog(ctx); handled || err != nil {
		return strings.TrimSpace(preferredPath), err
	}

	startedAt := time.Now()
	path, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title:            "Open Go Project",
		DefaultDirectory: "~",
	})
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path != "" {
		return path, nil
	}

	fallbackPath, usedFallback, err := fallbackOpenDirectoryDialog(time.Since(startedAt))
	if err != nil {
		return "", err
	}
	if usedFallback {
		return strings.TrimSpace(fallbackPath), nil
	}
	return "", nil
}

func generateBridgeRunID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "run_" + hex.EncodeToString(b)
	}
	return "run_" + hex.EncodeToString(b)
}

func defaultOpenFileDialog(ctx context.Context) (string, error) {
	path, err := openGoFileDialog(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}

func defaultEmitEvent(ctx context.Context, eventName string, payload interface{}) {
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, eventName, payload)
}
