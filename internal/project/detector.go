package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ModuleInfo describes whether a project has Go module context.
type ModuleInfo struct {
	Path       string
	ModuleFile string
	HasModule  bool
}

// DetectModule checks for a go.mod file in the given path.
func DetectModule(ctx context.Context, path string) (ModuleInfo, error) {
	if err := ctx.Err(); err != nil {
		return ModuleInfo{}, fmt.Errorf("detect module context: %w", err)
	}
	if path == "" {
		return ModuleInfo{}, fmt.Errorf("path is required")
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return ModuleInfo{}, fmt.Errorf("resolve absolute path: %w", err)
	}

	moduleFile := filepath.Join(absolutePath, "go.mod")
	if _, err := os.Stat(moduleFile); err != nil {
		if os.IsNotExist(err) {
			return ModuleInfo{
				Path:       absolutePath,
				ModuleFile: moduleFile,
				HasModule:  false,
			}, nil
		}
		return ModuleInfo{}, fmt.Errorf("inspect go.mod: %w", err)
	}

	return ModuleInfo{
		Path:       absolutePath,
		ModuleFile: moduleFile,
		HasModule:  true,
	}, nil
}
