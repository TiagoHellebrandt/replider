-- Un-hide documents wrongly flagged as message furniture (2026-08-26).
--
-- The MIME walk used to flag ANY part carrying a Content-ID as inline —
-- but Gmail stamps a Content-ID on regular attachments too, so contract
-- PDFs were flagged inline and the dashboard's attachment list (which
-- filters furniture out) never showed them. The parser now follows the
-- disposition; this backfill applies the same rule to rows already
-- written: only images stay inline.
UPDATE message_attachments
   SET is_inline = FALSE
 WHERE is_inline
   AND content_type NOT LIKE 'image/%';
