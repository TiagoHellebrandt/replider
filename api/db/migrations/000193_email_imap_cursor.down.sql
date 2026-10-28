-- Dropping this loses every poller's position. The next poll re-anchors to the
-- mailbox's current top, so nothing already read is re-imported — but anything
-- that arrived while the table was gone is skipped, because the anchor is the
-- newest message rather than the oldest unread one.
DROP TABLE IF EXISTS email_imap_cursors;
