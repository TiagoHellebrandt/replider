package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connect spins an in-memory client session against a role's server.
func connect(t *testing.T, role string) *mcpsdk.ClientSession {
	t.Helper()
	return connectFor(t, Deps{}, role)
}

func connectFor(t *testing.T, deps Deps, role string) *mcpsdk.ClientSession {
	t.Helper()
	server := buildServer(deps, role)
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func listToolNames(t *testing.T, session *mcpsdk.ClientSession) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for name := range listTools(t, session) {
		names[name] = true
	}
	return names
}

func listTools(t *testing.T, session *mcpsdk.ClientSession) map[string]*mcpsdk.Tool {
	t.Helper()
	res, err := session.ListTools(context.Background(), &mcpsdk.ListToolsParams{})
	require.NoError(t, err)
	byName := map[string]*mcpsdk.Tool{}
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
	}
	return byName
}

func listToolNamesFor(t *testing.T, deps Deps, role string) map[string]bool {
	t.Helper()
	return listToolNames(t, connectFor(t, deps, role))
}

func listPromptNames(t *testing.T, session *mcpsdk.ClientSession) map[string]bool {
	t.Helper()
	res, err := session.ListPrompts(context.Background(), &mcpsdk.ListPromptsParams{})
	require.NoError(t, err)
	names := map[string]bool{}
	for _, p := range res.Prompts {
		names[p.Name] = true
	}
	return names
}

// legacyToolNames is the archived v1 surface. Every one of these is code
// the repo keeps and the SDR server no longer lists.
var legacyToolNames = []string{
	"list_agents", "create_agent", "update_agent", "toggle_agent", "delete_agent", "preview_agent_chat",
	"list_knowledge_sources", "add_knowledge",
	"create_telegram_channel",
	"list_calendars", "upsert_calendar",
	"prospect_businesses",
	"list_commission_rules", "simulate_commission", "upsert_commission_rule",
	"get_commission_statement", "mark_commission_paid",
	"get_contract_summary", "upload_contract", "get_contract_red_flags", "set_clause_alert", "list_expiring_contracts",
	"create_signature_request", "get_signature_status",
	"query_logs", "get_case_report",
	"list_sequences", "add_lead_to_sequence", "pause_sequence",
}

// pipelineToolNames is the seller's job: everything here resolves through
// the *ForActor use cases, so the same list serves vendedor and gestor.
var pipelineToolNames = []string{
	"list_funnels", "list_funnel_leads", "enrich_lead",
	"draft_lead_approach", "draft_approaches_batch", "approve_send_approach",
	"get_funnel_stages", "move_lead", "get_lead",
}

// accountToolNames need the account, not the pipeline: admin only.
var accountToolNames = []string{
	"list_team_members", "invite_team_member",
	"get_billing", "create_checkout_link",
	"upsert_funnel", "create_lead",
	"sync_whatsapp_templates", "create_whatsapp_template",
	"list_webhook_events",
}

func TestMCPServer_InstructionsAreSDRPersona(t *testing.T) {
	session := connect(t, "admin")
	init := session.InitializeResult()
	require.NotNil(t, init)
	assert.Contains(t, init.Instructions, "SDR")
	assert.Contains(t, init.Instructions, "hospita")
	assert.Contains(t, init.Instructions, "get_account_health")
	assert.Contains(t, init.Instructions, "get_pending_actions")
	assert.Contains(t, init.Instructions, "approve_send_approach")
	assert.Contains(t, init.Instructions, "send_message_to_customer")
	assert.Contains(t, init.Instructions, "R$")
	assert.Contains(t, init.Instructions, "timezone")
	// The persona must not point at the v1 product or its tools.
	for _, stale := range []string{"Customer Success", "update_agent", "preview_agent_chat", "prospect_businesses", "Telegram", "Instagram"} {
		assert.NotContains(t, init.Instructions, stale)
	}
}

// Role SATISFACTION, not string equality: an owner is an admin, a gestor
// runs the floor, a vendedor works their pipeline, and anything unknown
// degrades to the least-privileged surface.
func TestMCPServer_ServerSelectionFollowsTheRBACTable(t *testing.T) {
	h := New(Deps{})

	assert.Same(t, h.adminServer, h.serverForRole("owner"))
	assert.Same(t, h.adminServer, h.serverForRole("admin"))
	assert.Same(t, h.gestorServer, h.serverForRole("gestor"))
	assert.Same(t, h.vendedorServer, h.serverForRole("vendedor"))
	assert.Same(t, h.attendantServer, h.serverForRole("attendant"))
	assert.Same(t, h.attendantServer, h.serverForRole(""))
	assert.Same(t, h.attendantServer, h.serverForRole("super_admin"))
}

func TestMCPServer_OwnerAndAdminSeeTheAccountSurface(t *testing.T) {
	for _, role := range []string{"owner", "admin"} {
		names := listToolNames(t, connect(t, role))
		for _, tool := range append(append([]string{
			"get_account_health", "get_usage", "list_channels", "update_channel",
			"list_conversations", "get_conversation", "assume_conversation", "send_message_to_customer", "release_conversation",
			"get_analytics",
		}, pipelineToolNames...), accountToolNames...) {
			assert.True(t, names[tool], "%s tools/list missing %s", role, tool)
		}
	}
}

func TestMCPServer_GestorSeesTheFloorNotTheAccount(t *testing.T) {
	names := listToolNames(t, connect(t, "gestor"))
	for _, tool := range append([]string{"get_account_health", "get_analytics", "list_inbox"}, pipelineToolNames...) {
		assert.True(t, names[tool], "gestor tools/list missing %s", tool)
	}
	for _, tool := range accountToolNames {
		assert.False(t, names[tool], "gestor tools/list must not contain %s", tool)
	}
}

func TestMCPServer_VendedorGetsTheActorScopedPipeline(t *testing.T) {
	names := listToolNames(t, connect(t, "vendedor"))
	for _, tool := range append([]string{"get_account_health", "get_usage", "list_channels"}, pipelineToolNames...) {
		assert.True(t, names[tool], "vendedor tools/list missing %s", tool)
	}
	for _, tool := range append([]string{"get_analytics"}, accountToolNames...) {
		assert.False(t, names[tool], "vendedor tools/list must not contain %s", tool)
	}
}

func TestMCPServer_AttendantDoesNotSeeThePipeline(t *testing.T) {
	names := listToolNames(t, connect(t, "attendant"))
	// Operational surface stays.
	for _, tool := range []string{
		"get_account_health", "get_usage", "list_channels",
		"list_conversations", "get_conversation",
		"assume_conversation", "send_message_to_customer", "release_conversation",
		"list_inbox",
	} {
		assert.True(t, names[tool], "attendant tools/list missing %s", tool)
	}
	for _, tool := range append(append([]string{"get_analytics", "update_channel"}, pipelineToolNames...), accountToolNames...) {
		assert.False(t, names[tool], "attendant tools/list must not contain %s", tool)
	}
}

// The v1 surface is code the repo keeps and the server no longer lists — a
// tool the product does not sell is a tool the model will reach for.
func TestMCPServer_LegacyToolsHiddenByDefault(t *testing.T) {
	for _, role := range []string{"owner", "admin", "gestor", "vendedor", "attendant"} {
		names := listToolNames(t, connect(t, role))
		for _, tool := range legacyToolNames {
			assert.False(t, names[tool], "%s tools/list must not contain legacy %s", role, tool)
		}
	}
}

func TestMCPServer_LegacyFlagRestoresTheV1Surface(t *testing.T) {
	names := listToolNamesFor(t, Deps{LegacyTools: true}, "admin")
	// The ones that register without a wired dependency (the rest nil-guard).
	for _, tool := range []string{
		"list_agents", "create_agent", "update_agent", "toggle_agent", "delete_agent", "preview_agent_chat",
		"list_knowledge_sources", "add_knowledge", "create_telegram_channel",
		"list_calendars", "upsert_calendar", "prospect_businesses",
	} {
		assert.True(t, names[tool], "legacy admin tools/list missing %s", tool)
	}
	// And the SDR surface is still there alongside.
	for _, tool := range pipelineToolNames {
		assert.True(t, names[tool])
	}
}

// Counts, for the record: the report that motivated this reads "82 tools,
// ~25 of them v1". The numbers are logged rather than asserted so a new
// tool does not fail an unrelated test.
func TestMCPServer_ToolCounts(t *testing.T) {
	sdr := len(listToolNames(t, connect(t, "admin")))
	legacy := len(listToolNamesFor(t, Deps{LegacyTools: true}, "admin"))
	t.Logf("admin tools with nil deps: sdr=%d legacy=%d", sdr, legacy)
	assert.Greater(t, legacy, sdr)
}

func TestMCPServer_DestructiveAndReadOnlyAnnotations(t *testing.T) {
	byName := listTools(t, connectFor(t, Deps{LegacyTools: true}, "admin"))
	require.NotNil(t, byName["delete_agent"].Annotations)
	require.NotNil(t, byName["delete_agent"].Annotations.DestructiveHint)
	assert.True(t, *byName["delete_agent"].Annotations.DestructiveHint)
	require.NotNil(t, byName["get_account_health"].Annotations)
	assert.True(t, byName["get_account_health"].Annotations.ReadOnlyHint)
}

// Descriptions are what the model reads. They must describe the product
// that exists: e-mail as the channel, no autopilot, no "enriching" state
// the pipeline never shows.
func TestMCPServer_DescriptionsSpeakTheSDRProduct(t *testing.T) {
	byName := listTools(t, connectFor(t, Deps{RelationshipUC: nil}, "admin"))
	assert.Contains(t, byName["approve_send_approach"].Description, "e-mail")
	assert.Contains(t, byName["draft_lead_approach"].Description, "e-mail")
	assert.Contains(t, byName["draft_approaches_batch"].Description, "e-mail")

	raw, err := json.Marshal(byName["list_funnel_leads"].InputSchema)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "enriching")
	assert.Contains(t, string(raw), "pending_approval")
}

func TestMCPServer_PromptsPerRole(t *testing.T) {
	sdrPrompts := []string{"revisar-fila-de-hoje", "preparar-oferta", "o-que-mudou-na-carteira"}
	legacyPrompts := []string{"melhorar-prompt-do-agente", "configurar-conta", "analisar-atendimento-semanal"}

	for _, role := range []string{"owner", "admin", "gestor", "vendedor"} {
		names := listPromptNames(t, connect(t, role))
		for _, p := range sdrPrompts {
			assert.True(t, names[p], "%s prompts missing %s", role, p)
		}
		for _, p := range legacyPrompts {
			assert.False(t, names[p], "%s prompts must not list legacy %s", role, p)
		}
	}

	att := listPromptNames(t, connect(t, "attendant"))
	for _, p := range sdrPrompts {
		assert.False(t, att[p], "attendant has no pipeline, so no %s", p)
	}

	legacy := listPromptNames(t, connectFor(t, Deps{LegacyTools: true}, "admin"))
	for _, p := range append(sdrPrompts, legacyPrompts...) {
		assert.True(t, legacy[p], "legacy admin prompts missing %s", p)
	}
}

func TestMCPServer_ToolWithoutIdentityReturnsIsError(t *testing.T) {
	session := connect(t, "admin")
	// No Identity in ctx (in-memory transport has no GinBridge) → the
	// handler error must surface as an IsError tool result, not a
	// protocol error.
	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "get_usage",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestGinBridge_CopiesIdentityIntoRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accountID", "acc1")
		c.Set("userID", "u1")
		c.Set("userName", "Ana")
		c.Set("role", "admin")
	})
	r.Use(GinBridge(nil))
	var got Identity
	var ok bool
	r.GET("/probe", func(c *gin.Context) {
		got, ok = IdentityFrom(c.Request.Context())
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))
	require.True(t, ok)
	assert.Equal(t, Identity{AccountID: "acc1", UserID: "u1", UserName: "Ana", Role: "admin"}, got)
}

// ── Inbox tools (§6.10 camada 3) ──────────────────────────────────────────
//
// The parity rule from the spec: every action a person can take on a screen
// has an equivalent tool. The co-pilot is how an operator works when they are
// on a phone, and a feature that exists only in the dashboard is one they
// cannot use where they actually are.

func TestMCPServer_InboxListingIsAvailableToEveryRole(t *testing.T) {
	for _, role := range []string{"owner", "admin", "gestor", "vendedor", "attendant"} {
		names := listToolNames(t, connect(t, role))
		assert.True(t, names["list_inbox"], "role %q must be able to read the inbox", role)
	}
}

// A tool that answers "sem risco" because it is unwired is worse than no
// tool: it is a confident wrong answer about whether to pay a boleto.
func TestMCPServer_RiskToolsStayUnregisteredWhenUnwired(t *testing.T) {
	names := listToolNames(t, connect(t, "admin"))

	assert.False(t, names["mark_legitimate"])
	assert.False(t, names["report_scam"])
	assert.False(t, names["get_thread_summary"])
}

// Writing the brand kit is an admin act: the forbidden-claims list is a
// compliance control, not a preference, and a salesperson must not be able
// to widen what they are allowed to promise.
func TestMCPServer_BrandKitWritesAreAdminOnly(t *testing.T) {
	deps := Deps{BrandRepo: stubBrandRepo{}, EmailTemplateRepo: stubTemplateRepo{}}

	admin := listToolNamesFor(t, deps, "admin")
	assert.True(t, admin["update_brand_kit"])

	for _, role := range []string{"gestor", "vendedor", "attendant"} {
		names := listToolNamesFor(t, deps, role)
		assert.False(t, names["update_brand_kit"], "%s must not write the brand kit", role)
		// Reading it is fine for anyone: a salesperson needs to know what they
		// may and may not say.
		assert.True(t, names["get_brand_kit"])
		assert.True(t, names["list_email_templates"])
		assert.True(t, names["get_email_metrics"])
		assert.False(t, names["upsert_email_template"])
	}
}

type stubBrandRepo struct{ domain.BrandKitRepository }
type stubTemplateRepo struct{ domain.EmailTemplateRepository }
