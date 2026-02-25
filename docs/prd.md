# GoPoke MVP PRD

## Document Control
- Product: GoPoke
- Version: 1.0 (MVP)
- Status: Draft
- Last Updated: 2026-02-21

## 1. Product Summary
GoPoke is a desktop app for running and iterating on Go snippets quickly, similar to a "Tinkerwell for Go" workflow.

The MVP prioritizes fast local execution, reliable run/cancel behavior, useful error feedback, and a lightweight snippet library.

## 2. Problem
Go developers often use ad-hoc scratch files, shell commands, and editor hacks to quickly test ideas. This slows iteration and creates friction around setup, environment handling, and output visibility.

## 3. Goals
- Provide a fast, focused workflow for writing and running Go snippets.
- Reduce setup friction by using project context and per-project environment settings.
- Make compile/runtime feedback clear and actionable.

## 4. Non-Goals (Out of MVP)
- Team collaboration and cloud sync.
- Remote/containerized execution.
- Plugin marketplace.
- AI features.
- Advanced database tooling and deep performance dashboards.

## 5. Target Users
- Go backend developers testing logic quickly.
- Developers exploring libraries/APIs inside existing Go projects.
- Engineers debugging small transformations and data-processing snippets.

## 6. Core User Flows
1. Open a local Go project and select working package.
2. Write or paste a snippet in the editor.
3. Run with one shortcut/action and view output instantly.
4. Click compiler/runtime errors to jump to source lines.
5. Save useful snippets for later reuse.

## 7. MVP Features

### 7.1 Project Context Loader
- Open local folder and detect `go.mod`.
- Select package/run target.
- Persist recent projects.

### 7.2 Go Snippet Editor
- Syntax highlighting.
- Autocomplete and basic code intelligence.
- Format code with `gofmt`.
- Keyboard shortcut: `Cmd+Enter` to run.

### 7.3 Fast Run Engine
- Long-lived worker process per project for low run latency.
- Actions: Run, Cancel, Re-run.
- Reuse module/build cache between runs.

### 7.4 Output and Error Panel
- Split view for stdout and stderr.
- Show exit status and execution duration.
- Parse and map compile/runtime errors to editor lines.

### 7.5 Per-Project Environment
- Load `.env` values when present.
- Editable env vars in app UI.
- Configurable working directory and Go toolchain selection.

### 7.6 Snippet Library
- Save, rename, duplicate, and delete snippets.
- Project-scoped snippet list.
- Search snippets by name/content.

### 7.7 Execution Safety
- Configurable timeout.
- Max output guardrail.
- Clear running state.
- Reliable process termination for stuck runs.

## 8. Functional Requirements
- FR-1: App must open a local folder and detect whether it is a Go module.
- FR-2: App must run snippets against selected project context.
- FR-3: App must support run cancellation and recover for immediate rerun.
- FR-4: App must display stdout/stderr independently.
- FR-5: App must map compiler/runtime errors to line numbers when parsable.
- FR-6: App must support snippet CRUD operations.
- FR-7: App must persist recent projects, snippet metadata, and project env settings.

## 9. Non-Functional Requirements
- NFR-1: Cold app start under 2s on target dev hardware.
- NFR-2: Warm run trigger under 200ms.
- NFR-3: Initial execution feedback shown within 500ms in common cases.
- NFR-4: UI must remain responsive during run/cancel cycles.
- NFR-5: No data loss for saved snippets after normal app restarts.

## 10. UX Requirements
- Native-feeling macOS title bar/toolbar behavior.
- Two-pane main layout: editor (left) and output (right).
- Clear run states: idle, running, canceled, failed, success.
- Clickable diagnostics that navigate to exact editor lines.

## 11. Technical Scope (MVP)
- Desktop shell: Wails (Go-first).
- Backend core packages:
  - `engine` (execution orchestration)
  - `runner` (worker lifecycle)
  - `project` (module/package detection)
  - `snippet` (persistence/search)
  - `env` (project environment settings)
- Frontend: React + Vite app embedded by Wails.
- Local persistence only.

## 12. Data Model (Initial)
- Project
  - `id`, `path`, `last_opened_at`, `default_package`, `toolchain`
- Snippet
  - `id`, `project_id`, `name`, `content`, `created_at`, `updated_at`
- Run
  - `id`, `project_id`, `snippet_id`, `started_at`, `duration_ms`, `exit_code`, `status`
- EnvVar
  - `id`, `project_id`, `key`, `value`, `masked`

## 13. Success Metrics
- 70%+ of weekly active users execute at least one snippet per session.
- Median runs per active session >= 4.
- Run cancellation success rate >= 99%.
- Error-to-line mapping success rate >= 95% for supported formats.

## 14. Risks and Mitigations
- Risk: Slow first run due to module/toolchain state.
  - Mitigation: Warm worker and cache reuse; show progress state.
- Risk: Inconsistent error parsing across Go versions.
  - Mitigation: Structured parser tests against version fixtures.
- Risk: Process leaks on repeated cancel/re-run.
  - Mitigation: Worker lifecycle ownership and kill-timeout fallback.

## 15. Release Criteria (MVP Exit)
- All MVP features in section 7 completed.
- Functional requirements FR-1 to FR-7 pass acceptance tests.
- Performance targets in section 9 met on target machines.
- No P0/P1 defects open.
- Basic onboarding and docs shipped.

## 16. Open Questions
- Will MVP support only macOS or macOS + Linux at launch?
- Is snippet import/export needed for first release?
- What minimum Go toolchain versions are officially supported?
