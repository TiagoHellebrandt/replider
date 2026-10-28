package mcp

import (
	"context"
	"encoding/json"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// metadataFieldInput mirrors domain.MetadataFieldDef so the tool schema is
// self-describing instead of taking an opaque JSON blob.
type metadataFieldInput struct {
	Key         string   `json:"key" jsonschema:"identificador snake_case, único no funil"`
	Label       string   `json:"label" jsonschema:"rótulo exibido no dashboard"`
	Type        string   `json:"type" jsonschema:"text, number, cpf, cnpj, date, bool ou enum"`
	Required    bool     `json:"required,omitempty" jsonschema:"campos required completos disparam o webhook lead.qualified"`
	AIHint      string   `json:"ai_hint,omitempty" jsonschema:"orientação para a IA sobre como/quando coletar o campo"`
	EnumOptions []string `json:"enum_options,omitempty" jsonschema:"valores permitidos quando type=enum"`
}

type upsertFunnelInput struct {
	FunnelID                  string               `json:"funnel_id,omitempty" jsonschema:"quando informado ATUALIZA o funil existente (substituição completa — envie o funil inteiro); vazio cria um novo"`
	Name                      string               `json:"name"`
	PersonaPrompt             string               `json:"persona_prompt,omitempty" jsonschema:"persona de vendas usada pela IA nas conversas e rascunhos do funil"`
	MetadataSchema            []metadataFieldInput `json:"metadata_schema,omitempty" jsonschema:"campos que a IA coleta invisivelmente durante a conversa"`
	WebhookURL                string               `json:"webhook_url,omitempty" jsonschema:"URL que recebe os eventos lead.qualified / lead.buy_intent"`
	WebhookSecret             string               `json:"webhook_secret,omitempty" jsonschema:"segredo HMAC do webhook — nunca é retornado; em update, vazio PRESERVA o segredo atual"`
	TriggerOnRequiredComplete bool                 `json:"trigger_on_required_complete,omitempty" jsonschema:"dispara o webhook quando todos os campos required forem coletados"`
	TriggerOnBuyIntent        bool                 `json:"trigger_on_buy_intent,omitempty" jsonschema:"dispara o webhook quando o lead sinalizar intenção de compra"`
	ChannelID                 string               `json:"channel_id,omitempty" jsonschema:"canal por onde as abordagens do funil são enviadas"`
	AgentID                   string               `json:"agent_id,omitempty"`
	ProductID                 string               `json:"product_id,omitempty"`
	Active                    *bool                `json:"active,omitempty" jsonschema:"padrão true na criação; em update, omitido mantém o valor atual"`
}

// funnelDetail is the full funnel projection minus webhook_secret — the
// secret is write-only over MCP.
type funnelDetail struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ProductID     *string `json:"product_id,omitempty"`
	AgentID       *string `json:"agent_id,omitempty"`
	ChannelID     *string `json:"channel_id,omitempty"`
	PersonaPrompt string  `json:"persona_prompt,omitempty"`
	// Passed through as decoded JSON for the same reason leadSummary's blobs
	// are: a json.RawMessage generates a "null|array" output schema and the
	// SDK then rejects the tool's own answer.
	MetadataSchema            any    `json:"metadata_schema,omitempty"`
	WebhookURL                string `json:"webhook_url,omitempty"`
	HasWebhookSecret          bool   `json:"has_webhook_secret"`
	TriggerOnRequiredComplete bool   `json:"trigger_on_required_complete"`
	TriggerOnBuyIntent        bool   `json:"trigger_on_buy_intent"`
	Active                    bool   `json:"active"`
}

func toFunnelDetail(f *domain.SalesFunnel) *funnelDetail {
	return &funnelDetail{
		ID:                        f.ID,
		Name:                      f.Name,
		ProductID:                 f.ProductID,
		AgentID:                   f.AgentID,
		ChannelID:                 f.ChannelID,
		PersonaPrompt:             f.PersonaPrompt,
		MetadataSchema:            decodeRawJSON(f.MetadataSchema),
		WebhookURL:                f.WebhookURL,
		HasWebhookSecret:          f.WebhookSecret != "",
		TriggerOnRequiredComplete: f.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        f.TriggerOnBuyIntent,
		Active:                    f.Active,
	}
}

type createLeadInput struct {
	FunnelID string            `json:"funnel_id" jsonschema:"funil que recebe o lead (veja list_funnels)"`
	Name     string            `json:"name,omitempty" jsonschema:"nome do contato/empresa"`
	Phone    string            `json:"phone" jsonschema:"telefone em E.164, ex.: +5565999998888 — vira o contato WhatsApp do lead"`
	Metadata map[string]string `json:"metadata,omitempty" jsonschema:"valores iniciais para campos declarados no metadata_schema do funil (chave → valor)"`
}

type createLeadOutput struct {
	LeadID string `json:"lead_id"`
	Status string `json:"status"`
	// ContactID is the contact's opaque id (US-F005), NOT the phone — a
	// contact can be reachable on several identifiers. Phone is echoed
	// separately so an agent can confirm who it just registered.
	ContactID string `json:"contact_id"`
	Phone     string `json:"phone"`
	Created   bool   `json:"created" jsonschema:"false quando já existia lead para o (funil, contato) — retornado sem alterações"`
}

// registerSalesManageTools exposes campaign setup over MCP: creating/updating
// funnels and registering leads manually, so an external agent can assemble an
// outbound campaign end-to-end (upsert_funnel → create_lead → draft →
// human-approved send). Admin-only via registerSalesTools.
func registerSalesManageTools(s *mcpsdk.Server, deps Deps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "upsert_funnel",
		Description: "Cria (sem funnel_id) ou atualiza (com funnel_id) um funil de vendas: nome, persona, campos de " +
			"coleta (metadata_schema), webhook e vínculos com canal/agente/produto. A atualização SUBSTITUI os campos " +
			"do funil — envie o funil completo (o webhook_secret vazio preserva o atual). Combine com create_lead e " +
			"prospect_businesses para montar uma campanha de ponta a ponta.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in upsertFunnelInput) (*mcpsdk.CallToolResult, *funnelDetail, error) {
		out, err := handleUpsertFunnel(ctx, deps, in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "create_lead",
		Description: "Cadastra manualmente um lead em um funil a partir de nome + telefone (E.164), com valores " +
			"iniciais opcionais para os campos do metadata_schema. O lead entra com status 'new', pronto para " +
			"draft_lead_approach → approve_send_approach. Idempotente: se já existir lead para o (funil, contato), " +
			"retorna o existente sem alterar. NÃO envia nenhuma mensagem.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in createLeadInput) (*mcpsdk.CallToolResult, *createLeadOutput, error) {
		out, err := handleCreateLead(ctx, deps, in)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})
}

// handleUpsertFunnel routes to FunnelUC.Create or .Update based on funnel_id.
// Named (not a closure) so unit tests can drive it with a WithIdentity ctx.
func handleUpsertFunnel(ctx context.Context, deps Deps, in upsertFunnelInput) (*funnelDetail, error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}

	schema, err := marshalMetadataSchema(in.MetadataSchema)
	if err != nil {
		return nil, toolError(err)
	}

	if strings.TrimSpace(in.FunnelID) == "" {
		f, err := deps.FunnelUC.Create(ctx, usecases.CreateFunnelDTO{
			AccountID:                 id.AccountID,
			Name:                      in.Name,
			ProductID:                 optString(in.ProductID),
			AgentID:                   optString(in.AgentID),
			ChannelID:                 optString(in.ChannelID),
			PersonaPrompt:             in.PersonaPrompt,
			MetadataSchema:            schema,
			WebhookURL:                in.WebhookURL,
			WebhookSecret:             in.WebhookSecret,
			TriggerOnRequiredComplete: in.TriggerOnRequiredComplete,
			TriggerOnBuyIntent:        in.TriggerOnBuyIntent,
			Active:                    in.Active,
		})
		if err != nil {
			return nil, toolError(err)
		}
		return toFunnelDetail(f), nil
	}

	// Update replaces every field, but the secret is unreadable over MCP (never
	// echoed back), so an empty webhook_secret preserves the stored one instead
	// of silently wiping it. Get also enforces ownership before any write.
	current, err := deps.FunnelUC.Get(ctx, id.AccountID, in.FunnelID)
	if err != nil {
		return nil, toolError(err)
	}
	secret := in.WebhookSecret
	if secret == "" {
		secret = current.WebhookSecret
	}
	f, err := deps.FunnelUC.Update(ctx, usecases.UpdateFunnelDTO{
		AccountID:                 id.AccountID,
		ID:                        in.FunnelID,
		Name:                      in.Name,
		ProductID:                 optString(in.ProductID),
		AgentID:                   optString(in.AgentID),
		ChannelID:                 optString(in.ChannelID),
		PersonaPrompt:             in.PersonaPrompt,
		MetadataSchema:            schema,
		WebhookURL:                in.WebhookURL,
		WebhookSecret:             secret,
		TriggerOnRequiredComplete: in.TriggerOnRequiredComplete,
		TriggerOnBuyIntent:        in.TriggerOnBuyIntent,
		Active:                    in.Active,
	})
	if err != nil {
		return nil, toolError(err)
	}
	return toFunnelDetail(f), nil
}

// handleCreateLead registers a lead manually via LeadUC.Create, scoped to the
// caller's account. Named for the same testability reason as handleUpsertFunnel.
func handleCreateLead(ctx context.Context, deps Deps, in createLeadInput) (*createLeadOutput, error) {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return nil, errNoIdentity
	}
	lead, created, err := deps.LeadUC.Create(ctx, usecases.CreateLeadDTO{
		AccountID: id.AccountID,
		FunnelID:  in.FunnelID,
		Name:      in.Name,
		Phone:     in.Phone,
		Metadata:  in.Metadata,
	})
	if err != nil {
		return nil, toolError(err)
	}
	// Echo the normalized phone (the use case normalizes to E.164) rather
	// than the raw input, so the agent sees what was actually stored.
	phone := in.Phone
	if normalized, ok := domain.NormalizeE164(in.Phone); ok {
		phone = normalized
	}
	return &createLeadOutput{
		LeadID:    lead.ID,
		Status:    string(lead.Status),
		ContactID: lead.ContactID,
		Phone:     phone,
		Created:   created,
	}, nil
}

// marshalMetadataSchema converts the typed tool input into the raw JSON the
// funnel use case validates/normalizes. nil input → nil (UC stores "[]").
func marshalMetadataSchema(fields []metadataFieldInput) (json.RawMessage, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	defs := make([]domain.MetadataFieldDef, 0, len(fields))
	for _, f := range fields {
		defs = append(defs, domain.MetadataFieldDef{
			Key:         f.Key,
			Label:       f.Label,
			Type:        domain.MetadataFieldType(f.Type),
			Required:    f.Required,
			AIHint:      f.AIHint,
			EnumOptions: f.EnumOptions,
		})
	}
	return json.Marshal(defs)
}

func optString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
