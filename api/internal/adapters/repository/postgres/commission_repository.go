package postgres

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// CommissionRepository is a raw-SQL repo (NOT sqlc) over the same pool,
// owning seller_settings + commission_ledger (US-007, vendedores v3).
// Same rationale as MonthlyProfitRepository: isolated feature, keeps
// the expensive-to-extend sqlc repos untouched.
type CommissionRepository struct {
	db DBTX
}

func NewCommissionRepository(db DBTX) *CommissionRepository {
	return &CommissionRepository{db: db}
}

// commissionLockClass namespaces the ledger's advisory locks
// (pg_advisory_xact_lock is a global int-pair space shared by the
// whole database).
const commissionLockClass = "commission_cycle"

// txBeginner is the slice of pgxpool.Pool (and pgx.Tx) the tx helpers
// need — DBTX itself stays the sqlc-generated minimal surface.
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// lockCycles takes the transaction-scoped advisory lock of each cycle,
// deduped and in ascending order — every multi-lock holder acquires in
// the same order, so lock waits can never form a cycle (deadlock).
func lockCycles(ctx context.Context, tx pgx.Tx, cycles []string) error {
	sorted := append([]string(nil), cycles...)
	sort.Strings(sorted)
	prev := ""
	for i, c := range sorted {
		if i > 0 && c == prev {
			continue
		}
		prev = c
		if _, err := tx.Exec(ctx,
			`SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`,
			commissionLockClass, c,
		); err != nil {
			return err
		}
	}
	return nil
}

// InCycleTx runs fn in one transaction holding the given cycles'
// advisory locks (US-006). ComputeCycle's whole read-compute-write pass
// runs under it, so two concurrent recomputes of the same cycle — or a
// recompute racing MarkPayoutPaid — serialize instead of interleaving
// their check-then-insert sequences.
func (r *CommissionRepository) InCycleTx(ctx context.Context, cycles []string, fn func(domain.CommissionRepository) error) error {
	b, ok := r.db.(txBeginner)
	if !ok {
		// Already transaction-bound (nested call): the outer tx holds
		// the locks; just run.
		return fn(r)
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	if err := lockCycles(ctx, tx, cycles); err != nil {
		return err
	}
	if err := fn(&CommissionRepository{db: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetSellerSettings returns nil, nil when the seller has no row —
// callers apply the 50/20/active defaults.
func (r *CommissionRepository) GetSellerSettings(ctx context.Context, userID string) (*domain.SellerSettings, error) {
	var s domain.SellerSettings
	err := r.db.QueryRow(ctx,
		`SELECT user_id, commission_rate_own::float8, commission_rate_inbound::float8,
		        is_seller, is_active_am, deactivated_at, residual_until, pix_key, updated_at
		 FROM seller_settings WHERE user_id = $1`,
		userID,
	).Scan(&s.UserID, &s.CommissionRateOwn, &s.CommissionRateInbound,
		&s.IsSeller, &s.IsActiveAM, &s.DeactivatedAt, &s.ResidualUntil, &s.PixKey, &s.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// UpsertSellerActivity writes the activity-clause state; an existing
// row keeps its rates and pix_key.
func (r *CommissionRepository) UpsertSellerActivity(ctx context.Context, userID string, isActive bool, deactivatedAt, residualUntil *time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO seller_settings (user_id, is_active_am, deactivated_at, residual_until)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id) DO UPDATE SET
		     is_active_am   = EXCLUDED.is_active_am,
		     deactivated_at = EXCLUDED.deactivated_at,
		     residual_until = EXCLUDED.residual_until,
		     updated_at     = NOW()`,
		userID, isActive, deactivatedAt, residualUntil,
	)
	return err
}

// SetSellerFlag grants/revokes the explicit seller flag (US-014); an
// existing row keeps its rates, activity state and pix_key.
func (r *CommissionRepository) SetSellerFlag(ctx context.Context, userID string, isSeller bool) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO seller_settings (user_id, is_seller)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET
		     is_seller  = EXCLUDED.is_seller,
		     updated_at = NOW()`,
		userID, isSeller,
	)
	return err
}

// ListCommissionBases joins the cycle's profit rows with the accounts'
// CURRENT seller link — accounts unlinked since the profit compute drop
// out (their stale pending entries are the owner's to reconcile) — plus
// the US-008 bonus signals inside [from, to): a 'plan_upgraded' billing
// event and a promoter (9-10) NPS response.
func (r *CommissionRepository) ListCommissionBases(ctx context.Context, cycle string, from, to time.Time) ([]domain.CommissionBase, error) {
	rows, err := r.db.Query(ctx,
		`SELECT a.seller_user_id, p.account_id, a.lead_source, p.net_profit_cents,
		        a.created_at, a.subscription_status,
		        EXISTS (SELECT 1 FROM billing_events be
		                WHERE be.account_id = p.account_id
		                  AND be.event_type = 'plan_upgraded'
		                  AND be.occurred_at >= $2 AND be.occurred_at < $3),
		        EXISTS (SELECT 1 FROM nps_responses n
		                WHERE n.account_id = p.account_id
		                  AND n.score >= 9
		                  AND n.created_at >= $2 AND n.created_at < $3)
		 FROM account_monthly_profit p
		 JOIN accounts a ON a.id = p.account_id
		 WHERE p.cycle = $1 AND a.seller_user_id IS NOT NULL
		 ORDER BY p.account_id`,
		cycle, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.CommissionBase{}
	for rows.Next() {
		var b domain.CommissionBase
		if err := rows.Scan(&b.SellerUserID, &b.AccountID, &b.LeadSource, &b.NetProfitCents,
			&b.AccountCreatedAt, &b.SubscriptionStatus, &b.HasPlanUpgrade, &b.HasPromoterNPS); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpsertEntry writes on the PENDING-only partial unique index
// (seller, account, cycle, reason) WHERE status='pending' (migration
// 000135): a recompute updates the tuple's pending row in place, while
// paid rows — immutable history — never arbitrate the conflict, so a
// new pending 'adjustment' inserts cleanly NEXT TO an already-settled
// one instead of silently no-oping and losing the delta (US-005).
func (r *CommissionRepository) UpsertEntry(ctx context.Context, e *domain.CommissionEntry) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO commission_ledger
		     (id, seller_user_id, account_id, cycle, base_net_profit_cents, rate_applied, amount_cents, reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (seller_user_id, account_id, cycle, reason) WHERE status = 'pending' DO UPDATE SET
		     base_net_profit_cents = EXCLUDED.base_net_profit_cents,
		     rate_applied          = EXCLUDED.rate_applied,
		     amount_cents          = EXCLUDED.amount_cents,
		     updated_at            = NOW()`,
		e.ID, e.SellerUserID, e.AccountID, e.Cycle, e.BaseNetProfitCents, e.RateApplied, e.AmountCents, e.Reason,
	)
	return err
}

// DeleteStaleProfitShare removes the account+cycle's pending
// profit-share entries that no longer match the computed
// (seller, reason). bonus_upgrade is in the set — it REPLACES standard
// in an upgrade cycle, so a reason flip either way must drop the stale
// row — and so is adjustment (a stale pending delta from an earlier
// paid-cycle recompute). Additive bonus rows (nps/retention) and paid
// rows survive.
func (r *CommissionRepository) DeleteStaleProfitShare(ctx context.Context, accountID, cycle, keepSellerUserID, keepReason string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM commission_ledger
		 WHERE account_id = $1 AND cycle = $2
		   AND status = 'pending'
		   AND reason IN ('standard','residual','sla_reduced','bonus_upgrade','adjustment')
		   AND NOT (seller_user_id = $3 AND reason = $4)`,
		accountID, cycle, keepSellerUserID, keepReason,
	)
	return err
}

// SumPaidProfitShare totals the (account, cycle)'s PAID profit-share
// money — the mutually-exclusive reasons plus settled adjustments —
// under ANY seller. Existence tells the recompute the cycle's profit
// already settled (US-004): record a delta adjustment, never a second
// full pending row. The second return is the seller of the most
// recently settled row (US-023): a clawback debits whoever the money
// actually went to, never the account's current seller.
func (r *CommissionRepository) SumPaidProfitShare(ctx context.Context, accountID, cycle string) (int64, string, bool, error) {
	var total int64
	var count int
	var paidSeller string
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0), COUNT(*),
		        COALESCE((SELECT seller_user_id FROM commission_ledger
		                  WHERE account_id = $1 AND cycle = $2 AND status = 'paid'
		                    AND reason IN ('standard','residual','sla_reduced','bonus_upgrade','adjustment')
		                  ORDER BY paid_at DESC NULLS LAST, created_at DESC
		                  LIMIT 1), '')
		 FROM commission_ledger
		 WHERE account_id = $1 AND cycle = $2 AND status = 'paid'
		   AND reason IN ('standard','residual','sla_reduced','bonus_upgrade','adjustment')`,
		accountID, cycle,
	).Scan(&total, &count, &paidSeller)
	return total, paidSeller, count > 0, err
}

// orphanedByUnlinkNote marks a pending ledger row whose account was
// unlinked from its seller — the recompute no longer manages it
// (ListCommissionBases only sees linked accounts) and a payout sweep
// would otherwise settle it silently. Surfaces in the owner ledger
// listing and the accounting CSV's note column.
const orphanedByUnlinkNote = "orphaned_by_unlink"

// FlagOrphanedPending stamps the account's PENDING ledger rows for
// owner reconciliation on seller unlink (US-008 review fix). Paid rows
// are history and stay untouched. Idempotent: already-flagged rows are
// skipped. Returns the number of rows flagged.
func (r *CommissionRepository) FlagOrphanedPending(ctx context.Context, accountID string) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE commission_ledger
		 SET note = CASE WHEN COALESCE(note, '') = '' THEN $2
		                 ELSE note || ' | ' || $2 END,
		     updated_at = NOW()
		 WHERE account_id = $1 AND status = 'pending'
		   AND COALESCE(note, '') NOT LIKE '%' || $2 || '%'`,
		accountID, orphanedByUnlinkNote,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// HasBonusEntry reports whether the account already earned the bonus
// reason in any of the given cycles (nil = any cycle ever). Any status
// and any SELLER count — a paid bonus must keep blocking re-grants, and
// a bonus granted before a seller reallocation keeps blocking after it
// (US-004: the current cycle is NOT excluded; the granted row itself
// makes the recompute idempotent).
func (r *CommissionRepository) HasBonusEntry(ctx context.Context, accountID, reason string, cycles []string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM commission_ledger
		     WHERE account_id = $1 AND reason = $2
		       AND ($3::text[] IS NULL OR cycle = ANY($3))
		 )`,
		accountID, reason, cycles,
	).Scan(&exists)
	return exists, err
}

// ListSLAConversations feeds the US-009 metric: every ASSIGNED
// conversation on the platform account created inside [from, to), with
// the first human reply — first OUTBOUND message from a non-bot sender,
// same direction+sender convention as the attendant analytics queries.
// Inbound rows store the contact's phone/id in sender (never the
// literal 'customer'), so filtering on sender alone would count the
// customer's own message as the "response". customer_account_id rides
// along so the use case can collapse a customer account's internal
// threads into one SLA unit (canal-interno US-009).
func (r *CommissionRepository) ListSLAConversations(ctx context.Context, accountID string, from, to time.Time) ([]domain.SLAConversation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT c.id, c.assigned_to, COALESCE(c.contact_name, ''),
		        COALESCE(c.created_at, NOW()),
		        (SELECT MIN(m.created_at) FROM messages m
		          WHERE m.conversation_id = c.id
		            AND m.direction = 'outbound'
		            AND m.sender NOT IN ('bot', 'customer')),
		        COALESCE(c.customer_account_id, '')
		 FROM conversations c
		 WHERE c.account_id = $1
		   AND c.assigned_to IS NOT NULL
		   AND c.created_at >= $2 AND c.created_at < $3`,
		accountID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SLAConversation{}
	for rows.Next() {
		var s domain.SLAConversation
		if err := rows.Scan(&s.ConversationID, &s.AssignedTo, &s.ContactName, &s.CreatedAt, &s.FirstHumanResponseAt, &s.CustomerAccountID); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSLAState returns nil, nil when the seller has no row for the
// cycle.
func (r *CommissionRepository) GetSLAState(ctx context.Context, sellerUserID, cycle string) (*domain.SellerSLAState, error) {
	var s domain.SellerSLAState
	err := r.db.QueryRow(ctx,
		`SELECT id, seller_user_id, cycle, total_conversations, breached_conversations,
		        breach, consecutive_breaches, action, computed_at
		 FROM seller_sla_state
		 WHERE seller_user_id = $1 AND cycle = $2`,
		sellerUserID, cycle,
	).Scan(&s.ID, &s.SellerUserID, &s.Cycle, &s.TotalConversations, &s.BreachedConversations,
		&s.Breach, &s.ConsecutiveBreaches, &s.Action, &s.ComputedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// UpsertSLAState writes on UNIQUE(seller_user_id, cycle) preserving the
// original id (RETURNING updates the struct — same pattern as the
// monthly-profit upsert).
func (r *CommissionRepository) UpsertSLAState(ctx context.Context, s *domain.SellerSLAState) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO seller_sla_state
		     (id, seller_user_id, cycle, total_conversations, breached_conversations,
		      breach, consecutive_breaches, action)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (seller_user_id, cycle) DO UPDATE SET
		     total_conversations    = EXCLUDED.total_conversations,
		     breached_conversations = EXCLUDED.breached_conversations,
		     breach                 = EXCLUDED.breach,
		     consecutive_breaches   = EXCLUDED.consecutive_breaches,
		     action                 = EXCLUDED.action,
		     computed_at            = NOW()
		 RETURNING id`,
		s.ID, s.SellerUserID, s.Cycle, s.TotalConversations, s.BreachedConversations,
		s.Breach, s.ConsecutiveBreaches, s.Action,
	).Scan(&s.ID)
}

func (r *CommissionRepository) ListSLAStatesByCycle(ctx context.Context, cycle string) ([]*domain.SellerSLAState, error) {
	rows, err := r.db.Query(ctx,
		`SELECT s.id, s.seller_user_id, s.cycle, s.total_conversations, s.breached_conversations,
		        s.breach, s.consecutive_breaches, s.action, s.computed_at, COALESCE(u.email, '')
		 FROM seller_sla_state s
		 LEFT JOIN users u ON u.id = s.seller_user_id
		 WHERE s.cycle = $1
		 ORDER BY u.email`,
		cycle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.SellerSLAState{}
	for rows.Next() {
		var s domain.SellerSLAState
		if err := rows.Scan(&s.ID, &s.SellerUserID, &s.Cycle, &s.TotalConversations, &s.BreachedConversations,
			&s.Breach, &s.ConsecutiveBreaches, &s.Action, &s.ComputedAt, &s.SellerEmail); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// ListDetractorFlags counts detractor NPS (0-6) per seller over the
// carteira inside [from, to). Informational for the owner panel only —
// nothing here ever feeds a rate.
func (r *CommissionRepository) ListDetractorFlags(ctx context.Context, from, to time.Time) ([]domain.SellerDetractorFlag, error) {
	rows, err := r.db.Query(ctx,
		`SELECT a.seller_user_id, COALESCE(u.email, ''), COUNT(*)
		 FROM nps_responses n
		 JOIN accounts a ON a.id = n.account_id
		 LEFT JOIN users u ON u.id = a.seller_user_id
		 WHERE a.seller_user_id IS NOT NULL
		   AND n.score <= 6
		   AND n.created_at >= $1 AND n.created_at < $2
		 GROUP BY a.seller_user_id, u.email
		 ORDER BY u.email`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerDetractorFlag{}
	for rows.Next() {
		var f domain.SellerDetractorFlag
		if err := rows.Scan(&f.SellerUserID, &f.SellerEmail, &f.DetractorResponses); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ListCarteiraAccounts returns the seller's linked accounts (US-010)
// with plan/MRR and the given (last closed) cycle's computed numbers.
// The commission subquery SUMs every ledger reason — profit share plus
// additive bonuses — while rate/reason come from the single
// mutually-exclusive profit-share row the recompute manages.
// The trailing engine-only columns (created_at, plan/limit overrides,
// latest usage from the account_usage_summary view) feed the co-pilot
// suggestion engine in the same round-trip (US-022 N+1 fix) — they are
// json:"-" on the domain struct and never reach the carteira payload.
func (r *CommissionRepository) ListCarteiraAccounts(ctx context.Context, sellerUserID, cycle string) ([]domain.SellerCarteiraAccount, error) {
	rows, err := r.db.Query(ctx,
		`SELECT a.id, a.name,
		        COALESCE(p.name, ''), COALESCE(p.price, 0), COALESCE(NULLIF(p.currency, ''), a.currency),
		        a.lead_source, a.subscription_status, a.seller_linked_at,
		        a.converted_conversation_id,
		        mp.net_profit_cents,
		        cl.total_cents, cl.rate_applied, cl.reason,
		        a.created_at, a.plan_id,
		        a.max_conversations_per_month_override, a.max_ai_tokens_per_month_override,
		        us.conversations_count, us.ai_tokens_used, us.addon_tokens
		 FROM accounts a
		 LEFT JOIN plans p ON p.id = a.plan_id
		 LEFT JOIN account_monthly_profit mp ON mp.account_id = a.id AND mp.cycle = $2
		 LEFT JOIN account_usage_summary us ON us.account_id = a.id
		 LEFT JOIN LATERAL (
		     SELECT SUM(l.amount_cents) AS total_cents,
		            (MAX(l.rate_applied) FILTER (WHERE l.reason IN ('standard','residual','sla_reduced','bonus_upgrade')))::float8 AS rate_applied,
		            MAX(l.reason) FILTER (WHERE l.reason IN ('standard','residual','sla_reduced','bonus_upgrade')) AS reason
		     FROM commission_ledger l
		     WHERE l.seller_user_id = $1 AND l.account_id = a.id AND l.cycle = $2
		 ) cl ON true
		 WHERE a.seller_user_id = $1
		 ORDER BY a.created_at DESC`,
		sellerUserID, cycle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerCarteiraAccount{}
	for rows.Next() {
		var acc domain.SellerCarteiraAccount
		if err := rows.Scan(&acc.AccountID, &acc.Name,
			&acc.PlanName, &acc.MRRCents, &acc.Currency,
			&acc.LeadSource, &acc.SubscriptionStatus, &acc.SellerLinkedAt,
			&acc.ConvertedConversationID,
			&acc.LastCycleNetProfitCents,
			&acc.LastCycleCommissionCents, &acc.LastCycleRateApplied, &acc.LastCycleReason,
			&acc.CreatedAt, &acc.PlanID,
			&acc.MaxConversationsPerMonthOverride, &acc.MaxAITokensPerMonthOverride,
			&acc.UsageConversations, &acc.UsageAITokens, &acc.UsageAddonTokens); err != nil {
			return nil, err
		}
		out = append(out, acc)
	}
	return out, rows.Err()
}

// SumPendingCommission totals the seller's unpaid ledger across every
// cycle — the "pending commission" header stat of the seller panel.
func (r *CommissionRepository) SumPendingCommission(ctx context.Context, sellerUserID string) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0) FROM commission_ledger
		 WHERE seller_user_id = $1 AND status = 'pending'`,
		sellerUserID,
	).Scan(&total)
	return total, err
}

// ListSellerStatement returns the seller's own ledger for one cycle
// (US-011), each row joined with the account name and the cycle's full
// profit breakdown — the transparency requirement: never just a total.
// Profit-share rows first, additive bonuses after.
func (r *CommissionRepository) ListSellerStatement(ctx context.Context, sellerUserID, cycle string) ([]domain.SellerStatementLine, error) {
	rows, err := r.db.Query(ctx,
		`SELECT l.account_id, COALESCE(a.name, ''), l.reason, l.rate_applied::float8,
		        l.amount_cents, l.status, l.paid_at,
		        mp.revenue_cents, mp.stripe_fee_cents, mp.token_cost_cents,
		        mp.tax_cents, mp.infra_cents, mp.net_profit_cents
		 FROM commission_ledger l
		 LEFT JOIN accounts a ON a.id = l.account_id
		 LEFT JOIN account_monthly_profit mp ON mp.account_id = l.account_id AND mp.cycle = l.cycle
		 WHERE l.seller_user_id = $1 AND l.cycle = $2
		 ORDER BY a.name, l.account_id,
		          l.reason IN ('bonus_nps','bonus_retention'), l.reason`,
		sellerUserID, cycle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerStatementLine{}
	for rows.Next() {
		var s domain.SellerStatementLine
		if err := rows.Scan(&s.AccountID, &s.AccountName, &s.Reason, &s.RateApplied,
			&s.AmountCents, &s.Status, &s.PaidAt,
			&s.RevenueCents, &s.StripeFeeCents, &s.TokenCostCents,
			&s.TaxCents, &s.InfraCents, &s.NetProfitCents); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListSellerCycles returns the distinct cycles the seller earned in,
// newest first.
func (r *CommissionRepository) ListSellerCycles(ctx context.Context, sellerUserID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT cycle FROM commission_ledger
		 WHERE seller_user_id = $1
		 ORDER BY cycle DESC`,
		sellerUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertSellerPixKey writes only pix_key (nil clears it); an existing
// row keeps its rates and activity state.
func (r *CommissionRepository) UpsertSellerPixKey(ctx context.Context, userID string, pixKey *string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO seller_settings (user_id, pix_key)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET
		     pix_key    = EXCLUDED.pix_key,
		     updated_at = NOW()`,
		userID, pixKey,
	)
	return err
}

// ListPayoutSummaries groups the ledger per seller for the owner payout
// screen (US-013). One pass over the cycle's rows plus any PENDING rows
// left in earlier cycles — those are the late-recompute adjustments that
// settle together with this cycle. FILTER splits the aggregate buckets.
func (r *CommissionRepository) ListPayoutSummaries(ctx context.Context, cycle string) ([]domain.SellerPayoutSummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT l.seller_user_id, COALESCE(u.name, ''), COALESCE(u.email, ''), ss.pix_key,
		        COALESCE(SUM(l.amount_cents) FILTER (WHERE l.cycle = $1 AND l.status = 'pending'), 0),
		        COUNT(*) FILTER (WHERE l.cycle = $1 AND l.status = 'pending'),
		        COALESCE(SUM(l.amount_cents) FILTER (WHERE l.cycle = $1 AND l.status = 'paid'), 0),
		        COALESCE(SUM(l.amount_cents) FILTER (WHERE l.cycle < $1 AND l.status = 'pending'), 0),
		        COUNT(*) FILTER (WHERE l.cycle < $1 AND l.status = 'pending')
		 FROM commission_ledger l
		 LEFT JOIN users u ON u.id = l.seller_user_id
		 LEFT JOIN seller_settings ss ON ss.user_id = l.seller_user_id
		 WHERE l.cycle = $1 OR (l.cycle < $1 AND l.status = 'pending')
		 GROUP BY l.seller_user_id, u.name, u.email, ss.pix_key
		 ORDER BY u.email, l.seller_user_id`,
		cycle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SellerPayoutSummary{}
	for rows.Next() {
		var s domain.SellerPayoutSummary
		if err := rows.Scan(&s.SellerUserID, &s.SellerName, &s.SellerEmail, &s.PixKey,
			&s.PendingCents, &s.PendingEntries, &s.PaidCents,
			&s.AdjustmentCents, &s.AdjustmentEntries); err != nil {
			return nil, err
		}
		s.TotalDueCents = s.PendingCents + s.AdjustmentCents
		out = append(out, s)
	}
	return out, rows.Err()
}

// MarkPayoutPaid settles everything the seller is owed through the
// cycle (cycle <= $2 — earlier-cycle adjustments ride along, YYYY-MM
// compares lexicographically). Only pending rows match, so a second
// click is a 0-row no-op and entries minted by a LATER recompute stay
// pending for the next cycle's payout.
//
// US-006: the settle runs in a transaction holding the advisory lock
// of the target cycle AND of every earlier cycle with pending rows for
// the seller. An in-flight ComputeCycle of any of those cycles holds
// the same lock, so the paid total is revalidated against the ledger
// AFTER that recompute commits — never a stale snapshot — and the
// recompute's DeleteStaleProfitShare can never race the rows being
// paid. Returns (rows settled, cents settled).
func (r *CommissionRepository) MarkPayoutPaid(ctx context.Context, sellerUserID, cycle string, paidAt time.Time, note *string) (int64, int64, error) {
	b, ok := r.db.(txBeginner)
	if !ok {
		return r.markPayoutPaidLocked(ctx, r.db, sellerUserID, cycle, paidAt, note)
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	// Lock the target cycle plus every earlier cycle the sweep can
	// touch. A cycle with no committed pending row needs no lock: any
	// row a concurrent recompute mints there is either visible to the
	// UPDATE (paid once, correctly) or stays pending for the next
	// payout — double-pay requires an unpaid check racing rows we
	// settle, and rows we settle exist here, so their cycle IS locked.
	cycles := []string{cycle}
	rows, err := tx.Query(ctx,
		`SELECT DISTINCT cycle FROM commission_ledger
		 WHERE seller_user_id = $1 AND cycle <= $2 AND status = 'pending'`,
		sellerUserID, cycle,
	)
	if err != nil {
		return 0, 0, err
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return 0, 0, err
		}
		cycles = append(cycles, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if err := lockCycles(ctx, tx, cycles); err != nil {
		return 0, 0, err
	}

	entries, totalCents, err := r.markPayoutPaidLocked(ctx, tx, sellerUserID, cycle, paidAt, note)
	if err != nil {
		return 0, 0, err
	}
	return entries, totalCents, tx.Commit(ctx)
}

// markPayoutPaidLocked flips the pending rows in one statement and
// returns what actually settled — the amount comes from the rows the
// UPDATE matched under the locks, which is the revalidation.
func (r *CommissionRepository) markPayoutPaidLocked(ctx context.Context, db DBTX, sellerUserID, cycle string, paidAt time.Time, note *string) (int64, int64, error) {
	var entries, totalCents int64
	err := db.QueryRow(ctx,
		`WITH settled AS (
		     UPDATE commission_ledger
		     SET status = 'paid', paid_at = $3, note = COALESCE($4, note), updated_at = NOW()
		     WHERE seller_user_id = $1 AND cycle <= $2 AND status = 'pending'
		     RETURNING amount_cents
		 )
		 SELECT COUNT(*), COALESCE(SUM(amount_cents), 0) FROM settled`,
		sellerUserID, cycle, paidAt, note,
	).Scan(&entries, &totalCents)
	return entries, totalCents, err
}

func (r *CommissionRepository) ListEntriesByCycle(ctx context.Context, cycle string) ([]*domain.CommissionEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT l.id, l.seller_user_id, l.account_id, l.cycle, l.base_net_profit_cents,
		        l.rate_applied::float8, l.amount_cents, l.reason, l.status, l.paid_at, l.note,
		        l.created_at, l.updated_at, COALESCE(u.email, ''), COALESCE(a.name, '')
		 FROM commission_ledger l
		 LEFT JOIN users u ON u.id = l.seller_user_id
		 LEFT JOIN accounts a ON a.id = l.account_id
		 WHERE l.cycle = $1
		 ORDER BY u.email, a.name, l.reason`,
		cycle,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*domain.CommissionEntry{}
	for rows.Next() {
		var e domain.CommissionEntry
		if err := rows.Scan(&e.ID, &e.SellerUserID, &e.AccountID, &e.Cycle, &e.BaseNetProfitCents,
			&e.RateApplied, &e.AmountCents, &e.Reason, &e.Status, &e.PaidAt, &e.Note,
			&e.CreatedAt, &e.UpdatedAt, &e.SellerEmail, &e.AccountName); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
