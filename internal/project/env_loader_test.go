package project

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDotEnv(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"# comment",
		"API_KEY=abc123",
		"export PORT=8080",
		"QUOTED=\"line\\nnext\"",
		"SINGLE='literal value'",
		"WITH_COMMENT=value # trailing",
		"BAD LINE",
		"1INVALID=bad",
		"UNFINISHED=\"oops",
		"",
	}, "\n")

	values, warnings := parseDotEnv(content)
	if got, want := values["API_KEY"], "abc123"; got != want {
		t.Fatalf("API_KEY = %q, want %q", got, want)
	}
	if got, want := values["PORT"], "8080"; got != want {
		t.Fatalf("PORT = %q, want %q", got, want)
	}
	if got, want := values["QUOTED"], "line\nnext"; got != want {
		t.Fatalf("QUOTED = %q, want %q", got, want)
	}
	if got, want := values["SINGLE"], "literal value"; got != want {
		t.Fatalf("SINGLE = %q, want %q", got, want)
	}
	if got, want := values["WITH_COMMENT"], "value"; got != want {
		t.Fatalf("WITH_COMMENT = %q, want %q", got, want)
	}
	if got, want := len(warnings), 3; got != want {
		t.Fatalf("len(warnings) = %d, want %d (%v)", got, want, warnings)
	}
}

func TestLoadDotEnvFileMissingIsNoop(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	values, warnings, err := loadDotEnvFile(projectDir)
	if err != nil {
		t.Fatalf("loadDotEnvFile() error = %v", err)
	}
	if got := len(values); got != 0 {
		t.Fatalf("len(values) = %d, want 0", got)
	}
	if got := len(warnings); got != 0 {
		t.Fatalf("len(warnings) = %d, want 0", got)
	}
}

func TestLoadDotEnvFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".env"), "A=1\nB=2\n")

	values, warnings, err := loadDotEnvFile(projectDir)
	if err != nil {
		t.Fatalf("loadDotEnvFile() error = %v", err)
	}
	if got, want := len(warnings), 0; got != want {
		t.Fatalf("len(warnings) = %d, want %d", got, want)
	}
	if got, want := values["A"], "1"; got != want {
		t.Fatalf("A = %q, want %q", got, want)
	}
	if got, want := values["B"], "2"; got != want {
		t.Fatalf("B = %q, want %q", got, want)
	}
}
