package richoutput

import (
	"encoding/json"
	"strings"
)

const markerPrefix = "//gopad:"

// Parse scans stdout line-by-line for //gopad:<type> <json> markers.
// It returns clean stdout (markers stripped) and extracted rich blocks.
// Malformed markers (bad JSON or missing payload) are kept in clean output.
func Parse(stdout string) (cleanStdout string, blocks []RichBlock) {
	if stdout == "" {
		return "", nil
	}

	lines := strings.Split(stdout, "\n")
	clean := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, markerPrefix) {
			clean = append(clean, line)
			continue
		}

		rest := trimmed[len(markerPrefix):]
		spaceIdx := strings.IndexByte(rest, ' ')
		if spaceIdx < 1 {
			// No type or no payload — keep line in clean output
			clean = append(clean, line)
			continue
		}

		blockType := rest[:spaceIdx]
		payload := strings.TrimSpace(rest[spaceIdx+1:])
		if payload == "" {
			clean = append(clean, line)
			continue
		}

		if !json.Valid([]byte(payload)) {
			clean = append(clean, line)
			continue
		}

		blocks = append(blocks, RichBlock{
			Type: blockType,
			Data: json.RawMessage(payload),
		})
	}

	return strings.Join(clean, "\n"), blocks
}
