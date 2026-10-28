package usecases

import (
	"context"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/replider/api/internal/core/domain"
)

// ComputeMonthlyProfitUseCase computes per-account monthly net profit
// (US-006, vendedores v3) — the honest base the commission ledger
// (US-007) multiplies its rates against.
//
// net = revenue (Stripe invoices PAID in the cycle minus refunds, real
// fee via each charge's balance_transaction) − token cost (the cycle's
// per-message stamped provider_cost_usd × manual USD→BRL rate) − tax
// (profit_tax_bps on revenue) − infra (flat monthly constant).
// Everything in integer BRL cents; recompute upserts, so reprocessing
// a cycle is idempotent.
type ComputeMonthlyProfitUseCase struct {
	repo     domain.MonthlyProfitRepository
	settings domain.PlatformSettingRepository
	accounts profitAccountGetter
	invoices PaidInvoiceLister
	auditLog domain.AuditLogRepository
}

// profitAccountGetter is the slim slice of AccountRepository the
// computation needs (narrow local interface — keeps mocks tiny).
type profitAccountGetter interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
}

// PaidInvoiceLister is the slim Stripe port: invoices paid inside
// [from, to) with the real balance_transaction fee expanded. nil-able —
// deployments without Stripe compute revenue 0.
type PaidInvoiceLister interface {
	ListPaidInvoicesInRange(customerID string, from, to time.Time) ([]domain.StripePaidInvoice, error)
}

func NewComputeMonthlyProfitUseCase(
	repo domain.MonthlyProfitRepository,
	settings domain.PlatformSettingRepository,
	accounts profitAccountGetter,
	invoices PaidInvoiceLister,
	auditLog domain.AuditLogRepository,
) *ComputeMonthlyProfitUseCase {
	return &ComputeMonthlyProfitUseCase{
		repo:     repo,
		settings: settings,
		accounts: accounts,
		invoices: invoices,
		auditLog: auditLog,
	}
}

// Defaults when the platform_settings rows are absent/corrupted
// (match the migration seeds).
const (
	DefaultProfitTaxBps   = 0
	DefaultInfraCostCents = 500
	maxProfitTaxBps       = 5000   // 50% sanity rail
	maxInfraCostCents     = 100000 // R$1000/account/month sanity rail
)

func errInvalidCycle() error {
	return &domain.AppError{Code: "PROFIT_INVALID_CYCLE", Message: "cycle must be YYYY-MM"}
}

// cycleLocation is the platform's business timezone — the same
// America/Cuiaba window the SLA metric uses (US-008 review fix). UTC
// month boundaries shifted every cycle edge by 4h: an invoice paid (or
// tokens burned) on the last evening of a month landed in the NEXT
// cycle's revenue/cost while the SLA signals stayed on local time.
var cycleLocation = func() *time.Location {
	loc, err := time.LoadLocation(DefaultSLATimezone)
	if err != nil {
		return time.UTC
	}
	return loc
}()

// parseCycle validates "YYYY-MM" and returns the month boundaries
// [from, to) in the platform business timezone (America/Cuiaba).
func parseCycle(cycle string) (from, to time.Time, err error) {
	from, err = time.ParseInLocation("2006-01", cycle, cycleLocation)
	if err != nil || from.Format("2006-01") != cycle {
		return time.Time{}, time.Time{}, errInvalidCycle()
	}
	return from, from.AddDate(0, 1, 0), nil
}

// RecomputeCycle recomputes one cycle. accountID narrows to a single
// account; empty recomputes every seller-linked account (the
// commission base). Upserts make reruns idempotent — same cycle, same
// inputs, same single row per account.
func (uc *ComputeMonthlyProfitUseCase) RecomputeCycle(ctx context.Context, actorEmail, cycle, accountID string) ([]*domain.AccountMonthlyProfit, error) {
	from, to, err := parseCycle(cycle)
	if err != nil {
		return nil, err
	}

	var targets []domain.ProfitComputeTarget
	if accountID != "" {
		acc, err := uc.accounts.GetByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		targets = []domain.ProfitComputeTarget{{AccountID: acc.ID, StripeCustomerID: acc.StripeCustomerID}}
	} else {
		targets, err = uc.repo.ListComputeTargets(ctx)
		if err != nil {
			return nil, err
		}
	}

	taxBps, err := uc.getIntSetting(ctx, domain.SettingProfitTaxBps, DefaultProfitTaxBps)
	if err != nil {
		return nil, err
	}
	infraCents, err := uc.getIntSetting(ctx, domain.SettingInfraCostCents, DefaultInfraCostCents)
	if err != nil {
		return nil, err
	}
	rateCents, err := uc.getIntSetting(ctx, domain.SettingUSDBRLRateCents, DefaultUSDBRLRateCents)
	if err != nil {
		return nil, err
	}

	out := make([]*domain.AccountMonthlyProfit, 0, len(targets))
	for _, t := range targets {
		p, err := uc.computeOne(ctx, t, cycle, from, to, int64(taxBps), int64(infraCents), int64(rateCents))
		if err != nil {
			return nil, err
		}
		if err := uc.repo.Upsert(ctx, p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	uc.audit(ctx, actorEmail, "monthly_profit.recompute", cycle)
	slog.Info("owner recomputed monthly profit",
		"actor", actorEmail, "cycle", cycle, "accounts", len(out), "single_account", accountID != "")
	return out, nil
}

func (uc *ComputeMonthlyProfitUseCase) computeOne(ctx context.Context, t domain.ProfitComputeTarget, cycle string, from, to time.Time, taxBps, infraCents, rateCents int64) (*domain.AccountMonthlyProfit, error) {
	var revenue, fee int64
	if uc.invoices != nil && t.StripeCustomerID != nil && *t.StripeCustomerID != "" {
		invoices, err := uc.invoices.ListPaidInvoicesInRange(*t.StripeCustomerID, from, to)
		if err != nil {
			return nil, err
		}
		for _, inv := range invoices {
			// Revenue is what the customer effectively kept paying:
			// paid minus refunded (refunds and refund-backed credit
			// notes come back on the charge's amount_refunded, same
			// currency as the invoice). Clamp at zero — a currency
			// edge can over-refund by a cent.
			netPaid := inv.AmountPaidCents - inv.RefundedCents
			if netPaid < 0 {
				netPaid = 0
			}
			// Only BRL and USD are priceable (USD converts at the same
			// manual rate as token costs). Any other currency — on the
			// invoice or on the balance_transaction fee — is excluded
			// whole (revenue AND fee) with a loud log: summing foreign
			// cents as raw BRL corrupts the commission base in either
			// direction (US-008 review fix).
			var revCents int64
			switch inv.Currency {
			case "brl":
				revCents = netPaid
			case "usd":
				revCents = roundedDiv(netPaid*rateCents, 100)
			default:
				slog.Error("monthly profit: invoice in unsupported currency — excluded, reconcile manually",
					"account_id", t.AccountID, "invoice_id", inv.ID,
					"currency", inv.Currency, "amount_paid_cents", inv.AmountPaidCents)
				continue
			}
			// The fee is denominated in the balance_transaction's own
			// SETTLEMENT currency — never assume BRL (a USD-settling
			// Stripe account reports USD fees). Empty means the charge
			// had no expanded balance_transaction (fee 0).
			var feeCents int64
			switch inv.FeeCurrency {
			case "", "brl":
				feeCents = inv.FeeCents
			case "usd":
				feeCents = roundedDiv(inv.FeeCents*rateCents, 100)
			default:
				slog.Error("monthly profit: balance_transaction fee in unsupported currency — invoice excluded, reconcile manually",
					"account_id", t.AccountID, "invoice_id", inv.ID,
					"fee_currency", inv.FeeCurrency, "fee_cents", inv.FeeCents)
				continue
			}
			revenue += revCents
			fee += feeCents
		}
	}

	tokenCost, err := uc.tokenCostCents(ctx, t.AccountID, from, to, rateCents)
	if err != nil {
		return nil, err
	}

	tax := roundedDiv(revenue*taxBps, 10000)
	net := revenue - fee - tokenCost - tax - infraCents

	return &domain.AccountMonthlyProfit{
		ID:             ulid.Make().String(),
		AccountID:      t.AccountID,
		Cycle:          cycle,
		RevenueCents:   revenue,
		StripeFeeCents: fee,
		TokenCostCents: tokenCost,
		TaxCents:       tax,
		InfraCents:     infraCents,
		NetProfitCents: net,
		IsNegative:     net < 0,
	}, nil
}

// tokenCostCents sums the cycle's per-message stamped cost
// (messages.provider_cost_usd, written at send time from the provider's
// usage report) and converts once via the manual USD→BRL rate. The
// stamp is immutable attribution — deleting or repointing an agent
// never rewrites a past cycle's cost. Rows with tokens but no stamped
// cost (pre-stamp history) cost zero and log a warning.
func (uc *ComputeMonthlyProfitUseCase) tokenCostCents(ctx context.Context, accountID string, from, to time.Time, rateCents int64) (int64, error) {
	tc, err := uc.repo.SumTokenCost(ctx, accountID, from, to)
	if err != nil {
		return 0, err
	}
	if tc.UnpricedInputTokens > 0 || tc.UnpricedOutputTokens > 0 {
		slog.Warn("monthly profit: token usage without a stamped provider cost — priced at zero",
			"account_id", accountID,
			"input_tokens", tc.UnpricedInputTokens, "output_tokens", tc.UnpricedOutputTokens)
	}
	// One rounding, at the end (US-008 review fix): rounding the USD
	// sum to whole USD cents FIRST and converting after compounded the
	// error — sub-cent costs rounded up to a full USD cent inflated the
	// BRL cost by the whole exchange rate.
	return int64(math.Round(tc.CostUSD * float64(rateCents))), nil
}

// roundedDiv divides non-negative cents half-up (money never truncates
// silently).
func roundedDiv(n, div int64) int64 {
	return (n + div/2) / div
}

// ProfitCycleListing bundles the cycle rows with the settings that
// produced them so the owner screen renders in one GET.
type ProfitCycleListing struct {
	Cycle           string                         `json:"cycle"`
	Items           []*domain.AccountMonthlyProfit `json:"items"`
	TaxBps          int                            `json:"tax_bps"`
	InfraCostCents  int                            `json:"infra_cost_cents"`
	USDBRLRateCents int                            `json:"usd_brl_rate_cents"`
}

func (uc *ComputeMonthlyProfitUseCase) List(ctx context.Context, cycle string) (*ProfitCycleListing, error) {
	if _, _, err := parseCycle(cycle); err != nil {
		return nil, err
	}
	items, err := uc.repo.ListByCycle(ctx, cycle)
	if err != nil {
		return nil, err
	}
	taxBps, err := uc.getIntSetting(ctx, domain.SettingProfitTaxBps, DefaultProfitTaxBps)
	if err != nil {
		return nil, err
	}
	infraCents, err := uc.getIntSetting(ctx, domain.SettingInfraCostCents, DefaultInfraCostCents)
	if err != nil {
		return nil, err
	}
	rateCents, err := uc.getIntSetting(ctx, domain.SettingUSDBRLRateCents, DefaultUSDBRLRateCents)
	if err != nil {
		return nil, err
	}
	return &ProfitCycleListing{
		Cycle:           cycle,
		Items:           items,
		TaxBps:          taxBps,
		InfraCostCents:  infraCents,
		USDBRLRateCents: rateCents,
	}, nil
}

// SetProfitSettings persists the owner-edited tax % (basis points) and
// flat infra cost. Changing them only affects FUTURE recomputes — past
// rows keep the values they were computed with until recomputed.
func (uc *ComputeMonthlyProfitUseCase) SetProfitSettings(ctx context.Context, actorEmail string, taxBps, infraCents int) error {
	if taxBps < 0 || taxBps > maxProfitTaxBps {
		return &domain.AppError{Code: "PROFIT_INVALID_TAX", Message: "tax_bps must be between 0 and 5000 (basis points on revenue)"}
	}
	if infraCents < 0 || infraCents > maxInfraCostCents {
		return &domain.AppError{Code: "PROFIT_INVALID_INFRA", Message: "infra_cost_cents must be between 0 and 100000 (BRL cents per account per month)"}
	}
	if err := uc.settings.SetSetting(ctx, domain.SettingProfitTaxBps, strconv.Itoa(taxBps), actorEmail); err != nil {
		return err
	}
	if err := uc.settings.SetSetting(ctx, domain.SettingInfraCostCents, strconv.Itoa(infraCents), actorEmail); err != nil {
		return err
	}
	uc.audit(ctx, actorEmail, "platform_setting.set_profit_settings", domain.SettingProfitTaxBps)
	slog.Info("owner set profit settings", "actor", actorEmail, "tax_bps", taxBps, "infra_cost_cents", infraCents)
	return nil
}

// getIntSetting parses an integer platform setting, falling back to
// def when absent or unparsable (same posture as GetUSDBRLRateCents).
func (uc *ComputeMonthlyProfitUseCase) getIntSetting(ctx context.Context, key string, def int) (int, error) {
	raw, err := uc.settings.GetSetting(ctx, key)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def, nil
	}
	return v, nil
}

func (uc *ComputeMonthlyProfitUseCase) audit(ctx context.Context, actorEmail, action, entityID string) {
	if uc.auditLog == nil {
		return
	}
	_ = uc.auditLog.Create(ctx, &domain.AuditLog{
		ActorID:    &actorEmail,
		Action:     action,
		EntityType: "monthly_profit",
		EntityID:   entityID,
	})
}
