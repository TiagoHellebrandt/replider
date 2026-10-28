package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// ProposalRepository persists priced offers.
//
// Raw SQL rather than sqlc because a proposal is always read WITH its lines, and
// the item fetch is a second query keyed on the ids just returned — the shape
// sqlc generates for that is a join whose rows have to be regrouped in Go
// anyway.
type ProposalRepository struct {
	db *pgxpool.Pool
}

func NewProposalRepository(db *pgxpool.Pool) *ProposalRepository {
	return &ProposalRepository{db: db}
}

// Create writes the proposal and its lines in ONE transaction. A proposal with
// no lines is not a smaller proposal, it is a total of zero presented as an
// offer — so the two writes must not be separable.
func (r *ProposalRepository) Create(ctx context.Context, p *domain.Proposal) error {
	if len(p.Items) == 0 {
		return domain.ErrProposalNoItems
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO proposals (id, account_id, lead_id, status, currency, body)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, p.ID, p.AccountID, p.LeadID, string(p.Status), p.Currency, p.Body)
	if err != nil {
		return fmt.Errorf("insert proposal: %w", err)
	}

	if err := insertItems(ctx, tx, p.ID, p.Items); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func insertItems(ctx context.Context, tx pgx.Tx, proposalID string, items []domain.ProposalItem) error {
	for i := range items {
		if items[i].ID == "" {
			items[i].ID = ulid.Make().String()
		}
		// A snapshot of a product that has since been deleted keeps its name and
		// price; product_id goes NULL rather than dangling.
		var productID interface{}
		if items[i].ProductID != "" {
			productID = items[i].ProductID
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO proposal_items
				(id, proposal_id, product_id, name, quantity, unit_price_cents)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, items[i].ID, proposalID, productID, items[i].Name,
			items[i].Quantity, items[i].UnitPriceCents)
		if err != nil {
			return fmt.Errorf("insert proposal item: %w", err)
		}
	}
	return nil
}

func (r *ProposalRepository) GetByID(ctx context.Context, accountID, id string) (*domain.Proposal, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, account_id, lead_id, status, currency, body, sent_at, created_at, updated_at
		FROM proposals
		WHERE account_id = $1 AND id = $2
	`, accountID, id)

	p, err := scanProposal(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrProposalNotFound
		}
		return nil, err
	}

	items, err := r.itemsFor(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Items = items
	return p, nil
}

// ListByLead answers "what have we already offered this customer?" — newest
// first, because the last offer is the one that shapes the next.
func (r *ProposalRepository) ListByLead(ctx context.Context, accountID, leadID string) ([]*domain.Proposal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, account_id, lead_id, status, currency, body, sent_at, created_at, updated_at
		FROM proposals
		WHERE account_id = $1 AND lead_id = $2
		ORDER BY created_at DESC
	`, accountID, leadID)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()

	var out []*domain.Proposal
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Lines are fetched per proposal rather than in one join: the join returns a
	// row per line and every caller of this list renders totals, so the regroup
	// would happen here either way.
	for _, p := range out {
		items, err := r.itemsFor(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		p.Items = items
	}
	return out, nil
}

func (r *ProposalRepository) itemsFor(ctx context.Context, proposalID string) ([]domain.ProposalItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, COALESCE(product_id, ''), name, quantity, unit_price_cents
		FROM proposal_items
		WHERE proposal_id = $1
		ORDER BY created_at, id
	`, proposalID)
	if err != nil {
		return nil, fmt.Errorf("list proposal items: %w", err)
	}
	defer rows.Close()

	var out []domain.ProposalItem
	for rows.Next() {
		var it domain.ProposalItem
		if err := rows.Scan(&it.ID, &it.ProductID, &it.Name, &it.Quantity, &it.UnitPriceCents); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReplaceItems swaps the whole set inside a transaction, and refuses on a
// proposal that has already gone out. The status check is part of the same
// statement chain rather than a prior read: two salespeople editing the same
// draft while it is being approved is exactly when a check-then-write loses.
func (r *ProposalRepository) ReplaceItems(ctx context.Context, accountID, id string, items []domain.ProposalItem, currency string) error {
	if len(items) == 0 {
		return domain.ErrProposalNoItems
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE proposals
		SET currency = $3, updated_at = NOW()
		WHERE account_id = $1 AND id = $2 AND status = 'draft'
	`, accountID, id, currency)
	if err != nil {
		return fmt.Errorf("touch proposal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return r.absentOrSent(ctx, accountID, id)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM proposal_items WHERE proposal_id = $1`, id); err != nil {
		return fmt.Errorf("clear proposal items: %w", err)
	}
	if err := insertItems(ctx, tx, id, items); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ProposalRepository) UpdateBody(ctx context.Context, accountID, id, body string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE proposals
		SET body = $3, updated_at = NOW()
		WHERE account_id = $1 AND id = $2 AND status = 'draft'
	`, accountID, id, body)
	if err != nil {
		return fmt.Errorf("update proposal body: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return r.absentOrSent(ctx, accountID, id)
	}
	return nil
}

// MarkSent is forward-only: the guard on `sent_at IS NULL` makes a second call a
// no-op rather than restamping the moment an offer left.
func (r *ProposalRepository) MarkSent(ctx context.Context, accountID, id string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE proposals
		SET status = 'sent', sent_at = $3, updated_at = NOW()
		WHERE account_id = $1 AND id = $2 AND sent_at IS NULL
	`, accountID, id, at)
	if err != nil {
		return fmt.Errorf("mark proposal sent: %w", err)
	}
	return nil
}

// CountCreatedSince backs the weekly outcome line on the Ofertas screen.
func (r *ProposalRepository) CountCreatedSince(ctx context.Context, accountID string, since time.Time) (int64, error) {
	var n int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM proposals WHERE account_id = $1 AND created_at >= $2
	`, accountID, since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count proposals since: %w", err)
	}
	return n, nil
}

// absentOrSent turns "no rows updated" into the reason. Reporting "not found"
// for an already-sent proposal would send somebody looking for a missing record
// that is sitting right there.
func (r *ProposalRepository) absentOrSent(ctx context.Context, accountID, id string) error {
	var status string
	err := r.db.QueryRow(ctx, `
		SELECT status FROM proposals WHERE account_id = $1 AND id = $2
	`, accountID, id).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrProposalNotFound
		}
		return err
	}
	return domain.ErrProposalNotEditable
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProposal(row scanner) (*domain.Proposal, error) {
	var p domain.Proposal
	var status string
	var sentAt *time.Time

	if err := row.Scan(
		&p.ID, &p.AccountID, &p.LeadID, &status, &p.Currency,
		&p.Body, &sentAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	p.Status = domain.ProposalStatus(status)
	p.SentAt = sentAt
	return &p, nil
}
