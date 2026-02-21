#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-$repo_root/artifacts}"
mkdir -p "$out_dir"

log_file="$out_dir/gp-038-fr-acceptance.log"
report_file="$out_dir/gp-038-fr-acceptance-report.md"

cd "$repo_root"

suite_status="PASSED"
if go test -v ./internal/acceptance >"$log_file" 2>&1; then
  suite_status="PASSED"
else
  suite_status="FAILED"
fi

fr_status() {
  local fr="$1"
  if grep -Eq -- "--- PASS: TestFRAcceptanceSuite/${fr}" "$log_file"; then
    echo "PASS"
    return
  fi
  if grep -Eq -- "--- FAIL: TestFRAcceptanceSuite/${fr}" "$log_file"; then
    echo "FAIL"
    return
  fi
  echo "NOT RUN"
}

timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

cat >"$report_file" <<EOF_REPORT
# GP-038 FR Acceptance Report

- Generated At (UTC): $timestamp
- Suite: \`go test -v ./internal/acceptance\`
- Result: **$suite_status**

| Functional Requirement | Status |
|---|---|
| FR-1 | $(fr_status FR-1) |
| FR-2 | $(fr_status FR-2) |
| FR-3 | $(fr_status FR-3) |
| FR-4 | $(fr_status FR-4) |
| FR-5 | $(fr_status FR-5) |
| FR-6 | $(fr_status FR-6) |
| FR-7 | $(fr_status FR-7) |

## Tail Log

\`\`\`text
$(tail -n 120 "$log_file")
\`\`\`
EOF_REPORT

if [[ "$suite_status" != "PASSED" ]]; then
  cat "$log_file"
  exit 1
fi
