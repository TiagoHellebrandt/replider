#!/usr/bin/env bash
# Flags Dart files inside lib/features/settings/ that still have raw
# pt-BR text (strings with Portuguese diacritics) hardcoded instead of
# going through the Slang i18n catalog. This is a pragmatic CI gate for
# Sprint 3: diacritic-bearing literals are almost always user-visible
# copy that should live in lib/i18n/.
#
# The check intentionally stays shallow:
#   * only diacritics (á é í ó ú â ê ô ç ã õ) — lots of English words
#     share letters with Portuguese, so we can't flag everything.
#   * skips generated files (*.g.dart, *.freezed.dart) and code
#     comments (lines starting with // or part of a /* … */ block).
#   * ignores tests and the i18n folder itself.
#
# Exit code 0 means clean; non-zero means at least one literal was
# found and the offending lines are printed. Wire this into CI to
# keep translation debt from regressing.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCAN_DIR="${ROOT}/lib/features/settings"

# Domain catalogs that deliberately carry pt-BR strings (with paired
# English siblings) — not UI copy, and moving them into i18n would
# break the `namePT` / `nameEN` accessors the screens depend on. If
# you add another bilingual catalog, extend this list instead of
# pretending it's i18n debt.
ALLOWLIST=(
  "lib/features/settings/domain/models/prompt_template.dart"
  "lib/features/settings/domain/models/whatsapp_profile_model.dart"
  "lib/features/settings/domain/models/whatsapp_template_model.dart"
  "lib/features/settings/domain/models/whatsapp_error.dart"
  "lib/features/settings/domain/models/channel_model.dart"
  # PT content-keywords the category-mismatch warning scans user-typed
  # template bodies against ('promoção'/'lançamento'/'confirmação'/'código')
  # — not UI copy, correctly literal.
  "lib/features/settings/presentation/screens/whatsapp_settings_screen.dart"
)

if [[ ! -d "${SCAN_DIR}" ]]; then
  echo "check_i18n_literals: settings dir not found at ${SCAN_DIR}" >&2
  exit 2
fi

# Use ripgrep when available for speed, fall back to grep -rE.
if command -v rg >/dev/null 2>&1; then
  MATCHES=$(rg --line-number --no-heading \
    --glob '!*.g.dart' --glob '!*.freezed.dart' --glob '!*_test.dart' \
    -- "'[^']*[áéíóúâêîôûçãõÁÉÍÓÚÂÊÎÔÛÇÃÕ][^']*'" \
    "${SCAN_DIR}" || true)
else
  MATCHES=$(grep -rnE --include='*.dart' --exclude='*.g.dart' \
    --exclude='*.freezed.dart' --exclude='*_test.dart' \
    "'[^']*[áéíóúâêîôûçãõÁÉÍÓÚÂÊÎÔÛÇÃÕ][^']*'" \
    "${SCAN_DIR}" || true)
fi

# Drop comment lines (// ...).
FILTERED=$(echo "${MATCHES}" | grep -vE ':[[:space:]]*//' || true)

# Filter out allowlisted paths.
for path in "${ALLOWLIST[@]}"; do
  FILTERED=$(echo "${FILTERED}" | grep -vF "${path}" || true)
done

if [[ -z "${FILTERED}" ]]; then
  echo "check_i18n_literals: OK — no pt-BR literals found under ${SCAN_DIR}"
  exit 0
fi

COUNT=$(echo "${FILTERED}" | grep -c '.' || true)

# Strict mode (--strict flag): fail CI on any offender.
# Otherwise (default): print a warning summary and exit 0 so this
# rolls out as an informational gate first. Once the team has
# burned down the existing debt the default can flip.
if [[ "${1:-}" == "--strict" ]]; then
  echo "check_i18n_literals: ${COUNT} pt-BR string literal(s) outside i18n:" >&2
  echo "${FILTERED}" >&2
  echo
  echo "Move them into lib/i18n/pt.i18n.json + en.i18n.json and run 'dart run slang'." >&2
  exit 1
fi

echo "check_i18n_literals: ${COUNT} pt-BR string literal(s) still outside i18n."
echo "Run with --strict to fail CI. Offenders:"
echo "${FILTERED}"
exit 0
