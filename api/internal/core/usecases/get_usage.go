package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

type GetUsageUseCase struct {
	usageRepo   domain.UsageRepository
	planRepo    domain.PlanRepository
	accountRepo domain.AccountRepository
	// aiCostRepo is optional (SetAICostRepo) — see US-M015.
	aiCostRepo domain.AICostRepository
	// salesEventRepo is optional (SetSalesEventRepo): without it the payload
	// simply carries no consumption in SDR units.
	salesEventRepo domain.SalesEventRepository
}

func NewGetUsageUseCase(usageRepo domain.UsageRepository, planRepo domain.PlanRepository, accountRepo domain.AccountRepository) *GetUsageUseCase {
	return &GetUsageUseCase{
		usageRepo:   usageRepo,
		planRepo:    planRepo,
		accountRepo: accountRepo,
	}
}

type GetUsageResponse struct {
	AITokensUsed              int64   `json:"ai_tokens_used"`
	AIInputTokensUsed         int64   `json:"ai_input_tokens_used"`
	AIOutputTokensUsed        int64   `json:"ai_output_tokens_used"`
	AddonTokens               int64   `json:"addon_tokens"`
	ConversationsCount        int32   `json:"conversations_count"`
	MaxAITokensPerMonth       int64   `json:"max_ai_tokens_per_month"`
	MaxConversationsPerMonth  int     `json:"max_conversations_per_month"`
	EffectiveLimit            int64   `json:"effective_limit"`
	TokensUsagePercent        float64 `json:"tokens_usage_percent"`
	ConversationsUsagePercent float64 `json:"conversations_usage_percent"`
	PlanName                  string  `json:"plan_name"`
	IsTrialing                bool    `json:"is_trialing"`
	// Conversation overage (see docs/OVERAGE_METERING_DESIGN.md). Lets
	// the dashboard tell the user it has passed the plan franchise and
	// is now paying per conversation, and how much so far this cycle.
	OverageConversationsCount int32 `json:"overage_conversations_count"`
	OverageCentsCharged       int32 `json:"overage_cents_charged"`
	OverageConversationCents  int   `json:"overage_conversation_cents"`
	CapReached                bool  `json:"cap_reached"`

	// Transparent token metering (US-M015): what the provider charged this
	// cycle, passed through with no markup. Nil when metering is unwired —
	// absent, never a zero that would read as "AI was free this month".
	AICostBRLCents  *int64   `json:"ai_cost_brl_cents,omitempty"`
	AICostUSD       *float64 `json:"ai_cost_usd,omitempty"`
	AICostRateCents *int     `json:"ai_cost_rate_cents,omitempty"`

	// The month's consumption in the units the customer buys. Tokens are our
	// cost; what they purchase is leads, research and approaches. Absent when it
	// cannot be counted — a zero here would read as "I used nothing", which is
	// different from "I don't know".
	SDR *SDRUsage `json:"sdr,omitempty"`
}

// SDRUsage é o mês corrente contado no log de eventos de venda — a mesma
// source the caps use to block. A second count would disagree with the first on
// the day somebody forgot to increment it.
type SDRUsage struct {
	Leads          int64 `json:"leads"`
	Enrichments    int64 `json:"enrichments"`
	ApproachesSent int64 `json:"approaches_sent"`
	Meetings       int64 `json:"meetings"`
}

// SetSalesEventRepo wires the month's SDR counters onto the usage payload.
// Optional: without it, the `sdr` block simply does not come.
func (uc *GetUsageUseCase) SetSalesEventRepo(r domain.SalesEventRepository) {
	uc.salesEventRepo = r
}

// sdrForCycle counts the current month. A failed count returns nil: saying
// "0 researches" because the query broke is the lie this block exists to
// prevent.
func (uc *GetUsageUseCase) sdrForCycle(ctx context.Context, accountID string) *SDRUsage {
	if uc.salesEventRepo == nil {
		return nil
	}
	now := time.Now()
	// The same window start the enrichment cap uses, so the number shown and the
	// number that blocks are one number.
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	count := func(kind domain.SalesEventKind) (int64, bool) {
		n, err := uc.salesEventRepo.CountSince(ctx, accountID, kind, from)
		if err != nil {
			slog.WarnContext(ctx, "usage: failed to count sales events",
				"account_id", accountID, "kind", kind, "error", err)
			return 0, false
		}
		return n, true
	}

	leads, ok1 := count(domain.SalesEventLeadCreated)
	enrichments, ok2 := count(domain.SalesEventEnrichmentRun)
	approaches, ok3 := count(domain.SalesEventApproachSent)
	meetings, ok4 := count(domain.SalesEventMeetingBooked)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return nil
	}
	return &SDRUsage{
		Leads:          leads,
		Enrichments:    enrichments,
		ApproachesSent: approaches,
		Meetings:       meetings,
	}
}

// SetAICostRepo wires the metered AI spend onto the usage payload
// (US-M015). Optional: unwired, the fields are simply absent.
func (uc *GetUsageUseCase) SetAICostRepo(r domain.AICostRepository) {
	uc.aiCostRepo = r
}

// aiCostForCycle totals the account's AI spend for the current billing
// cycle. A failure logs and returns nil rather than 0 — showing "R$0,00"
// because a query failed is exactly the lie this feature exists to avoid.
func (uc *GetUsageUseCase) aiCostForCycle(ctx context.Context, accountID string) *domain.AICostSummary {
	if uc.aiCostRepo == nil {
		return nil
	}
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	summary, err := uc.aiCostRepo.SumByAccount(ctx, accountID, from, now)
	if err != nil {
		slog.WarnContext(ctx, "usage: failed to total AI cost", "account_id", accountID, "error", err)
		return nil
	}
	return summary
}

func (uc *GetUsageUseCase) Execute(ctx context.Context, accountID string) (*GetUsageResponse, error) {
	// 1. Fetch Account to get the PlanID
	account, err := uc.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, domain.ErrAccountNotFound
	}

	if account == nil {
		return nil, domain.ErrAccountNotFound
	}

	// 2. Fetch Usage from the materialized view (US-065). This read-only
	// dashboard surface tolerates up to one 5-min refresh cycle of staleness
	// in exchange for collapsing the per-request latest-cycle aggregation into
	// a single unique-index lookup. Live plan-limit enforcement still reads the
	// table directly via GetByAccountID.
	usage, err := uc.usageRepo.GetSummaryByAccountID(ctx, accountID)
	if err != nil {
		return nil, domain.ErrAccountNotFound // Fallback error
	}
	if usage == nil {
		// If usage somehow doesn't exist, we can return zeroes
		return &GetUsageResponse{
			AITokensUsed:             0,
			ConversationsCount:       0,
			MaxAITokensPerMonth:      10000, // Trial defaults — match domain.TrialPlan()
			MaxConversationsPerMonth: 50,
		}, nil
	}

	conversationsCount := int32(0)
	if usage.ConversationsCount != nil {
		conversationsCount = *usage.ConversationsCount
	}

	aiTokensUsed := int64(0)
	if usage.AITokensUsed != nil {
		aiTokensUsed = *usage.AITokensUsed
	}

	aiInputTokensUsed := int64(0)
	if usage.AIInputTokensUsed != nil {
		aiInputTokensUsed = *usage.AIInputTokensUsed
	}

	aiOutputTokensUsed := int64(0)
	if usage.AIOutputTokensUsed != nil {
		aiOutputTokensUsed = *usage.AIOutputTokensUsed
	}

	addonTokens := int64(0)
	if usage.AddonTokens != nil {
		addonTokens = *usage.AddonTokens
	}

	// 3. Fetch Plan. Fallbacks mirror domain.TrialPlan() so the banner
	// and server-side usage guard agree on the trial allowance.
	maxTokens := int64(10000)
	maxConversations := 50
	planName := "Trial"
	overageRate := 0
	isTrialing := account.SubscriptionStatus == domain.SubscriptionStatusTrialing

	if account.PlanID != nil {
		plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
		if err == nil && plan != nil {
			// EffectiveMax*, not the raw plan column: a contract plan
			// stores 0 and relies on the account's override (or the trial
			// floor). Reading the raw value showed those accounts a limit
			// of zero — and made the usage banner disagree with the guard
			// that actually enforces the cap, which uses EffectiveMax.
			maxTokens = account.EffectiveMaxAITokensPerMonth(plan)
			maxConversations = account.EffectiveMaxConversationsPerMonth(plan)
			planName = plan.Name
			overageRate = plan.OverageConversationCents
		}
	}

	effectiveLimit := maxTokens + addonTokens

	tokensUsagePercent := float64(0)
	if effectiveLimit > 0 {
		tokensUsagePercent = float64(aiTokensUsed) / float64(effectiveLimit) * 100
	}

	conversationsUsagePercent := float64(0)
	if maxConversations > 0 {
		conversationsUsagePercent = float64(conversationsCount) / float64(maxConversations) * 100
	}

	resp := &GetUsageResponse{
		AITokensUsed:              aiTokensUsed,
		AIInputTokensUsed:         aiInputTokensUsed,
		AIOutputTokensUsed:        aiOutputTokensUsed,
		AddonTokens:               addonTokens,
		ConversationsCount:        conversationsCount,
		MaxAITokensPerMonth:       maxTokens,
		MaxConversationsPerMonth:  maxConversations,
		EffectiveLimit:            effectiveLimit,
		TokensUsagePercent:        tokensUsagePercent,
		ConversationsUsagePercent: conversationsUsagePercent,
		PlanName:                  planName,
		IsTrialing:                isTrialing,
		OverageConversationsCount: usage.OverageConversationsCount,
		OverageCentsCharged:       usage.OverageCentsCharged,
		OverageConversationCents:  overageRate,
		CapReached:                usage.CapReachedAt != nil,
	}
	if cost := uc.aiCostForCycle(ctx, accountID); cost != nil {
		resp.AICostBRLCents = &cost.TotalBRLCents
		resp.AICostUSD = &cost.TotalUSD
		resp.AICostRateCents = &cost.RateCents
	}
	resp.SDR = uc.sdrForCycle(ctx, accountID)
	return resp, nil
}
