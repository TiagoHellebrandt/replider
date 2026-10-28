package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/replider/api/internal/core/domain"
)

// Account-scoped AI for the helper services (sentiment, intent, summary,
// smart replies, transcription, vision).
//
// These used to call AIFactory.GetProvider — the PLATFORM key — on every
// inbound message, so AI_REQUIRE_BYOK (the customer pays the provider
// directly) never applied to them. Each helper now resolves the account and
// its plan and asks the factory for the ACCOUNT's provider; the platform
// account is still exempt inside the factory.

// UsageMeter is the narrow port the helpers use to land a call in the cost
// ledger. Satisfied by *usecases.AIMeter (this package cannot import
// usecases). Nil-safe at every call site.
type UsageMeter interface {
	Record(ctx context.Context, accountID, operation, provider, model string, usage domain.AIUsage)
}

// ErrHelperSkipped marks a best-effort helper (sentiment/intent) that chose
// not to run because the account has no key of its own. Callers log it at
// DEBUG and move on — the inbound path never depends on these.
var ErrHelperSkipped = errors.New("ai helper skipped: account has no provider key")

// accountAIScope resolves (account, plan) for a helper call. Both repos are
// optional so a unit test or a partial wiring still builds.
type accountAIScope struct {
	accounts domain.AccountRepository
	plans    domain.PlanRepository
}

func (s accountAIScope) resolve(ctx context.Context, accountID string) (*domain.Account, *domain.Plan) {
	plan := domain.TrialPlan()
	if s.accounts == nil || accountID == "" {
		return nil, plan
	}
	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil || account == nil {
		slog.DebugContext(ctx, "ai scope: account lookup failed", "account_id", accountID, "error", err)
		return nil, plan
	}
	if account.PlanID != nil && *account.PlanID != "" && s.plans != nil {
		if p, err := s.plans.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}
	return account, plan
}

// providerFor asks the factory for the account's provider. `providerName`
// is the helper's preferred provider ("google" for the cheap classifiers);
// the plan's provider wins when the account is BYOK on another vendor,
// because the customer's key is for THAT vendor.
func (s accountAIScope) providerFor(ctx context.Context, factory domain.AIFactory, providerName, accountID string) (domain.AIProvider, *domain.Plan, error) {
	account, plan := s.resolve(ctx, accountID)
	name := providerName
	if account != nil && plan != nil && plan.AIProvider != "" {
		name = plan.AIProvider
	}
	p, err := factory.GetProviderForAccount(ctx, name, account, plan)
	return p, plan, err
}

func recordUsage(ctx context.Context, m UsageMeter, accountID, operation string, plan *domain.Plan, usage domain.AIUsage) {
	if m == nil || plan == nil {
		return
	}
	m.Record(ctx, accountID, operation, plan.AIProvider, plan.AIModel, usage)
}

// AI account context: the transcriber/vision adapters are called from
// handlers that hold the channel but implement an interface with no account
// parameter. The account travels on the context instead.
type aiAccountKey struct{}

// WithAIAccount tags ctx with the account an AI media call is made for.
func WithAIAccount(ctx context.Context, accountID string) context.Context {
	if accountID == "" {
		return ctx
	}
	return context.WithValue(ctx, aiAccountKey{}, accountID)
}

// AIAccountFrom reads the account WithAIAccount stored ("" when absent).
func AIAccountFrom(ctx context.Context) string {
	if v, ok := ctx.Value(aiAccountKey{}).(string); ok {
		return v
	}
	return ""
}
