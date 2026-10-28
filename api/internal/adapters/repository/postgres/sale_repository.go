package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Raw SQL, like the other tables sqlc does not own.
type SaleRepository struct{ db DBTX }

func NewSaleRepository(db DBTX) *SaleRepository {
	return &SaleRepository{db: db}
}

const saleCols = `id, account_id, contact_id, lead_id, seller_user_id, items,
	gross_cents, cost_cents, tax_cents, currency, sold_at, cycle_days,
	source, confirmed_by_user_id, notes, created_at, updated_at`

func scanSale(row pgx.Row) (*domain.Sale, error) {
	s := &domain.Sale{}
	var leadID, sellerID, confirmedBy *string
	var source string
	var items []byte
	err := row.Scan(&s.ID, &s.AccountID, &s.ContactID, &leadID, &sellerID,
		&items, &s.GrossCents, &s.CostCents, &s.TaxCents, &s.Currency,
		&s.SoldAt, &s.CycleDays, &source, &confirmedBy, &s.Notes,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	s.Source = domain.SaleSource(source)
	if leadID != nil {
		s.LeadID = *leadID
	}
	if sellerID != nil {
		s.SellerUserID = *sellerID
	}
	if confirmedBy != nil {
		s.ConfirmedByUserID = *confirmedBy
	}
	if len(items) > 0 {
		_ = json.Unmarshal(items, &s.Items)
	}
	return s, nil
}

func (r *SaleRepository) Create(ctx context.Context, s *domain.Sale) error {
	items, err := json.Marshal(s.Items)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO sales (id, account_id, contact_id, lead_id, seller_user_id,
		    items, gross_cents, cost_cents, tax_cents, currency, sold_at,
		    cycle_days, source, confirmed_by_user_id, notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		s.ID, s.AccountID, s.ContactID, nilIfEmpty(s.LeadID),
		nilIfEmpty(s.SellerUserID), items, s.GrossCents, s.CostCents,
		s.TaxCents, s.Currency, s.SoldAt, s.CycleDays, string(s.Source),
		nilIfEmpty(s.ConfirmedByUserID), s.Notes)
	return err
}

func (r *SaleRepository) Get(ctx context.Context, id, accountID string) (*domain.Sale, error) {
	return scanSale(r.db.QueryRow(ctx,
		`SELECT `+saleCols+` FROM sales WHERE id = $1 AND account_id = $2`,
		id, accountID))
}

// Update rewrites the editable fields.
//
// The SOURCE is deliberately not among them: a sale a model proposed and a
// human confirmed stays "ai_extracted" forever, because that is how it got
// here and an audit that can be rewritten is not an audit.
func (r *SaleRepository) Update(ctx context.Context, s *domain.Sale) error {
	items, err := json.Marshal(s.Items)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE sales
		    SET contact_id     = $3,
		        lead_id        = $4,
		        seller_user_id = $5,
		        items          = $6,
		        gross_cents    = $7,
		        cost_cents     = $8,
		        tax_cents      = $9,
		        sold_at        = $10,
		        cycle_days     = $11,
		        notes          = $12,
		        updated_at     = NOW()
		  WHERE id = $1 AND account_id = $2`,
		s.ID, s.AccountID, s.ContactID, nilIfEmpty(s.LeadID),
		nilIfEmpty(s.SellerUserID), items, s.GrossCents, s.CostCents,
		s.TaxCents, s.SoldAt, s.CycleDays, s.Notes)
	return err
}

func (r *SaleRepository) Delete(ctx context.Context, id, accountID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM sales WHERE id = $1 AND account_id = $2`, id, accountID)
	return err
}

func (r *SaleRepository) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.Sale, error) {
	return r.query(ctx,
		`SELECT `+saleCols+` FROM sales
		  WHERE account_id = $1
		  ORDER BY sold_at DESC
		  LIMIT $2 OFFSET $3`, accountID, limit, offset)
}

// ListByContact is the portfolio view: everything one customer bought.
func (r *SaleRepository) ListByContact(ctx context.Context, accountID, contactID string) ([]*domain.Sale, error) {
	return r.query(ctx,
		`SELECT `+saleCols+` FROM sales
		  WHERE account_id = $1 AND contact_id = $2
		  ORDER BY sold_at DESC`, accountID, contactID)
}

// ListByPeriod is every sale of a month, OLDEST FIRST (§6.6 G2).
//
// The order is the contract, not a detail: the commission engine walks the
// month accumulating the period total, and a tier reached mid-month has to pay
// at the tier the sale reached. Newest-first would pay the whole month at the
// top rate.
func (r *SaleRepository) ListByPeriod(ctx context.Context, accountID string, from, to time.Time) ([]*domain.Sale, error) {
	return r.query(ctx,
		`SELECT `+saleCols+` FROM sales
		  WHERE account_id = $1 AND sold_at >= $2 AND sold_at < $3
		  ORDER BY sold_at ASC, id ASC`, accountID, from, to)
}

// CountByContactBefore is how many times a customer bought before a moment.
//
// What "recompra" turns on. Strictly before, so the sale being calculated
// never counts itself as its own precedent.
func (r *SaleRepository) CountByContactBefore(
	ctx context.Context, accountID, contactID string, before time.Time,
) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM sales
		  WHERE account_id = $1 AND contact_id = $2 AND sold_at < $3`,
		accountID, contactID, before).Scan(&count)
	return count, err
}

func (r *SaleRepository) query(ctx context.Context, sql string, args ...any) ([]*domain.Sale, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Sale
	for rows.Next() {
		s, err := scanSale(rows)
		if err != nil {
			return nil, err
		}
		if s != nil {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

// Totals sums the period, keeping the three amounts apart.
//
// Never a single "revenue" number: what a company keeps is gross minus cost
// minus tax, and a report that shows only gross is the one that makes a
// discounted month look like a good one.
func (r *SaleRepository) Totals(ctx context.Context, accountID string, from, to time.Time) (domain.SaleTotals, error) {
	var t domain.SaleTotals
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(gross_cents), 0),
		        COALESCE(SUM(cost_cents), 0),
		        COALESCE(SUM(tax_cents), 0)
		   FROM sales
		  WHERE account_id = $1 AND sold_at >= $2 AND sold_at < $3`,
		accountID, from, to).
		Scan(&t.Count, &t.GrossCents, &t.CostCents, &t.TaxCents)
	return t, err
}

// HasSale answers the inbox-triage question with one boolean.
//
// EXISTS rather than a count: the classifier only needs to know whether this
// contact is a customer, and counting every sale on every inbound message
// would be a wasted read per message.
func (r *SaleRepository) HasSale(ctx context.Context, accountID, contactID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM sales WHERE account_id = $1 AND contact_id = $2)`,
		accountID, contactID).Scan(&exists)
	return exists, err
}

// nilIfEmpty maps an empty string to a NULL, so an absent lead or seller is
// stored as absent rather than as an empty id that joins to nothing.
func nilIfEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

var _ domain.SaleRepository = (*SaleRepository)(nil)
