package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceCreate(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	goModContent := "module example.com/myapp\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.sum"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := createWorkspace(projectDir)
	if err != nil {
		t.Fatalf("createWorkspace() error = %v", err)
	}
	defer ws.cleanup()

	if _, err := os.Stat(ws.dir); os.IsNotExist(err) {
		t.Fatal("workspace dir does not exist")
	}

	goMod, err := os.ReadFile(filepath.Join(ws.dir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod error = %v", err)
	}
	if !strings.Contains(string(goMod), "module gopad-snippet") {
		t.Fatalf("go.mod missing module declaration, got: %s", goMod)
	}
	if !strings.Contains(string(goMod), "go 1.22") {
		t.Fatalf("go.mod missing go version, got: %s", goMod)
	}
}

func TestWorkspaceSnippetURI(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/myapp\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := createWorkspace(projectDir)
	if err != nil {
		t.Fatalf("createWorkspace() error = %v", err)
	}
	defer ws.cleanup()

	uri := ws.snippetURI()
	if !strings.HasPrefix(uri, "file://") {
		t.Fatalf("uri = %q, want file:// prefix", uri)
	}
	if !strings.HasSuffix(uri, "/main.go") {
		t.Fatalf("uri = %q, want /main.go suffix", uri)
	}
}

func TestWorkspaceCleanup(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/myapp\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := createWorkspace(projectDir)
	if err != nil {
		t.Fatalf("createWorkspace() error = %v", err)
	}

	wsDir := ws.dir
	ws.cleanup()

	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Fatal("workspace dir should be removed after cleanup")
	}
}

func TestWorkspaceCreateNoGoMod(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	ws, err := createWorkspace(projectDir)
	if err != nil {
		t.Fatalf("createWorkspace() error = %v", err)
	}
	defer ws.cleanup()

	goMod, err := os.ReadFile(filepath.Join(ws.dir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod error = %v", err)
	}
	if !strings.Contains(string(goMod), "module gopad-snippet") {
		t.Fatalf("go.mod missing module, got: %s", goMod)
	}
}
