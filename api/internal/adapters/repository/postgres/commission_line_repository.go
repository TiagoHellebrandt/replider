package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Commission entries: the customer's own monthly statement (§6.6, G2).
//
// Raw SQL, like the sales table it reads from.
type CommissionLineRepository struct{ db DBTX }

func NewCommissionLineRepository(db DBTX) *CommissionLineRepository {
	return &CommissionLineRepository{db: db}
}

// clientCommissionLockClass keeps this table's advisory locks in their own
// namespace, away from the platform's own seller ledger. Two features called
// "commission" serializing on the same key would make one customer's month
// close wait on ours.
const clientCommissionLockClass = "client_commission_cycle"

const commissionLineCols = `e.id, e.account_id, e.cycle, e.seller_user_id,
	e.kind, e.sale_id, e.rule_id, e.role, e.base_cents, e.rate_percent,
	e.amount_cents, e.explanation, e.note, e.status, e.paid_at,
	e.created_at, e.updated_at, COALESCE(u.name, '')`

func scanCommissionLine(row pgx.Row) (*domain.CommissionLine, error) {
	e := &domain.CommissionLine{}
	var kind, status string
	var saleID, ruleID *string

	err := row.Scan(&e.ID, &e.AccountID, &e.Cycle, &e.SellerUserID,
		&kind, &saleID, &ruleID, &e.Role, &e.BaseCents, &e.RatePercent,
		&e.AmountCents, &e.Explanation, &e.Note, &status, &e.PaidAt,
		&e.CreatedAt, &e.UpdatedAt, &e.SellerName)
	if err != nil {
		return nil, err
	}
	e.Kind = domain.CommissionLineKind(kind)
	e.Status = domain.CommissionLineStatus(status)
	if saleID != nil {
		e.SaleID = *saleID
	}
	if ruleID != nil {
		e.RuleID = *ruleID
	}
	return e, nil
}

// ReplacePendingForCycle rewrites a cycle's pending lines in ONE transaction.
//
// Delete-then-insert rather than upsert: a recomputation can legitimately
// produce fewer lines than last time (a sale was deleted, a rule turned off),
// and an upsert would leave the vanished ones behind, paying for work that no
// longer exists.
//
// Paid lines are never in the delete's scope. That is the whole immutability
// guarantee, and it lives in this WHERE clause.
func (r *CommissionLineRepository) ReplacePendingForCycle(
	ctx context.Context, accountID, cycle string, entries []*domain.CommissionLine,
) error {
	b, ok := r.db.(txBeginner)
	if !ok {
		// Already transaction-bound (nested call): the outer tx holds the lock.
		return r.replacePending(ctx, r.db, accountID, cycle, entries)
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	// One writer per (account, cycle). Two concurrent closes would otherwise
	// both delete and both insert, and the month would end up doubled.
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`,
		clientCommissionLockClass, accountID+"|"+cycle); err != nil {
		return err
	}
	if err := r.replacePending(ctx, tx, accountID, cycle, entries); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *CommissionLineRepository) replacePending(
	ctx context.Context, db DBTX, accountID, cycle string, entries []*domain.CommissionLine,
) error {
	if _, err := db.Exec(ctx,
		`DELETE FROM commission_lines
		  WHERE account_id = $1 AND cycle = $2 AND status = 'pending'
		    AND note = ''`, accountID, cycle); err != nil {
		return err
	}
	for _, e := range entries {
		if e == nil {
			continue
		}
		if err := insertCommissionLine(ctx, db, e); err != nil {
			return err
		}
	}
	return nil
}

// insertCommissionLine is shared by the recomputation and by a manual
// adjustment, so both write the same shape.
func insertCommissionLine(ctx context.Context, db DBTX, e *domain.CommissionLine) error {
	var paidAt *time.Time
	if e.Status == domain.CommissionPaid {
		paidAt = e.PaidAt
	}
	_, err := db.Exec(ctx,
		`INSERT INTO commission_lines (id, account_id, cycle, seller_user_id,
		    kind, sale_id, rule_id, role, base_cents, rate_percent,
		    amount_cents, explanation, note, status, paid_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		e.ID, e.AccountID, e.Cycle, e.SellerUserID, string(e.Kind),
		nilIfEmpty(e.SaleID), nilIfEmpty(e.RuleID), e.Role,
		e.BaseCents, e.RatePercent, e.AmountCents,
		e.Explanation, e.Note, string(e.Status), paidAt)
	return err
}

// Insert adds one line. Used for adjustments, which a recomputation must not
// wipe — hence the note-bearing rows being out of the delete's scope above.
func (r *CommissionLineRepository) Insert(ctx context.Context, e *domain.CommissionLine) error {
	return insertCommissionLine(ctx, r.db, e)
}

func (r *CommissionLineRepository) ListByCycle(
	ctx context.Context, accountID, cycle, sellerUserID string,
) ([]*domain.CommissionLine, error) {
	// LEFT JOIN: the line survives the salesperson's user row being deleted,
	// and a statement that will not load is worse than one missing a name.
	rows, err := r.db.Query(ctx,
		`SELECT `+commissionLineCols+`
		   FROM commission_lines e
		   LEFT JOIN users u ON u.id = e.seller_user_id
		  WHERE e.account_id = $1 AND e.cycle = $2
		    AND ($3 = '' OR e.seller_user_id = $3)
		  ORDER BY e.seller_user_id, e.kind, e.created_at`,
		accountID, cycle, sellerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.CommissionLine
	for rows.Next() {
		e, err := scanCommissionLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *CommissionLineRepository) SumPaidByCycle(
	ctx context.Context, accountID, cycle string,
) (map[string]int64, error) {
	rows, err := r.db.Query(ctx,
		`SELECT seller_user_id, COALESCE(SUM(amount_cents), 0)
		   FROM commission_lines
		  WHERE account_id = $1 AND cycle = $2 AND status = 'paid'
		  GROUP BY seller_user_id`, accountID, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var seller string
		var cents int64
		if err := rows.Scan(&seller, &cents); err != nil {
			return nil, err
		}
		out[seller] = cents
	}
	return out, rows.Err()
}

// MarkPaid settles a seller's pending lines and returns them.
//
// UPDATE ... RETURNING, so what comes back is what the database actually
// changed rather than what the caller believed a moment earlier — a concurrent
// recomputation could have moved the numbers between the read and the write.
func (r *CommissionLineRepository) MarkPaid(
	ctx context.Context, accountID, cycle, sellerUserID, note string, at time.Time,
) ([]*domain.CommissionLine, int64, error) {
	rows, err := r.db.Query(ctx,
		`UPDATE commission_lines
		    SET status = 'paid', paid_at = $4,
		        note = CASE WHEN $5 = '' THEN note
		                    WHEN note = '' THEN $5
		                    ELSE note || ' · ' || $5 END,
		        updated_at = NOW()
		  WHERE account_id = $1 AND cycle = $2 AND seller_user_id = $3
		    AND status = 'pending'
		  RETURNING id, amount_cents`,
		accountID, cycle, sellerUserID, at, note)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ids []string
	var total int64
	for rows.Next() {
		var id string
		var cents int64
		if err := rows.Scan(&id, &cents); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
		total += cents
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return nil, 0, nil
	}

	settled, err := r.listByIDs(ctx, accountID, ids)
	if err != nil {
		return nil, 0, err
	}
	return settled, total, nil
}

func (r *CommissionLineRepository) listByIDs(
	ctx context.Context, accountID string, ids []string,
) ([]*domain.CommissionLine, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+commissionLineCols+`
		   FROM commission_lines e
		   LEFT JOIN users u ON u.id = e.seller_user_id
		  WHERE e.account_id = $1 AND e.id = ANY($2)
		  ORDER BY e.kind, e.created_at`, accountID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.CommissionLine
	for rows.Next() {
		e, err := scanCommissionLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PendingTotalForSeller is the salesperson's own running total (G3).
func (r *CommissionLineRepository) PendingTotalForSeller(
	ctx context.Context, accountID, cycle, sellerUserID string,
) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount_cents), 0) FROM commission_lines
		  WHERE account_id = $1 AND cycle = $2 AND seller_user_id = $3
		    AND status = 'pending'`,
		accountID, cycle, sellerUserID).Scan(&total)
	return total, err
}

var _ domain.CommissionLineRepository = (*CommissionLineRepository)(nil)
