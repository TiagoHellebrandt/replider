package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Contracts (§6.11, G4). Raw SQL, like the sales tables it sits beside.
type ContractRepository struct{ db DBTX }

func NewContractRepository(db DBTX) *ContractRepository {
	return &ContractRepository{db: db}
}

const contractCols = `id, account_id, contact_id, title, parent_contract_id, version,
	object_path, file_name, extraction_status, extraction_error, ai_proposal,
	confirmed_at, confirmed_by_user_id, parties, subject, value_cents, currency,
	payment_terms, starts_on, ends_on, auto_renewal, notice_days,
	adjustment_index, adjustment_month, sla_text, penalties, exclusivity,
	exclusivity_text, guarantees, termination, notes,
	price_registry, registry_number, registry_balance_cents, registry_balance_on,
	last_adjusted_on, created_at, updated_at`

func scanContract(row pgx.Row) (*domain.Contract, error) {
	c := &domain.Contract{}
	var contactID, parentID, confirmedBy *string
	var status string
	var proposal, parties, penalties []byte

	err := row.Scan(&c.ID, &c.AccountID, &contactID, &c.Title, &parentID, &c.Version,
		&c.ObjectPath, &c.FileName, &status, &c.ExtractionError, &proposal,
		&c.ConfirmedAt, &confirmedBy, &parties, &c.Subject, &c.ValueCents, &c.Currency,
		&c.PaymentTerms, &c.StartsOn, &c.EndsOn, &c.AutoRenewal, &c.NoticeDays,
		&c.AdjustmentIndex, &c.AdjustmentMonth, &c.SLAText, &penalties, &c.Exclusivity,
		&c.ExclusivityText, &c.Guarantees, &c.Termination, &c.Notes,
		&c.PriceRegistry, &c.RegistryNumber, &c.RegistryBalanceCents,
		&c.RegistryBalanceOn, &c.LastAdjustedOn,
		&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	c.ExtractionStatus = domain.ContractExtractionStatus(status)
	if contactID != nil {
		c.ContactID = *contactID
	}
	if parentID != nil {
		c.ParentContractID = *parentID
	}
	if confirmedBy != nil {
		c.ConfirmedByUserID = *confirmedBy
	}
	// A malformed list is treated as absent rather than failing the read: a
	// contract that loads without its penalties is visible and fixable; one that
	// will not load takes the whole repository page down.
	_ = json.Unmarshal(parties, &c.Parties)
	_ = json.Unmarshal(penalties, &c.Penalties)
	_ = json.Unmarshal(proposal, &c.AIProposal)
	return c, nil
}

func (r *ContractRepository) Create(ctx context.Context, c *domain.Contract) error {
	parties, penalties, proposal, err := marshalContractJSON(c)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO contracts (id, account_id, contact_id, title,
		    parent_contract_id, version, object_path, file_name,
		    extraction_status, extraction_error, ai_proposal,
		    confirmed_at, confirmed_by_user_id, parties, subject, value_cents,
		    currency, payment_terms, starts_on, ends_on, auto_renewal,
		    notice_days, adjustment_index, adjustment_month, sla_text,
		    penalties, exclusivity, exclusivity_text, guarantees, termination, notes,
		    price_registry, registry_number, registry_balance_cents,
		    registry_balance_on, last_adjusted_on)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,
		         $19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,
		         $32,$33,$34,$35,$36)`,
		c.ID, c.AccountID, nilIfEmpty(c.ContactID), c.Title,
		nilIfEmpty(c.ParentContractID), c.Version, c.ObjectPath, c.FileName,
		string(c.ExtractionStatus), c.ExtractionError, proposal,
		c.ConfirmedAt, nilIfEmpty(c.ConfirmedByUserID), parties, c.Subject, c.ValueCents,
		c.Currency, c.PaymentTerms, c.StartsOn, c.EndsOn, c.AutoRenewal,
		c.NoticeDays, c.AdjustmentIndex, c.AdjustmentMonth, c.SLAText,
		penalties, c.Exclusivity, c.ExclusivityText, c.Guarantees, c.Termination, c.Notes,
		c.PriceRegistry, c.RegistryNumber, c.RegistryBalanceCents,
		c.RegistryBalanceOn, c.LastAdjustedOn)
	return err
}

// Update writes the human-owned fields ONLY.
//
// extraction_status and ai_proposal are deliberately absent: they belong to the
// worker, and letting a save carry them would mean a person editing a date could
// silently relabel the model's reading as their own.
func (r *ContractRepository) Update(ctx context.Context, c *domain.Contract) error {
	parties, penalties, _, err := marshalContractJSON(c)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx,
		`UPDATE contracts
		    SET contact_id = $3, title = $4, object_path = $5, file_name = $6,
		        confirmed_at = $7, confirmed_by_user_id = $8, parties = $9,
		        subject = $10, value_cents = $11, currency = $12,
		        payment_terms = $13, starts_on = $14, ends_on = $15,
		        auto_renewal = $16, notice_days = $17, adjustment_index = $18,
		        adjustment_month = $19, sla_text = $20, penalties = $21,
		        exclusivity = $22, exclusivity_text = $23, guarantees = $24,
		        termination = $25, notes = $26,
		        price_registry = $27, registry_number = $28,
		        registry_balance_cents = $29, registry_balance_on = $30,
		        last_adjusted_on = $31,
		        updated_at = NOW()
		  WHERE id = $1 AND account_id = $2`,
		c.ID, c.AccountID, nilIfEmpty(c.ContactID), c.Title, c.ObjectPath, c.FileName,
		c.ConfirmedAt, nilIfEmpty(c.ConfirmedByUserID), parties,
		c.Subject, c.ValueCents, c.Currency, c.PaymentTerms, c.StartsOn, c.EndsOn,
		c.AutoRenewal, c.NoticeDays, c.AdjustmentIndex, c.AdjustmentMonth,
		c.SLAText, penalties, c.Exclusivity, c.ExclusivityText, c.Guarantees,
		c.Termination, c.Notes,
		c.PriceRegistry, c.RegistryNumber, c.RegistryBalanceCents,
		c.RegistryBalanceOn, c.LastAdjustedOn)
	return err
}

func (r *ContractRepository) Get(ctx context.Context, accountID, id string) (*domain.Contract, error) {
	return scanContract(r.db.QueryRow(ctx,
		`SELECT `+contractCols+` FROM contracts
		  WHERE id = $1 AND account_id = $2`, id, accountID))
}

func (r *ContractRepository) Delete(ctx context.Context, accountID, id string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM contracts WHERE id = $1 AND account_id = $2`, id, accountID)
	return err
}

// List returns the top-level contracts only. Amendments belong to their parent's
// detail — a repository listing that mixes the original and three aditivos as
// four rows is a list nobody can read.
func (r *ContractRepository) List(ctx context.Context, accountID string, limit, offset int) ([]*domain.Contract, error) {
	return r.queryContracts(ctx,
		`SELECT `+contractCols+` FROM contracts
		  WHERE account_id = $1 AND parent_contract_id IS NULL
		  ORDER BY created_at DESC
		  LIMIT $2 OFFSET $3`, accountID, limit, offset)
}

// ListAmendments is one chain, oldest first: an aditivo only makes sense read in
// the order it was signed.
func (r *ContractRepository) ListAmendments(ctx context.Context, accountID, parentID string) ([]*domain.Contract, error) {
	return r.queryContracts(ctx,
		`SELECT `+contractCols+` FROM contracts
		  WHERE account_id = $1 AND parent_contract_id = $2
		  ORDER BY version ASC`, accountID, parentID)
}

// NextVersion is the next place in a chain.
//
// MAX+1 read inside the same statement rather than counting rows: a deleted
// amendment must not hand its number to the next one, because the paper it
// refers to still says "aditivo 2".
func (r *ContractRepository) NextVersion(ctx context.Context, accountID, parentID string) (int, error) {
	var next int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 1) + 1 FROM contracts
		  WHERE account_id = $1 AND parent_contract_id = $2`,
		accountID, parentID).Scan(&next)
	return next, err
}

// DueForAttention is the alert worker's cross-account sweep.
//
// Confirmed only: a red flag is a claim about somebody's agreement, and making
// it from a reading nobody checked is how the product tells a customer their
// contract renews itself when it does not.
func (r *ContractRepository) DueForAttention(
	ctx context.Context, through time.Time, limit int,
) ([]*domain.Contract, error) {
	if limit <= 0 {
		limit = 200
	}
	return r.queryContracts(ctx,
		`SELECT `+contractCols+` FROM contracts
		  WHERE confirmed_at IS NOT NULL
		    AND (
		      -- A term ending inside the horizon (or already ended).
		      (ends_on IS NOT NULL AND ends_on <= $1)
		      -- A price that rises on a stated month: the date arithmetic is the
		      -- domain's, so every one of them is a candidate.
		      OR adjustment_month IS NOT NULL
		      -- Pre-approved revenue still on the table.
		      OR (price_registry AND registry_balance_cents > 0)
		    )
		  ORDER BY ends_on NULLS LAST, created_at
		  LIMIT $2`, through, limit)
}

// SetExtraction stores the model's reading.
//
// The fill is applied ONLY to fields nobody has filled in yet, and only while
// the contract is unconfirmed: somebody who corrected a date while the worker
// was running must not have their correction overwritten by the reading that got
// it wrong. COALESCE/NULLIF do that in the statement, so there is no read-modify
// -write window for a concurrent save to fall into.
func (r *ContractRepository) SetExtraction(
	ctx context.Context, accountID, id string, status domain.ContractExtractionStatus,
	proposal map[string]any, extractionError string, fill *domain.Contract,
) error {
	if fill == nil {
		_, err := r.db.Exec(ctx,
			`UPDATE contracts
			    SET extraction_status = $3, extraction_error = $4, updated_at = NOW()
			  WHERE id = $1 AND account_id = $2`,
			id, accountID, string(status), extractionError)
		return err
	}

	proposalJSON, err := json.Marshal(orEmptyMap(proposal))
	if err != nil {
		return err
	}
	parties, err := json.Marshal(orEmpty(fill.Parties))
	if err != nil {
		return err
	}
	penalties, err := json.Marshal(orEmpty(fill.Penalties))
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		`UPDATE contracts
		    SET extraction_status = $3,
		        extraction_error = '',
		        ai_proposal = $4,
		        title = CASE WHEN title = '' THEN COALESCE(NULLIF($5, ''), title) ELSE title END,
		        subject = CASE WHEN subject = '' THEN $6 ELSE subject END,
		        parties = CASE WHEN parties = '[]'::jsonb THEN $7::jsonb ELSE parties END,
		        value_cents = CASE WHEN value_cents = 0 THEN $8 ELSE value_cents END,
		        payment_terms = CASE WHEN payment_terms = '' THEN $9 ELSE payment_terms END,
		        starts_on = COALESCE(starts_on, $10),
		        ends_on = COALESCE(ends_on, $11),
		        auto_renewal = CASE WHEN auto_renewal THEN auto_renewal ELSE $12 END,
		        notice_days = CASE WHEN notice_days = 0 THEN $13 ELSE notice_days END,
		        adjustment_index = CASE WHEN adjustment_index = '' THEN $14 ELSE adjustment_index END,
		        adjustment_month = COALESCE(adjustment_month, $15),
		        sla_text = CASE WHEN sla_text = '' THEN $16 ELSE sla_text END,
		        penalties = CASE WHEN penalties = '[]'::jsonb THEN $17::jsonb ELSE penalties END,
		        exclusivity = CASE WHEN exclusivity THEN exclusivity ELSE $18 END,
		        exclusivity_text = CASE WHEN exclusivity_text = '' THEN $19 ELSE exclusivity_text END,
		        guarantees = CASE WHEN guarantees = '' THEN $20 ELSE guarantees END,
		        termination = CASE WHEN termination = '' THEN $21 ELSE termination END,
		        updated_at = NOW()
		  WHERE id = $1 AND account_id = $2 AND confirmed_at IS NULL`,
		id, accountID, string(status), proposalJSON,
		fill.Title, fill.Subject, parties, fill.ValueCents, fill.PaymentTerms,
		fill.StartsOn, fill.EndsOn, fill.AutoRenewal, fill.NoticeDays,
		fill.AdjustmentIndex, fill.AdjustmentMonth, fill.SLAText, penalties,
		fill.Exclusivity, fill.ExclusivityText, fill.Guarantees, fill.Termination)
	if err != nil {
		return err
	}

	// A contract confirmed while the worker ran keeps its human fields, but the
	// proposal is still recorded: the point of keeping both readings is being
	// able to compare them afterwards.
	_, err = r.db.Exec(ctx,
		`UPDATE contracts
		    SET extraction_status = $3, extraction_error = '', ai_proposal = $4,
		        updated_at = NOW()
		  WHERE id = $1 AND account_id = $2 AND confirmed_at IS NOT NULL`,
		id, accountID, string(status), proposalJSON)
	return err
}

func (r *ContractRepository) queryContracts(ctx context.Context, sql string, args ...any) ([]*domain.Contract, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Contract
	for rows.Next() {
		c, err := scanContract(rows)
		if err != nil {
			return nil, err
		}
		if c != nil {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

func marshalContractJSON(c *domain.Contract) (parties, penalties, proposal []byte, err error) {
	if parties, err = json.Marshal(orEmpty(c.Parties)); err != nil {
		return nil, nil, nil, err
	}
	if penalties, err = json.Marshal(orEmpty(c.Penalties)); err != nil {
		return nil, nil, nil, err
	}
	if proposal, err = json.Marshal(orEmptyMap(c.AIProposal)); err != nil {
		return nil, nil, nil, err
	}
	return parties, penalties, proposal, nil
}

// orEmptyMap keeps a nil map out of the JSONB column: `null` would fail the NOT
// NULL default and read back as a broken object.
func orEmptyMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

var _ domain.ContractRepository = (*ContractRepository)(nil)
