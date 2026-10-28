-- cid: resolution for the email "view original" (2026-08-26).
--
-- Inline email images (a photo pasted into Gmail, a signature logo) arrive
-- as attachments referenced from the HTML by `cid:<content-id>`. The id was
-- parsed and then DROPPED, so the stored original rendered every embedded
-- image broken — in both the sandboxed inline view and the external one.
-- With the id stored, the original handler rewrites cid: references into
-- data: URIs from the attachment bytes: embedded images render inside the
-- strict CSP without one byte fetched from the network.
ALTER TABLE message_attachments
    ADD COLUMN IF NOT EXISTS content_id TEXT;
