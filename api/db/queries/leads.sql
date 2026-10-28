-- name: UpsertLead :one
-- Idempotent on (funnel_id, contact_id): a second call for the same pair
-- updates the existing lead's mutable fields instead of inserting a duplicate.
-- Returns the resulting row so callers get the (possibly pre-existing) id.
INSERT INTO leads (
    id, account_id, funnel_id, contact_id, status, source, score,
    collected_metadata, enrichment, approach_draft, buy_intent_at, qualified_at,
    owner_user_id, visibility, matched_products, anticipated_objections,
    created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW(), NOW())
ON CONFLICT (funnel_id, contact_id) DO UPDATE
SET status             = EXCLUDED.status,
    -- Same rule as UpdateLeadStatus: a status change re-seats the card.
    stage_id           = CASE WHEN EXCLUDED.status IS DISTINCT FROM leads.status
                           THEN COALESCE((SELECT fs.id FROM funnel_stages fs WHERE fs.funnel_id = leads.funnel_id AND fs.maps_to_status = EXCLUDED.status ORDER BY fs.position LIMIT 1), leads.stage_id)
                           ELSE leads.stage_id END,
    -- Ownership is assigned through AssignLeadOwner, never silently reset by
    -- a re-import: COALESCE keeps whoever already owns the lead (US-F009).
    owner_user_id      = COALESCE(leads.owner_user_id, EXCLUDED.owner_user_id),
    visibility         = EXCLUDED.visibility,
    source             = EXCLUDED.source,
    score              = EXCLUDED.score,
    collected_metadata = EXCLUDED.collected_metadata,
    enrichment         = EXCLUDED.enrichment,
    approach_draft     = EXCLUDED.approach_draft,
    buy_intent_at      = EXCLUDED.buy_intent_at,
    qualified_at       = EXCLUDED.qualified_at,
    matched_products   = EXCLUDED.matched_products,
    anticipated_objections = EXCLUDED.anticipated_objections,
    updated_at         = NOW()
RETURNING *;

-- name: GetLeadByFunnelAndContact :one
SELECT * FROM leads WHERE funnel_id = $1 AND contact_id = $2 LIMIT 1;

-- name: GetLeadByID :one
SELECT * FROM leads WHERE id = $1 LIMIT 1;

-- name: ListLeadsByFunnel :many
-- When the status arg is NULL all leads for the funnel are returned; otherwise
-- the result is filtered to the given status. Paginated via limit/offset
-- (the dashboard pipeline / lead list, US-015).
SELECT * FROM leads
WHERE funnel_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT sqlc.arg('row_limit') OFFSET sqlc.arg('row_offset');

-- name: UpdateLeadMetadata :exec
UPDATE leads
SET collected_metadata = $2,
    enrichment         = $3,
    updated_at         = NOW()
WHERE id = $1;

-- name: UpdateLeadStatus :exec
-- Status and board column move TOGETHER (analysis 2026-09-01, "stage × status
-- drift"): seven writers used to change status and leave stage_id where the
-- last drag put it, so a lost lead could sit in "Ganhos". The column is the
-- funnel's first stage mapping the new status; a transient status with no
-- column (enriching, pending_approval) keeps the current one.
UPDATE leads
SET status     = $2,
    stage_id   = COALESCE((SELECT fs.id FROM funnel_stages fs WHERE fs.funnel_id = leads.funnel_id AND fs.maps_to_status = $2 ORDER BY fs.position LIMIT 1), leads.stage_id),
    updated_at = NOW()
WHERE leads.id = $1;

-- name: MarkLeadBuyIntent :exec
-- Stamps buy_intent_at = NOW() the FIRST time a lead signals purchase intent.
-- The `buy_intent_at IS NULL` guard makes it idempotent: subsequent calls are
-- a no-op so the first-intent timestamp is preserved.
UPDATE leads
SET buy_intent_at = NOW(),
    updated_at    = NOW()
WHERE id = $1 AND buy_intent_at IS NULL;

-- name: MarkLeadQualified :one
-- Stamps qualified_at = NOW() and promotes status to 'qualified' the FIRST time
-- a lead completes the funnel's required-metadata gate. The `qualified_at IS NULL`
-- guard makes it idempotent: a second call matches no row and returns
-- pgx.ErrNoRows, so the qualified webhook fires exactly once (US-016).
UPDATE leads
SET qualified_at = NOW(),
    status       = 'qualified',
    updated_at   = NOW()
WHERE id = $1 AND qualified_at IS NULL
RETURNING id;


-- name: AssignLeadOwner :one
-- Assigns (or clears, with NULL) the salesperson responsible for a lead.
-- Account-scoped so a caller cannot reassign another tenant's lead.
UPDATE leads
SET owner_user_id = $3,
    updated_at    = NOW()
WHERE id = $1 AND account_id = $2
RETURNING *;

-- name: SetLeadVisibility :one
UPDATE leads
SET visibility = $3,
    updated_at = NOW()
WHERE id = $1 AND account_id = $2
RETURNING *;

-- name: ListLeadsByFunnelForOwner :many
-- The vendedor's pipeline view: their own leads plus unassigned and
-- team-visible ones. Filtering in SQL (not in Go over a fetched page) is
-- what keeps pagination honest — an in-memory filter hides rows that rank
-- past the page limit.
SELECT * FROM leads
WHERE account_id = $1
  AND funnel_id = $2
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status)::text)
  AND (
        owner_user_id IS NULL
        OR owner_user_id = sqlc.arg(owner_user_id)::text
        OR visibility = 'team'
      )
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListLeadsByAccountAndStatus :many
-- The approval queue (US-M020): everything waiting on a human, across every
-- funnel. Oldest FIRST by default — a draft that has sat in the queue longest
-- is the one most likely to go stale, and a newest-first queue quietly starves
-- it. The operator may still ask for newest first when they are working the
-- drafts a search just produced.
--
-- The funnel narrow and the ordering both live HERE rather than in Go: a filter
-- applied to an already-paginated page hides every matching row past the limit,
-- which is the bug that makes a filter worse than no filter.
SELECT * FROM leads
WHERE account_id = $1
  AND status = $2
  AND (sqlc.narg(funnel_id)::text IS NULL OR funnel_id = sqlc.narg(funnel_id)::text)
-- One statement, both directions. When newest_first is false the CASE yields
-- NULL for every row, so they all tie and the second key decides — never
-- interpolate a direction into the SQL text.
ORDER BY CASE WHEN sqlc.arg(newest_first)::bool THEN updated_at END DESC,
         updated_at ASC
LIMIT $3 OFFSET $4;

-- name: ListLeadsByAccountAndStatusForOwner :many
-- Same queue, scoped the way a vendedor sees the pipeline (US-F009): own
-- leads plus unassigned and team-visible ones, filtered in SQL so paging
-- cannot hide their own drafts.
SELECT * FROM leads
WHERE account_id = $1
  AND status = $2
  AND (
        owner_user_id IS NULL
        OR owner_user_id = sqlc.arg(owner_user_id)::text
        OR visibility = 'team'
      )
  AND (sqlc.narg(funnel_id)::text IS NULL OR funnel_id = sqlc.narg(funnel_id)::text)
ORDER BY CASE WHEN sqlc.arg(newest_first)::bool THEN updated_at END DESC,
         updated_at ASC
LIMIT $3 OFFSET $4;

-- name: CountQueueByFunnelForAccount :many
-- How much of the approval queue sits in each funnel, over the WHOLE queue.
--
-- Counted in SQL for the same reason the pipeline tiles are: a chip labelled
-- from the current page tells the operator the backlog is 50 when it is 340,
-- and the funnel holding the other 290 does not even get a chip.
--
-- Deliberately ignores the selected funnel: the chips have to keep showing
-- where the rest of the work is, or picking one collapses every other count to
-- zero and the operator loses the map.
SELECT l.funnel_id, COALESCE(f.name, '') AS funnel_name, COUNT(*)::bigint AS total
FROM leads l
LEFT JOIN sales_funnels f ON f.id = l.funnel_id
WHERE l.account_id = $1
  AND l.status = $2
GROUP BY l.funnel_id, f.name
ORDER BY total DESC, funnel_name ASC;

-- name: CountQueueByFunnelForOwner :many
-- Same map under the vendedor visibility rules: a chip must never promise rows
-- the board would refuse to show them.
SELECT l.funnel_id, COALESCE(f.name, '') AS funnel_name, COUNT(*)::bigint AS total
FROM leads l
LEFT JOIN sales_funnels f ON f.id = l.funnel_id
WHERE l.account_id = $1
  AND l.status = $2
  AND (
        l.owner_user_id IS NULL
        OR l.owner_user_id = sqlc.arg(owner_user_id)::text
        OR l.visibility = 'team'
      )
GROUP BY l.funnel_id, f.name
ORDER BY total DESC, funnel_name ASC;

-- name: CountLeadsByStatusForAccount :many
-- Pipeline counts for the command centre (US-M023). Grouped in SQL because
-- the screen must show the WHOLE pipeline, not the first page of it —
-- counting a paginated list is how a dashboard quietly under-reports.
SELECT status, COUNT(*)::bigint AS total
FROM leads
WHERE account_id = $1
GROUP BY status;

-- name: CountLeadsByStatusForOwner :many
-- Same counts under the vendedor visibility rules (US-F009), so the tiles
-- never total more leads than the board would actually show them.
SELECT status, COUNT(*)::bigint AS total
FROM leads
WHERE account_id = $1
  AND (
        owner_user_id IS NULL
        OR owner_user_id = sqlc.arg(owner_user_id)::text
        OR visibility = 'team'
      )
GROUP BY status;

-- name: CountUnownedLeadsForAccount :one
-- An unassigned lead is nobody's job, and silence looks like progress.
SELECT COUNT(*)::bigint
FROM leads
WHERE account_id = $1
  AND (owner_user_id IS NULL OR owner_user_id = '')
  AND status NOT IN ('won', 'lost');

-- name: ClaimLeadForSend :one
-- Compare-and-set for the approval send (analysis 2026-09-01): flips
-- pending_approval → contacted atomically so two approvers (or the schedule
-- worker racing a person) cannot both send. Returns pgx.ErrNoRows when the
-- lead was not pending — the caller reads that as "somebody else got it".
UPDATE leads
SET status     = 'contacted',
    stage_id   = COALESCE((SELECT fs.id FROM funnel_stages fs WHERE fs.funnel_id = leads.funnel_id AND fs.maps_to_status = 'contacted' ORDER BY fs.position LIMIT 1), leads.stage_id),
    updated_at = NOW()
WHERE leads.id = $1 AND leads.status = 'pending_approval'
RETURNING leads.id;

-- name: CountUndeliverablePendingApproval :one
-- How much of the approval queue has nowhere to be posted.
--
-- The account-level "you have no channel" check answers a different question,
-- and production proved they come apart: 70 contacts, 52 with a phone, ZERO
-- with an e-mail, and a queue of finished offers addressed to none of them.
-- Told "aprove ou descarte", the operator could do neither.
--
-- Three predicates rather than one so the caller can apply the channels it
-- actually has: a mailbox-only account is blocked by no_email, a WhatsApp-only
-- one by no_phone, and an account with both only by no_address. A lead whose
-- contact row is gone counts as unreachable — the LEFT JOIN's NULLs collapse
-- into the empty string on purpose.
SELECT
    COUNT(*) FILTER (
        WHERE COALESCE(ct.email, '') = '' AND COALESCE(ct.phone, '') = ''
    )::bigint AS no_address,
    COUNT(*) FILTER (WHERE COALESCE(ct.email, '') = '')::bigint AS no_email,
    COUNT(*) FILTER (WHERE COALESCE(ct.phone, '') = '')::bigint AS no_phone,
    COUNT(*)::bigint                                            AS pending
FROM leads l
    LEFT JOIN contacts ct ON ct.id = l.contact_id
WHERE l.account_id = $1
  AND l.status = 'pending_approval';
