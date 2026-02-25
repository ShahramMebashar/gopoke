package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Manager orchestrates toolchain downloads and installations.
type Manager struct {
	mu          sync.Mutex
	baseDir     string
	downloading map[string]context.CancelFunc
}

// NewManager creates a download manager with the given base directory.
func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:     baseDir,
		downloading: make(map[string]context.CancelFunc),
	}
}

// ToolchainDir returns the base toolchain directory.
func (m *Manager) ToolchainDir() string {
	return m.baseDir
}

// GoSDKDir returns the path where Go SDK will be extracted.
func (m *Manager) GoSDKDir() string {
	return filepath.Join(m.baseDir, "go")
}

// GoBinPath returns the expected path to the go binary in the managed SDK.
func (m *Manager) GoBinPath() string {
	bin := "go"
	if runtime.GOOS == "windows" {
		bin = "go.exe"
	}
	return filepath.Join(m.GoSDKDir(), "bin", bin)
}

// ToolBinDir returns the directory for installed tool binaries.
func (m *Manager) ToolBinDir() string {
	return filepath.Join(m.baseDir, "bin")
}

// DownloadGoSDK downloads and installs a Go SDK version.
func (m *Manager) DownloadGoSDK(ctx context.Context, version string, onProgress OnProgress) error {
	dlCtx, cancel, err := m.startDownload(ctx, "go")
	if err != nil {
		return err
	}
	defer cancel()
	defer m.finishDownload("go")

	return DownloadGoSDK(dlCtx, version, m.baseDir, onProgress)
}

// InstallGopls installs gopls using the configured (or managed) Go binary.
func (m *Manager) InstallGopls(ctx context.Context, goPath string, onProgress OnProgress) error {
	dlCtx, cancel, err := m.startDownload(ctx, "gopls")
	if err != nil {
		return err
	}
	defer cancel()
	defer m.finishDownload("gopls")

	effectiveGo := goPath
	if effectiveGo == "" {
		if _, err := os.Stat(m.GoBinPath()); err == nil {
			effectiveGo = m.GoBinPath()
		}
	}

	return InstallGoplsBinary(dlCtx, effectiveGo, m.ToolBinDir(), onProgress)
}

// InstallStaticcheck installs staticcheck using the configured (or managed) Go binary.
func (m *Manager) InstallStaticcheck(ctx context.Context, goPath string, onProgress OnProgress) error {
	dlCtx, cancel, err := m.startDownload(ctx, "staticcheck")
	if err != nil {
		return err
	}
	defer cancel()
	defer m.finishDownload("staticcheck")

	effectiveGo := goPath
	if effectiveGo == "" {
		if _, err := os.Stat(m.GoBinPath()); err == nil {
			effectiveGo = m.GoBinPath()
		}
	}

	return InstallStaticcheckBinary(dlCtx, effectiveGo, m.ToolBinDir(), onProgress)
}

// CancelDownload cancels an in-progress download/install.
func (m *Manager) CancelDownload(tool string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.downloading[tool]; ok {
		cancel()
	}
}

// DefaultBaseDir returns the platform-appropriate toolchain directory.
func DefaultBaseDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "toolchain")
	}
	return filepath.Join(configDir, "gopoke", "toolchain")
}

// startDownload atomically reserves a download slot and returns a cancellable context.
func (m *Manager) startDownload(ctx context.Context, tool string) (context.Context, context.CancelFunc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.downloading[tool]; exists {
		return nil, nil, fmt.Errorf("%s download already in progress", tool)
	}
	dlCtx, cancel := context.WithCancel(ctx)
	m.downloading[tool] = cancel
	return dlCtx, cancel, nil
}

func (m *Manager) finishDownload(tool string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.downloading, tool)
}
