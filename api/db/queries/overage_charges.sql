-- name: CreateOverageCharge :exec
INSERT INTO overage_charges (
    id, account_id, amount_cents, running_total_cents,
    overage_conversations_count, capped, status
) VALUES ($1, $2, $3, $4, $5, $6, 'pending');

-- name: GetPendingOverageCharge :one
-- The unique partial index guarantees at most one pending row per
-- account, but ORDER BY keeps the query deterministic regardless.
SELECT * FROM overage_charges
WHERE account_id = $1 AND status = 'pending'
ORDER BY created_at ASC
LIMIT 1;

-- name: MarkOverageChargeCharged :exec
UPDATE overage_charges
SET status = 'charged', charged_at = NOW()
WHERE id = $1;

-- name: MarkOverageChargeNeedsReview :exec
UPDATE overage_charges
SET status = 'needs_review'
WHERE id = $1;

-- name: SumChargedOverageCents :one
-- Cycle-to-date charged total — the settler's source of truth for how
-- much has already been billed. Archived rows (closed cycles) are
-- excluded, so each cycle starts the sum back at zero.
SELECT COALESCE(SUM(amount_cents), 0)::int AS total
FROM overage_charges
WHERE account_id = $1 AND status = 'charged';

-- name: ArchiveChargedOverage :exec
-- Closes the cycle on invoice.paid: charged rows stop counting toward
-- the next cycle's total. pending / needs_review rows are left as-is.
UPDATE overage_charges
SET status = 'archived'
WHERE account_id = $1 AND status = 'charged';
