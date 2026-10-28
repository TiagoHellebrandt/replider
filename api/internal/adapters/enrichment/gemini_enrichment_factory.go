package enrichment

import (
	"context"
	"strings"

	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
)

// GeminiEnrichmentFactory builds account-scoped GeminiGroundingAdapters,
// resolving the BYOK Google/Gemini key the same way AIFactory does: contract
// (BYOK) plans use the account's stored key; self-serve plans use the platform
// env key. It enforces the Gemini-only gate (US-023).
type GeminiEnrichmentFactory struct {
	cfg *config.Config
}

var _ domain.LeadEnrichmentFactory = (*GeminiEnrichmentFactory)(nil)

// NewGeminiEnrichmentFactory wires the factory with platform config (env key +
// default provider).
func NewGeminiEnrichmentFactory(cfg *config.Config) *GeminiEnrichmentFactory {
	return &GeminiEnrichmentFactory{cfg: cfg}
}

// ProviderForAccount returns a grounded enrichment provider for the account, or
// ErrEnrichmentRequiresGemini when the resolved AI provider is not Gemini, or
// ErrBYOKNotConfigured when the (contract-plan) account has no Gemini key.
func (f *GeminiEnrichmentFactory) ProviderForAccount(_ context.Context, account *domain.Account, plan *domain.Plan) (domain.LeadEnrichmentProvider, error) {
	providerName := ""
	if plan != nil {
		providerName = plan.AIProvider
	}
	if providerName == "" {
		providerName = f.cfg.AIProvider
	}

	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "gemini", "google":
		// allowed — grounding is Gemini-only
	default:
		return nil, domain.ErrEnrichmentRequiresGemini
	}

	// Resolve the key. Contract plans bring their own (BYOK); self-serve plans
	// fall back to the platform env key.
	key := f.cfg.GeminiAPIKey
	if account != nil && plan != nil && account.IsContractPlan(plan) {
		key = ""
		if account.GeminiAPIKey != nil {
			key = *account.GeminiAPIKey
		}
	}
	if strings.TrimSpace(key) == "" {
		return nil, domain.ErrBYOKNotConfigured
	}

	return NewGeminiGroundingAdapter(key, ""), nil
}
