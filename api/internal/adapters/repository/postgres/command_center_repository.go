package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Command centre (§6.13, I2).
//
// Six independent reads, one per kind of thing that can need a human, plus the
// period counters and the snooze table. Raw SQL: each statement joins a
// different pair of tables to reach a contact name, and none of them belongs to
// a single entity's repository.
//
// Every query is account-scoped in its OWN WHERE clause and bounded by LIMIT.
// The bound matters: this feeds the first screen, and an account with four
// thousand stale drafts must not turn "what needs me" into a four-thousand-row
// download. The ranking then picks the top of each bucket, so a bounded read
// still surfaces the biggest item in every category.
type CommandCenterRepository struct {
	db *pgxpool.Pool
}

func NewCommandCenterRepository(db *pgxpool.Pool) *CommandCenterRepository {
	return &CommandCenterRepository{db: db}
}

var _ domain.CommandCenterRepository = (*CommandCenterRepository)(nil)

// scanActions runs a query whose columns are, in order: target id, customer
// name, detail, money cents, currency, due_on, since_at. Shared so six
// statements cannot drift into six slightly different row shapes.
func (r *CommandCenterRepository) scanActions(
	ctx context.Context,
	kind domain.PendingActionKind,
	deepLink func(targetID string) string,
	sql string,
	args ...any,
) ([]domain.PendingAction, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.PendingAction{}
	for rows.Next() {
		var a domain.PendingAction
		var due, since *time.Time
		if err := rows.Scan(&a.TargetID, &a.CustomerName, &a.Detail,
			&a.MoneyAtStakeCents, &a.Currency, &due, &since); err != nil {
			return nil, err
		}
		a.Kind = kind
		a.DueOn = due
		a.SinceAt = since
		a.DeepLink = deepLink(a.TargetID)
		out = append(out, a)
	}
	return out, rows.Err()
}

// PendingApprovals is the human-in-the-loop backlog, oldest first.
//
// `ownerUserID` narrows to one salesperson's own drafts. Empty means the whole
// account — the caller decides, because a manager needs the floor's backlog and
// a salesperson needs theirs.
func (r *CommandCenterRepository) PendingApprovals(ctx context.Context, accountID, ownerUserID string, limit int) ([]domain.PendingAction, error) {
	// `updated_at` as since_at: the last time the draft was written, which
	// is the closest thing to "when it entered the queue" without a column
	// for it. It MOVES on a redraft, so this age is only comparable against
	// other drafts — which is all RankPendingActions uses it for now that
	// kind is ranked ahead of age. Do not compare it against another kind's
	// age; the unowned-lead age comes from an immutable created_at.
	sql := `SELECT l.id, COALESCE(ct.name, ''), COALESCE(l.source, ''),
	               0::BIGINT, '', NULL::TIMESTAMPTZ, l.updated_at
	        FROM leads l
	        LEFT JOIN contacts ct ON ct.id = l.contact_id
	        WHERE l.account_id = $1 AND l.status = 'pending_approval'`
	args := []any{accountID}
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		sql += ` AND l.owner_user_id = $2`
	}
	args = append(args, limit)
	sql += fmt.Sprintf(` ORDER BY l.updated_at ASC LIMIT $%d`, len(args))

	return r.scanActions(ctx, domain.PendingApproveDraft,
		func(string) string { return "/prospect?tab=queue" }, sql, args...)
}

// UnownedLeads is live leads nobody is responsible for.
func (r *CommandCenterRepository) UnownedLeads(ctx context.Context, accountID string, limit int) ([]domain.PendingAction, error) {
	return r.scanActions(ctx, domain.PendingUnownedLead,
		func(id string) string { return "/funnel?lead=" + id },
		`SELECT l.id, COALESCE(ct.name, ''), COALESCE(l.source, ''),
		        0::BIGINT, '', NULL::TIMESTAMPTZ, l.created_at
		 FROM leads l
		 LEFT JOIN contacts ct ON ct.id = l.contact_id
		 WHERE l.account_id = $1
		   AND l.owner_user_id IS NULL
		   AND l.status NOT IN ('won', 'lost')
		 ORDER BY l.created_at ASC
		 LIMIT $2`,
		accountID, limit)
}

// ViewedSignatures is documents the customer OPENED and did not sign.
//
// The highest-intent state in the product: they looked. Ranked by the value of
// the document, which is why the amount travels with the row.
func (r *CommandCenterRepository) ViewedSignatures(ctx context.Context, accountID string, limit int) ([]domain.PendingAction, error) {
	return r.scanActions(ctx, domain.PendingSignatureViewed,
		func(string) string { return "/settings/signatures" },
		`SELECT s.id, COALESCE(ct.name, ''), s.title,
		        s.value_cents, 'brl', NULL::TIMESTAMPTZ, s.viewed_at
		 FROM signature_requests s
		 LEFT JOIN contacts ct ON ct.id = s.contact_id
		 WHERE s.account_id = $1
		   AND s.status = 'viewed'
		 ORDER BY s.value_cents DESC, s.viewed_at ASC
		 LIMIT $2`,
		accountID, limit)
}

// ContractAlerts is the pending contract deadlines from the daily sweep (G6).
func (r *CommandCenterRepository) ContractAlerts(ctx context.Context, accountID string, limit int) ([]domain.PendingAction, error) {
	return r.scanActions(ctx, domain.PendingContractAlert,
		// Deep-links to the CONTRACT, not the alert: the alert is the
		// notification, the contract is where somebody acts.
		func(string) string { return "/settings/contracts/alerts" },
		`SELECT a.id, COALESCE(ct.name, ''), a.title,
		        a.money_at_stake_cents, 'brl', a.due_on, a.created_at
		 FROM contract_alerts a
		 JOIN contracts c ON c.id = a.contract_id
		 LEFT JOIN contacts ct ON ct.id = c.contact_id
		 WHERE a.account_id = $1
		   AND a.status = 'pending'
		 ORDER BY a.due_on ASC
		 LIMIT $2`,
		accountID, limit)
}

// UnconfirmedSales is AI-extracted sales no human has signed off.
//
// Until confirmed they pay nobody a commission and enter no portfolio, so the
// amount at stake is the sale itself — this is money already earned and not yet
// counted.
func (r *CommandCenterRepository) UnconfirmedSales(ctx context.Context, accountID string, limit int) ([]domain.PendingAction, error) {
	return r.scanActions(ctx, domain.PendingConfirmSale,
		func(string) string { return "/sales/portfolio" },
		`SELECT s.id, COALESCE(ct.name, ''), COALESCE(s.notes, ''),
		        s.gross_cents, s.currency, NULL::TIMESTAMPTZ, s.created_at
		 FROM sales s
		 LEFT JOIN contacts ct ON ct.id = s.contact_id
		 WHERE s.account_id = $1
		   AND s.source = 'ai_extracted'
		   AND COALESCE(s.confirmed_by_user_id, '') = ''
		 ORDER BY s.gross_cents DESC, s.created_at ASC
		 LIMIT $2`,
		accountID, limit)
}

// DueReorders is customers past the cycle their last purchase should have
// lasted. The amount is what they bought last time — the best available estimate
// of what this call is worth.
func (r *CommandCenterRepository) DueReorders(ctx context.Context, accountID string, limit int) ([]domain.PendingAction, error) {
	return r.scanActions(ctx, domain.PendingReorderDue,
		func(id string) string { return "/sales/relationship" },
		`SELECT n.id, COALESCE(ct.name, ''), '',
		        COALESCE(s.gross_cents, 0)::BIGINT, COALESCE(s.currency, ''),
		        n.due_at::date, n.created_at
		 FROM reorder_nudges n
		 LEFT JOIN contacts ct ON ct.id = n.contact_id
		 LEFT JOIN sales s ON s.id = n.sale_id
		 WHERE n.account_id = $1
		   AND n.due_at <= NOW()
		 ORDER BY n.due_at ASC
		 LIMIT $2`,
		accountID, limit)
}

// WaitingReplies is customers already talking whose last message nobody
// answered. waiting_since is set by the inbound and cleared by a human
// reply/takeover, so "still set on an open conversation" IS the definition
// of a customer waiting — no message-table scan needed.
func (r *CommandCenterRepository) WaitingReplies(ctx context.Context, accountID, ownerUserID string, limit int) ([]domain.PendingAction, error) {
	sql := `SELECT c.id, COALESCE(c.contact_name, c.contact_id), COALESCE(ch.name, ''),
	               0::BIGINT, '', NULL::TIMESTAMPTZ, c.waiting_since
	        FROM conversations c
	        LEFT JOIN channels ch ON ch.id = c.channel_id
	        WHERE c.account_id = $1 AND c.status = 'open'
	          AND c.waiting_since IS NOT NULL`
	args := []any{accountID}
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		// The attendant's own assignments plus the unassigned pile: an
		// unclaimed customer is everybody's silence.
		sql += ` AND (c.assigned_to = $2 OR c.assigned_to IS NULL)`
	}
	args = append(args, limit)
	sql += fmt.Sprintf(` ORDER BY c.waiting_since ASC LIMIT $%d`, len(args))

	return r.scanActions(ctx, domain.PendingWaitingReply,
		func(string) string { return "/home" }, sql, args...)
}

// PeriodTotals counts the two tiles that need a window: meetings booked and
// gross closed.
//
// Meetings come from the sales event log (the same source the case report uses)
// and the gross from `sales`, so the home screen and the SDR report can never
// disagree about the month.
func (r *CommandCenterRepository) PeriodTotals(ctx context.Context, accountID string, from, to time.Time) (int64, int64, string, error) {
	var meetings, gross int64
	var currency string
	err := r.db.QueryRow(ctx,
		`SELECT
		     (SELECT COUNT(*) FROM sales_events e
		      WHERE e.account_id = $1 AND e.kind = 'meeting_booked'
		        AND e.occurred_at >= $2 AND e.occurred_at < $3)::BIGINT,
		     COALESCE((SELECT SUM(s.gross_cents) FROM sales s
		      WHERE s.account_id = $1
		        AND s.sold_at >= $2 AND s.sold_at < $3), 0)::BIGINT,
		     COALESCE((SELECT MAX(s.currency) FROM sales s
		      WHERE s.account_id = $1
		        AND s.sold_at >= $2 AND s.sold_at < $3), '')`,
		accountID, from, to,
	).Scan(&meetings, &gross, &currency)
	return meetings, gross, currency, err
}

// LiveDismissals is this user's un-expired snoozes.
func (r *CommandCenterRepository) LiveDismissals(ctx context.Context, accountID, userID string, now time.Time) (map[domain.DismissalKey]struct{}, error) {
	rows, err := r.db.Query(ctx,
		`SELECT kind, target_id
		 FROM pending_action_dismissals
		 WHERE account_id = $1 AND user_id = $2 AND snoozed_until > $3`,
		accountID, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[domain.DismissalKey]struct{}{}
	for rows.Next() {
		var kind, target string
		if err := rows.Scan(&kind, &target); err != nil {
			return nil, err
		}
		out[domain.DismissalKey{
			Kind:     domain.PendingActionKind(kind),
			TargetID: target,
		}] = struct{}{}
	}
	return out, rows.Err()
}

// Dismiss upserts a snooze.
//
// ON CONFLICT moves the deadline rather than stacking rows: re-snoozing the same
// thing is the common case (it came back, the answer is still "not now"), and a
// pile of historical dismissals would make the filter read grow forever.
func (r *CommandCenterRepository) Dismiss(ctx context.Context, d *domain.PendingActionDismissal) error {
	if d.ID == "" {
		d.ID = ulid.Make().String()
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO pending_action_dismissals
		     (id, account_id, user_id, kind, target_id, snoozed_until, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (account_id, user_id, kind, target_id) DO UPDATE
		 SET snoozed_until = EXCLUDED.snoozed_until,
		     note          = EXCLUDED.note`,
		d.ID, d.AccountID, d.UserID, string(d.Kind), d.TargetID,
		d.SnoozedUntil, d.Note)
	return err
}
