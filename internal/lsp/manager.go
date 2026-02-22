package lsp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Manager owns the LSP proxy lifecycle per project.
type Manager struct {
	mu          sync.RWMutex
	proxy       *Proxy
	workspace   *workspace
	projectPath string
	ready       bool
	lastError   string
	logger      *slog.Logger
}

// NewManager creates an LSP manager.
func NewManager() *Manager {
	return &Manager{
		logger: slog.Default(),
	}
}

// StartForProject starts the LSP proxy for a project. Stops any existing session first.
func (m *Manager) StartForProject(ctx context.Context, projectPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.proxy != nil {
		m.stopLocked()
	}

	goplsPath := findGoplsBinary()
	if goplsPath == "" {
		m.lastError = "gopls not found in PATH"
		return fmt.Errorf("gopls not found in PATH; install with: go install golang.org/x/tools/gopls@latest")
	}

	ws, err := createWorkspace(projectPath)
	if err != nil {
		m.lastError = err.Error()
		return fmt.Errorf("create workspace: %w", err)
	}

	proxy, err := NewProxy(goplsPath, ws.dir, m.logger)
	if err != nil {
		ws.cleanup()
		m.lastError = err.Error()
		return fmt.Errorf("create proxy: %w", err)
	}

	m.proxy = proxy
	m.workspace = ws
	m.projectPath = projectPath
	m.ready = true
	m.lastError = ""

	go func() {
		if err := proxy.Serve(); err != nil {
			m.logger.Warn("lsp proxy serve error", "error", err)
		}
		m.mu.Lock()
		if m.proxy == proxy {
			m.ready = false
		}
		m.mu.Unlock()
	}()

	return nil
}

// Port returns the WebSocket proxy port, or 0 if not running.
func (m *Manager) Port() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.proxy == nil {
		return 0
	}
	return m.proxy.Port()
}

// WorkspaceInfo returns workspace details for the frontend.
func (m *Manager) WorkspaceInfo() WorkspaceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.workspace == nil {
		return WorkspaceInfo{}
	}
	return WorkspaceInfo{
		Dir:        m.workspace.dir,
		SnippetURI: m.workspace.snippetURI(),
	}
}

// Status returns the current LSP status.
func (m *Manager) Status() StatusResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return StatusResult{
		Ready: m.ready,
		Error: m.lastError,
	}
}

// Stop shuts down the proxy and cleans up.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.proxy != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		m.proxy.Shutdown(shutdownCtx)
		m.proxy = nil
	}

	if m.workspace != nil {
		m.workspace.cleanup()
		m.workspace = nil
	}

	m.ready = false
	m.projectPath = ""
}
