package lsp

// StatusResult is the LSP status for the frontend.
type StatusResult struct {
	Ready bool   `json:"ready"`
	Error string `json:"error"`
}

// WorkspaceInfo describes the LSP workspace for the frontend.
type WorkspaceInfo struct {
	Dir        string `json:"dir"`
	SnippetURI string `json:"snippetUri"`
}
