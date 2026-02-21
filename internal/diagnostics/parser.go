package diagnostics

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

const (
	// KindCompile indicates a compiler diagnostic.
	KindCompile = "compile"
	// KindPanic indicates a runtime panic diagnostic.
	KindPanic = "panic"
)

var (
	compilePattern = regexp.MustCompile(`^((?:[A-Za-z]:)?[^:\n]+\.go):([0-9]+):([0-9]+):\s*(.+)$`)
	panicFrame     = regexp.MustCompile(`^\s*((?:[A-Za-z]:)?[^:\n]+\.go):([0-9]+)(?::([0-9]+))?\s*(?:\+0x[0-9a-fA-F]+)?\s*$`)
)

// Diagnostic describes one actionable location from run output.
type Diagnostic struct {
	Kind    string
	File    string
	Line    int
	Column  int
	Message string
	Raw     string
}

// ParseCompileErrors extracts compile diagnostics from stderr output.
func ParseCompileErrors(stderr string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	scanner := bufio.NewScanner(strings.NewReader(stderr))
	for scanner.Scan() {
		line := scanner.Text()
		matches := compilePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) != 5 {
			continue
		}
		lineNumber, err := strconv.Atoi(matches[2])
		if err != nil || lineNumber <= 0 {
			continue
		}
		columnNumber, err := strconv.Atoi(matches[3])
		if err != nil || columnNumber <= 0 {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    KindCompile,
			File:    matches[1],
			Line:    lineNumber,
			Column:  columnNumber,
			Message: strings.TrimSpace(matches[4]),
			Raw:     line,
		})
	}
	return diagnostics
}

// ParseRuntimePanics extracts runtime panic stack frame diagnostics.
func ParseRuntimePanics(stderr string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	scanner := bufio.NewScanner(strings.NewReader(stderr))
	pendingMessage := ""
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "panic:") {
			pendingMessage = strings.TrimSpace(strings.TrimPrefix(trimmed, "panic:"))
			continue
		}
		matches := panicFrame.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}
		lineNumber, err := strconv.Atoi(matches[2])
		if err != nil || lineNumber <= 0 {
			continue
		}
		columnNumber := 1
		if matches[3] != "" {
			parsedColumn, colErr := strconv.Atoi(matches[3])
			if colErr == nil && parsedColumn > 0 {
				columnNumber = parsedColumn
			}
		}
		message := "runtime panic"
		if pendingMessage != "" {
			message = pendingMessage
		}
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    KindPanic,
			File:    matches[1],
			Line:    lineNumber,
			Column:  columnNumber,
			Message: message,
			Raw:     line,
		})
	}
	return diagnostics
}

// ParseAll parses compile and runtime diagnostics from one stderr payload.
func ParseAll(stderr string) []Diagnostic {
	compile := ParseCompileErrors(stderr)
	panicFrames := ParseRuntimePanics(stderr)
	result := make([]Diagnostic, 0, len(compile)+len(panicFrames))
	result = append(result, compile...)
	result = append(result, panicFrames...)
	return result
}
