package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultTimeout limits snippet run duration for MVP safety.
const DefaultTimeout = 15 * time.Second

const (
	// DefaultMaxOutputBytes caps stdout and stderr captured for one run.
	DefaultMaxOutputBytes = 128 * 1024
	// defaultKillGracePeriod is how long graceful stop gets before forced kill.
	defaultKillGracePeriod = 400 * time.Millisecond
)

// RunRequest captures user-provided input for one snippet execution.
type RunRequest struct {
	RunID       string `json:"runId"`
	ProjectPath string `json:"projectPath"`
	PackagePath string `json:"packagePath"`
	Source      string `json:"source"`
	TimeoutMS   int64  `json:"timeoutMs"`
}

// StdoutChunkHandler receives incremental stdout chunks while a run is active.
type StdoutChunkHandler func(chunk string)

// StderrChunkHandler receives incremental stderr chunks while a run is active.
type StderrChunkHandler func(chunk string)

// RunOptions controls process execution details for one snippet run.
type RunOptions struct {
	WorkingDirectory string
	Environment      map[string]string
	Toolchain        string
	Timeout          time.Duration
	OnStdoutChunk    StdoutChunkHandler
	OnStderrChunk    StderrChunkHandler
	MaxStdoutBytes   int
	MaxStderrBytes   int
	KillGracePeriod  time.Duration
}

// Diagnostic contains one parsed compiler/runtime mapping from run output.
type Diagnostic struct {
	Kind    string
	File    string
	Line    int
	Column  int
	Message string
}

// RichBlock mirrors richoutput.RichBlock for JSON serialization to the frontend.
type RichBlock struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Result contains one snippet execution outcome.
type Result struct {
	Stdout          string       `json:"Stdout"`
	Stderr          string       `json:"Stderr"`
	ExitCode        int          `json:"ExitCode"`
	DurationMS      int64        `json:"DurationMS"`
	TimedOut        bool         `json:"TimedOut"`
	Canceled        bool         `json:"Canceled"`
	StdoutTruncated bool         `json:"StdoutTruncated"`
	StderrTruncated bool         `json:"StderrTruncated"`
	Diagnostics     []Diagnostic `json:"Diagnostics"`
	CleanStdout     string       `json:"CleanStdout,omitempty"`
	RichBlocks      []RichBlock  `json:"RichBlocks,omitempty"`
}

// RunGoSnippet executes a Go snippet with `go run` in the selected project context.
func RunGoSnippet(ctx context.Context, projectPath string, snippet string, timeout time.Duration) (Result, error) {
	return RunGoSnippetWithOptions(ctx, projectPath, snippet, RunOptions{
		Timeout: timeout,
	})
}

// RunGoSnippetWithOptions executes a Go snippet with explicit working directory and env values.
func RunGoSnippetWithOptions(ctx context.Context, projectPath string, snippet string, options RunOptions) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("run snippet context: %w", err)
	}
	if strings.TrimSpace(projectPath) == "" {
		return Result{}, fmt.Errorf("project path is required")
	}
	if strings.TrimSpace(snippet) == "" {
		return Result{}, fmt.Errorf("snippet is required")
	}

	absoluteProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return Result{}, fmt.Errorf("resolve project path: %w", err)
	}
	info, err := os.Stat(absoluteProjectPath)
	if err != nil {
		return Result{}, fmt.Errorf("inspect project path: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("project path must be a directory")
	}

	workingDirectory := strings.TrimSpace(options.WorkingDirectory)
	if workingDirectory == "" {
		workingDirectory = absoluteProjectPath
	} else {
		if !filepath.IsAbs(workingDirectory) {
			workingDirectory = filepath.Join(absoluteProjectPath, workingDirectory)
		}
		workingDirectory = filepath.Clean(workingDirectory)
	}

	workingDirectoryInfo, err := os.Stat(workingDirectory)
	if err != nil {
		return Result{}, fmt.Errorf("inspect working directory: %w", err)
	}
	if !workingDirectoryInfo.IsDir() {
		return Result{}, fmt.Errorf("working directory must be a directory")
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	cacheDir := filepath.Join(absoluteProjectPath, ".gopoke-run-cache")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create run cache dir: %w", err)
	}

	filePath, err := stableSnippetFilePath(cacheDir, snippet)
	if err != nil {
		return Result{}, fmt.Errorf("resolve snippet cache path: %w", err)
	}
	cleanSnippetCache(cacheDir, filepath.Base(filePath))
	if err := os.WriteFile(filePath, []byte(snippet), 0o600); err != nil {
		return Result{}, fmt.Errorf("write snippet file: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	toolchain := strings.TrimSpace(options.Toolchain)
	if toolchain == "" {
		toolchain = "go"
	}

	command := exec.Command(toolchain, "run", filePath)
	command.Dir = workingDirectory
	command.Env = mergeEnvironment(os.Environ(), options.Environment)
	configureCommandForLifecycle(command)

	stdoutCapture := newLimitedCaptureWriter(resolveMaxBytes(options.MaxStdoutBytes), options.OnStdoutChunk)
	stderrCapture := newLimitedCaptureWriter(resolveMaxBytes(options.MaxStderrBytes), options.OnStderrChunk)
	command.Stdout = stdoutCapture
	command.Stderr = stderrCapture

	startedAt := time.Now()
	if err := command.Start(); err != nil {
		return Result{}, fmt.Errorf("start snippet command: %w", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- command.Wait()
	}()
	err = waitForCommandExit(runCtx, command, waitCh, resolveKillGracePeriod(options.KillGracePeriod))
	duration := time.Since(startedAt)

	result := Result{
		Stdout:          stdoutCapture.String(),
		Stderr:          stderrCapture.String(),
		ExitCode:        0,
		DurationMS:      duration.Milliseconds(),
		StdoutTruncated: stdoutCapture.Truncated(),
		StderrTruncated: stderrCapture.Truncated(),
	}

	if err == nil {
		return result, nil
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.ExitCode = -1
		if strings.TrimSpace(result.Stderr) == "" {
			result.Stderr = "execution timed out"
		}
		return result, nil
	}
	if errors.Is(runCtx.Err(), context.Canceled) {
		result.Canceled = true
		result.ExitCode = -1
		if strings.TrimSpace(result.Stderr) == "" {
			result.Stderr = "execution canceled"
		}
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return Result{}, fmt.Errorf("run snippet command: %w", err)
}

func mergeEnvironment(base []string, overrides map[string]string) []string {
	merged := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}
		merged[key] = value
	}
	for key, value := range overrides {
		if strings.TrimSpace(key) == "" {
			continue
		}
		merged[key] = value
	}

	result := make([]string, 0, len(merged))
	for key, value := range merged {
		result = append(result, key+"="+value)
	}
	return result
}

func resolveMaxBytes(value int) int {
	if value > 0 {
		return value
	}
	return DefaultMaxOutputBytes
}

func resolveKillGracePeriod(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return defaultKillGracePeriod
}

func stableSnippetFilePath(cacheDir string, snippet string) (string, error) {
	if strings.TrimSpace(cacheDir) == "" {
		return "", fmt.Errorf("cache dir is required")
	}
	sum := sha256.Sum256([]byte(snippet))
	fileName := "snippet-" + hex.EncodeToString(sum[:12]) + ".go"
	return filepath.Join(cacheDir, fileName), nil
}

func waitForCommandExit(ctx context.Context, command *exec.Cmd, waitCh <-chan error, killGracePeriod time.Duration) error {
	select {
	case waitErr := <-waitCh:
		return waitErr
	case <-ctx.Done():
		_ = signalInterrupt(command)

		timer := time.NewTimer(killGracePeriod)
		defer timer.Stop()

		select {
		case waitErr := <-waitCh:
			return waitErr
		case <-timer.C:
			_ = forceKill(command)
			return <-waitCh
		}
	}
}

type limitedCaptureWriter struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	maxBytes  int
	size      int
	truncated bool
	onChunk   func(string)
}

func newLimitedCaptureWriter(maxBytes int, onChunk func(string)) *limitedCaptureWriter {
	return &limitedCaptureWriter{
		maxBytes: maxBytes,
		onChunk:  onChunk,
	}
}

func (w *limitedCaptureWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	w.mu.Lock()

	accepted := p
	remaining := w.maxBytes - w.size
	if remaining <= 0 {
		w.truncated = true
		w.mu.Unlock()
		return len(p), nil
	}
	if len(accepted) > remaining {
		accepted = accepted[:remaining]
		w.truncated = true
	}

	var chunk string
	if len(accepted) > 0 {
		if _, err := w.buffer.Write(accepted); err != nil {
			w.mu.Unlock()
			return 0, err
		}
		w.size += len(accepted)
		if w.onChunk != nil {
			chunk = string(accepted)
		}
	}
	if len(accepted) < len(p) {
		w.truncated = true
	}
	w.mu.Unlock()

	if chunk != "" {
		w.onChunk(chunk)
	}
	return len(p), nil
}

func (w *limitedCaptureWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func (w *limitedCaptureWriter) Truncated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.truncated
}

func cleanSnippetCache(cacheDir string, keepFileName string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == keepFileName {
			continue
		}
		if strings.HasPrefix(entry.Name(), "snippet-") && strings.HasSuffix(entry.Name(), ".go") {
			os.Remove(filepath.Join(cacheDir, entry.Name()))
		}
	}
}
