package storage

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"gopoke/internal/settings"
)

// SchemaVersionV1 is the initial on-disk schema version.
const SchemaVersionV1 = 1

// Snapshot is persisted as one atomic state file for MVP.
type Snapshot struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	Projects       []ProjectRecord         `json:"projects"`
	Snippets       []SnippetRecord         `json:"snippets"`
	Runs           []RunRecord             `json:"runs"`
	EnvVars        []EnvVarRecord          `json:"envVars"`
	GlobalSettings settings.GlobalSettings `json:"globalSettings"`
	Meta           SnapshotMetadata        `json:"meta"`
}

// SnapshotMetadata stores top-level bookkeeping.
type SnapshotMetadata struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ProjectRecord captures persisted project data.
type ProjectRecord struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	LastOpenedAt time.Time `json:"lastOpenedAt"`
	DefaultPkg   string    `json:"defaultPackage"`
	WorkingDir   string    `json:"workingDirectory"`
	Toolchain    string    `json:"toolchain"`
}

// SnippetRecord captures persisted snippet data.
type SnippetRecord struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RunRecord captures metadata for a run.
type RunRecord struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"projectId"`
	SnippetID  string    `json:"snippetId"`
	StartedAt  time.Time `json:"startedAt"`
	DurationMS int64     `json:"durationMs"`
	ExitCode   int       `json:"exitCode"`
	Status     string    `json:"status"`
}

// EnvVarRecord captures a project-level environment variable.
type EnvVarRecord struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Masked    bool   `json:"masked"`
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return prefix + "_" + hex.EncodeToString(b)
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func newSnapshot() Snapshot {
	now := time.Now().UTC()
	return Snapshot{
		SchemaVersion:  SchemaVersionV1,
		Projects:       make([]ProjectRecord, 0),
		Snippets:       make([]SnippetRecord, 0),
		Runs:           make([]RunRecord, 0),
		EnvVars:        make([]EnvVarRecord, 0),
		GlobalSettings: settings.Defaults(),
		Meta: SnapshotMetadata{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}
