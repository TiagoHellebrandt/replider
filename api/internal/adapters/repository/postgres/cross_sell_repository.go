package postgres

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

// CrossSellRepository reads co-purchase patterns out of an account's OWN
// sales (§6.5, F5).
//
// Within the account, deliberately. What a distributor sells to whom is their
// commercial position, and pooling it across accounts would mean one
// customer's product mix informing a competitor's pitch.
type CrossSellRepository struct{ db DBTX }

func NewCrossSellRepository(db DBTX) *CrossSellRepository {
	return &CrossSellRepository{db: db}
}

// normalizedItems is the shared CTE: one row per (customer, normalised item).
//
// The normalisation is duplicated between here and domain.NormalizeItemLabel
// on purpose — one lives in SQL because a GROUP BY cannot call Go, the other
// in Go because the suggestion filter compares what a customer already owns.
// They are kept deliberately simple so they cannot drift far, and
// TestCrossSell_SQLAndGoNormalizersAgree pins that they agree.
//
// translate(), not unaccent(): the extension is not installed on this
// database and installing one for a cosmetic fold would make every deploy
// depend on superuser rights. The accent table is the same one
// domain.accentFolder uses.
//
// An item that names a catalogue product (product_id) is labelled by the
// PRODUCT's name, not by how the line was typed: "Luvas cx 100" filed by
// hand and "luva de procedimento" out of the ERP import are one product to
// the catalogue and must be one signal here. An item with only a product_id
// and no description (the app's item picker) used to be dropped entirely.
const normalizedItems = `
	WITH items AS (
	  SELECT s.contact_id,
	         TRIM(REGEXP_REPLACE(
	           REGEXP_REPLACE(
	             REGEXP_REPLACE(
	               TRANSLATE(LOWER(COALESCE(p.name, item ->> 'description')),
	                         'áàâãäéèêëíìîïóòôõöúùûüçñ',
	                         'aaaaaeeeeiiiiooooouuuucn'),
	               '\y(cx|caixa|pct|pacote|un|unid|unidade|c/|com)\y\.?\s*[0-9]*', ' ', 'g'),
	             '[0-9]+|\s+', ' ', 'g'),
	           -- Plural fold, the SQL half of domain.NormalizeItemLabel.
	           '([a-z]{3,})s\y', '\1', 'g')) AS label
	    FROM sales s
	    CROSS JOIN LATERAL jsonb_array_elements(s.items) AS item
	    LEFT JOIN products p
	           ON p.id = NULLIF(item ->> 'product_id', '')
	          AND p.account_id = s.account_id
	   WHERE s.account_id = $1
	     AND COALESCE(p.name, item ->> 'description', '') <> ''
	),
	distinct_items AS (
	  -- DISTINCT on purpose: one customer ordering the same pair twelve
	  -- times is one pattern, not twelve.
	  SELECT DISTINCT contact_id, label FROM items WHERE label <> ''
	)`

// Pairs returns every co-purchase pattern with enough evidence.
//
// Filtered in SQL rather than in Go: an account with two hundred products has
// forty thousand ordered pairs, and almost none of them clear the floor.
func (r *CrossSellRepository) Pairs(ctx context.Context, accountID string) ([]domain.CoPurchasePair, error) {
	rows, err := r.db.Query(ctx, normalizedItems+`,
		totals AS (
		  SELECT label, COUNT(DISTINCT contact_id) AS customers
		    FROM distinct_items GROUP BY label
		)
		SELECT a.label,
		       b.label,
		       COUNT(DISTINCT a.contact_id) AS both_customers,
		       t.customers                  AS product_customers
		  FROM distinct_items a
		  JOIN distinct_items b
		    ON b.contact_id = a.contact_id AND b.label <> a.label
		  JOIN totals t ON t.label = a.label
		 GROUP BY a.label, b.label, t.customers
		 HAVING COUNT(DISTINCT a.contact_id) >= $2
		    AND COUNT(DISTINCT a.contact_id)::float / t.customers >= $3
		 ORDER BY both_customers DESC
		 LIMIT 500`,
		accountID, domain.MinCoPurchaseCustomers, domain.MinCoPurchaseConfidence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CoPurchasePair
	for rows.Next() {
		var p domain.CoPurchasePair
		if err := rows.Scan(&p.Product, &p.Partner, &p.Customers,
			&p.ProductCustomers); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ItemsBoughtBy lists the normalised labels one customer has bought.
func (r *CrossSellRepository) ItemsBoughtBy(ctx context.Context, accountID, contactID string) ([]string, error) {
	rows, err := r.db.Query(ctx, normalizedItems+`
		SELECT label FROM distinct_items WHERE contact_id = $2`,
		accountID, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		out = append(out, label)
	}
	return out, rows.Err()
}

var _ domain.CrossSellRepository = (*CrossSellRepository)(nil)
