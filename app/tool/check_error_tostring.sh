#!/usr/bin/env bash
# US-096: bans raw `.toString()` on caught errors in the View layer.
#
# Every user-visible error message MUST route through
# `parseApiError(e)` (lib/core/error/api_error_parser.dart), which maps
# DioException codes / network failures to localized copy. Surfacing a
# raw `e.toString()` leaks stack-y text like "DioException [..]:
# SocketException" and is never translated. Epic 11 (US-083..US-095)
# burned down the existing offenders; this gate keeps them from coming
# back.
#
# Scope: only `presentation/screens/` and `presentation/widgets/` — the
# View layer that renders to the user. Controllers / core may legitimately
# stringify a cause before parsing it, so they are out of scope.
#
# The banned shape is `<errId>.toString()` where <errId> is a common
# caught-exception identifier (e, err, error, ex, exception). Numeric /
# property `.toString()` calls (e.g. `selection.length.toString()`,
# `count.toString()`) have a different receiver token and are NOT matched.
#
# Exit 0 = clean; non-zero = at least one offender (lines printed).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# A regex token boundary keeps `length.toString()` / `synced.toString()`
# from matching — only a bare error identifier as the receiver trips it.
PATTERN='\b(e|err|error|ex|exception)\.toString\(\)'

# Restrict to the rendered View layer.
PATH_FILTER='presentation/(screens|widgets)/'

if command -v rg >/dev/null 2>&1; then
  MATCHES=$(rg --line-number --no-heading --glob '!*.g.dart' --glob '!*_test.dart' \
    -e "${PATTERN}" "${ROOT}/lib" 2>/dev/null | grep -E "${PATH_FILTER}" || true)
else
  MATCHES=$(grep -rnE --include='*.dart' --exclude='*.g.dart' --exclude='*_test.dart' \
    "${PATTERN}" "${ROOT}/lib" 2>/dev/null | grep -E "${PATH_FILTER}" || true)
fi

# Drop comment lines (// ...).
MATCHES=$(echo "${MATCHES}" | grep -vE ':[[:space:]]*//' || true)

if [[ -z "${MATCHES}" ]]; then
  echo "check_error_tostring: OK — no raw error .toString() in the View layer."
  exit 0
fi

COUNT=$(echo "${MATCHES}" | grep -c '.' || true)
echo "check_error_tostring: ${COUNT} raw error .toString() in the View layer:" >&2
echo "${MATCHES}" >&2
echo >&2
echo "Route the error through parseApiError(e) from" >&2
echo "lib/core/error/api_error_parser.dart instead of <err>.toString()." >&2
exit 1
