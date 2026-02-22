package lsp

import (
	"testing"
)

func TestFindGoplsBinaryReturnsPath(t *testing.T) {
	t.Parallel()

	// findGoplsBinary returns empty string if gopls is not installed,
	// or an absolute path if it is. Either result is valid in CI/local.
	path := findGoplsBinary()
	if path != "" {
		if path[0] != '/' {
			t.Fatalf("findGoplsBinary() = %q, want absolute path or empty", path)
		}
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
