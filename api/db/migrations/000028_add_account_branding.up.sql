-- Account-level branding + locale defaults.
--
--  * logo_url: public GCS URL of the workspace logo. Shown in the user
--    account chip + future customer-facing surfaces (email headers,
--    webchat widget). Null = initials fallback.
--
--  * timezone: IANA name (e.g. "America/Sao_Paulo") used as the
--    default when creating new agents; existing per-agent timezones
--    are unaffected. Required so the onboarding never asks twice.
--    Defaults to America/Sao_Paulo because ~99% of our signups are
--    Brazilian SMBs — can edit from the Account screen.
ALTER TABLE accounts
  ADD COLUMN logo_url text NULL,
  ADD COLUMN timezone text NOT NULL DEFAULT 'America/Sao_Paulo';
