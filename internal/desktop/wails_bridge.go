package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"gopad/internal/execution"
	"gopad/internal/lsp"
	"gopad/internal/project"
	"gopad/internal/runner"
	"gopad/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// NativeToolbarUpdater is set by main to update the native toolbar run/stop state.
var NativeToolbarUpdater func(isRunning bool)

const runStdoutChunkEventName = "gopad:run:stdout-chunk"
const runStderrChunkEventName = "gopad:run:stderr-chunk"

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
	SyncSnippetToLSP(ctx context.Context, content string) error
	OpenSnippetInLSP(ctx context.Context, content string) error
	LSPCompletion(ctx context.Context, line, column int) ([]lsp.CompletionItem, error)
	LSPHover(ctx context.Context, line, column int) (lsp.HoverResult, error)
	LSPDefinition(ctx context.Context, line, column int) ([]lsp.Location, error)
	LSPSignatureHelp(ctx context.Context, line, column int) (lsp.SignatureResult, error)
	LSPStatus(ctx context.Context) lsp.StatusResult
	SetLSPDiagnosticHandler(handler lsp.DiagnosticHandler)
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
	emitEvent           func(ctx context.Context, eventName string, payload interface{})
}

// NewWailsBridge creates a binding bridge for a running app service.
func NewWailsBridge(app ApplicationService) *WailsBridge {
	return &WailsBridge{
		app:                 app,
		ctx:                 context.Background(),
		openDirectoryDialog: defaultOpenDirectoryDialog,
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

	b.app.SetLSPDiagnosticHandler(func(event lsp.DiagnosticEvent) {
		b.emitEvent(ctx, lspDiagnosticsEventName, event)
	})

	// Start LSP against scratch workspace for immediate completions
	go func() {
		scratchDir := b.app.ScratchDir()
		if scratchDir != "" {
			if lspErr := b.app.StartLSP(ctx, scratchDir); lspErr != nil {
				fmt.Printf("scratch gopls start: %v\n", lspErr)
			}
		}
	}()
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

	// Start LSP in background — non-blocking, errors logged not returned
	go func() {
		if lspErr := b.app.StartLSP(ctx, path); lspErr != nil {
			// LSP is optional; log but don't fail project open
			fmt.Printf("gopls start: %v\n", lspErr)
		}
	}()

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

const lspDiagnosticsEventName = "gopad:lsp:diagnostics"

// Completion returns LSP completions at the given position.
func (b *WailsBridge) Completion(line int, column int) ([]lsp.CompletionItem, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return nil, err
	}
	items, err := b.app.LSPCompletion(ctx, line, column)
	if err != nil {
		return nil, fmt.Errorf("lsp completion: %w", err)
	}
	return items, nil
}

// Hover returns LSP hover info at the given position.
func (b *WailsBridge) Hover(line int, column int) (lsp.HoverResult, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return lsp.HoverResult{}, err
	}
	result, err := b.app.LSPHover(ctx, line, column)
	if err != nil {
		return lsp.HoverResult{}, fmt.Errorf("lsp hover: %w", err)
	}
	return result, nil
}

// Definition returns LSP definition location at the given position.
func (b *WailsBridge) Definition(line int, column int) ([]lsp.Location, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return nil, err
	}
	locations, err := b.app.LSPDefinition(ctx, line, column)
	if err != nil {
		return nil, fmt.Errorf("lsp definition: %w", err)
	}
	return locations, nil
}

// SignatureHelp returns LSP signature help at the given position.
func (b *WailsBridge) SignatureHelp(line int, column int) (lsp.SignatureResult, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return lsp.SignatureResult{}, err
	}
	result, err := b.app.LSPSignatureHelp(ctx, line, column)
	if err != nil {
		return lsp.SignatureResult{}, fmt.Errorf("lsp signature help: %w", err)
	}
	return result, nil
}

// SyncSnippet sends updated snippet content to gopls.
func (b *WailsBridge) SyncSnippet(content string) error {
	ctx, err := b.requestContext()
	if err != nil {
		return err
	}
	return b.app.SyncSnippetToLSP(ctx, content)
}

// LSPStatus returns LSP readiness status.
func (b *WailsBridge) LSPStatus() (lsp.StatusResult, error) {
	ctx, err := b.requestContext()
	if err != nil {
		return lsp.StatusResult{}, err
	}
	return b.app.LSPStatus(ctx), nil
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

func defaultEmitEvent(ctx context.Context, eventName string, payload interface{}) {
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, eventName, payload)
}
