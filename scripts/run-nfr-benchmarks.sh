#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-$repo_root/artifacts}"
mkdir -p "$out_dir"

log_file="$out_dir/gp-039-nfr-benchmarks.log"
report_file="$out_dir/gp-039-nfr-benchmarks-report.md"

timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

cd "$repo_root"

suite_status="PASSED"
if go test -run '^$' -bench 'BenchmarkNFR(StartupLatency|WarmRunTriggerLatency|FirstFeedbackLatency)$' -benchtime=5x ./internal/benchmarks >"$log_file" 2>&1; then
  suite_status="PASSED"
else
  suite_status="FAILED"
fi

extract_metric() {
  local metric_unit="$1"
  awk -v unit="$metric_unit" '
    {
      for (i = 1; i <= NF; i++) {
        if ($i == unit) {
          print $(i - 1)
          exit
        }
      }
    }
  ' "$log_file"
}

metric_status() {
  local value="$1"
  local threshold="$2"
  if [[ -z "$value" ]]; then
    echo "NOT RUN"
    return
  fi
  awk -v value="$value" -v threshold="$threshold" 'BEGIN { if ((value + 0) <= (threshold + 0)) print "PASS"; else print "FAIL" }'
}

startup_ms="$(extract_metric 'startup_ms/op')"
warm_trigger_ms="$(extract_metric 'warm_trigger_ms/op')"
first_feedback_ms="$(extract_metric 'first_feedback_ms/op')"

nfr1_status="$(metric_status "$startup_ms" "2000")"
nfr2_status="$(metric_status "$warm_trigger_ms" "200")"
nfr3_status="$(metric_status "$first_feedback_ms" "500")"

cat >"$report_file" <<EOF_REPORT
# GP-039 NFR Benchmark Report

- Generated At (UTC): $timestamp
- Suite: \`go test -run '^$' -bench 'BenchmarkNFR(StartupLatency|WarmRunTriggerLatency|FirstFeedbackLatency)$' -benchtime=5x ./internal/benchmarks\`
- Result: **$suite_status**

| NFR | Metric | Threshold | Measured | Status |
|---|---|---:|---:|---|
| NFR-1 (Cold start) | startup latency | <= 2000ms | ${startup_ms:-N/A}ms | $nfr1_status |
| NFR-2 (Warm run trigger) | warm trigger latency | <= 200ms | ${warm_trigger_ms:-N/A}ms | $nfr2_status |
| NFR-3 (First feedback) | time to first output | <= 500ms | ${first_feedback_ms:-N/A}ms | $nfr3_status |

## Tail Log

\`\`\`text
$(tail -n 120 "$log_file")
\`\`\`
EOF_REPORT

if [[ "$suite_status" != "PASSED" ]]; then
  cat "$log_file"
  exit 1
fi

if [[ -n "${ENFORCE_NFR_THRESHOLDS:-}" ]]; then
  if [[ "$nfr1_status" != "PASS" || "$nfr2_status" != "PASS" || "$nfr3_status" != "PASS" ]]; then
    echo "NFR thresholds not met; see $report_file"
    exit 1
  fi
fi
