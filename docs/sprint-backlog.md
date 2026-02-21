# GoPad Sprint Backlog (MVP)

## Document Control
- Product: GoPad
- Scope: MVP from `docs/prd.md` and `docs/roadmap.md`
- Last Updated: 2026-02-21
- Sprint Cadence: 1 week

## Planning Assumptions
- Story point scale: 1 (small), 2 (medium), 3 (large but sprint-safe).
- A ticket is "done" only if acceptance criteria pass and tests are added/updated.
- IDs use `GP-###` and are ordered by dependency.

## Sprint 1 (Foundation)

### GP-001 Create Wails app skeleton
- Estimate: 2
- Depends on: None
- Acceptance Criteria:
  - App boots on macOS from local dev command.
  - Main window opens with no runtime errors.

### GP-002 Build two-pane shell layout
- Estimate: 2
- Depends on: GP-001
- Acceptance Criteria:
  - Left editor pane and right output pane render.
  - Pane sizing remains usable after window resize.

### GP-003 Scaffold backend package structure
- Estimate: 2
- Depends on: GP-001
- Acceptance Criteria:
  - `engine`, `runner`, `project`, `snippet`, `env` packages exist.
  - Build succeeds with package stubs wired to app startup.

### GP-004 Define persistence schema v1
- Estimate: 2
- Depends on: GP-003
- Acceptance Criteria:
  - Initial schema supports Project, Snippet, Run, EnvVar entities.
  - Schema initialization runs once without duplication.

### GP-005 Add local storage bootstrap and health check
- Estimate: 1
- Depends on: GP-004
- Acceptance Criteria:
  - App initializes storage on first launch.
  - Health check endpoint/method reports ready state.

### GP-006 Add startup and run-latency telemetry hooks
- Estimate: 2
- Depends on: GP-001
- Acceptance Criteria:
  - Startup time captured per app launch.
  - Run trigger and first output timestamps are captured.

### GP-007 Set up CI for fmt, lint, unit tests
- Estimate: 2
- Depends on: GP-003
- Acceptance Criteria:
  - CI runs `gofmt` check, lint, and unit tests on pull requests.
  - CI status is required for merge.

## Sprint 2 (Core Loop Part A)

### GP-008 Implement project folder open flow
- Estimate: 2
- Depends on: GP-002
- Acceptance Criteria:
  - User can select a local directory from UI.
  - Invalid path errors are shown clearly.

### GP-009 Implement `go.mod` detection service
- Estimate: 1
- Depends on: GP-008
- Acceptance Criteria:
  - Service returns module-found or module-missing state.
  - Non-module folders do not crash or block UI.

### GP-010 Implement package/run target discovery
- Estimate: 2
- Depends on: GP-009
- Acceptance Criteria:
  - App lists runnable target options for selected project.
  - Selection persists for current session.

### GP-011 Persist and display recent projects
- Estimate: 1
- Depends on: GP-008, GP-005
- Acceptance Criteria:
  - Opened projects are saved with last-opened timestamp.
  - Recent list renders on startup.

### GP-012 Integrate editor with Go syntax highlighting
- Estimate: 2
- Depends on: GP-002
- Acceptance Criteria:
  - Go syntax highlighting works for snippet content.
  - Large snippets remain responsive while typing.

### GP-013 Add `gofmt` formatting action
- Estimate: 2
- Depends on: GP-012
- Acceptance Criteria:
  - Format action rewrites editor buffer with `gofmt` output.
  - Formatting errors are surfaced in UI.

### GP-014 Wire `Cmd+Enter` run shortcut
- Estimate: 1
- Depends on: GP-012
- Acceptance Criteria:
  - `Cmd+Enter` triggers run from editor.
  - Shortcut does not conflict with text input behavior.

## Sprint 3 (Core Loop Part B)

### GP-015 Build runner process lifecycle manager
- Estimate: 3
- Depends on: GP-003
- Acceptance Criteria:
  - Long-lived worker process is created per project.
  - Worker lifecycle can be started and cleanly stopped.

### GP-016 Implement run execution pipeline
- Estimate: 3
- Depends on: GP-010, GP-015
- Acceptance Criteria:
  - Run request executes snippet with selected project context.
  - Env vars and working directory are passed correctly.

### GP-017 Stream stdout to output pane
- Estimate: 2
- Depends on: GP-016
- Acceptance Criteria:
  - Stdout appears incrementally while process is running.
  - Stream handles multiline output safely.

### GP-018 Stream stderr to output pane
- Estimate: 2
- Depends on: GP-016
- Acceptance Criteria:
  - Stderr appears separately from stdout.
  - Stderr updates do not block stdout rendering.

### GP-019 Implement cancel action for active run
- Estimate: 2
- Depends on: GP-016
- Acceptance Criteria:
  - Cancel stops active process and updates status promptly.
  - Canceling an idle state is a no-op without error.

### GP-020 Implement rerun last snippet action
- Estimate: 1
- Depends on: GP-016
- Acceptance Criteria:
  - User can rerun with one action after completion/cancel.
  - Rerun uses current project context and env.

### GP-021 Add run state model in UI
- Estimate: 1
- Depends on: GP-017, GP-018, GP-019
- Acceptance Criteria:
  - UI reflects idle/running/success/failed/canceled states.
  - Buttons enable/disable correctly by state.

### GP-022 Add warm-run latency benchmark harness
- Estimate: 2
- Depends on: GP-015, GP-016
- Acceptance Criteria:
  - Benchmark reports trigger-to-first-output timing.
  - Baseline report is committed to docs/artifacts.

## Sprint 4 (Feedback and Safety)

### GP-023 Add exit code and duration reporting
- Estimate: 1
- Depends on: GP-016
- Acceptance Criteria:
  - Each run records and displays exit code and duration.
  - Missing values are handled without UI errors.

### GP-024 Build diagnostic parser for compile errors
- Estimate: 2
- Depends on: GP-018
- Acceptance Criteria:
  - Parser extracts file/line/column from common compile errors.
  - Parser has fixture tests.

### GP-025 Build diagnostic parser for runtime panics
- Estimate: 2
- Depends on: GP-018
- Acceptance Criteria:
  - Parser extracts actionable line targets from panic output.
  - Parser has fixture tests.

### GP-026 Implement clickable diagnostics in output
- Estimate: 2
- Depends on: GP-024, GP-025, GP-012
- Acceptance Criteria:
  - Clicking diagnostic jumps editor cursor to mapped line.
  - Invalid mappings fail gracefully.

### GP-027 Add run timeout enforcement
- Estimate: 1
- Depends on: GP-016
- Acceptance Criteria:
  - Timeout stops long-running executions automatically.
  - Timeout reason is visible in run status/output.

### GP-028 Add max output size guardrail
- Estimate: 1
- Depends on: GP-017, GP-018
- Acceptance Criteria:
  - Output is capped at configured maximum.
  - UI indicates when output was truncated.

### GP-029 Add hard-kill fallback and leak checks
- Estimate: 2
- Depends on: GP-019, GP-027
- Acceptance Criteria:
  - Stuck processes are terminated by fallback kill path.
  - Stress tests show no process leak regression.

## Sprint 5 (Project Ergonomics)

### GP-030 Implement `.env` loader per project
- Estimate: 2
- Depends on: GP-008, GP-005
- Acceptance Criteria:
  - `.env` file loads when project opens.
  - Invalid lines are reported without aborting load.

### GP-031 Build env var editor UI with masking
- Estimate: 2
- Depends on: GP-030
- Acceptance Criteria:
  - User can add/edit/remove env vars in UI.
  - Masked values are hidden by default.

### GP-032 Implement working directory selector
- Estimate: 1
- Depends on: GP-008
- Acceptance Criteria:
  - User can set working directory per project.
  - Selection persists across restarts.

### GP-033 Implement Go toolchain selector
- Estimate: 2
- Depends on: GP-010
- Acceptance Criteria:
  - Available toolchains are listed and selectable.
  - Selected toolchain is used in run execution.

### GP-034 Implement snippet CRUD backend
- Estimate: 2
- Depends on: GP-004
- Acceptance Criteria:
  - Create/read/update/delete snippet operations pass unit tests.
  - Invalid snippet payloads return clear errors.

### GP-035 Build snippet library UI
- Estimate: 2
- Depends on: GP-034
- Acceptance Criteria:
  - Project-scoped snippet list renders and loads selected snippet.
  - Rename, duplicate, and delete actions work from UI.

### GP-036 Implement snippet search
- Estimate: 1
- Depends on: GP-034
- Acceptance Criteria:
  - Search filters by snippet name and content.
  - Search results update in near real time on input.

### GP-037 Add persistence integrity tests
- Estimate: 2
- Depends on: GP-034, GP-011, GP-031
- Acceptance Criteria:
  - Restart tests confirm projects/snippets/env survive relaunch.
  - Corruption scenarios are detected and handled.

## Sprint 6 (Stabilization and Release)

### GP-038 Build FR acceptance test suite
- Estimate: 3
- Depends on: GP-037
- Acceptance Criteria:
  - Automated coverage exists for FR-1 through FR-7.
  - Test report is generated in CI.

### GP-039 Build NFR benchmark suite
- Estimate: 2
- Depends on: GP-022, GP-023
- Acceptance Criteria:
  - Benchmarks measure startup, warm run trigger, first feedback latency.
  - Results are tracked against NFR thresholds.

### GP-040 Run stress tests for run/cancel reliability
- Estimate: 2
- Depends on: GP-029
- Acceptance Criteria:
  - Repeated run/cancel cycles achieve target reliability.
  - Failures include actionable logs.

### GP-041 Fix P0/P1 defects from bug bash
- Estimate: 3
- Depends on: GP-038, GP-040
- Acceptance Criteria:
  - All P0/P1 issues are closed or explicitly waived.
  - Regression tests are added for each fixed P0/P1.

### GP-042 Ship onboarding and help docs
- Estimate: 1
- Depends on: GP-038
- Acceptance Criteria:
  - First-run instructions cover project open, run, cancel, save snippet.
  - Help content is reachable from app UI.

### GP-043 Implement packaging/signing pipeline
- Estimate: 2
- Depends on: GP-041
- Acceptance Criteria:
  - Release build pipeline generates installable artifact.
  - Signing/notarization steps are scripted for target platform.

### GP-044 Complete release checklist and RC signoff
- Estimate: 1
- Depends on: GP-039, GP-041, GP-042, GP-043
- Acceptance Criteria:
  - MVP release checklist is fully completed.
  - RC build is tagged and approved for GA.

## Backlog Hygiene Rules
- New scope must map to MVP goals or move to post-MVP.
- Carry-over tasks are split before moving to next sprint.
- No sprint starts without explicit dependency checks.
