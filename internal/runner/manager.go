package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const defaultStopTimeout = 2 * time.Second

// Worker holds public lifecycle information for a project worker process.
type Worker struct {
	ProjectPath string
	StartedAt   time.Time
	PID         int
	Running     bool
}

type managedWorker struct {
	info    Worker
	command *exec.Cmd
	done    chan struct{}
	waitErr error
}

// CommandFactory creates a long-lived worker command for a project.
type CommandFactory func(projectPath string) (*exec.Cmd, error)

// Option customizes the lifecycle manager.
type Option func(*Manager)

// WithCommandFactory overrides the worker command factory.
func WithCommandFactory(factory CommandFactory) Option {
	return func(m *Manager) {
		if factory != nil {
			m.commandFactory = factory
		}
	}
}

// WithStopTimeout sets timeout before hard-killing a worker.
func WithStopTimeout(timeout time.Duration) Option {
	return func(m *Manager) {
		if timeout > 0 {
			m.stopTimeout = timeout
		}
	}
}

// Manager owns worker lifecycle per project.
type Manager struct {
	mu             sync.RWMutex
	workers        map[string]*managedWorker
	commandFactory CommandFactory
	stopTimeout    time.Duration
}

// NewManager creates a process-based lifecycle manager.
func NewManager(options ...Option) *Manager {
	manager := &Manager{
		workers:        make(map[string]*managedWorker),
		commandFactory: defaultWorkerCommandFactory,
		stopTimeout:    defaultStopTimeout,
	}
	for _, option := range options {
		option(manager)
	}
	return manager
}

// StartWorker starts or reuses a long-lived worker process for a project.
func (m *Manager) StartWorker(ctx context.Context, projectPath string) (Worker, error) {
	if err := ctx.Err(); err != nil {
		return Worker{}, fmt.Errorf("start worker context: %w", err)
	}
	normalizedProjectPath, err := normalizeProjectPath(projectPath)
	if err != nil {
		return Worker{}, err
	}

	m.mu.Lock()
	existing, ok := m.workers[normalizedProjectPath]
	if ok && existing.info.Running {
		m.mu.Unlock()
		return existing.info, nil
	}

	command, err := m.commandFactory(normalizedProjectPath)
	if err != nil {
		m.mu.Unlock()
		return Worker{}, fmt.Errorf("create worker command: %w", err)
	}
	command.Stdout = io.Discard
	command.Stderr = io.Discard

	if err := command.Start(); err != nil {
		m.mu.Unlock()
		return Worker{}, fmt.Errorf("start worker command: %w", err)
	}

	worker := &managedWorker{
		info: Worker{
			ProjectPath: normalizedProjectPath,
			StartedAt:   time.Now().UTC(),
			PID:         command.Process.Pid,
			Running:     true,
		},
		command: command,
		done:    make(chan struct{}),
	}

	m.workers[normalizedProjectPath] = worker
	m.mu.Unlock()

	go m.waitForWorkerExit(normalizedProjectPath, worker)
	return worker.info, nil
}

// StopWorker stops a running worker for a project.
func (m *Manager) StopWorker(ctx context.Context, projectPath string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("stop worker context: %w", err)
	}
	normalizedProjectPath, err := normalizeProjectPath(projectPath)
	if err != nil {
		return err
	}

	m.mu.RLock()
	worker, ok := m.workers[normalizedProjectPath]
	running := ok && worker.info.Running
	m.mu.RUnlock()
	if !running {
		return nil
	}

	if worker.command.Process == nil {
		return nil
	}

	if err := worker.command.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("signal worker process: %w", err)
	}

	if err := m.waitForStop(ctx, worker); err != nil {
		return err
	}
	return nil
}

// StopAll cleanly stops all known workers.
func (m *Manager) StopAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("stop all workers context: %w", err)
	}

	m.mu.RLock()
	projectPaths := make([]string, 0, len(m.workers))
	for path := range m.workers {
		projectPaths = append(projectPaths, path)
	}
	m.mu.RUnlock()

	for _, projectPath := range projectPaths {
		if err := m.StopWorker(ctx, projectPath); err != nil {
			return fmt.Errorf("stop worker %s: %w", projectPath, err)
		}
	}
	return nil
}

// IsRunning reports whether a project's worker is currently running.
func (m *Manager) IsRunning(projectPath string) bool {
	normalizedProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	worker, ok := m.workers[normalizedProjectPath]
	return ok && worker.info.Running
}

func (m *Manager) waitForWorkerExit(projectPath string, worker *managedWorker) {
	waitErr := worker.command.Wait()

	m.mu.Lock()
	if current, ok := m.workers[projectPath]; ok && current == worker {
		current.info.Running = false
		current.waitErr = waitErr
		delete(m.workers, projectPath)
	}
	m.mu.Unlock()
	close(worker.done)
}

func (m *Manager) waitForStop(ctx context.Context, worker *managedWorker) error {
	timer := time.NewTimer(m.stopTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("stop worker context: %w", ctx.Err())
	case <-worker.done:
		return nil
	case <-timer.C:
		if worker.command.Process != nil {
			if err := worker.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return fmt.Errorf("kill worker process: %w", err)
			}
		}
		select {
		case <-worker.done:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("stop worker context: %w", ctx.Err())
		}
	}
}

func normalizeProjectPath(projectPath string) (string, error) {
	if projectPath == "" {
		return "", fmt.Errorf("project path is required")
	}
	absoluteProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	info, err := os.Stat(absoluteProjectPath)
	if err != nil {
		return "", fmt.Errorf("inspect project path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path must be a directory")
	}
	return absoluteProjectPath, nil
}

func defaultWorkerCommandFactory(projectPath string) (*exec.Cmd, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	command := exec.Command(executablePath)
	command.Env = append(
		os.Environ(),
		"GOPOKE_WORKER_MODE=1",
		"GOPOKE_WORKER_PROJECT="+projectPath,
	)
	return command, nil
}
