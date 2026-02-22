package lsp

import (
	"encoding/json"
	"testing"
)

func TestStatusResultJSON(t *testing.T) {
	t.Parallel()
	s := StatusResult{Ready: true, Error: "some error"}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got StatusResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != s {
		t.Fatalf("round-trip: got %+v, want %+v", got, s)
	}

	// Verify JSON field names
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["ready"]; !ok {
		t.Fatal("missing JSON key 'ready'")
	}
	if _, ok := m["error"]; !ok {
		t.Fatal("missing JSON key 'error'")
	}
}

func TestWorkspaceInfoJSON(t *testing.T) {
	t.Parallel()
	w := WorkspaceInfo{Dir: "/tmp/ws", SnippetURI: "file:///tmp/ws/main.go"}
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got WorkspaceInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != w {
		t.Fatalf("round-trip: got %+v, want %+v", got, w)
	}

	// Verify JSON field names (especially camelCase snippetUri)
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["dir"]; !ok {
		t.Fatal("missing JSON key 'dir'")
	}
	if _, ok := m["snippetUri"]; !ok {
		t.Fatal("missing JSON key 'snippetUri'")
	}
}
