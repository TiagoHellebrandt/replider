package services

import (
	"context"
	"strings"
	"sync"

	"github.com/replider/api/internal/adapters/ai"
	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
)

type AIFactory struct {
	cfg *config.Config
	// breakers persist failure counts between calls (US-026) — a per-call
	// breaker would reset each time and never trip. Keyed per (provider,
	// account) since 2026-09-01: with BYOK every tenant runs on its OWN key,
	// so one customer's revoked/over-quota key must trip THAT customer's
	// breaker, not everybody's. The platform key has its own entry
	// (breakerPlatformKey).
	breakersMu sync.Mutex
	breakers   map[string]*CircuitBreaker
}

// breakerPlatformKey is the account slot for calls made on the platform's
// env key (GetProvider, self-serve fallbacks, the platform account).
const breakerPlatformKey = "platform"

func NewAIFactory(cfg *config.Config) *AIFactory {
	return &AIFactory{cfg: cfg, breakers: map[string]*CircuitBreaker{}}
}

// breakerFor returns the breaker for one (provider, account) pair, creating
// it on first use. Entries are never evicted: one small struct per tenant
// per vendor is bounded by the customer count.
func (f *AIFactory) breakerFor(providerName, accountKey string) *CircuitBreaker {
	if accountKey == "" {
		accountKey = breakerPlatformKey
	}
	key := providerName + "|" + accountKey
	f.breakersMu.Lock()
	defer f.breakersMu.Unlock()
	if f.breakers == nil {
		f.breakers = map[string]*CircuitBreaker{}
	}
	cb, ok := f.breakers[key]
	if !ok {
		cb = NewCircuitBreaker()
		f.breakers[key] = cb
	}
	return cb
}

// GetProvider dynamically creates the correct AIProvider given the agent's
// provider name. Every provider is wrapped in RetryProvider so transient
// failures (5xx, rate limit, timeout) are automatically retried with
// exponential backoff before surfacing an error to the caller.
func (f *AIFactory) GetProvider(ctx context.Context, providerName string) (domain.AIProvider, error) {
	return f.buildProvider(ctx, providerName, "", f.cfg.OpenAIAPIKey, f.cfg.GeminiAPIKey, breakerPlatformKey)
}

// GetProviderForAccount picks the right API key based on the plan:
//   - Self-serve plans (StripePriceID set): use platform env keys.
//   - Contract plans (StripePriceID nil → BYOK): use the account's
//     stored key for the chosen provider; ErrBYOKKeyMissing when
//     unset so the caller can route to the "configure your key"
//     fallback instead of a generic AI-down message.
//
// nil account or nil plan fall back to GetProvider — keeps preview /
// admin-only paths that don't have an account in scope working.
func (f *AIFactory) GetProviderForAccount(ctx context.Context, providerName string, account *domain.Account, plan *domain.Plan) (domain.AIProvider, error) {
	// AI_REQUIRE_BYOK: the whole product runs on the customer's own key —
	// a self-serve/trial account no longer falls back to the platform env
	// key, it gets the same "configure your key" refusal a contract plan
	// gets. The platform account is exempt: its internal CS agent runs on
	// the env key we already pay for.
	requireBYOK := f.cfg.AIRequireBYOK && account != nil &&
		!f.cfg.IsPlatformAccount(account.ID)

	if !requireBYOK && (account == nil || plan == nil || !account.IsContractPlan(plan)) {
		model := ""
		if plan != nil {
			model = plan.AIModel
		}
		return f.buildProvider(ctx, providerName, model, f.cfg.OpenAIAPIKey, f.cfg.GeminiAPIKey, breakerPlatformKey)
	}
	if plan == nil {
		plan = domain.TrialPlan()
	}

	if providerName == "" {
		providerName = plan.AIProvider
	}
	if providerName == "" {
		providerName = f.cfg.AIProvider
	}

	openAIKey := derefStringPtr(account.OpenAIAPIKey)
	geminiKey := derefStringPtr(account.GeminiAPIKey)

	// "Our key" contract path: an Enterprise deal can be billed on the
	// platform key (at the plan's higher multipliers) instead of BYOK.
	// Only when the owner explicitly flipped platform_key_allowed AND the
	// account hasn't set its own key — a configured BYOK key still wins.
	missingKey := false
	switch providerName {
	case "openai":
		missingKey = openAIKey == ""
	case "gemini", "google":
		missingKey = geminiKey == ""
	default:
		// Unknown provider on a contract plan — bail rather than silently
		// using env keys (which would invalidate the BYOK contract).
		return nil, domain.ErrBYOKKeyMissing
	}
	if missingKey {
		if account.PlatformKeyAllowed {
			return f.buildProvider(ctx, providerName, plan.AIModel, f.cfg.OpenAIAPIKey, f.cfg.GeminiAPIKey, breakerPlatformKey)
		}
		return nil, domain.ErrBYOKKeyMissing
	}

	// The plan names the model; until now it was ignored and every account
	// silently ran the default.
	return f.buildProvider(ctx, providerName, plan.AIModel, openAIKey, geminiKey, account.ID)
}

// Fallback models, used when nothing upstream names one.
//
// gemini-2.0-flash was deprecated for new users (404 from the SDK as of
// 2026-04); gemini-2.5-flash succeeded it. gemini-3.7-flash (2026-08) is
// the current flash tier — migration 000206 moved plans and agents to it.
const (
	defaultGeminiModel = "gemini-3.7-flash"
	defaultOpenAIModel = "gpt-4o-mini"
)

// resolveModel picks the model actually sent to the provider.
//
// This used to be a hardcoded string, which made plans.ai_model decorative:
// every account ran gemini-2.5-flash no matter what its plan row said, and
// the cost meter then labelled the spend with the model from the plan — a
// model that never ran. Worse, it made "use a stronger model where being
// wrong is expensive" impossible to act on, which is the whole point of
// having the column.
func resolveModel(providerName, requested string) string {
	if m := strings.TrimSpace(requested); m != "" {
		return m
	}
	if providerName == "openai" {
		return defaultOpenAIModel
	}
	return defaultGeminiModel
}

func (f *AIFactory) buildProvider(ctx context.Context, providerName, model, openAIKey, geminiKey, breakerAccount string) (domain.AIProvider, error) {
	if providerName == "" {
		providerName = f.cfg.AIProvider
	}
	model = resolveModel(providerName, model)

	var p domain.AIProvider
	var err error
	switch providerName {
	case "openai":
		p = ai.NewOpenAIProvider(openAIKey, model)
	case "gemini", "google":
		p, err = ai.NewGeminiProvider(ctx, geminiKey, model)
	default:
		p, err = ai.NewGeminiProvider(ctx, geminiKey, model)
	}
	if err != nil {
		return nil, err
	}
	// Layering: breaker(retry(tracing(provider))). Retry absorbs transient
	// blips; the breaker trips only once retries are consistently exhausted,
	// i.e. the provider is genuinely down. Tracing sits innermost so its
	// span measures the real provider round-trip, not retry back-off (US-066).
	return NewBreakerProvider(ai.NewRetryProvider(NewTracingProvider(p, providerName)), f.breakerFor(providerName, breakerAccount)), nil
}

func derefStringPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// GetCostConfig returns (inputMultiplier, outputMultiplier)
// according to the "Replider Credit" system for strict margin protection.
func (f *AIFactory) GetCostConfig(provider, model string) (float64, float64) {
	if provider == "" {
		provider = f.cfg.AIProvider
	}

	switch provider {
	case "openai":
		if model == "gpt-4o" {
			return 25.0, 75.0
		}
		// default openai
		return 5.0, 15.0
	case "gemini":
		if model == "gemini-1.5-pro" {
			return 5.0, 15.0
		}
		// default back to flash
		return 1.0, 1.0
	default:
		return 1.0, 1.0
	}
}
