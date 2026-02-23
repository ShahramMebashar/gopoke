package richoutput

import (
	"encoding/json"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantClean  string
		wantBlocks int
		wantTypes  []string
		nilBlocks  bool // expect nil (not empty) blocks slice
	}{
		{
			name:       "empty input",
			input:      "",
			wantClean:  "",
			wantBlocks: 0,
			nilBlocks:  true,
		},
		{
			name:       "no markers",
			input:      "hello\nworld\n",
			wantClean:  "hello\nworld\n",
			wantBlocks: 0,
		},
		{
			name:       "single table marker",
			input:      "before\n//gopad:table [{\"a\":1,\"b\":2}]\nafter",
			wantClean:  "before\nafter",
			wantBlocks: 1,
			wantTypes:  []string{TypeTable},
		},
		{
			name:       "single json marker",
			input:      `//gopad:json {"key":"value"}`,
			wantClean:  "",
			wantBlocks: 1,
			wantTypes:  []string{TypeJSON},
		},
		{
			name:       "multiple markers",
			input:      "line1\n//gopad:table [{\"x\":1}]\nline2\n//gopad:json {\"y\":2}\nline3",
			wantClean:  "line1\nline2\nline3",
			wantBlocks: 2,
			wantTypes:  []string{TypeTable, TypeJSON},
		},
		{
			name:       "malformed JSON kept in clean output",
			input:      "ok\n//gopad:table {bad json}\nstill ok",
			wantClean:  "ok\n//gopad:table {bad json}\nstill ok",
			wantBlocks: 0,
		},
		{
			name:       "empty payload kept in clean output",
			input:      "//gopad:table ",
			wantClean:  "//gopad:table ",
			wantBlocks: 0,
		},
		{
			name:       "no space after type kept in clean output",
			input:      "//gopad:table",
			wantClean:  "//gopad:table",
			wantBlocks: 0,
		},
		{
			name:       "unknown type still extracted",
			input:      `//gopad:chart {"labels":["a","b"]}`,
			wantClean:  "",
			wantBlocks: 1,
			wantTypes:  []string{"chart"},
		},
		{
			name:       "leading whitespace",
			input:      "  //gopad:json {\"k\":1}",
			wantClean:  "",
			wantBlocks: 1,
			wantTypes:  []string{TypeJSON},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean, blocks := Parse(tt.input)

			if clean != tt.wantClean {
				t.Errorf("clean:\n  got  %q\n  want %q", clean, tt.wantClean)
			}

			if tt.nilBlocks {
				if blocks != nil {
					t.Errorf("expected nil blocks, got %d", len(blocks))
				}
				return
			}

			if len(blocks) != tt.wantBlocks {
				t.Fatalf("blocks: got %d, want %d", len(blocks), tt.wantBlocks)
			}

			for i, wantType := range tt.wantTypes {
				if blocks[i].Type != wantType {
					t.Errorf("block[%d] type: got %q, want %q", i, blocks[i].Type, wantType)
				}
				if !json.Valid(blocks[i].Data) {
					t.Errorf("block[%d] data is not valid JSON: %s", i, blocks[i].Data)
				}
			}
		})
	}
}
