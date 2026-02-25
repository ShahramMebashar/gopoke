# GoPoke

A desktop app for running and iterating on Go snippets quickly — **Tinkerwell for Go**.

Write Go code against a real local project, run it instantly, and see output in real time. No terminal juggling, no scratch file management.

## Why GoPoke?

Go developers often use ad-hoc scratch files, shell commands, and editor hacks to test ideas. GoPoke eliminates that friction:

- **Project-aware** — snippets run in the context of your actual Go module with full access to your dependencies
- **Instant feedback** — streaming output appears as your code runs, not after it finishes
- **Smart editor** — Monaco editor with gopls-powered autocompletion, hover docs, and go-to-definition
- **Zero config** — open a folder and start writing

## Features

### Editor

- Monaco editor with full VS Code API surface
- **gopls integration** via LSP-over-WebSocket — autocompletion, hover, go-to-definition, quick fixes
- **14 built-in themes** — Dark (GitHub Dark, One Dark Pro, Dracula, Monokai, Material Darker, Nord, Catppuccin Mocha, Solarized Dark), Light (GitHub Light, Solarized Light), High Contrast
- Configurable font family (JetBrains Mono, SF Mono, Menlo, Fira Code, Source Code Pro, Cascadia Code)
- Font size 10–24px, toggleable line numbers
- Format on save via gopls (`goimports` + `gofmt`)

### Snippet Execution

- Run snippets with **Cmd+Enter** — output streams in real time
- **15-second default timeout** (configurable per run)
- **Graceful cancellation** — SIGINT → 400ms grace → force kill
- Run states: idle, running, success, failed, canceled, timed out
- **128 KB output cap** per stream (truncation flagged)
- **Warm worker process** — keeps one subprocess per project alive to maintain build cache. Cold start ~120ms for first output

### Snippet Library

- Create, save, rename, duplicate, delete snippets per project
- Search snippets by name or content
- Sorted by most recently updated
- Content-hash-based caching — unchanged snippets skip file writes

### Diagnostics

- **Compile errors** parsed and mapped to file:line:column
- **Runtime panics** detected with stack frame extraction
- Click a diagnostic to jump to the line in the editor

### Rich Output

Snippets can emit structured output using the `//gopoke:` protocol:

```go
fmt.Println(`//gopoke:table [{"name":"Alice","age":30},{"name":"Bob","age":25}]`)
fmt.Println(`//gopoke:json {"status":"ok","count":42}`)
```

- `//gopoke:table` — renders as an HTML table
- `//gopoke:json` — renders as a key-value card with type-colored values
- Raw tab always available alongside rich output

### Project Management

- Open local Go projects via native OS directory picker
- Auto-detects `go.mod` and module name
- Discovers runnable package targets via `go list`
- **Run target selector** — choose which `main` package to execute against
- **Working directory selector** — run from project root or any discovered package directory
- **Go toolchain selector** — auto-discovers all `go*` binaries in PATH (e.g., `go`, `go1.22`, `go1.23`)
- **Recent projects** — last 12 opened projects, one click to reopen

### Single File Mode

- Open individual `.go` files via native file picker
- **Cmd+S** saves back to disk
- Automatically opens parent directory as project context

### Go Playground Integration

- **Share** — upload snippet to [go.dev/play](https://go.dev/play), URL copied to clipboard
- **Import** — paste a playground URL or hash to load a snippet
- 64 KB source limit enforced before upload

### Environment Variables

- Per-project env var management with add/edit/delete
- **Masked values** — secrets shown as `********` with reveal toggle
- **`.env` auto-import** — reads `.env` from project root on open
- Env vars injected into snippet process at runtime

### Scratch Mode

- Works without any project open
- Auto-creates a temporary module for immediate use
- LSP starts at app boot — completions available before opening a project

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Cmd+Enter | Run snippet |
| Cmd+S | Save file |
| Cmd+B | Toggle sidebar |
| Cmd+, | Settings |
| Cmd+1 | Snippets tab |
| Cmd+2 | Env tab |
| Cmd+3 | Project tab |
| Cmd+4 | Recent tab |

### Toolbar

- **macOS** — native NSToolbar with: Toggle Sidebar, Open Folder, Open File, New Snippet, Format, Run/Stop, Rerun Last, Share, Import, Settings
- **Linux/Windows** — HTML toolbar with the same actions

## Tech Stack

| Layer | Technology |
|-------|------------|
| Desktop shell | [Wails v2](https://wails.io) |
| Backend | Go 1.25 |
| Frontend | React 18, Vite 5 |
| Editor | Monaco via `@codingame/monaco-vscode-api` |
| LSP | `gopls` (external, found via PATH) |
| State | Local JSON file (atomic writes, in-memory cache) |
| macOS integration | CGo + Objective-C (NSToolbar) |

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 18+
- `gopls` installed (`go install golang.org/x/tools/gopls@latest`)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) (for development)

### Build

```bash
# Install frontend dependencies
make frontend-install

# Build the desktop binary
make build

# Run the app
./gopoke
```

### Development

```bash
# Start frontend dev server
make frontend-dev

# Run with Wails tags (in another terminal)
make run

# Run tests
make test

# Full verification (fmt + vet + tests)
make check
```

### All Make Targets

```
frontend-install  Install frontend dependencies
frontend-build    Build frontend production assets
frontend-dev      Start frontend dev server (Vite)
frontend-clean    Remove generated frontend dist assets
fmt               Format all Go files
fmt-check         Fail if Go files are not formatted
test              Run Go tests
test-wails        Run tests for Wails-tagged app package
vet               Run go vet
run               Run desktop app
build             Build desktop binary
check             Run local verification checks
bench-nfr         Run NFR benchmark suite
stress-run-cancel Run run/cancel reliability stress suite
release-macos     Build/package macOS app bundle + zip
clean             Remove built desktop binary
clean-cache       Remove local Go build cache
```

## Architecture

```
cmd/gopoke/          Entry point, Wails app setup, macOS native toolbar
internal/
  app/               Dependency wiring, business logic orchestration
  desktop/           Wails bridge — exposes all methods to frontend via RPC
  execution/         go run process management, output streaming, timeout/cancel
  runner/            Long-lived worker process lifecycle (warm builds)
  lsp/               WebSocket-to-gopls proxy + workspace isolation
  project/           Project open, module detection, run target discovery
  storage/           Local JSON state persistence (atomic writes)
  richoutput/        Marker-based rich output parser (//gopoke: protocol)
  diagnostics/       Compile error + runtime panic parser
  formatting/        gofmt wrapper
  env/               Per-project environment variable service
  playground/        Go Playground share/import client
  telemetry/         Startup timing recorder
```

## Performance

Measured on Apple M3 Max:

| Metric | Target | Actual |
|--------|--------|--------|
| App startup | ≤ 2000ms | **0.37ms** |
| Warm run trigger | ≤ 200ms | **0.004ms** |
| First output | ≤ 500ms | **119.8ms** |

## License

All rights reserved.
