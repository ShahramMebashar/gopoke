package lsp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Proxy bridges a WebSocket connection to a gopls stdio process.
// It forwards raw LSP JSON-RPC messages transparently in both directions.
type Proxy struct {
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	logger   *slog.Logger

	// goplsPath is resolved once when the proxy is created.
	goplsPath    string
	workspaceDir string
}

// wsUpgrader allows all origins because the WebSocket is only exposed on
// localhost and consumed by the Wails webview running on the same machine.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NewProxy creates a WebSocket-to-stdio LSP proxy.
// It binds to localhost:0 (OS-assigned port) but does not start serving yet.
func NewProxy(goplsPath, workspaceDir string, logger *slog.Logger) (*Proxy, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	p := &Proxy{
		listener:     ln,
		logger:       logger,
		goplsPath:    goplsPath,
		workspaceDir: workspaceDir,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/lsp", p.handleWS)
	p.server = &http.Server{Handler: mux}

	return p, nil
}

// Port returns the listening port.
func (p *Proxy) Port() int {
	return p.listener.Addr().(*net.TCPAddr).Port
}

// Serve starts accepting WebSocket connections. Blocks until Shutdown is called.
func (p *Proxy) Serve() error {
	err := p.server.Serve(p.listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the proxy server.
func (p *Proxy) Shutdown(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}

func (p *Proxy) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		p.logger.Warn("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx, p.goplsPath, "serve")
	cmd.Dir = p.workspaceDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		p.logger.Warn("create stdin pipe", "error", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.logger.Warn("create stdout pipe", "error", err)
		return
	}
	if err := cmd.Start(); err != nil {
		p.logger.Warn("start gopls", "error", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// WS → gopls stdin: read WebSocket messages, wrap with Content-Length, write to stdin
	go func() {
		defer wg.Done()
		defer stdin.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(msg))
			if _, err := io.WriteString(stdin, header); err != nil {
				cancel()
				return
			}
			if _, err := stdin.Write(msg); err != nil {
				cancel()
				return
			}
		}
	}()

	// gopls stdout → WS: scan Content-Length framed messages, write as WebSocket messages
	go func() {
		defer wg.Done()
		defer cancel()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
		scanner.Split(splitContentLength)

		for scanner.Scan() {
			data := scanner.Bytes()
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			p.logger.Debug("gopls stdout scanner", "error", err)
		}
	}()

	wg.Wait()
	_ = cmd.Wait()
}

// splitContentLength is a bufio.SplitFunc for LSP Content-Length framing.
func splitContentLength(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
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

// findGoplsBinary locates gopls in PATH.
func findGoplsBinary() string {
	path, err := exec.LookPath("gopls")
	if err != nil {
		return ""
	}
	return path
}
