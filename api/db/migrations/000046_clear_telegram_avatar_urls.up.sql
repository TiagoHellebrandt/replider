-- Stale contact_avatar_url rows from the pre-proxy era embed the
-- Telegram bot token directly in the URL
-- (https://api.telegram.org/file/bot<TOKEN>/...). Null them out so the
-- CSP block on the browser goes away and the next inbound re-populates
-- with a proxied URL (api.replider.com/v1/telegram/file?...). Also
-- reset contact_avatar_fetched_at so the 24h refresh window fires
-- immediately on the next message.
UPDATE conversations
SET contact_avatar_url = NULL,
    contact_avatar_fetched_at = NULL
WHERE contact_avatar_url LIKE 'https://api.telegram.org/%';
