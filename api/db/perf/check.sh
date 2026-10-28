#!/usr/bin/env bash
# =============================================================================
# Replider — EXPLAIN ANALYZE perf gate (US-062)
# =============================================================================
# Runs every tagged query under db/perf/queries/ through
# `EXPLAIN (ANALYZE, FORMAT JSON)` against a seeded database and FAILS if any
# Seq Scan node touched more than $SEQ_SCAN_ROW_LIMIT rows (default 1000).
# "Touched" = rows emitted + rows removed by filter, × loops — i.e. the number
# of rows the planner actually walked sequentially. That catches a dropped or
# unused index (the regression this gate exists to block) without flagging a
# tiny seq scan over a dimension table.
#
# Usage:
#   DATABASE_URL=postgres://user:pass@host:5432/db ./db/perf/check.sh
#   (or pass a libpq conninfo string as $1)
#
# Requires: psql, jq.
# =============================================================================
set -euo pipefail

CONN="${1:-${DATABASE_URL:-}}"
if [[ -z "$CONN" ]]; then
  echo "error: no connection string (set DATABASE_URL or pass it as \$1)" >&2
  exit 2
fi

SEQ_SCAN_ROW_LIMIT="${SEQ_SCAN_ROW_LIMIT:-1000}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QUERY_DIR="$SCRIPT_DIR/queries"

command -v jq   >/dev/null || { echo "error: jq not found"   >&2; exit 2; }
command -v psql >/dev/null || { echo "error: psql not found" >&2; exit 2; }

echo "== EXPLAIN ANALYZE perf gate (seq-scan limit: ${SEQ_SCAN_ROW_LIMIT} rows) =="

# jq program: max rows walked across every Seq Scan node in the plan tree.
JQ_MAX_SEQ_ROWS='
  [ .. | objects
    | select((."Node Type"? // "") == "Seq Scan")
    | ((."Actual Rows" // 0) + (."Rows Removed by Filter" // 0)) * (."Actual Loops" // 1)
  ] | max // 0'

failures=0
shopt -s nullglob
for qfile in "$QUERY_DIR"/*.sql; do
  name="$(basename "$qfile" .sql)"
  sql="$(cat "$qfile")"

  plan="$(psql "$CONN" -X -At -v ON_ERROR_STOP=1 \
            -c "EXPLAIN (ANALYZE, FORMAT JSON, TIMING OFF, SUMMARY OFF) $sql")"

  max_seq="$(printf '%s' "$plan" | jq "$JQ_MAX_SEQ_ROWS")"

  if (( max_seq > SEQ_SCAN_ROW_LIMIT )); then
    echo "FAIL  ${name}: Seq Scan walked ${max_seq} rows (> ${SEQ_SCAN_ROW_LIMIT})"
    echo "----- plan -----"
    printf '%s\n' "$plan" | jq -r '.[0].Plan | .. | objects
      | select(."Node Type"?) | "  \(."Node Type")\(if ."Relation Name" then " on \(."Relation Name")" else "" end) — rows=\(."Actual Rows" // 0) removed=\(."Rows Removed by Filter" // 0)"'
    echo "----------------"
    failures=$((failures + 1))
  else
    echo "ok    ${name}: max Seq Scan rows = ${max_seq}"
  fi
done

if (( failures > 0 )); then
  echo "== perf gate FAILED: ${failures} query(ies) seq-scanned > ${SEQ_SCAN_ROW_LIMIT} rows =="
  exit 1
fi
echo "== perf gate passed =="
