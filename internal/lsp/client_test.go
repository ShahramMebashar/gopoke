package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"
)

// fakePipe simulates gopls stdin/stdout for testing.
type fakePipe struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func newFakePipe() *fakePipe {
	r, w := io.Pipe()
	return &fakePipe{reader: r, writer: w}
}

func (p *fakePipe) respondTo(id int, result any) {
	data, _ := json.Marshal(result)
	resp := jsonrpcResponse{JSONRPC: "2.0", ID: id, Result: data}
	respData, _ := json.Marshal(resp)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(respData))
	p.writer.Write([]byte(header))
	p.writer.Write(respData)
}

func TestClientCallReturnsResult(t *testing.T) {
	t.Parallel()

	stdinReader, stdinWriter := io.Pipe()
	pipe := newFakePipe()

	client := newClient(stdinWriter, pipe.reader)
	go client.readLoop()

	// Read the request from stdin in background
	go func() {
		scanner := bufio.NewScanner(stdinReader)
		scanner.Split(splitContentLength)
		if scanner.Scan() {
			var req jsonrpcRequest
			json.Unmarshal(scanner.Bytes(), &req)
			pipe.respondTo(req.ID, map[string]bool{"ready": true})
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := client.call(ctx, "test/method", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("call() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}
	if parsed["ready"] != true {
		t.Fatalf("result ready = %v, want true", parsed["ready"])
	}
}

func TestClientCallTimeout(t *testing.T) {
	t.Parallel()

	stdinReader, stdinWriter := io.Pipe()
	pipe := newFakePipe()

	// Drain stdin so writes don't block
	go func() {
		io.Copy(io.Discard, stdinReader)
	}()

	client := newClient(stdinWriter, pipe.reader)
	go client.readLoop()

	// Don't send any response — let the context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.call(ctx, "test/timeout", nil)
	if err == nil {
		t.Fatal("call() error = nil, want timeout error")
	}
	pipe.writer.Close()
}
