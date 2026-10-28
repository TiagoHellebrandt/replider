# Database Migrations

Migrations live in `api/db/migrations/` as sequential `NNNNNN_name.{up,down}.sql`
pairs (golang-migrate). They run **automatically on every production deploy**
(the API container's `entrypoint.prod.sh` runs `migrate up` before serving).

## Migration linter (US-190)

Because forward migrations run unattended on deploy, a stray `DROP TABLE` or
`DROP COLUMN` in an `*.up.sql` file destroys customer data the moment the
container restarts — with no human in the loop. To make that impossible by
accident, CI runs `scripts/lint-migrations.sh`.

### What it checks

- Scans every `api/db/migrations/*.up.sql` for destructive DDL:
  `DROP TABLE` and `DROP COLUMN` (including `IF EXISTS` / `CASCADE` variants).
- A match **fails the build** unless the file contains an explicit opt-in
  marker comment:

  ```sql
  -- ALLOW-DESTRUCTIVE: <reason the data loss is intentional>
  ```

- `--` comments are stripped before matching, so a `DROP` mentioned in an
  explanatory comment never trips the linter — only real statements do.
- `*.down.sql` files are **not** scanned. A rollback is destructive by
  definition (it undoes the up), is run manually by an operator, and never
  executes automatically on deploy.

### Where it runs

The linter is a step in the `lint` job of `.github/workflows/deploy-api.yml`,
which runs on every PR that touches `api/**`. It also runs locally:

```bash
./scripts/lint-migrations.sh                 # default: api/db/migrations
./scripts/lint-migrations.sh path/to/dir     # lint a different directory
```

Exit code `0` = clean, `1` = an unmarked destructive statement was found.

### When to add the marker

Add `-- ALLOW-DESTRUCTIVE` only when the data loss is genuinely intended and
you have confirmed the column/table is unused or its data is being migrated
elsewhere. The marker is a deliberate, reviewable red flag in the diff —
prefer additive migrations (new columns, backfill, switch reads, drop later)
over destructive ones whenever possible.
