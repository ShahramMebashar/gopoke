package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCompileErrorsFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fixture  string
		expected []Diagnostic
	}{
		{
			name:    "multiple compile errors",
			fixture: "compile_errors/multiple.txt",
			expected: []Diagnostic{
				{
					Kind:    KindCompile,
					File:    "./main.go",
					Line:    14,
					Column:  2,
					Message: "undefined: missingValue",
				},
				{
					Kind:    KindCompile,
					File:    "./main.go",
					Line:    27,
					Column:  18,
					Message: "cannot use \"abc\" (untyped string constant) as int value in argument to sum",
				},
			},
		},
		{
			name:    "absolute path compile error",
			fixture: "compile_errors/absolute_path.txt",
			expected: []Diagnostic{
				{
					Kind:    KindCompile,
					File:    "/Users/tester/work/main.go",
					Line:    9,
					Column:  10,
					Message: "not enough arguments in call to handler",
				},
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := loadFixture(t, testCase.fixture)
			got := ParseCompileErrors(fixture)
			if len(got) != len(testCase.expected) {
				t.Fatalf("len(got) = %d, want %d", len(got), len(testCase.expected))
			}
			for i := range testCase.expected {
				assertDiagnosticEqual(t, got[i], testCase.expected[i])
			}
		})
	}
}

func TestParseRuntimePanicsFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fixture  string
		expected []Diagnostic
	}{
		{
			name:    "simple panic stack",
			fixture: "runtime_panics/simple.txt",
			expected: []Diagnostic{
				{
					Kind:    KindPanic,
					File:    "/var/folders/a1/b2/example/main.go",
					Line:    17,
					Column:  1,
					Message: "index out of range [3] with length 3",
				},
				{
					Kind:    KindPanic,
					File:    "/var/folders/a1/b2/example/wrapper.go",
					Line:    31,
					Column:  1,
					Message: "index out of range [3] with length 3",
				},
			},
		},
		{
			name:    "windows path stack",
			fixture: "runtime_panics/column_optional.txt",
			expected: []Diagnostic{
				{
					Kind:    KindPanic,
					File:    `C:\Users\tester\project\main.go`,
					Line:    44,
					Column:  1,
					Message: "boom",
				},
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := loadFixture(t, testCase.fixture)
			got := ParseRuntimePanics(fixture)
			if len(got) != len(testCase.expected) {
				t.Fatalf("len(got) = %d, want %d", len(got), len(testCase.expected))
			}
			for i := range testCase.expected {
				assertDiagnosticEqual(t, got[i], testCase.expected[i])
			}
		})
	}
}

func loadFixture(t *testing.T, relativePath string) string {
	t.Helper()
	path := filepath.Join("testdata", relativePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

func assertDiagnosticEqual(t *testing.T, got Diagnostic, want Diagnostic) {
	t.Helper()

	if got.Kind != want.Kind {
		t.Fatalf("Kind = %q, want %q", got.Kind, want.Kind)
	}
	if got.File != want.File {
		t.Fatalf("File = %q, want %q", got.File, want.File)
	}
	if got.Line != want.Line {
		t.Fatalf("Line = %d, want %d", got.Line, want.Line)
	}
	if got.Column != want.Column {
		t.Fatalf("Column = %d, want %d", got.Column, want.Column)
	}
	if got.Message != want.Message {
		t.Fatalf("Message = %q, want %q", got.Message, want.Message)
	}
}
