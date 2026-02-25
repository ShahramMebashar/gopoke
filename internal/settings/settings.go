package settings

// GlobalSettings stores app-wide configuration persisted across sessions.
type GlobalSettings struct {
	GoPath             string `json:"goPath"`             // Path to go binary (e.g. /usr/local/go/bin/go). Empty = auto-detect.
	GoplsPath          string `json:"goplsPath"`          // Path to gopls binary. Empty = auto-detect.
	StaticcheckPath    string `json:"staticcheckPath"`    // Path to staticcheck binary. Empty = auto-detect.
	DefaultTimeoutMS   int64  `json:"defaultTimeoutMS"`
	MaxOutputBytes     int64  `json:"maxOutputBytes"`
	GoPathOverride     string `json:"goPathOverride"`
	GoModCacheOverride string `json:"goModCacheOverride"`
	EditorTheme        string `json:"editorTheme"`
	EditorFontFamily   string `json:"editorFontFamily"`
	EditorFontSize     int    `json:"editorFontSize"`
	EditorLineNumbers  bool   `json:"editorLineNumbers"`
}

const (
	DefaultTimeoutMS   = int64(30000)
	DefaultMaxOutput   = int64(1_048_576)
	DefaultFontFamily  = "JetBrains Mono"
	DefaultFontSize    = 14
	DefaultTheme       = "Default Dark Modern"
)

// Defaults returns GlobalSettings with sensible defaults.
func Defaults() GlobalSettings {
	return GlobalSettings{
		DefaultTimeoutMS:  DefaultTimeoutMS,
		MaxOutputBytes:    DefaultMaxOutput,
		EditorTheme:       DefaultTheme,
		EditorFontFamily:  DefaultFontFamily,
		EditorFontSize:    DefaultFontSize,
		EditorLineNumbers: true,
	}
}

// WithDefaults fills zero-value fields with defaults.
func WithDefaults(s GlobalSettings) GlobalSettings {
	d := Defaults()
	if s.DefaultTimeoutMS <= 0 {
		s.DefaultTimeoutMS = d.DefaultTimeoutMS
	}
	if s.MaxOutputBytes <= 0 {
		s.MaxOutputBytes = d.MaxOutputBytes
	}
	if s.EditorTheme == "" {
		s.EditorTheme = d.EditorTheme
	}
	if s.EditorFontFamily == "" {
		s.EditorFontFamily = d.EditorFontFamily
	}
	if s.EditorFontSize <= 0 {
		s.EditorFontSize = d.EditorFontSize
	}
	// EditorLineNumbers: bool defaults to false, but our default is true.
	// We can't distinguish "user set false" from "zero value" without a pointer.
	// So we only apply default on fresh/empty settings (all fields zero).
	return s
}

// Validate checks settings constraints and clamps values.
func Validate(s GlobalSettings) GlobalSettings {
	if s.DefaultTimeoutMS < 1000 {
		s.DefaultTimeoutMS = 1000
	}
	if s.DefaultTimeoutMS > 300000 {
		s.DefaultTimeoutMS = 300000
	}
	if s.MaxOutputBytes < 1024 {
		s.MaxOutputBytes = 1024
	}
	if s.MaxOutputBytes > 10_485_760 {
		s.MaxOutputBytes = 10_485_760
	}
	if s.EditorFontSize < 10 {
		s.EditorFontSize = 10
	}
	if s.EditorFontSize > 24 {
		s.EditorFontSize = 24
	}
	return s
}
