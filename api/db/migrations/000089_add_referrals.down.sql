DROP TABLE IF EXISTS referrals;
DROP INDEX IF EXISTS idx_accounts_referral_code;
ALTER TABLE accounts DROP COLUMN IF EXISTS referred_by_account_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS referral_code;
