package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	waadapter "github.com/replider/api/internal/adapters/channels/whatsapp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// leadSummary is a schema-friendly projection of domain.Lead. Enrichment and
// CollectedMetadata stay raw JSON — Claude reads them as-is.
//
// They are typed `any` and NOT `json.RawMessage`, and that is the whole point
// of this projection. `json.RawMessage` is a `[]byte`, so the SDK generated
// `"type": ["null", "array"]` for it and then validated every OUTPUT against
// that schema — so `get_lead`, `list_funnel_leads` and `approve_send_approach`
// failed outright for any lead carrying enrichment, which in production is
// nearly all of them:
//
//	validating /properties/enrichment: type map[address:… place_id:…]
//	has type "object", want one of "null, array"
//
// The whole "converse with your funnel" surface was dead and nothing noticed,
// because the app reads the REST routes and never comes through here. `any`
// generates an unconstrained schema, which is the honest description of a blob
// we deliberately pass through without shaping.
type leadSummary struct {
	ID            string `json:"id"`
	FunnelID      string `json:"funnel_id"`
	ContactID     string `json:"contact_id"`
	Status        string `json:"status"`
	Score         int    `json:"score"`
	ApproachDraft string `json:"approach_draft,omitempty"`
	Enrichment    any    `json:"enrichment,omitempty"`
	Metadata      any    `json:"collected_metadata,omitempty"`
}

func toLeadSummary(l *domain.Lead) leadSummary {
	return leadSummary{
		ID:            l.ID,
		FunnelID:      l.FunnelID,
		ContactID:     l.ContactID,
		Status:        string(l.Status),
		Score:         l.Score,
		ApproachDraft: l.ApproachDraft,
		Enrichment:    decodeRawJSON(l.Enrichment),
		Metadata:      decodeRawJSON(l.CollectedMetadata),
	}
}

// decodeRawJSON turns a stored JSON blob into the value the tool result
// carries. Unparsable or absent becomes nil so the field is omitted — a lead
// whose enrichment somehow holds garbage still answers with everything else,
// which beats failing the whole read.
func decodeRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}

// registerPipelineTools exposes the outbound-sales pipeline (US-020/022/023):
// import/radar → enrich → AI draft → HUMAN-approved send.
//
// Registered for every sales role: each read resolves through the *ForActor
// use cases, so a vendedor's MCP session lists their own leads and a gestor's
// lists the floor's — the same rows the screens would show them. Only setup
// (upsert_funnel, create_lead, WhatsApp templates) is admin-gated. The
// approval gate is non-negotiable — there is deliberately no tool that sends
// an approach without the user seeing and confirming the text.
func registerPipelineTools(s *mcpsdk.Server, deps Deps, admin bool) {
	type funnelSummary struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		AgentID    *string `json:"agent_id,omitempty"`
		ChannelID  *string `json:"channel_id,omitempty"`
		HasWebhook bool    `json:"has_webhook"`
	}
	type listFunnelsOutput struct {
		Funnels []funnelSummary `json:"funnels"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "list_funnels",
		Description: "Lista os funis de venda da conta (leads são sempre organizados dentro de um funil).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, *listFunnelsOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		funnels, err := deps.FunnelUC.List(ctx, id.AccountID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listFunnelsOutput{Funnels: make([]funnelSummary, 0, len(funnels))}
		for _, f := range funnels {
			out.Funnels = append(out.Funnels, funnelSummary{
				ID:         f.ID,
				Name:       f.Name,
				AgentID:    f.AgentID,
				ChannelID:  f.ChannelID,
				HasWebhook: f.WebhookURL != "",
			})
		}
		return nil, out, nil
	})

	type listLeadsInput struct {
		FunnelID string `json:"funnel_id"`
		Status   string `json:"status,omitempty" jsonschema:"filtra por status: new, prospected, enriched, pending_approval, contacted, responded, qualified, won, lost; vazio lista todos"`
		Limit    int32  `json:"limit,omitempty" jsonschema:"máximo de leads (padrão 50)"`
		Offset   int32  `json:"offset,omitempty"`
	}
	type listLeadsOutput struct {
		Leads []leadSummary `json:"leads"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_funnel_leads",
		Description: "Lista os leads de um funil, opcionalmente filtrados por status. Use status=pending_approval " +
			"para ver a fila de abordagens aguardando aprovação humana.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in listLeadsInput) (*mcpsdk.CallToolResult, *listLeadsOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}
		// US-F009: same per-resource scoping as GET /v1/funnels/:id/leads —
		// a vendedor's MCP session sees their pipeline, not the floor's.
		leads, err := deps.LeadUC.ListByFunnelForActor(ctx, id.Actor(), in.FunnelID, domain.LeadStatus(in.Status), limit, in.Offset)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &listLeadsOutput{Leads: make([]leadSummary, 0, len(leads))}
		for _, l := range leads {
			out.Leads = append(out.Leads, toLeadSummary(l))
		}
		return nil, out, nil
	})

	if deps.LegacyTools {
		registerPlacesProspecting(s, deps)
	}

	type leadIDInput struct {
		LeadID string `json:"lead_id"`
	}
	type enrichOutput struct {
		Started bool   `json:"started"`
		Message string `json:"message"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "enrich_lead",
		Description: "Inicia o enriquecimento de um lead (pesquisa na web via Gemini: site, redes sociais, contexto " +
			"do negócio). Assíncrono — consulte o lead depois com get_lead. Requer provedor Gemini na conta.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in leadIDInput) (*mcpsdk.CallToolResult, *enrichOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		if err := deps.EnrichLeadUC.Start(ctx, id.AccountID, in.LeadID); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &enrichOutput{Started: true, Message: "Enriquecimento iniciado — o resultado aparece no lead em instantes."}, nil
	})

	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "draft_lead_approach",
		Description: "Gera um rascunho de oferta por IA para UM lead — um e-mail (canal principal) com o " +
			"produto certo para o parque daquele hospital — e o coloca na fila de aprovação " +
			"(status pending_approval). NÃO envia nada — o envio exige aprovação humana explícita.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in leadIDInput) (*mcpsdk.CallToolResult, *leadSummary, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		lead, err := deps.ApproachUC.Draft(ctx, id.AccountID, in.LeadID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := toLeadSummary(lead)
		return nil, &out, nil
	})

	type batchDraftInput struct {
		FunnelID string `json:"funnel_id"`
		Limit    int    `json:"limit,omitempty" jsonschema:"máximo de leads a rascunhar neste lote (padrão 10)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "draft_approaches_batch",
		Description: "Gera rascunhos de oferta por IA em lote (e-mails, canal principal) para os leads elegíveis " +
			"do funil e os enfileira para aprovação humana. NÃO envia nada. Contatos com opt-out são pulados " +
			"automaticamente. Se `skipped_budget` vier maior que zero, a franquia mensal de rascunhos do plano " +
			"não cobriu o resto — DIGA isso ao usuário com o número, em vez de relatar só quantos foram gerados.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in batchDraftInput) (*mcpsdk.CallToolResult, *usecases.BatchDraftResult, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 10
		}
		res, err := deps.ApproachUC.DraftBatch(ctx, id.AccountID, in.FunnelID, limit)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, res, nil
	})

	type approveSendInput struct {
		LeadID string `json:"lead_id"`
		Text   string `json:"text" jsonschema:"texto FINAL aprovado pelo usuário — exatamente o que será enviado ao lead"`
		// Email is the product's main channel: without a subject the mail
		// went out as "Mensagem de <empresa>", which reads as spam.
		Subject          string `json:"subject,omitempty" jsonschema:"assunto do e-mail (canal e-mail); ignorado no WhatsApp"`
		TemplateName     string `json:"template_name,omitempty" jsonschema:"template WhatsApp aprovado que carrega a mensagem (obrigatório só no canal WhatsApp)"`
		TemplateLanguage string `json:"template_language,omitempty" jsonschema:"idioma do template (padrão pt_BR)"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "approve_send_approach",
		Description: "Envia a oferta aprovada para um lead em pending_approval — MENSAGEM REAL para o hospital, " +
			"pelo e-mail conectado da conta (canal principal; WhatsApp só se o funil apontar para um canal WhatsApp). " +
			"NUNCA chame sem antes mostrar ao usuário o texto exato, o assunto e o destinatário e receber confirmação " +
			"explícita nesta conversa. Passa pelas guardas de entregabilidade e de opt-out; conta no teto de envios do plano.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in approveSendInput) (*mcpsdk.CallToolResult, *leadSummary, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		lead, err := deps.ApproachUC.ApproveSend(ctx, id.AccountID, in.LeadID, usecases.ApproachSendInput{
			Text:             in.Text,
			ActorUserID:      id.UserID,
			Subject:          in.Subject,
			TemplateName:     in.TemplateName,
			TemplateLanguage: in.TemplateLanguage,
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := toLeadSummary(lead)
		return nil, &out, nil
	})

	// Kanban (US-M008). Same use case as the HTTP endpoints, so the
	// ownership scoping and the stage↔status coupling are identical.
	registerKanbanTools(s, deps)

	if !admin {
		return
	}
	registerWhatsAppTemplateTools(s, deps)
	// Campaign setup (upsert_funnel + create_lead) lives in
	// tools_sales_manage.go; registered here so it inherits the admin gate.
	registerSalesManageTools(s, deps)
}

// registerPlacesProspecting is the v1 Google Places importer. The SDR
// product finds hospitals through CNES (find_hospitals_with_equipment) and
// PNCP signals; Places is kept behind Deps.LegacyTools.
func registerPlacesProspecting(s *mcpsdk.Server, deps Deps) {
	type prospectInput struct {
		FunnelID     string  `json:"funnel_id" jsonschema:"funil que recebe os leads criados"`
		Query        string  `json:"query,omitempty" jsonschema:"busca em texto livre no Google Places, ex.: 'pet shop em Cuiabá'"`
		Latitude     float64 `json:"latitude,omitempty" jsonschema:"busca por proximidade (alternativa à query)"`
		Longitude    float64 `json:"longitude,omitempty"`
		RadiusMeters float64 `json:"radius_meters,omitempty" jsonschema:"raio da busca por proximidade"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "prospect_businesses",
		Description: "Busca empresas no Google Places (por texto OU por raio geográfico) e as importa como leads " +
			"prospectados no funil — deduplicado por telefone, limitado pelo plano. NÃO envia nenhuma mensagem; " +
			"o contato só acontece depois de rascunho + aprovação humana.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in prospectInput) (*mcpsdk.CallToolResult, *usecases.ProspectingResult, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		res, err := deps.ProspectingUC.Execute(ctx, usecases.ProspectingSearchDTO{
			AccountID:    id.AccountID,
			FunnelID:     in.FunnelID,
			Query:        in.Query,
			Latitude:     in.Latitude,
			Longitude:    in.Longitude,
			RadiusMeters: in.RadiusMeters,
			LanguageCode: "pt-BR",
		})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, res, nil
	})
}

// registerWhatsAppTemplateTools is the WhatsApp leg of the send path:
// admin-only, and only meaningful when a funnel points at a WhatsApp channel.
func registerWhatsAppTemplateTools(s *mcpsdk.Server, deps Deps) {
	type syncTemplatesInput struct {
		ChannelID string `json:"channel_id" jsonschema:"canal WhatsApp dono dos templates"`
	}
	type templateSummary struct {
		Name       string `json:"name"`
		Language   string `json:"language"`
		Category   string `json:"category"`
		MetaStatus string `json:"meta_status"`
		Body       string `json:"body"`
	}
	type syncTemplatesOutput struct {
		Templates []templateSummary `json:"templates"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "sync_whatsapp_templates",
		Description: "Sincroniza e lista os templates WhatsApp aprovados pela Meta em um canal. Só templates com " +
			"meta_status APPROVED podem ser usados em approve_send_approach.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in syncTemplatesInput) (*mcpsdk.CallToolResult, *syncTemplatesOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		templates, err := deps.SyncTemplatesUC.Execute(ctx, id.AccountID, in.ChannelID)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out := &syncTemplatesOutput{Templates: make([]templateSummary, 0, len(templates))}
		for _, tpl := range templates {
			out.Templates = append(out.Templates, templateSummary{
				Name:       tpl.Name,
				Language:   tpl.Language,
				Category:   tpl.Category,
				MetaStatus: tpl.MetaStatus,
				Body:       tpl.Body,
			})
		}
		return nil, out, nil
	})

	type createTemplateInput struct {
		ChannelID         string   `json:"channel_id" jsonschema:"canal WhatsApp dono do template"`
		Name              string   `json:"name" jsonschema:"nome do template: minúsculas, números e _ (ex.: abordagem_v1)"`
		Category          string   `json:"category,omitempty" jsonschema:"MARKETING (padrão), UTILITY ou AUTHENTICATION"`
		Language          string   `json:"language,omitempty" jsonschema:"idioma (padrão pt_BR)"`
		Body              string   `json:"body" jsonschema:"texto do corpo; variáveis como {{1}} — não pode começar nem terminar com variável"`
		Footer            string   `json:"footer,omitempty" jsonschema:"rodapé opcional (texto pequeno)"`
		QuickReplyButtons []string `json:"quick_reply_buttons,omitempty" jsonschema:"botões de resposta rápida; MARKETING exige um de descadastro, ex.: 'Parar de receber'"`
		BodyExamples      []string `json:"body_examples,omitempty" jsonschema:"um valor de exemplo por variável {{n}} do corpo (a Meta exige para aprovar)"`
	}
	type createTemplateOutput struct {
		Submitted bool   `json:"submitted"`
		Name      string `json:"name"`
		Message   string `json:"message"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "create_whatsapp_template",
		Description: "Cria um template de mensagem WhatsApp e o submete à aprovação da Meta (minutos a horas). " +
			"Mostre nome, corpo e botões ao usuário antes de submeter. Templates MARKETING exigem um botão de " +
			"descadastro (ex.: 'Parar de receber') — obrigatório, o envio é bloqueado sem ele. Depois de aprovado, " +
			"confira com sync_whatsapp_templates e use em approve_send_approach.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in createTemplateInput) (*mcpsdk.CallToolResult, *createTemplateOutput, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		category := strings.ToUpper(strings.TrimSpace(in.Category))
		if category == "" {
			category = "MARKETING"
		}
		language := in.Language
		if language == "" {
			language = "pt_BR"
		}
		if !waadapter.TemplateNameRegex.MatchString(in.Name) {
			return nil, nil, toolError(&domain.AppError{Code: "VALIDATION_ERROR", Message: "nome inválido: use só minúsculas, números e _ (ex.: abordagem_v1)"})
		}
		if category == "MARKETING" {
			hasOptOut := false
			for _, b := range in.QuickReplyButtons {
				if waadapter.IsOptOutText(b) {
					hasOptOut = true
					break
				}
			}
			if !hasOptOut {
				return nil, nil, toolError(&domain.AppError{Code: "TEMPLATE_MISSING_OPT_OUT", Message: "template MARKETING precisa de um botão de descadastro — inclua 'Parar de receber' em quick_reply_buttons"})
			}
		}
		// Meta requires an example value for every {{n}} in the body.
		varCount := templateVarCount(in.Body)
		if varCount > 0 && len(in.BodyExamples) != varCount {
			return nil, nil, toolError(&domain.AppError{Code: "VALIDATION_ERROR", Message: fmt.Sprintf("o corpo tem %d variável(is) {{n}} — envie exatamente %d body_examples", varCount, varCount)})
		}

		bodyComponent := map[string]interface{}{"type": "BODY", "text": in.Body}
		if varCount > 0 {
			bodyComponent["example"] = map[string]interface{}{"body_text": [][]string{in.BodyExamples}}
		}
		components := []map[string]interface{}{bodyComponent}
		if in.Footer != "" {
			components = append(components, map[string]interface{}{"type": "FOOTER", "text": in.Footer})
		}
		if len(in.QuickReplyButtons) > 0 {
			buttons := make([]map[string]interface{}, 0, len(in.QuickReplyButtons))
			for _, b := range in.QuickReplyButtons {
				buttons = append(buttons, map[string]interface{}{"type": "QUICK_REPLY", "text": b})
			}
			components = append(components, map[string]interface{}{"type": "BUTTONS", "buttons": buttons})
		}
		if reason := waadapter.PrevalidateComponents(components); reason != "" {
			return nil, nil, toolError(&domain.AppError{Code: "VALIDATION_ERROR", Message: reason})
		}

		channel, err := deps.ChannelRepo.GetByID(ctx, in.ChannelID)
		if err != nil || channel == nil || channel.AccountID != id.AccountID {
			return nil, nil, toolError(domain.ErrChannelNotFound)
		}
		if channel.Type != "whatsapp" {
			return nil, nil, toolError(&domain.AppError{Code: "VALIDATION_ERROR", Message: "templates só existem em canais WhatsApp"})
		}
		wabaID, accessToken := waChannelCreds(channel)
		if wabaID == "" || accessToken == "" {
			return nil, nil, toolError(&domain.AppError{Code: "CHANNEL_MISSING_CONFIG", Message: "canal sem waba_id/access_token configurados — reconecte o WhatsApp no dashboard"})
		}

		if err := waadapter.CreateTemplate(ctx, wabaID, accessToken, waadapter.TemplatePayload{
			Name:                in.Name,
			Category:            category,
			Language:            language,
			Components:          components,
			AllowCategoryChange: true,
		}); err != nil {
			return nil, nil, toolError(err)
		}
		return nil, &createTemplateOutput{
			Submitted: true,
			Name:      in.Name,
			Message:   "Template submetido à Meta. A aprovação costuma sair em minutos/horas — acompanhe com sync_whatsapp_templates.",
		}, nil
	})

}

// waChannelCreds pulls waba_id + access_token out of the channel's
// Config bag (interface{} in the domain; the repo hands it back as a
// decrypted map). Mirrors usecases.extractWAChannelCreds.
func waChannelCreds(ch *domain.Channel) (wabaID, accessToken string) {
	cfg := map[string]string{}
	switch v := ch.Config.(type) {
	case map[string]interface{}:
		for k, val := range v {
			if s, ok := val.(string); ok {
				cfg[k] = s
			}
		}
	default:
		raw, _ := json.Marshal(v)
		_ = json.Unmarshal(raw, &cfg)
	}
	return cfg["waba_id"], cfg["access_token"]
}

// templateVarCount returns the highest {{n}} index used in the body —
// Meta's positional params are 1-based and contiguous.
func templateVarCount(body string) int {
	max := 0
	for _, m := range templateVarRegex.FindAllStringSubmatch(body, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max
}

var templateVarRegex = regexp.MustCompile(`\{\{(\d+)\}\}`)
