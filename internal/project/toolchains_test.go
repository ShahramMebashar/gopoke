package project

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestDiscoverToolchains(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	toolchains, err := DiscoverToolchains(context.Background())
	if err != nil {
		t.Fatalf("DiscoverToolchains() error = %v", err)
	}
	if len(toolchains) == 0 {
		t.Fatal("len(toolchains) = 0, want > 0")
	}
	if got := toolchains[0].Name; got != "go" {
		t.Fatalf("toolchains[0].Name = %q, want %q", got, "go")
	}
	if strings.TrimSpace(toolchains[0].Path) == "" {
		t.Fatal("toolchains[0].Path is empty")
	}
}

func TestResolveToolchainBinary(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not available")
	}

	resolved, err := ResolveToolchainBinary("go")
	if err != nil {
		t.Fatalf("ResolveToolchainBinary(go) error = %v", err)
	}
	if strings.TrimSpace(resolved) == "" {
		t.Fatal("resolved path is empty")
	}

	if _, err := ResolveToolchainBinary("go-not-installed-123"); err == nil {
		t.Fatal("ResolveToolchainBinary(invalid) error = nil, want non-nil")
	}
}
