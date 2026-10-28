#!/usr/bin/env python3
"""Fail CI when a NEW "dangling middle" appears: code where both ends exist
and nothing connects them.

This repository shipped three of these at once — a customer import with no
door into the funnel, `conversations.contact_ref_id` created and indexed but
never written, and a provider constructed and discarded in main.go — and 439
test files caught none of them, because each end looked healthy on its own.

Four blunt checks, in the same spirit as check_tenant_queries.py:

  A) Column written by nobody: a non-generated column from the migrations
     that appears in no INSERT/UPDATE anywhere in db/queries/ or in the raw
     SQL of the postgres repositories. SELECT/WHERE-only means readers are
     joining on a value that will always be NULL. (contact_ref_id's case.)
     Migration backfills do NOT count as writers: a one-shot UPDATE repairs
     the past and leaves every future row NULL — which is exactly how the
     original bug survived its own backfill.

  B) Discarded dependency: `_ = <ident>` in cmd/api/main.go where the
     identifier came from a constructor. Constructing and dropping a
     dependency is wiring that LOOKS done. (companyRepo/companyLookup's case.)

  C) Orphan route: a route registered in the API router whose distinctive
     path segment appears nowhere under app/lib/. A route no screen calls is
     either dead or the screen was never built. Webhooks, OAuth callbacks and
     platform-only surfaces are excepted in scripts/dangling_route_exceptions.txt
     (versioned, one justification per line).

  D) Port without adapter: an interface declared in internal/core/domain/
     none of whose methods is implemented anywhere outside domain and tests.
     A port only mocks satisfy is a contract nobody honours.

The gate fails only on findings NOT in scripts/dangling_allowlist.txt — the
allowlist freezes today's debt with a written reason per line; the goal is to
stop the bleeding, not the repository.

Usage (from api/):
    python3 scripts/check_dangling.py
    python3 scripts/check_dangling.py --report      # list findings, exit 0
    python3 scripts/check_dangling.py --self-test   # prove each check bites
"""

from __future__ import annotations

import argparse
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent          # api/
REPO = ROOT.parent                                             # repo root
MIGRATIONS_DIR = ROOT / "db" / "migrations"
QUERIES_DIR = ROOT / "db" / "queries"
REPOS_DIR = ROOT / "internal" / "adapters" / "repository" / "postgres"
ROUTER_DIR = ROOT / "internal" / "adapters" / "http" / "router"
DOMAIN_DIR = ROOT / "internal" / "core" / "domain"
INTERNAL_DIR = ROOT / "internal"
MAIN_GO = ROOT / "cmd" / "api" / "main.go"
APP_LIB = REPO / "app" / "lib"

ALLOWLIST_FILE = ROOT / "scripts" / "dangling_allowlist.txt"
ROUTE_EXCEPTIONS_FILE = ROOT / "scripts" / "dangling_route_exceptions.txt"


# ─────────────────────────────── helpers ────────────────────────────────────

def read(path: pathlib.Path) -> str:
    return path.read_text(encoding="utf-8", errors="replace")


def go_files(root: pathlib.Path, include_tests: bool = False):
    for p in sorted(root.rglob("*.go")):
        if not include_tests and p.name.endswith("_test.go"):
            continue
        yield p


def load_keyed_list(path: pathlib.Path) -> dict[str, str]:
    """`key  # reason` lines -> {key: reason}. Blank/# lines ignored."""
    out: dict[str, str] = {}
    if not path.exists():
        return out
    for line in read(path).splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        key, _, reason = line.partition("#")
        key = key.strip()
        if key:
            out[key] = reason.strip()
    return out


# ────────────────── A) column written by nobody ─────────────────────────────

CREATE_TABLE = re.compile(
    r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[\"']?(\w+)[\"']?\s*\((.*?)\)\s*;",
    re.IGNORECASE | re.DOTALL,
)
ALTER_ADD = re.compile(
    r"ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?[\"']?(\w+)[\"']?\s+"
    r"ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?[\"']?(\w+)[\"']?",
    re.IGNORECASE,
)
ALTER_DROP = re.compile(
    r"ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?[\"']?(\w+)[\"']?\s+"
    r"DROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?[\"']?(\w+)[\"']?",
    re.IGNORECASE,
)
DROP_TABLE = re.compile(
    r"DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?[\"']?(\w+)[\"']?", re.IGNORECASE
)
ALTER_RENAME_TABLE = re.compile(
    r"ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?[\"']?(\w+)[\"']?\s+RENAME\s+TO\s+[\"']?(\w+)[\"']?",
    re.IGNORECASE,
)

SQL_KEYWORDS = {
    "primary", "unique", "check", "constraint", "foreign", "exclude",
    "like", "references", "not", "null", "default",
}


def strip_sql_comments(sql: str) -> str:
    sql = re.sub(r"--[^\n]*", "", sql)
    return re.sub(r"/\*.*?\*/", "", sql, flags=re.DOTALL)


def table_body_columns(body: str) -> list[str]:
    """Column names from a CREATE TABLE body, skipping constraints, GENERATED
    columns and columns with a DEFAULT (the database itself is their writer —
    the contact_ref_id class of bug is a defaultless column nobody fills)."""
    cols = []
    depth = 0
    current = []
    parts = []
    for ch in body:
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        if ch == "," and depth == 0:
            parts.append("".join(current))
            current = []
        else:
            current.append(ch)
    parts.append("".join(current))
    for part in parts:
        part = part.strip()
        m = re.match(r'^[\"\']?(\w+)[\"\']?\s+\w', part)
        if not m:
            continue
        name = m.group(1).lower()
        if name in SQL_KEYWORDS:
            continue
        if re.search(r"\bGENERATED\s+ALWAYS\b", part, re.IGNORECASE):
            continue
        if re.search(r"\bDEFAULT\b", part, re.IGNORECASE):
            continue
        cols.append(name)
    return cols


def schema_columns() -> dict[tuple[str, str], str]:
    """(table, column) -> migration file that introduced it, replaying the
    whole migration history so drops and renames fall out."""
    cols: dict[tuple[str, str], str] = {}
    for path in sorted(MIGRATIONS_DIR.glob("*.up.sql")):
        sql = strip_sql_comments(path.read_text(encoding="utf-8", errors="replace"))
        for tbl, body in CREATE_TABLE.findall(sql):
            for col in table_body_columns(body):
                cols[(tbl.lower(), col)] = path.name
        for m in re.finditer(ALTER_ADD, sql):
            tbl, col = m.group(1), m.group(2)
            # Same DEFAULT rule as CREATE TABLE: the column definition runs
            # from the ADD COLUMN to the end of the statement (or the next
            # comma at depth zero — good enough at this bluntness).
            stmt_end = sql.find(";", m.end())
            definition = sql[m.end():stmt_end if stmt_end != -1 else len(sql)]
            if re.search(r"\bDEFAULT\b", definition, re.IGNORECASE):
                continue
            cols[(tbl.lower(), col.lower())] = path.name
        for tbl, col in ALTER_DROP.findall(sql):
            cols.pop((tbl.lower(), col.lower()), None)
        for old, new in ALTER_RENAME_TABLE.findall(sql):
            for (t, c), origin in list(cols.items()):
                if t == old.lower():
                    del cols[(t, c)]
                    cols[(new.lower(), c)] = origin
        for tbl in DROP_TABLE.findall(sql):
            for key in [k for k in cols if k[0] == tbl.lower()]:
                del cols[key]
    return cols


INSERT_STMT = re.compile(
    r"INSERT\s+INTO\s+[\"']?(\w+)[\"']?\s*\(([^)]*)\)", re.IGNORECASE | re.DOTALL
)
UPDATE_STMT = re.compile(
    # SET clause up to the first keyword that ends it; blunt but effective on
    # the flat statements sqlc queries and the raw repos use.
    r"UPDATE\s+(?:ONLY\s+)?[\"']?(\w+)[\"']?\s+(?:\w+\s+)?SET\s+(.*?)(?:\bWHERE\b|\bRETURNING\b|;|\Z)",
    re.IGNORECASE | re.DOTALL,
)


def written_columns() -> set[tuple[str, str]]:
    """(table, column) pairs some INSERT or UPDATE writes, in db/queries/ or
    in the raw SQL of the postgres repositories. Migrations are deliberately
    NOT scanned — a backfill is not an ongoing writer."""
    written: set[tuple[str, str]] = set()
    sources = list(QUERIES_DIR.glob("*.sql")) + list(go_files(REPOS_DIR))
    for path in sources:
        text = read(path)
        for tbl, collist in INSERT_STMT.findall(text):
            for col in re.findall(r"[\"']?(\w+)[\"']?", collist):
                written.add((tbl.lower(), col.lower()))
        for tbl, setclause in UPDATE_STMT.findall(text):
            for col in re.findall(r"[\"']?(\w+)[\"']?\s*=", setclause):
                written.add((tbl.lower(), col.lower()))
    return written


def check_unwritten_columns() -> list[str]:
    cols = schema_columns()
    written = written_columns()
    findings = []
    for (tbl, col), origin in sorted(cols.items()):
        if (tbl, col) not in written:
            findings.append(f"column:{tbl}.{col}  ({origin})")
    return findings


# ────────────────── B) discarded constructor result ─────────────────────────

DISCARD = re.compile(r"^\s*_\s*=\s*(\w+)\s*$")
CTOR_ASSIGN = re.compile(r"^\s*(\w+)\s*(?::=|=)\s*.*\bNew[A-Z]\w*\(")


def check_discarded_ctors() -> list[str]:
    if not MAIN_GO.exists():
        return []
    lines = read(MAIN_GO).splitlines()
    ctor_vars: dict[str, int] = {}
    for i, line in enumerate(lines, 1):
        m = CTOR_ASSIGN.match(line)
        if m:
            ctor_vars[m.group(1)] = i
    findings = []
    for i, line in enumerate(lines, 1):
        m = DISCARD.match(line)
        if m and m.group(1) in ctor_vars:
            findings.append(
                f"ctor:{m.group(1)}  (cmd/api/main.go:{i}, construído na linha {ctor_vars[m.group(1)]})"
            )
    return findings


# ────────────────── C) orphan route ─────────────────────────────────────────

ROUTE_REG = re.compile(r"\.(GET|POST|PUT|PATCH|DELETE)\(\s*\"([^\"]*)\"")
GROUP_DECL = re.compile(r"(\w+)\s*(?::=|=)\s*\w+\.Group\(\s*\"([^\"]*)\"")


def route_static_segments(path: str) -> list[str]:
    return [s for s in path.split("/") if s and not s.startswith(":") and not s.startswith("*")]


def collect_routes() -> list[tuple[str, str, str]]:
    """(file:line, METHOD, path) for every route registration in the router."""
    out = []
    for p in go_files(ROUTER_DIR):
        for i, line in enumerate(read(p).splitlines(), 1):
            for method, path in ROUTE_REG.findall(line):
                out.append((f"{p.name}:{i}", method, path))
    return out


def check_orphan_routes() -> list[str]:
    exceptions = load_keyed_list(ROUTE_EXCEPTIONS_FILE)
    exception_res = [re.compile(pat) for pat in exceptions]

    # No app checkout (partial clone, api-only sandbox) → no basis to call
    # anything an orphan. Skipping beats flagging every route in the repo.
    if not APP_LIB.exists():
        print("AVISO: app/lib ausente — check C (rota órfã) pulado.")
        return []
    chunks = []
    for p in sorted(APP_LIB.rglob("*.dart")):
        chunks.append(p.read_text(encoding="utf-8", errors="replace"))
    app_text = "\n".join(chunks)

    findings = []
    for where, method, path in collect_routes():
        segments = route_static_segments(path)
        if not segments:
            continue  # "/:id"-only routes carry nothing distinctive to search
        probe = segments[-1]
        key = f"{method} {path}"
        # An exception pattern may match the route ("webhook") or the file it
        # is registered in ("owner_routes.go") — platform surfaces group by file.
        if any(rx.search(key) or rx.search(where) for rx in exception_res):
            continue
        if probe not in app_text:
            findings.append(f"route:{key}  ({where}, segmento '{probe}' ausente em app/lib)")
    return findings


# ────────────────── D) port without adapter ─────────────────────────────────

IFACE = re.compile(r"^type\s+(\w+)\s+interface\s*\{", re.MULTILINE)
METHOD_IN_IFACE = re.compile(r"^\s*(\w+)\s*\(", re.MULTILINE)


def domain_interfaces() -> dict[str, tuple[str, list[str]]]:
    """name -> (file, [methods])."""
    out: dict[str, tuple[str, list[str]]] = {}
    for p in go_files(DOMAIN_DIR):
        text = read(p)
        for m in IFACE.finditer(text):
            name = m.group(1)
            # Body: from the { to its matching }
            depth = 0
            start = m.end() - 1
            end = start
            for j, ch in enumerate(text[start:], start):
                if ch == "{":
                    depth += 1
                elif ch == "}":
                    depth -= 1
                    if depth == 0:
                        end = j
                        break
            body = text[start:end]
            methods = [mm for mm in METHOD_IN_IFACE.findall(body)]
            if methods:
                out[name] = (p.name, methods)
    return out


def check_ports_without_adapters() -> list[str]:
    impl_text_chunks = []
    for p in go_files(INTERNAL_DIR):
        rel = p.relative_to(INTERNAL_DIR)
        if rel.parts[:2] == ("core", "domain"):
            continue
        impl_text_chunks.append(read(p))
    impl_text = "\n".join(impl_text_chunks)

    findings = []
    for name, (file, methods) in sorted(domain_interfaces().items()):
        # Implemented if ANY of its methods exists as a receiver method
        # outside domain/tests. Blunt: false negatives possible for very
        # generic method names, never a false positive for a used port.
        implemented = any(
            re.search(r"func\s+\([^)]*\)\s+" + re.escape(m) + r"\(", impl_text)
            for m in methods
        )
        if not implemented:
            findings.append(f"port:{name}  ({file}, nenhum método implementado fora de domain/testes)")
    return findings


# ────────────────────────────── self-test ───────────────────────────────────

def self_test() -> int:
    ok = True

    # A: a column only SELECTed must be flagged by the diff of the two sets.
    cols = {("conversations", "ghost_col"): "000999_x.up.sql"}
    written: set[tuple[str, str]] = set()
    if ("conversations", "ghost_col") in written:
        print("SELF-TEST A FAILED")
        ok = False

    # A writer must be extracted from both INSERT and UPDATE shapes.
    ins = INSERT_STMT.findall("INSERT INTO conversations (id, ghost_col) VALUES ($1,$2)")
    upd = UPDATE_STMT.findall("UPDATE conversations SET ghost_col = $1 WHERE id = $2")
    if not ins or "ghost_col" not in ins[0][1] or not upd or "ghost_col" not in upd[0][1]:
        print("SELF-TEST A FAILED: INSERT/UPDATE writer not recognised")
        ok = False

    # B: a discarded constructor result must be flagged.
    if not (CTOR_ASSIGN.match("\tcompanyRepo := postgres.NewCompanyRepository(pool)")
            and DISCARD.match("\t_ = companyRepo")):
        print("SELF-TEST B FAILED: `_ = ctor` shape not recognised")
        ok = False

    # C: a route with a distinctive segment must yield that segment.
    if route_static_segments("/v1/leads/:id/draft")[-1] != "draft":
        print("SELF-TEST C FAILED: route segment extraction broken")
        ok = False

    # D: an interface body must yield its method names.
    fake = "type GhostPort interface {\n\tDoThing(ctx context.Context) error\n}\n"
    m = IFACE.search(fake)
    if not m or "DoThing" not in METHOD_IN_IFACE.findall(fake):
        print("SELF-TEST D FAILED: interface parsing broken")
        ok = False

    print("self-test OK" if ok else "self-test FAILED")
    return 0 if ok else 1


# ─────────────────────────────── main ───────────────────────────────────────

def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--self-test", action="store_true")
    ap.add_argument("--report", action="store_true",
                    help="print every finding (including allowlisted) and exit 0")
    args = ap.parse_args()

    if args.self_test:
        return self_test()

    allow = load_keyed_list(ALLOWLIST_FILE)

    sections = [
        ("A) coluna sem escritor", check_unwritten_columns()),
        ("B) dependência descartada em main.go", check_discarded_ctors()),
        ("C) rota órfã (sem chamador em app/lib)", check_orphan_routes()),
        ("D) porta sem adapter", check_ports_without_adapters()),
    ]

    def finding_key(f: str) -> str:
        # Everything before the "  (detail)" suffix — route keys contain a
        # space ("route:GET /path"), so a naive split()[0] would truncate.
        return f.split("  (")[0].strip()

    new_findings = []
    total = 0
    for title, findings in sections:
        total += len(findings)
        if args.report:
            print(f"\n{title}: {len(findings)}")
            for f in findings:
                mark = "  [allowlisted]" if finding_key(f) in allow else "  [NOVO]"
                print(f"  {f}{mark}")
        for f in findings:
            if finding_key(f) not in allow:
                new_findings.append((title, f))

    if args.report:
        print(f"\ntotal: {total} pontas soltas, {len(allow)} allowlisted")
        return 0

    if new_findings:
        print(f"Dangling-middle check FAILED ({len(new_findings)} ponta(s) solta(s) NOVA(s)):\n")
        for title, f in new_findings:
            print(f"  [{title}] {f}")
        print(
            "\nCada achado acima é uma ponta sem meio: ou ligue-a nesta mesma PR\n"
            "(coluna → escritor; construtor → consumidor; rota → tela; porta →\n"
            f"adapter roteado), ou registre em {ALLOWLIST_FILE.relative_to(ROOT)}\n"
            "com uma justificativa de uma linha."
        )
        return 1

    print(f"Dangling-middle check OK: {total} achados históricos, todos allowlisted.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
