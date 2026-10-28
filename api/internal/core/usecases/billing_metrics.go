package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// BillingMetricsUseCase derives MRR / churn / LTV from the append-only
// billing_events ledger (US-071). The repository does the SQL
// aggregation; this use case turns the raw counts into the business
// ratios so the arithmetic stays unit-testable without a DB.
//
// MRR is reported per currency AND as a BRL-consolidated total (US-018
// review fixes): the repository never adds BRL and USD cents together —
// the USD slice is converted here at the owner-managed
// usd_brl_rate_cents so ARPA/LTV stay meaningful single-currency values.
type BillingMetricsUseCase struct {
	repo     domain.BillingEventRepository
	settings domain.PlatformSettingRepository
	now      func() time.Time
}

func NewBillingMetricsUseCase(repo domain.BillingEventRepository) *BillingMetricsUseCase {
	return &BillingMetricsUseCase{repo: repo, now: time.Now}
}

// SetPlatformSettings wires the platform_settings KV so the USD→BRL
// consolidation uses the owner-managed rate. Optional — nil falls back
// to DefaultUSDBRLRateCents.
func (uc *BillingMetricsUseCase) SetPlatformSettings(s domain.PlatformSettingRepository) {
	uc.settings = s
}

// BillingMetricsResult is the admin dashboard payload. Monetary values
// are in cents to avoid float rounding; ratios are unitless.
type BillingMetricsResult struct {
	// WindowDays is the churn lookback window used.
	WindowDays int `json:"window_days"`
	// MRRCents is monthly recurring revenue consolidated in BRL cents:
	// the BRL slice plus the USD slice converted at USDBRLRateCents.
	MRRCents int64 `json:"mrr_cents"`
	// MRRBRLCents is the BRL-native slice of MRR (BRL accounts only).
	MRRBRLCents int64 `json:"mrr_brl_cents"`
	// MRRUSDCents is the USD slice of MRR, in USD cents (unconverted).
	MRRUSDCents int64 `json:"mrr_usd_cents"`
	// USDBRLRateCents is the conversion rate applied (BRL cents per USD).
	USDBRLRateCents int64 `json:"usd_brl_rate_cents"`
	// ActiveAccounts is paying accounts (latest event not a churn).
	ActiveAccounts int `json:"active_accounts"`
	// ChurnedInWindow is churn events within the window.
	ChurnedInWindow int `json:"churned_in_window"`
	// ARPACents is average revenue per active account (MRR / active),
	// in consolidated BRL cents.
	ARPACents int64 `json:"arpa_cents"`
	// ChurnRate is churned / (active + churned) over the window, [0,1].
	ChurnRate float64 `json:"churn_rate"`
	// LTVCents is estimated lifetime value: ARPA / churn rate. Zero when
	// churn rate is zero (lifetime unbounded — not meaningful to render).
	LTVCents int64 `json:"ltv_cents"`
}

// Execute computes the metrics over the given churn window. A windowDays
// of <= 0 falls back to 30.
func (uc *BillingMetricsUseCase) Execute(ctx context.Context, windowDays int) (*BillingMetricsResult, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	since := uc.now().AddDate(0, 0, -windowDays)

	agg, err := uc.repo.Aggregate(ctx, since)
	if err != nil {
		return nil, err
	}

	rate, err := getIntSetting(ctx, uc.settings, domain.SettingUSDBRLRateCents, DefaultUSDBRLRateCents)
	if err != nil {
		slog.Warn("billing metrics: failed to read usd_brl rate, using default", "error", err)
		rate = DefaultUSDBRLRateCents
	}
	usdAsBRL, ok := domain.ConvertCents(agg.MRRUSDCents, domain.CurrencyUSD, domain.CurrencyBRL, rate)
	if !ok {
		// Non-positive rate — refuse to fabricate a conversion; report
		// the BRL slice only and keep the raw USD slice visible.
		slog.Error("billing metrics: unpriceable usd_brl rate, MRR consolidation excludes USD", "rate_cents", rate)
		usdAsBRL = 0
	}
	if agg.OtherCurrencyAccounts > 0 {
		slog.Error("billing metrics: active accounts with unsupported currency excluded from MRR",
			"count", agg.OtherCurrencyAccounts)
	}

	res := &BillingMetricsResult{
		WindowDays:      windowDays,
		MRRCents:        agg.MRRBRLCents + usdAsBRL,
		MRRBRLCents:     agg.MRRBRLCents,
		MRRUSDCents:     agg.MRRUSDCents,
		USDBRLRateCents: rate,
		ActiveAccounts:  agg.ActiveAccounts,
		ChurnedInWindow: agg.ChurnedInWindow,
	}

	if agg.ActiveAccounts > 0 {
		res.ARPACents = res.MRRCents / int64(agg.ActiveAccounts)
	}

	// Churn rate uses the cohort that was around in the window: the
	// accounts still active plus the ones that churned in it.
	denom := agg.ActiveAccounts + agg.ChurnedInWindow
	if denom > 0 {
		res.ChurnRate = float64(agg.ChurnedInWindow) / float64(denom)
	}

	// LTV = ARPA / churn rate. Undefined when nobody churns; report 0 so
	// the dashboard doesn't render an infinity.
	if res.ChurnRate > 0 {
		res.LTVCents = int64(float64(res.ARPACents) / res.ChurnRate)
	}

	return res, nil
}
