package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type ScamReportRepository struct{ db DBTX }

func NewScamReportRepository(db DBTX) *ScamReportRepository {
	return &ScamReportRepository{db: db}
}

// Report records one company's verdict.
//
// Idempotent per (account, domain): a single customer must not be able to
// push a domain over the network threshold by reporting it repeatedly. A
// second report updates the reason — someone adding detail is improving the
// record, not casting a second vote.
func (r *ScamReportRepository) Report(ctx context.Context, rep *domain.ScamReport) error {
	var msgID any
	if rep.MessageID != "" {
		msgID = rep.MessageID
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO scam_reports
		   (id, account_id, message_id, sender_domain, payment_values, reason)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (account_id, sender_domain) DO UPDATE
		    SET reason = CASE WHEN EXCLUDED.reason <> '' THEN EXCLUDED.reason
		                      ELSE scam_reports.reason END,
		        payment_values = EXCLUDED.payment_values`,
		rep.ID, rep.AccountID, msgID, rep.SenderDomain, rep.PaymentValues, rep.Reason)
	return err
}

// CountReportingAccounts excludes the asking account: a company's own report
// is not corroboration of itself, and counting it would let one customer
// quarantine a domain for their own inbox and call it a network verdict.
func (r *ScamReportRepository) CountReportingAccounts(ctx context.Context, senderDomain, exceptAccountID string) (int, error) {
	if senderDomain == "" {
		return 0, nil
	}
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT account_id) FROM scam_reports
		  WHERE sender_domain = $1 AND account_id <> $2`,
		senderDomain, exceptAccountID).Scan(&n)
	return n, err
}

// CountByPaymentValue catches the scammer who burns a domain but keeps the
// bank account, which is the cheaper half of their setup to reuse.
func (r *ScamReportRepository) CountByPaymentValue(ctx context.Context, value, exceptAccountID string) (int, error) {
	if value == "" {
		return 0, nil
	}
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT account_id) FROM scam_reports
		  WHERE $1 = ANY(payment_values) AND account_id <> $2`,
		value, exceptAccountID).Scan(&n)
	return n, err
}
