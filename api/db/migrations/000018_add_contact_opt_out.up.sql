-- LGPD / CAN-SPAM compliance: track when a contact explicitly requests to
-- stop receiving messages. Broadcast campaigns must filter these out.
-- Single timestamp column (NULL = active, non-null = opted out AND when).
ALTER TABLE contacts ADD COLUMN opted_out_at TIMESTAMPTZ;

-- Partial index: speeds up the "exclude opted-out" filter during campaign
-- creation without penalizing the (much larger) set of active contacts.
CREATE INDEX idx_contacts_opted_out
    ON contacts(account_id, external_id)
    WHERE opted_out_at IS NOT NULL;
