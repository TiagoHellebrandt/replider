package services

import (
	"testing"
	"time"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestUsageGuard_Check_Conversations_AtLimit(t *testing.T) {
	g := &UsageGuard{}
	count := int32(1000)
	usage := &domain.AccountUsage{ConversationsCount: &count}
	plan := &domain.Plan{MaxConversationsPerMonth: 1000}

	err := g.Check(nil, plan, usage, "conversations")
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)
}

func TestUsageGuard_Check_Conversations_UnderLimit(t *testing.T) {
	g := &UsageGuard{}
	count := int32(999)
	usage := &domain.AccountUsage{ConversationsCount: &count}
	plan := &domain.Plan{MaxConversationsPerMonth: 1000}

	err := g.Check(nil, plan, usage, "conversations")
	assert.NoError(t, err)
}

func TestUsageGuard_Check_Tokens_AtHardLimit(t *testing.T) {
	g := &UsageGuard{}
	// Plan: 100K tokens, hard limit = 110K (10% grace)
	tokensUsed := int64(110000)
	usage := &domain.AccountUsage{AITokensUsed: &tokensUsed}
	plan := &domain.Plan{MaxAITokensPerMonth: 100000}

	err := g.Check(nil, plan, usage, "tokens")
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)
}

func TestUsageGuard_Check_Tokens_WithinGrace(t *testing.T) {
	g := &UsageGuard{}
	// Just under hard limit
	tokensUsed := int64(109999)
	usage := &domain.AccountUsage{AITokensUsed: &tokensUsed}
	plan := &domain.Plan{MaxAITokensPerMonth: 100000}

	err := g.Check(nil, plan, usage, "tokens")
	assert.NoError(t, err)
}

func TestUsageGuard_Check_Tokens_AddonCoverOverage(t *testing.T) {
	g := &UsageGuard{}
	// Plan: 100K, addon: 50K → effective 150K, hard limit 165K
	tokensUsed := int64(120000)
	addonTokens := int64(50000)
	usage := &domain.AccountUsage{AITokensUsed: &tokensUsed, AddonTokens: &addonTokens}
	plan := &domain.Plan{MaxAITokensPerMonth: 100000}

	err := g.Check(nil, plan, usage, "tokens")
	assert.NoError(t, err)
}

func TestUsageGuard_Check_Tokens_AddonExhausted(t *testing.T) {
	g := &UsageGuard{}
	// Plan: 100K, addon: 10K → effective 110K, hard limit 121K
	tokensUsed := int64(121000)
	addonTokens := int64(10000)
	usage := &domain.AccountUsage{AITokensUsed: &tokensUsed, AddonTokens: &addonTokens}
	plan := &domain.Plan{MaxAITokensPerMonth: 100000}

	err := g.Check(nil, plan, usage, "tokens")
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)
}

func TestUsageGuard_Check_Trial(t *testing.T) {
	g := &UsageGuard{}
	trial := domain.TrialPlan()

	// Trial: 10K tokens, hard limit = 11K
	tokensUsed := int64(11000)
	usage := &domain.AccountUsage{AITokensUsed: &tokensUsed}

	err := g.Check(nil, trial, usage, "tokens")
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)

	// Trial: 100 conversations
	convCount := int32(100)
	usage2 := &domain.AccountUsage{ConversationsCount: &convCount}
	err = g.Check(nil, trial, usage2, "conversations")
	assert.Equal(t, domain.ErrUsageLimitExceeded, err)
}

func TestUsageGuard_CheckAccountActive_PastDueGraceExpired(t *testing.T) {
	g := &UsageGuard{}
	eightDaysAgo := time.Now().AddDate(0, 0, -8)
	account := &domain.Account{
		SubscriptionStatus: domain.SubscriptionStatusPastDue,
		PastDueAt:          &eightDaysAgo,
	}
	err := g.CheckAccountActive(account)
	assert.Equal(t, domain.ErrAccountSuspended, err)
}

func TestUsageGuard_CheckAccountActive_PastDueWithinGrace(t *testing.T) {
	g := &UsageGuard{}
	threeDaysAgo := time.Now().AddDate(0, 0, -3)
	account := &domain.Account{
		SubscriptionStatus: domain.SubscriptionStatusPastDue,
		PastDueAt:          &threeDaysAgo,
	}
	err := g.CheckAccountActive(account)
	assert.NoError(t, err)
}

func TestUsageGuard_CheckAccountActive_ActiveAccount(t *testing.T) {
	g := &UsageGuard{}
	account := &domain.Account{SubscriptionStatus: domain.SubscriptionStatusActive}
	err := g.CheckAccountActive(account)
	assert.NoError(t, err)
}

func TestUsageGuard_Check_NilFields(t *testing.T) {
	g := &UsageGuard{}
	// All nil fields should be treated as zero → under limit
	usage := &domain.AccountUsage{}
	plan := &domain.Plan{MaxAITokensPerMonth: 100000, MaxConversationsPerMonth: 1000}

	assert.NoError(t, g.Check(nil, plan, usage, "conversations"))
	assert.NoError(t, g.Check(nil, plan, usage, "tokens"))
}

// Contract plans (Plan.StripePriceID == nil) opt out of the platform's
// token meter — they bring their own provider key and are billed
// off-platform. The guard must short-circuit instead of treating the
// 0 cap as "fully consumed" the moment any tokens are reported.
func TestUsageGuard_Check_Tokens_ContractPlanSkipsLimit(t *testing.T) {
	g := &UsageGuard{}
	plan := &domain.Plan{Name: "Enterprise", MaxAITokensPerMonth: 0, StripePriceID: nil}
	tokensUsed := int64(5_000_000)
	usage := &domain.AccountUsage{AITokensUsed: &tokensUsed}
	account := &domain.Account{ID: "acc_1"}

	err := g.Check(account, plan, usage, "tokens")
	assert.NoError(t, err)
}

// Um contrato sem número negociado não é ilimitado: responde pelo teto de
// plataforma. Abaixo dele passa — ninguém é travado por uma coluna em branco —
// e acima dele barra, porque disco e custo existem, e a tela do cliente não
// pode prometer o contrário.
func TestUsageGuard_Check_Conversations_ContractPlanUsesTheCeiling(t *testing.T) {
	g := &UsageGuard{}
	plan := &domain.Plan{Name: "Enterprise", MaxConversationsPerMonth: 0, StripePriceID: nil}
	account := &domain.Account{ID: "acc_1"}

	under := int32(domain.CeilingConversationsPerMonth - 1)
	assert.NoError(t, g.Check(account, plan,
		&domain.AccountUsage{ConversationsCount: &under}, "conversations"))

	over := int32(domain.CeilingConversationsPerMonth + 1)
	assert.Error(t, g.Check(account, plan,
		&domain.AccountUsage{ConversationsCount: &over}, "conversations"),
		"passar do teto tem que barrar — senão o teto é decorativo")
}

func TestUsageGuard_Check_Conversations_OverrideEnforced(t *testing.T) {
	g := &UsageGuard{}
	plan := &domain.Plan{Name: "Enterprise", MaxConversationsPerMonth: 0, StripePriceID: nil}
	override := 100
	count := int32(100)
	usage := &domain.AccountUsage{ConversationsCount: &count}
	account := &domain.Account{ID: "acc_1", MaxConversationsPerMonthOverride: &override}

	assert.Equal(t, domain.ErrUsageLimitExceeded, g.Check(account, plan, usage, "conversations"))
}

// starterOveragePlan mirrors the Starter tier: R$399, 700-conversation
// franchise, 70¢/conversation overage, 2× cap (→ 79800¢ ceiling).
func starterOveragePlan() *domain.Plan {
	return &domain.Plan{
		Name:                     "Starter",
		Price:                    39900,
		MaxConversationsPerMonth: 700,
		OverageConversationCents: 70,
		OverageCapMultiplier:     2.0,
	}
}

func TestUsageGuard_Check_Conversations_OverageAllowedUnderCap(t *testing.T) {
	g := &UsageGuard{}
	// 750 conversations → 50 over → 3500¢, well under the 79800¢ cap.
	count := int32(750)
	usage := &domain.AccountUsage{ConversationsCount: &count}

	assert.NoError(t, g.Check(nil, starterOveragePlan(), usage, "conversations"))
}

func TestUsageGuard_Check_Conversations_BlockedAtOverageCap(t *testing.T) {
	g := &UsageGuard{}
	// 1840 conversations → 1140 over → 1140 × 70¢ = 79800¢ == cap.
	count := int32(1840)
	usage := &domain.AccountUsage{ConversationsCount: &count}

	assert.Equal(t, domain.ErrUsageLimitExceeded, g.Check(nil, starterOveragePlan(), usage, "conversations"))
}

func TestUsageGuard_Check_Conversations_NoOverageRateHardStops(t *testing.T) {
	g := &UsageGuard{}
	// Plan without an overage rate (Trial-like) must hard-stop at the
	// franchise even though conversations could otherwise overflow.
	count := int32(700)
	usage := &domain.AccountUsage{ConversationsCount: &count}
	plan := &domain.Plan{Name: "Solo", MaxConversationsPerMonth: 700, OverageConversationCents: 0}

	assert.Equal(t, domain.ErrUsageLimitExceeded, g.Check(nil, plan, usage, "conversations"))
}
