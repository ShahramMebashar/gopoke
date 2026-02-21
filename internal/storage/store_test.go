package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBootstrapCreatesSchemaV1Snapshot(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	store := New(rootDir)

	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	snapshot, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := snapshot.SchemaVersion, SchemaVersionV1; got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}

	if got, want := store.Path(), filepath.Join(rootDir, stateFileName); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestBootstrapIsIdempotent(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())

	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("first Bootstrap() error = %v", err)
	}
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("second Bootstrap() error = %v", err)
	}
}

func TestHealthReadyAfterBootstrap(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	report, err := store.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if !report.Ready {
		t.Fatal("ready = false, want true")
	}
	if got, want := report.SchemaVersion, SchemaVersionV1; got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}
}

func TestRecordProjectOpenUpsertsByPath(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	recordOne, err := store.RecordProjectOpen(context.Background(), "/tmp/project", "./cmd/api")
	if err != nil {
		t.Fatalf("RecordProjectOpen(first) error = %v", err)
	}

	time.Sleep(2 * time.Millisecond)

	recordTwo, err := store.RecordProjectOpen(context.Background(), "/tmp/project", "./cmd/web")
	if err != nil {
		t.Fatalf("RecordProjectOpen(second) error = %v", err)
	}

	if got, want := recordTwo.ID, recordOne.ID; got != want {
		t.Fatalf("record IDs differ: %q vs %q", got, want)
	}
	if got, want := recordTwo.DefaultPkg, "./cmd/web"; got != want {
		t.Fatalf("default package = %q, want %q", got, want)
	}

	recent, err := store.RecentProjects(context.Background(), 10)
	if err != nil {
		t.Fatalf("RecentProjects() error = %v", err)
	}
	if got, want := len(recent), 1; got != want {
		t.Fatalf("len(recent) = %d, want %d", got, want)
	}
}

func TestRecentProjectsSortedByLastOpened(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	_, err := store.RecordProjectOpen(context.Background(), "/tmp/a", ".")
	if err != nil {
		t.Fatalf("RecordProjectOpen(/tmp/a) error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	_, err = store.RecordProjectOpen(context.Background(), "/tmp/b", ".")
	if err != nil {
		t.Fatalf("RecordProjectOpen(/tmp/b) error = %v", err)
	}

	recent, err := store.RecentProjects(context.Background(), 1)
	if err != nil {
		t.Fatalf("RecentProjects() error = %v", err)
	}
	if got, want := len(recent), 1; got != want {
		t.Fatalf("len(recent) = %d, want %d", got, want)
	}
	if got, want := recent[0].Path, filepath.Clean("/tmp/b"); got != want {
		t.Fatalf("recent[0].Path = %q, want %q", got, want)
	}
}

func TestProjectByPath(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	_, err := store.RecordProjectOpen(context.Background(), "/tmp/project-x", "./cmd/api")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}

	record, found, err := store.ProjectByPath(context.Background(), "/tmp/project-x")
	if err != nil {
		t.Fatalf("ProjectByPath() error = %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if got, want := record.DefaultPkg, "./cmd/api"; got != want {
		t.Fatalf("record.DefaultPkg = %q, want %q", got, want)
	}
}

func TestUpdateProjectDefaultPackage(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	initial, err := store.RecordProjectOpen(context.Background(), "/tmp/project-y", "./cmd/old")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}

	updated, err := store.UpdateProjectDefaultPackage(context.Background(), "/tmp/project-y", "./cmd/new")
	if err != nil {
		t.Fatalf("UpdateProjectDefaultPackage() error = %v", err)
	}
	if got, want := updated.DefaultPkg, "./cmd/new"; got != want {
		t.Fatalf("updated.DefaultPkg = %q, want %q", got, want)
	}
	if !updated.LastOpenedAt.Equal(initial.LastOpenedAt) {
		t.Fatalf("LastOpenedAt changed: got %s want %s", updated.LastOpenedAt, initial.LastOpenedAt)
	}
}

func TestRecordRunAndProjectRuns(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	project, err := store.RecordProjectOpen(context.Background(), "/tmp/project-runs", ".")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}

	firstStarted := time.Now().UTC().Add(-2 * time.Second)
	secondStarted := time.Now().UTC().Add(-1 * time.Second)

	firstRun, err := store.RecordRun(context.Background(), RunRecord{
		ProjectID:  project.ID,
		SnippetID:  "sn_1",
		StartedAt:  firstStarted,
		DurationMS: 120,
		ExitCode:   0,
		Status:     "success",
	})
	if err != nil {
		t.Fatalf("RecordRun(first) error = %v", err)
	}
	if firstRun.ID == "" {
		t.Fatal("firstRun.ID is empty, want generated run ID")
	}

	secondRun, err := store.RecordRun(context.Background(), RunRecord{
		ID:         "run_custom",
		ProjectID:  project.ID,
		SnippetID:  "sn_1",
		StartedAt:  secondStarted,
		DurationMS: 340,
		ExitCode:   2,
		Status:     "failed",
	})
	if err != nil {
		t.Fatalf("RecordRun(second) error = %v", err)
	}
	if got, want := secondRun.ID, "run_custom"; got != want {
		t.Fatalf("secondRun.ID = %q, want %q", got, want)
	}

	runs, err := store.ProjectRuns(context.Background(), project.ID, 10)
	if err != nil {
		t.Fatalf("ProjectRuns() error = %v", err)
	}
	if got, want := len(runs), 2; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].ID, secondRun.ID; got != want {
		t.Fatalf("runs[0].ID = %q, want %q", got, want)
	}
	if got, want := runs[1].ID, firstRun.ID; got != want {
		t.Fatalf("runs[1].ID = %q, want %q", got, want)
	}

	limited, err := store.ProjectRuns(context.Background(), project.ID, 1)
	if err != nil {
		t.Fatalf("ProjectRuns(limit=1) error = %v", err)
	}
	if got, want := len(limited), 1; got != want {
		t.Fatalf("len(limited) = %d, want %d", got, want)
	}
	if got, want := limited[0].ID, secondRun.ID; got != want {
		t.Fatalf("limited[0].ID = %q, want %q", got, want)
	}
}

func TestUpdateProjectWorkingDirectoryAndToolchain(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	record, err := store.RecordProjectOpen(context.Background(), "/tmp/project-settings", ".")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}

	updatedWD, err := store.UpdateProjectWorkingDirectory(context.Background(), record.Path, "/tmp/project-settings/cmd/api")
	if err != nil {
		t.Fatalf("UpdateProjectWorkingDirectory() error = %v", err)
	}
	if got, want := updatedWD.WorkingDir, filepath.Clean("/tmp/project-settings/cmd/api"); got != want {
		t.Fatalf("updatedWD.WorkingDir = %q, want %q", got, want)
	}

	updatedToolchain, err := store.UpdateProjectToolchain(context.Background(), record.Path, "go1.25.1")
	if err != nil {
		t.Fatalf("UpdateProjectToolchain() error = %v", err)
	}
	if got, want := updatedToolchain.Toolchain, "go1.25.1"; got != want {
		t.Fatalf("updatedToolchain.Toolchain = %q, want %q", got, want)
	}

	found, ok, err := store.ProjectByPath(context.Background(), record.Path)
	if err != nil {
		t.Fatalf("ProjectByPath() error = %v", err)
	}
	if !ok {
		t.Fatal("project not found after updates")
	}
	if got, want := found.WorkingDir, filepath.Clean("/tmp/project-settings/cmd/api"); got != want {
		t.Fatalf("found.WorkingDir = %q, want %q", got, want)
	}
	if got, want := found.Toolchain, "go1.25.1"; got != want {
		t.Fatalf("found.Toolchain = %q, want %q", got, want)
	}
}

func TestProjectEnvVarCRUD(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	project, err := store.RecordProjectOpen(context.Background(), "/tmp/project-env", ".")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}

	created, err := store.UpdateProjectEnvVar(context.Background(), project.ID, "API_KEY", "one", true)
	if err != nil {
		t.Fatalf("UpdateProjectEnvVar(create) error = %v", err)
	}
	if got, want := created.Key, "API_KEY"; got != want {
		t.Fatalf("created.Key = %q, want %q", got, want)
	}
	if !created.Masked {
		t.Fatal("created.Masked = false, want true")
	}

	updated, err := store.UpdateProjectEnvVar(context.Background(), project.ID, "API_KEY", "two", false)
	if err != nil {
		t.Fatalf("UpdateProjectEnvVar(update) error = %v", err)
	}
	if got, want := updated.ID, created.ID; got != want {
		t.Fatalf("updated.ID = %q, want %q", got, want)
	}
	if updated.Masked {
		t.Fatal("updated.Masked = true, want false")
	}

	vars, err := store.ProjectEnvVars(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ProjectEnvVars() error = %v", err)
	}
	if got, want := len(vars), 1; got != want {
		t.Fatalf("len(vars) = %d, want %d", got, want)
	}
	if got, want := vars[0].Value, "two"; got != want {
		t.Fatalf("vars[0].Value = %q, want %q", got, want)
	}

	envMap, err := store.ProjectEnvMap(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ProjectEnvMap() error = %v", err)
	}
	if got, want := envMap["API_KEY"], "two"; got != want {
		t.Fatalf("envMap[API_KEY] = %q, want %q", got, want)
	}

	if err := store.DeleteProjectEnvVar(context.Background(), project.ID, "API_KEY"); err != nil {
		t.Fatalf("DeleteProjectEnvVar() error = %v", err)
	}
	vars, err = store.ProjectEnvVars(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ProjectEnvVars(after delete) error = %v", err)
	}
	if got, want := len(vars), 0; got != want {
		t.Fatalf("len(vars) = %d, want %d", got, want)
	}
}

func TestSnippetCRUD(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	project, err := store.RecordProjectOpen(context.Background(), "/tmp/project-snippets", ".")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}

	saved, err := store.SaveSnippet(context.Background(), SnippetRecord{
		ProjectID: project.ID,
		Name:      "HTTP Probe",
		Content:   "package main\nfunc main(){}\n",
	})
	if err != nil {
		t.Fatalf("SaveSnippet(create) error = %v", err)
	}
	if saved.ID == "" {
		t.Fatal("saved.ID is empty, want generated ID")
	}
	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Fatal("saved timestamps are zero")
	}

	time.Sleep(2 * time.Millisecond)
	updated, err := store.SaveSnippet(context.Background(), SnippetRecord{
		ID:        saved.ID,
		ProjectID: project.ID,
		Name:      "HTTP Probe Renamed",
		Content:   "package main\nfunc main(){println(\"ok\")}\n",
	})
	if err != nil {
		t.Fatalf("SaveSnippet(update) error = %v", err)
	}
	if got, want := updated.CreatedAt, saved.CreatedAt; !got.Equal(want) {
		t.Fatalf("updated.CreatedAt changed: got %s want %s", got, want)
	}
	if !updated.UpdatedAt.After(saved.UpdatedAt) {
		t.Fatalf("updated.UpdatedAt = %s, want after %s", updated.UpdatedAt, saved.UpdatedAt)
	}

	gotByID, found, err := store.SnippetByID(context.Background(), saved.ID)
	if err != nil {
		t.Fatalf("SnippetByID() error = %v", err)
	}
	if !found {
		t.Fatal("SnippetByID() found = false, want true")
	}
	if got, want := gotByID.Name, "HTTP Probe Renamed"; got != want {
		t.Fatalf("gotByID.Name = %q, want %q", got, want)
	}

	listed, err := store.ProjectSnippets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ProjectSnippets() error = %v", err)
	}
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(listed) = %d, want %d", got, want)
	}

	if err := store.DeleteSnippet(context.Background(), saved.ID); err != nil {
		t.Fatalf("DeleteSnippet() error = %v", err)
	}
	listed, err = store.ProjectSnippets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ProjectSnippets(after delete) error = %v", err)
	}
	if got, want := len(listed), 0; got != want {
		t.Fatalf("len(listed) = %d, want %d", got, want)
	}
}

func TestSaveSnippetRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	project, err := store.RecordProjectOpen(context.Background(), "/tmp/project-snippets-invalid", ".")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}

	_, err = store.SaveSnippet(context.Background(), SnippetRecord{
		ProjectID: project.ID,
		Name:      "",
		Content:   "package main\nfunc main(){}\n",
	})
	if err == nil || !strings.Contains(err.Error(), "snippet name is required") {
		t.Fatalf("SaveSnippet(empty name) error = %v, want name validation error", err)
	}

	_, err = store.SaveSnippet(context.Background(), SnippetRecord{
		ProjectID: project.ID,
		Name:      "NoContent",
		Content:   "",
	})
	if err == nil || !strings.Contains(err.Error(), "snippet content is required") {
		t.Fatalf("SaveSnippet(empty content) error = %v, want content validation error", err)
	}
}

func TestStorePersistenceAcrossRestart(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	store := New(rootDir)
	if err := store.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	project, err := store.RecordProjectOpen(context.Background(), "/tmp/project-restart", "./cmd/api")
	if err != nil {
		t.Fatalf("RecordProjectOpen() error = %v", err)
	}
	if _, err := store.UpdateProjectWorkingDirectory(context.Background(), project.Path, "/tmp/project-restart/cmd/api"); err != nil {
		t.Fatalf("UpdateProjectWorkingDirectory() error = %v", err)
	}
	if _, err := store.UpdateProjectToolchain(context.Background(), project.Path, "go"); err != nil {
		t.Fatalf("UpdateProjectToolchain() error = %v", err)
	}
	if _, err := store.UpdateProjectEnvVar(context.Background(), project.ID, "TOKEN", "abc", true); err != nil {
		t.Fatalf("UpdateProjectEnvVar() error = %v", err)
	}
	snippet, err := store.SaveSnippet(context.Background(), SnippetRecord{
		ProjectID: project.ID,
		Name:      "SavedSnippet",
		Content:   "package main\nfunc main(){}\n",
	})
	if err != nil {
		t.Fatalf("SaveSnippet() error = %v", err)
	}
	if snippet.ID == "" {
		t.Fatal("snippet.ID is empty")
	}

	restarted := New(rootDir)
	if err := restarted.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap(restarted) error = %v", err)
	}

	reloadedProject, found, err := restarted.ProjectByPath(context.Background(), project.Path)
	if err != nil {
		t.Fatalf("ProjectByPath(restarted) error = %v", err)
	}
	if !found {
		t.Fatal("project not found after restart")
	}
	if got, want := reloadedProject.WorkingDir, filepath.Clean("/tmp/project-restart/cmd/api"); got != want {
		t.Fatalf("reloadedProject.WorkingDir = %q, want %q", got, want)
	}
	if got, want := reloadedProject.Toolchain, "go"; got != want {
		t.Fatalf("reloadedProject.Toolchain = %q, want %q", got, want)
	}

	envVars, err := restarted.ProjectEnvVars(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ProjectEnvVars(restarted) error = %v", err)
	}
	if got, want := len(envVars), 1; got != want {
		t.Fatalf("len(envVars) = %d, want %d", got, want)
	}
	if got, want := envVars[0].Key, "TOKEN"; got != want {
		t.Fatalf("envVars[0].Key = %q, want %q", got, want)
	}

	snippets, err := restarted.ProjectSnippets(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("ProjectSnippets(restarted) error = %v", err)
	}
	if got, want := len(snippets), 1; got != want {
		t.Fatalf("len(snippets) = %d, want %d", got, want)
	}
	if got, want := snippets[0].ID, snippet.ID; got != want {
		t.Fatalf("snippets[0].ID = %q, want %q", got, want)
	}
}

func TestBootstrapDetectsCorruptStateFile(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	statePath := filepath.Join(rootDir, stateFileName)
	if err := os.WriteFile(statePath, []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}

	store := New(rootDir)
	err := store.Bootstrap(context.Background())
	if err == nil {
		t.Fatal("Bootstrap() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "decode state json") {
		t.Fatalf("Bootstrap() error = %q, want decode state json error", err)
	}
}

func TestBootstrapDetectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	statePath := filepath.Join(rootDir, stateFileName)
	if err := os.WriteFile(statePath, []byte(`{"schemaVersion":999}`), 0o644); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}

	store := New(rootDir)
	err := store.Bootstrap(context.Background())
	if err == nil {
		t.Fatal("Bootstrap() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unsupported schema version") {
		t.Fatalf("Bootstrap() error = %q, want unsupported schema version error", err)
	}
}
