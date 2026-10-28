package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/replider/api/internal/core/domain"
)

// personaMetaPrompt instructs the model to write a sales persona (system
// prompt body) in PT-BR with a consultative tone. The output is fed back
// into a funnel's agent as its persona_prompt, so it must read as direct
// runtime instructions — no preamble, no markdown headings.
const personaMetaPrompt = `Você é um especialista em criar personas de vendas consultivas para agentes de IA que conversam com clientes via WhatsApp em português do Brasil.

A partir das informações do produto/serviço abaixo, escreva o CORPO de um system prompt que define a persona de um vendedor consultivo. Regras:
- Escreva SOMENTE o corpo da persona, em português do Brasil — sem preâmbulo, sem explicações, sem títulos em markdown, sem blocos de código.
- Tom consultivo e acolhedor: a persona faz perguntas, entende a necessidade do cliente e recomenda o produto de forma natural — nunca insistente ou robótica.
- Inclua: identidade do vendedor, conhecimento sobre o produto (nome, benefícios, faixa de preço quando informada), abordagem de vendas consultiva e regras de tom.
- Nunca invente fatos sobre o produto que não foram informados. Quando faltar um dado, oriente a persona a perguntar ou encaminhar para um humano.
- Instruções imperativas e diretas ("Sempre…", "Nunca…"). No máximo 400 palavras.`

// GeneratePersonaDTO is the input for the persona generator. Either
// ProductID (fetched + ownership-checked) or a free-form Description must
// be supplied; ProductID takes precedence when both are present.
type GeneratePersonaDTO struct {
	AccountID   string
	ProductID   string
	Description string
}

// GeneratePersonaResponse carries the generated persona body.
type GeneratePersonaResponse struct {
	Persona string `json:"persona"`
}

// GeneratePersonaUseCase generates a sales persona prompt from a product
// (or free description) using the account's AI provider/BYOK key (US-005),
// mirroring the BYOK resolution path of the prompt-synthesis use cases.
type GeneratePersonaUseCase struct {
	accountRepo domain.AccountRepository
	planRepo    domain.PlanRepository
	productRepo domain.ProductRepository
	aiFactory   domain.AIFactory
}

func NewGeneratePersonaUseCase(
	accountRepo domain.AccountRepository,
	planRepo domain.PlanRepository,
	productRepo domain.ProductRepository,
	aiFactory domain.AIFactory,
) *GeneratePersonaUseCase {
	return &GeneratePersonaUseCase{
		accountRepo: accountRepo,
		planRepo:    planRepo,
		productRepo: productRepo,
		aiFactory:   aiFactory,
	}
}

func (uc *GeneratePersonaUseCase) Execute(ctx context.Context, dto GeneratePersonaDTO) (*GeneratePersonaResponse, error) {
	productID := strings.TrimSpace(dto.ProductID)
	description := strings.TrimSpace(dto.Description)
	if productID == "" && description == "" {
		return nil, &domain.AppError{Code: "PERSONA_INPUT_REQUIRED", Message: "product_id or description is required"}
	}

	account, err := uc.accountRepo.GetByID(ctx, dto.AccountID)
	if err != nil || account == nil {
		return nil, domain.ErrAccountNotFound
	}

	plan := domain.TrialPlan()
	if account.PlanID != nil {
		if p, err := uc.planRepo.GetByID(ctx, *account.PlanID); err == nil && p != nil {
			plan = p
		}
	}

	// Resolve the product description for the prompt. A product_id is
	// always ownership-checked; a free description is used as-is.
	payload := description
	if productID != "" {
		product, err := uc.productRepo.GetByID(ctx, productID)
		if err != nil || product == nil {
			return nil, domain.ErrProductNotFound
		}
		if product.AccountID != dto.AccountID {
			return nil, domain.ErrProductNotFound
		}
		payload = buildProductPersonaPayload(product, description)
	}

	provider, err := uc.aiFactory.GetProviderForAccount(ctx, plan.AIProvider, account, plan)
	if err != nil {
		// BYOK key absent → tailored sales-facing error so the UI can
		// nudge the admin to configure their AI key.
		if errors.Is(err, domain.ErrBYOKKeyMissing) {
			return nil, domain.ErrBYOKNotConfigured
		}
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("failed to initialize AI provider: %v", err),
		}
	}

	resp, err := provider.GenerateResponse(ctx, personaMetaPrompt, []domain.ChatMessage{
		{Role: "user", Content: payload},
	}, nil)
	if err != nil || resp == nil {
		return nil, &domain.AppError{
			Code:    domain.ErrAIProviderUnavailable.Code,
			Message: fmt.Sprintf("AI provider failed to generate persona: %v", err),
		}
	}

	persona := strings.TrimSpace(resp.Content)
	if persona == "" {
		return nil, &domain.AppError{Code: "PERSONA_GENERATION_EMPTY", Message: "AI returned an empty persona"}
	}

	return &GeneratePersonaResponse{Persona: persona}, nil
}

// buildProductPersonaPayload renders the product fields the model needs.
// Free-form attributes (JSONB) are flattened into "key: value" lines so
// vertical-specific data (warranty, sku, ...) reaches the prompt without
// the caller knowing the schema. An optional extra description is appended.
func buildProductPersonaPayload(p *domain.Product, extra string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Produto: %s\n", p.Name)
	if d := strings.TrimSpace(p.Description); d != "" {
		fmt.Fprintf(&b, "Descrição: %s\n", d)
	}
	if p.PriceCents > 0 {
		currency := p.Currency
		if currency == "" {
			currency = "BRL"
		}
		fmt.Fprintf(&b, "Preço: %.2f %s\n", float64(p.PriceCents)/100.0, currency)
	}
	if len(p.Attributes) > 0 {
		var attrs map[string]any
		if err := json.Unmarshal(p.Attributes, &attrs); err == nil && len(attrs) > 0 {
			b.WriteString("Atributos:\n")
			for k, v := range attrs {
				fmt.Fprintf(&b, "  - %s: %v\n", k, v)
			}
		}
	}
	if extra = strings.TrimSpace(extra); extra != "" {
		fmt.Fprintf(&b, "Contexto adicional: %s\n", extra)
	}
	return b.String()
}
