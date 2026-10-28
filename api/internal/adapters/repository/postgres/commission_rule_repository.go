package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/replider/api/internal/core/domain"
)

// Client-facing commission rules (§6.6, G1). Raw SQL, like the other tables
// sqlc does not own.
type CommissionRuleRepository struct{ db DBTX }

func NewCommissionRuleRepository(db DBTX) *CommissionRuleRepository {
	return &CommissionRuleRepository{db: db}
}

const commissionRuleCols = `id, account_id, name, scope, scope_id, base,
	deduct_tax, deduct_cost, rate_percent, repurchase_rate_percent,
	tiers, milestones, splits, active, created_at, updated_at`

func scanCommissionRule(row pgx.Row) (*domain.CommissionRule, error) {
	r := &domain.CommissionRule{}
	var scope, base string
	var scopeID *string
	var tiers, milestones, splits []byte

	err := row.Scan(&r.ID, &r.AccountID, &r.Name, &scope, &scopeID, &base,
		&r.Deductions.Tax, &r.Deductions.Cost, &r.RatePercent,
		&r.RepurchaseRatePercent, &tiers, &milestones, &splits,
		&r.Active, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	r.Scope = domain.CommissionScopeKind(scope)
	r.Base = domain.CommissionBaseKind(base)
	if scopeID != nil {
		r.ScopeID = *scopeID
	}
	// A malformed list is treated as absent rather than failing the read: a
	// rule that loads without its tiers is visible and fixable on the screen;
	// one that will not load at all takes the whole commission page down.
	_ = json.Unmarshal(tiers, &r.Tiers)
	_ = json.Unmarshal(milestones, &r.Milestones)
	_ = json.Unmarshal(splits, &r.Splits)
	return r, nil
}

func (r *CommissionRuleRepository) List(ctx context.Context, accountID string) ([]*domain.CommissionRule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+commissionRuleCols+` FROM commission_rules
		  WHERE account_id = $1
		  ORDER BY active DESC, created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.CommissionRule
	for rows.Next() {
		rule, err := scanCommissionRule(rows)
		if err != nil {
			return nil, err
		}
		if rule != nil {
			out = append(out, rule)
		}
	}
	return out, rows.Err()
}

func (r *CommissionRuleRepository) Get(ctx context.Context, accountID, id string) (*domain.CommissionRule, error) {
	return scanCommissionRule(r.db.QueryRow(ctx,
		`SELECT `+commissionRuleCols+` FROM commission_rules
		  WHERE id = $1 AND account_id = $2`, id, accountID))
}

func (r *CommissionRuleRepository) Upsert(ctx context.Context, rule *domain.CommissionRule) error {
	tiers, err := json.Marshal(orEmpty(rule.Tiers))
	if err != nil {
		return err
	}
	milestones, err := json.Marshal(orEmpty(rule.Milestones))
	if err != nil {
		return err
	}
	splits, err := json.Marshal(orEmpty(rule.Splits))
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO commission_rules (id, account_id, name, scope, scope_id,
		    base, deduct_tax, deduct_cost, rate_percent,
		    repurchase_rate_percent, tiers, milestones, splits, active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT (id) DO UPDATE
		    SET name = EXCLUDED.name,
		        scope = EXCLUDED.scope,
		        scope_id = EXCLUDED.scope_id,
		        base = EXCLUDED.base,
		        deduct_tax = EXCLUDED.deduct_tax,
		        deduct_cost = EXCLUDED.deduct_cost,
		        rate_percent = EXCLUDED.rate_percent,
		        repurchase_rate_percent = EXCLUDED.repurchase_rate_percent,
		        tiers = EXCLUDED.tiers,
		        milestones = EXCLUDED.milestones,
		        splits = EXCLUDED.splits,
		        active = EXCLUDED.active,
		        updated_at = NOW()
		  WHERE commission_rules.account_id = EXCLUDED.account_id`,
		rule.ID, rule.AccountID, rule.Name, string(rule.Scope),
		nilIfEmpty(rule.ScopeID), string(rule.Base),
		rule.Deductions.Tax, rule.Deductions.Cost, rule.RatePercent,
		rule.RepurchaseRatePercent, tiers, milestones, splits, rule.Active)
	return err
}

func (r *CommissionRuleRepository) Delete(ctx context.Context, accountID, id string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM commission_rules WHERE id = $1 AND account_id = $2`,
		id, accountID)
	return err
}

// orEmpty keeps a nil slice out of the JSONB column: `null` would fail the
// NOT NULL default and read back as a broken list.
func orEmpty[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

var _ domain.CommissionRuleRepository = (*CommissionRuleRepository)(nil)
