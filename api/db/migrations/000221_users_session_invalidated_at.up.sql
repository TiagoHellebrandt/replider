-- 2026-09-01 hardening: member removal must end the member's sessions.
-- A JWT lives 24h and the logout blacklist only knows tokens it was handed,
-- so removing someone from the team left their open dashboard working until
-- expiry. The auth middleware and the WebSocket first-frame auth now refuse
-- any token whose iat is not after this stamp. Written by DeactivateUser.
ALTER TABLE users ADD COLUMN IF NOT EXISTS session_invalidated_at TIMESTAMPTZ;
