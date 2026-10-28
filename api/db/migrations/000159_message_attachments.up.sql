-- Inbound email attachments (US-M005 follow-up).
--
-- A separate table rather than more columns on messages: one email carries N
-- files, and messages.media_url is a single value shaped for WhatsApp media.
-- Each row also carries its own scan verdict, because a message with three
-- attachments can legitimately have one infected and two clean.

CREATE TABLE IF NOT EXISTS message_attachments (
    id              CHAR(26)     PRIMARY KEY,
    message_id      CHAR(26)     NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    -- Denormalised so every read can be scoped without joining through
    -- conversations. Attachment downloads are the one place where a missed
    -- tenant check hands a customer's contract to another account.
    account_id      CHAR(26)     NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

    filename        TEXT         NOT NULL,
    content_type    TEXT         NOT NULL DEFAULT '',
    size_bytes      BIGINT       NOT NULL DEFAULT 0,
    -- GCS object path. The bucket is private; reads go out as signed URLs
    -- generated on demand, same as every other stored file.
    object_path     TEXT         NOT NULL,

    -- Inline furniture (signature logos, tracking pixels) is kept but not
    -- counted as "an attachment" in the UI.
    is_inline       BOOLEAN      NOT NULL DEFAULT FALSE,

    -- DEFAULT 'pending', never 'clean': a crash between storing the file and
    -- scanning it must leave the row unknown, not safe. Only 'clean' is
    -- servable, so the failure mode of every bug here is a file that will
    -- not download rather than one that should not have.
    scan_status     TEXT         NOT NULL DEFAULT 'pending'
        CHECK (scan_status IN ('pending', 'clean', 'infected', 'skipped', 'failed')),
    scan_signature  TEXT         NOT NULL DEFAULT '',
    scanned_at      TIMESTAMPTZ,

    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- The read path: every attachment of one message, in insertion order.
CREATE INDEX IF NOT EXISTS idx_message_attachments_message
    ON message_attachments (message_id, created_at);

-- Drives the rescan worker and the "quantos ficaram sem varredura" answer.
CREATE INDEX IF NOT EXISTS idx_message_attachments_unscanned
    ON message_attachments (created_at)
    WHERE scan_status IN ('pending', 'failed');
