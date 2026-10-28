#!/usr/bin/env python3
"""Fail CI when a sqlc query touches a multi-tenant table without scoping to
an account (US-F013 / V17).

Tenant isolation in this codebase is enforced by every query carrying an
`account_id` predicate. That is a convention, and conventions rot: one
forgotten WHERE clause is a cross-tenant data leak that no test necessarily
catches, because the fixture usually has a single account.

The checker is deliberately blunt:

  * a table is TENANT-SCOPED if any migration gives it an `account_id` column;
  * a query that reads/writes such a table must mention `account_id`;
  * anything that legitimately cannot (joins that inherit scoping from a
    parent, platform-wide cron reads) is allowlisted BY NAME with a written
    reason, so the exception is a review artifact instead of an oversight.

Usage (from api/):
    python3 scripts/check_tenant_queries.py
    python3 scripts/check_tenant_queries.py --self-test   # proves it bites
"""

from __future__ import annotations

import argparse
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
QUERIES_DIR = ROOT / "db" / "queries"
MIGRATIONS_DIR = ROOT / "db" / "migrations"
ALLOWLIST_FILE = ROOT / "scripts" / "tenant_query_allowlist.txt"

# `-- name: GetFoo :one`
QUERY_HEADER = re.compile(r"^--\s*name:\s*(\w+)\s*:(\w+)", re.MULTILINE)

CREATE_TABLE = re.compile(
    r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[\"']?(\w+)[\"']?\s*\((.*?)\);",
    re.IGNORECASE | re.DOTALL,
)
ALTER_ADD_ACCOUNT = re.compile(
    r"ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?[\"']?(\w+)[\"']?\s+ADD\s+COLUMN\s+"
    r"(?:IF\s+NOT\s+EXISTS\s+)?account_id",
    re.IGNORECASE,
)

# Table references we care about. Enough to catch the shapes sqlc queries use;
# a missed reference is a false NEGATIVE for that query only, never a false
# positive that blocks an innocent PR.
TABLE_REF = re.compile(
    r"\b(?:FROM|JOIN|UPDATE|INSERT\s+INTO|DELETE\s+FROM)\s+(?:ONLY\s+)?[\"']?(\w+)[\"']?",
    re.IGNORECASE,
)


def tenant_tables() -> set[str]:
    """Tables that carry an account_id column, per the migration history."""
    tables: set[str] = set()
    for path in sorted(MIGRATIONS_DIR.glob("*.up.sql")):
        sql = path.read_text(encoding="utf-8", errors="replace")
        for name, body in CREATE_TABLE.findall(sql):
            if re.search(r"\baccount_id\b", body, re.IGNORECASE):
                tables.add(name.lower())
        for name in ALTER_ADD_ACCOUNT.findall(sql):
            tables.add(name.lower())
    return tables


def load_allowlist() -> dict[str, str]:
    """Query name -> reason. Blank lines and `#` comments ignored."""
    allow: dict[str, str] = {}
    if not ALLOWLIST_FILE.exists():
        return allow
    for line in ALLOWLIST_FILE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        name, _, reason = line.partition("#")
        name = name.strip()
        if name:
            allow[name] = reason.strip()
    return allow


def split_queries(sql: str) -> list[tuple[str, str, str]]:
    """(name, kind, body) for every `-- name:` block in a file."""
    out: list[tuple[str, str, str]] = []
    matches = list(QUERY_HEADER.finditer(sql))
    for i, m in enumerate(matches):
        end = matches[i + 1].start() if i + 1 < len(matches) else len(sql)
        out.append((m.group(1), m.group(2), sql[m.end():end]))
    return out


def strip_comments(body: str) -> str:
    return re.sub(r"--[^\n]*", "", body)


def offending_tables(body: str, tenant: set[str]) -> list[str]:
    """Tenant tables referenced by a query that never mentions account_id."""
    stmt = strip_comments(body)
    if re.search(r"\baccount_id\b", stmt, re.IGNORECASE):
        return []
    hit = {t.lower() for t in TABLE_REF.findall(stmt)} & tenant
    return sorted(hit)


def check(paths: list[pathlib.Path], tenant: set[str], allow: dict[str, str]):
    violations = []
    checked = 0
    for path in paths:
        sql = path.read_text(encoding="utf-8", errors="replace")
        for name, _kind, body in split_queries(sql):
            checked += 1
            if name in allow:
                continue
            tables = offending_tables(body, tenant)
            if tables:
                violations.append((path.name, name, tables))
    return checked, violations


SELF_TEST_SQL = """
-- name: LeakEveryTenantsLeads :many
SELECT * FROM leads WHERE status = $1;
"""


def self_test(tenant: set[str]) -> int:
    """Prove the checker bites: a deliberately unscoped query must be flagged,
    and the same query WITH account_id must pass."""
    bad = split_queries(SELF_TEST_SQL)[0]
    if not offending_tables(bad[2], tenant):
        print("SELF-TEST FAILED: unscoped query on `leads` was not flagged")
        return 1
    good = split_queries(
        "-- name: Ok :many\nSELECT * FROM leads WHERE account_id = $1;\n"
    )[0]
    if offending_tables(good[2], tenant):
        print("SELF-TEST FAILED: account-scoped query was flagged")
        return 1
    print("self-test OK: unscoped query flagged, scoped query accepted")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--self-test", action="store_true",
                    help="verify the checker flags a deliberately broken query")
    args = ap.parse_args()

    tenant = tenant_tables()
    if not tenant:
        print("ERROR: no tenant tables found — is db/migrations/ present?")
        return 2

    if args.self_test:
        return self_test(tenant)

    allow = load_allowlist()
    paths = sorted(QUERIES_DIR.glob("*.sql"))
    checked, violations = check(paths, tenant, allow)

    if violations:
        print(f"Tenant-scope check FAILED ({len(violations)} of {checked} queries):\n")
        for file, name, tables in violations:
            print(f"  {file}: {name} touches {', '.join(tables)} without account_id")
        print(
            "\nEvery query against a multi-tenant table must filter by account_id.\n"
            "If this query is legitimately scoped some other way (a parent id that\n"
            "is itself account-scoped, or a platform-wide job), add it to\n"
            f"{ALLOWLIST_FILE.relative_to(ROOT)} with a one-line reason."
        )
        return 1

    print(f"Tenant-scope check OK: {checked} queries, "
          f"{len(tenant)} tenant tables, {len(allow)} allowlisted.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
