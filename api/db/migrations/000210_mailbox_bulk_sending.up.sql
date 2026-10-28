-- Hybrid sending (2026-08-25): one-by-one approvals ALWAYS leave through the
-- customer's own mailbox (warmup caps intact); BATCH sends require the domain
-- registered and verified on the platform's bulk provider (Resend), so the
-- per-message cost only exists for accounts that asked for volume.
--
-- bulk_domain_id  — the provider's id for this mailbox's domain (written when
--                   the admin enables volume sending on the channel).
-- bulk_verified_at — stamped when the provider reports the domain's records
--                    verified. Forward-only, like dns_verified_at.
ALTER TABLE email_mailboxes
    ADD COLUMN IF NOT EXISTS bulk_domain_id TEXT,
    ADD COLUMN IF NOT EXISTS bulk_verified_at TIMESTAMPTZ;
