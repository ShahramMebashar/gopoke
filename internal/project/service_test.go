package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopoke/internal/storage"
)

func TestServiceOpenAndRecent(t *testing.T) {
	t.Parallel()

	store := storage.New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	service := NewService(store)

	projectOne := t.TempDir()
	writeProjectFiles(t, projectOne, true)
	resultOne, err := service.Open(context.Background(), projectOne)
	if err != nil {
		t.Fatalf("Open(projectOne) error = %v", err)
	}
	if !resultOne.Module.HasModule {
		t.Fatal("projectOne Module.HasModule = false, want true")
	}
	if got, want := len(resultOne.Targets), 1; got != want {
		t.Fatalf("len(projectOne targets) = %d, want %d", got, want)
	}

	// Create ordering gap to make recency deterministic.
	time.Sleep(5 * time.Millisecond)

	projectTwo := t.TempDir()
	writeProjectFiles(t, projectTwo, false)
	resultTwo, err := service.Open(context.Background(), projectTwo)
	if err != nil {
		t.Fatalf("Open(projectTwo) error = %v", err)
	}
	if resultTwo.Module.HasModule {
		t.Fatal("projectTwo Module.HasModule = true, want false")
	}

	recent, err := service.Recent(context.Background(), 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if got, want := len(recent), 2; got != want {
		t.Fatalf("len(recent) = %d, want %d", got, want)
	}
	if got, want := recent[0].Path, filepath.Clean(projectTwo); got != want {
		t.Fatalf("recent[0].Path = %q, want %q", got, want)
	}
	if got, want := recent[1].Path, filepath.Clean(projectOne); got != want {
		t.Fatalf("recent[1].Path = %q, want %q", got, want)
	}
}

func TestServiceOpenRejectsInvalidPath(t *testing.T) {
	t.Parallel()

	store := storage.New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	service := NewService(store)
	if _, err := service.Open(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Open() error = nil, want non-nil")
	}
}

func TestServiceSetDefaultPackagePersistsAcrossOpen(t *testing.T) {
	t.Parallel()

	store := storage.New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	service := NewService(store)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "go.mod"), "module example.com/test\n")
	writeFile(t, filepath.Join(projectDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(projectDir, "cmd", "api", "main.go"), "package main\n\nfunc main() {}\n")

	firstOpen, err := service.Open(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	if got, want := firstOpen.Project.DefaultPkg, "."; got != want {
		t.Fatalf("first default package = %q, want %q", got, want)
	}

	updated, err := service.SetDefaultPackage(context.Background(), projectDir, "./cmd/api")
	if err != nil {
		t.Fatalf("SetDefaultPackage() error = %v", err)
	}
	if got, want := updated.DefaultPkg, "./cmd/api"; got != want {
		t.Fatalf("updated default package = %q, want %q", got, want)
	}

	secondOpen, err := service.Open(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	if got, want := secondOpen.Project.DefaultPkg, "./cmd/api"; got != want {
		t.Fatalf("second default package = %q, want %q", got, want)
	}
}

func TestServiceSetDefaultPackageRejectsUnknownTarget(t *testing.T) {
	t.Parallel()

	store := storage.New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	service := NewService(store)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "go.mod"), "module example.com/test\n")
	writeFile(t, filepath.Join(projectDir, "main.go"), "package main\n\nfunc main() {}\n")
	_, err := service.Open(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if _, err := service.SetDefaultPackage(context.Background(), projectDir, "./missing"); err == nil {
		t.Fatal("SetDefaultPackage() error = nil, want non-nil")
	}
}

func TestServiceOpenLoadsDotEnvAndReportsInvalidLines(t *testing.T) {
	t.Parallel()

	store := storage.New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	service := NewService(store)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "go.mod"), "module example.com/test\n")
	writeFile(t, filepath.Join(projectDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(projectDir, ".env"), strings.Join([]string{
		"API_TOKEN=abc",
		"INVALID LINE",
		"DB_PORT=5432",
	}, "\n"))

	result, err := service.Open(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, want := len(result.EnvVars), 2; got != want {
		t.Fatalf("len(result.EnvVars) = %d, want %d", got, want)
	}
	if got := len(result.EnvLoadWarnings); got == 0 {
		t.Fatal("len(result.EnvLoadWarnings) = 0, want parse warning")
	}
	if got, want := result.EnvVars[0].Key, "API_TOKEN"; got != want {
		t.Fatalf("result.EnvVars[0].Key = %q, want %q", got, want)
	}
}

func TestServiceOpenPreservesMaskedValueForDotEnvOverrides(t *testing.T) {
	t.Parallel()

	store := storage.New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	service := NewService(store)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "go.mod"), "module example.com/test\n")
	writeFile(t, filepath.Join(projectDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(projectDir, ".env"), "SECRET=from-file\n")

	firstOpen, err := service.Open(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	if _, err := store.UpdateProjectEnvVar(context.Background(), firstOpen.Project.ID, "SECRET", "manual", true); err != nil {
		t.Fatalf("UpdateProjectEnvVar() error = %v", err)
	}

	secondOpen, err := service.Open(context.Background(), projectDir)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	if got, want := len(secondOpen.EnvVars), 1; got != want {
		t.Fatalf("len(secondOpen.EnvVars) = %d, want %d", got, want)
	}
	if got, want := secondOpen.EnvVars[0].Value, "from-file"; got != want {
		t.Fatalf("secondOpen.EnvVars[0].Value = %q, want %q", got, want)
	}
	if !secondOpen.EnvVars[0].Masked {
		t.Fatal("secondOpen.EnvVars[0].Masked = false, want true")
	}
}

func writeProjectFiles(t *testing.T, root string, withModule bool) {
	t.Helper()
	if withModule {
		writeFile(t, filepath.Join(root, "go.mod"), "module example.com/test\n")
	}
	writeFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("docs"), 0o644); err != nil {
		t.Fatalf("WriteFile(README) error = %v", err)
	}
}
