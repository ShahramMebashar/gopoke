package lsp

import "encoding/json"

// --- JSON-RPC 2.0 wire types ---

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonrpcError) Error() string {
	return e.Message
}

// jsonrpcNotification is a request without an id (server→client or client→server).
type jsonrpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// --- LSP position types ---

// Position is a zero-based line/character offset (LSP spec).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a start/end position pair.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a URI + range.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a document by URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams is used for completion, hover, definition.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// --- Completion types ---

// CompletionItem is a simplified completion result for the frontend.
type CompletionItem struct {
	Label      string `json:"label"`
	Detail     string `json:"detail"`
	Kind       string `json:"kind"`
	InsertText string `json:"insertText"`
	SortText   string `json:"sortText"`
}

// lspCompletionItem is the raw LSP completion item from gopls.
type lspCompletionItem struct {
	Label            string `json:"label"`
	Kind             int    `json:"kind"`
	Detail           string `json:"detail"`
	InsertText       string `json:"insertText"`
	InsertTextFormat int    `json:"insertTextFormat"`
	SortText         string `json:"sortText"`
}

// lspCompletionList is the raw LSP completion response.
type lspCompletionList struct {
	IsIncomplete bool                `json:"isIncomplete"`
	Items        []lspCompletionItem `json:"items"`
}

func completionItemKindString(kind int) string {
	switch kind {
	case 1:
		return "text"
	case 2:
		return "method"
	case 3:
		return "function"
	case 4:
		return "constructor"
	case 5:
		return "field"
	case 6:
		return "variable"
	case 7:
		return "class"
	case 8:
		return "interface"
	case 9:
		return "module"
	case 10:
		return "property"
	case 13:
		return "enum"
	case 14:
		return "keyword"
	case 15:
		return "snippet"
	case 21:
		return "constant"
	case 22:
		return "struct"
	case 23:
		return "event"
	default:
		return "text"
	}
}

// --- Hover types ---

// HoverResult is the simplified hover response for the frontend.
type HoverResult struct {
	Contents string `json:"contents"`
	Range    *Range `json:"range,omitempty"`
}

// lspHoverResult is the raw LSP hover response.
type lspHoverResult struct {
	Contents lspMarkupContent `json:"contents"`
	Range    *Range           `json:"range,omitempty"`
}

type lspMarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// --- Signature help types ---

// SignatureResult is the simplified signature help for the frontend.
type SignatureResult struct {
	Label           string   `json:"label"`
	Parameters      []string `json:"parameters"`
	ActiveParameter int      `json:"activeParameter"`
}

// lspSignatureHelp is the raw LSP response.
type lspSignatureHelp struct {
	Signatures      []lspSignatureInfo `json:"signatures"`
	ActiveSignature int                `json:"activeSignature"`
	ActiveParameter int                `json:"activeParameter"`
}

type lspSignatureInfo struct {
	Label      string             `json:"label"`
	Parameters []lspParameterInfo `json:"parameters"`
}

type lspParameterInfo struct {
	Label string `json:"label"`
}

// --- Diagnostic types ---

// DiagnosticEvent is pushed to the frontend via Wails events.
type DiagnosticEvent struct {
	Diagnostics []LspDiagnostic `json:"diagnostics"`
}

// LspDiagnostic is one diagnostic for the frontend.
type LspDiagnostic struct {
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"endLine"`
	EndColumn int    `json:"endColumn"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

// lspPublishDiagnosticsParams is the raw notification from gopls.
type lspPublishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

type lspDiagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
}

func diagnosticSeverityString(severity int) string {
	switch severity {
	case 1:
		return "error"
	case 2:
		return "warning"
	case 3:
		return "info"
	case 4:
		return "hint"
	default:
		return "error"
	}
}

// --- didOpen / didChange / didClose ---

type didOpenTextDocumentParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didChangeTextDocumentParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []textDocumentContentChange     `json:"contentChanges"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type textDocumentContentChange struct {
	Text string `json:"text"`
}

type didCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// --- LSP Status ---

// StatusResult is the LSP status for the frontend.
type StatusResult struct {
	Ready bool   `json:"ready"`
	Error string `json:"error"`
}
