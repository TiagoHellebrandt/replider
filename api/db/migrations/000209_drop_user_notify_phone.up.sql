-- The WhatsApp approval-command flow (M-approval: OFERTAS / APROVAR / PULAR
-- texted at the account's own WhatsApp number) is removed. The PWA is the
-- mobile approval surface, and Meta starts charging for service messages in
-- Oct/2026 — a per-message bill for a convenience the app already covers.
-- With the feature's code gone this column would have no runtime writer,
-- which is exactly the dangling-middle shape the CI gate exists to refuse.
-- ALLOW-DESTRUCTIVE: the column had no runtime writer once the WhatsApp
-- approval commands were removed; already applied in production (2026-08-24).
DROP INDEX IF EXISTS idx_users_notify_phone_per_account;
ALTER TABLE users DROP COLUMN IF EXISTS notify_phone;
