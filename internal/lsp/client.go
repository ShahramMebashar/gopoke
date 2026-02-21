package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// NotificationHandler processes server-initiated notifications.
type NotificationHandler func(method string, params json.RawMessage)

type client struct {
	stdin  io.Writer
	stdout io.Reader

	mu       sync.Mutex
	nextID   atomic.Int64
	pending  map[int]chan jsonrpcResponse
	onNotify NotificationHandler

	writeMu sync.Mutex
}

func newClient(stdin io.Writer, stdout io.Reader) *client {
	return &client{
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[int]chan jsonrpcResponse),
	}
}

func (c *client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))

	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		rawParams = data
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	ch := make(chan jsonrpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.writeMessage(req); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("lsp request %q: %w", method, ctx.Err())
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (c *client) notify(method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		rawParams = data
	}

	notif := jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
	}
	return c.writeMessage(notif)
}

func (c *client) writeMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if _, err := io.WriteString(c.stdin, header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}

func (c *client) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	scanner.Split(splitContentLength)

	for scanner.Scan() {
		data := scanner.Bytes()
		c.handleMessage(data)
	}
}

func (c *client) handleMessage(data []byte) {
	// Try as response (has "id" and "result" or "error")
	var resp jsonrpcResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.ID > 0 {
		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
		return
	}

	// Try as notification (has "method" but no "id")
	var notif jsonrpcNotification
	if err := json.Unmarshal(data, &notif); err == nil && notif.Method != "" {
		if c.onNotify != nil {
			c.onNotify(notif.Method, notif.Params)
		}
	}
}

// splitContentLength is a bufio.SplitFunc for LSP Content-Length framing.
func splitContentLength(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	headerEnd := strings.Index(string(data), "\r\n\r\n")
	if headerEnd < 0 {
		if atEOF {
			return 0, nil, fmt.Errorf("incomplete LSP header")
		}
		return 0, nil, nil // need more data
	}

	header := string(data[:headerEnd])
	contentLength := 0
	for _, line := range strings.Split(header, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			val := strings.TrimSpace(line[len("content-length:"):])
			contentLength, err = strconv.Atoi(val)
			if err != nil {
				return 0, nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
		}
	}
	if contentLength <= 0 {
		return 0, nil, fmt.Errorf("missing Content-Length header")
	}

	totalLen := headerEnd + 4 + contentLength
	if len(data) < totalLen {
		if atEOF {
			return 0, nil, fmt.Errorf("incomplete LSP body")
		}
		return 0, nil, nil // need more data
	}

	body := data[headerEnd+4 : totalLen]
	return totalLen, body, nil
}
