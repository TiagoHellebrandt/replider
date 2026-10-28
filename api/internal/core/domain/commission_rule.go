package domain

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Client-facing commission rules (§6.6, G1).
//
// This is NOT the platform's own payroll (commission_ledger, which pays
// Replider's account managers a share of Replider's profit). This is the
// customer's own rule for their own salespeople, applied to their own sales.
// The two share nothing on purpose: one is a fixed internal policy, the other
// is whatever the customer already does in a spreadsheet today.
//
// A STRUCTURED builder, not a formula field. A free-text formula is a
// promise the product cannot keep — it has to be parsed, it can divide by
// zero, it silently means something different when a field is renamed, and
// the first time it pays somebody wrong the customer stops trusting every
// number on the screen. Structure is also what makes simulation honest: the
// UI can say "com venda média de R$10k, o vendedor ganha R$X" because there
// is a finite set of things the rule can do.

// CommissionBaseKind is what the percentage applies to.
type CommissionBaseKind string

const (
	// BaseGross is the invoice. Simple, and dangerous in distribution: the
	// cost of goods is most of the invoice, so 5% of gross can exceed 100%
	// of margin. Offered because plenty of companies genuinely pay this way,
	// and refusing to model reality does not change it.
	BaseGross CommissionBaseKind = "gross"
	// BaseNet is gross minus the deductions chosen below.
	BaseNet CommissionBaseKind = "net"
	// BaseMargin is gross minus cost of goods — what the company actually
	// kept — and is the base this product nudges towards.
	BaseMargin CommissionBaseKind = "margin"
)

// CommissionScopeKind says which sales a rule governs.
type CommissionScopeKind string

const (
	// ScopeGlobal is the fallback: every sale with no more specific rule.
	ScopeGlobal CommissionScopeKind = "global"
	// ScopeSeller overrides for one salesperson — the veteran on a different
	// deal, which is the single most common exception in a real company.
	ScopeSeller CommissionScopeKind = "seller"
	// ScopeTeam is by team.
	ScopeTeam CommissionScopeKind = "team"
	// ScopeProduct pays differently for one product line: equipment and
	// consumables rarely share a rate.
	ScopeProduct CommissionScopeKind = "product"
)

// Specificity orders overlapping rules. The most specific wins, and ties are
// broken by the newest rule — never by chance, because a payout that changes
// depending on row order is a payout nobody can explain.
func (s CommissionScopeKind) Specificity() int {
	switch s {
	case ScopeSeller:
		return 3
	case ScopeProduct:
		return 2
	case ScopeTeam:
		return 1
	default:
		return 0
	}
}

// CommissionDeductions is what comes off gross before a percentage applies.
//
// Booleans rather than amounts: the amounts live on the Sale. A rule that
// carried its own numbers would drift from the invoice it is paid on.
//
// There is no separate freight flag, because a Sale records cost and tax and
// nothing else. Freight is part of the cost of the goods, and offering a
// switch that deducts a number nobody records would be a setting that quietly
// does nothing.
type CommissionDeductions struct {
	Tax  bool `json:"tax"`
	Cost bool `json:"cost"`
}

// CommissionTier is one escalão: from this much sold in the period, this rate.
type CommissionTier struct {
	// FromCents is the period total at which the tier starts. The first tier
	// normally starts at 0.
	FromCents int64 `json:"from_cents"`
	// RatePercent is the percentage of the base. 2.5 means 2.5%.
	RatePercent float64 `json:"rate_percent"`
}

// CommissionMilestone is a fixed bonus for reaching a target in the period.
type CommissionMilestone struct {
	TargetCents int64  `json:"target_cents"`
	BonusCents  int64  `json:"bonus_cents"`
	Label       string `json:"label,omitempty"`
}

// CommissionSplit divides one sale's commission between people.
//
// This is where the introduction from §6.3 becomes money: whoever opened the
// door gets a named share, decided in advance, instead of an argument after
// the deal closes.
type CommissionSplit struct {
	// Role is free text the company recognises ("closer", "quem indicou",
	// "gerente"). Not an enum: every company names these differently, and an
	// enum would force somebody to pick the wrong word for their own process.
	Role string `json:"role"`
	// SharePercent of the commission, not of the sale.
	SharePercent float64 `json:"share_percent"`
}

// CommissionRule is one rule.
type CommissionRule struct {
	ID        string `json:"id"`
	AccountID string `json:"-"`
	Name      string `json:"name"`

	Scope CommissionScopeKind `json:"scope"`
	// ScopeID is the seller/team/product the scope points at. Empty for
	// global.
	ScopeID string `json:"scope_id,omitempty"`

	Base       CommissionBaseKind   `json:"base"`
	Deductions CommissionDeductions `json:"deductions"`

	// RatePercent is the flat rate, used when there are no tiers.
	RatePercent float64 `json:"rate_percent"`
	// Tiers, when present, REPLACE the flat rate.
	Tiers []CommissionTier `json:"tiers,omitempty"`

	// RepurchaseRatePercent pays differently for a customer who already
	// bought. Nil means "same as a new sale" — which is different from zero,
	// and a company that genuinely pays nothing on repurchase has to say so.
	RepurchaseRatePercent *float64 `json:"repurchase_rate_percent,omitempty"`

	Milestones []CommissionMilestone `json:"milestones,omitempty"`
	Splits     []CommissionSplit     `json:"splits,omitempty"`

	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MaxCommissionRatePercent bounds a rate.
//
// A hundred percent of the base, and no more. Above that the rule pays out
// more than the sale produced, which is never what somebody meant to type —
// it is a decimal point in the wrong place, and catching it here is cheaper
// than catching it in a payout.
const MaxCommissionRatePercent = 100.0

// Validate reports the rule a customer cannot have meant.
func (r *CommissionRule) Validate() error {
	if r == nil {
		return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "regra vazia"}
	}
	if strings.TrimSpace(r.Name) == "" {
		return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "dê um nome à regra"}
	}
	switch r.Base {
	case BaseGross, BaseNet, BaseMargin:
	default:
		return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "base de cálculo inválida"}
	}
	switch r.Scope {
	case ScopeGlobal:
		// A global rule with a target would silently apply to nobody.
		if strings.TrimSpace(r.ScopeID) != "" {
			return &AppError{Code: "COMMISSION_RULE_INVALID",
				Message: "uma regra global não aponta para ninguém"}
		}
	case ScopeSeller, ScopeTeam, ScopeProduct:
		if strings.TrimSpace(r.ScopeID) == "" {
			return &AppError{Code: "COMMISSION_RULE_INVALID",
				Message: "escolha a quem esta regra se aplica"}
		}
	default:
		return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "escopo inválido"}
	}

	if err := validRate(r.RatePercent); err != nil {
		return err
	}
	if r.RepurchaseRatePercent != nil {
		if err := validRate(*r.RepurchaseRatePercent); err != nil {
			return err
		}
	}
	for _, tier := range r.Tiers {
		if tier.FromCents < 0 {
			return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "escalão com meta negativa"}
		}
		if err := validRate(tier.RatePercent); err != nil {
			return err
		}
	}
	for _, m := range r.Milestones {
		if m.TargetCents <= 0 || m.BonusCents < 0 {
			return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "bônus por marco inválido"}
		}
	}

	// Splits either divide the whole commission or do not exist. Anything in
	// between quietly loses money that somebody earned, or invents money
	// nobody budgeted.
	if len(r.Splits) > 0 {
		var total float64
		for _, s := range r.Splits {
			if strings.TrimSpace(s.Role) == "" {
				return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "split sem papel"}
			}
			if s.SharePercent < 0 {
				return &AppError{Code: "COMMISSION_RULE_INVALID", Message: "split negativo"}
			}
			total += s.SharePercent
		}
		// A cent of float slack, because 33.33 × 3 is not 100 in binary.
		if total < 99.99 || total > 100.01 {
			return &AppError{Code: "COMMISSION_RULE_SPLIT_NOT_WHOLE",
				Message: "os splits precisam somar 100%"}
		}
	}
	return nil
}

func validRate(rate float64) error {
	if rate < 0 || rate > MaxCommissionRatePercent {
		return &AppError{Code: "COMMISSION_RULE_INVALID",
			Message: "percentual precisa ficar entre 0 e 100"}
	}
	return nil
}

// SortedTiers returns the tiers ascending, so the lookup can walk them once.
func (r *CommissionRule) SortedTiers() []CommissionTier {
	out := make([]CommissionTier, len(r.Tiers))
	copy(out, r.Tiers)
	sort.Slice(out, func(i, j int) bool { return out[i].FromCents < out[j].FromCents })
	return out
}

// RateFor picks the percentage for one sale.
//
// periodTotalCents is what this salesperson has already sold in the period,
// INCLUDING the sale being calculated — a tier reached by the sale itself
// should pay at the tier it reached, which is how everybody explains it out
// loud.
func (r *CommissionRule) RateFor(periodTotalCents int64, repurchase bool) float64 {
	if repurchase && r.RepurchaseRatePercent != nil {
		// A repurchase rate REPLACES the ladder. Mixing the two would mean a
		// company that pays less for repeat business still climbing tiers on
		// it, which is not what "menos para recompra" means to anybody.
		return *r.RepurchaseRatePercent
	}
	if len(r.Tiers) == 0 {
		return r.RatePercent
	}
	rate := 0.0
	for _, tier := range r.SortedTiers() {
		if periodTotalCents >= tier.FromCents {
			rate = tier.RatePercent
			continue
		}
		break
	}
	return rate
}

// PickCommissionRule chooses which rule governs a sale.
//
// Most specific wins; a tie goes to the newest. Deterministic on purpose: a
// payout that depends on which row the database returned first is a payout
// nobody can defend in a conversation with the person it underpaid.
func PickCommissionRule(rules []*CommissionRule, sellerUserID, teamID string, productIDs []string) *CommissionRule {
	owns := map[string]struct{}{}
	for _, id := range productIDs {
		owns[id] = struct{}{}
	}

	var best *CommissionRule
	for _, rule := range rules {
		if rule == nil || !rule.Active {
			continue
		}
		switch rule.Scope {
		case ScopeSeller:
			if rule.ScopeID != sellerUserID || sellerUserID == "" {
				continue
			}
		case ScopeTeam:
			if rule.ScopeID != teamID || teamID == "" {
				continue
			}
		case ScopeProduct:
			if _, has := owns[rule.ScopeID]; !has {
				continue
			}
		case ScopeGlobal:
		default:
			continue
		}

		if best == nil ||
			rule.Scope.Specificity() > best.Scope.Specificity() ||
			(rule.Scope.Specificity() == best.Scope.Specificity() &&
				rule.CreatedAt.After(best.CreatedAt)) {
			best = rule
		}
	}
	return best
}

// CommissionRuleRepository persists the rules.
type CommissionRuleRepository interface {
	List(ctx context.Context, accountID string) ([]*CommissionRule, error)
	Get(ctx context.Context, accountID, id string) (*CommissionRule, error)
	Upsert(ctx context.Context, rule *CommissionRule) error
	Delete(ctx context.Context, accountID, id string) error
}
