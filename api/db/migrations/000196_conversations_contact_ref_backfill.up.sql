-- Backfill conversations.contact_ref_id.
--
-- The column and its index have existed since 000009 and nothing ever wrote
-- them. Three readers assumed otherwise:
--
--   * the lead panel's "open the conversation" lookup
--     (FindConversationByAccountContact),
--   * the lead timeline (LeadTimelineRepository.ListForContact),
--   * the opt-out guard on AutoResumeStuckConversations, whose
--     `NOT EXISTS (SELECT 1 FROM contacts ct WHERE ct.id = c.contact_ref_id
--     AND ct.opted_out_at IS NOT NULL)` was vacuously TRUE for every row —
--     i.e. the guard was dead and the bot could resume a thread with someone
--     who had asked it to stop.
--
-- From this migration on the write happens at creation time. This file repairs
-- the history so those readers work on the rows that already exist.
--
-- Two passes because there are two ways to recognise the person behind a
-- thread, and they disagree on rows written before the identity index existed:
--
--   1. contacts.external_id holds the sender exactly as the channel wrote it,
--      which is exactly what conversations.contact_id holds. Unique per
--      account (idx_contacts_account_external), so this pass is unambiguous.
--   2. contact_identities.value holds the NORMALISED form: '+5565...' for a
--      WhatsApp sender Meta delivered as bare digits, lowercased for email.
--      This pass catches the threads pass 1 misses for exactly that reason.
--
-- Anything neither pass recognises stays NULL. That is the honest answer: a
-- wrong contact_ref_id would attach one customer's messages to another
-- customer's lead, which is worse than an empty timeline.

-- Pass 1 — the raw identifier, matched verbatim.
UPDATE conversations conv
SET contact_ref_id = ct.id
FROM contacts ct
WHERE conv.contact_ref_id IS NULL
  AND conv.contact_id <> ''
  AND ct.account_id = conv.account_id
  AND ct.external_id = conv.contact_id;

-- Pass 2 — the normalised identifier, for the threads pass 1 could not match.
-- Written as a scalar subquery rather than UPDATE..FROM so a contact_id that
-- somehow matches two identity rows resolves deterministically (oldest wins)
-- instead of picking an arbitrary one.
UPDATE conversations conv
SET contact_ref_id = (
        SELECT ci.contact_id
          FROM contact_identities ci
         WHERE ci.account_id = conv.account_id
           AND ci.value IN (conv.contact_id,
                            '+' || conv.contact_id,
                            lower(conv.contact_id))
         ORDER BY ci.created_at, ci.id
         LIMIT 1
    )
WHERE conv.contact_ref_id IS NULL
  AND conv.contact_id <> ''
  AND EXISTS (
        SELECT 1
          FROM contact_identities ci
         WHERE ci.account_id = conv.account_id
           AND ci.value IN (conv.contact_id,
                            '+' || conv.contact_id,
                            lower(conv.contact_id))
    );
