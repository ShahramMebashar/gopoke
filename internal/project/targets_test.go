package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRunTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(root, "helper.go"), "package main\n\nfunc helper() {}\n")

	cmdDir := filepath.Join(root, "cmd", "api")
	writeFile(t, filepath.Join(cmdDir, "main.go"), "package main\n\nfunc main() {}\n")

	internalDir := filepath.Join(root, "internal", "pkg")
	writeFile(t, filepath.Join(internalDir, "foo.go"), "package pkg\n\nfunc Foo() {}\n")

	targets, err := DiscoverRunTargets(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverRunTargets() error = %v", err)
	}

	if got, want := len(targets), 2; got != want {
		t.Fatalf("len(targets) = %d, want %d", got, want)
	}
	if got, want := targets[0].Package, "."; got != want {
		t.Fatalf("targets[0].Package = %q, want %q", got, want)
	}
	if got, want := targets[1].Package, "./cmd/api"; got != want {
		t.Fatalf("targets[1].Package = %q, want %q", got, want)
	}
}

func TestDiscoverRunTargetsSkipsHiddenAndVendor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, ".tmp", "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(root, "vendor", "x", "main.go"), "package main\n\nfunc main() {}\n")

	targets, err := DiscoverRunTargets(context.Background(), root)
	if err != nil {
		t.Fatalf("DiscoverRunTargets() error = %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("len(targets) = %d, want 0", len(targets))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
