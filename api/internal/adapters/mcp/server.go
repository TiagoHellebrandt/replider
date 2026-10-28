package mcp

import (
	"context"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

const serverVersion = "1.0.0"

// Deps carries every usecase/repo the tool surface needs — all already
// constructed in main.go's manual DI chain.
type Deps struct {
	// LegacyTools re-registers the archived v1 surface (agents, knowledge,
	// calendars, Telegram, commissions, contracts, signatures, Places
	// prospecting, logs, case report, sequences). Off by default since the
	// SDR pivot: a tool the product no longer sells is a tool the model
	// will reach for. Env: MCP_LEGACY_TOOLS=true.
	LegacyTools bool

	HealthReportUC *usecases.GetAccountHealthReportUseCase
	GetUsageUC     *usecases.GetUsageUseCase

	ListAgentsUC  *usecases.ListAgentsUseCase
	CreateAgentUC *usecases.CreateAgentUseCase
	UpdateAgentUC *usecases.UpdateAgentUseCase
	ToggleAgentUC *usecases.ToggleAgentStatusUseCase
	DeleteAgentUC *usecases.DeleteAgentUseCase
	PreviewChatUC *usecases.PreviewChatUseCase

	ListChannelsUC  *usecases.ListChannelsUseCase
	CreateChannelUC *usecases.CreateChannelUseCase
	UpdateChannelUC *usecases.UpdateChannelUseCase
	ToggleChannelUC *usecases.ToggleChannelUseCase

	KnowledgeUC *usecases.KnowledgeUseCase

	ConvRepo   domain.ConversationRepository
	MsgRepo    domain.MessageRepository
	TakeoverUC *usecases.ConversationTakeoverUseCase

	// CaseMetricsUC serves the prospecting case report (US-M012). Nil
	// leaves the tool unregistered rather than answering with zeros.
	CaseMetricsUC   *usecases.CaseMetricsUseCase
	OverviewUC      *usecases.GetAnalyticsOverviewUseCase
	ExpensiveConvUC *usecases.GetExpensiveConversationsUseCase
	// SDRAnalyticsUC serves the SDR report and its drill-down (I1). Nil
	// hides both from the tool list rather than answering with an error.
	SDRAnalyticsUC *usecases.SDRAnalyticsUseCase
	// CommandCenterUC serves "what needs me right now" (I2).
	CommandCenterUC *usecases.CommandCenterUseCase
	// ExportUC serves the universal export (I3).
	ExportUC *usecases.ExportUseCase
	// WebhookRepo backs upsert_webhook (J2). Nil leaves only the read-only
	// catalogue tool registered.
	WebhookRepo domain.AccountWebhookRepository
	// APIKeyUC backs create_api_key (J1). Nil leaves the tool unregistered —
	// a tool that cannot mint a credential must not pretend it can.
	APIKeyUC *usecases.APIKeyUseCase
	// AppLogUC serves the production log for remote diagnosis. Nil leaves the
	// tool off the list: a log tool with no log would only make the model spend
	// a call discovering that.
	AppLogUC        *usecases.AppLogUseCase
	AttendantPerfUC *usecases.GetAttendantPerformanceUseCase
	LiveSupUC       *usecases.GetLiveSupervisionUseCase

	ListCalendarsUC  *usecases.ListCalendarsUseCase
	CreateCalendarUC *usecases.CreateCalendarUseCase
	UpdateCalendarUC *usecases.UpdateCalendarUseCase

	ListTeamUC   *usecases.ListTeamMembersUseCase
	InviteTeamUC *usecases.InviteTeamMemberUseCase

	FunnelUC        *usecases.FunnelUseCase
	LeadUC          *usecases.LeadUseCase
	ProspectingUC   *usecases.ProspectingUseCase
	EnrichLeadUC    *usecases.EnrichLeadUseCase
	ApproachUC      *usecases.ApproachUseCase
	SyncTemplatesUC *usecases.SyncTemplatesUseCase
	// ChannelRepo backs create_whatsapp_template (resolves the channel's
	// WABA id + access token; reads come back decrypted from the repo).
	ChannelRepo domain.ChannelRepository

	// EmailRiskUC and BriefUC back the inbox tools (§6.10 / §6.8.2). Nil
	// leaves each tool unregistered — a tool that answers "no risk"
	// because it is unwired is worse than no tool.
	EmailRiskUC *usecases.EmailRiskUseCase
	BriefUC     *usecases.ConversationBriefUseCase
	// BrandRepo and EmailTemplateRepo back the identity/templates tools
	// (§6.7 / §6.8). Nil leaves them unregistered.
	BrandRepo         domain.BrandKitRepository
	EmailTemplateRepo domain.EmailTemplateRepository
	// SignalRepo backs the buying-signal tools (§2.1).
	SignalRepo domain.SignalRepository
	// AudienceUC backs the audience tools (§2 D3).
	AudienceUC *usecases.AudienceUseCase
	// SequenceUC and StrategyRepo back the cadence/strategy tools
	// (§1.5 E1, §4 E2/E3).
	SequenceUC   *usecases.SequenceUseCase
	StrategyRepo domain.StrategyRepository
	// SaleUC backs register_sale (§5 F1).
	SaleUC *usecases.SaleUseCase
	// PortfolioRepo and ProductCycles back the portfolio/repurchase tools
	// (§6.5 F2/F3).
	PortfolioRepo domain.PortfolioRepository
	ProductCycles ProductCycleWriter
	// RelationshipUC and CrossSellUC back schedule_greeting and
	// suggest_cross_sell (§6.5 F4/F5).
	RelationshipUC *usecases.RelationshipUseCase
	CrossSellUC    *usecases.CrossSellUseCase
	// CommissionRuleUC backs the commission-rule tools (§6.6 G1). Admin-only.
	CommissionRuleUC *usecases.CommissionRuleUseCase
	// CommissionStmtUC backs the statement tools (§6.6 G2). Admin-only —
	// get_commission_statement is everybody's pay in one answer.
	CommissionStmtUC *usecases.CommissionStatementUseCase
	// ContractUC backs the contract tools (§6.11 G4). Admin-only.
	ContractUC *usecases.ContractUseCase
	// ContractAlertUC backs the clause-alert tools (§6.11 G5/G6). Admin-only.
	ContractAlertUC *usecases.ContractAlertUseCase
	// SignatureUC backs the signature tools (§6.11 G7). Admin-only: sending a
	// document spends the customer's own quota and can register a Sale.
	SignatureUC *usecases.SignatureUseCase
	// CNESParkRepo backs get_installed_park (tese-agente-autonomo): the
	// public DATASUS park per hospital. Nil leaves the tool unregistered.
	CNESParkRepo domain.CNESParkRepository
	// TerritoryHuntUC backs find_hospitals_with_equipment — the same hunt
	// the Radar's REST face runs (one implementation, two doors). Nil
	// leaves the tool unregistered.
	TerritoryHuntUC TerritoryHunter

	PlanRepo    domain.PlanRepository
	AccountRepo domain.AccountRepository
	CheckoutUC  *usecases.CreateCheckoutSessionUseCase

	// AppBaseURL is the API origin (Telegram webhook registration);
	// DashboardURL is where checkout redirects land.
	AppBaseURL   string
	DashboardURL string
}

// ProductCycleWriter sets a product's repurchase cycles.
//
// Declared here rather than imported from the HTTP handlers: two adapters
// must not depend on each other, and the port is two integers.
type ProductCycleWriter interface {
	SetCycles(ctx context.Context, id, accountID string, consumptionDays, shelfLifeDays int) error
}

// Handler owns the pre-built per-tier MCP servers. Building them at
// startup keeps request handling allocation-free and — more importantly
// — means a non-admin's tools/list simply doesn't contain admin tools.
//
// Four tiers mirror the RBAC table (domain/rbac.go): owner/admin get the
// account (team, billing, exports, integrations, funnel setup); gestor
// gets the floor (pipeline + analytics); vendedor gets the pipeline; the
// legacy attendant keeps the inbox. Inside a tier the rows are scoped by
// the ACTOR in each use case (US-F009), so a vendedor's list_funnel_leads
// is their own leads — the same rows the screen would show them.
type Handler struct {
	adminServer     *mcpsdk.Server
	gestorServer    *mcpsdk.Server
	vendedorServer  *mcpsdk.Server
	attendantServer *mcpsdk.Server
	httpHandler     http.Handler
}

func New(deps Deps) *Handler {
	// The health report is the first call of every session; it needs the
	// SDR probes (territory, queue age, carteira) that main.go's bootstrap
	// aggregate does not carry, and it must speak the same legacy/SDR
	// dialect as the tool list it points at.
	if deps.HealthReportUC != nil {
		deps.HealthReportUC.SetSDRProbes(deps.AccountRepo, deps.LeadUC, deps.PortfolioRepo)
		deps.HealthReportUC.SetLegacyIssues(deps.LegacyTools)
	}
	h := &Handler{
		adminServer:     buildServer(deps, string(domain.RoleAdminRole)),
		gestorServer:    buildServer(deps, string(domain.RoleGestor)),
		vendedorServer:  buildServer(deps, string(domain.RoleVendedor)),
		attendantServer: buildServer(deps, string(domain.RoleAtendenteRole)),
	}
	h.httpHandler = mcpsdk.NewStreamableHTTPHandler(
		func(r *http.Request) *mcpsdk.Server {
			id, ok := IdentityFrom(r.Context())
			if !ok {
				return nil // 400s at the SDK layer; auth middleware should prevent this
			}
			return h.serverForRole(id.Role)
		},
		&mcpsdk.StreamableHTTPOptions{
			// Stateless + JSON responses: every POST is independent, no
			// session affinity needed behind nginx.
			Stateless:    true,
			JSONResponse: true,
		},
	)
	return h
}

// HTTPHandler is mounted at /mcp via gin.WrapH.
func (h *Handler) HTTPHandler() http.Handler {
	return h.httpHandler
}

// serverForRole picks the tool surface by role SATISFACTION, not string
// equality: an owner is an admin, a gestor is not. Before this an owner's
// session landed on the attendant server and lost every sales tool.
func (h *Handler) serverForRole(role string) *mcpsdk.Server {
	r := domain.NormalizeRole(role)
	switch {
	case domain.RoleSatisfies(r, domain.RoleAdminRole):
		return h.adminServer
	case domain.RoleSatisfies(r, domain.RoleGestor):
		return h.gestorServer
	case domain.RoleSatisfies(r, domain.RoleVendedor):
		return h.vendedorServer
	default:
		return h.attendantServer
	}
}

func buildServer(deps Deps, role string) *mcpsdk.Server {
	s := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "replider", Title: "Replider", Version: serverVersion},
		&mcpsdk.ServerOptions{Instructions: instructionsPTBR},
	)
	r := domain.NormalizeRole(role)
	admin := domain.RoleSatisfies(r, domain.RoleAdminRole)
	manager := domain.RoleSatisfies(r, domain.RoleGestor)
	seller := domain.RoleSatisfies(r, domain.RoleVendedor)

	registerAccountTools(s, deps)
	// The v1 surface (AI agents answering customers, RAG knowledge,
	// calendars, Telegram bots). Code kept, registration flagged: see
	// Deps.LegacyTools.
	if deps.LegacyTools {
		registerAgentTools(s, deps, admin)
		registerKnowledgeTools(s, deps, admin)
	}
	registerChannelTools(s, deps, admin)
	registerConversationTools(s, deps)
	registerInboxTools(s, deps)
	registerBrandTools(s, deps, admin)
	registerSignalTools(s, deps)
	registerAudienceTools(s, deps)
	// Sequences have no running worker: a tool that promises a cadence
	// nobody executes is a broken promise, so it stays behind the flag.
	if deps.LegacyTools {
		registerSequenceTools(s, deps)
	}
	registerStrategyTools(s, deps)
	registerSaleTools(s, deps)
	registerPortfolioTools(s, deps)
	registerRelationshipTools(s, deps)
	// "Por onde começo?" is the question every role has, so this is not
	// admin-gated: the use case already scopes a salesperson to their own work.
	registerCommandCenterTools(s, deps)
	// Public DATASUS data — same table for every account, so no role gate.
	registerCNESTools(s, deps)
	// The pipeline itself — funnels, leads, drafts, approval, Kanban — is
	// every seller's job. Each tool resolves through the *ForActor use
	// cases, so a vendedor's list is their own leads, a gestor's is the
	// floor's. Only setup (upsert_funnel, create_lead, WhatsApp templates)
	// stays admin.
	if seller {
		registerPipelineTools(s, deps, admin)
	}
	// Reading the floor's numbers is a manager's job, not a seller's.
	if manager {
		registerAnalyticsTools(s, deps)
	}
	if admin {
		if deps.LegacyTools {
			registerLogTools(s, deps)
			registerCaseReportTool(s, deps)
		}
		registerTeamTools(s, deps)
		registerBillingTools(s, deps)
		if deps.LegacyTools {
			// A pay policy is admin business: reading a colleague's rate is
			// reading their salary.
			registerCommissionTools(s, deps)
			registerCommissionStatementTools(s, deps)
			// A contract carries the price, the penalties and the exclusivity a
			// competitor would pay for: admin-only, like the redaction upstream.
			registerContractTools(s, deps)
			registerContractAlertTools(s, deps)
			registerSignatureTools(s, deps)
		}
		// Exporting is admin business: one file carries the customer list, the
		// prices and the pipeline.
		registerExportTools(s, deps)
		// Wiring an integration is admin business: a webhook ships the
		// lifecycle of the whole account to a third party.
		registerWebhookTools(s, deps)
		registerAPIKeyTool(s, deps)
	}
	if deps.LegacyTools {
		registerCalendarTools(s, deps, admin)
	}
	registerPrompts(s, seller, deps.LegacyTools)
	return s
}

// helpers shared by the tools_*.go files

func boolPtr(b bool) *bool { return &b }

var readOnly = &mcpsdk.ToolAnnotations{ReadOnlyHint: true}
