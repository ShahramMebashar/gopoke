# GoPoke Task Tracker

## Status Legend
- `TODO`: Not started.
- `IN_PROGRESS`: Started and actively being worked.
- `DONE`: Implemented with passing local tests.
- `BLOCKED`: Waiting on dependency/tooling decision.

## Current Snapshot (2026-02-21)
- Done: 43
- In Progress: 0
- Blocked: 0
- Todo: 1

## Sprint 1

| Ticket | Status | Notes |
|---|---|---|
| GP-001 | DONE | Wails bridge + runtime entrypoint verified with launch probe (`go run -tags 'wails,desktop,production' ./cmd/gopoke`) and app startup log observed. |
| GP-002 | TODO | UI shell not implemented yet. |
| GP-003 | DONE | Package scaffolding added under `internal/`. |
| GP-004 | DONE | Schema v1 and storage snapshot types implemented. |
| GP-005 | DONE | Storage bootstrap + health check implemented with tests. |
| GP-006 | DONE | Startup/run telemetry recorder implemented with tests. |
| GP-007 | DONE | CI added for `gofmt`, `go vet`, and `go test`. |

## Sprint 2

| Ticket | Status | Notes |
|---|---|---|
| GP-008 | DONE | Native folder picker (`ChooseProjectDirectory`) and project open UI flow implemented with clear invalid-path error rendering. |
| GP-009 | DONE | `internal/project` module detection service + tests. |
| GP-010 | DONE | Explicit target selector UI added with persisted default package (`SetProjectDefaultPackage`) and reopen preservation. |
| GP-011 | DONE | Startup recent-project list now loads via `RecentProjects` and supports click-to-open into existing `OpenProject` flow. |
| GP-012 | DONE | React editor surface implemented using CodeMirror with Go syntax highlighting, line numbers, and live line/char metrics. |
| GP-013 | DONE | `Format (gofmt)` action wired through Wails bridge; editor buffer updates on success and surfaces gofmt errors in status. |
| GP-014 | DONE | `Cmd+Enter` shortcut wired in CodeMirror (`Mod-Enter`) to run snippet pipeline; also available via Run button with output panel. |

## Sprint 3

| Ticket | Status | Notes |
|---|---|---|
| GP-015 | DONE | Process-based worker manager implemented (start/reuse/stop/stop-all) with headless worker mode and lifecycle tests. |
| GP-016 | DONE | Run execution pipeline now accepts typed run requests (`projectPath`, `packagePath`, `source`) and resolves cwd/env from selected/default project context before execution. |
| GP-017 | DONE | Run stdout now streams incrementally from Go process to frontend via Wails events (`gopoke:run:stdout-chunk`) and appends safely into output pane during active run. |
| GP-018 | DONE | Stderr now streams incrementally over dedicated Wails event channel (`gopoke:run:stderr-chunk`) and renders independently from stdout. |
| GP-019 | DONE | Active-run cancel API added end-to-end (`CancelRun`), canceled runs return `Canceled=true`, and idle cancel is a no-op without error. |
| GP-020 | DONE | `Re-run Last` action added to editor toolbar and reruns last executed snippet content with current project target/env context. |
| GP-021 | DONE | Explicit UI run state model added (`idle/running/success/failed/canceled`) with state-driven run/rerun/cancel button gating. |
| GP-022 | DONE | Warm-run latency benchmark harness added (`BenchmarkWarmRunLatency`) with baseline artifact committed at `docs/artifacts/gp-022-warm-run-baseline.md`. |

## Sprint 4

| Ticket | Status | Notes |
|---|---|---|
| GP-023 | DONE | Run metadata now persists per execution (`RunRecord`) with status/exit code/duration and UI metric display is null-safe for partial states. |
| GP-024 | DONE | Added compile diagnostic parser (`internal/diagnostics`) extracting file/line/column from common compiler error formats with fixture-driven tests. |
| GP-025 | DONE | Added runtime panic parser extracting actionable stack-frame line targets with fixture-driven tests (including optional-column and Windows-path formats). |
| GP-026 | DONE | Output panel now renders clickable diagnostics; clicking jumps CodeMirror cursor to mapped line/column and invalid mappings fail gracefully with status feedback. |
| GP-027 | DONE | Timeout enforcement now supports per-run `timeoutMs` override and surfaces timeout reason explicitly (`Run timed out...` + stderr timeout marker). |
| GP-028 | DONE | Added output guardrail with capped stdout/stderr capture + stream truncation flags; UI displays truncation indicators in run metadata. |
| GP-029 | DONE | Implemented hard-kill fallback lifecycle (`SIGINT` then forced kill) plus cancel stress/fallback tests to guard against stuck process leaks. |

## Sprint 5

| Ticket | Status | Notes |
|---|---|---|
| GP-030 | DONE | Project open now loads `.env`, parses `KEY=VALUE` entries, upserts into persisted env vars, and reports invalid line warnings without aborting open. |
| GP-031 | DONE | Added env var CRUD APIs/UI with masked-by-default rendering, show/hide toggle, inline edit, and delete actions. |
| GP-032 | DONE | Added persisted project working directory selection (`SetProjectWorkingDirectory`) and execution now honors saved working directory on runs. |
| GP-033 | DONE | Added Go toolchain discovery/selection APIs and run execution now uses the selected toolchain binary path. |
| GP-034 | DONE | Implemented persistent snippet CRUD in storage/app/Wails layers with validation and unit tests for create/read/update/delete + invalid payload handling. |
| GP-035 | DONE | Added project-scoped snippet library UI with load/select plus save/new/duplicate/delete actions and rename-through-save workflow. |
| GP-036 | DONE | Implemented near-real-time snippet filtering by name and content in React via local search state + memoized filtering. |
| GP-037 | DONE | Added restart integrity tests for project/snippet/env persistence and corruption detection tests for invalid state file handling. |

## Sprint 6

| Ticket | Status | Notes |
|---|---|---|
| GP-038 | DONE | Added `internal/acceptance` FR suite with explicit `FR-1`..`FR-7` subtests and CI report generation via `scripts/run-fr-acceptance.sh` with uploaded artifact (`gp-038-fr-acceptance-report`). |
| GP-039 | DONE | Added `internal/benchmarks` NFR suite (`BenchmarkNFRStartupLatency`, `BenchmarkNFRWarmRunTriggerLatency`, `BenchmarkNFRFirstFeedbackLatency`) with threshold report script `scripts/run-nfr-benchmarks.sh` + CI artifact upload. |
| GP-040 | DONE | Added tagged run/cancel stress harness (`internal/stress/TestRunCancelReliabilityStress`) with reliability summary/failure log lines and report automation via `scripts/run-run-cancel-stress.sh` + CI artifact upload. |
| GP-041 | DONE | Closed P1 defects from bug bash: warm-run cache churn (`internal/execution` stable snippet cache paths) and early-cancel error normalization (`internal/app` now returns canceled/timed-out results instead of context errors); regression tests added and closure log captured in `docs/artifacts/gp-041-bug-bash-defects.md`. |
| GP-042 | DONE | Shipped onboarding/help docs at `docs/onboarding.md` and added in-app Help entry (`open-help` toolbar action + help card checklist/tips) reachable from UI. |
| GP-043 | DONE | Added macOS packaging/signing pipeline with `cmd/gopoke/wails.json`, `scripts/release-macos.sh`, and manual GitHub workflow `.github/workflows/release-macos.yml` producing zip/report artifacts with optional codesign/notarization env hooks. |
| GP-044 | DONE | Added release checklist `docs/release-checklist.md` and RC signoff automation `scripts/release-rc-signoff.sh` generating `artifacts/gp-044-rc-signoff.md` with gate evidence and optional tag creation flow (`RC_CREATE_TAG=1`). |

## Next Up
1. Implement GP-002 UI shell.
