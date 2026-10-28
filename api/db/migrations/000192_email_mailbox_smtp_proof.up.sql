-- Two ways a sending identity can be proven, because there are two
-- transports and they are authenticated by different people.
--
-- `dns_verified_at` (000151) means OUR records are published — the SPF
-- include and DKIM CNAME of the volume provider we send through. That is the
-- only proof that matters when WE are the one talking to Gmail.
--
-- But the first transport a customer connects is their OWN mail provider
-- (Locaweb, Titan, Zoho, Google Workspace) over SMTP with their own
-- credentials. There, the provider already publishes SPF/DKIM for the domain
-- and signs the message itself; our records are irrelevant while we are not
-- the sender. Requiring them anyway is what made every cold email approval
-- refuse forever: `dns_verified_at` had no writer, so `DomainAuthenticated()`
-- was permanently false and the DNS screen could not lift it.
--
-- The honest proof for that transport is that the provider ACCEPTED an
-- authenticated SMTP session from us and relayed a message. So the test send
-- is the verification: no DNS, no guessing, and it fails loudly when the
-- password is wrong instead of on the customer's first real proposal.
ALTER TABLE email_mailboxes
    ADD COLUMN IF NOT EXISTS smtp_verified_at TIMESTAMPTZ;

COMMENT ON COLUMN email_mailboxes.smtp_verified_at IS
    'Last successful authenticated send through the customer own SMTP. Unlocks cold sending at the conservative per-mailbox cap, with no DNS. Cleared when the transport credentials change.';
