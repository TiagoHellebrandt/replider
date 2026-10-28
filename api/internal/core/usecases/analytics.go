package usecases

import (
	"context"

	"github.com/replider/api/internal/core/domain"
)

type GetAnalyticsUseCase struct {
	analyticsRepo domain.AnalyticsRepository
	usageRepo     domain.UsageRepository
	planRepo      domain.PlanRepository
	accountRepo   domain.AccountRepository
	csatRepo      domain.CSATRepository // optional — nil disables CSAT aggregates
}

func NewGetAnalyticsUseCase(
	analyticsRepo domain.AnalyticsRepository,
	usageRepo domain.UsageRepository,
	planRepo domain.PlanRepository,
	accountRepo domain.AccountRepository,
) *GetAnalyticsUseCase {
	return &GetAnalyticsUseCase{
		analyticsRepo: analyticsRepo,
		usageRepo:     usageRepo,
		planRepo:      planRepo,
		accountRepo:   accountRepo,
	}
}

// SetCSATRepo wires the CSAT repository. Omitted by default so legacy
// tests that construct the use case without a CSAT backend still work.
func (uc *GetAnalyticsUseCase) SetCSATRepo(r domain.CSATRepository) {
	uc.csatRepo = r
}

type AnalyticsResponse struct {
	Conversations    domain.ConversationStats  `json:"conversations"`
	Messages         domain.MessageStats       `json:"messages"`
	DailyActivity    []domain.DailyCount       `json:"daily_activity"`
	SLA              *domain.SLAMetrics        `json:"sla"`
	AgentPerformance []domain.AgentPerformance `json:"agent_performance"`
	AI               *domain.AIPerformance     `json:"ai"`
	CSAT             *domain.CSATStats         `json:"csat,omitempty"`
	TokensUsed       int64                     `json:"tokens_used"`
	TokensLimit      int64                     `json:"tokens_limit"`
}

func (uc *GetAnalyticsUseCase) Execute(ctx context.Context, accountID string) (*AnalyticsResponse, error) {
	convStats, err := uc.analyticsRepo.GetConversationStats(ctx, accountID)
	if err != nil {
		return nil, err
	}

	msgStats, err := uc.analyticsRepo.GetMessageStats(ctx, accountID)
	if err != nil {
		return nil, err
	}

	dailyCounts, err := uc.analyticsRepo.GetDailyConversationCounts(ctx, accountID)
	if err != nil {
		return nil, err
	}

	slaMetrics, err := uc.analyticsRepo.GetSLAMetrics(ctx, accountID)
	if err != nil {
		return nil, err
	}

	agentPerf, err := uc.analyticsRepo.GetAgentPerformance(ctx, accountID)
	if err != nil {
		return nil, err
	}

	// AI performance: main retention driver. This is what the customer
	// looks at every month to decide whether the subscription is worth it
	// ("o bot resolveu 78% sozinho"). Don't fail the whole endpoint if
	// this aggregate blows up — degrade gracefully.
	aiPerf, err := uc.analyticsRepo.GetAIPerformance(ctx, accountID)
	if err != nil {
		aiPerf = nil
	}

	// CSAT: optional (repo may not be wired in older deployments or tests).
	// Same soft-fail treatment as AI — never take down /analytics because
	// an aggregate is misbehaving.
	var csat *domain.CSATStats
	if uc.csatRepo != nil {
		if stats, err := uc.csatRepo.Stats(ctx, accountID); err == nil {
			csat = stats
		}
	}

	// Get token usage info
	var tokensUsed int64
	var tokensLimit int64

	usage, err := uc.usageRepo.GetByAccountID(ctx, accountID)
	if err == nil && usage != nil {
		if usage.AITokensUsed != nil {
			tokensUsed = *usage.AITokensUsed
		}
		addonTokens := int64(0)
		if usage.AddonTokens != nil {
			addonTokens = *usage.AddonTokens
		}

		// Get plan limits
		account, err := uc.accountRepo.GetByID(ctx, accountID)
		if err == nil && account != nil && account.PlanID != nil {
			plan, err := uc.planRepo.GetByID(ctx, *account.PlanID)
			if err == nil && plan != nil {
				tokensLimit = plan.MaxAITokensPerMonth + addonTokens
			}
		}
	}

	return &AnalyticsResponse{
		Conversations:    *convStats,
		Messages:         *msgStats,
		DailyActivity:    dailyCounts,
		SLA:              slaMetrics,
		AgentPerformance: agentPerf,
		AI:               aiPerf,
		CSAT:             csat,
		TokensUsed:       tokensUsed,
		TokensLimit:      tokensLimit,
	}, nil
}
