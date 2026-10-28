-- ── LGPD export + anonymization queries ────────────────────────────────────
--
-- These all operate on a single (account_id, contact_external_id) scope.
-- They are called from the ExportContactUseCase / AnonymizeContactUseCase.
-- Every mutation is account-scoped so a tenant cannot mutate another
-- tenant's data even if an admin ID is compromised (defense-in-depth on
-- top of the handler-level ownership check).

-- name: LgpdListConversationsByContact :many
-- Lists all conversations in the account whose contact_id matches the
-- given external id (phone / chat_id). Uses contact_id on conversations
-- (which is the same externalID used to match inbound messages), NOT
-- contact_ref_id (which is the FK to the contacts row — that row itself
-- can be missing for anonymous contacts).
SELECT id, account_id, channel_id, agent_id, contact_id, contact_name, status, created_at
FROM conversations
WHERE account_id = $1 AND contact_id = $2
ORDER BY created_at DESC;

-- name: LgpdListMessagesByContact :many
-- Fetches every message across every conversation that the contact took
-- part in. Joined via conversations to preserve account_id scoping.
SELECT m.id, m.conversation_id, m.channel_id, m.sender, m.sender_name,
       m.type, m.content, m.direction, m.created_at
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
WHERE c.account_id = $1 AND c.contact_id = $2
ORDER BY m.created_at ASC;

-- name: LgpdListCampaignRecipientsByPhone :many
-- Every campaign send that targeted this externalID, with the campaign
-- name + status for context. Account-scoped via campaigns join.
SELECT cr.id, cr.campaign_id, c.name AS campaign_name, cr.recipient,
       cr.status, cr.sent_at, cr.delivered_at, cr.read_at, cr.failed_at
FROM campaign_recipients cr
JOIN campaigns c ON c.id = cr.campaign_id
WHERE c.account_id = $1 AND cr.recipient = $2
ORDER BY cr.created_at DESC;

-- name: LgpdAnonymizeContact :exec
-- Replaces PII on the contact row but preserves the row itself (FK targets
-- in conversations + aggregate counters in usage rely on its existence).
-- opted_out_at is also set so re-engagement is never automatic.
UPDATE contacts
SET name           = '[Apagado]',
    email          = NULL,
    phone          = NULL,
    avatar_url     = NULL,
    metadata       = '{}'::jsonb,
    external_id    = $3,          -- caller provides a unique placeholder
    opted_out_at   = COALESCE(opted_out_at, NOW()),
    updated_at     = NOW()
WHERE account_id = $1 AND external_id = $2;

-- name: LgpdAnonymizeMessages :exec
-- Erases message content across every conversation the contact took part
-- in. We preserve metadata (timestamps, direction) so analytics and
-- campaign counters remain accurate — only PII-containing text is lost.
UPDATE messages
SET content    = '[Removido a pedido do titular]',
    sender_name= NULL,
    media_url  = NULL,
    metadata   = NULL
FROM conversations c
WHERE messages.conversation_id = c.id
  AND c.account_id = $1
  AND c.contact_id = $2;

-- name: LgpdAnonymizeConversations :exec
-- Blanks out the denormalized contact_name column on conversations.
-- Does NOT change contact_id (we need it to keep the relationship
-- between messages and the now-anonymized contact row) but prevents
-- the name from showing up on the inbox list.
UPDATE conversations
SET contact_name = NULL
WHERE account_id = $1 AND contact_id = $2;
