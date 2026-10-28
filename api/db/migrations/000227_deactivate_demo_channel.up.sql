-- The demo seed (000005) creates a Telegram channel with a placeholder token
-- ("0000000000:DEMO_TOKEN_NOT_REAL"). It is is_active=true, so the boot-time
-- Telegram webhook re-registration loop tries it every start and logs an ERROR
-- ("Unauthorized: invalid token specified"). A channel with a fake token can
-- never work — deactivate it so the noise stops. The demo conversation/messages
-- are kept for the review/demo account.
UPDATE channels SET is_active = false, updated_at = NOW()
WHERE id LIKE '01DEMO_CHANNEL_%';
