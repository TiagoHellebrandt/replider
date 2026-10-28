-- The seller's own WhatsApp number (E.164), so the approval queue can reach
-- her where she already lives — she texts the company's number "OFERTAS",
-- reads each draft, replies "APROVAR <código>" — without opening the app.
--
-- Unique per account among the set numbers: an inbound from this number on
-- the account's WhatsApp channel is treated as a COMMAND from this user, so
-- two users sharing one number would make the sender ambiguous.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS notify_phone TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_notify_phone_per_account
    ON users (account_id, notify_phone)
    WHERE notify_phone IS NOT NULL;
