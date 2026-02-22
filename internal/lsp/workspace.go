package lsp

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type workspace struct {
	dir         string
	projectPath string
}

func createWorkspace(projectPath string) (*workspace, error) {
	wsDir := filepath.Join(projectPath, ".gopad-lsp-workspace")
	if err := os.MkdirAll(wsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}

	goVersion := readGoVersionFromProject(projectPath)
	if goVersion == "" {
		goVersion = "1.22"
	}

	goModContent := fmt.Sprintf("module gopad-snippet\n\ngo %s\n", goVersion)

	if err := os.WriteFile(filepath.Join(wsDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		return nil, fmt.Errorf("write workspace go.mod: %w", err)
	}

	// Copy go.sum if it exists in the project
	goSumPath := filepath.Join(projectPath, "go.sum")
	if data, err := os.ReadFile(goSumPath); err == nil {
		os.WriteFile(filepath.Join(wsDir, "go.sum"), data, 0o644)
	}

	return &workspace{
		dir:         wsDir,
		projectPath: projectPath,
	}, nil
}

func (w *workspace) snippetFilePath() string {
	return filepath.Join(w.dir, "main.go")
}

func (w *workspace) snippetURI() string {
	return "file://" + w.snippetFilePath()
}

func (w *workspace) cleanup() {
	os.RemoveAll(w.dir)
}

func readGoVersionFromProject(projectPath string) string {
	goModPath := filepath.Join(projectPath, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go"))
		}
	}
	return ""
}
