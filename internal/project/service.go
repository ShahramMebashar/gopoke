package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"gopoke/internal/storage"
)

// Service handles project open/discovery workflows.
type Service struct {
	store *storage.Store
}

// OpenProjectResult returns information required by UI after opening a project.
type OpenProjectResult struct {
	Project         storage.ProjectRecord
	Module          ModuleInfo
	Targets         []RunTarget
	EnvVars         []storage.EnvVarRecord
	EnvLoadWarnings []string
}

// NewService constructs a project service.
func NewService(store *storage.Store) *Service {
	return &Service{store: store}
}

// Open validates a project path, discovers module context and run targets, and persists it as recent.
func (s *Service) Open(ctx context.Context, path string) (OpenProjectResult, error) {
	if err := ctx.Err(); err != nil {
		return OpenProjectResult{}, fmt.Errorf("open project context: %w", err)
	}
	if path == "" {
		return OpenProjectResult{}, fmt.Errorf("project path is required")
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("resolve project path: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("inspect project path: %w", err)
	}
	if !info.IsDir() {
		return OpenProjectResult{}, fmt.Errorf("project path must be a directory")
	}

	moduleInfo, err := DetectModule(ctx, absolutePath)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("detect module: %w", err)
	}
	targets, err := DiscoverRunTargets(ctx, absolutePath)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("discover run targets: %w", err)
	}

	existing, found, err := s.store.ProjectByPath(ctx, absolutePath)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("load existing project: %w", err)
	}

	defaultPackage := ""
	if found && hasRunTarget(targets, existing.DefaultPkg) {
		defaultPackage = existing.DefaultPkg
	} else if len(targets) > 0 {
		defaultPackage = targets[0].Package
	}

	record, err := s.store.RecordProjectOpen(ctx, absolutePath, defaultPackage)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("persist recent project: %w", err)
	}

	envFromFile, envWarnings, err := loadDotEnvFile(absolutePath)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("load .env: %w", err)
	}
	if len(envFromFile) > 0 {
		currentEnv, envErr := s.store.ProjectEnvVars(ctx, record.ID)
		if envErr != nil {
			return OpenProjectResult{}, fmt.Errorf("load current env vars: %w", envErr)
		}
		existingByKey := make(map[string]storage.EnvVarRecord, len(currentEnv))
		for _, variable := range currentEnv {
			existingByKey[variable.Key] = variable
		}

		keys := make([]string, 0, len(envFromFile))
		for key := range envFromFile {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			masked := false
			if existing, exists := existingByKey[key]; exists {
				masked = existing.Masked
			}
			if _, envErr := s.store.UpdateProjectEnvVar(ctx, record.ID, key, envFromFile[key], masked); envErr != nil {
				return OpenProjectResult{}, fmt.Errorf("persist .env variable %q: %w", key, envErr)
			}
		}
	}

	envVars, err := s.store.ProjectEnvVars(ctx, record.ID)
	if err != nil {
		return OpenProjectResult{}, fmt.Errorf("load project env vars: %w", err)
	}
	return OpenProjectResult{
		Project:         record,
		Module:          moduleInfo,
		Targets:         targets,
		EnvVars:         envVars,
		EnvLoadWarnings: envWarnings,
	}, nil
}

// Recent returns most-recently-opened projects.
func (s *Service) Recent(ctx context.Context, limit int) ([]storage.ProjectRecord, error) {
	records, err := s.store.RecentProjects(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("load recent projects: %w", err)
	}
	return records, nil
}

// SetDefaultPackage stores a project's default run target package.
func (s *Service) SetDefaultPackage(ctx context.Context, projectPath string, packagePath string) (storage.ProjectRecord, error) {
	if err := ctx.Err(); err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("set default package context: %w", err)
	}
	if projectPath == "" {
		return storage.ProjectRecord{}, fmt.Errorf("project path is required")
	}
	if packagePath == "" {
		return storage.ProjectRecord{}, fmt.Errorf("package path is required")
	}

	absolutePath, err := filepath.Abs(projectPath)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("resolve project path: %w", err)
	}
	targets, err := DiscoverRunTargets(ctx, absolutePath)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("discover run targets: %w", err)
	}
	if !hasRunTarget(targets, packagePath) {
		return storage.ProjectRecord{}, fmt.Errorf("package %q is not a runnable target", packagePath)
	}

	record, err := s.store.UpdateProjectDefaultPackage(ctx, absolutePath, packagePath)
	if err != nil {
		return storage.ProjectRecord{}, fmt.Errorf("persist default package: %w", err)
	}
	return record, nil
}

func hasRunTarget(targets []RunTarget, packagePath string) bool {
	return slices.ContainsFunc(targets, func(target RunTarget) bool {
		return target.Package == packagePath
	})
}
