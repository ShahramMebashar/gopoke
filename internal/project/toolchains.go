package project

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var goToolchainPattern = regexp.MustCompile(`^go(?:\d+(?:\.\d+)*)?$`)

// ToolchainInfo describes one available Go toolchain on PATH.
type ToolchainInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// DiscoverToolchains enumerates Go toolchain binaries available on PATH.
func DiscoverToolchains(ctx context.Context) ([]ToolchainInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("discover toolchains context: %w", err)
	}

	candidateNames := map[string]struct{}{
		"go": {},
	}
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("discover toolchains context: %w", err)
		}
		if strings.TrimSpace(directory) == "" {
			continue
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !goToolchainPattern.MatchString(name) {
				continue
			}
			candidateNames[name] = struct{}{}
		}
	}

	names := make([]string, 0, len(candidateNames))
	for name := range candidateNames {
		if name != "go" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	names = append([]string{"go"}, names...)

	toolchains := make([]ToolchainInfo, 0, len(names))
	seenPaths := make(map[string]struct{})
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("discover toolchains context: %w", err)
		}
		resolvedPath, err := ResolveToolchainBinary(name)
		if err != nil {
			continue
		}
		if _, exists := seenPaths[resolvedPath]; exists {
			continue
		}
		seenPaths[resolvedPath] = struct{}{}
		toolchains = append(toolchains, ToolchainInfo{
			Name:    name,
			Path:    resolvedPath,
			Version: toolchainVersion(ctx, resolvedPath),
		})
	}

	if len(toolchains) == 0 {
		return nil, fmt.Errorf("no Go toolchains found in PATH")
	}
	return toolchains, nil
}

// ResolveToolchainBinary resolves a toolchain name/path to an executable binary path.
func ResolveToolchainBinary(value string) (string, error) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", fmt.Errorf("toolchain is required")
	}
	if filepath.IsAbs(candidate) {
		info, err := os.Stat(candidate)
		if err != nil {
			return "", fmt.Errorf("inspect toolchain path: %w", err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("toolchain path must be a file")
		}
		if !isExecutable(info.Mode()) {
			return "", fmt.Errorf("toolchain path is not executable")
		}
		return candidate, nil
	}
	resolvedPath, err := exec.LookPath(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve toolchain %q: %w", candidate, err)
	}
	return resolvedPath, nil
}

func toolchainVersion(ctx context.Context, binaryPath string) string {
	output, err := exec.CommandContext(ctx, binaryPath, "version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "unknown"
	}
	return text
}

func isExecutable(mode os.FileMode) bool {
	return mode&0o111 != 0
}
