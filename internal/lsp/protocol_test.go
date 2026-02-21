package lsp

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCRequestMarshal(t *testing.T) {
	t.Parallel()
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"capabilities":{}}`),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got, want := parsed["jsonrpc"], "2.0"; got != want {
		t.Fatalf("jsonrpc = %v, want %v", got, want)
	}
	if got, want := parsed["method"], "initialize"; got != want {
		t.Fatalf("method = %v, want %v", got, want)
	}
}

func TestJSONRPCResponseUnmarshal(t *testing.T) {
	t.Parallel()
	raw := `{"jsonrpc":"2.0","id":1,"result":{"capabilities":{}}}`
	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got, want := resp.ID, 1; got != want {
		t.Fatalf("ID = %d, want %d", got, want)
	}
	if resp.Error != nil {
		t.Fatalf("Error = %v, want nil", resp.Error)
	}
}

func TestJSONRPCNotificationHasNoID(t *testing.T) {
	t.Parallel()
	notif := jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  json.RawMessage(`{}`),
	}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := parsed["id"]; ok {
		t.Fatal("notification should not have id field")
	}
}

func TestCompletionItemKindToString(t *testing.T) {
	t.Parallel()
	if got := completionItemKindString(1); got != "text" {
		t.Fatalf("kind 1 = %q, want %q", got, "text")
	}
	if got := completionItemKindString(3); got != "function" {
		t.Fatalf("kind 3 = %q, want %q", got, "function")
	}
	if got := completionItemKindString(6); got != "variable" {
		t.Fatalf("kind 6 = %q, want %q", got, "variable")
	}
	if got := completionItemKindString(9); got != "module" {
		t.Fatalf("kind 9 = %q, want %q", got, "module")
	}
	if got := completionItemKindString(999); got != "text" {
		t.Fatalf("kind 999 = %q, want %q", got, "text")
	}
}
