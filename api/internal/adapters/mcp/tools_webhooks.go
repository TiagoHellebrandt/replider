package mcp

import (
	"context"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/replider/api/internal/core/domain"
	"github.com/replider/api/internal/core/usecases"
)

// Lifecycle webhooks (§6.19, J2).
//
// The copilot can answer "como eu ligo isso no meu ERP?" with the actual event
// list and then wire it, instead of sending somebody to a settings screen to
// copy names by hand.
func registerWebhookTools(s *mcpsdk.Server, deps Deps) {
	type eventsInput struct{}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "list_webhook_events",
		Description: "Lista os eventos que uma assinatura de webhook pode nomear, com o que cada um " +
			"significa. Use antes de criar uma assinatura: nome fora desta lista é recusado, e é assim " +
			"que se evita um webhook que parece configurado e nunca dispara.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ eventsInput) (*mcpsdk.CallToolResult, any, error) {
		if _, ok := IdentityFrom(ctx); !ok {
			return nil, nil, errNoIdentity
		}
		return nil, domain.WebhookEventCatalogue(), nil
	})

	if deps.WebhookRepo == nil {
		return
	}

	type upsertInput struct {
		ID          string   `json:"id,omitempty" jsonschema:"id de uma assinatura existente para atualizar; vazio cria uma nova"`
		URL         string   `json:"url" jsonschema:"endpoint HTTPS que vai receber os eventos"`
		Events      []string `json:"events" jsonschema:"nomes vindos de list_webhook_events"`
		Description string   `json:"description,omitempty"`
		Active      *bool    `json:"active,omitempty" jsonschema:"padrão true"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "upsert_webhook",
		Description: "Cria ou atualiza uma assinatura de webhook. O payload vai assinado com HMAC-SHA256; " +
			"o segredo é gerado na criação e mostrado UMA vez — avise o usuário para guardá-lo, porque " +
			"não dá para lê-lo de novo. Evento fora do catálogo é recusado.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in upsertInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}

		events := make([]string, 0, len(in.Events))
		for _, e := range in.Events {
			events = append(events, strings.TrimSpace(e))
		}
		if err := domain.ValidateWebhookEvents(events); err != nil {
			return nil, nil, toolError(err)
		}

		// Create or update against the real repository shape: there is no
		// upsert, so an id names an existing row and its absence means create.
		if in.ID == "" {
			created, err := deps.WebhookRepo.Create(ctx, &domain.AccountWebhook{
				AccountID:   id.AccountID,
				URL:         strings.TrimSpace(in.URL),
				Events:      events,
				IsActive:    in.Active == nil || *in.Active,
				Description: optionalText(in.Description),
				// The secret is minted by the repository. It comes back on
				// THIS response and never again — the model has to hand it to
				// the user now.
			})
			if err != nil {
				return nil, nil, toolError(err)
			}
			return nil, created, nil
		}

		existing, err := deps.WebhookRepo.GetByID(ctx, id.AccountID, strings.TrimSpace(in.ID))
		if err != nil {
			return nil, nil, toolError(err)
		}
		if existing == nil {
			return nil, nil, toolError(&domain.AppError{
				Code: "WEBHOOK_NOT_FOUND", Message: "assinatura não encontrada"})
		}
		existing.URL = strings.TrimSpace(in.URL)
		existing.Events = events
		if in.Active != nil {
			existing.IsActive = *in.Active
		}
		if desc := strings.TrimSpace(in.Description); desc != "" {
			existing.Description = &desc
		}
		hook, err := deps.WebhookRepo.Update(ctx, existing)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, hook, nil
	})
}

// optionalText turns an empty string into a nil pointer, so an omitted
// description is stored as NULL rather than as "".
func optionalText(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

// registerAPIKeyTool mints machine credentials (§6.19, J1).
//
// The one write tool in the product whose OUTPUT is a secret. The description
// says so, because a model that treats it like any other result will happily
// repeat it later in a conversation that gets shared.
func registerAPIKeyTool(s *mcpsdk.Server, deps Deps) {
	if deps.APIKeyUC == nil {
		return
	}
	type createKeyInput struct {
		Name          string   `json:"name" jsonschema:"para que serve a chave, ex: 'ERP Protheus'. Uma parede de chaves sem nome é uma parede que ninguém revoga com confiança."`
		Scopes        []string `json:"scopes" jsonschema:"permissões, ex: [\"leads:read\",\"sales:write\"]. Sem hierarquia: leads:write NÃO inclui leads:read."`
		ExpiresInDays int      `json:"expires_in_days,omitempty" jsonschema:"validade em dias; omitido = sem expiração"`
	}
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name: "create_api_key",
		Description: "Cria uma chave de máquina da CONTA (não de uma pessoa), com escopos explícitos. " +
			"A chave em texto claro volta UMA vez e nunca mais — só o hash fica guardado. " +
			"Mostre a chave ao usuário imediatamente, diga que ela não pode ser recuperada, e NÃO a repita " +
			"depois na conversa. Peça os escopos antes de criar: escopo a mais numa chave que vaza é " +
			"exatamente o dano que o escopo existe para limitar.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in createKeyInput) (*mcpsdk.CallToolResult, any, error) {
		id, ok := IdentityFrom(ctx)
		if !ok {
			return nil, nil, errNoIdentity
		}
		dto := usecases.CreateAPIKeyDTO{
			AccountID:       id.AccountID,
			CreatedByUserID: id.UserID,
			Name:            strings.TrimSpace(in.Name),
			Scopes:          in.Scopes,
		}
		if in.ExpiresInDays > 0 {
			days := in.ExpiresInDays
			dto.ExpiresInDays = &days
		}
		created, err := deps.APIKeyUC.Create(ctx, dto)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return nil, created, nil
	})
}
