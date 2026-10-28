-- The audit log was originally scoped to internal ULIDs only. LGPD audit
-- entries need to record the titular's external_id (phone number), which
-- is variable length. Relaxing to TEXT costs a few bytes per row and
-- unlocks auditing of anything — webhook tokens, OAuth IDs, etc.
ALTER TABLE audit_logs
    ALTER COLUMN entity_id TYPE TEXT;

-- Searchable index by (account_id, created_at DESC) so the admin listing
-- endpoint can paginate recent entries without a full table scan.
CREATE INDEX idx_audit_logs_account_created
    ON audit_logs(account_id, created_at DESC);
