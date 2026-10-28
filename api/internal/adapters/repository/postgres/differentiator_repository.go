package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/replider/api/internal/core/domain"
)

// DifferentiatorRepository stores "why ours rather than the one they own".
// Raw SQL and whole-list Replace, mirroring the objection library it sits
// beside — the edit surface is a short ordered list.
type DifferentiatorRepository struct{ db *pgxpool.Pool }

func NewDifferentiatorRepository(db *pgxpool.Pool) *DifferentiatorRepository {
	return &DifferentiatorRepository{db: db}
}

func (r *DifferentiatorRepository) List(accountID string) ([]domain.Differentiator, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, account_id, COALESCE(product_id, ''), label, detail, position, created_at, updated_at
		FROM differentiators WHERE account_id = $1 ORDER BY position, created_at`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.Differentiator{}
	for rows.Next() {
		var d domain.Differentiator
		if err := rows.Scan(&d.ID, &d.AccountID, &d.ProductID, &d.Label,
			&d.Detail, &d.Position, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Replace swaps the whole library in ONE transaction. Deleting outside the
// transaction would leave an account with no differentiators if the insert
// failed — and a draft written in that window argues from nothing.
func (r *DifferentiatorRepository) Replace(accountID string, items []domain.Differentiator) error {
	ctx := context.Background()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM differentiators WHERE account_id = $1`, accountID); err != nil {
		return err
	}
	for i, it := range items {
		if it.ID == "" {
			it.ID = ulid.Make().String()
		}
		var productID *string
		if it.ProductID != "" {
			p := it.ProductID
			productID = &p
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO differentiators (id, account_id, product_id, label, detail, position)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			it.ID, accountID, productID, it.Label, it.Detail, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// PublicContractRepository stores what PNCP answered about a buyer. Shared and
// account-independent, like the CNES tables: a signed public contract is
// public record, not one customer's data.
type PublicContractRepository struct{ db *pgxpool.Pool }

func NewPublicContractRepository(db *pgxpool.Pool) *PublicContractRepository {
	return &PublicContractRepository{db: db}
}

func (r *PublicContractRepository) ListByBuyer(ctx context.Context, buyerCNPJ string, limit int) ([]domain.PublicContract, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT pncp_id, buyer_cnpj, buyer_name, supplier_cnpj, supplier_name,
		       object, value_cents, signed_at, expires_at, uf, city,
		       ata_pncp_id, from_adhesion
		FROM public_contracts
		WHERE buyer_cnpj = $1
		ORDER BY signed_at DESC NULLS LAST
		LIMIT $2`, buyerCNPJ, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.PublicContract{}
	for rows.Next() {
		var c domain.PublicContract
		var signed, expires *time.Time
		if err := rows.Scan(&c.PNCPID, &c.BuyerCNPJ, &c.BuyerName, &c.SupplierCNPJ,
			&c.SupplierName, &c.Object, &c.ValueCents, &signed, &expires,
			&c.UF, &c.City, &c.AtaPNCPID, &c.FromAdhesion); err != nil {
			return nil, err
		}
		c.SignedAt, c.ExpiresAt = signed, expires
		out = append(out, c)
	}
	return out, rows.Err()
}

// Save upserts by PNCP id: the same contract fetched twice is one row, and a
// re-fetch refreshes what the record now says.
func (r *PublicContractRepository) Save(ctx context.Context, items []domain.PublicContract) error {
	for _, c := range items {
		if c.PNCPID == "" || c.BuyerCNPJ == "" {
			continue
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO public_contracts (pncp_id, buyer_cnpj, buyer_name, supplier_cnpj,
				supplier_name, object, value_cents, signed_at, expires_at, uf, city,
				ata_pncp_id, from_adhesion, fetched_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, NOW())
			ON CONFLICT (pncp_id) DO UPDATE SET
				supplier_name = EXCLUDED.supplier_name,
				supplier_cnpj = EXCLUDED.supplier_cnpj,
				object        = EXCLUDED.object,
				value_cents   = EXCLUDED.value_cents,
				expires_at    = EXCLUDED.expires_at,
				ata_pncp_id   = EXCLUDED.ata_pncp_id,
				from_adhesion = EXCLUDED.from_adhesion,
				fetched_at    = NOW()`,
			c.PNCPID, c.BuyerCNPJ, c.BuyerName, c.SupplierCNPJ, c.SupplierName,
			c.Object, c.ValueCents, c.SignedAt, c.ExpiresAt, c.UF, c.City,
			c.AtaPNCPID, c.FromAdhesion); err != nil {
			return err
		}
	}
	return nil
}

func (r *PublicContractRepository) LastScan(ctx context.Context, buyerCNPJ string) (time.Time, error) {
	var at time.Time
	err := r.db.QueryRow(ctx,
		`SELECT scanned_at FROM public_contract_scans WHERE buyer_cnpj = $1`, buyerCNPJ).Scan(&at)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Never asked. Zero, not an error: the caller's question is
			// "should I fetch?", and "never" answers it.
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return at, nil
}

func (r *PublicContractRepository) MarkScanned(ctx context.Context, buyerCNPJ string, found int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO public_contract_scans (buyer_cnpj, scanned_at, found)
		VALUES ($1, NOW(), $2)
		ON CONFLICT (buyer_cnpj) DO UPDATE SET scanned_at = NOW(), found = EXCLUDED.found`,
		buyerCNPJ, found)
	return err
}

// ListExpiring is the calendar.
//
// The single read in this product a salesperson could not assemble by hand: a
// public contract has a published end date, so the purchase that replaces it
// is a known event months before any tender exists. Arriving then is the
// difference between influencing a specification and answering one.
//
// Keywords are required and matched against the object with the SAME
// whole-word, accent-folded rule the tender matcher uses. Without them the
// answer is every contract in the state — school meals, street lighting, fuel
// — and a calendar nobody can act on is a calendar nobody opens.
func (r *PublicContractRepository) ListExpiring(
	ctx context.Context, f domain.ContractCalendarFilter,
) ([]domain.PublicContract, error) {
	states := domain.NormalizeUFs(f.States)
	if len(states) == 0 || len(f.Keywords) == 0 {
		// Not an error. An account with no territory or no words has nothing
		// to watch, and answering "everything" would be worse than nothing.
		return []domain.PublicContract{}, nil
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 60
	}

	rows, err := r.db.Query(ctx, `
		SELECT pncp_id, buyer_cnpj, buyer_name, supplier_cnpj, supplier_name,
		       object, value_cents, signed_at, expires_at, uf, city,
		       ata_pncp_id, from_adhesion
		  FROM public_contracts
		 WHERE uf = ANY($1)
		   AND expires_at IS NOT NULL
		   AND expires_at >= $2
		   AND expires_at <= $3
		 ORDER BY expires_at ASC, value_cents DESC
		 LIMIT $4`,
		states, f.From, f.To, limit*4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// The keyword match runs in Go, not in SQL, for one reason: it has to be
	// the SAME function the radar uses. A LIKE in SQL and ContainsWord in Go
	// would disagree the first time a plural or an accent showed up, and two
	// screens disagreeing about what matches is how a feed stops being
	// trusted. Over-fetching by 4x and filtering here is cheap at these sizes.
	out := []domain.PublicContract{}
	for rows.Next() {
		var c domain.PublicContract
		var signed, expires *time.Time
		if err := rows.Scan(&c.PNCPID, &c.BuyerCNPJ, &c.BuyerName, &c.SupplierCNPJ,
			&c.SupplierName, &c.Object, &c.ValueCents, &signed, &expires,
			&c.UF, &c.City, &c.AtaPNCPID, &c.FromAdhesion); err != nil {
			return nil, err
		}
		c.SignedAt, c.ExpiresAt = signed, expires
		if !matchesAnyKeyword(c.Object, f.Keywords) {
			continue
		}
		// The words alone are not enough, and the radar already learned this
		// the expensive way. Half the keywords a catalogue yields are ordinary
		// Portuguese — "aquecido", "balança", "ventilador", "reservatório" —
		// and a state buys those for canteens and campuses all year. Measured
		// over Mato Grosso's first 362 collected contracts: four matched, and
		// all four were restaurant or campus equipment.
		//
		// R$ 757k of bouncy castles at the top of a salesperson's morning is
		// what taught them not to trust the radar; a heated food cart labelled
		// "buying window" would teach them the same about this.
		// Health context alone let through medical SERVICES and a hospital's
		// IT purchase. See IsGoodsPurchaseInOurTrade for the measurement.
		if !domain.IsGoodsPurchaseInOurTrade(c.Object, c.BuyerName) {
			continue
		}
		out = append(out, c)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// matchesAnyKeyword asks the radar's question of a contract's object.
func matchesAnyKeyword(object string, keywords []string) bool {
	hay := strings.ToLower(domain.FoldAccents(object))
	for _, kw := range keywords {
		k := strings.ToLower(domain.FoldAccents(strings.TrimSpace(kw)))
		if k != "" && domain.ContainsWord(hay, k) {
			return true
		}
	}
	return false
}

// BuyersNeedingScan lists the establishments whose contracts are stalest.
//
// The sweep is BY BUYER and not by date range, because PNCP's /contratos
// endpoint silently ignores its own `uf` parameter — verified against the live
// API on 2026-09-11, where uf=MT returned contracts from SC, SP, PA and CE. A
// territory-wide date walk would therefore mean reading the whole country:
// 234.769 contracts in forty days, 4.696 pages. That is the same mistake that
// earned a rate-limit ban in September. `cnpjOrgao` DOES filter, and CNES
// already gives us every hospital's CNPJ, so the buyers are the axis.
func (r *PublicContractRepository) BuyersNeedingScan(
	ctx context.Context, states, cityCodes []string, staleBefore time.Time, limit int,
) ([]domain.ContractScanTarget, error) {
	states = domain.NormalizeUFs(states)
	if len(states) == 0 {
		return []domain.ContractScanTarget{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 40
	}
	// City codes, not city names — the same IBGE codes every other territory
	// read uses (resolveTerritoryScope). Matching on names in SQL would need a
	// fold the database does not have, and would disagree with the Go fold the
	// rest of the product narrows by.
	codes := make([]string, 0, len(cityCodes))
	for _, c := range cityCodes {
		if c = strings.TrimSpace(c); c != "" {
			codes = append(codes, c)
		}
	}

	// The state comes from the MUNICIPALITY, not from the establishment.
	//
	// `cnes_establishments` has no `uf` column — it is keyed by IBGE city code
	// and the state lives on `ibge_municipalities`. Querying a column that does
	// not exist failed the whole sweep with one WARN line and a silently empty
	// calendar, which is how this shipped and reached production.
	//
	// So the municipality join is INNER, not LEFT: an establishment whose city
	// code resolves to nothing has no state, and a row with no state cannot be
	// scoped to a territory.
	// TWO sources of buyers, and the second one is the important half.
	//
	// CNES lists health ESTABLISHMENTS, so the CNPJ it holds is a hospital's.
	// The bodies that actually run the big purchases — a state health
	// department, a regional health consortium, EBSERH — are not health
	// establishments and are simply not in that register. Measured on
	// 2026-09-14: of the 48 buyers that had already published a tender
	// matching this account's words, 47 were absent from CNES. The sweep would
	// never have asked any of them, including the state of Mato Grosso itself.
	//
	// Every one of those is a buyer we know is in the market, because they
	// published a notice for what this account sells. They belong in the
	// sweep more than the private clinic next door does.
	rows, err := r.db.Query(ctx, `
		WITH buyers AS (
		    SELECT regexp_replace(COALESCE(e.cnpj, ''), '\D', '', 'g') AS cnpj,
		           COALESCE(NULLIF(e.legal_name, ''), e.trade_name, '')  AS name,
		           m.uf                                                  AS uf,
		           COALESCE(m.name, '')                                  AS city,
		           e.city_code                                           AS city_code
		      FROM cnes_establishments e
		      JOIN ibge_municipalities m ON m.code = e.city_code
		    UNION ALL
		    -- Buyers we met through their own tenders. No city code: a signal
		    -- carries a city NAME, so these are scoped by state only — which
		    -- is right for a body that buys for a whole state anyway.
		    SELECT DISTINCT regexp_replace(COALESCE(s.cnpj, ''), '\D', '', 'g'),
		           s.organization, s.state, COALESCE(s.city, ''), NULL
		      FROM signals s
		     WHERE s.cnpj IS NOT NULL
		)
		SELECT DISTINCT ON (digits.cnpj)
		       digits.cnpj,
		       b.name,
		       b.uf,
		       b.city,
		       -- NULL, not a sentinel: pgx refuses to scan -infinity into a
		       -- time.Time, and "never asked" is exactly what NULL means. The
		       -- sentinel stays in the ORDER BY, where nothing reads it.
		       sc.scanned_at
		  FROM buyers b
		  JOIN LATERAL (SELECT b.cnpj AS cnpj) digits ON TRUE
		  LEFT JOIN public_contract_scans sc ON sc.buyer_cnpj = digits.cnpj
		 WHERE b.uf = ANY($1)
		   AND length(digits.cnpj) = 14
		   -- The city narrow applies to establishments only: a tender buyer
		   -- has no IBGE code, and a body that publishes for the whole state
		   -- is not something to filter down to one town.
		   AND (cardinality($2::text[]) = 0 OR b.city_code IS NULL OR b.city_code = ANY($2))
		   AND (sc.scanned_at IS NULL OR sc.scanned_at < $3)
		 ORDER BY digits.cnpj, COALESCE(sc.scanned_at, TIMESTAMPTZ '-infinity') ASC
		 LIMIT $4`,
		states, codes, staleBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []domain.ContractScanTarget{}
	for rows.Next() {
		var t domain.ContractScanTarget
		var last *time.Time
		if err := rows.Scan(&t.CNPJ, &t.Name, &t.UF, &t.City, &last); err != nil {
			return nil, err
		}
		if last != nil {
			t.LastScannedAt = *last
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
