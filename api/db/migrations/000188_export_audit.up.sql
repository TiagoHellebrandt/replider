-- Who exported what (§6.18, I3).
--
-- A commercial export is the customer list, the prices and the pipeline in one
-- file. A company that cannot answer "who downloaded that, and when?" has no
-- answer for a departing salesperson either — which is the reason this table
-- exists. Satisfying LGPD Art. 18's portability duty is a by-product of doing
-- that first thing properly, not the motivation.
CREATE TABLE IF NOT EXISTS export_audits (
    id         CHAR(26)    PRIMARY KEY,
    account_id CHAR(26)    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    -- No FK to users: this row has to survive the person leaving, and "who
    -- exported the customer list" is precisely the question asked afterwards.
    -- The email is stored rather than joined for the same reason.
    user_id    CHAR(26)    NOT NULL,
    user_email TEXT        NOT NULL DEFAULT '',

    entity     TEXT        NOT NULL,
    format     TEXT        NOT NULL,
    -- The screen's active filters, rendered deterministically (sorted) so two
    -- identical exports produce two identical audit lines.
    filters    TEXT        NOT NULL DEFAULT '',

    rows       INTEGER     NOT NULL DEFAULT 0,
    -- The file hit the row ceiling. Recorded so an auditor reading "50000" knows
    -- it was not the whole table.
    truncated  BOOLEAN     NOT NULL DEFAULT FALSE,

    ip         TEXT        NOT NULL DEFAULT '',
    user_agent TEXT        NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The admin read: this account's exports, newest first.
CREATE INDEX IF NOT EXISTS idx_export_audits_account
    ON export_audits (account_id, created_at DESC);
