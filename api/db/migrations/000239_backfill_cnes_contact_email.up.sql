-- The address the approval queue has been waiting for, for the hospitals
-- already in the funnel.
--
-- The CNES door copied the registry's name and phone onto the contact and
-- left the e-mail behind. Measured on 2026-09-17: 0 of one account's 73
-- contacts carried an address, 48 of its 60 hunted hospitals had one
-- published, and 50 drafts sat approved-ready with nowhere to send them.
--
-- The runtime writer now carries it (usecases.cnesContactDraft), but it only
-- runs for establishments the sweep has NOT already put in the funnel — which
-- is exactly the set that is stuck. This fills those once.
--
-- Deliberately narrow. Only contacts that
--   * have no e-mail at all (a value somebody typed always wins), and
--   * are the contact of a cnes_park lead whose enrichment names the CNES id,
--     so the establishment is the one THIS card was built from — never a
--     name or phone guess.
--
-- The address is NOT registered in contact_identities, for the same reason
-- the writer refuses to key on it: 3.208 published addresses in Mato Grosso
-- are only 2.670 distinct, and thirty-three establishments share
-- saude@rondonopolis.mt.gov.br. It reaches the organization; it does not
-- identify this hospital.
UPDATE contacts c
SET email = lower(trim(e.email)),
    updated_at = NOW()
FROM leads l
JOIN cnes_establishments e ON e.cnes = l.enrichment->>'cnes'
WHERE l.contact_id = c.id
  AND l.account_id = c.account_id
  AND l.source = 'cnes_park'
  AND (c.email IS NULL OR trim(c.email) = '')
  AND e.email IS NOT NULL
  AND position('@' IN e.email) > 1;
