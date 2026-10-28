package usecases

import (
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func healthUC() *GetAccountHealthReportUseCase {
	return &GetAccountHealthReportUseCase{}
}

func issueCodes(report *AccountHealthReport) map[string]HealthIssue {
	codes := map[string]HealthIssue{}
	for _, issue := range report.Issues {
		codes[issue.Code] = issue
	}
	return codes
}

func boolp(b bool) *bool { return &b }
func intp(n int) *int    { return &n }

// A fresh SDR account: nothing imported, no e-mail, no territory. The
// report must say what is missing for the loop to run — and must NOT talk
// about agents or knowledge bases, which this product no longer sells.
func TestHealthReport_FreshSDRAccountFiresLoopIssues(t *testing.T) {
	report := &AccountHealthReport{Issues: []HealthIssue{}}
	report.Setup = HealthSetup{
		HasTerritory:    boolp(false),
		HasSalesHistory: boolp(false),
	}

	healthUC().detectIssues(report, &DashboardBootstrapResponse{})

	codes := issueCodes(report)
	assert.Equal(t, "high", codes["NO_TERRITORY"].Severity)
	assert.Equal(t, "high", codes["NO_PRODUCTS"].Severity)
	assert.Equal(t, "medium", codes["NO_CUSTOMERS"].Severity)
	assert.Equal(t, "high", codes["NO_EMAIL_CHANNEL"].Severity)
	assert.Equal(t, "low", codes["NO_SALES_HISTORY"].Severity)
	assert.NotContains(t, codes, "QUEUE_IDLE", "an empty queue is not an idle queue")

	for _, legacy := range []string{"NO_AGENT", "KNOWLEDGE_EMPTY", "NO_CHANNEL", "WEAK_PROMPT", "ALL_AGENTS_PAUSED"} {
		assert.NotContains(t, codes, legacy, "v1 issue %s must not fire on the SDR report", legacy)
	}
	// 3 high + 1 medium + 1 low → 100 - 75 - 10 - 5 = 10
	assert.Equal(t, 10, computeHealthScore(report))
}

// The setup fixes are screens in the app. A fix_tool there would send the
// model chasing a tool that does not exist; the suggestion says where to go.
func TestHealthReport_SetupIssuesPointAtTheAppNotATool(t *testing.T) {
	report := &AccountHealthReport{Issues: []HealthIssue{}}
	report.Setup = HealthSetup{HasTerritory: boolp(false)}

	healthUC().detectIssues(report, &DashboardBootstrapResponse{})

	codes := issueCodes(report)
	for _, code := range []string{"NO_TERRITORY", "NO_PRODUCTS", "NO_CUSTOMERS", "NO_EMAIL_CHANNEL"} {
		assert.Empty(t, codes[code].FixTool, "%s has no MCP fix", code)
		assert.Contains(t, codes[code].Suggestion, "app", "%s must tell the user where the fix lives", code)
	}
	assert.Contains(t, codes["NO_TERRITORY"].Suggestion, "PATCH /accounts")
	assert.Contains(t, codes["NO_PRODUCTS"].Suggestion, "Produtos")
}

// An unwired probe leaves its section unknown, and unknown never fires:
// "defina o território" to an account that has one is worse than silence.
func TestHealthReport_UnknownProbesStaySilent(t *testing.T) {
	report := &AccountHealthReport{Issues: []HealthIssue{}}
	report.Setup = HealthSetup{
		ProductsAdded: true, CustomersAdded: true, HasEmailChannel: true,
		// HasTerritory, HasSalesHistory, OldestPendingDraftDays: nil
	}

	healthUC().detectIssues(report, &DashboardBootstrapResponse{})

	assert.Empty(t, report.Issues)
	assert.Equal(t, 100, computeHealthScore(report))
}

func TestHealthReport_QueueIdleAfterThreeDays(t *testing.T) {
	ready := HealthSetup{
		HasTerritory: boolp(true), ProductsAdded: true, CustomersAdded: true,
		HasEmailChannel: true, HasSalesHistory: boolp(true),
	}

	fresh := ready
	fresh.OldestPendingDraftDays = intp(2)
	report := &AccountHealthReport{Issues: []HealthIssue{}, Setup: fresh}
	healthUC().detectIssues(report, &DashboardBootstrapResponse{})
	assert.NotContains(t, issueCodes(report), "QUEUE_IDLE")

	stale := ready
	stale.OldestPendingDraftDays = intp(4)
	report = &AccountHealthReport{Issues: []HealthIssue{}, Setup: stale}
	healthUC().detectIssues(report, &DashboardBootstrapResponse{})
	idle, ok := issueCodes(report)["QUEUE_IDLE"]
	assert.True(t, ok)
	assert.Equal(t, "get_pending_actions", idle.FixTool)
}

func TestHealthReport_EmailChannelIsWhatCounts(t *testing.T) {
	uc := healthUC()
	report := &AccountHealthReport{Issues: []HealthIssue{}}
	boot := &DashboardBootstrapResponse{
		Me: &GetMeResponse{},
		Channels: []*domain.Channel{
			{Type: "whatsapp"},
			{Type: "email"},
		},
	}

	uc.fillSetup(t.Context(), report, boot, "acc1")

	assert.True(t, report.Setup.HasEmailChannel)
	assert.Equal(t, 2, report.Setup.ChannelCount)
	assert.ElementsMatch(t, []string{"whatsapp", "email"}, report.Setup.ChannelTypes)

	whatsappOnly := &DashboardBootstrapResponse{Me: &GetMeResponse{}, Channels: []*domain.Channel{{Type: "whatsapp"}}}
	report = &AccountHealthReport{Issues: []HealthIssue{}}
	uc.fillSetup(t.Context(), report, whatsappOnly, "acc1")
	assert.False(t, report.Setup.HasEmailChannel, "a WhatsApp number is not the e-mail the offers go out on")
}

func TestHealthReport_OnboardingMissingStepsAreTheSalesMilestones(t *testing.T) {
	report := &AccountHealthReport{Issues: []HealthIssue{}}
	boot := &DashboardBootstrapResponse{Me: &GetMeResponse{
		OnboardingProgress: &domain.OnboardingProgress{ProductsAdded: true, ChannelConnected: true},
	}}

	healthUC().fillSetup(t.Context(), report, boot, "acc1")

	assert.True(t, report.Setup.ProductsAdded)
	assert.False(t, report.Setup.CustomersAdded)
	assert.Equal(t, []string{"importar_clientes", "primeira_oferta", "primeira_resposta", "convidar_equipe"},
		report.Setup.OnboardingMissing)
	for _, v1 := range []string{"criar_agente", "adicionar_conhecimento", "testar_agente"} {
		assert.NotContains(t, report.Setup.OnboardingMissing, v1)
	}
}

// The plan issues survive the pivot untouched: a capped account cannot
// draft or send whatever product it thinks it is using.
func TestHealthReport_UsageIssuesStay(t *testing.T) {
	ready := HealthSetup{
		HasTerritory: boolp(true), ProductsAdded: true, CustomersAdded: true,
		HasEmailChannel: true, HasSalesHistory: boolp(true),
	}
	report := &AccountHealthReport{Issues: []HealthIssue{}, Setup: ready}
	report.Usage = &HealthUsage{TokensUsagePercent: 85}
	healthUC().detectIssues(report, &DashboardBootstrapResponse{})
	codes := issueCodes(report)
	assert.Contains(t, codes, "USAGE_NEAR_LIMIT")
	assert.NotContains(t, codes, "USAGE_CAP_REACHED")

	report = &AccountHealthReport{Issues: []HealthIssue{}, Setup: ready}
	report.Usage = &HealthUsage{CapReached: true}
	healthUC().detectIssues(report, &DashboardBootstrapResponse{})
	assert.Equal(t, "create_checkout_link", issueCodes(report)["USAGE_CAP_REACHED"].FixTool)
}

// ── v1 checks, only behind the legacy flag ──────────────────────────────

func TestHealthReport_LegacyIssuesOnlyWithLegacyTools(t *testing.T) {
	shortPrompt := "Você é um atendente."
	boot := &DashboardBootstrapResponse{Agents: []*domain.Agent{
		{ID: "a1", Name: "Fraco", IsActive: true, SystemPrompt: &shortPrompt},
	}}
	setup := HealthSetup{
		HasTerritory: boolp(true), ProductsAdded: true, CustomersAdded: true,
		HasEmailChannel: true, HasSalesHistory: boolp(true),
		AgentCount: 1, ActiveAgentCount: 1, ChannelCount: 1,
	}

	report := &AccountHealthReport{Issues: []HealthIssue{}, Setup: setup}
	healthUC().detectIssues(report, boot)
	assert.Empty(t, report.Issues, "SDR report ignores agents and knowledge")

	legacy := healthUC()
	legacy.SetLegacyIssues(true)
	report = &AccountHealthReport{Issues: []HealthIssue{}, Setup: setup}
	report.Performance = &HealthPerformance{TotalConversations: 50, CSATAverage: 3.2, CSATResponses: 10, AvgFirstResponseSeconds: 600}
	legacy.detectIssues(report, boot)
	codes := issueCodes(report)
	assert.Equal(t, "update_agent", codes["WEAK_PROMPT"].FixTool)
	assert.Equal(t, "a1", codes["WEAK_PROMPT"].AgentID)
	assert.Contains(t, codes, "KNOWLEDGE_EMPTY")
	assert.Contains(t, codes, "LOW_CSAT")
	assert.Contains(t, codes, "SLOW_FIRST_RESPONSE")
}

func TestHealthReport_HealthyAccountScores100(t *testing.T) {
	report := &AccountHealthReport{Issues: []HealthIssue{}}
	assert.Equal(t, 100, computeHealthScore(report))
}

func int64p(n int64) *int64 { return &n }

// The report used to give advice nobody could take.
//
// It told an operator holding 50 finished offers to "aprove ou descarte cada
// um" while none of the recipients had an address any connected channel could
// reach — a queue that is neither approvable nor safely discardable, because
// the offers are good and the missing piece is a buyer's e-mail nobody looked
// for. Forty days of an untouched backlog is what that advice produces.
func TestHealthReport_AQueueWithNoAddressesSaysTheRealProblem(t *testing.T) {
	stuck := HealthSetup{
		HasTerritory: boolp(true), ProductsAdded: true, CustomersAdded: true,
		HasEmailChannel: true, HasSalesHistory: boolp(true),
		PendingDrafts: int64p(50), OldestPendingDraftDays: intp(19),
		UndeliverableDrafts: int64p(37),
	}
	report := &AccountHealthReport{Issues: []HealthIssue{}, Setup: stuck}
	healthUC().detectIssues(report, &DashboardBootstrapResponse{})

	codes := issueCodes(report)
	blocked, ok := codes["QUEUE_UNDELIVERABLE"]
	assert.True(t, ok, "a queue that cannot be posted must say so")
	assert.Equal(t, "high", blocked.Severity)
	assert.Contains(t, blocked.Suggestion, "37 de 50")
	assert.Contains(t, blocked.Suggestion, "contato de compras")

	// And the idle-queue line stops asking for the impossible.
	idle := codes["QUEUE_IDLE"]
	assert.Contains(t, idle.Suggestion, "QUEUE_UNDELIVERABLE")
	assert.NotContains(t, idle.Suggestion, "aprove ou descarte")
}

// With every recipient reachable, the old advice is the right advice: the
// queue really is just waiting on a person.
func TestHealthReport_AReachableQueueKeepsTheOrdinaryNudge(t *testing.T) {
	fine := HealthSetup{
		HasTerritory: boolp(true), ProductsAdded: true, CustomersAdded: true,
		HasEmailChannel: true, HasSalesHistory: boolp(true),
		PendingDrafts: int64p(50), OldestPendingDraftDays: intp(19),
		UndeliverableDrafts: int64p(0),
	}
	report := &AccountHealthReport{Issues: []HealthIssue{}, Setup: fine}
	healthUC().detectIssues(report, &DashboardBootstrapResponse{})

	codes := issueCodes(report)
	assert.NotContains(t, codes, "QUEUE_UNDELIVERABLE")
	assert.Contains(t, codes["QUEUE_IDLE"].Suggestion, "aprove ou descarte")
}

// An unwired or failed probe leaves the count absent, and absent must never
// be read as zero — that would silently promise the queue is deliverable.
func TestHealthReport_AnUnknownReachCountFiresNothing(t *testing.T) {
	unknown := HealthSetup{
		HasTerritory: boolp(true), ProductsAdded: true, CustomersAdded: true,
		HasEmailChannel: true, HasSalesHistory: boolp(true),
		PendingDrafts: int64p(50), OldestPendingDraftDays: intp(19),
	}
	report := &AccountHealthReport{Issues: []HealthIssue{}, Setup: unknown}
	healthUC().detectIssues(report, &DashboardBootstrapResponse{})

	assert.NotContains(t, issueCodes(report), "QUEUE_UNDELIVERABLE")
}
