package download

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallGoplsBinary installs gopls using go install.
func InstallGoplsBinary(ctx context.Context, goPath string, targetBinDir string, onProgress OnProgress) error {
	return goInstallTool(ctx, goPath, targetBinDir, "gopls", "golang.org/x/tools/gopls@latest", onProgress)
}

// InstallStaticcheckBinary installs staticcheck using go install.
func InstallStaticcheckBinary(ctx context.Context, goPath string, targetBinDir string, onProgress OnProgress) error {
	return goInstallTool(ctx, goPath, targetBinDir, "staticcheck", "honnef.co/go/tools/cmd/staticcheck@latest", onProgress)
}

func goInstallTool(ctx context.Context, goPath string, targetBinDir string, toolName string, pkg string, onProgress OnProgress) error {
	goBin := goPath
	if goBin == "" {
		goBin = "go"
	}

	if err := os.MkdirAll(targetBinDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	if onProgress != nil {
		onProgress(Progress{
			Tool:    toolName,
			Stage:   "installing",
			Message: fmt.Sprintf("Installing %s...", toolName),
		})
	}

	cmd := exec.CommandContext(ctx, goBin, "install", pkg)
	// Build a clean env with GOBIN set, removing any pre-existing GOBIN.
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GOBIN=") {
			env = append(env, e)
		}
	}
	cmd.Env = append(env, "GOBIN="+filepath.Clean(targetBinDir))

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start go install: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if onProgress != nil {
			onProgress(Progress{
				Tool:    toolName,
				Stage:   "installing",
				Message: line,
			})
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("go install %s: %w", toolName, err)
	}

	if onProgress != nil {
		onProgress(Progress{
			Tool:    toolName,
			Stage:   "complete",
			Percent: 100,
			Message: fmt.Sprintf("%s installed successfully", toolName),
		})
	}

	return nil
}
