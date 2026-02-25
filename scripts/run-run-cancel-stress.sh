#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-$repo_root/artifacts}"
mkdir -p "$out_dir"

log_file="$out_dir/gp-040-run-cancel-stress.log"
report_file="$out_dir/gp-040-run-cancel-stress-report.md"

timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
iterations="${GOPOKE_STRESS_ITERATIONS:-24}"

cd "$repo_root"

suite_status="PASSED"
if GOPOKE_STRESS_ITERATIONS="$iterations" go test -tags stress -run TestRunCancelReliabilityStress -v ./internal/stress >"$log_file" 2>&1; then
  suite_status="PASSED"
else
  suite_status="FAILED"
fi

summary_line="$(grep 'STRESS_SUMMARY' "$log_file" | tail -n 1 || true)"

extract_summary_value() {
  local key="$1"
  local line="$2"
  if [[ -z "$line" ]]; then
    echo ""
    return
  fi
  awk -v key="$key" '
    {
      for (i = 1; i <= NF; i++) {
        split($i, pair, "=")
        if (pair[1] == key) {
          print pair[2]
          exit
        }
      }
    }
  ' <<<"$line"
}

successes="$(extract_summary_value "successes" "$summary_line")"
failures="$(extract_summary_value "failures" "$summary_line")"
reliability="$(extract_summary_value "reliability" "$summary_line")"
target="$(extract_summary_value "target" "$summary_line")"

if [[ -z "$successes" ]]; then successes="N/A"; fi
if [[ -z "$failures" ]]; then failures="N/A"; fi
if [[ -z "$reliability" ]]; then reliability="N/A"; fi
if [[ -z "$target" ]]; then target="N/A"; fi

failure_lines="$(grep 'STRESS_FAILURE' "$log_file" || true)"

cat >"$report_file" <<EOF_REPORT
# GP-040 Run/Cancel Stress Report

- Generated At (UTC): $timestamp
- Suite: \`go test -tags stress -run TestRunCancelReliabilityStress -v ./internal/stress\`
- Config: \`GOPOKE_STRESS_ITERATIONS=$iterations\`
- Result: **$suite_status**

| Metric | Value |
|---|---|
| Iterations | $iterations |
| Successes | $successes |
| Failures | $failures |
| Reliability | $reliability |
| Reliability Target | $target |

## Failure Log

\`\`\`text
${failure_lines:-No failure lines emitted.}
\`\`\`

## Tail Log

\`\`\`text
$(tail -n 140 "$log_file")
\`\`\`
EOF_REPORT

if [[ "$suite_status" != "PASSED" ]]; then
  cat "$log_file"
  exit 1
fi
