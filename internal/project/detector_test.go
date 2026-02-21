package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectModule(t *testing.T) {
	t.Parallel()

	t.Run("module exists", func(t *testing.T) {
		t.Parallel()

		projectDir := t.TempDir()
		modulePath := filepath.Join(projectDir, "go.mod")
		if err := os.WriteFile(modulePath, []byte("module example.com/test\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(go.mod) error = %v", err)
		}

		info, err := DetectModule(context.Background(), projectDir)
		if err != nil {
			t.Fatalf("DetectModule() error = %v", err)
		}
		if !info.HasModule {
			t.Fatal("HasModule = false, want true")
		}
		if got, want := info.ModuleFile, modulePath; got != want {
			t.Fatalf("ModuleFile = %q, want %q", got, want)
		}
	})

	t.Run("module missing", func(t *testing.T) {
		t.Parallel()

		projectDir := t.TempDir()
		info, err := DetectModule(context.Background(), projectDir)
		if err != nil {
			t.Fatalf("DetectModule() error = %v", err)
		}
		if info.HasModule {
			t.Fatal("HasModule = true, want false")
		}
	})
}
