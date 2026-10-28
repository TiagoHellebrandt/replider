package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/replider/api/internal/core/domain"
)

// GetAccountHealthReportUseCase is the MCP session opener: one call that
// gives the model the full picture of an SDR account — whether the loop
// (territory → catalogue → carteira → e-mail → drafts → approval) can run,
// usage vs plan limits — plus a deterministic issues[] list with fix_tool
// pointers where a tool exists and plain instructions where the fix is a
// screen in the app.
//
// NOTE: distinct from AccountHealthUseCase (US-177), which is the
// owner-panel cross-account retention sweep.
//
// Thin orchestration over existing aggregates (bootstrap + analytics
// overview) plus three SDR probes wired by the MCP adapter; like the
// bootstrap, secondary reads degrade to warnings instead of failing the
// whole report.
type GetAccountHealthReportUseCase struct {
	bootstrapUC *GetDashboardBootstrapUseCase
	overviewUC  *GetAnalyticsOverviewUseCase
	knowledgeUC *KnowledgeUseCase

	// SDR probes (SetSDRProbes). Nil probes leave the matching section
	// unknown — and unknown never fires an issue, because "importe o
	// catálogo" to an account that has one is worse than silence.
	accounts  domain.AccountRepository
	leads     *LeadUseCase
	portfolio domain.PortfolioRepository

	// SetQueueReach: how much of the queue has nowhere to be posted, and the
	// channels that decide it. Both nil leaves the question unasked — the
	// report says nothing rather than guessing.
	queueReach     queueReachProbe
	healthChannels healthChannelLister

	// legacyIssues re-enables the v1 checks (agents, knowledge, CS
	// metrics). Their fix_tools only exist when the MCP server registers
	// the legacy surface, so the two flags travel together.
	legacyIssues bool
	now          func() time.Time
}

func NewGetAccountHealthReportUseCase(
	bootstrapUC *GetDashboardBootstrapUseCase,
	overviewUC *GetAnalyticsOverviewUseCase,
	knowledgeUC *KnowledgeUseCase,
) *GetAccountHealthReportUseCase {
	return &GetAccountHealthReportUseCase{
		bootstrapUC: bootstrapUC,
		overviewUC:  overviewUC,
		knowledgeUC: knowledgeUC,
		now:         time.Now,
	}
}

// SetSDRProbes wires the reads the SDR issues need: the account (territory),
// the approval queue (idle drafts) and the carteira (sales history). Any nil
// argument leaves its section unknown.
func (uc *GetAccountHealthReportUseCase) SetSDRProbes(accounts domain.AccountRepository, leads *LeadUseCase, portfolio domain.PortfolioRepository) {
	uc.accounts = accounts
	uc.leads = leads
	uc.portfolio = portfolio
}

// queueReachProbe counts the queued drafts with no address of each kind.
type queueReachProbe interface {
	CountUndeliverablePending(ctx context.Context, accountID string) (domain.UndeliverableQueueCounts, error)
}

// healthChannelLister is the one channel fact this report needs.
type healthChannelLister interface {
	ListByAccountID(ctx context.Context, accountID string) ([]*domain.Channel, error)
}

// SetQueueReach wires the "can this queue actually be posted" probe.
//
// It exists because the report's advice was impossible to follow. It told an
// operator with 50 drafts to "aprove ou descarte cada um" while none of the
// recipients had an address any of their channels could reach — a queue that
// is neither approvable nor safely discardable, because the offers are good
// and the missing piece is a contact nobody looked for.
func (uc *GetAccountHealthReportUseCase) SetQueueReach(p queueReachProbe, channels healthChannelLister) {
	uc.queueReach = p
	uc.healthChannels = channels
}

// SetLegacyIssues toggles the v1 customer-service checks. Keep in step with
// the MCP server's legacy-tools flag: an issue whose fix_tool is not listed
// sends the model chasing a tool that does not exist.
func (uc *GetAccountHealthReportUseCase) SetLegacyIssues(on bool) { uc.legacyIssues = on }

type HealthIssue struct {
	Severity   string `json:"severity"` // high | medium | low
	Code       string `json:"code"`
	Title      string `json:"title"`
	Suggestion string `json:"suggestion"`
	// FixTool names the MCP tool that resolves this issue. Empty when the
	// fix lives in the app (a screen, an import) — the suggestion says where.
	FixTool string `json:"fix_tool,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
}

type HealthSetup struct {
	OnboardingComplete bool     `json:"onboarding_complete"`
	OnboardingMissing  []string `json:"onboarding_missing_steps,omitempty"`

	// The SDR loop's preconditions. Pointers where the probe may be unwired:
	// nil means "not checked", which is different from false.
	HasTerritory    *bool `json:"has_territory,omitempty"`
	ProductsAdded   bool  `json:"products_added"`
	CustomersAdded  bool  `json:"customers_added"`
	HasEmailChannel bool  `json:"has_email_channel"`
	HasSalesHistory *bool `json:"has_sales_history,omitempty"`
	// PendingDrafts is the approval backlog visible to the caller;
	// OldestPendingDraftDays is how long its oldest item has waited.
	PendingDrafts          *int64 `json:"pending_drafts,omitempty"`
	OldestPendingDraftDays *int   `json:"oldest_pending_draft_days,omitempty"`
	// UndeliverableDrafts is how many of those have no address this account's
	// channels can reach. Absent when the probe is unwired or failed — an
	// unknown count must not be read as zero.
	UndeliverableDrafts *int64 `json:"undeliverable_drafts,omitempty"`

	ChannelCount int      `json:"channel_count"`
	ChannelTypes []string `json:"channel_types"`

	// v1 fields, still populated (they cost nothing) but only judged when
	// legacy issues are on.
	AgentCount           int `json:"agent_count"`
	ActiveAgentCount     int `json:"active_agent_count"`
	KnowledgeSourceCount int `json:"knowledge_source_count"`
}

type HealthUsage struct {
	PlanName                  string  `json:"plan_name"`
	IsTrialing                bool    `json:"is_trialing"`
	ConversationsUsed         int32   `json:"conversations_used"`
	ConversationsLimit        int     `json:"conversations_limit"`
	TokensUsagePercent        float64 `json:"tokens_usage_percent"`
	ConversationsUsagePercent float64 `json:"conversations_usage_percent"`
	CapReached                bool    `json:"cap_reached"`
}

type HealthPerformance struct {
	PeriodDays              int     `json:"period_days"`
	TotalConversations      int     `json:"total_conversations"`
	ResolvedByAIPct         float64 `json:"resolved_by_ai_pct"`
	AvgFirstResponseSeconds int     `json:"avg_first_response_seconds"`
	CSATAverage             float64 `json:"csat_average"`
	CSATResponses           int     `json:"csat_responses"`
}

type AccountHealthReport struct {
	Score       int                `json:"score"` // 0-100
	AccountName string             `json:"account_name"`
	Timezone    string             `json:"timezone,omitempty"`
	Setup       HealthSetup        `json:"setup"`
	Usage       *HealthUsage       `json:"usage,omitempty"`
	Performance *HealthPerformance `json:"performance,omitempty"`
	Issues      []HealthIssue      `json:"issues"`
	Warnings    []string           `json:"warnings,omitempty"`
}

// Deterministic gap-detection thresholds. The model turns these into
// conversation; keeping detection in Go keeps the report reproducible.
const (
	weakPromptMinChars      = 200
	nearLimitPercent        = 80
	lowCSATThreshold        = 4.0
	slowFirstResponseSecs   = 300
	healthAnalyticsWindow   = 30
	minConversationsForPerf = 5
	// queueIdleDays is how long a draft may wait for a human before the
	// queue counts as abandoned. Three days: a weekend plus one.
	queueIdleDays = 3
)

// Execute builds the report for the calling actor. The actor matters: the
// approval-queue probe is scoped like the screen (a vendedor's own drafts,
// a gestor's whole floor), so QUEUE_IDLE means "YOUR queue is idle".
func (uc *GetAccountHealthReportUseCase) Execute(ctx context.Context, actor domain.Actor) (*AccountHealthReport, error) {
	boot, err := uc.bootstrapUC.Execute(ctx, actor.UserID, actor.AccountID)
	if err != nil {
		return nil, err
	}

	report := &AccountHealthReport{
		AccountName: boot.Me.Account.Name,
		Issues:      []HealthIssue{},
		Warnings:    boot.Warnings,
	}

	uc.fillSetup(ctx, report, boot, actor.AccountID)
	uc.fillSDR(ctx, report, actor)
	uc.fillUsage(report, boot)
	if uc.legacyIssues {
		uc.fillPerformance(ctx, report, actor.AccountID)
	}
	uc.detectIssues(report, boot)
	report.Score = computeHealthScore(report)
	return report, nil
}

func (uc *GetAccountHealthReportUseCase) fillSetup(ctx context.Context, report *AccountHealthReport, boot *DashboardBootstrapResponse, accountID string) {
	setup := &report.Setup
	setup.AgentCount = len(boot.Agents)
	for _, a := range boot.Agents {
		if a.IsActive {
			setup.ActiveAgentCount++
		}
	}
	setup.ChannelTypes = []string{}
	seenTypes := map[string]bool{}
	for _, ch := range boot.Channels {
		setup.ChannelCount++
		if ch.Type == "email" {
			setup.HasEmailChannel = true
		}
		if !seenTypes[ch.Type] {
			seenTypes[ch.Type] = true
			setup.ChannelTypes = append(setup.ChannelTypes, ch.Type)
		}
	}

	if op := boot.Me.OnboardingProgress; op != nil {
		setup.OnboardingComplete = op.CompletionPercentage >= 100
		setup.ProductsAdded = op.ProductsAdded
		setup.CustomersAdded = op.CustomersAdded
		// The six SALES milestones, in the order the checklist card shows them.
		if !op.ChannelConnected {
			setup.OnboardingMissing = append(setup.OnboardingMissing, "conectar_email")
		}
		if !op.ProductsAdded {
			setup.OnboardingMissing = append(setup.OnboardingMissing, "importar_produtos")
		}
		if !op.CustomersAdded {
			setup.OnboardingMissing = append(setup.OnboardingMissing, "importar_clientes")
		}
		if !op.ProposalMade {
			setup.OnboardingMissing = append(setup.OnboardingMissing, "primeira_oferta")
		}
		if !op.FirstInboundReceived {
			setup.OnboardingMissing = append(setup.OnboardingMissing, "primeira_resposta")
		}
		if !op.TeamInvited {
			setup.OnboardingMissing = append(setup.OnboardingMissing, "convidar_equipe")
		}
	}

	if !uc.legacyIssues || uc.knowledgeUC == nil {
		return
	}
	// Knowledge source count across the account's agents (best-effort).
	for _, a := range boot.Agents {
		sources, err := uc.knowledgeUC.ListSources(ctx, accountID, a.ID)
		if err != nil {
			slog.WarnContext(ctx, "health report: knowledge listing failed", "agent_id", a.ID, "error", err)
			report.Warnings = append(report.Warnings, "knowledge")
			break
		}
		setup.KnowledgeSourceCount += len(sources)
	}
}

// fillSDR runs the three probes the bootstrap aggregate does not carry.
// Each degrades to a warning and leaves its field nil: an issue is never
// fired on a read that failed.
func (uc *GetAccountHealthReportUseCase) fillSDR(ctx context.Context, report *AccountHealthReport, actor domain.Actor) {
	setup := &report.Setup

	if uc.accounts != nil {
		account, err := uc.accounts.GetByID(ctx, actor.AccountID)
		switch {
		case err != nil || account == nil:
			slog.WarnContext(ctx, "health report: account read failed", "account_id", actor.AccountID, "error", err)
			report.Warnings = append(report.Warnings, "account")
		default:
			has := account.Territory != nil && account.Territory.UF != ""
			setup.HasTerritory = &has
			report.Timezone = account.Timezone
		}
	}

	if uc.leads != nil {
		page, err := uc.leads.ApprovalQueue(ctx, actor, ApprovalQueueOptions{NewestFirst: false}, 1, 0)
		switch {
		case err == domain.ErrForbidden:
			// An attendant has no queue; not a warning, not an issue.
		case err != nil:
			slog.WarnContext(ctx, "health report: approval queue read failed", "account_id", actor.AccountID, "error", err)
			report.Warnings = append(report.Warnings, "approval_queue")
		default:
			total := page.Total
			if total < int64(len(page.Items)) {
				total = int64(len(page.Items))
			}
			setup.PendingDrafts = &total
			if len(page.Items) > 0 {
				days := int(uc.now().Sub(page.Items[0].UpdatedAt).Hours() / 24)
				setup.OldestPendingDraftDays = &days
			}
			uc.fillQueueReach(ctx, setup, actor.AccountID)
		}
	}

	if uc.portfolio != nil {
		customers, err := uc.portfolio.List(ctx, actor.AccountID, 1, 0)
		if err != nil {
			slog.WarnContext(ctx, "health report: portfolio read failed", "account_id", actor.AccountID, "error", err)
			report.Warnings = append(report.Warnings, "portfolio")
		} else {
			has := len(customers) > 0
			setup.HasSalesHistory = &has
		}
	}
}

// fillQueueReach crosses the queue's missing addresses with the channels the
// account has. Best-effort on both reads: an unknown count stays absent, and
// absent never fires an issue.
func (uc *GetAccountHealthReportUseCase) fillQueueReach(ctx context.Context, setup *HealthSetup, accountID string) {
	if uc.queueReach == nil || uc.healthChannels == nil {
		return
	}
	counts, err := uc.queueReach.CountUndeliverablePending(ctx, accountID)
	if err != nil {
		slog.WarnContext(ctx, "health report: undeliverable queue count failed",
			"account_id", accountID, "error", err)
		return
	}
	channels, err := uc.healthChannels.ListByAccountID(ctx, accountID)
	if err != nil {
		slog.WarnContext(ctx, "health report: channel read failed",
			"account_id", accountID, "error", err)
		return
	}
	stuck := counts.UndeliverableWith(channels)
	setup.UndeliverableDrafts = &stuck
}

func (uc *GetAccountHealthReportUseCase) fillUsage(report *AccountHealthReport, boot *DashboardBootstrapResponse) {
	if boot.Usage == nil {
		return
	}
	report.Usage = &HealthUsage{
		PlanName:                  boot.Usage.PlanName,
		IsTrialing:                boot.Usage.IsTrialing,
		ConversationsUsed:         boot.Usage.ConversationsCount,
		ConversationsLimit:        boot.Usage.MaxConversationsPerMonth,
		TokensUsagePercent:        boot.Usage.TokensUsagePercent,
		ConversationsUsagePercent: boot.Usage.ConversationsUsagePercent,
		CapReached:                boot.Usage.CapReached,
	}
}

func (uc *GetAccountHealthReportUseCase) fillPerformance(ctx context.Context, report *AccountHealthReport, accountID string) {
	if uc.overviewUC == nil {
		return
	}
	overview, err := uc.overviewUC.Execute(ctx, accountID, healthAnalyticsWindow)
	if err != nil {
		slog.WarnContext(ctx, "health report: analytics overview failed", "account_id", accountID, "error", err)
		report.Warnings = append(report.Warnings, "analytics")
		return
	}
	report.Performance = &HealthPerformance{
		PeriodDays:              overview.PeriodDays,
		TotalConversations:      overview.TotalConversations,
		ResolvedByAIPct:         overview.ResolvedByAIPct,
		AvgFirstResponseSeconds: overview.AvgFirstResponseSeconds,
		CSATAverage:             overview.CSAT.Average,
		CSATResponses:           overview.CSAT.Responses,
	}
}

func (uc *GetAccountHealthReportUseCase) detectIssues(report *AccountHealthReport, boot *DashboardBootstrapResponse) {
	add := func(severity, code, title, suggestion, fixTool, agentID string) {
		report.Issues = append(report.Issues, HealthIssue{
			Severity: severity, Code: code, Title: title,
			Suggestion: suggestion, FixTool: fixTool, AgentID: agentID,
		})
	}
	setup := report.Setup

	// ── The SDR loop, in the order it has to be set up ──────────────────
	// None of these has a fix_tool: territory, catalogue and customers are
	// screens in the app (and imports), and the e-mail connects by OAuth.
	// Saying so is the fix — a tool name here would be a lie.
	if setup.HasTerritory != nil && !*setup.HasTerritory {
		add("high", "NO_TERRITORY", "Território de venda não definido",
			"Sem UF/cidades o radar não sabe onde procurar hospitais. Defina em Configurações → Empresa "+
				"(PATCH /accounts pelo app). Não há ferramenta MCP para isso — oriente o usuário à tela.",
			"", "")
	}
	if !setup.ProductsAdded {
		add("high", "NO_PRODUCTS", "Catálogo de produtos vazio",
			"Sem catálogo nenhuma oferta pode ser rascunhada. Importe o CSV em Produtos, no app — "+
				"não há ferramenta MCP para importar.",
			"", "")
	}
	if !setup.CustomersAdded {
		add("medium", "NO_CUSTOMERS", "Nenhum cliente importado",
			"Sem a carteira o produto não sabe quem já compra, quem recompra e quem falta abordar. "+
				"Importe o CSV em Clientes, no app — não há ferramenta MCP para importar.",
			"", "")
	}
	if !setup.HasEmailChannel {
		add("high", "NO_EMAIL_CHANNEL", "Nenhum e-mail conectado",
			"O e-mail é o canal principal de envio: sem ele nenhuma oferta sai. Conecte o Gmail em um "+
				"clique (ou SMTP em Avançado) em Canais, no app. list_channels mostra o que já existe.",
			"", "")
	}
	// Named BEFORE the idle-queue line, and it changes what that line may say.
	//
	// "Aprove ou descarte cada um" was advice nobody could take: the offers
	// were finished and correct, and the recipients had no address any
	// connected channel could reach. Telling someone to act on work that
	// cannot be acted on is how a product teaches them to ignore it.
	undeliverable := int64(0)
	if setup.UndeliverableDrafts != nil {
		undeliverable = *setup.UndeliverableDrafts
	}
	if undeliverable > 0 {
		pending := int64(0)
		if setup.PendingDrafts != nil {
			pending = *setup.PendingDrafts
		}
		add("high", "QUEUE_UNDELIVERABLE", "Ofertas prontas sem endereço para enviar",
			fmt.Sprintf("%d de %d ofertas da fila estão endereçadas a contatos que nenhum canal seu alcança "+
				"(sem e-mail, ou sem telefone para o canal conectado). Aprovar não resolve e descartar joga "+
				"fora trabalho bom: o conserto é achar o contato de compras e preenchê-lo em Clientes. "+
				"enrich_lead ajuda quando há site.", undeliverable, pending),
			"enrich_lead", "")
	}
	if setup.OldestPendingDraftDays != nil && *setup.OldestPendingDraftDays > queueIdleDays {
		suggestion := "Há rascunho esperando aprovação humana há mais de 3 dias. Uma oferta envelhece: " +
			"revise a fila agora (revisar-fila-de-hoje) e aprove ou descarte cada um."
		if undeliverable > 0 {
			suggestion = "Há rascunho esperando aprovação humana há mais de 3 dias. Resolva primeiro " +
				"QUEUE_UNDELIVERABLE — parte desta fila não pode ser enviada, e mandar aprovar o que " +
				"não tem para onde ir é o motivo de ela estar parada."
		}
		add("medium", "QUEUE_IDLE", "Fila de aprovação parada", suggestion, "get_pending_actions", "")
	}
	if setup.HasSalesHistory != nil && !*setup.HasSalesHistory {
		add("low", "NO_SALES_HISTORY", "Carteira sem histórico de vendas",
			"Nenhuma venda registrada: recompra, cross-sell e datas de relacionamento não têm de onde "+
				"partir. Registre as vendas já feitas com register_sale (ou importe em Clientes).",
			"register_sale", "")
	}

	// ── v1 checks (agents answering customers), only with the v1 tools ──
	if uc.legacyIssues {
		uc.detectLegacyIssues(report, boot, add)
	}

	// ── Plan ───────────────────────────────────────────────────────────
	if u := report.Usage; u != nil {
		if u.CapReached {
			add("high", "USAGE_CAP_REACHED", "Limite do plano atingido",
				"Rascunhos e envios ficam parados até o próximo ciclo ou upgrade de plano.",
				"create_checkout_link", "")
		} else if u.TokensUsagePercent >= nearLimitPercent || u.ConversationsUsagePercent >= nearLimitPercent {
			add("medium", "USAGE_NEAR_LIMIT", "Consumo perto do limite do plano",
				"Acima de 80% do limite mensal. Avalie um upgrade para não interromper as ofertas.",
				"get_billing", "")
		}
	}
}

func (uc *GetAccountHealthReportUseCase) detectLegacyIssues(report *AccountHealthReport, boot *DashboardBootstrapResponse, add func(severity, code, title, suggestion, fixTool, agentID string)) {
	if report.Setup.AgentCount == 0 {
		add("high", "NO_AGENT", "Nenhum agente de IA criado",
			"Crie um agente com um prompt que descreva o negócio, o tom de voz e quando escalar para humano.",
			"create_agent", "")
	} else {
		if report.Setup.ActiveAgentCount == 0 {
			add("high", "ALL_AGENTS_PAUSED", "Todos os agentes estão pausados",
				"Nenhuma conversa está sendo respondida automaticamente. Reative um agente.",
				"toggle_agent", "")
		}
		for _, a := range boot.Agents {
			promptLen := 0
			if a.SystemPrompt != nil {
				promptLen = len(*a.SystemPrompt)
			}
			if promptLen < weakPromptMinChars {
				add("medium", "WEAK_PROMPT", "Prompt do agente \""+a.Name+"\" é curto ou genérico",
					"Um prompt forte descreve o negócio, produtos, tom de voz, o que não responder e quando transferir para humano. Proponha uma melhoria e valide com preview_agent_chat.",
					"update_agent", a.ID)
			}
		}
	}

	if report.Setup.ChannelCount == 0 {
		add("high", "NO_CHANNEL", "Nenhum canal conectado",
			"Sem canal, nenhum cliente consegue falar com o agente. O Telegram conecta em 2 minutos com um token do BotFather; WhatsApp conecta pelo dashboard.",
			"create_telegram_channel", "")
	}

	if report.Setup.KnowledgeSourceCount == 0 {
		add("high", "KNOWLEDGE_EMPTY", "Base de conhecimento vazia",
			"O agente responde melhor com contexto do negócio: FAQ, catálogo, políticas de troca/entrega. Adicione conteúdo por texto ou importe de uma URL.",
			"add_knowledge", "")
	}

	if p := report.Performance; p != nil && p.TotalConversations >= minConversationsForPerf {
		if p.CSATResponses > 0 && p.CSATAverage < lowCSATThreshold {
			add("medium", "LOW_CSAT", "Satisfação dos clientes abaixo do ideal",
				"CSAT médio abaixo de 4. Analise as conversas com nota baixa e ajuste o prompt ou a base de conhecimento.",
				"get_analytics", "")
		}
		if p.AvgFirstResponseSeconds > slowFirstResponseSecs {
			add("medium", "SLOW_FIRST_RESPONSE", "Tempo de primeira resposta alto",
				"Primeira resposta acima de 5 minutos em média. Verifique se o bot está pausado em muitas conversas ou se a equipe está sobrecarregada.",
				"get_analytics", "")
		}
	}
}

// computeHealthScore is a simple deduction model: start at 100, subtract
// per issue by severity. The model presents the number; the issues carry
// the substance.
func computeHealthScore(report *AccountHealthReport) int {
	score := 100
	for _, issue := range report.Issues {
		switch issue.Severity {
		case "high":
			score -= 25
		case "medium":
			score -= 10
		default:
			score -= 5
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}
