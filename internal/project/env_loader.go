package project

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func loadDotEnvFile(projectPath string) (map[string]string, []string, error) {
	dotEnvPath := filepath.Join(projectPath, ".env")
	raw, err := os.ReadFile(dotEnvPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil, nil
		}
		return nil, nil, fmt.Errorf("read %s: %w", dotEnvPath, err)
	}

	values, warnings := parseDotEnv(string(raw))
	return values, warnings, nil
}

func parseDotEnv(content string) (map[string]string, []string) {
	values := make(map[string]string)
	warnings := make([]string, 0)

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "export ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
		}

		separator := strings.Index(trimmed, "=")
		if separator <= 0 {
			warnings = append(warnings, fmt.Sprintf("line %d: expected KEY=VALUE assignment", lineNumber))
			continue
		}

		key := strings.TrimSpace(trimmed[:separator])
		if !envKeyPattern.MatchString(key) {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid key %q", lineNumber, key))
			continue
		}

		valueRaw := strings.TrimSpace(trimmed[separator+1:])
		value, ok := parseDotEnvValue(valueRaw)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid quoted value", lineNumber))
			continue
		}
		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		warnings = append(warnings, fmt.Sprintf("scan error: %v", err))
	}

	return values, warnings
}

func parseDotEnvValue(raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	if raw[0] == '\'' || raw[0] == '"' {
		quote := raw[0]
		if len(raw) < 2 || raw[len(raw)-1] != quote {
			return "", false
		}
		if quote == '\'' {
			return raw[1 : len(raw)-1], true
		}
		unquoted, err := strconv.Unquote(raw)
		if err != nil {
			return "", false
		}
		return unquoted, true
	}

	for i := 0; i < len(raw); i++ {
		if raw[i] != '#' {
			continue
		}
		if i == 0 {
			return "", true
		}
		if raw[i-1] == ' ' || raw[i-1] == '\t' {
			return strings.TrimSpace(raw[:i]), true
		}
	}
	return strings.TrimSpace(raw), true
}
