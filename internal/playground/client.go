package playground

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	shareEndpoint = "https://go.dev/_/share"
	fetchEndpoint = "https://go.dev/play/p/"
)

const maxSourceBytes = 64 * 1024

// ShareResult contains the playground URL after a successful share.
type ShareResult struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
}

// Share uploads source to the Go Playground and returns the URL.
func Share(ctx context.Context, source string) (ShareResult, error) {
	if len(source) > maxSourceBytes {
		return ShareResult{}, fmt.Errorf("source exceeds %d byte limit", maxSourceBytes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, shareEndpoint, strings.NewReader(source))
	if err != nil {
		return ShareResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ShareResult{}, fmt.Errorf("share request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ShareResult{}, fmt.Errorf("share failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return ShareResult{}, fmt.Errorf("read response: %w", err)
	}
	hash := strings.TrimSpace(string(body))
	if hash == "" {
		return ShareResult{}, fmt.Errorf("empty hash in response")
	}
	return ShareResult{
		URL:  "https://go.dev/play/p/" + hash,
		Hash: hash,
	}, nil
}

// Import fetches source code from a Go Playground URL or hash.
func Import(ctx context.Context, urlOrHash string) (string, error) {
	hash := extractHash(urlOrHash)
	if hash == "" {
		return "", fmt.Errorf("invalid playground URL or hash: %q", urlOrHash)
	}
	fetchURL := fetchEndpoint + hash + ".go"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("playground snippet not found: %s", hash)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBytes+1))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	return string(body), nil
}

// extractHash parses a playground URL or raw hash into just the hash.
// Accepts: "abc123", "https://go.dev/play/p/abc123", "https://play.golang.org/p/abc123"
func extractHash(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	for _, prefix := range []string{
		"https://go.dev/play/p/",
		"http://go.dev/play/p/",
		"https://play.golang.org/p/",
		"http://play.golang.org/p/",
	} {
		if after, found := strings.CutPrefix(input, prefix); found {
			hash := strings.TrimSuffix(after, ".go")
			if hash != "" {
				return hash
			}
		}
	}
	for _, c := range input {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return ""
		}
	}
	return input
}
