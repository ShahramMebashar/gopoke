package settings

import "testing"

func TestDefaults(t *testing.T) {
	t.Parallel()
	d := Defaults()
	if d.DefaultTimeoutMS != DefaultTimeoutMS {
		t.Fatalf("timeout = %d, want %d", d.DefaultTimeoutMS, DefaultTimeoutMS)
	}
	if d.MaxOutputBytes != DefaultMaxOutput {
		t.Fatalf("maxOutput = %d, want %d", d.MaxOutputBytes, DefaultMaxOutput)
	}
	if d.EditorTheme != "Default Dark Modern" {
		t.Fatalf("theme = %q, want %q", d.EditorTheme, "Default Dark Modern")
	}
	if d.EditorFontFamily != DefaultFontFamily {
		t.Fatalf("font = %q, want %q", d.EditorFontFamily, DefaultFontFamily)
	}
	if d.EditorFontSize != DefaultFontSize {
		t.Fatalf("fontSize = %d, want %d", d.EditorFontSize, DefaultFontSize)
	}
	if !d.EditorLineNumbers {
		t.Fatal("lineNumbers = false, want true")
	}
}

func TestWithDefaultsFillsZeroValues(t *testing.T) {
	t.Parallel()
	s := WithDefaults(GlobalSettings{})
	if s.DefaultTimeoutMS != DefaultTimeoutMS {
		t.Fatalf("timeout = %d, want %d", s.DefaultTimeoutMS, DefaultTimeoutMS)
	}
	if s.EditorTheme != "Default Dark Modern" {
		t.Fatalf("theme = %q, want %q", s.EditorTheme, "Default Dark Modern")
	}
}

func TestWithDefaultsPreservesUserValues(t *testing.T) {
	t.Parallel()
	s := WithDefaults(GlobalSettings{
		DefaultTimeoutMS: 5000,
		EditorTheme:      "monokai",
		EditorFontFamily: "Fira Code",
		EditorFontSize:   16,
	})
	if s.DefaultTimeoutMS != 5000 {
		t.Fatalf("timeout = %d, want 5000", s.DefaultTimeoutMS)
	}
	if s.EditorTheme != "monokai" {
		t.Fatalf("theme = %q, want monokai", s.EditorTheme)
	}
	if s.EditorFontFamily != "Fira Code" {
		t.Fatalf("font = %q, want Fira Code", s.EditorFontFamily)
	}
	if s.EditorFontSize != 16 {
		t.Fatalf("fontSize = %d, want 16", s.EditorFontSize)
	}
}

func TestValidateClampsValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input GlobalSettings
		check func(t *testing.T, s GlobalSettings)
	}{
		{
			name:  "timeout too low",
			input: GlobalSettings{DefaultTimeoutMS: 100},
			check: func(t *testing.T, s GlobalSettings) {
				if s.DefaultTimeoutMS != 1000 {
					t.Fatalf("timeout = %d, want 1000", s.DefaultTimeoutMS)
				}
			},
		},
		{
			name:  "timeout too high",
			input: GlobalSettings{DefaultTimeoutMS: 999999},
			check: func(t *testing.T, s GlobalSettings) {
				if s.DefaultTimeoutMS != 300000 {
					t.Fatalf("timeout = %d, want 300000", s.DefaultTimeoutMS)
				}
			},
		},
		{
			name:  "font size too small",
			input: GlobalSettings{EditorFontSize: 5},
			check: func(t *testing.T, s GlobalSettings) {
				if s.EditorFontSize != 10 {
					t.Fatalf("fontSize = %d, want 10", s.EditorFontSize)
				}
			},
		},
		{
			name:  "font size too large",
			input: GlobalSettings{EditorFontSize: 30},
			check: func(t *testing.T, s GlobalSettings) {
				if s.EditorFontSize != 24 {
					t.Fatalf("fontSize = %d, want 24", s.EditorFontSize)
				}
			},
		},
		{
			name:  "max output too small",
			input: GlobalSettings{MaxOutputBytes: 100},
			check: func(t *testing.T, s GlobalSettings) {
				if s.MaxOutputBytes != 1024 {
					t.Fatalf("maxOutput = %d, want 1024", s.MaxOutputBytes)
				}
			},
		},
		{
			name:  "max output too large",
			input: GlobalSettings{MaxOutputBytes: 99_999_999},
			check: func(t *testing.T, s GlobalSettings) {
				if s.MaxOutputBytes != 10_485_760 {
					t.Fatalf("maxOutput = %d, want 10485760", s.MaxOutputBytes)
				}
			},
		},
		{
			name: "valid values unchanged",
			input: GlobalSettings{
				DefaultTimeoutMS: 5000,
				MaxOutputBytes:   1_000_000,
				EditorFontSize:   14,
			},
			check: func(t *testing.T, s GlobalSettings) {
				if s.DefaultTimeoutMS != 5000 {
					t.Fatalf("timeout = %d, want 5000", s.DefaultTimeoutMS)
				}
				if s.MaxOutputBytes != 1_000_000 {
					t.Fatalf("maxOutput = %d, want 1000000", s.MaxOutputBytes)
				}
				if s.EditorFontSize != 14 {
					t.Fatalf("fontSize = %d, want 14", s.EditorFontSize)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := Validate(tt.input)
			tt.check(t, result)
		})
	}
}
