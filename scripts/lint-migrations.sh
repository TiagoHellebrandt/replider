#!/bin/sh
# Migration linter — guards against accidental data loss (US-190).
#
# Forward (`*.up.sql`) migrations run automatically on every prod deploy
# (see entrypoint.prod.sh). A `DROP TABLE` / `DROP COLUMN` slipped into an
# up migration therefore destroys customer data the moment the container
# restarts, with no review gate. This linter scans every `*.up.sql` for
# destructive DDL and fails unless the file explicitly opts in with an
#   -- ALLOW-DESTRUCTIVE
# marker comment, which forces the author to acknowledge the data loss in
# the diff and survives code review as a visible red flag.
#
# Down (`*.down.sql`) migrations are intentionally NOT scanned: a rollback
# is destructive by definition (it undoes the up), is run manually by an
# operator, and never executes automatically on deploy.
#
# Usage: ./scripts/lint-migrations.sh [migrations_dir]
#   migrations_dir defaults to api/db/migrations
#
# Exit codes: 0 = clean, 1 = unmarked destructive statement found.

set -eu

# Resolve the migrations directory relative to the repo root so the script
# works from CI (run at root) and from a developer shell alike.
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
MIGRATIONS_DIR="${1:-$REPO_ROOT/api/db/migrations}"

if [ ! -d "$MIGRATIONS_DIR" ]; then
  echo "::error::migrations directory not found: $MIGRATIONS_DIR" >&2
  exit 1
fi

# Destructive DDL patterns. `IF EXISTS`/`CASCADE` variants are still caught
# because the match is on the `DROP TABLE` / `DROP COLUMN` prefix.
DESTRUCTIVE_REGEX='DROP[[:space:]]+TABLE|DROP[[:space:]]+COLUMN'
MARKER='ALLOW-DESTRUCTIVE'

violations=0

for file in "$MIGRATIONS_DIR"/*.up.sql; do
  # Guard the glob in case the directory has no up migrations yet.
  [ -e "$file" ] || continue

  # Strip single-line `--` comments before matching so a DROP mentioned in
  # an explanatory comment doesn't trip the linter; only real statements do.
  stripped=$(sed 's/--.*$//' "$file")

  if printf '%s\n' "$stripped" | grep -qiE "$DESTRUCTIVE_REGEX"; then
    # Destructive statement present — require the opt-in marker (matched on
    # the RAW file so the marker lives in a `--` comment as intended).
    if ! grep -qi "$MARKER" "$file"; then
      echo "::error file=${file}::Destructive DDL (DROP TABLE/COLUMN) without an '-- ${MARKER}' marker. Add the marker comment to acknowledge intentional data loss, or remove the statement." >&2
      echo "  ✗ $(basename "$file")" >&2
      printf '%s\n' "$stripped" | grep -niE "$DESTRUCTIVE_REGEX" | sed 's/^/      /' >&2
      violations=$((violations + 1))
    fi
  fi
done

if [ "$violations" -gt 0 ]; then
  echo "" >&2
  echo "Found $violations destructive up-migration(s) without an '-- ${MARKER}' marker." >&2
  echo "Destructive forward migrations run automatically on deploy and can lose customer data." >&2
  exit 1
fi

echo "Migration linter: all up migrations clean (no unmarked DROP TABLE/COLUMN)."
