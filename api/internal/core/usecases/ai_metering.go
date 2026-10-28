package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/services"
)

// Transparent token metering (US-M015).
//
// Every AI call that is NOT a message goes through here. The rule is the
// same as case instrumentation: metering must never break the operation it
// measures — but unlike instrumentation, a MISSED entry has a price. It
// under-bills, which is the wrong direction when the promise is "the
// provider's real cost, no markup". So a failure is logged at ERROR, not
// WARN: it is money quietly not charged.
type AIMeter struct {
	repo domain.AICostRepository
}

func NewAIMeter(repo domain.AICostRepository) *AIMeter {
	return &AIMeter{repo: repo}
}

func (m *AIMeter) enabled() bool { return m != nil && m.repo != nil }

// Record prices one AI call and appends it to the ledger.
//
// The cost is computed HERE, at call time, from the tokens the provider
// reported — never recomputed later, because the owner-editable price table
// can change and a past month's invoice must not move with it.
func (m *AIMeter) Record(ctx context.Context, accountID, operation, provider, model string, usage domain.AIUsage) {
	if !m.enabled() || accountID == "" {
		return
	}
	if usage.PromptTokens <= 0 && usage.CompletionTokens <= 0 {
		// A provider that reported nothing is not a free call, but we
		// have nothing honest to charge for it either. Logged so a
		// provider that stops reporting usage is visible rather than
		// silently free.
		slog.WarnContext(ctx, "ai metering: provider reported no token usage",
			"operation", operation, "provider", provider, "model", model)
		return
	}
	// Cached prompt tokens are billed at a fraction of the input rate (K3), so
	// pricing them at full rate would hide the saving the prompt layering exists
	// to produce — and would keep reporting the same cost after the optimisation
	// landed. A provider that reports no cached count leaves this at 0 and is
	// priced exactly as before.
	cost := services.ProviderCostUSDCached(provider, model,
		usage.PromptTokens, usage.CachedTokens, usage.CompletionTokens)
	entry := &domain.AICostEntry{
		AccountID:    accountID,
		Operation:    operation,
		Provider:     provider,
		Model:        model,
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		CachedTokens: usage.CachedTokens,
		CostUSD:      cost,
	}
	if err := m.repo.Record(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "ai metering: failed to record cost — this under-bills the account",
			"operation", operation, "account_id", accountID, "cost_usd", cost, "error", err)
	}
}

// AICostUseCase serves the billing line.
type AICostUseCase struct {
	repo domain.AICostRepository
}

func NewAICostUseCase(repo domain.AICostRepository) *AICostUseCase {
	return &AICostUseCase{repo: repo}
}

// Summary aggregates [from, to) for one account.
func (uc *AICostUseCase) Summary(ctx context.Context, accountID string, from, to time.Time) (*domain.AICostSummary, error) {
	if uc == nil || uc.repo == nil {
		return nil, &domain.AppError{
			Code:    "AI_METERING_UNAVAILABLE",
			Message: "medição de custo de IA não configurada",
		}
	}
	return uc.repo.SumByAccount(ctx, accountID, from, to)
}
