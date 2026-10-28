#!/usr/bin/env bash
# US-111: bans raw `NetworkImage(` / `Image.network(` outside the
# `AppNetworkImage` definition.
#
# Epic 13 (US-102..US-110) routed every network image through
# `AppNetworkImage` (lib/design_system/components/cached_network_image.dart),
# which adds a loading placeholder, an error fallback, and an automatic
# `cacheWidth` so a 2000px original isn't decoded into memory just to render
# at 64px. Reaching for a raw `Image.network(...)` widget or a bare
# `NetworkImage(...)` ImageProvider skips all of that and re-introduces the
# unbounded-memory / no-fallback behaviour the epic removed. This gate keeps
# those fixes from regressing.
#
# Scope: all of `lib/` EXCEPT the one file that legitimately wraps
# `Image.network` — `cached_network_image.dart` (the AppNetworkImage
# definition). Generated (`*.g.dart`) and test (`*_test.dart`) files are out
# of scope.
#
# `AppNetworkImage(` is NOT matched: the `\b` word boundary before
# `NetworkImage` requires a non-word char (or start of token) immediately
# before the `N`, and `AppNetworkImage` has a word char (`p`) there.
#
# Exit 0 = clean; non-zero = at least one offender (lines printed).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Raw network-image entry points. The `\b` keeps `AppNetworkImage(` from
# matching the `NetworkImage(` alternative.
PATTERN='(\bNetworkImage\(|\bImage\.network\()'

# The single allowed home of `Image.network` — the AppNetworkImage wrapper.
DEFINITION='design_system/components/cached_network_image.dart'

if command -v rg >/dev/null 2>&1; then
  MATCHES=$(rg --line-number --no-heading --glob '!*.g.dart' --glob '!*_test.dart' \
    -e "${PATTERN}" "${ROOT}/lib" 2>/dev/null | grep -vF "${DEFINITION}" || true)
else
  MATCHES=$(grep -rnE --include='*.dart' --exclude='*.g.dart' --exclude='*_test.dart' \
    "${PATTERN}" "${ROOT}/lib" 2>/dev/null | grep -vF "${DEFINITION}" || true)
fi

# Drop comment lines (// ...).
MATCHES=$(echo "${MATCHES}" | grep -vE ':[[:space:]]*//' || true)

if [[ -z "${MATCHES}" ]]; then
  echo "check_network_image: OK — no raw NetworkImage/Image.network outside AppNetworkImage."
  exit 0
fi

COUNT=$(echo "${MATCHES}" | grep -c '.' || true)
echo "check_network_image: ${COUNT} raw NetworkImage/Image.network outside AppNetworkImage:" >&2
echo "${MATCHES}" >&2
echo >&2
echo "Use AppNetworkImage from" >&2
echo "lib/design_system/components/cached_network_image.dart instead — it adds" >&2
echo "a placeholder, error fallback, and cacheWidth. For ImageProvider sites" >&2
echo "(CircleAvatar/DecorationImage) render AppNetworkImage as a circular child." >&2
exit 1
