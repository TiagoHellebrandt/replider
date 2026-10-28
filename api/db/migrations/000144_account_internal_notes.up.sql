-- Canal interno US-007: one shared internal note per CUSTOMER account,
-- editable by the platform seller/admin, shown on the side panel of any
-- internal thread of that account. Who/when live on the row itself
-- (visible audit); the full trail goes to audit_logs on every edit.
CREATE TABLE IF NOT EXISTS account_internal_notes (
    account_id CHAR(26) PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    note TEXT NOT NULL DEFAULT '',
    updated_by CHAR(26) REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
