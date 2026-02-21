package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const stateFileName = "state.json"

// HealthReport describes storage readiness.
type HealthReport struct {
	Ready         bool
	Path          string
	SchemaVersion int
}

// Store owns local on-disk state operations.
type Store struct {
	mu      sync.RWMutex
	rootDir string
	path    string
}

// New creates a store rooted at the provided directory.
func New(rootDir string) *Store {
	return &Store{
		rootDir: rootDir,
		path:    filepath.Join(rootDir, stateFileName),
	}
}

// Path returns the full state file location.
func (s *Store) Path() string {
	return s.path
}

// Bootstrap initializes the storage directory and schema file.
func (s *Store) Bootstrap(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("bootstrap context: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	_, err := os.Stat(s.path)
	switch {
	case err == nil:
		_, loadErr := s.loadLocked()
		if loadErr != nil {
			return fmt.Errorf("load existing state: %w", loadErr)
		}
		return nil
	case errors.Is(err, os.ErrNotExist):
		return s.writeLocked(newSnapshot())
	default:
		return fmt.Errorf("inspect state file: %w", err)
	}
}

// Health verifies state readability and reports schema information.
func (s *Store) Health(ctx context.Context) (HealthReport, error) {
	if err := ctx.Err(); err != nil {
		return HealthReport{}, fmt.Errorf("health context: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return HealthReport{}, fmt.Errorf("load state for health: %w", err)
	}

	return HealthReport{
		Ready:         true,
		Path:          s.path,
		SchemaVersion: snapshot.SchemaVersion,
	}, nil
}

// Load returns the current snapshot.
func (s *Store) Load(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("load context: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return Snapshot{}, fmt.Errorf("load state: %w", err)
	}
	return snapshot, nil
}

// RecordProjectOpen upserts a project and updates its last-opened timestamp.
func (s *Store) RecordProjectOpen(ctx context.Context, path string, defaultPackage string) (ProjectRecord, error) {
	if err := ctx.Err(); err != nil {
		return ProjectRecord{}, fmt.Errorf("record project context: %w", err)
	}
	if path == "" {
		return ProjectRecord{}, fmt.Errorf("project path is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return ProjectRecord{}, fmt.Errorf("load state: %w", err)
	}

	now := time.Now().UTC()
	normalizedPath := filepath.Clean(path)
	var record ProjectRecord
	found := false

	for i, existing := range snapshot.Projects {
		if existing.Path == normalizedPath {
			record = existing
			record.LastOpenedAt = now
			if strings.TrimSpace(defaultPackage) != "" {
				record.DefaultPkg = defaultPackage
			}
			snapshot.Projects[i] = record
			found = true
			break
		}
	}

	if !found {
		record = ProjectRecord{
			ID:           generateID("prj"),
			Path:         normalizedPath,
			LastOpenedAt: now,
			DefaultPkg:   defaultPackage,
		}
		snapshot.Projects = append(snapshot.Projects, record)
	}

	snapshot.Meta.UpdatedAt = now
	if err := s.writeLocked(snapshot); err != nil {
		return ProjectRecord{}, fmt.Errorf("persist project state: %w", err)
	}
	return record, nil
}

// ProjectByPath returns a project record for a given path.
func (s *Store) ProjectByPath(ctx context.Context, path string) (ProjectRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return ProjectRecord{}, false, fmt.Errorf("project by path context: %w", err)
	}
	if path == "" {
		return ProjectRecord{}, false, fmt.Errorf("project path is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return ProjectRecord{}, false, fmt.Errorf("load state: %w", err)
	}

	normalizedPath := filepath.Clean(path)
	for _, project := range snapshot.Projects {
		if project.Path == normalizedPath {
			return project, true, nil
		}
	}
	return ProjectRecord{}, false, nil
}

// UpdateProjectDefaultPackage updates default package for a project without changing recency.
func (s *Store) UpdateProjectDefaultPackage(ctx context.Context, path string, defaultPackage string) (ProjectRecord, error) {
	if err := ctx.Err(); err != nil {
		return ProjectRecord{}, fmt.Errorf("update default package context: %w", err)
	}
	if path == "" {
		return ProjectRecord{}, fmt.Errorf("project path is required")
	}
	if strings.TrimSpace(defaultPackage) == "" {
		return ProjectRecord{}, fmt.Errorf("default package is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return ProjectRecord{}, fmt.Errorf("load state: %w", err)
	}

	normalizedPath := filepath.Clean(path)
	for i, existing := range snapshot.Projects {
		if existing.Path != normalizedPath {
			continue
		}
		existing.DefaultPkg = defaultPackage
		snapshot.Projects[i] = existing
		snapshot.Meta.UpdatedAt = time.Now().UTC()
		if err := s.writeLocked(snapshot); err != nil {
			return ProjectRecord{}, fmt.Errorf("persist project default package: %w", err)
		}
		return existing, nil
	}
	return ProjectRecord{}, fmt.Errorf("project not found")
}

// UpdateProjectWorkingDirectory updates the saved working directory for a project without changing recency.
func (s *Store) UpdateProjectWorkingDirectory(ctx context.Context, path string, workingDirectory string) (ProjectRecord, error) {
	if err := ctx.Err(); err != nil {
		return ProjectRecord{}, fmt.Errorf("update working directory context: %w", err)
	}
	if path == "" {
		return ProjectRecord{}, fmt.Errorf("project path is required")
	}
	if strings.TrimSpace(workingDirectory) == "" {
		return ProjectRecord{}, fmt.Errorf("working directory is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return ProjectRecord{}, fmt.Errorf("load state: %w", err)
	}

	normalizedPath := filepath.Clean(path)
	for i, existing := range snapshot.Projects {
		if existing.Path != normalizedPath {
			continue
		}
		existing.WorkingDir = filepath.Clean(workingDirectory)
		snapshot.Projects[i] = existing
		snapshot.Meta.UpdatedAt = time.Now().UTC()
		if err := s.writeLocked(snapshot); err != nil {
			return ProjectRecord{}, fmt.Errorf("persist project working directory: %w", err)
		}
		return existing, nil
	}
	return ProjectRecord{}, fmt.Errorf("project not found")
}

// UpdateProjectToolchain updates the selected Go toolchain for a project without changing recency.
func (s *Store) UpdateProjectToolchain(ctx context.Context, path string, toolchain string) (ProjectRecord, error) {
	if err := ctx.Err(); err != nil {
		return ProjectRecord{}, fmt.Errorf("update project toolchain context: %w", err)
	}
	if path == "" {
		return ProjectRecord{}, fmt.Errorf("project path is required")
	}
	if strings.TrimSpace(toolchain) == "" {
		return ProjectRecord{}, fmt.Errorf("toolchain is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return ProjectRecord{}, fmt.Errorf("load state: %w", err)
	}

	normalizedPath := filepath.Clean(path)
	for i, existing := range snapshot.Projects {
		if existing.Path != normalizedPath {
			continue
		}
		existing.Toolchain = strings.TrimSpace(toolchain)
		snapshot.Projects[i] = existing
		snapshot.Meta.UpdatedAt = time.Now().UTC()
		if err := s.writeLocked(snapshot); err != nil {
			return ProjectRecord{}, fmt.Errorf("persist project toolchain: %w", err)
		}
		return existing, nil
	}
	return ProjectRecord{}, fmt.Errorf("project not found")
}

// RecentProjects returns projects sorted by most recently opened first.
func (s *Store) RecentProjects(ctx context.Context, limit int) ([]ProjectRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("recent projects context: %w", err)
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	projects := slices.Clone(snapshot.Projects)
	slices.SortFunc(projects, func(a, b ProjectRecord) int {
		switch {
		case a.LastOpenedAt.After(b.LastOpenedAt):
			return -1
		case a.LastOpenedAt.Before(b.LastOpenedAt):
			return 1
		default:
			if a.Path < b.Path {
				return -1
			}
			if a.Path > b.Path {
				return 1
			}
			return 0
		}
	})

	if limit == 0 || limit >= len(projects) {
		return projects, nil
	}
	return projects[:limit], nil
}

// UpdateProjectEnvVar sets one environment variable for a project.
func (s *Store) UpdateProjectEnvVar(ctx context.Context, projectID string, key string, value string, masked bool) (EnvVarRecord, error) {
	if err := ctx.Err(); err != nil {
		return EnvVarRecord{}, fmt.Errorf("update env var context: %w", err)
	}
	if projectID == "" {
		return EnvVarRecord{}, fmt.Errorf("project ID is required")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return EnvVarRecord{}, fmt.Errorf("env var key is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return EnvVarRecord{}, fmt.Errorf("load state: %w", err)
	}

	now := time.Now().UTC()
	var record EnvVarRecord
	found := false

	for i, existing := range snapshot.EnvVars {
		if existing.ProjectID == projectID && existing.Key == key {
			record = existing
			record.Value = value
			record.Masked = masked
			snapshot.EnvVars[i] = record
			found = true
			break
		}
	}

	if !found {
		record = EnvVarRecord{
			ID:        generateID("env"),
			ProjectID: projectID,
			Key:       key,
			Value:     value,
			Masked:    masked,
		}
		snapshot.EnvVars = append(snapshot.EnvVars, record)
	}

	snapshot.Meta.UpdatedAt = now
	if err := s.writeLocked(snapshot); err != nil {
		return EnvVarRecord{}, fmt.Errorf("persist env vars: %w", err)
	}
	return record, nil
}

// DeleteProjectEnvVar removes one environment variable for a project.
func (s *Store) DeleteProjectEnvVar(ctx context.Context, projectID string, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete env var context: %w", err)
	}
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("env var key is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	filtered := make([]EnvVarRecord, 0, len(snapshot.EnvVars))
	removed := false
	for _, envVar := range snapshot.EnvVars {
		if envVar.ProjectID == projectID && envVar.Key == key {
			removed = true
			continue
		}
		filtered = append(filtered, envVar)
	}
	if !removed {
		return nil
	}

	snapshot.EnvVars = filtered
	snapshot.Meta.UpdatedAt = time.Now().UTC()
	if err := s.writeLocked(snapshot); err != nil {
		return fmt.Errorf("persist env vars: %w", err)
	}
	return nil
}

// ProjectEnvVars returns all environment variables for a project.
func (s *Store) ProjectEnvVars(ctx context.Context, projectID string) ([]EnvVarRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("project env vars context: %w", err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	result := make([]EnvVarRecord, 0)
	for _, envVar := range snapshot.EnvVars {
		if envVar.ProjectID == projectID {
			result = append(result, envVar)
		}
	}

	slices.SortFunc(result, func(a, b EnvVarRecord) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})
	return result, nil
}

// ProjectEnvMap returns project env vars as a key-value map for execution.
func (s *Store) ProjectEnvMap(ctx context.Context, projectID string) (map[string]string, error) {
	vars, err := s.ProjectEnvVars(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(vars))
	for _, envVar := range vars {
		result[envVar.Key] = envVar.Value
	}
	return result, nil
}

// SaveSnippet inserts or updates one snippet for a project.
func (s *Store) SaveSnippet(ctx context.Context, record SnippetRecord) (SnippetRecord, error) {
	if err := ctx.Err(); err != nil {
		return SnippetRecord{}, fmt.Errorf("save snippet context: %w", err)
	}
	if record.ProjectID == "" {
		return SnippetRecord{}, fmt.Errorf("project ID is required")
	}
	record.Name = strings.TrimSpace(record.Name)
	if record.Name == "" {
		return SnippetRecord{}, fmt.Errorf("snippet name is required")
	}
	if strings.TrimSpace(record.Content) == "" {
		return SnippetRecord{}, fmt.Errorf("snippet content is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return SnippetRecord{}, fmt.Errorf("load state: %w", err)
	}
	if !projectExists(snapshot.Projects, record.ProjectID) {
		return SnippetRecord{}, fmt.Errorf("project not found")
	}

	now := time.Now().UTC()
	if record.ID == "" {
		if snippetNameExists(snapshot.Snippets, record.ProjectID, "", record.Name) {
			return SnippetRecord{}, fmt.Errorf("snippet name %q already exists", record.Name)
		}
		record.ID = generateID("sn")
		record.CreatedAt = now
		record.UpdatedAt = now
		snapshot.Snippets = append(snapshot.Snippets, record)
	} else {
		updated := false
		for i, existing := range snapshot.Snippets {
			if existing.ID != record.ID {
				continue
			}
			if existing.ProjectID != record.ProjectID {
				return SnippetRecord{}, fmt.Errorf("snippet project mismatch")
			}
			if snippetNameExists(snapshot.Snippets, record.ProjectID, existing.ID, record.Name) {
				return SnippetRecord{}, fmt.Errorf("snippet name %q already exists", record.Name)
			}
			existing.Name = record.Name
			existing.Content = record.Content
			existing.UpdatedAt = now
			snapshot.Snippets[i] = existing
			record = existing
			updated = true
			break
		}
		if !updated {
			return SnippetRecord{}, fmt.Errorf("snippet not found")
		}
	}

	snapshot.Meta.UpdatedAt = now
	if err := s.writeLocked(snapshot); err != nil {
		return SnippetRecord{}, fmt.Errorf("persist snippets: %w", err)
	}
	return record, nil
}

// ProjectSnippets returns project snippets sorted by latest update first.
func (s *Store) ProjectSnippets(ctx context.Context, projectID string) ([]SnippetRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("project snippets context: %w", err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	result := make([]SnippetRecord, 0)
	for _, snippet := range snapshot.Snippets {
		if snippet.ProjectID == projectID {
			result = append(result, snippet)
		}
	}
	slices.SortFunc(result, func(a, b SnippetRecord) int {
		switch {
		case a.UpdatedAt.After(b.UpdatedAt):
			return -1
		case a.UpdatedAt.Before(b.UpdatedAt):
			return 1
		default:
			if a.Name < b.Name {
				return -1
			}
			if a.Name > b.Name {
				return 1
			}
			return 0
		}
	})
	return result, nil
}

// SnippetByID returns one snippet by ID.
func (s *Store) SnippetByID(ctx context.Context, snippetID string) (SnippetRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return SnippetRecord{}, false, fmt.Errorf("snippet by id context: %w", err)
	}
	if strings.TrimSpace(snippetID) == "" {
		return SnippetRecord{}, false, fmt.Errorf("snippet ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return SnippetRecord{}, false, fmt.Errorf("load state: %w", err)
	}
	for _, snippet := range snapshot.Snippets {
		if snippet.ID == snippetID {
			return snippet, true, nil
		}
	}
	return SnippetRecord{}, false, nil
}

// DeleteSnippet removes one snippet by ID.
func (s *Store) DeleteSnippet(ctx context.Context, snippetID string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete snippet context: %w", err)
	}
	snippetID = strings.TrimSpace(snippetID)
	if snippetID == "" {
		return fmt.Errorf("snippet ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	filtered := make([]SnippetRecord, 0, len(snapshot.Snippets))
	removed := false
	for _, snippet := range snapshot.Snippets {
		if snippet.ID == snippetID {
			removed = true
			continue
		}
		filtered = append(filtered, snippet)
	}
	if !removed {
		return nil
	}

	snapshot.Snippets = filtered
	snapshot.Meta.UpdatedAt = time.Now().UTC()
	if err := s.writeLocked(snapshot); err != nil {
		return fmt.Errorf("persist snippets: %w", err)
	}
	return nil
}

// RecordRun appends one run metadata record to the local state snapshot.
func (s *Store) RecordRun(ctx context.Context, record RunRecord) (RunRecord, error) {
	if err := ctx.Err(); err != nil {
		return RunRecord{}, fmt.Errorf("record run context: %w", err)
	}
	if record.ProjectID == "" {
		return RunRecord{}, fmt.Errorf("project ID is required")
	}
	if strings.TrimSpace(record.Status) == "" {
		return RunRecord{}, fmt.Errorf("run status is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return RunRecord{}, fmt.Errorf("load state: %w", err)
	}

	now := time.Now().UTC()
	if record.ID == "" {
		record.ID = generateID("run")
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = now
	}
	record.StartedAt = record.StartedAt.UTC()
	if record.DurationMS < 0 {
		record.DurationMS = 0
	}

	snapshot.Runs = append(snapshot.Runs, record)
	snapshot.Meta.UpdatedAt = now
	if err := s.writeLocked(snapshot); err != nil {
		return RunRecord{}, fmt.Errorf("persist run record: %w", err)
	}
	return record, nil
}

// ProjectRuns returns runs for one project sorted by latest start time first.
func (s *Store) ProjectRuns(ctx context.Context, projectID string, limit int) ([]RunRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("project runs context: %w", err)
	}
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, err := s.loadLocked()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	runs := make([]RunRecord, 0)
	for _, run := range snapshot.Runs {
		if run.ProjectID == projectID {
			runs = append(runs, run)
		}
	}

	slices.SortFunc(runs, func(a, b RunRecord) int {
		switch {
		case a.StartedAt.After(b.StartedAt):
			return -1
		case a.StartedAt.Before(b.StartedAt):
			return 1
		default:
			if a.ID < b.ID {
				return -1
			}
			if a.ID > b.ID {
				return 1
			}
			return 0
		}
	})

	if limit == 0 || limit >= len(runs) {
		return runs, nil
	}
	return runs[:limit], nil
}

func (s *Store) loadLocked() (Snapshot, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return Snapshot{}, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode state json: %w", err)
	}
	if snapshot.SchemaVersion != SchemaVersionV1 {
		return Snapshot{}, fmt.Errorf("unsupported schema version: %d", snapshot.SchemaVersion)
	}
	return snapshot, nil
}

func (s *Store) writeLocked(snapshot Snapshot) error {
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state json: %w", err)
	}

	tempFile, err := os.CreateTemp(s.rootDir, "state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(encoded); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("sync temp state: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("close temp state: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}

func projectExists(projects []ProjectRecord, projectID string) bool {
	return slices.ContainsFunc(projects, func(project ProjectRecord) bool {
		return project.ID == projectID
	})
}

func snippetNameExists(snippets []SnippetRecord, projectID string, excludeID string, name string) bool {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	return slices.ContainsFunc(snippets, func(snippet SnippetRecord) bool {
		if snippet.ProjectID != projectID {
			return false
		}
		if excludeID != "" && snippet.ID == excludeID {
			return false
		}
		return strings.ToLower(strings.TrimSpace(snippet.Name)) == normalizedName
	})
}
