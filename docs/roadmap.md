# GoPad MVP Roadmap

## Document Control
- Product: GoPad
- Version: 1.0 (MVP)
- Status: Draft
- Last Updated: 2026-02-21

## Roadmap Principles
- Keep scope aligned to PRD section 7 only.
- Ship value incrementally with releasable checkpoints.
- Track readiness with FR/NFR gates, not just feature completion.

## Phase 0: Foundation (Week 1)
### Objectives
- Establish the executable desktop skeleton and baseline architecture.
- De-risk core run loop and process lifecycle early.

### Deliverables
- Wails app bootstrapped with two-pane layout shell.
- Backend package scaffolding:
  - `engine`
  - `runner`
  - `project`
  - `snippet`
  - `env`
- Local storage setup for projects/snippets/settings.
- Basic telemetry hooks for startup time and run latency.

### Exit Criteria
- App launches and renders editor/output panes.
- Baseline startup metric collection works.
- No blocking architecture unknowns remain for run engine.

## Phase 1: Core Execution Loop (Week 2-3)
### Objectives
- Deliver the fastest path from snippet input to visible output.

### Deliverables
- Project Context Loader:
  - open folder
  - detect `go.mod`
  - select package/run target
  - persist recent projects
- Snippet editor essentials:
  - syntax highlighting
  - `gofmt` action
  - `Cmd+Enter` run shortcut
- Fast Run Engine v1:
  - run/cancel/re-run
  - long-lived worker lifecycle
  - stdout/stderr streaming

### FR Coverage
- FR-1, FR-2, FR-3, FR-4 (partial for basic panel rendering)

### Exit Criteria
- User can open project, run snippet, cancel run, and rerun.
- UI stays responsive through repeated run/cancel cycles.
- Warm run trigger meets target trend toward NFR-2.

## Phase 2: Developer Feedback Quality (Week 4)
### Objectives
- Make results understandable and actionable immediately.

### Deliverables
- Output/Error panel improvements:
  - separate stdout/stderr
  - exit code and duration
  - run state indicators (idle/running/canceled/failed/success)
- Diagnostic parser:
  - map compile/runtime errors to editor lines
  - clickable navigation from output to source line
- Execution safety controls:
  - timeout
  - max output cap
  - hard-kill fallback for stuck process

### FR Coverage
- FR-4 (complete), FR-5

### Exit Criteria
- Error-to-line mapping validates against fixture set.
- Cancel reliability reaches target confidence for MVP.
- No process leaks in stress run/cancel test suite.

## Phase 3: Project Ergonomics (Week 5)
### Objectives
- Reduce setup friction and preserve developer flow across sessions.

### Deliverables
- Per-project environment:
  - `.env` load support
  - editable env var UI
  - working directory and toolchain selection
- Snippet library:
  - create/save/rename/duplicate/delete
  - project-scoped list
  - search by name/content
- Persistence hardening for project/snippet metadata.

### FR Coverage
- FR-6, FR-7

### Exit Criteria
- Saved snippets survive app restart without corruption.
- Project-specific env settings load correctly on reopen.
- Snippet search latency remains acceptable on seed dataset.

## Phase 4: Stabilization and MVP Release (Week 6)
### Objectives
- Convert feature-complete build into a release candidate.

### Deliverables
- End-to-end acceptance test pass for FR-1 to FR-7.
- Performance pass for NFR-1 to NFR-5.
- Bug bash and P0/P1 triage closure.
- Basic onboarding/help documentation.
- Packaging/signing pipeline for target platform(s).

### Exit Criteria
- All PRD MVP features complete.
- No open P0/P1 defects.
- Release checklist approved for GA.

## Milestone Gates
- M1 (end Phase 1): Core run loop usable daily by internal team.
- M2 (end Phase 2): Feedback loop quality acceptable for private alpha.
- M3 (end Phase 3): Feature-complete MVP for beta users.
- M4 (end Phase 4): Production-ready MVP release.

## Dependencies and Critical Path
- Critical path: worker lifecycle -> diagnostic mapping -> persistence stability.
- External dependencies:
  - Go toolchain compatibility matrix
  - editor component integration
  - macOS packaging/signing setup

## Risk Register (Roadmap View)
- Slow first run due to module/toolchain state.
  - Mitigation: warm worker + cache priming strategy.
- Error parsing drift across Go versions.
  - Mitigation: versioned parser fixtures in CI.
- Cancel/re-run instability under load.
  - Mitigation: lifecycle ownership model + stress tests.

## Post-MVP Candidates (Not Scheduled)
- Team sharing/cloud sync.
- Remote/container execution.
- Plugin ecosystem.
- AI-assisted workflows.
