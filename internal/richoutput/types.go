package richoutput

import "encoding/json"

// Block type constants for known rich output renderers.
const (
	TypeTable = "table"
	TypeJSON  = "json"
)

// RichBlock holds one parsed rich output block extracted from program stdout.
type RichBlock struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}
