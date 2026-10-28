-- US-003: personal prospecting link for platform-account sellers.
-- users.seller_ref_code backs replider.com/register?ref=SELLERCODE — a
-- signup carrying a valid code stamps accounts.seller_user_id +
-- lead_source='seller_ref' (the 50% commission track).
-- Distinct namespace from accounts.referral_code (US-174 customer referral).
ALTER TABLE users ADD COLUMN IF NOT EXISTS seller_ref_code TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_seller_ref_code
    ON users(seller_ref_code) WHERE seller_ref_code IS NOT NULL;
