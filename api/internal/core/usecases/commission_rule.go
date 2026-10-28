package usecases

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// Client-facing commission rules (§6.6, G1).
//
// The customer writes the rule they already use; the product does the
// arithmetic every month instead of a spreadsheet somebody rebuilds by hand.
type CommissionRuleUseCase struct {
	repo domain.CommissionRuleRepository
}

func NewCommissionRuleUseCase(repo domain.CommissionRuleRepository) *CommissionRuleUseCase {
	return &CommissionRuleUseCase{repo: repo}
}

func (uc *CommissionRuleUseCase) List(ctx context.Context, accountID string) ([]*domain.CommissionRule, error) {
	rules, err := uc.repo.List(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []*domain.CommissionRule{}
	}
	return rules, nil
}

func (uc *CommissionRuleUseCase) Get(ctx context.Context, accountID, id string) (*domain.CommissionRule, error) {
	rule, err := uc.repo.Get(ctx, accountID, id)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, &domain.AppError{
			Code: "COMMISSION_RULE_NOT_FOUND", Message: "regra não encontrada"}
	}
	return rule, nil
}

// Save creates or updates a rule.
//
// Validation runs BEFORE the write and refuses anything that cannot have been
// meant — a 500% rate, splits that do not total 100%, a scoped rule pointing
// at nobody. A rule that is wrong in the database pays somebody wrong at the
// end of the month, and by then it is a conversation, not a bug report.
func (uc *CommissionRuleUseCase) Save(ctx context.Context, accountID string, rule *domain.CommissionRule) (*domain.CommissionRule, error) {
	if rule == nil {
		return nil, &domain.AppError{
			Code: "COMMISSION_RULE_INVALID", Message: "regra vazia"}
	}
	rule.AccountID = accountID
	rule.Name = strings.TrimSpace(rule.Name)
	rule.ScopeID = strings.TrimSpace(rule.ScopeID)
	if rule.Scope == "" {
		rule.Scope = domain.ScopeGlobal
	}
	if rule.Base == "" {
		rule.Base = domain.BaseMargin
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}

	if strings.TrimSpace(rule.ID) == "" {
		rule.ID = ulid.Make().String()
	} else {
		// An update must target a rule this account owns. Without this a
		// client could rewrite another company's payout rule by guessing an
		// id — the repository's WHERE would refuse the write, but silently,
		// and the caller would be told it saved.
		existing, err := uc.repo.Get(ctx, accountID, rule.ID)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, &domain.AppError{
				Code: "COMMISSION_RULE_NOT_FOUND", Message: "regra não encontrada"}
		}
		rule.CreatedAt = existing.CreatedAt
	}

	if err := uc.repo.Upsert(ctx, rule); err != nil {
		return nil, translateCommissionRuleConflict(err)
	}
	return rule, nil
}

func (uc *CommissionRuleUseCase) Delete(ctx context.Context, accountID, id string) error {
	return uc.repo.Delete(ctx, accountID, id)
}

// Simulate answers "com venda média de R$10k, o vendedor ganha quanto?"
// before the rule is saved.
//
// Takes the rule from the REQUEST rather than the database on purpose: the
// whole point is to try a rule nobody has committed to yet.
func (uc *CommissionRuleUseCase) Simulate(
	ctx context.Context, accountID string, rule *domain.CommissionRule,
	grossCents int64, marginPercent float64, salesPerMonth int,
) (*domain.CommissionSimulation, error) {
	if rule == nil {
		return nil, &domain.AppError{
			Code: "COMMISSION_RULE_INVALID", Message: "regra vazia"}
	}
	rule.AccountID = accountID
	if rule.Scope == "" {
		rule.Scope = domain.ScopeGlobal
	}
	if rule.Base == "" {
		rule.Base = domain.BaseMargin
	}
	if rule.Name == "" {
		// A simulation does not need a name, but Validate does — and every
		// other rule it enforces is exactly what a simulation must respect.
		rule.Name = "simulação"
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if grossCents <= 0 {
		return nil, &domain.AppError{
			Code: "COMMISSION_SIMULATION_INVALID", Message: "informe um valor de venda"}
	}

	sim := domain.SimulateCommission(rule, grossCents, marginPercent, salesPerMonth)
	if sim == nil {
		return nil, &domain.AppError{
			Code: "COMMISSION_SIMULATION_INVALID", Message: "could not simular"}
	}
	return sim, nil
}

// translateCommissionRuleConflict turns the one unique index into an answer
// somebody can act on. "duplicate key value violates unique constraint" tells
// the user nothing about the second global rule they just tried to create.
func translateCommissionRuleConflict(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "uniq_commission_rule_global") {
		return &domain.AppError{
			Code:    "COMMISSION_RULE_GLOBAL_EXISTS",
			Message: "já existe uma regra geral ativa — edite a existente ou desative-a antes de criar outra",
		}
	}
	return err
}
