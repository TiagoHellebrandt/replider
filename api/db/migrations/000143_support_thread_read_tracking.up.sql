-- US-004 (canal-interno-cs): read tracking + email-fallback idempotency
-- for internal CS threads. Both columns are meaningful ONLY on internal
-- threads (customer_account_id IS NOT NULL) and stay NULL everywhere else.
--
-- support_last_read_at        — stamped whenever the thread-owner user
--                               opens (GET) or writes to (POST) their
--                               support thread.
-- support_fallback_email_at   — stamped when the unread-fallback email
--                               is sent. One email per "unread window":
--                               due only while fallback <= last_read
--                               (both NULL-coalesced), so subsequent
--                               unread messages inside the same window
--                               never re-spam, and reading the thread
--                               re-arms the fallback.
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS support_last_read_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS support_fallback_email_at TIMESTAMPTZ NULL;
