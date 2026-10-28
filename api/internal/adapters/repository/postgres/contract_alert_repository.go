package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Clause alerts and the recovered-revenue ledger (§6.11, G5/G6).
type ContractAlertRepository struct{ db DBTX }

func NewContractAlertRepository(db DBTX) *ContractAlertRepository {
	return &ContractAlertRepository{db: db}
}

const contractAlertCols = `a.id, a.account_id, a.contract_id, a.kind, a.title,
	a.detail, a.due_on, a.money_at_stake_cents, a.status, a.done_at, a.done_note,
	a.lead_id, a.last_notified_at, a.created_by_user_id, a.created_at, a.updated_at,
	COALESCE(c.title, '')`

func scanContractAlert(row pgx.Row) (*domain.ContractAlert, error) {
	a := &domain.ContractAlert{}
	var kind, status string
	var leadID, createdBy *string

	err := row.Scan(&a.ID, &a.AccountID, &a.ContractID, &kind, &a.Title,
		&a.Detail, &a.DueOn, &a.MoneyAtStakeCents, &status, &a.DoneAt, &a.DoneNote,
		&leadID, &a.LastNotifiedAt, &createdBy, &a.CreatedAt, &a.UpdatedAt,
		&a.ContractTitle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	a.Kind = domain.ContractAlertKind(kind)
	a.Status = domain.ContractAlertStatus(status)
	if leadID != nil {
		a.LeadID = *leadID
	}
	if createdBy != nil {
		a.CreatedByUserID = *createdBy
	}
	return a, nil
}

// Create writes one alert, or returns the pending one that already covers the
// same deadline.
//
// Asking twice for a task on the same day is not an error — it is the same task,
// and the worker and the 1-click button both legitimately ask. The partial
// unique index makes the second call a no-op and the RETURNING-less path reads
// the winner back.
func (r *ContractAlertRepository) Create(
	ctx context.Context, a *domain.ContractAlert,
) (*domain.ContractAlert, error) {
	rows, err := r.db.Query(ctx,
		`INSERT INTO contract_alerts (id, account_id, contract_id, kind, title,
		    detail, due_on, money_at_stake_cents, created_by_user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (contract_id, kind, due_on) WHERE status = 'pending'
		    DO NOTHING
		 RETURNING id`,
		a.ID, a.AccountID, a.ContractID, string(a.Kind), a.Title,
		a.Detail, a.DueOn, a.MoneyAtStakeCents, nilIfEmpty(a.CreatedByUserID))
	if err != nil {
		return nil, err
	}
	inserted := rows.Next()
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if inserted {
		return r.Get(ctx, a.AccountID, a.ID)
	}
	// Somebody (or the worker) got there first: hand back THAT alert, so the
	// caller's "agendado" chip points at a row that exists.
	return scanContractAlert(r.db.QueryRow(ctx,
		`SELECT `+contractAlertCols+`
		   FROM contract_alerts a
		   LEFT JOIN contracts c ON c.id = a.contract_id
		  WHERE a.account_id = $1 AND a.contract_id = $2 AND a.kind = $3
		    AND a.due_on = $4 AND a.status = 'pending'`,
		a.AccountID, a.ContractID, string(a.Kind), a.DueOn))
}

func (r *ContractAlertRepository) Get(ctx context.Context, accountID, id string) (*domain.ContractAlert, error) {
	return scanContractAlert(r.db.QueryRow(ctx,
		`SELECT `+contractAlertCols+`
		   FROM contract_alerts a
		   LEFT JOIN contracts c ON c.id = a.contract_id
		  WHERE a.id = $1 AND a.account_id = $2`, id, accountID))
}

func (r *ContractAlertRepository) ListByContract(
	ctx context.Context, accountID, contractID string,
) ([]*domain.ContractAlert, error) {
	return r.query(ctx,
		`SELECT `+contractAlertCols+`
		   FROM contract_alerts a
		   LEFT JOIN contracts c ON c.id = a.contract_id
		  WHERE a.account_id = $1 AND a.contract_id = $2
		  ORDER BY a.due_on`, accountID, contractID)
}

// ListDue is the account's pending deadlines, soonest first.
//
// Overdue rows come first because they sort earliest — deliberately: a deadline
// that passed is more urgent than one coming, and burying it below "today" is how
// it stays missed.
func (r *ContractAlertRepository) ListDue(
	ctx context.Context, accountID string, through time.Time, limit int,
) ([]*domain.ContractAlert, error) {
	if limit <= 0 {
		limit = 50
	}
	return r.query(ctx,
		`SELECT `+contractAlertCols+`
		   FROM contract_alerts a
		   LEFT JOIN contracts c ON c.id = a.contract_id
		  WHERE a.account_id = $1 AND a.status = 'pending' AND a.due_on <= $2
		  ORDER BY a.due_on, a.money_at_stake_cents DESC
		  LIMIT $3`, accountID, through, limit)
}

func (r *ContractAlertRepository) Complete(
	ctx context.Context, accountID, id string, status domain.ContractAlertStatus,
	note string, at time.Time,
) (*domain.ContractAlert, error) {
	// WHERE status = 'pending': completing an alert twice must not restamp the
	// date it was closed, which is the field an audit reads.
	_, err := r.db.Exec(ctx,
		`UPDATE contract_alerts
		    SET status = $3, done_at = $4, done_note = $5, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2 AND status = 'pending'`,
		id, accountID, string(status), at, note)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, accountID, id)
}

func (r *ContractAlertRepository) AttachLead(ctx context.Context, accountID, id, leadID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE contract_alerts SET lead_id = $3, updated_at = NOW()
		  WHERE id = $1 AND account_id = $2`, id, accountID, nilIfEmpty(leadID))
	return err
}

func (r *ContractAlertRepository) MarkNotified(
	ctx context.Context, accountID string, ids []string, at time.Time,
) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE contract_alerts SET last_notified_at = $3
		  WHERE account_id = $1 AND id = ANY($2)`, accountID, ids, at)
	return err
}

func (r *ContractAlertRepository) query(
	ctx context.Context, sql string, args ...any,
) ([]*domain.ContractAlert, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.ContractAlert
	for rows.Next() {
		a, err := scanContractAlert(rows)
		if err != nil {
			return nil, err
		}
		if a != nil {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}

var _ domain.ContractAlertRepository = (*ContractAlertRepository)(nil)

// ── The recovered-revenue ledger ───────────────────────────────────────────

type ContractRecoveryRepository struct{ db DBTX }

func NewContractRecoveryRepository(db DBTX) *ContractRecoveryRepository {
	return &ContractRecoveryRepository{db: db}
}

func (r *ContractRecoveryRepository) Create(ctx context.Context, rec *domain.ContractRecovery) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO contract_recoveries (id, account_id, contract_id, alert_id,
		    kind, amount_cents, note, recorded_by_user_id, recognised_on)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		rec.ID, rec.AccountID, rec.ContractID, nilIfEmpty(rec.AlertID),
		string(rec.Kind), rec.AmountCents, rec.Note,
		nilIfEmpty(rec.RecordedByUserID), rec.RecognisedOn)
	return err
}

func (r *ContractRecoveryRepository) List(
	ctx context.Context, accountID string, from, to time.Time, limit int,
) ([]*domain.ContractRecovery, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(ctx,
		`SELECT r.id, r.account_id, r.contract_id, r.alert_id, r.kind,
		        r.amount_cents, r.note, r.recorded_by_user_id, r.recognised_on,
		        r.created_at, COALESCE(c.title, '')
		   FROM contract_recoveries r
		   LEFT JOIN contracts c ON c.id = r.contract_id
		  WHERE r.account_id = $1 AND r.recognised_on >= $2 AND r.recognised_on < $3
		  ORDER BY r.recognised_on DESC, r.created_at DESC
		  LIMIT $4`, accountID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.ContractRecovery
	for rows.Next() {
		rec := &domain.ContractRecovery{}
		var kind string
		var alertID, recordedBy *string
		if err := rows.Scan(&rec.ID, &rec.AccountID, &rec.ContractID, &alertID,
			&kind, &rec.AmountCents, &rec.Note, &recordedBy, &rec.RecognisedOn,
			&rec.CreatedAt, &rec.ContractTitle); err != nil {
			return nil, err
		}
		rec.Kind = domain.ContractAlertKind(kind)
		if alertID != nil {
			rec.AlertID = *alertID
		}
		if recordedBy != nil {
			rec.RecordedByUserID = *recordedBy
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Summary is the headline number, split by what it plugged.
//
// The split matters: "recuperei R$400k" lands differently when it is R$300k of
// forgotten index (a leak that was running every month) and R$100k of renewals
// saved (a one-off).
func (r *ContractRecoveryRepository) Summary(
	ctx context.Context, accountID string, from, to time.Time,
) (*domain.ContractRecoverySummary, error) {
	rows, err := r.db.Query(ctx,
		`SELECT kind, COALESCE(SUM(amount_cents), 0), COUNT(*)
		   FROM contract_recoveries
		  WHERE account_id = $1 AND recognised_on >= $2 AND recognised_on < $3
		  GROUP BY kind`, accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &domain.ContractRecoverySummary{From: from, To: to, ByKind: map[string]int64{}}
	for rows.Next() {
		var kind string
		var cents int64
		var count int
		if err := rows.Scan(&kind, &cents, &count); err != nil {
			return nil, err
		}
		out.ByKind[kind] = cents
		out.TotalCents += cents
		out.Count += count
	}
	return out, rows.Err()
}

var _ domain.ContractRecoveryRepository = (*ContractRecoveryRepository)(nil)
