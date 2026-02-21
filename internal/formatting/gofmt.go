package formatting

import (
	"fmt"
	"go/format"
	"strings"
)

// GoSource applies gofmt formatting to a snippet that represents a Go file.
func GoSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("source is required")
	}
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return "", fmt.Errorf("gofmt source: %w", err)
	}
	return string(formatted), nil
}
