-- Team activity tracking.
--
--  * invite_sent_at: stamped when an admin invites a new member (or
--    re-invites them). The Team screen renders a "Pending" badge until
--    the user verifies their email; admins also see a "Resend invite"
--    action that bumps this timestamp.
--
--  * last_seen_at: stamped by the auth middleware on every JWT-auth'd
--    request so admins can tell who's actually using the workspace
--    ("Active 3 days ago") vs. who to deactivate. Nullable because
--    brand-new invited users haven't logged in yet.
ALTER TABLE users
  ADD COLUMN invite_sent_at timestamptz NULL,
  ADD COLUMN last_seen_at   timestamptz NULL;
