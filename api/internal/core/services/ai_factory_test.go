package services

import (
	"context"
	"testing"

	"github.com/replider/api/internal/config"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestAIFactory_GetCostConfig(t *testing.T) {
	cfg := &config.Config{AIProvider: "gemini"}
	factory := NewAIFactory(cfg)

	tests := []struct {
		name           string
		provider       string
		model          string
		wantInputMult  float64
		wantOutputMult float64
	}{
		{
			name:           "gemini flash (default) has 1:1 multiplier",
			provider:       "gemini",
			model:          "gemini-1.5-flash",
			wantInputMult:  1.0,
			wantOutputMult: 1.0,
		},
		{
			name:           "gemini pro is more expensive (5x/15x)",
			provider:       "gemini",
			model:          "gemini-1.5-pro",
			wantInputMult:  5.0,
			wantOutputMult: 15.0,
		},
		{
			name:           "gpt-4o is most expensive (25x/75x)",
			provider:       "openai",
			model:          "gpt-4o",
			wantInputMult:  25.0,
			wantOutputMult: 75.0,
		},
		{
			name:           "openai default is mid-tier (5x/15x)",
			provider:       "openai",
			model:          "gpt-3.5-turbo",
			wantInputMult:  5.0,
			wantOutputMult: 15.0,
		},
		{
			name:           "unknown provider defaults to 1:1",
			provider:       "anthropic",
			model:          "claude-3",
			wantInputMult:  1.0,
			wantOutputMult: 1.0,
		},
		{
			name:           "empty provider falls back to config default (gemini flash)",
			provider:       "",
			model:          "gemini-1.5-flash",
			wantInputMult:  1.0,
			wantOutputMult: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inMult, outMult := factory.GetCostConfig(tt.provider, tt.model)
			assert.Equal(t, tt.wantInputMult, inMult, "input multiplier mismatch")
			assert.Equal(t, tt.wantOutputMult, outMult, "output multiplier mismatch")
		})
	}
}

// Contract plans (Plan.StripePriceID == nil) must use the account's
// own API key — using the platform's env-level key would invalidate
// the BYOK contract. Without a configured key the factory returns
// ErrBYOKKeyMissing so the inbound pipeline can save the
// "configure your API key" fallback instead of silently degrading.
func TestAIFactory_GetProviderForAccount_BYOKMissingKey(t *testing.T) {
	cfg := &config.Config{AIProvider: "gemini", GeminiAPIKey: "platform-fallback"}
	factory := NewAIFactory(cfg)
	contractPlan := &domain.Plan{
		ID:            "plan_enterprise",
		Name:          "Enterprise",
		AIProvider:    "gemini",
		StripePriceID: nil, // contract plan
	}
	account := &domain.Account{
		ID:           "acc_1",
		PlanID:       &contractPlan.ID,
		GeminiAPIKey: nil, // BYOK slot empty
	}

	provider, err := factory.GetProviderForAccount(context.Background(), "gemini", account, contractPlan)

	assert.Nil(t, provider)
	assert.ErrorIs(t, err, domain.ErrBYOKKeyMissing)
}

func TestAIFactory_GetProviderForAccount_PlatformKeyAllowedFallsBackToEnv(t *testing.T) {
	cfg := &config.Config{AIProvider: "gemini", GeminiAPIKey: "platform-key"}
	factory := NewAIFactory(cfg)
	contractPlan := &domain.Plan{
		ID:            "plan_enterprise",
		Name:          "Enterprise",
		AIProvider:    "gemini",
		StripePriceID: nil, // contract plan
	}
	// Enterprise "our key" deal: contract account, no BYOK key, but the
	// owner allowed the platform key — must build a provider, not error.
	account := &domain.Account{
		ID:                 "acc_ent",
		PlanID:             &contractPlan.ID,
		GeminiAPIKey:       nil,
		PlatformKeyAllowed: true,
	}

	provider, err := factory.GetProviderForAccount(context.Background(), "gemini", account, contractPlan)

	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestAIFactory_GetProviderForAccount_SelfServeFallsBackToEnv(t *testing.T) {
	cfg := &config.Config{AIProvider: "gemini", GeminiAPIKey: "env-key"}
	factory := NewAIFactory(cfg)
	stripeID := "price_silver_brl"
	selfServePlan := &domain.Plan{
		ID:            "plan_silver",
		Name:          "Pro",
		AIProvider:    "gemini",
		StripePriceID: &stripeID,
	}
	account := &domain.Account{ID: "acc_2", PlanID: &selfServePlan.ID}

	// Self-serve account on Gemini hits the platform env key — succeeds
	// even with no per-account key (which is the whole point of self-
	// serve plans).
	provider, err := factory.GetProviderForAccount(context.Background(), "gemini", account, selfServePlan)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestAIFactory_GetProviderForAccount_NilAccountFallsThrough(t *testing.T) {
	// Preview / admin paths that don't have an account in scope go
	// through the legacy GetProvider semantics — the factory must not
	// require an account to build a provider when the use-case isn't
	// account-scoped (preview chat, smoke tests).
	cfg := &config.Config{AIProvider: "gemini", GeminiAPIKey: "env-key"}
	factory := NewAIFactory(cfg)
	provider, err := factory.GetProviderForAccount(context.Background(), "gemini", nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
}
