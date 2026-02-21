package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	defaultRequestTimeout = 5 * time.Second
)

// DiagnosticHandler receives diagnostics from gopls for the frontend.
type DiagnosticHandler func(event DiagnosticEvent)

// Manager owns the gopls process lifecycle per project.
type Manager struct {
	mu          sync.RWMutex
	client      *client
	workspace   *workspace
	process     *os.Process
	projectPath string
	ready       bool
	lastError   string
	logger      *slog.Logger
	onDiag      DiagnosticHandler
	done        chan struct{}
}

// NewManager creates an LSP manager.
func NewManager() *Manager {
	return &Manager{
		logger: slog.Default(),
	}
}

// SetDiagnosticHandler registers the callback for gopls diagnostics.
func (m *Manager) SetDiagnosticHandler(handler DiagnosticHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDiag = handler
}

// StartForProject starts gopls for a project. Stops any existing session first.
func (m *Manager) StartForProject(ctx context.Context, projectPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing session
	if m.process != nil {
		m.stopLocked()
	}

	goplsPath := findGoplsBinary("")
	if goplsPath == "" {
		m.lastError = "gopls not found in PATH"
		return fmt.Errorf("gopls not found in PATH; install with: go install golang.org/x/tools/gopls@latest")
	}

	ws, err := createWorkspace(projectPath)
	if err != nil {
		m.lastError = err.Error()
		return fmt.Errorf("create workspace: %w", err)
	}

	cmd := exec.CommandContext(ctx, goplsPath, "serve")
	cmd.Dir = ws.dir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		ws.cleanup()
		return fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ws.cleanup()
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		ws.cleanup()
		m.lastError = err.Error()
		return fmt.Errorf("start gopls: %w", err)
	}

	c := newClient(stdin, stdout)
	c.onNotify = m.handleNotification

	m.client = c
	m.workspace = ws
	m.process = cmd.Process
	m.projectPath = projectPath
	m.done = make(chan struct{})

	go func() {
		c.readLoop()
		close(m.done)
		m.mu.Lock()
		m.ready = false
		m.mu.Unlock()
	}()

	if err := m.initializeLocked(ctx); err != nil {
		m.stopLocked()
		return fmt.Errorf("lsp initialize: %w", err)
	}

	m.ready = true
	m.lastError = ""
	return nil
}

func (m *Manager) initializeLocked(ctx context.Context) error {
	initCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()

	initParams := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   "file://" + m.workspace.dir,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"completion": map[string]any{
					"completionItem": map[string]any{
						"snippetSupport": false,
					},
				},
				"hover":                map[string]any{},
				"signatureHelp":        map[string]any{},
				"definition":           map[string]any{},
				"publishDiagnostics":   map[string]any{},
			},
		},
	}

	_, err := m.client.call(initCtx, "initialize", initParams)
	if err != nil {
		return err
	}

	return m.client.notify("initialized", struct{}{})
}

// OpenSnippet sends didOpen for the snippet file.
func (m *Manager) OpenSnippet(content string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ready || m.client == nil || m.workspace == nil {
		return fmt.Errorf("lsp not ready")
	}

	version := m.workspace.syncSnippet(content)
	return m.client.notify("textDocument/didOpen", didOpenTextDocumentParams{
		TextDocument: textDocumentItem{
			URI:        m.workspace.snippetURI(),
			LanguageID: "go",
			Version:    version,
			Text:       content,
		},
	})
}

// SyncSnippet sends didChange for updated snippet content.
func (m *Manager) SyncSnippet(content string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ready || m.client == nil || m.workspace == nil {
		return nil // silently skip if not ready
	}

	version := m.workspace.syncSnippet(content)
	return m.client.notify("textDocument/didChange", didChangeTextDocumentParams{
		TextDocument: versionedTextDocumentIdentifier{
			URI:     m.workspace.snippetURI(),
			Version: version,
		},
		ContentChanges: []textDocumentContentChange{
			{Text: content},
		},
	})
}

// Completion returns completions at the given 1-based line/column.
func (m *Manager) Completion(ctx context.Context, line, column int) ([]CompletionItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ready || m.client == nil {
		return nil, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()

	result, err := m.client.call(reqCtx, "textDocument/completion", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: m.workspace.snippetURI()},
		Position:     Position{Line: line - 1, Character: column - 1},
	})
	if err != nil {
		return nil, err
	}

	var list lspCompletionList
	if err := json.Unmarshal(result, &list); err != nil {
		// Try as array directly
		var items []lspCompletionItem
		if err2 := json.Unmarshal(result, &items); err2 != nil {
			return nil, fmt.Errorf("unmarshal completion: %w", err)
		}
		list.Items = items
	}

	completions := make([]CompletionItem, 0, len(list.Items))
	for _, item := range list.Items {
		insertText := item.InsertText
		if insertText == "" {
			insertText = item.Label
		}
		completions = append(completions, CompletionItem{
			Label:      item.Label,
			Detail:     item.Detail,
			Kind:       completionItemKindString(item.Kind),
			InsertText: insertText,
			SortText:   item.SortText,
		})
	}
	return completions, nil
}

// Hover returns hover info at the given 1-based line/column.
func (m *Manager) Hover(ctx context.Context, line, column int) (HoverResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ready || m.client == nil {
		return HoverResult{}, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()

	result, err := m.client.call(reqCtx, "textDocument/hover", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: m.workspace.snippetURI()},
		Position:     Position{Line: line - 1, Character: column - 1},
	})
	if err != nil {
		return HoverResult{}, err
	}
	if result == nil || string(result) == "null" {
		return HoverResult{}, nil
	}

	var hover lspHoverResult
	if err := json.Unmarshal(result, &hover); err != nil {
		return HoverResult{}, fmt.Errorf("unmarshal hover: %w", err)
	}

	return HoverResult{
		Contents: hover.Contents.Value,
		Range:    hover.Range,
	}, nil
}

// Definition returns the definition location at the given 1-based line/column.
func (m *Manager) Definition(ctx context.Context, line, column int) ([]Location, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ready || m.client == nil {
		return nil, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()

	result, err := m.client.call(reqCtx, "textDocument/definition", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: m.workspace.snippetURI()},
		Position:     Position{Line: line - 1, Character: column - 1},
	})
	if err != nil {
		return nil, err
	}
	if result == nil || string(result) == "null" {
		return nil, nil
	}

	// gopls returns []Location or Location
	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		var single Location
		if err2 := json.Unmarshal(result, &single); err2 != nil {
			return nil, fmt.Errorf("unmarshal definition: %w", err)
		}
		locations = []Location{single}
	}
	return locations, nil
}

// SignatureHelp returns signature info at the given 1-based line/column.
func (m *Manager) SignatureHelp(ctx context.Context, line, column int) (SignatureResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.ready || m.client == nil {
		return SignatureResult{}, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()

	result, err := m.client.call(reqCtx, "textDocument/signatureHelp", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: m.workspace.snippetURI()},
		Position:     Position{Line: line - 1, Character: column - 1},
	})
	if err != nil {
		return SignatureResult{}, err
	}
	if result == nil || string(result) == "null" {
		return SignatureResult{}, nil
	}

	var help lspSignatureHelp
	if err := json.Unmarshal(result, &help); err != nil {
		return SignatureResult{}, fmt.Errorf("unmarshal signature help: %w", err)
	}

	if len(help.Signatures) == 0 {
		return SignatureResult{}, nil
	}

	active := help.ActiveSignature
	if active >= len(help.Signatures) {
		active = 0
	}
	sig := help.Signatures[active]
	params := make([]string, 0, len(sig.Parameters))
	for _, p := range sig.Parameters {
		params = append(params, p.Label)
	}

	return SignatureResult{
		Label:           sig.Label,
		Parameters:      params,
		ActiveParameter: help.ActiveParameter,
	}, nil
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

// Stop shuts down gopls and cleans up.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.client != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		m.client.call(shutdownCtx, "shutdown", nil) //nolint:errcheck
		m.client.notify("exit", nil)                //nolint:errcheck
		cancel()
	}

	if m.process != nil {
		m.process.Kill() //nolint:errcheck
		m.process = nil
	}

	if m.workspace != nil {
		m.workspace.cleanup()
		m.workspace = nil
	}

	if m.done != nil {
		select {
		case <-m.done:
		case <-time.After(time.Second):
		}
	}

	m.client = nil
	m.ready = false
	m.projectPath = ""
}

func (m *Manager) handleNotification(method string, params json.RawMessage) {
	if method != "textDocument/publishDiagnostics" {
		return
	}

	var diagParams lspPublishDiagnosticsParams
	if err := json.Unmarshal(params, &diagParams); err != nil {
		m.logger.Warn("unmarshal diagnostics notification", "error", err)
		return
	}

	// Only process diagnostics for our snippet file
	m.mu.RLock()
	snippetURI := ""
	if m.workspace != nil {
		snippetURI = m.workspace.snippetURI()
	}
	handler := m.onDiag
	m.mu.RUnlock()

	if snippetURI == "" || diagParams.URI != snippetURI {
		return
	}

	if handler == nil {
		return
	}

	diagnostics := make([]LspDiagnostic, 0, len(diagParams.Diagnostics))
	for _, d := range diagParams.Diagnostics {
		diagnostics = append(diagnostics, LspDiagnostic{
			Line:      d.Range.Start.Line + 1,
			Column:    d.Range.Start.Character + 1,
			EndLine:   d.Range.End.Line + 1,
			EndColumn: d.Range.End.Character + 1,
			Severity:  diagnosticSeverityString(d.Severity),
			Message:   d.Message,
		})
	}

	handler(DiagnosticEvent{Diagnostics: diagnostics})
}

func findGoplsBinary(restrictPath string) string {
	if restrictPath != "" {
		old := os.Getenv("PATH")
		os.Setenv("PATH", restrictPath)
		defer os.Setenv("PATH", old)
	}
	path, err := exec.LookPath("gopls")
	if err != nil {
		return ""
	}
	return path
}
