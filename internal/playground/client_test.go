package playground

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShare(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("abc123"))
	}))
	defer server.Close()

	origEndpoint := shareEndpoint
	shareEndpoint = server.URL
	defer func() { shareEndpoint = origEndpoint }()

	result, err := Share(context.Background(), "package main\n")
	if err != nil {
		t.Fatalf("Share() error = %v", err)
	}
	if result.Hash != "abc123" {
		t.Fatalf("hash = %q, want %q", result.Hash, "abc123")
	}
	if !strings.Contains(result.URL, "abc123") {
		t.Fatalf("URL = %q, want it to contain hash", result.URL)
	}
}

func TestImport(t *testing.T) {
	t.Parallel()

	want := "package main\n\nfunc main() {}\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(want))
	}))
	defer server.Close()

	origEndpoint := fetchEndpoint
	fetchEndpoint = server.URL + "/"
	defer func() { fetchEndpoint = origEndpoint }()

	got, err := Import(context.Background(), "xyz789")
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}

func TestExtractHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"abc123", "abc123"},
		{"https://go.dev/play/p/abc123", "abc123"},
		{"http://go.dev/play/p/abc123", "abc123"},
		{"https://play.golang.org/p/abc123", "abc123"},
		{"http://play.golang.org/p/abc123", "abc123"},
		{"https://go.dev/play/p/abc123.go", "abc123"},
		{"a-b_c", "a-b_c"},
		{"", ""},
		{"   ", ""},
		{"not a valid url!", ""},
		{"https://example.com/abc", ""},
	}

	for _, tc := range tests {
		got := extractHash(tc.input)
		if got != tc.want {
			t.Errorf("extractHash(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestShareSizeLimit(t *testing.T) {
	t.Parallel()

	bigSource := strings.Repeat("a", maxSourceBytes+1)
	_, err := Share(context.Background(), bigSource)
	if err == nil {
		t.Fatal("Share() error = nil, want size limit error")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("error = %q, want it to mention byte limit", err.Error())
	}
}

func TestImportNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	origEndpoint := fetchEndpoint
	fetchEndpoint = server.URL + "/"
	defer func() { fetchEndpoint = origEndpoint }()

	_, err := Import(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Import() error = nil, want not found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want it to mention not found", err.Error())
	}
}
