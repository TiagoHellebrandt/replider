DROP INDEX IF EXISTS idx_audit_logs_account_created;
ALTER TABLE audit_logs
    ALTER COLUMN entity_id TYPE CHAR(26);
