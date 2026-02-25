package download

import (
	"context"
	"testing"
)

func TestDefaultBaseDir(t *testing.T) {
	t.Parallel()
	dir := DefaultBaseDir()
	if dir == "" {
		t.Fatal("DefaultBaseDir() returned empty string")
	}
}

func TestManagerPaths(t *testing.T) {
	t.Parallel()
	m := NewManager("/tmp/test-toolchain")

	if got := m.ToolchainDir(); got != "/tmp/test-toolchain" {
		t.Fatalf("ToolchainDir() = %q, want /tmp/test-toolchain", got)
	}
	if got := m.GoSDKDir(); got != "/tmp/test-toolchain/go" {
		t.Fatalf("GoSDKDir() = %q, want /tmp/test-toolchain/go", got)
	}
	if got := m.ToolBinDir(); got != "/tmp/test-toolchain/bin" {
		t.Fatalf("ToolBinDir() = %q, want /tmp/test-toolchain/bin", got)
	}
}

func TestCalcPercent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		received int64
		total    int64
		want     float64
	}{
		{0, 100, 0},
		{50, 100, 50},
		{100, 100, 100},
		{150, 100, 100}, // clamped
		{0, 0, 0},       // zero total
		{50, -1, 0},     // negative total
	}
	for _, tt := range tests {
		got := calcPercent(tt.received, tt.total)
		if got != tt.want {
			t.Errorf("calcPercent(%d, %d) = %f, want %f", tt.received, tt.total, got, tt.want)
		}
	}
}

func TestStartDownloadPreventsDouble(t *testing.T) {
	t.Parallel()
	m := NewManager(t.TempDir())
	ctx := context.Background()

	_, cancel1, err := m.startDownload(ctx, "go")
	if err != nil {
		t.Fatalf("first startDownload should succeed: %v", err)
	}

	_, _, err = m.startDownload(ctx, "go")
	if err == nil {
		t.Fatal("second startDownload should fail")
	}

	cancel1()
	m.finishDownload("go")

	_, cancel3, err := m.startDownload(ctx, "go")
	if err != nil {
		t.Fatalf("startDownload after finish should succeed: %v", err)
	}
	cancel3()
	m.finishDownload("go")
}
