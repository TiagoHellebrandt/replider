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
type PortfolioRepository struct{ db DBTX }

func NewPortfolioRepository(db DBTX) *PortfolioRepository {
	return &PortfolioRepository{db: db}
}

// portfolioSelect is the customer view of sales.
//
// The predicted date is per PRODUCT: for each thing the customer buys, the
// LATEST purchase of it plus its cycle, and the nearest of those is the
// customer's next_due_at. A customer who bought gloves monthly and a
// tomograph once should be chased about gloves — the tomograph has no cycle
// and must not drown out the thing that recurs — and a customer who
// reordered gloves last week must not still show the run-out date of the
// order before it.
const portfolioSelect = `
	SELECT c.id,
	       COALESCE(NULLIF(c.name, ''), c.external_id) AS contact_name,
	       COUNT(s.id)                                  AS purchases,
	       COALESCE(SUM(s.gross_cents), 0)              AS gross_cents,
	       COALESCE(SUM(s.gross_cents - s.cost_cents - s.tax_cents), 0) AS net_cents,
	       MAX(s.sold_at)                               AS last_purchase_at,
	       -- The nearest predicted run-out among the products they still
	       -- buy. A sale with no cycle contributes nothing rather than a guess.
	       (` + portfolioNextDue + `)                   AS next_due_at,
	       -- The relationship's pulse, in either direction: a customer
	       -- nobody has spoken to in four months is one a competitor is
	       -- speaking to.
	       (SELECT MAX(m.created_at)
	          FROM messages m
	          JOIN conversations cv ON cv.id = m.conversation_id
	         WHERE cv.account_id = c.account_id AND cv.contact_ref_id = c.id)
	                                                    AS last_contact_at
	  FROM contacts c
	  -- LEFT: since 2026-09-01 the Carteira is THE customer list (the app's
	  -- /contacts list was merged into it), so a contact who was imported
	  -- and has not bought yet is a row with zero purchases, not an absence.
	  LEFT JOIN sales s ON s.contact_id = c.id AND s.account_id = c.account_id`

// portfolioIsCustomer keeps the Carteira meaning what its header says:
// "seus clientes — importados ou conquistados".
//
// A contact belongs here when we can REACH them, or have transacted with
// them, or have talked to them. Everything else is a prospect, and the
// Carteira is not the prospect list — the Kanban is.
//
// This exists because tender signals mint a contact keyed by CNPJ, with no
// phone and no e-mail, so a public defender's office ended up in a hospital
// equipment distributor's customer list. Nobody sold anything to the
// Defensoria Pública and nobody ever will; the list a salesperson uses to
// plan repeat business must not fill up with bodies that were never
// customers.
//
// Measured on the account it was found in: 52 imported customers all carry a
// phone and none came from a tender; the 17 with no phone and no e-mail
// include all 9 tender-minted ones and 8 registry hospitals nobody has
// contacted. The cut lands exactly on that line.
const portfolioIsCustomer = `(
	     NULLIF(c.phone, '') IS NOT NULL
	  OR NULLIF(c.email, '') IS NOT NULL
	  OR EXISTS (SELECT 1 FROM sales sx
	              WHERE sx.contact_id = c.id AND sx.account_id = c.account_id)
	  OR EXISTS (SELECT 1 FROM conversations cvx
	              WHERE cvx.contact_ref_id = c.id AND cvx.account_id = c.account_id)
	)`

// saleProductLines is one row per (sale, item), with the item's product key
// and the cycle that applies to it.
//
// The key is the catalogue product when the item names one, else the typed
// description; a sale with no items at all is one line keyed on nothing, so
// a sale recorded as just a total still predicts. The cycle is the sale's own
// when it recorded one, else the product's.
const saleProductLines = `
	SELECT s.id AS sale_id, s.account_id, s.contact_id, s.sold_at, s.gross_cents, s.items,
	       COALESCE(NULLIF(item ->> 'product_id', ''),
	                'desc:' || LOWER(TRIM(COALESCE(item ->> 'description', '')))) AS product_key,
	       COALESCE(NULLIF(s.cycle_days, 0), p.consumption_cycle_days, 0) AS cycle_days,
	       CASE WHEN item = '{}'::jsonb THEN '[]'::jsonb ELSE jsonb_build_array(item) END AS line_items
	  FROM sales s
	  CROSS JOIN LATERAL jsonb_array_elements(
	    CASE WHEN jsonb_typeof(s.items) = 'array' AND jsonb_array_length(s.items) > 0
	         THEN s.items ELSE '[{}]'::jsonb END) AS item
	  LEFT JOIN products p
	         ON p.id = NULLIF(item ->> 'product_id', '')
	        AND p.account_id = s.account_id`

// portfolioNextDue is the scalar for one contact: MIN over the products they
// buy of (latest purchase of that product + its cycle), ignoring products
// with no cycle. Correlated on the outer contact row.
const portfolioNextDue = `
	SELECT MIN(latest.sold_at + (latest.cycle_days || ' days')::interval)
	  FROM (
	    SELECT DISTINCT ON (l.product_key) l.sold_at, l.cycle_days
	      FROM (` + saleProductLines + `) l
	     WHERE l.account_id = c.account_id AND l.contact_id = c.id
	     ORDER BY l.product_key, l.sold_at DESC
	  ) latest
	 WHERE latest.cycle_days > 0`

func scanPortfolio(row pgx.Row) (*domain.PortfolioCustomer, error) {
	c := &domain.PortfolioCustomer{}
	err := row.Scan(&c.ContactID, &c.ContactName, &c.Purchases, &c.GrossCents,
		&c.NetCents, &c.LastPurchaseAt, &c.NextDueAt, &c.LastContactAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// List returns the account's customers — bought or merely imported — most
// urgent first.
//
// Ordered in SQL rather than in Go: a portfolio is paginated, and sorting a
// page would put the right customers on the wrong page — the person opening
// this has an hour and the question is who to call FIRST.
//
// The ordering mirrors domain.PortfolioUrgency in the two ways that matter —
// overdue before due-soon, then everything else — and the exact score is
// computed in Go for display. Two expressions of one rule, kept deliberately
// coarse here so they cannot drift into disagreeing about the top of the list.
func (r *PortfolioRepository) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.PortfolioCustomer, error) {
	rows, err := r.db.Query(ctx, portfolioSelect+`
		 WHERE c.account_id = $1
		   AND `+portfolioIsCustomer+`
		 GROUP BY c.id, c.account_id, c.name, c.external_id
		 ORDER BY
		   -- Nulls last: a customer with no prediction is worth listing and
		   -- is never the most urgent thing on the screen.
		   next_due_at ASC NULLS LAST,
		   -- A contact with no sale yet (NULL) sorts after every buyer.
		   MAX(s.sold_at) DESC NULLS LAST,
		   c.name ASC
		 LIMIT $2 OFFSET $3`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.PortfolioCustomer
	for rows.Next() {
		c, err := scanPortfolio(rows)
		if err != nil {
			return nil, err
		}
		if c != nil {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

func (r *PortfolioRepository) Get(ctx context.Context, accountID, contactID string) (*domain.PortfolioCustomer, error) {
	return scanPortfolio(r.db.QueryRow(ctx, portfolioSelect+`
		 WHERE c.account_id = $1 AND c.id = $2
		 GROUP BY c.id, c.account_id, c.name, c.external_id`,
		accountID, contactID))
}

// DueForReorder lists customers predicted to run out within the window.
//
// Spans every account: the worker runs once and fans out, the same shape as
// the signal poller.
func (r *PortfolioRepository) DueForReorder(ctx context.Context, within time.Time, limit int) ([]*domain.ReorderCandidate, error) {
	return r.dueForReorder(ctx, within, limit, "")
}

// DueForReorderByAccount is the same question for one tenant.
//
// A separate query rather than filtering the cross-account one in Go: a page
// filtered afterwards would show an account nothing whenever its customers
// ranked past the limit — the same bug the inbox and the conversation list
// each had once.
func (r *PortfolioRepository) DueForReorderByAccount(ctx context.Context, accountID string, within time.Time, limit int) ([]*domain.ReorderCandidate, error) {
	return r.dueForReorder(ctx, within, limit, accountID)
}

func (r *PortfolioRepository) dueForReorder(ctx context.Context, within time.Time, limit int, accountID string) ([]*domain.ReorderCandidate, error) {
	// The account filter is a parameter, not string interpolation: a NULL
	// means "every account", which is what the worker wants.
	var scope *string
	if accountID != "" {
		scope = &accountID
	}
	// Per (customer, product): the LATEST purchase of each product is the
	// one that predicts, and only it. A customer who reordered gloves last
	// week is not asked about gloves — but IS still asked about the gauze
	// they bought two months ago, which the old whole-customer rule hid.
	rows, err := r.db.Query(ctx,
		`WITH latest AS (
		   SELECT DISTINCT ON (l.account_id, l.contact_id, l.product_key)
		          l.account_id, l.contact_id, l.sale_id, l.product_key, l.sold_at,
		          l.gross_cents, l.cycle_days, l.line_items,
		          l.sold_at + (l.cycle_days || ' days')::interval AS due_at
		     FROM (`+saleProductLines+`) l
		    WHERE ($3::text IS NULL OR l.account_id = $3)
		    ORDER BY l.account_id, l.contact_id, l.product_key, l.sold_at DESC
		 )
		 SELECT latest.account_id, c.id,
		        COALESCE(NULLIF(c.name, ''), c.external_id),
		        latest.sale_id, latest.product_key, latest.line_items, latest.sold_at,
		        latest.gross_cents, latest.due_at
		   FROM latest
		   JOIN contacts c ON c.id = latest.contact_id AND c.account_id = latest.account_id
		  WHERE latest.cycle_days > 0
		    AND latest.due_at <= $1
		    -- Never asked about before. A worker that runs hourly must not
		    -- turn a helpful reminder into a week of daily ones.
		    -- Per PRODUCT: the gauze prediction is not silenced because the
		    -- gloves of the same sale were already offered (migration 000224).
		    AND NOT EXISTS (
		      SELECT 1 FROM reorder_nudges n
		       WHERE n.account_id = latest.account_id AND n.sale_id = latest.sale_id
		         AND n.product_key = latest.product_key)
		  ORDER BY latest.due_at
		  LIMIT $2`, within, limit, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.ReorderCandidate
	for rows.Next() {
		cand := &domain.ReorderCandidate{}
		var items []byte
		if err := rows.Scan(&cand.AccountID, &cand.ContactID, &cand.ContactName,
			&cand.SaleID, &cand.ProductKey, &items, &cand.LastSoldAt,
			&cand.GrossCents, &cand.DueAt); err != nil {
			return nil, err
		}
		if len(items) > 0 {
			_ = json.Unmarshal(items, &cand.Items)
		}
		out = append(out, cand)
	}
	return out, rows.Err()
}

// RecordNudge marks a predicted reorder as already offered.
//
// DO NOTHING on conflict: two workers racing is an ordinary state, and the
// honest outcome is that the customer is asked once.
func (r *PortfolioRepository) RecordNudge(ctx context.Context, id, accountID, contactID, saleID, productKey string, dueAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO reorder_nudges (id, account_id, contact_id, sale_id, product_key, due_at)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (account_id, sale_id, product_key) DO NOTHING`,
		id, accountID, contactID, saleID, productKey, dueAt)
	return err
}

var _ domain.PortfolioRepository = (*PortfolioRepository)(nil)
