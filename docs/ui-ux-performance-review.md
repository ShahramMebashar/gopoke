# GoPad UI / UX / Performance Review

## UI

### Architecture

- **1,706-line monolithic `App.jsx`** — 30+ useState hooks, 40+ useCallback handlers, all JSX in one return. Extract components: `Sidebar`, `ProjectTab`, `SnippetsTab`, `EnvTab`, `OutputPane`, `StatusBar`, `SettingsPanel`. Each becomes testable and independently re-renderable.
- **No error boundary** — a Monaco crash takes down the entire app. Wrap at minimum the editor pane.
- **No reusable components** — buttons, list items, form fields are all raw HTML with CSS classes. A small `<ListItem>`, `<Field>`, `<Badge>` system would eliminate the inline style repetitions (lines 1290, 1345, 1353, 1452, 1482).

### Layout

- **Pane separator is not resizable** — the `div.pane-separator` is a static 1px line. Users can't adjust editor vs output ratio. A draggable splitter is expected in any IDE-like tool.
- **Output pane is always 50/50** — when there's no output, the right pane shows "Run a snippet to see output" taking half the screen. The editor should fill available space until first run.
- **Sidebar fixed at 320px** — not resizable either.

### Visual

- **No stderr differentiation** — stdout and stderr are concatenated with a text separator `--- stderr ---`. Color-code stderr (red-tinted) so users can instantly distinguish error output from normal output.
- **No output copy button** — users must manually select text from a `<pre>` tag.
- **Theme swatch is decorative only** — the color bars don't preview actual syntax. A small code preview (3-4 lines of Go) would be far more useful.

---

## UX

### First-Run Experience

- **Hostile empty state** — sidebar defaults to *closed*. New users see an editor and an empty output pane. No project is loaded. Every sidebar tab except Help shows "Open a project to..." messages. The app should either:
  - Auto-open sidebar on first launch showing the Help or Recent tab
  - Show a welcome overlay with "Open Folder" CTA
  - Auto-open the last-used project

### Critical UX Gaps

1. **`handleJumpToDiagnostic` doesn't jump** (`App.jsx:1112-1121`) — it only sets a status message. It should call `editor.revealLineInCenter(line)` and `editor.setPosition({lineNumber, column})` on the Monaco instance. This is the single biggest UX miss.

2. **No delete confirmations** — `handleDeleteSnippet` and `handleDeleteEnvVar` execute immediately. One misclick destroys data with no undo.

3. **`isBusy` blocks everything globally** — while a snippet runs (could be 15s), users can't browse snippets, switch tabs, or edit env vars. The busy lock should be scoped to the specific operation.

4. **Cmd+B conflicts** — this is "toggle bold" in most text editors. Users typing in Monaco will accidentally toggle the sidebar. Use a less common binding or make it configurable.

5. **No drag-and-drop** — desktop apps expect folder drop on the window to open a project.

6. **Output truncation is silent** — the 128KB limit in the Go backend sets `StdoutTruncated: true` but the frontend never checks or displays this. Users will see incomplete output with no warning.

7. **Tab order doesn't match usage** — "Project" is first but rarely used after initial setup. Snippets or a "Run" focused view should be primary.

### Minor UX

- No way to clear output without re-running
- No snippet auto-save or dirty indicator
- Env var "Edit" button populates the form but doesn't visually indicate you're editing (vs creating)
- Recent projects show full absolute paths — truncate to `~/Projects/foo`
- Settings Escape handler is on the panel div, not a global keydown — only works when the panel has focus

---

## Performance

### Bundle

- **14MB dist** — acceptable for desktop, but the `extensionHost.worker` (1.7MB) and duplicate `iconv-lite-umd` (291KB x 2) could be addressed with Vite's `manualChunks`.

### Rendering

1. **Every keystroke re-renders the entire app** — `setSnippet(newText)` triggers a render of the 1,706-line component including all sidebar tabs, output pane, status bar, and settings. Extracting components with `React.memo` would eliminate 90% of this.

2. **`editorAppConfig` recreates on every code change** (`MonacoEditor.jsx:145-155`) — the `useMemo` depends on `code`, so every keystroke creates a new config object. Since Monaco manages its own model, the initial `code` prop should only set the *initial* value, not re-derive config on every change.

3. **String concatenation in streaming** (`App.jsx:554-559`) — `Stdout: base.Stdout + chunk` creates a new string allocation per chunk. For large outputs, use an array of chunks and join only for display (or use a ref-based buffer).

4. **`lineCount` recalculates via `split("\n")` on every render** (`App.jsx:424-427`) — splits the entire source string just to count newlines. A simple loop counting `\n` chars avoids the array allocation. Or get it from Monaco's model.

5. **No list virtualization** — the snippet list, env var list, and diagnostic list render all items. With 100+ snippets this would degrade.

6. **`combinedOutput` memo** — fine for small outputs, but at 128KB it creates a new combined string on every chunk arrival (since `runResult` changes each chunk).

### Missing Optimizations

- No `React.lazy` / `Suspense` for the settings panel or sidebar tabs
- No `useTransition` for non-urgent state updates (snippet search filtering)
- Monaco `refreshEditorPresentation` does 4 try/catch blocks with language toggling — a heavyweight operation triggered on every settings change

---

## Top 5 Priority Changes

| Priority | Change | Impact |
|----------|--------|--------|
| 1 | **Decompose App.jsx** into 8-10 components with `React.memo` | Performance + maintainability |
| 2 | **Make diagnostics actually jump to line** in Monaco | Core UX fix |
| 3 | **Add resizable panes** (draggable splitter) | Expected IDE behavior |
| 4 | **Fix empty state** — auto-open sidebar, show welcome, restore last project | First-run experience |
| 5 | **Decouple `editorAppConfig` from `code`** — stop recreating Monaco config on every keystroke | Performance (biggest single win) |
