package lsp

import (
	"testing"
)

func TestFindGoplsBinaryNotFound(t *testing.T) {
	t.Parallel()

	path := findGoplsBinary("/nonexistent/path/only")
	if path != "" {
		t.Fatalf("findGoplsBinary() = %q, want empty for missing binary", path)
	}
}

func TestManagerNewDefaults(t *testing.T) {
	t.Parallel()

	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}

	status := m.Status()
	if status.Ready {
		t.Fatal("Status().Ready = true, want false before any project opened")
	}
}

func TestManagerStatusNotReady(t *testing.T) {
	t.Parallel()

	m := NewManager()
	status := m.Status()
	if status.Ready {
		t.Fatal("Status().Ready = true before start")
	}
}
