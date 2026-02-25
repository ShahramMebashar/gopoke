package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopoke/internal/diagnostics"
	"gopoke/internal/execution"
	"gopoke/internal/formatting"
	"gopoke/internal/lsp"
	"gopoke/internal/playground"
	"gopoke/internal/project"
	"gopoke/internal/richoutput"
	"gopoke/internal/runner"
	"gopoke/internal/storage"
	"gopoke/internal/telemetry"
)

// DefaultShutdownTimeout controls graceful shutdown time for the app.
const DefaultShutdownTimeout = 5 * time.Second

// OpenGoFileResult holds content and project context from opening a single .go file.
type OpenGoFileResult struct {
	Content       string                    `json:"content"`
	FilePath      string                    `json:"filePath"`
	ProjectResult project.OpenProjectResult `json:"projectResult"`
}

const (
	runStatusSuccess  = "success"
	runStatusFailed   = "failed"
	runStatusCanceled = "canceled"
	runStatusTimedOut = "timed_out"
)

// Application wires core dependencies for the GoPad process.
type Application struct {
	logger         *slog.Logger
	store          *storage.Store
	projects       *project.Service
	workers        *runner.Manager
	lspManager     *lsp.Manager
	runMu          sync.Mutex
	activeRuns     map[string]context.CancelFunc
	telemetry      *telemetry.Recorder
	startupMetrics telemetry.StartupEvent
	scratchDir     string // temp dir for projectless runs and LSP
}

type resolvedRunRequest struct {
	projectID        string
	projectPath      string
	source           string
	workingDirectory string
	toolchain        string
	environment      map[string]string
	timeout          time.Duration
}

// New creates an application with default local dependencies.
func New() *Application {
	return NewWithDataRoot(defaultDataRoot())
}

// NewWithDataRoot creates an application rooted at the provided data directory.
func NewWithDataRoot(dataRoot string) *Application {
	if strings.TrimSpace(dataRoot) == "" {
		dataRoot = defaultDataRoot()
	}
	return &Application{
		logger:    slog.Default(),
		store:     storage.New(filepath.Join(dataRoot, "state")),
		telemetry: telemetry.NewRecorder(),
	}
}

// Start boots storage and records startup metrics.
func (a *Application) Start(ctx context.Context) error {
	startedAt := time.Now()
	if err := a.store.Bootstrap(ctx); err != nil {
		return fmt.Errorf("bootstrap storage: %w", err)
	}

	// Create scratch workspace for projectless mode
	scratchDir := filepath.Join(os.TempDir(), fmt.Sprintf("gopoke-scratch-%d", os.Getpid()))
	if err := os.MkdirAll(scratchDir, 0o700); err != nil {
		return fmt.Errorf("create scratch workspace: %w", err)
	}
	goModContent := "module gopoke-scratch\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(scratchDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		return fmt.Errorf("write scratch go.mod: %w", err)
	}
	a.scratchDir = scratchDir

	a.projects = project.NewService(a.store)
	a.workers = runner.NewManager()
	a.lspManager = lsp.NewManager()
	a.activeRuns = make(map[string]context.CancelFunc)
	a.startupMetrics = a.telemetry.MarkStartupComplete(startedAt)
	a.logger.Info(
		"application started",
		"storagePath", a.store.Path(),
		"startupDurationMs", a.startupMetrics.Duration.Milliseconds(),
	)
	return nil
}

// Stop shuts down workers and LSP, then releases resources.
func (a *Application) Stop(ctx context.Context) error {
	if a.scratchDir != "" {
		os.RemoveAll(a.scratchDir)
	}
	if a.lspManager != nil {
		a.lspManager.Stop()
	}
	if a.workers != nil {
		if err := a.workers.StopAll(ctx); err != nil {
			return fmt.Errorf("stop worker manager: %w", err)
		}
	}
	return nil
}

// ScratchDir returns the path to the scratch workspace for projectless mode.
func (a *Application) ScratchDir() string {
	return a.scratchDir
}

// Health checks whether local storage is initialized.
func (a *Application) Health(ctx context.Context) (storage.HealthReport, error) {
	report, err := a.store.Health(ctx)
	if err != nil {
		return storage.HealthReport{}, fmt.Errorf("storage health: %w", err)
	}
	return report, nil
}

// OpenProject validates and loads project context for a workspace.
func (a *Application) OpenProject(ctx context.Context, path string) (project.OpenProjectResult, error) {
	if a.projects == nil {
		return project.OpenProjectResult{}, fmt.Errorf("project service not initialized")
	}
	resolvedPath, err := resolveInputPath(path)
	if err != nil {
		return project.OpenProjectResult{}, err
	}
	result, err := a.projects.Open(ctx, resolvedPath)
	if err != nil {
		return project.OpenProjectResult{}, fmt.Errorf("open project: %w", err)
	}
	return result, nil
}

// RecentProjects returns recently opened projects.
func (a *Application) RecentProjects(ctx context.Context, limit int) ([]storage.ProjectRecord, error) {
	if a.projects == nil {
		return nil, fmt.Errorf("project service not initialized")
	}
	records, err := a.projects.Recent(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("recent projects: %w", err)
	}
	return records, nil
}

// DiscoverRunTargets returns runnable package targets for a project path.
func (a *Application) DiscoverRunTargets(ctx context.Context, path string) ([]project.RunTarget, error) {
	resolvedPath, err := resolveInputPath(path)
	if err != nil {
		return nil, err
	}
	targets, err := project.DiscoverRunTargets(ctx, resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("discover run targets: %w", err)
	}
	return targets, nil
}

// SetProjectDefaultPackage updates a project's default run target package.
func (a *Application) SetProjectDefaultPackage(ctx context.Context, projectPath string, packagePath string) (storage.ProjectRecord, error) {
	if a.projects == nil {
		return storage.ProjectRecord{}, fmt.Errorf("project service not initialized")
	}
	resolvedProjectPath, err := resolveInputPath(projectPath)
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	record, err := a.projects.SetDefaultPackage(ctx, resolvedProjectPath, packagePath)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("set project default package: %w", err)
	}
	return record, nil
}

// ProjectEnvVars returns persisted project environment variables.
func (a *Application) ProjectEnvVars(ctx context.Context, projectPath string) ([]storage.EnvVarRecord, error) {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	vars, err := a.store.ProjectEnvVars(ctx, projectRecord.ID)
	if err != nil {
		return nil, fmt.Errorf("load project env vars: %w", err)
	}
	return vars, nil
}

// UpsertProjectEnvVar creates or updates one environment variable for a project.
func (a *Application) UpsertProjectEnvVar(ctx context.Context, projectPath string, key string, value string, masked bool) (storage.EnvVarRecord, error) {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return storage.EnvVarRecord{}, err
	}
	record, err := a.store.UpdateProjectEnvVar(ctx, projectRecord.ID, key, value, masked)
	if err != nil {
		return storage.EnvVarRecord{}, fmt.Errorf("upsert project env var: %w", err)
	}
	return record, nil
}

// DeleteProjectEnvVar removes one environment variable from a project.
func (a *Application) DeleteProjectEnvVar(ctx context.Context, projectPath string, key string) error {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return err
	}
	if err := a.store.DeleteProjectEnvVar(ctx, projectRecord.ID, key); err != nil {
		return fmt.Errorf("delete project env var: %w", err)
	}
	return nil
}

// SetProjectWorkingDirectory persists working directory for a project.
func (a *Application) SetProjectWorkingDirectory(ctx context.Context, projectPath string, workingDirectory string) (storage.ProjectRecord, error) {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	resolvedWorkingDirectory, err := resolveProjectWorkingDirectory(projectRecord.Path, workingDirectory)
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	updated, err := a.store.UpdateProjectWorkingDirectory(ctx, projectRecord.Path, resolvedWorkingDirectory)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("set project working directory: %w", err)
	}
	return updated, nil
}

// AvailableToolchains returns detected Go toolchains from PATH.
func (a *Application) AvailableToolchains(ctx context.Context) ([]project.ToolchainInfo, error) {
	toolchains, err := project.DiscoverToolchains(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover toolchains: %w", err)
	}
	return toolchains, nil
}

// SetProjectToolchain persists selected Go toolchain for a project.
func (a *Application) SetProjectToolchain(ctx context.Context, projectPath string, toolchain string) (storage.ProjectRecord, error) {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	resolvedToolchain, err := project.ResolveToolchainBinary(toolchain)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("resolve selected toolchain: %w", err)
	}
	updated, err := a.store.UpdateProjectToolchain(ctx, projectRecord.Path, resolvedToolchain)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("set project toolchain: %w", err)
	}
	return updated, nil
}

// ProjectSnippets returns snippets for one project.
func (a *Application) ProjectSnippets(ctx context.Context, projectPath string) ([]storage.SnippetRecord, error) {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	snippets, err := a.store.ProjectSnippets(ctx, projectRecord.ID)
	if err != nil {
		return nil, fmt.Errorf("load project snippets: %w", err)
	}
	return snippets, nil
}

// SaveProjectSnippet creates or updates one snippet in project scope.
func (a *Application) SaveProjectSnippet(ctx context.Context, projectPath string, snippetID string, name string, content string) (storage.SnippetRecord, error) {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return storage.SnippetRecord{}, err
	}
	snippet, err := a.store.SaveSnippet(ctx, storage.SnippetRecord{
		ID:        strings.TrimSpace(snippetID),
		ProjectID: projectRecord.ID,
		Name:      name,
		Content:   content,
	})
	if err != nil {
		return storage.SnippetRecord{}, fmt.Errorf("save project snippet: %w", err)
	}
	return snippet, nil
}

// DeleteProjectSnippet deletes one snippet in project scope.
func (a *Application) DeleteProjectSnippet(ctx context.Context, projectPath string, snippetID string) error {
	projectRecord, err := a.projectRecordByPath(ctx, projectPath)
	if err != nil {
		return err
	}
	snippet, found, err := a.store.SnippetByID(ctx, snippetID)
	if err != nil {
		return fmt.Errorf("load project snippet: %w", err)
	}
	if !found {
		return nil
	}
	if snippet.ProjectID != projectRecord.ID {
		return fmt.Errorf("snippet does not belong to selected project")
	}
	if err := a.store.DeleteSnippet(ctx, snippetID); err != nil {
		return fmt.Errorf("delete project snippet: %w", err)
	}
	return nil
}

// FormatSnippet applies gofmt-style formatting to the provided snippet.
func (a *Application) FormatSnippet(ctx context.Context, source string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("format snippet context: %w", err)
	}
	formatted, err := formatting.GoSource(source)
	if err != nil {
		return "", fmt.Errorf("format snippet: %w", err)
	}
	return formatted, nil
}

// RunSnippet executes snippet source in selected project context.
func (a *Application) RunSnippet(
	ctx context.Context,
	request execution.RunRequest,
	onStdoutChunk execution.StdoutChunkHandler,
	onStderrChunk execution.StderrChunkHandler,
) (execution.Result, error) {
	if err := ctx.Err(); err != nil {
		return execution.Result{}, fmt.Errorf("run snippet context: %w", err)
	}
	runID := strings.TrimSpace(request.RunID)
	if runID == "" {
		runID = generateRunID()
		request.RunID = runID
	}

	runCtx, cancel := context.WithCancel(ctx)
	if err := a.registerActiveRun(runID, cancel); err != nil {
		cancel()
		return execution.Result{}, fmt.Errorf("register active run: %w", err)
	}
	defer func() {
		cancel()
		a.unregisterActiveRun(runID)
	}()
	runStartedAt := time.Now().UTC()

	resolvedRequest, err := a.resolveRunRequest(runCtx, request)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			result := canceledRunResult(runStartedAt)
			if recordErr := a.recordRunResult(ctx, runID, "", runStartedAt, result); recordErr != nil {
				a.logger.Warn("record run metadata failed", "runID", runID, "error", recordErr)
			}
			return result, nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			result := timedOutRunResult(runStartedAt)
			if recordErr := a.recordRunResult(ctx, runID, "", runStartedAt, result); recordErr != nil {
				a.logger.Warn("record run metadata failed", "runID", runID, "error", recordErr)
			}
			return result, nil
		}
		return execution.Result{}, fmt.Errorf("resolve run request: %w", err)
	}

	if a.workers != nil {
		if _, err := a.workers.StartWorker(runCtx, resolvedRequest.projectPath); err != nil {
			if errors.Is(err, context.Canceled) {
				result := canceledRunResult(runStartedAt)
				if recordErr := a.recordRunResult(ctx, runID, resolvedRequest.projectID, runStartedAt, result); recordErr != nil {
					a.logger.Warn("record run metadata failed", "runID", runID, "error", recordErr)
				}
				return result, nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				result := timedOutRunResult(runStartedAt)
				if recordErr := a.recordRunResult(ctx, runID, resolvedRequest.projectID, runStartedAt, result); recordErr != nil {
					a.logger.Warn("record run metadata failed", "runID", runID, "error", recordErr)
				}
				return result, nil
			}
			return execution.Result{}, fmt.Errorf("ensure project worker: %w", err)
		}
	}

	result, err := execution.RunGoSnippetWithOptions(
		runCtx,
		resolvedRequest.projectPath,
		resolvedRequest.source,
		execution.RunOptions{
			WorkingDirectory: resolvedRequest.workingDirectory,
			Environment:      resolvedRequest.environment,
			Toolchain:        resolvedRequest.toolchain,
			Timeout:          resolvedRequest.timeout,
			OnStdoutChunk:    onStdoutChunk,
			OnStderrChunk:    onStderrChunk,
			MaxStdoutBytes:   execution.DefaultMaxOutputBytes,
			MaxStderrBytes:   execution.DefaultMaxOutputBytes,
		},
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			result := canceledRunResult(runStartedAt)
			if recordErr := a.recordRunResult(ctx, runID, resolvedRequest.projectID, runStartedAt, result); recordErr != nil {
				a.logger.Warn("record run metadata failed", "runID", runID, "error", recordErr)
			}
			return result, nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			result := timedOutRunResult(runStartedAt)
			if recordErr := a.recordRunResult(ctx, runID, resolvedRequest.projectID, runStartedAt, result); recordErr != nil {
				a.logger.Warn("record run metadata failed", "runID", runID, "error", recordErr)
			}
			return result, nil
		}
		return execution.Result{}, fmt.Errorf("run snippet: %w", err)
	}
	result.Diagnostics = convertDiagnostics(diagnostics.ParseAll(result.Stderr))

	cleanStdout, richBlocks := richoutput.Parse(result.Stdout)
	result.CleanStdout = cleanStdout
	result.RichBlocks = convertRichBlocks(richBlocks)

	if err := a.recordRunResult(ctx, runID, resolvedRequest.projectID, runStartedAt, result); err != nil {
		a.logger.Warn("record run metadata failed", "runID", runID, "error", err)
	}
	return result, nil
}

// CancelRun requests cancellation for an active run. Missing/idle runs are a no-op.
func (a *Application) CancelRun(ctx context.Context, runID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cancel run context: %w", err)
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}

	a.runMu.Lock()
	cancel, ok := a.activeRuns[runID]
	if ok {
		delete(a.activeRuns, runID)
	}
	a.runMu.Unlock()

	if !ok {
		return nil
	}
	cancel()
	return nil
}

func (a *Application) resolveRunRequest(ctx context.Context, request execution.RunRequest) (resolvedRunRequest, error) {
	if a.store == nil {
		return resolvedRunRequest{}, fmt.Errorf("storage service not initialized")
	}
	if strings.TrimSpace(request.Source) == "" {
		return resolvedRunRequest{}, fmt.Errorf("snippet is required")
	}
	timeout := execution.DefaultTimeout
	if request.TimeoutMS > 0 {
		timeout = time.Duration(request.TimeoutMS) * time.Millisecond
	}
	// Projectless mode: use scratch workspace
	if strings.TrimSpace(request.ProjectPath) == "" {
		if a.scratchDir == "" {
			return resolvedRunRequest{}, fmt.Errorf("scratch workspace not initialized")
		}
		resolvedToolchain, err := project.ResolveToolchainBinary("go")
		if err != nil {
			return resolvedRunRequest{}, fmt.Errorf("resolve default toolchain: %w", err)
		}
		return resolvedRunRequest{
			projectPath:      a.scratchDir,
			source:           request.Source,
			workingDirectory: a.scratchDir,
			toolchain:        resolvedToolchain,
			environment:      make(map[string]string),
			timeout:          timeout,
		}, nil
	}
	absoluteProjectPath, err := resolveInputPath(request.ProjectPath)
	if err != nil {
		return resolvedRunRequest{}, err
	}
	info, err := os.Stat(absoluteProjectPath)
	if err != nil {
		return resolvedRunRequest{}, fmt.Errorf("inspect project path: %w", err)
	}
	if !info.IsDir() {
		return resolvedRunRequest{}, fmt.Errorf("project path must be a directory")
	}

	projectRecord, foundProject, err := a.store.ProjectByPath(ctx, absoluteProjectPath)
	if err != nil {
		return resolvedRunRequest{}, fmt.Errorf("load project context: %w", err)
	}

	selectedPackage := strings.TrimSpace(request.PackagePath)
	if selectedPackage == "" && foundProject {
		selectedPackage = strings.TrimSpace(projectRecord.DefaultPkg)
	}

	if !foundProject {
		defaultPkg := selectedPackage
		if defaultPkg == "" {
			defaultPkg = "."
		}
		projectRecord, err = a.store.RecordProjectOpen(ctx, absoluteProjectPath, defaultPkg)
		if err != nil {
			return resolvedRunRequest{}, fmt.Errorf("persist project context: %w", err)
		}
		foundProject = true
	}

	workingDirectory, err := resolveWorkingDirectory(ctx, absoluteProjectPath, selectedPackage, projectRecord.WorkingDir)
	if err != nil {
		return resolvedRunRequest{}, err
	}

	envMap := make(map[string]string)
	if foundProject {
		envMap, err = a.store.ProjectEnvMap(ctx, projectRecord.ID)
		if err != nil {
			return resolvedRunRequest{}, fmt.Errorf("load project env: %w", err)
		}
	}

	selectedToolchain := strings.TrimSpace(projectRecord.Toolchain)
	if selectedToolchain == "" {
		selectedToolchain = "go"
	}
	resolvedToolchain, err := project.ResolveToolchainBinary(selectedToolchain)
	if err != nil {
		return resolvedRunRequest{}, fmt.Errorf("resolve project toolchain: %w", err)
	}

	return resolvedRunRequest{
		projectID:        projectRecord.ID,
		projectPath:      absoluteProjectPath,
		source:           request.Source,
		workingDirectory: workingDirectory,
		toolchain:        resolvedToolchain,
		environment:      envMap,
		timeout:          timeout,
	}, nil
}

func resolveWorkingDirectory(ctx context.Context, projectPath string, packagePath string, savedWorkingDirectory string) (string, error) {
	if strings.TrimSpace(savedWorkingDirectory) != "" {
		return resolveProjectWorkingDirectory(projectPath, savedWorkingDirectory)
	}
	if packagePath == "" || packagePath == "." {
		return projectPath, nil
	}

	targets, err := project.DiscoverRunTargets(ctx, projectPath)
	if err != nil {
		return "", fmt.Errorf("discover run targets: %w", err)
	}
	for _, target := range targets {
		if target.Package == packagePath {
			return target.Path, nil
		}
	}
	return "", fmt.Errorf("package %q is not a runnable target", packagePath)
}

func (a *Application) registerActiveRun(runID string, cancel context.CancelFunc) error {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	if a.activeRuns == nil {
		a.activeRuns = make(map[string]context.CancelFunc)
	}
	if _, exists := a.activeRuns[runID]; exists {
		return fmt.Errorf("run %q is already active", runID)
	}
	a.activeRuns[runID] = cancel
	return nil
}

func (a *Application) unregisterActiveRun(runID string) {
	a.runMu.Lock()
	defer a.runMu.Unlock()
	if a.activeRuns == nil {
		return
	}
	delete(a.activeRuns, runID)
}

func (a *Application) recordRunResult(
	ctx context.Context,
	runID string,
	projectID string,
	startedAt time.Time,
	result execution.Result,
) error {
	if projectID == "" {
		return nil
	}

	_, err := a.store.RecordRun(ctx, storage.RunRecord{
		ID:         runID,
		ProjectID:  projectID,
		SnippetID:  "",
		StartedAt:  startedAt,
		DurationMS: result.DurationMS,
		ExitCode:   result.ExitCode,
		Status:     runStatusFromResult(result),
	})
	if err != nil {
		return fmt.Errorf("store run record: %w", err)
	}
	return nil
}

func runStatusFromResult(result execution.Result) string {
	switch {
	case result.Canceled:
		return runStatusCanceled
	case result.TimedOut:
		return runStatusTimedOut
	case result.ExitCode == 0:
		return runStatusSuccess
	default:
		return runStatusFailed
	}
}

func canceledRunResult(startedAt time.Time) execution.Result {
	return execution.Result{
		ExitCode:   -1,
		DurationMS: time.Since(startedAt).Milliseconds(),
		Canceled:   true,
		Stderr:     "execution canceled",
	}
}

func timedOutRunResult(startedAt time.Time) execution.Result {
	return execution.Result{
		ExitCode:   -1,
		DurationMS: time.Since(startedAt).Milliseconds(),
		TimedOut:   true,
		Stderr:     "execution timed out",
	}
}

func convertDiagnostics(items []diagnostics.Diagnostic) []execution.Diagnostic {
	if len(items) == 0 {
		return nil
	}
	converted := make([]execution.Diagnostic, 0, len(items))
	for _, item := range items {
		converted = append(converted, execution.Diagnostic{
			Kind:    item.Kind,
			File:    item.File,
			Line:    item.Line,
			Column:  item.Column,
			Message: item.Message,
		})
	}
	return converted
}

func convertRichBlocks(blocks []richoutput.RichBlock) []execution.RichBlock {
	if len(blocks) == 0 {
		return nil
	}
	converted := make([]execution.RichBlock, len(blocks))
	for i, b := range blocks {
		converted[i] = execution.RichBlock{
			Type: b.Type,
			Data: b.Data,
		}
	}
	return converted
}

// StartProjectWorker ensures a long-lived worker process exists for a project.
func (a *Application) StartProjectWorker(ctx context.Context, projectPath string) (runner.Worker, error) {
	if a.workers == nil {
		return runner.Worker{}, fmt.Errorf("worker manager not initialized")
	}
	resolvedProjectPath, err := resolveInputPath(projectPath)
	if err != nil {
		return runner.Worker{}, err
	}
	worker, err := a.workers.StartWorker(ctx, resolvedProjectPath)
	if err != nil {
		return runner.Worker{}, fmt.Errorf("start project worker: %w", err)
	}
	return worker, nil
}

// StopProjectWorker cleanly stops a project worker process.
func (a *Application) StopProjectWorker(ctx context.Context, projectPath string) error {
	if a.workers == nil {
		return fmt.Errorf("worker manager not initialized")
	}
	resolvedProjectPath, err := resolveInputPath(projectPath)
	if err != nil {
		return err
	}
	if err := a.workers.StopWorker(ctx, resolvedProjectPath); err != nil {
		return fmt.Errorf("stop project worker: %w", err)
	}
	return nil
}

// StartLSP starts gopls for a project path.
func (a *Application) StartLSP(ctx context.Context, projectPath string) error {
	if a.lspManager == nil {
		return fmt.Errorf("lsp manager not initialized")
	}
	resolvedPath, err := resolveInputPath(projectPath)
	if err != nil {
		return err
	}
	return a.lspManager.StartForProject(ctx, resolvedPath)
}

// StopLSP shuts down gopls.
func (a *Application) StopLSP(ctx context.Context) error {
	if a.lspManager == nil {
		return nil
	}
	a.lspManager.Stop()
	return nil
}

// LSPWebSocketPort returns the WebSocket proxy port for monaco-languageclient.
func (a *Application) LSPWebSocketPort(ctx context.Context) int {
	if a.lspManager == nil {
		return 0
	}
	return a.lspManager.Port()
}

// LSPWorkspaceInfo returns workspace details for the frontend LSP client.
func (a *Application) LSPWorkspaceInfo(ctx context.Context) lsp.WorkspaceInfo {
	if a.lspManager == nil {
		return lsp.WorkspaceInfo{}
	}
	return a.lspManager.WorkspaceInfo()
}

// LSPStatus returns current LSP readiness.
func (a *Application) LSPStatus(ctx context.Context) lsp.StatusResult {
	if a.lspManager == nil {
		return lsp.StatusResult{Ready: false, Error: "lsp not initialized"}
	}
	return a.lspManager.Status()
}

// OpenGoFile reads a single .go file and opens its parent directory as a project.
func (a *Application) OpenGoFile(ctx context.Context, filePath string) (OpenGoFileResult, error) {
	resolvedPath, err := resolveInputPath(filePath)
	if err != nil {
		return OpenGoFileResult{}, err
	}
	if !strings.HasSuffix(resolvedPath, ".go") {
		return OpenGoFileResult{}, fmt.Errorf("file must have .go extension")
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return OpenGoFileResult{}, fmt.Errorf("inspect file: %w", err)
	}
	if info.IsDir() {
		return OpenGoFileResult{}, fmt.Errorf("path is a directory, not a file")
	}
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return OpenGoFileResult{}, fmt.Errorf("read file: %w", err)
	}
	projectDir := filepath.Dir(resolvedPath)
	projectResult, err := a.OpenProject(ctx, projectDir)
	if err != nil {
		return OpenGoFileResult{}, fmt.Errorf("open parent project: %w", err)
	}
	return OpenGoFileResult{
		Content:       string(content),
		FilePath:      resolvedPath,
		ProjectResult: projectResult,
	}, nil
}

// SaveGoFile writes content back to a .go file on disk.
func (a *Application) SaveGoFile(ctx context.Context, filePath string, content string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save file context: %w", err)
	}
	resolvedPath, err := resolveInputPath(filePath)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(resolvedPath, ".go") {
		return fmt.Errorf("file must have .go extension")
	}
	if _, err := os.Stat(resolvedPath); err != nil {
		return fmt.Errorf("file must already exist to save: %w", err)
	}
	if err := os.WriteFile(resolvedPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// PlaygroundShare uploads the snippet to the Go Playground.
func (a *Application) PlaygroundShare(ctx context.Context, source string) (playground.ShareResult, error) {
	if strings.TrimSpace(source) == "" {
		return playground.ShareResult{}, fmt.Errorf("source is required")
	}
	return playground.Share(ctx, source)
}

// PlaygroundImport fetches source from a Go Playground URL.
func (a *Application) PlaygroundImport(ctx context.Context, urlOrHash string) (string, error) {
	if strings.TrimSpace(urlOrHash) == "" {
		return "", fmt.Errorf("playground URL or hash is required")
	}
	return playground.Import(ctx, urlOrHash)
}

func (a *Application) projectRecordByPath(ctx context.Context, projectPath string) (storage.ProjectRecord, error) {
	if a.store == nil {
		return storage.ProjectRecord{}, fmt.Errorf("storage service not initialized")
	}
	absolutePath, err := resolveInputPath(projectPath)
	if err != nil {
		return storage.ProjectRecord{}, err
	}
	record, found, err := a.store.ProjectByPath(ctx, absolutePath)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("load project context: %w", err)
	}
	if !found {
		return storage.ProjectRecord{}, fmt.Errorf("project not found; open project first")
	}
	return record, nil
}

func resolveProjectWorkingDirectory(projectPath string, workingDirectory string) (string, error) {
	resolved := strings.TrimSpace(workingDirectory)
	if resolved == "" {
		return "", fmt.Errorf("working directory is required")
	}
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(projectPath, resolved)
	}
	resolved = filepath.Clean(resolved)

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect working directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory must be a directory")
	}

	relativePath, err := filepath.Rel(projectPath, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve working directory relative path: %w", err)
	}
	if relativePath == ".." || strings.HasPrefix(filepath.ToSlash(relativePath), "../") {
		return "", fmt.Errorf("working directory must be inside project")
	}
	return resolved, nil
}

func defaultDataRoot() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ".gopoke"
	}
	return filepath.Join(configDir, "gopoke")
}

func generateRunID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "run_fallback"
	}
	return "run_" + hex.EncodeToString(b)
}

func resolveInputPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("project path is required")
	}

	if strings.HasPrefix(trimmed, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		switch {
		case trimmed == "~":
			trimmed = homeDir
		case strings.HasPrefix(trimmed, "~/"):
			trimmed = filepath.Join(homeDir, trimmed[2:])
		default:
			return "", fmt.Errorf("unsupported home path %q (use ~/...)", path)
		}
	}

	absolutePath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	return absolutePath, nil
}
