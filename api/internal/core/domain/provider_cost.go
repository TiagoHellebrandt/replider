package domain

import (
	"context"
	"time"
)

// ProviderCost is one row of the manual API-price table (US-005,
// vendedores v3). Costs are integer cents of USD per 1M tokens. A price
// change is a NEW row with a later EffectiveFrom — history is preserved
// so past-cycle profit recomputes (US-006) keep using the price that
// was in force then.
type ProviderCost struct {
	ID                      string    `json:"id"`
	Provider                string    `json:"provider"`
	Model                   string    `json:"model"`
	InputCostPer1MCentsUSD  int64     `json:"input_cost_per_1m_cents_usd"`
	OutputCostPer1MCentsUSD int64     `json:"output_cost_per_1m_cents_usd"`
	EffectiveFrom           time.Time `json:"effective_from"`
	CreatedAt               time.Time `json:"created_at"`
}

// ProviderCostRepository owns the provider_costs table.
type ProviderCostRepository interface {
	// List returns every row, newest effective_from first within each
	// provider/model group (the owner screen shows full history).
	List(ctx context.Context) ([]*ProviderCost, error)
	Insert(ctx context.Context, c *ProviderCost) error
	// Update rewrites an existing row in place (typo fixes). Returns
	// false when the id does not exist.
	Update(ctx context.Context, id string, inputCents, outputCents int64, effectiveFrom time.Time) (bool, error)
	// Delete removes a row. Returns false when the id does not exist.
	Delete(ctx context.Context, id string) (bool, error)
	// GetEffective returns the price in force for provider/model on the
	// given date: greatest effective_from <= at. nil when none.
	GetEffective(ctx context.Context, provider, model string, at time.Time) (*ProviderCost, error)
}

// SettingUSDBRLRateCents is the platform_settings key holding the
// manual USD->BRL conversion rate as integer BRL cents per 1 USD
// (550 = R$5.50). Owner-edited; deliberately not a live FX feed so
// profit computations are reproducible.
const SettingUSDBRLRateCents = "usd_brl_rate_cents"

// PlatformSettingRepository is the slim KV port over platform_settings.
type PlatformSettingRepository interface {
	// GetSetting returns "" (no error) when the key is absent.
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value, updatedBy string) error
}
