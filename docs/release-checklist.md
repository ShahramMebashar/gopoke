# GoPad MVP Release Checklist

## Quality Gates

- [x] FR acceptance suite (`FR-1` through `FR-7`) is automated and reportable.
- [x] NFR benchmark suite covers startup, warm trigger, and first feedback latency.
- [x] Run/cancel stress suite meets reliability target with actionable failure logs.
- [x] P0/P1 defects are closed or explicitly waived with regression coverage.
- [x] Onboarding/help docs are shipped and reachable from app UI.
- [x] Packaging/signing pipeline exists for macOS artifacts.
- [x] RC signoff artifact can be generated for release approval.

## Evidence

- FR report: `artifacts/gp-038-fr-acceptance-report.md`
- NFR report: `artifacts/gp-039-nfr-benchmarks-report.md`
- Stress report: `artifacts/gp-040-run-cancel-stress-report.md`
- Defect closure: `docs/artifacts/gp-041-bug-bash-defects.md`
- Onboarding docs: `docs/onboarding.md`
- Packaging docs: `docs/release-macos.md`
- RC signoff: `artifacts/gp-044-rc-signoff.md`
